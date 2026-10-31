package isureal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

func windowLockApp(db *sqlx.DB) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	h := &HandlerContext{DB: db}
	e.POST("/windows", openPostingWindowHandler(h))
	e.POST("/register", registerHandler(h))
	auth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("session", &Session{UserID: c.Request().Header.Get("Actor")})
			return next(c)
		}
	}
	e.PATCH("/me", updateMeHandler(h), auth)
	e.DELETE("/me", deleteAccountHandler(h), auth)
	return e
}

func windowLockRequest(t *testing.T, e *echo.Echo, method, path, actor, body string) <-chan *httptest.ResponseRecorder {
	t.Helper()
	done := make(chan *httptest.ResponseRecorder, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	go func() {
		defer cancel()
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Actor", actor)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		done <- rec
	}()
	return done
}

func windowLockResult(t *testing.T, result <-chan *httptest.ResponseRecorder, status int) *httptest.ResponseRecorder {
	t.Helper()
	select {
	case rec := <-result:
		if rec.Code != status {
			t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body.String())
		}
		return rec
	case <-time.After(3 * time.Second):
		t.Fatal("request did not progress")
		return nil
	}
}

func waitPostingLock(t *testing.T, db *sqlx.DB, key, count int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := db.Get(&n, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND classid=20261031 AND objid=$1 AND NOT granted`, key); err != nil {
			t.Fatal(err)
		}
		if n >= count {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no %d waiters for advisory lock %d", count, key)
}

func postingGate(t *testing.T, db *sqlx.DB) *sqlx.Tx {
	t.Helper()
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(20261031,7299)`); err != nil {
		t.Fatal(err)
	}
	return tx
}

func checkWindowRecipients(t *testing.T, db *sqlx.DB, response *httptest.ResponseRecorder, expected string) openPostingWindowResponse {
	t.Helper()
	var window openPostingWindowResponse
	if err := json.Unmarshal(response.Body.Bytes(), &window); err != nil {
		t.Fatal(err)
	}
	var users []string
	if err := db.Select(&users, `SELECT user_id FROM notifications WHERE posting_window_id=$1 ORDER BY user_id`, window.ID); err != nil {
		t.Fatal(err)
	}
	if strings.Join(users, ",") != expected {
		t.Fatalf("recipients %v, want %s", users, expected)
	}
	var wrongTimes int
	if err := db.Get(&wrongTimes, `SELECT count(*) FROM notifications WHERE posting_window_id=$1 AND created_at<>$2`, window.ID, window.StartsAt); err != nil || wrongTimes != 0 {
		t.Fatalf("notification times: %d %v", wrongTimes, err)
	}
	if !window.OnTimeDeadline.Equal(window.StartsAt.Add(30 * time.Second)) {
		t.Fatalf("deadline differs: %+v", window)
	}
	return window
}

func TestPostingWindowRegionLock(t *testing.T) {
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "false")
	db, _ := architectureDB(t)
	e := windowLockApp(db)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region,status) VALUES
 ('stay','stay','Stay',repeat('ab',32),'asia','active'),
 ('leaving','leaving','Leaving',repeat('ab',32),'asia','active'),
 ('moving','moving','Moving',repeat('ab',32),'europe','active'),
 ('inactive','inactive','Inactive',repeat('ab',32),'asia','deleted')`)
	// Pause the actual window handler after it acquired its region lock. This
	// keeps registration/membership changes ordered around starts_at/recipients.
	mustSQL(t, db, `CREATE FUNCTION pause_asia_window() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.region='asia' THEN PERFORM pg_advisory_xact_lock(20261031,7299); END IF; RETURN NEW; END $$;
 CREATE TRIGGER pause_asia_window BEFORE INSERT ON posting_windows FOR EACH ROW EXECUTE FUNCTION pause_asia_window()`)
	gate := postingGate(t, db)
	defer gate.Rollback()
	window := windowLockRequest(t, e, "POST", "/windows", "", `{"region":"asia","localDate":"2025-01-01","onTimeSeconds":30}`)
	waitPostingLock(t, db, 7299, 1)
	register := windowLockRequest(t, e, "POST", "/register", "", `{"username":"later","displayName":"Later","region":"asia"}`)
	move := windowLockRequest(t, e, "PATCH", "/me", "moving", `{"displayName":"Moved","region":"asia"}`)
	remove := windowLockRequest(t, e, "DELETE", "/me", "leaving", "")
	duplicate := windowLockRequest(t, e, "POST", "/windows", "", `{"region":"asia","localDate":"2025-01-01","onTimeSeconds":30}`)
	waitPostingLock(t, db, 7201, 4)
	// These must finish while asia's window remains paused. The old users table
	// lock blocked both the profile update and registration in another region.
	windowLockResult(t, windowLockRequest(t, e, "PATCH", "/me", "stay", `{"displayName":"Changed","bio":"still independent"}`), 200)
	windowLockResult(t, windowLockRequest(t, e, "POST", "/register", "", `{"username":"other","displayName":"Other","region":"america"}`), 201)
	checkWindowRecipients(t, db, windowLockResult(t, windowLockRequest(t, e, "POST", "/windows", "", `{"region":"europe","localDate":"2025-01-01","onTimeSeconds":30}`), 201), "moving")
	for _, pending := range []<-chan *httptest.ResponseRecorder{window, register, move, remove, duplicate} {
		select {
		case rec := <-pending:
			t.Fatalf("request escaped membership barrier: %d %s", rec.Code, rec.Body.String())
		default:
		}
	}
	if err := gate.Commit(); err != nil {
		t.Fatal(err)
	}
	first := checkWindowRecipients(t, db, windowLockResult(t, window, 201), "leaving,stay")
	var registered registerResponse
	if err := json.Unmarshal(windowLockResult(t, register, 201).Body.Bytes(), &registered); err != nil {
		t.Fatal(err)
	}
	windowLockResult(t, move, 200)
	windowLockResult(t, remove, 204)
	windowLockResult(t, duplicate, 409)
	expected := []string{registered.User.ID, "moving", "stay"}
	// UUIDs precede these fixture names lexicographically.
	second := checkWindowRecipients(t, db, windowLockResult(t, windowLockRequest(t, e, "POST", "/windows", "", `{"region":"asia","localDate":"2025-01-02","onTimeSeconds":30}`), 201), strings.Join(expected, ","))
	var ended time.Time
	if err := db.Get(&ended, `SELECT ended_at FROM posting_windows WHERE id=$1`, first.ID); err != nil || !ended.Equal(second.StartsAt) {
		t.Fatalf("previous window ended_at: %v %v", ended, err)
	}
}

func TestPostingMembershipBeforeWindow(t *testing.T) {
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "false")
	for _, operation := range []string{"register", "move", "delete"} {
		t.Run(operation, func(t *testing.T) {
			db, _ := architectureDB(t)
			e := windowLockApp(db)
			region := "europe"
			if operation == "delete" {
				region = "asia"
			}
			mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region) VALUES('actor','actor','Actor',repeat('ab',32),$1)`, region)
			mustSQL(t, db, `CREATE FUNCTION pause_membership() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN PERFORM pg_advisory_xact_lock(20261031,7299); RETURN NEW; END $$;
 CREATE TRIGGER pause_membership BEFORE INSERT OR UPDATE ON users FOR EACH ROW EXECUTE FUNCTION pause_membership()`)
			gate := postingGate(t, db)
			defer gate.Rollback()
			method, path, body, status := "POST", "/register", `{"username":"early","displayName":"Early","region":"asia"}`, 201
			if operation == "move" {
				method, path, body, status = "PATCH", "/me", `{"region":"asia"}`, 200
			} else if operation == "delete" {
				method, path, body, status = "DELETE", "/me", "", 204
			}
			writer := windowLockRequest(t, e, method, path, "actor", body)
			waitPostingLock(t, db, 7299, 1)
			window := windowLockRequest(t, e, "POST", "/windows", "", `{"region":"asia","localDate":"2025-01-01","onTimeSeconds":30}`)
			waitPostingLock(t, db, 7201, 1)
			if err := gate.Commit(); err != nil {
				t.Fatal(err)
			}
			written := windowLockResult(t, writer, status)
			expected := ""
			if operation == "move" {
				expected = "actor"
			} else if operation == "register" {
				var user registerResponse
				if err := json.Unmarshal(written.Body.Bytes(), &user); err != nil {
					t.Fatal(err)
				}
				expected = user.User.ID
			}
			checkWindowRecipients(t, db, windowLockResult(t, window, 201), expected)
		})
	}
}

func TestPostingWindowConcurrentStarts(t *testing.T) {
	db, _ := architectureDB(t)
	e := windowLockApp(db)
	for day := 1; day <= 8; day++ {
		var results []<-chan *httptest.ResponseRecorder
		for range 3 {
			results = append(results, windowLockRequest(t, e, "POST", "/windows", "", fmt.Sprintf(`{"region":"asia","localDate":"2025-02-%02d"}`, day)))
		}
		counts := map[int]int{}
		for _, result := range results {
			select {
			case rec := <-result:
				counts[rec.Code]++
			case <-time.After(3 * time.Second):
				t.Fatal("concurrent starts stuck")
			}
		}
		if counts[201] != 1 || counts[409] != 2 {
			t.Fatal(counts)
		}
	}
}

func TestPostingWindowWaitsForInFlightPost(t *testing.T) {
	t.Setenv("ISUREAL_DERIVED_NOTIFICATIONS", "false")
	db, _ := architectureDB(t)
	e := windowLockApp(db)
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region) VALUES('actor','actor','Actor',repeat('ab',32),'asia')`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('old','asia','2025-01-01',now(),now()+interval '30 seconds')`)
	post, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer post.Rollback()
	// The actual post handler locks the user first, then the current window.
	var id string
	if err := post.Get(&id, `SELECT id FROM users WHERE id='actor' FOR NO KEY UPDATE`); err != nil {
		t.Fatal(err)
	}
	if err := post.Get(&id, `SELECT id FROM posting_windows WHERE id='old' FOR SHARE`); err != nil {
		t.Fatal(err)
	}
	var pid int
	if err := post.Get(&pid, `SELECT pg_backend_pid()`); err != nil {
		t.Fatal(err)
	}
	window := windowLockRequest(t, e, "POST", "/windows", "", `{"region":"asia","localDate":"2025-01-02","onTimeSeconds":30}`)
	blocked := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := db.Get(&blocked, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("window did not wait for in-flight post")
	}
	move := windowLockRequest(t, e, "PATCH", "/me", "actor", `{"region":"europe"}`)
	waitPostingLock(t, db, 7201, 1)
	if _, err := post.Exec(`INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at) VALUES('p','actor','old','image','public','none',now())`); err != nil {
		t.Fatal(err)
	}
	if err := post.Commit(); err != nil {
		t.Fatal(err)
	}
	checkWindowRecipients(t, db, windowLockResult(t, window, 201), "actor")
	windowLockResult(t, move, 200)
}
