package isureal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

// Freeze the previous handlers as independent response and query-count oracles.
func originalMyCurrentPostHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ctx := c.Request().Context()
		userID := GetSession(c).UserID
		var postID string
		err := h.DB.GetContext(ctx, &postID, `
   SELECT p.id FROM posts p
   JOIN posting_windows w ON w.id = p.posting_window_id
   JOIN users u ON u.id = p.author_id
   JOIN posting_windows current ON current.region = u.region AND current.ended_at IS NULL
                              AND current.local_date = w.local_date
   WHERE p.author_id = $1 AND p.deleted_at IS NULL
  `, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "post_not_found")
		}
		if err != nil {
			return err
		}
		post, err := getPost(ctx, h.DB, postID)
		if errors.Is(err, sql.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "post_not_found")
		}
		if err != nil {
			return err
		}
		user, err := getUser(ctx, h.DB, userID)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, getMyCurrentPostResponse(newPostResponse(post, user)))
	}
}

func originalUserReadHandler(h *HandlerContext) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input getUserRequest
		if err := c.Bind(&input); err != nil {
			return err
		}
		ctx := c.Request().Context()
		viewer := GetSession(c).UserID
		user, err := getUser(ctx, h.DB, input.UserID)
		if errors.Is(err, sql.ErrNoRows) {
			return NewAPIError(http.StatusNotFound, "relationship_not_found")
		}
		if err != nil {
			return err
		}
		blocked, err := isBlocked(ctx, h.DB, viewer, user.ID)
		if err != nil {
			return err
		}
		if blocked {
			return NewAPIError(http.StatusNotFound, "relationship_not_found")
		}
		friends, err := getFriendIDs(ctx, h.DB, viewer)
		if err != nil {
			return err
		}
		if user.AccountType != "certified_public" && !slices.Contains(friends, user.ID) {
			user.Bio = ""
		}
		return c.JSON(http.StatusOK, getUserResponse(newRelationshipUserResponse(user)))
	}
}

type readQueryCounter struct{ count atomic.Uint64 }

func (q *readQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.count.Add(1)
	return ctx
}
func (q *readQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func singleReadFixture(t *testing.T) (*sqlx.DB, *echo.Echo, *readQueryCounter) {
	t.Helper()
	db, dsn := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,bio,profile_image_id,region)
 SELECT id,id,'Name '||id,repeat('ab',32),'Bio '||id,'profile-'||id,'asia'
 FROM unnest(ARRAY['a','m','z','empty','deleted']) id`)
	mustSQL(t, db, `UPDATE users SET status='deleted' WHERE id='deleted'`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline,ended_at) VALUES
 ('old','asia','2024-12-31','2024-12-31','2025-01-01','2025-01-01'),
 ('asia','asia','2025-01-01','2025-01-01','2025-01-01 01:00',NULL),
 ('europe','europe','2025-01-01','2025-01-01','2025-01-01 02:00',NULL),
 ('america','america','2025-01-02','2025-01-02','2025-01-02 01:00',NULL)`)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,caption,visibility,location_kind,latitude,longitude,posted_at) VALUES
 ('old','m','old','old-image','old','public','none',NULL,NULL,'2024-12-31'),
 ('current','m','asia','current-image','現在の投稿','friends_of_friends','exact',35.5,139.25,'2025-01-01 02:00'),
 ('peer','a','asia','peer-image','peer','friends','none',NULL,NULL,'2025-01-01')`)
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	counter := &readQueryCounter{}
	cfg.Tracer = counter
	counted := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	configureDatabasePool(counted.DB)
	t.Cleanup(func() { counted.Close() })
	h := &HandlerContext{DB: counted}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("session", &Session{UserID: c.Request().Header.Get("Actor")})
			return next(c)
		}
	})
	e.GET("/old/current", originalMyCurrentPostHandler(h))
	e.GET("/new/current", getMyCurrentPostHandler(h))
	e.GET("/old/users/:userId", originalUserReadHandler(h))
	e.GET("/new/users/:userId", getUserHandler(h))
	return db, e, counter
}

func singleReadCall(e *echo.Echo, actor, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Actor", actor)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func normalizeReadResponse(t *testing.T, data []byte) any {
	t.Helper()
	var body any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	var walk func(any)
	walk = func(v any) {
		if m, ok := v.(map[string]any); ok {
			for k, v := range m {
				if (k == "imageUrl" || k == "profileImageUrl") && v != nil {
					u, err := url.Parse(v.(string))
					if err != nil || u.Query().Get("exp") == "" || u.Query().Get("sig") == "" {
						t.Fatal("invalid signed image URL", v)
					}
					// Calls can straddle an expiry bucket; compare the image identity.
					m[k] = u.Path
				} else {
					walk(v)
				}
			}
		}
	}
	walk(body)
	return body
}

func compareSingleRead(t *testing.T, e *echo.Echo, counter *readQueryCounter, actor, path string, status int) {
	t.Helper()
	counter.count.Store(0)
	old := singleReadCall(e, actor, "/old/"+path)
	oldQueries := counter.count.Swap(0)
	got := singleReadCall(e, actor, "/new/"+path)
	newQueries := counter.count.Load()
	if old.Code != status || got.Code != old.Code || !reflect.DeepEqual(normalizeReadResponse(t, old.Body.Bytes()), normalizeReadResponse(t, got.Body.Bytes())) {
		t.Fatalf("actor=%s path=%s: old %d %s new %d %s want status %d", actor, path, old.Code, old.Body, got.Code, got.Body, status)
	}
	if newQueries != 1 || (status == 200 && oldQueries != 3) {
		t.Fatalf("actor=%s path=%s: query counts old=%d new=%d", actor, path, oldQueries, newQueries)
	}
}

func TestSingleQueryCurrentPost(t *testing.T) {
	db, e, counter := singleReadFixture(t)
	check := func(actor string, code int) { t.Helper(); compareSingleRead(t, e, counter, actor, "current", code) }
	check("m", 200)
	check("empty", 404)
	check("missing", 404)
	// Same local date after a region move must still return the Asia post,
	// even if the original posting window has ended.
	mustSQL(t, db, `UPDATE users SET region='europe',bio='Updated',display_name='Updated' WHERE id='m'`)
	mustSQL(t, db, `UPDATE posting_windows SET ended_at=now() WHERE id='asia'`)
	check("m", 200)
	mustSQL(t, db, `UPDATE users SET region='america' WHERE id='m'`)
	check("m", 404)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,caption,visibility,location_kind,locality,posted_at)
 VALUES('america','m','america','america-image','new','public','approximate','Tokyo','2025-01-02')`)
	check("m", 200)
	mustSQL(t, db, `UPDATE posts SET deleted_at=now() WHERE id='america'`)
	check("m", 404)
	mustSQL(t, db, `UPDATE posts SET deleted_at=NULL WHERE id='america'`)
	check("m", 200)
	mustSQL(t, db, `UPDATE users SET profile_image_id=NULL WHERE id='m'`)
	check("m", 200)
	// Middleware ordinarily rejects inactive users; preserve the existing
	// error if deletion occurs between session validation and this handler.
	mustSQL(t, db, `UPDATE users SET status='deleted' WHERE id='m'`)
	check("m", 500)
	mustSQL(t, db, `UPDATE users SET status='active',secret_key_hash='invalid' WHERE id='m'`)
	check("m", 500)
	mustSQL(t, db, `UPDATE users SET secret_key_hash=repeat('ab',32) WHERE id='m'`)
	mustSQL(t, db, `UPDATE posting_windows SET ended_at=now() WHERE id='america'`)
	check("m", 404)
}

func TestSingleQueryUserProfile(t *testing.T) {
	db, e, counter := singleReadFixture(t)
	check := func(target string, status int) {
		t.Helper()
		compareSingleRead(t, e, counter, "m", "users/"+target, status)
	}
	for _, target := range []string{"a", "m", "z"} {
		for _, account := range []string{"regular", "certified_public"} {
			mustSQL(t, db, `UPDATE users SET account_type=$2 WHERE id=$1`, target, account)
			check(target, 200)
			if target != "m" {
				mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id) VALUES('f',$1,$2)`, min("m", target), max("m", target))
				check(target, 200)
				mustSQL(t, db, `DELETE FROM friendships`)
				check(target, 200)
				for _, pair := range [][2]string{{"m", target}, {target, "m"}} {
					mustSQL(t, db, `INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('b',$1,$2)`, pair[0], pair[1])
					check(target, 404)
					mustSQL(t, db, `DELETE FROM block_relations`)
					check(target, 200)
				}
			}
		}
	}
	check("deleted", 404)
	check("missing", 404)
	mustSQL(t, db, `UPDATE users SET profile_image_id=NULL,bio='changed',display_name='changed' WHERE id='a'`)
	check("a", 200)
	mustSQL(t, db, `UPDATE users SET secret_key_hash='invalid' WHERE id='a'`)
	check("a", 500)
}

func TestSingleQueryReadsLocalPerformance(t *testing.T) {
	if os.Getenv("ISUREAL_SINGLE_READ_PERF") != "1" {
		t.Skip("optional local comparison")
	}
	db, e, counter := singleReadFixture(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region)
 SELECT 'x'||n,'x'||n,'User '||n,repeat('ab',32),'asia' FROM generate_series(1,5000) n`)
	mustSQL(t, db, `INSERT INTO friendships(id,user_low_id,user_high_id)
 SELECT 'f'||n,'m','x'||n FROM generate_series(1,200) n`)
	mustSQL(t, db, `ANALYZE users; ANALYZE friendships; ANALYZE block_relations; ANALYZE posts; ANALYZE posting_windows`)
	for _, path := range []string{"current", "users/a"} {
		var samples [2][]time.Duration
		var queries [2]uint64
		for n := 0; n < 61; n++ {
			for j := 0; j < 2; j++ {
				i := (n + j) % 2
				counter.count.Store(0)
				start := time.Now()
				rec := singleReadCall(e, "m", "/"+[]string{"old", "new"}[i]+"/"+path)
				elapsed := time.Since(start)
				if rec.Code != 200 {
					t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
				}
				if n >= 10 {
					samples[i] = append(samples[i], elapsed)
					queries[i] += counter.count.Load()
				}
			}
		}
		for i := range samples {
			sort.Slice(samples[i], func(a, b int) bool { return samples[i][a] < samples[i][b] })
		}
		t.Logf("%s: median old=%s new=%s, queries old=%d new=%d (%d calls)", path, samples[0][25], samples[1][25], queries[0], queries[1], len(samples[0]))
	}
}
