package isureal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

type friendTraceKey struct{}
type friendRequestTrace struct {
	count        atomic.Uint64
	mu           sync.Mutex
	lockedAt     time.Time
	held         time.Duration
	beforeCommit time.Duration
}

func (q *friendRequestTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	q.count.Add(1)
	if data.SQL == "commit" {
		q.mu.Lock()
		if !q.lockedAt.IsZero() {
			q.beforeCommit = time.Since(q.lockedAt)
		}
		q.mu.Unlock()
	}
	return context.WithValue(ctx, friendTraceKey{}, data.SQL)
}
func (q *friendRequestTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	q.mu.Lock()
	defer q.mu.Unlock()
	query, _ := ctx.Value(friendTraceKey{}).(string)
	if strings.Contains(query, `ORDER BY id COLLATE "C" FOR NO KEY UPDATE`) && data.Err == nil {
		q.lockedAt = time.Now()
	}
	if query == "commit" && !q.lockedAt.IsZero() {
		q.held = time.Since(q.lockedAt)
		q.lockedAt = time.Time{}
	}
}

func friendRoundtripFixture(t *testing.T) (*sqlx.DB, *echo.Echo, *friendRequestTrace, string) {
	t.Helper()
	db, dsn := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region)
 SELECT id,id,id,repeat('ab',32),'asia' FROM unnest(ARRAY['a','b','c','deleted']) id`)
	mustSQL(t, db, `UPDATE users SET status='deleted' WHERE id='deleted'`)
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	app := cfg.RuntimeParams["search_path"] + "_friend_test"
	cfg.RuntimeParams["application_name"] = app
	cfg.RuntimeParams["statement_timeout"] = "5000"
	trace := &friendRequestTrace{}
	cfg.Tracer = trace
	counted := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	t.Cleanup(func() { counted.Close() })
	if err := counted.Ping(); err != nil {
		t.Fatal(err)
	}
	h := &HandlerContext{DB: counted}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("session", &Session{UserID: c.Request().Header.Get("Actor")})
			return next(c)
		}
	})
	e.POST("/old/requests", originalCreateFriendRequestHandler(h))
	e.POST("/new/requests", createFriendRequestHandler(h))
	e.POST("/old/requests/:requestId/:action", originalTransitionFriendRequestHandler(h))
	e.POST("/new/requests/:requestId/:action", transitionFriendRequestHandler(h))
	return db, e, trace, app
}

func friendRequestCall(e *echo.Echo, actor, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Actor", actor)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	return w
}

const pendingFriendFixture = `INSERT INTO friend_requests(id,requester_id,recipient_id,status,created_at,updated_at)
 VALUES('r','a','b','pending',clock_timestamp(),clock_timestamp())`

func TestFriendRequestRoundtripsMatchOriginal(t *testing.T) {
	db, e, trace, _ := friendRoundtripFixture(t)
	for _, tc := range []struct {
		name, setup, actor, path, body string
		code                           int
		oldQueries, newQueries         uint64
	}{
		{"new", "", "a", "", `{"recipientId":"b"}`, 201, 6, 5},
		{"reverse", pendingFriendFixture, "b", "", `{"recipientId":"a"}`, 201, 7, 5},
		{"duplicate", pendingFriendFixture, "a", "", `{"recipientId":"b"}`, 409, 0, 0},
		{"friends", `INSERT INTO friendships(id,user_low_id,user_high_id) VALUES('f','a','b')`, "a", "", `{"recipientId":"b"}`, 409, 0, 0},
		{"self", "", "a", "", `{"recipientId":"a"}`, 400, 0, 0},
		{"empty", "", "a", "", `{"recipientId":""}`, 400, 0, 0},
		{"missing", "", "a", "", `{"recipientId":"missing"}`, 404, 0, 0},
		{"deleted", "", "a", "", `{"recipientId":"deleted"}`, 404, 0, 0},
		{"accept", pendingFriendFixture, "b", "/r/accept", "", 200, 8, 6},
		{"reject", pendingFriendFixture, "b", "/r/reject", "", 200, 6, 6},
		{"cancel", pendingFriendFixture, "a", "/r/cancel", "", 200, 6, 6},
		{"wrong-actor", pendingFriendFixture, "a", "/r/accept", "", 403, 0, 0},
		{"wrong-cancel", pendingFriendFixture, "b", "/r/cancel", "", 403, 0, 0},
		{"wrong-action", pendingFriendFixture, "b", "/r/wrong", "", 403, 0, 0},
		{"missing-request", "", "b", "/r/accept", "", 404, 0, 0},
		{"blocked-create", `INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('bl','a','b')`, "a", "", `{"recipientId":"b"}`, 409, 0, 0},
		{"reverse-blocked-create", `INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('bl','b','a')`, "a", "", `{"recipientId":"b"}`, 409, 0, 0},
		{"blocked-accept", pendingFriendFixture + `; INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('bl','a','b')`, "b", "/r/accept", "", 409, 0, 0},
		{"nonpending-before-block", pendingFriendFixture + `; UPDATE friend_requests SET status='cancelled'; INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('bl','b','a')`, "b", "/r/accept", "", 403, 0, 0},
		{"insert-failure-rolls-back", pendingFriendFixture + `; INSERT INTO friendships(id,user_low_id,user_high_id) VALUES('f','a','b')`, "b", "/r/accept", "", 500, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var previous []string
			for _, version := range []string{"old", "new"} {
				mustSQL(t, db, `TRUNCATE friendships,friend_requests,block_relations`)
				if tc.setup != "" {
					mustSQL(t, db, tc.setup)
				}
				before := trace.count.Load()
				w := friendRequestCall(e, tc.actor, "/"+version+"/requests"+tc.path, tc.body)
				queries := trace.count.Load() - before
				if w.Code != tc.code {
					t.Fatalf("%s: status %d want %d: %s", version, w.Code, tc.code, w.Body.String())
				}
				wantQueries := tc.oldQueries
				if version == "new" {
					wantQueries = tc.newQueries
				}
				if wantQueries > 0 && queries != wantQueries {
					t.Fatalf("%s queries=%d want %d", version, queries, wantQueries)
				}
				var response map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if w.Code < 300 {
					var body createFriendRequestResponse
					if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					stored, err := getFriendRequest(context.Background(), db, body.RelationID)
					if err != nil || stored.Status != body.Status || !stored.UpdatedAt.Equal(body.UpdatedAt) {
						t.Fatal("response differs from committed row", err)
					}
					delete(response, "relationId")
					delete(response, "updatedAt")
				}
				var rows []string
				if err := db.Select(&rows, `SELECT requester_id||':'||recipient_id||':'||status FROM friend_requests
 UNION ALL SELECT user_low_id||':'||user_high_id||':'||coalesce(r.status,'no-source') FROM friendships f LEFT JOIN friend_requests r ON r.id=f.source_request_id ORDER BY 1`); err != nil {
					t.Fatal(err)
				}
				body, _ := json.Marshal(response)
				rows = append(rows, string(body))
				if version == "old" {
					previous = rows
				} else if !reflect.DeepEqual(rows, previous) {
					t.Fatalf("new state/response %v differs from old %v", rows, previous)
				}
				if wantQueries > 0 {
					t.Logf("%s SQL statements including BEGIN/COMMIT=%d", version, queries)
				}
			}
		})
	}
}

func TestFriendRequestReadsStateAfterLockWait(t *testing.T) {
	db, e, _, app := friendRoundtripFixture(t)
	for _, version := range []string{"old", "new"} {
		for _, mutation := range []string{"pending", "block", "deleted", "cancel"} {
			t.Run(version+"/"+mutation, func(t *testing.T) {
				mustSQL(t, db, `TRUNCATE friendships,friend_requests,block_relations; UPDATE users SET status='active' WHERE id IN ('a','b')`)
				if mutation == "cancel" {
					mustSQL(t, db, pendingFriendFixture)
				}
				tx, err := db.Beginx()
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				if _, err := tx.Exec(`SELECT id FROM users WHERE id='a' FOR NO KEY UPDATE`); err != nil {
					t.Fatal(err)
				}
				update, want, actor, path, body := pendingFriendFixture, 409, "a", "", `{"recipientId":"b"}`
				switch mutation {
				case "block":
					update = `INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('bl','b','a')`
				case "deleted":
					update = `UPDATE users SET status='deleted' WHERE id='a'`
					want = 404
				case "cancel":
					update = `UPDATE friend_requests SET status='cancelled'`
					want = 403
					actor = "b"
					path = "/r/accept"
					body = ""
				}
				if _, err := tx.Exec(update); err != nil {
					t.Fatal(err)
				}
				done := make(chan *httptest.ResponseRecorder, 1)
				go func() { done <- friendRequestCall(e, actor, "/"+version+"/requests"+path, body) }()
				waiting := false
				for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
					if err := db.Get(&waiting, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')`, app); err != nil {
						t.Fatal(err)
					}
					if waiting {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
				w := <-done
				if !waiting {
					t.Fatal("did not observe real row-lock wait")
				}
				if w.Code != want {
					t.Fatalf("after lock wait: %d want %d: %s", w.Code, want, w.Body.String())
				}
			})
		}
	}
}

func TestFriendRequestLocalTiming(t *testing.T) {
	if os.Getenv("ISUREAL_FRIEND_REQUEST_PERF") != "1" {
		t.Skip("optional local timing")
	}
	db, e, trace, _ := friendRoundtripFixture(t)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region)
 SELECT 'peer-'||n,'peer-'||n,'peer',repeat('ab',32),'asia' FROM generate_series(1,5000) n;
 INSERT INTO friend_requests(id,requester_id,recipient_id,status,created_at,updated_at)
 SELECT 'background-'||n||'-'||k,'peer-'||n,'peer-'||((n+k-1)%5000+1),'pending',clock_timestamp(),clock_timestamp()
 FROM generate_series(1,5000) n CROSS JOIN generate_series(1,5) k;
 ANALYZE users; ANALYZE friend_requests; ANALYZE friendships; ANALYZE block_relations`)
	for _, operation := range []string{"new", "reverse", "accept"} {
		samples := map[string][]float64{"old": {}, "new": {}}
		precommit := map[string][]float64{"old": {}, "new": {}}
		for i := 0; i < 61; i++ {
			versions := []string{"old", "new"}
			if i%2 == 1 {
				versions = []string{"new", "old"}
			}
			for _, version := range versions {
				// Keep table identities/plans and 25k background requests stable.
				mustSQL(t, db, `DELETE FROM friendships WHERE user_low_id='a'; DELETE FROM friend_requests WHERE requester_id IN ('a','b')`)
				actor, path, body, want := "a", "", `{"recipientId":"b"}`, 201
				if operation != "new" {
					mustSQL(t, db, pendingFriendFixture)
					actor = "b"
					body = `{"recipientId":"a"}`
				}
				if operation == "accept" {
					path = "/r/accept"
					body = ""
					want = 200
				}
				w := friendRequestCall(e, actor, "/"+version+"/requests"+path, body)
				if w.Code != want {
					t.Fatal(w.Code, w.Body.String())
				}
				trace.mu.Lock()
				held := trace.held
				beforeCommit := trace.beforeCommit
				trace.mu.Unlock()
				if held <= 0 {
					t.Fatal("no committed lock duration")
				}
				if i >= 10 {
					samples[version] = append(samples[version], float64(held)/float64(time.Millisecond))
					precommit[version] = append(precommit[version], float64(beforeCommit)/float64(time.Millisecond))
				}
			}
		}
		for _, version := range []string{"old", "new"} {
			sort.Float64s(samples[version])
			sort.Float64s(precommit[version])
			t.Log(fmt.Sprintf("%s %s local lock-to-commit-end median %.6f ms; lock-to-commit-start %.6f ms", operation, version, samples[version][len(samples[version])/2], precommit[version][len(precommit[version])/2]))
		}
	}
}
