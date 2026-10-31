package isureal

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestAggregatedRecommendationScores(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region,status)
 SELECT 'u'||lpad(n::text,4,'0'),'u'||n,'User '||n,repeat('ab',32),'asia',CASE WHEN n%17=0 THEN 'deleted' ELSE 'active' END FROM generate_series(0,399) n`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('w','asia','2025-01-01',now(),now()+interval '30 seconds')`)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at)
 SELECT 'p'||lpad(n::text,4,'0'),'u'||lpad((n%200)::text,4,'0'),'w','img'||n,'public','none',now() FROM generate_series(0,299) n`)
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id)
 SELECT 'f'||n||'-'||k,'u'||lpad(n::text,4,'0'),'u'||lpad((200+(n+k)%200)::text,4,'0') FROM generate_series(0,199) n CROSS JOIN generate_series(0,9) k`)
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id)
 SELECT 'cross'||n,'u'||lpad(n::text,4,'0'),'u'||lpad((n+1)::text,4,'0') FROM generate_series(0,198) n`)
	mustSQL(t, db, `INSERT INTO realmojis(id,post_id,reactor_id,image_id,reaction_type,created_at,deleted_at,replaced_at)
 SELECT 'r'||n||'-'||k,'p'||lpad(n::text,4,'0'),'u'||lpad((240+k)::text,4,'0'),'img','happy',now(),
 CASE WHEN k%19=0 THEN now() END,CASE WHEN k%7=0 THEN now() END FROM generate_series(0,299) n CROSS JOIN generate_series(0,59) k`)
	mustSQL(t, db, `INSERT INTO comments(id,post_id,author_id,body,created_at,deleted_at)
 SELECT 'c'||n||'-'||k,'p'||lpad(n::text,4,'0'),'u'||lpad((240+k%80)::text,4,'0'),'comment',now(),
 CASE WHEN k%13=0 THEN now() END FROM generate_series(0,299) n CROSS JOIN generate_series(0,99) k`)
	mustSQL(t, db, `ANALYZE users; ANALYZE posts; ANALYZE friendships; ANALYZE realmojis; ANALYZE comments`)
	var posts []PostWithAuthor
	if err := db.Select(&posts, `SELECT id,author_id,posted_at FROM posts ORDER BY id`); err != nil {
		t.Fatal(err)
	}
	mutual := map[string]int{}
	for i := 0; i < 400; i++ {
		mutual[fmt.Sprintf("u%04d", i)] = i % 13
	}
	friends := map[string]bool{}
	for i := 240; i < 280; i++ {
		friends[fmt.Sprintf("u%04d", i)] = i < 270
	}
	viewers := []viewerContext{
		{},
		{MutualFriends: mutual},
		{Friends: friends},
		{Friends: friends, MutualFriends: mutual},
	}
	for i, viewer := range viewers {
		for _, n := range []int{0, 1, 51, 64, 255, 256, 300} {
			want, err := recommendationScores(t.Context(), db, viewer, posts[:n])
			if err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTxx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := rankedRecommendationScores(t.Context(), tx, viewer, posts[:n])
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("viewer %d / %d posts: aggregated scores differ", i, n)
			}
		}
	}
	// Measure rows transferred by the old two statements, independently of the
	// response's 50-item cap; scoring itself must still cover every candidate.
	var relationshipRows, engagementRows int
	if err := db.Get(&relationshipRows, `SELECT count(*) FROM friendships WHERE user_low_id IN (SELECT author_id FROM posts) OR user_high_id IN (SELECT author_id FROM posts)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&engagementRows, `SELECT count(*) FROM (
 SELECT r.post_id,r.reactor_id AS user_id FROM realmojis r JOIN users u ON u.id=r.reactor_id AND u.status='active'
 WHERE r.replaced_at IS NULL AND r.deleted_at IS NULL
 UNION
 SELECT c.post_id,c.author_id FROM comments c JOIN users u ON u.id=c.author_id AND u.status='active' WHERE c.deleted_at IS NULL
 ) e`); err != nil {
		t.Fatal(err)
	}
	t.Logf("old rows: relationships=%d engagements=%d; aggregate rows=%d", relationshipRows, engagementRows, len(posts))
	measure := func(label string, n int) {
		var totals [2]time.Duration
		for repetition := 0; repetition < 6; repetition++ {
			var results [2]map[string]int64
			for i := 0; i < 2; i++ {
				tx, err := db.BeginTxx(t.Context(), nil)
				if err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				if i == 0 {
					results[i], err = recommendationScores(t.Context(), tx, viewers[3], posts[:n])
				} else {
					results[i], err = rankedRecommendationScores(t.Context(), tx, viewers[3], posts[:n])
				}
				elapsed := time.Since(start)
				rollbackErr := tx.Rollback()
				if err != nil {
					t.Fatal(err)
				}
				if rollbackErr != nil {
					t.Fatal(rollbackErr)
				}
				if repetition > 0 {
					totals[i] += elapsed
				}
			}
			if !reflect.DeepEqual(results[0], results[1]) {
				t.Fatal("scores differ", label, n)
			}
		}
		t.Logf("%s candidates=%d old=%s new=%s (includes SET LOCAL; excludes common BEGIN/ROLLBACK; 5 alternating samples after warmup)", label, n, totals[0]/5, totals[1]/5)
	}
	measure("dense", 300)
	// Current benchmark-created posts have far fewer reactions than the dense
	// history above. Check that distribution separately rather than extrapolate.
	mustSQL(t, db, `TRUNCATE comments,realmojis;
 INSERT INTO comments(id,post_id,author_id,body,created_at) SELECT 's'||n,'p'||lpad(n::text,4,'0'),'u0241','comment',now() FROM generate_series(0,99) n;
 INSERT INTO realmojis(id,post_id,reactor_id,image_id,reaction_type,created_at) SELECT 's'||n,'p'||lpad(n::text,4,'0'),'u0242','img','happy',now() FROM generate_series(100,199) n;
 ANALYZE comments; ANALYZE realmojis`)
	for _, n := range []int{1, 51, 64, 255, 256, 300} {
		measure("sparse", n)
	}
}
