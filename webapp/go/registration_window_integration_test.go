package isureal

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func TestConcurrentRegistrationAndWindow(t *testing.T) {
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
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.POST("/register", registerHandler(&HandlerContext{DB: db}))
	e.POST("/window", openPostingWindowHandler(&HandlerContext{DB: db}))
	post := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	const workers = 12
	codes := make(chan int, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- post("/register", `{"username":"same","displayName":"Name","region":"asia"}`).Code
		}()
	}
	wg.Wait()
	close(codes)
	count := map[int]int{}
	for code := range codes {
		count[code]++
	}
	if count[201] != 1 || count[409] != workers-1 {
		t.Fatalf("same username results: %v", count)
	}
	for i := 0; i < 8; i++ {
		rec := post("/register", fmt.Sprintf(`{"username":"user%d","displayName":"Name","region":"asia"}`, i))
		if rec.Code != 201 {
			t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
		}
		var result struct {
			SecretKey string                                `json:"secretKey"`
			User      struct{ ID, Region, Username string } `json:"user"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.SecretKey == "" || result.User.ID == "" || result.User.Region != "asia" {
			t.Fatalf("registration response: %s %v", rec.Body.String(), err)
		}
	}
	windowCodes := make(chan int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			windowCodes <- post("/window", `{"region":"asia","localDate":"2026-10-31","onTimeSeconds":300}`).Code
		}()
	}
	wg.Wait()
	close(windowCodes)
	count = map[int]int{}
	for code := range windowCodes {
		count[code]++
	}
	if count[201] != 1 || count[409] != workers-1 {
		t.Fatalf("same window results: %v", count)
	}
	var windows, notifications int
	if err := db.Get(&windows, `SELECT count(*) FROM posting_windows WHERE region='asia' AND ended_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&notifications, `SELECT count(*) FROM notifications`); err != nil {
		t.Fatal(err)
	}
	if windows != 1 || notifications != 9 {
		t.Fatalf("active windows=%d notifications=%d", windows, notifications)
	}
}
