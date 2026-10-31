package isureal

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

// Run only against a disposable local DB. Each run owns its schema.
func TestRelationshipTransactionsLocal(t *testing.T) {
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
	name := "relationship_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(`CREATE SCHEMA ` + name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(`DROP SCHEMA ` + name + ` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	params := parsed.Query()
	params.Set("search_path", name)
	params.Set("statement_timeout", "5000")
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
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users(id,username,secret_key_hash,display_name,region,account_type) VALUES ('a','a','x','a','asia','certified_public'),('b','b','x','b','asia','certified_public'),('c','c','x','c','asia','regular'),('deleted','deleted','x','deleted','asia','regular')`)
	exec(`UPDATE users SET status='deleted' WHERE id='deleted'`)
	exec(`UPDATE users SET secret_key_hash=repeat('ab',32)`)
	h := &HandlerContext{DB: db}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("session", &Session{UserID: c.Request().Header.Get("Actor")})
			return next(c)
		}
	})
	e.POST("/comments/:postId", createCommentHandler(h))
	e.POST("/requests", createFriendRequestHandler(h))
	e.POST("/requests/:requestId/:action", transitionFriendRequestHandler(h))
	e.PUT("/follows/:userId", followHandler(h))
	e.DELETE("/follows/:userId", unfollowHandler(h))
	e.PUT("/blocks/:userId", blockHandler(h))
	e.DELETE("/blocks/:userId", unblockHandler(h))
	e.DELETE("/friends/:userId", deleteFriendshipHandler(h))
	call := func(actor, method, path, body string) (int, string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Actor", actor)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	check := func(want int, actor, method, path, body string) string {
		t.Helper()
		code, body := call(actor, method, path, body)
		if code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, code, want, body)
		}
		return body
	}
	count := func(query string, want int) {
		t.Helper()
		var n int
		if err := db.Get(&n, query); err != nil || n != want {
			t.Fatalf("%s: %d want %d: %v", query, n, want, err)
		}
	}
	reset := func() { exec(`TRUNCATE block_relations,follows,friendships,friend_requests`) }
	request := func(actor, target string) createFriendRequestResponse {
		t.Helper()
		body := check(201, actor, "POST", "/requests", fmt.Sprintf(`{"recipientId":%q}`, target))
		var r createFriendRequestResponse
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			t.Fatal(err)
		}
		var stored FriendRequest
		if err := db.Get(&stored, `SELECT * FROM friend_requests WHERE id=$1`, r.RelationID); err != nil {
			t.Fatal(err)
		}
		if r.Status != stored.Status || !r.UpdatedAt.Equal(stored.UpdatedAt) {
			t.Fatal("RETURNING response differs from stored state")
		}
		return r
	}
	parallel := func(fs ...func() int) []int {
		codes := make([]int, len(fs))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, f := range fs {
			wg.Add(1)
			go func(i int, f func() int) { defer wg.Done(); <-start; codes[i] = f() }(i, f)
		}
		close(start)
		wg.Wait()
		return codes
	}
	for _, target := range []string{"a", "", "missing", "deleted"} {
		want := 404
		if target == "a" || target == "" {
			want = 400
		}
		check(want, "a", "POST", "/requests", fmt.Sprintf(`{"recipientId":%q}`, target))
	}
	check(409, "a", "PUT", "/follows/c", "")
	check(204, "a", "PUT", "/follows/b", "")
	check(409, "a", "PUT", "/follows/b", "")
	check(204, "a", "DELETE", "/follows/b", "")
	check(409, "a", "DELETE", "/follows/b", "")
	r := request("a", "b")
	check(409, "a", "POST", "/requests", `{"recipientId":"b"}`)
	reverse := request("b", "a")
	if reverse.RelationID != r.RelationID || reverse.Status != "accepted" {
		t.Fatal("reverse request did not accept existing request")
	}
	count(`SELECT count(*) FROM friendships`, 1)
	check(409, "a", "POST", "/requests", `{"recipientId":"b"}`)
	check(204, "a", "DELETE", "/friends/b", "")
	check(409, "a", "DELETE", "/friends/b", "")
	for _, action := range []string{"accept", "reject", "cancel"} {
		reset()
		r = request("a", "b")
		actor := "b"
		if action == "cancel" {
			actor = "a"
		}
		other := "a"
		if actor == "a" {
			other = "b"
		}
		check(403, other, "POST", "/requests/"+r.RelationID+"/"+action, "")
		body := check(200, actor, "POST", "/requests/"+r.RelationID+"/"+action, "")
		var result transitionFriendRequestResponse
		if err := json.Unmarshal([]byte(body), &result); err != nil {
			t.Fatal(err)
		}
		var stored FriendRequest
		if err := db.Get(&stored, `SELECT * FROM friend_requests WHERE id=$1`, r.RelationID); err != nil {
			t.Fatal(err)
		}
		if result.Status != stored.Status || !result.UpdatedAt.Equal(stored.UpdatedAt) {
			t.Fatal("transition RETURNING differs")
		}
		check(403, actor, "POST", "/requests/"+r.RelationID+"/"+action, "")
	}
	reset()
	r = request("a", "b")
	exec(`INSERT INTO follows(id,follower_id,followee_id) VALUES ('f1','a','b'),('f2','b','a'); INSERT INTO friendships(id,user_low_id,user_high_id) VALUES ('friend','a','b')`)
	check(204, "a", "PUT", "/blocks/b", "")
	check(409, "a", "PUT", "/blocks/b", "")
	count(`SELECT count(*) FROM follows`, 0)
	count(`SELECT count(*) FROM friendships`, 0)
	count(`SELECT count(*) FROM friend_requests WHERE status='cancelled'`, 1)
	check(409, "b", "PUT", "/follows/a", "")
	check(409, "b", "POST", "/requests", `{"recipientId":"a"}`)
	check(403, "b", "DELETE", "/blocks/a", "")
	check(204, "a", "DELETE", "/blocks/b", "")
	check(403, "a", "DELETE", "/blocks/b", "")
	reset()
	exec(`INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES ('window','asia',CURRENT_DATE,clock_timestamp(),clock_timestamp()+interval '1 hour')`)
	exec(`INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at) VALUES ('post-a','a','window','image-a','public','none',clock_timestamp()),('post-b','b','window','image-b','public','none',clock_timestamp())`)
	check(201, "a", "POST", "/comments/post-a", `{"body":"self"}`)
	check(201, "b", "POST", "/comments/post-a", `{"body":"other"}`)
	check(204, "a", "PUT", "/blocks/b", "")
	check(403, "b", "POST", "/comments/post-a", `{"body":"blocked"}`)
	count(`SELECT count(*) FROM comments`, 2)

	for i := 0; i < 16; i++ {
		reset()
		codes := parallel(func() int { c, _ := call("a", "POST", "/requests", `{"recipientId":"b"}`); return c }, func() int { c, _ := call("a", "POST", "/requests", `{"recipientId":"b"}`); return c })
		if !((codes[0] == 201 && codes[1] == 409) || (codes[1] == 201 && codes[0] == 409)) {
			t.Fatalf("duplicate request: %v", codes)
		}
		count(`SELECT count(*) FROM friend_requests WHERE status='pending'`, 1)
		reset()
		codes = parallel(func() int { c, _ := call("a", "POST", "/requests", `{"recipientId":"b"}`); return c }, func() int { c, _ := call("b", "POST", "/requests", `{"recipientId":"a"}`); return c })
		if codes[0] != 201 || codes[1] != 201 {
			t.Fatalf("opposite requests: %v", codes)
		}
		count(`SELECT count(*) FROM friendships`, 1)
		reset()
		r = request("a", "b")
		path := "/requests/" + r.RelationID + "/accept"
		codes = parallel(func() int { c, _ := call("b", "POST", path, ""); return c }, func() int { c, _ := call("b", "POST", path, ""); return c })
		if !((codes[0] == 200 && codes[1] == 403) || (codes[1] == 200 && codes[0] == 403)) {
			t.Fatalf("double accept: %v", codes)
		}
		count(`SELECT count(*) FROM friendships`, 1)
		reset()
		r = request("a", "b")
		path = "/requests/" + r.RelationID + "/accept"
		codes = parallel(func() int { c, _ := call("b", "POST", path, ""); return c }, func() int { c, _ := call("a", "PUT", "/blocks/b", ""); return c })
		if (codes[0] != 200 && codes[0] != 403) || codes[1] != 204 {
			t.Fatalf("accept/block: %v", codes)
		}
		count(`SELECT count(*) FROM friendships`, 0)
		count(`SELECT count(*) FROM friend_requests WHERE status='pending'`, 0)
		reset()
		codes = parallel(func() int { c, _ := call("a", "PUT", "/follows/b", ""); return c }, func() int { c, _ := call("b", "PUT", "/blocks/a", ""); return c })
		if (codes[0] != 204 && codes[0] != 409) || codes[1] != 204 {
			t.Fatalf("follow/block: %v", codes)
		}
		count(`SELECT count(*) FROM follows`, 0)
	}
}
