package isureal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

func TestCreateFriendRequestPairLocal(t *testing.T) {
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

	if _, err := db.Exec(`INSERT INTO users(id,username,secret_key_hash,display_name,region) VALUES
 ('a','a','hash','a','asia'),('m','m','hash','m','asia'),('z','z','hash','z','asia');
 INSERT INTO friendships(id,user_low_id,user_high_id) VALUES ('friend','a','z')`); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	actor := "a"
	e.POST("/requests", func(c *echo.Context) error {
		c.Set("session", &Session{UserID: actor})
		return createFriendRequestHandler(&HandlerContext{DB: db})(c)
	})
	for _, tc := range []struct {
		actor, target string
		status        int
		result        string
	}{
		{"a", "z", 409, "relationship_conflict"},
		{"z", "a", 409, "relationship_conflict"},
		{"m", "a", 201, "pending"},
		{"m", "a", 409, "relationship_conflict"},
		{"a", "m", 201, "accepted"},
		{"m", "a", 409, "relationship_conflict"},
	} {
		actor = tc.actor
		req := httptest.NewRequest(http.MethodPost, "/requests", strings.NewReader(`{"recipientId":"`+tc.target+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s -> %s: status %d, body %s", actor, tc.target, rec.Code, rec.Body.String())
		}
		if tc.status == 409 {
			if !strings.Contains(rec.Body.String(), tc.result) {
				t.Fatal(rec.Body.String())
			}
		} else {
			var response createFriendRequestResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Status != tc.result {
				t.Fatalf("status = %s", response.Status)
			}
		}
	}
	var count int
	if err := db.Get(&count, `SELECT count(*) FROM friend_requests`); err != nil || count != 1 {
		t.Fatalf("requests = %d, error = %v", count, err)
	}
}
