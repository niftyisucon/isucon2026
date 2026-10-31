package isureal

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

func TestSessionCacheEnabledOnlyForSharedCore(t *testing.T) {
	cases := []struct {
		name   string
		config Config
		env    string
		want   bool
	}{
		{"shared core", Config{ListenRegions: "asia:8080,europe:8081,america:8082"}, "", true},
		{"explicit off", Config{ListenRegions: "asia:8080,europe:8081,america:8082"}, "false", false},
		{"single region process", Config{ListenRegions: "asia:8080"}, "", false},
		{"default process", Config{}, "", false},
		{"edge", Config{ListenRegions: "europe:8080", EdgeUpstream: "http://core"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISUREAL_SESSION_CACHE", tc.env)
			if got := sessionCacheEnabled(&tc.config); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestNilSessionCacheIsDisabled(t *testing.T) {
	var c *sessionCache
	c.put(Session{UserID: "u", TokenHash: "t"}, c.start())
	c.invalidateUser("u")
	c.reset()
	if c.get("t") != nil {
		t.Fatal("nil cache returned a session")
	}
}

func TestSessionCacheInvalidation(t *testing.T) {
	c := newSessionCache()
	c.put(Session{UserID: "u", TokenHash: "a", Region: "asia"}, c.start())
	c.put(Session{UserID: "v", TokenHash: "b"}, c.start())
	got := c.get("a")
	if got == nil || got.UserID != "u" || got.Region != "asia" {
		t.Fatalf("hit %+v", got)
	}
	// Middleware callers receive a private copy.
	got.Region = "europe"
	if c.get("a").Region != "asia" {
		t.Fatal("cached session was mutated through a returned pointer")
	}
	c.invalidateUser("u")
	if c.get("a") != nil {
		t.Fatal("invalidated session is still cached")
	}
	if c.get("b") == nil {
		t.Fatal("other users must stay cached")
	}
	c.reset()
	if c.get("b") != nil {
		t.Fatal("reset left a session")
	}
}

func TestSessionCacheRejectsLookupsOlderThanInvalidation(t *testing.T) {
	c := newSessionCache()
	started := c.start()
	c.invalidateUser("u")
	c.put(Session{UserID: "u", TokenHash: "stale"}, started)
	if c.get("stale") != nil {
		t.Fatal("a lookup that began before the invalidation was stored")
	}
	// Unrelated invalidations must not throw away this user's lookup.
	started = c.start()
	c.invalidateUser("v")
	c.put(Session{UserID: "u", TokenHash: "fresh"}, started)
	if c.get("fresh") == nil {
		t.Fatal("another user's invalidation dropped this lookup")
	}
	started = c.start()
	c.reset()
	c.put(Session{UserID: "u", TokenHash: "pre-reset"}, started)
	if c.get("pre-reset") != nil {
		t.Fatal("a lookup that began before reset was stored")
	}
	c.put(Session{UserID: "u", TokenHash: "post-reset"}, c.start())
	if c.get("post-reset") == nil {
		t.Fatal("a lookup after reset was rejected")
	}
}

func TestSessionCacheHandlersLocal(t *testing.T) {
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
	name := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	sum := sha256.Sum256([]byte("secret"))
	if _, err = db.Exec(`INSERT INTO users(id,username,secret_key_hash,display_name,region) VALUES ('user','user',$1,'User','asia')`, fmt.Sprintf("%x", sum)); err != nil {
		t.Fatal(err)
	}

	h := &HandlerContext{DB: db, Config: &Config{Region: "asia"}, Sessions: newSessionCache()}
	auth := SessionMiddleware(h, true)
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.POST("/login", loginHandler(h))
	e.POST("/logout", logoutHandler(h), auth)
	e.PATCH("/me", updateMeHandler(h), auth)
	e.POST("/delete", deleteAccountHandler(h), auth)
	e.GET("/probe", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) }, auth, RegionMiddleware(h))

	send := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	login := func() string {
		t.Helper()
		rec := send(http.MethodPost, "/login", "", `{"username":"user","secretKey":"secret"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
		}
		var result struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Token
	}
	expect := func(token string, want int) {
		t.Helper()
		if rec := send(http.MethodGet, "/probe", token, ""); rec.Code != want {
			t.Fatalf("probe status %d, want %d: %s", rec.Code, want, rec.Body.String())
		}
	}

	// The second request proves the hit path does not read PostgreSQL.
	token := login()
	expect(token, http.StatusNoContent)
	if _, err = db.Exec(`UPDATE login_sessions SET active=false WHERE user_id='user'`); err != nil {
		t.Fatal(err)
	}
	expect(token, http.StatusNoContent)
	if _, err = db.Exec(`UPDATE login_sessions SET active=true WHERE user_id='user'`); err != nil {
		t.Fatal(err)
	}

	if rec := send(http.MethodPost, "/logout", token, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("logout status %d", rec.Code)
	}
	expect(token, http.StatusUnauthorized)

	// A login replaces the previous token immediately.
	old := login()
	expect(old, http.StatusNoContent)
	current := login()
	expect(old, http.StatusUnauthorized)
	expect(current, http.StatusNoContent)

	// A region move must reach RegionMiddleware on the very next request.
	if rec := send(http.MethodPatch, "/me", current, `{"region":"europe"}`); rec.Code != http.StatusOK {
		t.Fatalf("region move status %d: %s", rec.Code, rec.Body.String())
	}
	expect(current, http.StatusForbidden)
	if rec := send(http.MethodPatch, "/me", current, `{"region":"asia"}`); rec.Code != http.StatusOK {
		t.Fatalf("region return status %d: %s", rec.Code, rec.Body.String())
	}
	expect(current, http.StatusNoContent)

	// A restart starts empty and rebuilds from PostgreSQL.
	h.Sessions = newSessionCache()
	expect(current, http.StatusNoContent)
	expect(current, http.StatusNoContent)

	// A profile edit changes nothing the session holds and keeps it cached.
	if rec := send(http.MethodPatch, "/me", current, `{"bio":"hello"}`); rec.Code != http.StatusOK {
		t.Fatalf("bio status %d: %s", rec.Code, rec.Body.String())
	}
	before := sessionCacheHits.Load()
	expect(current, http.StatusNoContent)
	if sessionCacheHits.Load() == before {
		t.Fatal("a profile edit dropped the session")
	}

	// Once logout has returned, no reader may succeed, even while others race it.
	for i := 0; i < 30; i++ {
		token := login()
		var loggedOut atomic.Bool
		var wg sync.WaitGroup
		errs := make(chan string, 8)
		for j := 0; j < 8; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < 20; k++ {
					done := loggedOut.Load()
					code := send(http.MethodGet, "/probe", token, "").Code
					if done && code != http.StatusUnauthorized {
						errs <- fmt.Sprintf("status %d after logout", code)
						return
					}
				}
			}()
		}
		if rec := send(http.MethodPost, "/logout", token, ""); rec.Code != http.StatusNoContent {
			t.Fatalf("logout status %d", rec.Code)
		}
		loggedOut.Store(true)
		wg.Wait()
		close(errs)
		for message := range errs {
			t.Fatal(message)
		}
		expect(token, http.StatusUnauthorized)
	}

	// initialize truncates login_sessions and then resets the cache.
	token = login()
	expect(token, http.StatusNoContent)
	if _, err = db.Exec(`TRUNCATE login_sessions`); err != nil {
		t.Fatal(err)
	}
	h.Sessions.reset()
	expect(token, http.StatusUnauthorized)

	token = login()
	expect(token, http.StatusNoContent)
	if rec := send(http.MethodPost, "/delete", token, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status %d: %s", rec.Code, rec.Body.String())
	}
	expect(token, http.StatusUnauthorized)
}

func BenchmarkSessionCacheGet(b *testing.B) {
	c := newSessionCache()
	c.put(Session{UserID: "u", TokenHash: "token", Region: "asia"}, c.start())
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if c.get("token") == nil {
				b.Fatal("miss")
			}
		}
	})
}
