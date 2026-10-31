package isureal

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestPostImageTransactionLocal(t *testing.T) {
	for _, fileMode := range []bool{false, true} {
		name := "database"
		if fileMode {
			name = "file"
		}
		t.Run(name, func(t *testing.T) { testPostImageTransactionLocal(t, fileMode) })
	}
}

func testPostImageTransactionLocal(t *testing.T, fileMode bool) {
	dsn := os.Getenv("ISUCON_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires disposable local PostgreSQL")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if u.Hostname() != "127.0.0.1" {
		t.Fatal("local database required")
	}
	admin, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "test_post_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(`CREATE SCHEMA ` + name); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA ` + name + ` CASCADE`)
	params := u.Query()
	params.Set("search_path", name)
	u.RawQuery = params.Encode()
	db, err := sqlx.Connect("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(q string) {
		t.Helper()
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := os.ReadFile("../sql/001-schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	exec(string(schema))
	exec(`INSERT INTO users(id,username,secret_key_hash,display_name,region) VALUES ('user','user',repeat('ab',32),'user','asia')`)
	var photo, body bytes.Buffer
	if err := jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("image", "test.jpg")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(photo.Bytes())
	w.WriteField("visibility", "public")
	w.WriteField("location_kind", "none")
	w.Close()
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	h := &HandlerContext{DB: db, Images: NewImageCache()}
	if fileMode {
		h.Files, err = NewImageStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		h.StorageColumn = true
	}
	e.POST("/posts", createPostHandler(h), func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error { c.Set("session", &Session{UserID: "user"}); return next(c) }
	})
	post := func() int {
		req := httptest.NewRequest("POST", "/posts", bytes.NewReader(body.Bytes()))
		req.Header.Set("Content-Type", w.FormDataContentType())
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}
	check := func(posts, images int) {
		t.Helper()
		var p, i int
		if err := db.Get(&p, `SELECT count(*) FROM posts`); err != nil {
			t.Fatal(err)
		}
		if err := db.Get(&i, `SELECT count(*) FROM image_objects`); err != nil {
			t.Fatal(err)
		}
		if p != posts || i != images {
			t.Fatalf("posts/images=%d/%d want %d/%d", p, i, posts, images)
		}
	}
	if s := post(); s != 409 {
		t.Fatalf("no window: %d", s)
	}
	check(0, 0)
	exec(`INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES ('window','asia',current_date,clock_timestamp()-interval '1 minute',clock_timestamp()+interval '1 minute')`)
	statuses := make(chan int, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); statuses <- post() }()
	}
	wg.Wait()
	close(statuses)
	created := 0
	for s := range statuses {
		if s == 201 {
			created++
		} else if s != 409 {
			t.Fatalf("concurrent: %d", s)
		}
	}
	if created != 1 {
		t.Fatalf("created=%d", created)
	}
	check(1, 1)
	if fileMode {
		var stored bool
		if err := db.Get(&stored, `SELECT bool_and(storage_key IS NOT NULL AND octet_length(data)=0) FROM image_objects`); err != nil || !stored {
			t.Fatalf("file metadata not persisted: %v", err)
		}
	}
	exec(`UPDATE posts SET deleted_at=clock_timestamp(); CREATE FUNCTION reject_post() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$; CREATE TRIGGER reject_post BEFORE INSERT ON posts FOR EACH ROW EXECUTE FUNCTION reject_post()`)
	if s := post(); s != 500 {
		t.Fatalf("post insert failure: %d", s)
	}
	check(1, 1)
	exec(`DROP TRIGGER reject_post ON posts; UPDATE users SET status='deleted',deleted_at=clock_timestamp()`)
	if s := post(); s == 201 {
		t.Fatal("deleted user accepted")
	}
	check(1, 1)
}
