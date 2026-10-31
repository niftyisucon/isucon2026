package isureal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

// Keep the pre-change decision as an oracle: it uses the complete two-hop graph.
func fullGraphCanComment(viewer viewerContext, post Post) bool {
	if !viewer.canView(post) || post.WindowEndedAt != nil {
		return false
	}
	if viewer.ID == post.AuthorID {
		return viewer.HasPostedToday
	}
	return post.Visibility == "public" || viewer.Friends[post.AuthorID]
}

func TestScopedViewerAuthorization(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region)
 SELECT id,id,id,repeat('ab',32),'asia' FROM unnest(ARRAY['a','b','c','d','m','x','y','z']) id`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('w','asia','2025-01-01','2025-01-01','2025-01-02')`)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at)
 SELECT 'p-'||id,id,'w','i-'||id,'friends_of_friends','none','2025-01-01' FROM users`)
	// Cover both edge orientations, two common friends, direct+FoF, self, and
	// unrelated users. No status filtering is added to intermediate friends.
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id) VALUES
 ('1','a','m'),('2','m','z'),('3','a','b'),('4','b','z'),('5','a','z'),('6','y','z'),('7','c','d')`)
	mustSQL(t, db, `UPDATE users SET status='deleted' WHERE id='z'`)
	ctx := t.Context()
	check := func() {
		t.Helper()
		full, fullErr := loadViewerContext(ctx, db, "m")
		var posts []PostWithAuthor
		posts, err := selectPostsWithAuthor(ctx, db, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range posts {
			got, err := canViewPost(ctx, db, "m", p.ID)
			want := fullErr == nil && full.canView(p.Post)
			if err != nil || got != want {
				t.Fatalf("view %s/%s: %v %v want %v", p.ID, p.Visibility, got, err, want)
			}
			comment, err := canComment(ctx, db, "m", p.ID)
			if expected := fullErr == nil && fullGraphCanComment(full, p.Post); err != nil || comment != expected {
				t.Fatalf("comment %s/%s: %v %v want %v", p.ID, p.Visibility, comment, err, expected)
			}
		}
	}
	for _, visibility := range []string{"public", "friends", "friends_of_friends", "unknown"} {
		t.Run(visibility, func(t *testing.T) {
			mustSQL(t, db, `UPDATE posts SET visibility=$1`, visibility)
			check()
			// Changes must be visible on the very next call; no cached ACLs.
			mustSQL(t, db, `INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('bm','m','a'),('by','y','m')`)
			check()
			mustSQL(t, db, `DELETE FROM block_relations`)
			mustSQL(t, db, `UPDATE posts SET deleted_at=now() WHERE author_id='m'`)
			check()
			mustSQL(t, db, `UPDATE posts SET deleted_at=NULL`)
			mustSQL(t, db, `UPDATE posting_windows SET ended_at=now()`)
			check()
			mustSQL(t, db, `UPDATE posting_windows SET ended_at=NULL`)
			mustSQL(t, db, `UPDATE users SET region='europe' WHERE id='m'`)
			check()
			mustSQL(t, db, `UPDATE users SET region='asia',status='deleted' WHERE id='m'`)
			check()
			mustSQL(t, db, `UPDATE users SET status='active' WHERE id='m'`)
		})
	}
	mustSQL(t, db, `UPDATE posts SET visibility='friends_of_friends'`)
	mustSQL(t, db, `DELETE FROM friendships WHERE id IN ('3','4')`)
	check()
	for _, postID := range []string{"p-z", "missing"} {
		if allowed, err := canViewPost(ctx, db, "m", postID); err != nil || allowed {
			t.Fatalf("inactive/missing author: %v %v", allowed, err)
		}
	}
	if _, err := loadViewerContextWithFoF(ctx, db, "missing", false, ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing viewer: %v", err)
	}
}

func TestScopedFeedPagesMatchFullGraph(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region)
 SELECT 'u'||lpad(n::text,3,'0'),'n'||n,'User '||n,repeat('ab',32),'asia' FROM generate_series(0,240) n`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('w','asia','2025-01-01','2025-01-01','2025-01-02')`)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at)
 SELECT 'p'||n,'u'||lpad(n::text,3,'0'),'w','i'||n,
 CASE n%3 WHEN 0 THEN 'public' WHEN 1 THEN 'friends' ELSE 'friends_of_friends' END,
 'none','2025-01-01'::timestamptz+(n/3)*interval '1 second' FROM generate_series(0,240) n`)
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id)
 SELECT 'f'||n,'u000','u'||lpad(n::text,3,'0') FROM generate_series(1,120) n`)
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id)
 SELECT 's'||n,'u001','u'||lpad(n::text,3,'0') FROM generate_series(121,240) n`)
	mustSQL(t, db, `INSERT INTO follows(id,follower_id,followee_id)
 SELECT 'f'||n,'u000','u'||lpad(n::text,3,'0') FROM generate_series(60,240) n`)
	mustSQL(t, db, `INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('a','u000','u002'),('b','u125','u000')`)
	mustSQL(t, db, `UPDATE users SET status='deleted' WHERE id='u010'`)
	mustSQL(t, db, `UPDATE posts SET deleted_at=now() WHERE id='p15'`)
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error { c.Set("session", &Session{UserID: "u000"}); return next(c) }
	})
	e.GET("/feeds/:feed", getFeedHandler(&HandlerContext{DB: db}))
	e.GET("/posts/:postId", getPostHandler(&HandlerContext{DB: db}))
	all, err := selectPostsWithAuthor(t.Context(), db, `ORDER BY p.posted_at DESC,p.id DESC`)
	if err != nil {
		t.Fatal(err)
	}
	full, err := loadViewerContext(t.Context(), db, "u000")
	if err != nil {
		t.Fatal(err)
	}
	following := map[string]bool{}
	for n := 60; n <= 240; n++ {
		following[fmt.Sprintf("u%03d", n)] = true
	}
	for _, feed := range []string{"public", "friends", "following"} {
		var want, got []string
		for _, p := range all {
			if p.AuthorID != full.ID && full.canView(p.Post) && matchesFeed(full, following, p.Post, feed) {
				want = append(want, p.ID)
			}
		}
		path := "/feeds/" + feed
		for page := 0; ; page++ {
			if page > 10 {
				t.Fatal("pagination did not finish")
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
			if rec.Code != 200 {
				t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
			}
			var r getFeedResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
				t.Fatal(err)
			}
			for _, item := range r.Items {
				got = append(got, item.ID)
			}
			if !r.HasMore {
				break
			}
			last := r.Items[len(r.Items)-1]
			q := url.Values{"beforeId": {last.ID}, "beforePostedAt": {last.PostedAt.Format(time.RFC3339Nano)}}
			path = "/feeds/" + feed + "?" + q.Encode()
		}
		if !reflect.DeepEqual(got, want) || len(want) <= 50 {
			t.Fatalf("%s: got %d want %d posts, equal=%v", feed, len(got), len(want), reflect.DeepEqual(got, want))
		}
		t.Logf("%s: all %d posts match full graph across pages", feed, len(got))
	}
	for _, p := range all {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", "/posts/"+p.ID, nil))
		want := 404
		if full.canView(p.Post) {
			want = 200
		}
		if rec.Code != want {
			t.Fatalf("%s: %d want %d", p.ID, rec.Code, want)
		}
	}
}

// Capture the production query without changing its execution, for optional
// local EXPLAIN/transfer measurements with the same bind parameters.
type viewerQueryCapture struct {
	*sqlx.DB
	query string
	args  []any
}

func (db *viewerQueryCapture) QueryRowxContext(ctx context.Context, query string, args ...any) *sqlx.Row {
	db.query, db.args = query, args
	return db.DB.QueryRowxContext(ctx, query, args...)
}

func TestViewerScopeLocalPerformance(t *testing.T) {
	if os.Getenv("ISUREAL_VIEWER_SCOPE_PERF") != "1" {
		t.Skip("optional local performance comparison")
	}
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region)
 SELECT 'u'||lpad(n::text,4,'0'),'n'||n,'User '||n,repeat('ab',32),'asia' FROM generate_series(0,4999) n`)
	// A 5,000-user graph with 25,000 undirected friendships, like the initial
	// table sizes, but synthetic degree distribution and no production data.
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id)
 SELECT n||'-'||d,least('u'||lpad(n::text,4,'0'),'u'||lpad(((n+d)%5000)::text,4,'0')),
 greatest('u'||lpad(n::text,4,'0'),'u'||lpad(((n+d)%5000)::text,4,'0'))
 FROM generate_series(0,4999) n CROSS JOIN generate_series(1,5) d`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('w','asia','2025-01-01','2025-01-01','2025-01-02')`)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at) VALUES('own','u0000','w','i','public','none','2025-01-01')`)
	for _, shape := range []string{"sparse", "dense"} {
		author := "u0010"
		if shape == "dense" {
			author = "u1050"
			mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id)
 SELECT 'v-'||n,'u0000','u'||lpad(n::text,4,'0') FROM generate_series(101,200) n`)
			mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id)
 SELECT 'd-'||n||'-'||m,'u'||lpad(n::text,4,'0'),'u'||lpad(m::text,4,'0')
 FROM generate_series(101,200) n CROSS JOIN generate_series(1001,1100) m`)
		}
		mustSQL(t, db, `ANALYZE users; ANALYZE friendships; ANALYZE block_relations; ANALYZE posts; ANALYZE posting_windows`)
		captures := []*viewerQueryCapture{{DB: db}, {DB: db}, {DB: db}}
		var durations [3][]time.Duration
		for i := 0; i < 61; i++ {
			for j := 0; j < 3; j++ {
				which := (i + j) % 3
				start := time.Now()
				var err error
				switch which {
				case 0:
					_, err = loadViewerContext(t.Context(), captures[which], "u0000")
				case 1:
					_, err = loadViewerContextWithFoF(t.Context(), captures[which], "u0000", false, "")
				case 2:
					_, err = loadPostViewerContext(t.Context(), captures[which], "u0000", Post{AuthorID: author, Visibility: "friends_of_friends"})
				}
				if err != nil {
					t.Fatal(err)
				}
				if i >= 10 {
					durations[which] = append(durations[which], time.Since(start))
				}
			}
		}
		for i, label := range []string{"full", "no-fof", "one-author"} {
			c := captures[i]
			var row struct {
				FriendshipsJSON []byte `db:"friendships_json"`
			}
			if err := db.Unsafe().Get(&row, c.query, c.args...); err != nil {
				t.Fatal(err)
			}
			var edges [][2]string
			if err := json.Unmarshal(row.FriendshipsJSON, &edges); err != nil {
				t.Fatal(err)
			}
			sort.Slice(durations[i], func(a, b int) bool { return durations[i][a] < durations[i][b] })
			t.Logf("%s %s: median=%s, FoF edges=%d, FoF JSON=%d bytes", shape, label, durations[i][len(durations[i])/2], len(edges), len(row.FriendshipsJSON))
			var plan []string
			if err := db.Select(&plan, "EXPLAIN (ANALYZE, BUFFERS) "+c.query, c.args...); err != nil {
				t.Fatal(err)
			}
			if dir := os.Getenv("ISUREAL_VIEWER_SCOPE_PLAN_DIR"); dir != "" {
				data, _ := json.MarshalIndent(plan, "", "  ")
				if err := os.WriteFile(dir+"/"+shape+"-"+label+".json", data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
