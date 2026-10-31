package isureal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

func TestSearchWriteResetAndLegacy(t *testing.T) {
	db, _ := architectureDB(t)
	// Exercise upgrading a deployed schema and an old initial_users snapshot.
	mustSQL(t, db, `ALTER TABLE users DROP COLUMN search_username, DROP COLUMN search_display_name`)
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,region,created_at)
 VALUES('owner','owner',repeat('ab',32),'Straße','asia','2025-01-01')`)
	execSQLFile(t, db, "003-initial-snapshot.sql")
	if err := prepareUserSearch(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	h := &HandlerContext{DB: db}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.POST("/api/auth/register", registerHandler(h))
	e.PATCH("/api/me", func(c *echo.Context) error {
		c.Set("session", &Session{UserID: "owner"})
		return updateMeHandler(h)(c)
	})
	call := func(method, path, body string, status int) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
	}
	check := func(query string, expected ...string) {
		t.Helper()
		var rows []searchCandidate
		if err := db.Select(&rows, searchCandidatesSQL, "viewer", "", "", normalizeSearchText(query)); err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, r := range rows {
			if strings.Contains(normalizeSearchText(r.Username), normalizeSearchText(query)) || strings.Contains(normalizeSearchText(r.DisplayName), normalizeSearchText(query)) {
				names = append(names, r.Username)
			}
		}
		if strings.Join(names, ",") != strings.Join(expected, ",") {
			t.Fatalf("query %q: %v, want %v", query, names, expected)
		}
	}
	check("STRASSE", "owner")
	call(http.MethodPost, "/api/auth/register", `{"username":"new_user","displayName":"CAFÉ","region":"asia"}`, 201)
	check("cafe\u0301", "new_user")
	call(http.MethodPatch, "/api/me", `{"displayName":"ﬃ"}`, 200)
	check("ffi", "owner")
	check("STRASSE")
	// Old binaries do not know about search columns. The trigger invalidates
	// the old value, so even before backfill the exact Go fallback sees the edit.
	mustSQL(t, db, `UPDATE users SET display_name='Σςσ' WHERE id='owner'`)
	check("σσσ", "owner")
	check("ffi")
	if err := prepareUserSearch(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	check("σσσ", "owner")
	for range 2 {
		execSQLFile(t, db, "reset.sql")
		// reset disables triggers, so its explicit NULL assignment is essential.
		check("STRASSE", "owner")
		check("σσσ")
		check("café")
		if err := prepareUserSearch(t.Context(), db); err != nil {
			t.Fatal(err)
		}
		check("STRASSE", "owner")
	}
	// An old init.sh disables triggers and does not clear the new columns.
	// Returning to the new binary must repair even non-NULL stale keys.
	mustSQL(t, db, `SET session_replication_role=replica; UPDATE users SET display_name='CAFÉ' WHERE id='owner'; SET session_replication_role=origin`)
	if err := prepareUserSearch(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	check("café", "owner")
	check("STRASSE")
}

func TestSearchCandidateVolume(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,region)
 SELECT 'u'||n, 'user_'||lpad(n::text,5,'0'), repeat('ab',32),
 CASE WHEN n<=100 THEN 'Straße' ELSE 'その他の利用者 '||n END, 'asia'
 FROM generate_series(1,5000) n`)
	start := time.Now()
	if err := prepareUserSearch(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	t.Logf("prepare 5000 synthetic users: %s", time.Since(start))
	mustSQL(t, db, "ANALYZE users")
	old := `SELECT id,username,display_name FROM users WHERE status='active' AND id<>$1 AND ($2='' OR (username,id)>($2,$3))
 AND id NOT IN (SELECT CASE WHEN blocker_id=$1 THEN blocked_id ELSE blocker_id END FROM block_relations WHERE blocker_id=$1 OR blocked_id=$1)
 ORDER BY username,id`
	for _, query := range []string{"strasse", "no-match", "user"} {
		for _, optimized := range []bool{false, true} {
			var rows []searchCandidate
			start = time.Now()
			for range 10 {
				rows = nil
				stmt, args := old, []any{"viewer", "", ""}
				if optimized {
					stmt, args = searchCandidatesSQL, append(args, query)
				}
				if err := db.Select(&rows, stmt, args...); err != nil {
					t.Fatal(err)
				}
				found := 0
				for _, r := range rows {
					if strings.Contains(normalizeSearchText(r.Username), query) || strings.Contains(normalizeSearchText(r.DisplayName), query) {
						found++
						if found == 51 {
							break
						}
					}
				}
			}
			if optimized && len(rows) > 51 {
				t.Fatal("too many prepared rows")
			}
			data, _ := json.Marshal(rows)
			t.Logf("query=%q optimized=%v rows=%d serialized-bytes=%d query+filter-mean=%s", query, optimized, len(rows), len(data), time.Since(start)/10)
		}
	}
}

func TestSearchConcurrentPreparation(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `ALTER TABLE users DROP COLUMN search_username, DROP COLUMN search_display_name`)
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,region) VALUES('owner','owner',repeat('ab',32),'Straße','asia')`)
	start := make(chan struct{})
	results := make(chan error, 3)
	for range 3 {
		go func() { <-start; results <- prepareUserSearch(t.Context(), db) }()
	}
	close(start)
	for range 3 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	var key string
	if err := db.Get(&key, `SELECT search_display_name FROM users WHERE id='owner'`); err != nil {
		t.Fatal(err)
	}
	if key != "strasse" {
		t.Fatal(key)
	}
}
