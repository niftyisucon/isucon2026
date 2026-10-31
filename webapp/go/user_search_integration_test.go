package isureal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

// Compare the HTTP response with the original full-row search using a disposable DB.
func TestSearchUsersLocal(t *testing.T) {
	dsn := os.Getenv("ISUCON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ISUCON_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("expected PostgreSQL URL")
	}
	admin, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "search_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(`CREATE SCHEMA ` + name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(`DROP SCHEMA ` + name + ` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	params := parsed.Query()
	params.Set("search_path", name)
	parsed.RawQuery = params.Encode()
	db, err := sqlx.Connect("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("../sql/001-schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("local fixture"))
	key := hex.EncodeToString(sum[:])
	add := func(id, username, display, status, kind string) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO users(id,username,secret_key_hash,display_name,region,bio,status,account_type) VALUES ($1,$2,$3,$4,'asia','private bio',$5,$6)`, id, username, key, display, status, kind); err != nil {
			t.Fatal(err)
		}
	}
	add("viewer", "viewer", "Straße", "active", "regular")
	add("deleted", "deleted", "Straße", "deleted", "regular")
	for i, display := range []string{"Straße", "STRASSE", "Σςσ", "Cafe\u0301", "CAFÉ", "100%_\\literal", "🙂", "İIıi", "ﬃ", "hello"} {
		add(fmt.Sprintf("unicode-%d", i), fmt.Sprintf("unicode-%02d", i), display, "active", "certified_public")
	}
	for i := 0; i < 130; i++ {
		// Usernames are unique in the real schema; cursor cases below still
		// exercise both sides of the (username, id) comparison.
		add(fmt.Sprintf("bulk-%03d", i), fmt.Sprintf("bulk-%03d", i), "needle", "active", "regular")
		if i < 60 {
			blocker, blocked := "viewer", fmt.Sprintf("bulk-%03d", i)
			if i%2 == 0 {
				blocker, blocked = blocked, blocker
			}
			if _, err := db.Exec(`INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES ($1,$2,$3)`, uuid.NewString(), blocker, blocked); err != nil {
				t.Fatal(err)
			}
		}
	}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.GET("/api/users", func(c *echo.Context) error {
		c.Set("session", &Session{UserID: "viewer"})
		return searchUsersHandler(&HandlerContext{DB: db})(c)
	})
	reference := func(q, beforeUsername, beforeID string) searchUsersResponse {
		t.Helper()
		var users []User
		if err := db.Select(&users, `SELECT `+strings.Join(userColumns, ", ")+` FROM users WHERE status='active' AND id<>$1 AND ($2='' OR (username,id)>($2,$3)) ORDER BY username,id`, "viewer", beforeUsername, beforeID); err != nil {
			t.Fatal(err)
		}
		blocked, err := getBlockedUserIDs(t.Context(), db, "viewer")
		if err != nil {
			t.Fatal(err)
		}
		result := searchUsersResponse{Items: make([]relationshipUserResponse, 0)}
		for _, user := range users {
			query := normalizeSearchText(q)
			if (!strings.Contains(normalizeSearchText(user.Username), query) && !strings.Contains(normalizeSearchText(user.DisplayName), query)) || blocked[user.ID] {
				continue
			}
			if err := user.setImageKey(); err != nil {
				t.Fatal(err)
			}
			if user.AccountType != "certified_public" {
				user.Bio = ""
			}
			result.Items = append(result.Items, newRelationshipUserResponse(user))
			if len(result.Items) == 51 {
				break
			}
		}
		result.HasMore = len(result.Items) > 50
		if result.HasMore {
			result.Items = result.Items[:50]
		}
		return result
	}
	for _, phase := range []string{"legacy", "prepared", "mixed"} {
		if phase == "prepared" {
			if err := prepareUserSearch(t.Context(), db); err != nil {
				t.Fatal(err)
			}
			var candidates []searchCandidate
			if err := db.Select(&candidates, searchCandidatesSQL, "viewer", "", "", "needle"); err != nil {
				t.Fatal(err)
			}
			if len(candidates) != 51 {
				t.Fatalf("prepared candidates=%d, want 51", len(candidates))
			}
		}
		if phase == "mixed" {
			// More than 51 unprepared nonmatches must not hide prepared matches.
			if _, err := db.Exec(`INSERT INTO users(id,username,secret_key_hash,display_name,region)
 SELECT 'legacy-'||n, 'aaaa-'||n, repeat('ab',32), 'unrelated', 'asia' FROM generate_series(1,60) n`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE users SET display_name='legacy nonmatch' WHERE username LIKE 'bulk-%' AND username<'bulk-065'`); err != nil {
				t.Fatal(err)
			}
		}
		for _, q := range []string{"STRASSE", "ς", "café", "CAFE\u0301", "%", "_", "\\", "🙂", "i", "ffi", "needle", "BULK", "no match"} {
			t.Run(phase+"/"+q, func(t *testing.T) {
				beforeUsername, beforeID := "", ""
				for page := 0; page < 4; page++ {
					params := url.Values{"q": {q}, "beforeUsername": {beforeUsername}, "beforeId": {beforeID}}
					rec := httptest.NewRecorder()
					e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users?"+params.Encode(), nil))
					if rec.Code != 200 {
						t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
					}
					var got searchUsersResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					want := reference(q, beforeUsername, beforeID)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("page %d mismatch\ngot: %+v\nwant: %+v", page, got, want)
					}
					if !got.HasMore {
						break
					}
					last := got.Items[len(got.Items)-1]
					beforeUsername, beforeID = last.Username, last.ID
				}
			})
		}
		for _, cursor := range []struct {
			username, id string
			count        int
			more         bool
		}{
			{"bulk-079", "bulk-078", 50, true},
			{"bulk-079", "bulk-079", 50, false},
			{"bulk-080", "bulk-080", 49, false},
		} {
			params := url.Values{"q": {"needle"}, "beforeUsername": {cursor.username}, "beforeId": {cursor.id}}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users?"+params.Encode(), nil))
			var got searchUsersResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if rec.Code != 200 || len(got.Items) != cursor.count || got.HasMore != cursor.more || !reflect.DeepEqual(got, reference("needle", cursor.username, cursor.id)) {
				t.Fatalf("boundary cursor %+v: status=%d items=%d more=%v", cursor, rec.Code, len(got.Items), got.HasMore)
			}
		}

		for _, query := range []string{"", "q=x&beforeId=id", "q=x&beforeUsername=name"} {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users?"+query, nil))
			if rec.Code != 400 {
				t.Fatalf("invalid query %q status %d", query, rec.Code)
			}
		}
	}
}
