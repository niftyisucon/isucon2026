package isureal

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestMemoryInteractionLimitsMatchOriginal(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,region,status)
 SELECT 'u'||n,'u'||n,repeat('ab',32),'User '||n,'asia',CASE WHEN n=903 THEN 'deleted' ELSE 'active' END FROM generate_series(0,903) n`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('w','asia','2025-01-01','2025-01-01','2025-01-02')`)
	ids := []string{}
	for i, n := range []int{0, 49, 50, 51, 800} {
		id := fmt.Sprintf("p%d", i)
		ids = append(ids, id)
		mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at) VALUES($1,'u0','w','image','public','none','2025-01-01')`, id)
		mustSQL(t, db, `INSERT INTO realmojis(id,post_id,reactor_id,image_id,reaction_type,created_at)
  SELECT $1||'-r'||lpad(n::text,4,'0'),$1,'u'||n,'image-'||n,'happy','2025-01-01'::timestamptz+(n/3)*interval '1 second' FROM generate_series(1,$2::int) n`, id, n)
		mustSQL(t, db, `INSERT INTO comments(id,post_id,author_id,body,created_at)
  SELECT $1||'-c'||lpad(n::text,4,'0'),$1,'u'||n,'body'||n,'2025-01-01'::timestamptz+(n/3)*interval '1 second' FROM generate_series(1,$2::int) n`, id, n)
		// Invalid rows sort before valid rows in each kind. LIMIT must follow filters.
		mustSQL(t, db, `INSERT INTO realmojis(id,post_id,reactor_id,image_id,reaction_type,created_at,deleted_at,replaced_at)
  SELECT $1||'-bad'||n,$1,'u'||n,'bad','happy','2099-01-01',CASE WHEN n=901 THEN now() END,CASE WHEN n=902 THEN now() END FROM generate_series(901,903) n`, id)
		mustSQL(t, db, `INSERT INTO comments(id,post_id,author_id,body,created_at,deleted_at)
  SELECT $1||'-bad'||n,$1,'u'||n,'bad','1990-01-01',CASE WHEN n=901 THEN now() END FROM unnest(ARRAY[901,903]) n`, id)
	}
	oldR, err := selectCurrentRealMojis(t.Context(), db, `AND r.post_id=ANY($1) ORDER BY r.created_at DESC,r.id DESC`, ids)
	if err != nil {
		t.Fatal(err)
	}
	oldC, err := selectCommentsWithAuthor(t.Context(), db, `AND c.post_id=ANY($1) ORDER BY c.created_at,c.id`, ids)
	if err != nil {
		t.Fatal(err)
	}
	newR, err := selectCurrentRealMojis(t.Context(), db, memoryReactionConditions, ids)
	if err != nil {
		t.Fatal(err)
	}
	newC, err := selectCommentsWithAuthor(t.Context(), db, memoryCommentConditions, ids)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	wantR := []RealMojiWithReactor{}
	for _, r := range oldR {
		if counts[r.PostID] < 50 {
			wantR = append(wantR, r)
			counts[r.PostID]++
		}
	}
	counts = map[string]int{}
	wantC := []CommentWithAuthor{}
	for _, c := range oldC {
		if counts[c.PostID] < 50 {
			wantC = append(wantC, c)
			counts[c.PostID]++
		}
	}
	if !reflect.DeepEqual(newR, wantR) || !reflect.DeepEqual(newC, wantC) {
		t.Fatal("bounded rows differ from original per-post truncation")
	}
	items := make([]memoryPostResponse, len(ids))
	for i, id := range ids {
		items[i].ID = id
	}
	if err := loadMemoryInteractions(t.Context(), db, items); err != nil {
		t.Fatal(err)
	}
	for i, item := range items {
		want := min([]int{0, 49, 50, 51, 800}[i], 50)
		if len(item.RealMojis) != want || len(item.Comments) != want {
			t.Fatalf("%s: reactions=%d comments=%d want=%d", item.ID, len(item.RealMojis), len(item.Comments), want)
		}
	}
	if err := loadMemoryInteractions(t.Context(), db, nil); err != nil {
		t.Fatal(err)
	}
	t.Logf("each kind returned rows %d -> %d", len(oldR), len(newR))
	mustSQL(t, db, `ANALYZE realmojis; ANALYZE comments; ANALYZE users`)
	for _, bounded := range []bool{false, true} {
		start := time.Now()
		for range 10 {
			rq, cq := `AND r.post_id=ANY($1) ORDER BY r.created_at DESC,r.id DESC`, `AND c.post_id=ANY($1) ORDER BY c.created_at,c.id`
			if bounded {
				rq, cq = memoryReactionConditions, memoryCommentConditions
			}
			if _, err := selectCurrentRealMojis(t.Context(), db, rq, ids); err != nil {
				t.Fatal(err)
			}
			if _, err := selectCommentsWithAuthor(t.Context(), db, cq, ids); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("bounded=%v query+decode+keys mean=%s", bounded, time.Since(start)/10)
	}
}
