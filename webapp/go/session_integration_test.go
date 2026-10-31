package isureal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentLoginLocal(t *testing.T) {
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
	if _, err = db.Exec(`INSERT INTO users(id,username,secret_key_hash,display_name,region) VALUES ('user','user',$1,'user','asia')`, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.POST("/login", loginHandler(&HandlerContext{DB: db}))
	const count = 12
	tokens := make([]string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"user","secretKey":"secret"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != 200 {
				errs <- fmt.Errorf("status %d: %s", rec.Code, rec.Body.String())
				return
			}
			var result struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				errs <- err
				return
			}
			tokens[i] = result.Token
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	unique := map[string]bool{}
	for _, token := range tokens {
		if token == "" || unique[token] {
			t.Fatal("empty or duplicate token")
		}
		unique[token] = true
	}
	var total, active int
	if err = db.QueryRow(`SELECT count(*),count(*) FILTER(WHERE active) FROM login_sessions WHERE user_id='user'`).Scan(&total, &active); err != nil {
		t.Fatal(err)
	}
	if total != count || active != 1 {
		t.Fatalf("sessions=%d active=%d", total, active)
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"user","secretKey":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("invalid key status %d", rec.Code)
	}
	if err = db.Get(&active, `SELECT count(*) FROM login_sessions WHERE user_id='user' AND active`); err != nil || active != 1 {
		t.Fatalf("invalid login changed sessions: %d %v", active, err)
	}
	if _, err = CreateLoginSession(db, "user"); err != nil {
		t.Fatalf("direct helper: %v", err)
	}
	if err = db.Get(&active, `SELECT count(*) FROM login_sessions WHERE user_id='user' AND active`); err != nil || active != 1 {
		t.Fatalf("direct helper active=%d %v", active, err)
	}
}
