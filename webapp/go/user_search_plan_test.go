package isureal

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func installSearchIndexes(t *testing.T, db *sqlx.DB) {
	t.Helper()
	migration, err := os.ReadFile("../sql/search-index.sql")
	if err != nil {
		t.Fatal(err)
	}
	mustSQL(t, db, string(migration))
}

func TestSearchCandidatePlans(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,region)
 SELECT 'u'||n, 'user_'||lpad(n::text,5,'0'), repeat('ab',32),
 CASE WHEN n<=100 THEN 'Straße' ELSE 'その他の利用者 '||n END, 'asia'
 FROM generate_series(1,5000) n`)
	if err := prepareUserSearch(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, db, "ANALYZE users; ANALYZE block_relations")
	old := `WITH candidates AS MATERIALIZED (
 SELECT id, username, display_name, search_username, search_display_name FROM users
 WHERE status='active' AND id<>$1 AND ($2='' OR (username,id)>($2,$3))
 AND id NOT IN (SELECT CASE WHEN blocker_id=$1 THEN blocked_id ELSE blocker_id END
   FROM block_relations WHERE blocker_id=$1 OR blocked_id=$1)
), matches AS (
 (SELECT id, username, display_name FROM candidates
  WHERE search_username IS NOT NULL AND search_display_name IS NOT NULL
    AND (strpos(search_username,$4)>0 OR strpos(search_display_name,$4)>0)
  ORDER BY username,id LIMIT 51)
 UNION ALL
 SELECT id, username, display_name FROM candidates
 WHERE search_username IS NULL OR search_display_name IS NULL
)
SELECT id, username, display_name FROM matches ORDER BY username,id`
	inline := strings.Replace(old, "AS MATERIALIZED", "AS NOT MATERIALIZED", 1)
	writeCost := func() time.Duration {
		tx, err := db.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		_, err = tx.Exec(`INSERT INTO users(id,username,secret_key_hash,display_name,region,search_username,search_display_name)
 SELECT 'write'||n,'write_'||n,repeat('ab',32),'Original Name','asia','write_'||n,'original name' FROM generate_series(1,100) n;
 UPDATE users SET display_name='New Name',search_display_name='new name' WHERE id LIKE 'write%'`)
		elapsed := time.Since(start)
		rollbackErr := tx.Rollback()
		if err != nil {
			t.Fatal(err)
		}
		if rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
		return elapsed
	}
	t.Logf("100 insert+update SQL without indexes=%s (single batch, rollback excluded)", writeCost())
	start := time.Now()
	installSearchIndexes(t, db)
	t.Logf("index migration on 5000 users=%s", time.Since(start))
	installSearchIndexes(t, db) // idempotent migration
	t.Logf("100 insert+update SQL with indexes=%s (single batch, rollback excluded)", writeCost())
	var indexBytes int64
	if err := db.Get(&indexBytes, `SELECT pg_relation_size('users_search_trgm')+pg_relation_size('users_search_legacy')`); err != nil {
		t.Fatal(err)
	}
	t.Logf("index size=%d bytes", indexBytes)
	// initialize clears then rebuilds normalized keys on every initial user.
	// Include that index-maintenance workload before measuring lookup costs.
	mustSQL(t, db, `UPDATE users SET search_username=NULL,search_display_name=NULL`)
	start = time.Now()
	if err := prepareUserSearch(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	t.Logf("rebuild keys for 5000 indexed users=%s", time.Since(start))
	mustSQL(t, db, "ANALYZE users")
	for _, query := range []string{"strasse", "no-match", "user", "user_04999", "そ", "%", "🙂"} {
		var results [3][]searchCandidate
		for i, stmt := range []string{old, inline, searchCandidatesSQL} {
			parameter := query
			if i == 2 {
				stmt, parameter = searchCandidatesQuery(query)
			}
			start := time.Now()
			for n := 0; n < 10; n++ {
				results[i] = nil
				if err := db.Select(&results[i], stmt, "viewer", "", "", parameter); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("q=%s variant=%d mean=%s rows=%d", query, i, time.Since(start)/10, len(results[i]))
			if i > 0 && !reflect.DeepEqual(results[0], results[i]) {
				t.Fatal("different candidates", query, i)
			}
			if query == "user_04999" {
				var raw []byte
				if err := db.Get(&raw, "EXPLAIN(ANALYZE,BUFFERS,FORMAT JSON) "+stmt, "viewer", "", "", parameter); err != nil {
					t.Fatal(err)
				}
				var plans []map[string]any
				if err := json.Unmarshal(raw, &plans); err != nil {
					t.Fatal(err)
				}
				t.Logf("variant=%d execution-ms=%v planning-ms=%v", i, plans[0]["Execution Time"], plans[0]["Planning Time"])
				if i == 2 && (!strings.Contains(string(raw), "users_search_trgm") || strings.Contains(string(raw), `"Node Type": "CTE Scan"`)) {
					t.Fatal("selective query did not use the substring index directly")
				}
			}
		}
	}
}
