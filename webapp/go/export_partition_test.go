package isureal

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

func TestExportPartitionLookup(t *testing.T) {
	db, _ := architectureDB(t)
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,region)
 VALUES('owner','owner',repeat('ab',32),'Owner','asia'),('peer','peer',repeat('cd',32),'Peer','asia')`)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline,ended_at)
 SELECT 'w'||n,'asia','2024-09-01'::date+n*9,'2024-09-01'::timestamptz+n*interval '9 days',
 '2024-09-01'::timestamptz+n*interval '9 days'+interval '30 seconds',now() FROM generate_series(1,100) n;
 INSERT INTO image_objects(id,owner_user_id,data,media_type,created_at)
 SELECT 'img'||n,'owner',decode(md5(n::text),'hex'),'image/jpeg','2024-09-01'::timestamptz+n*interval '9 days' FROM generate_series(1,100) n;
 INSERT INTO posts(id,author_id,posting_window_id,image_id,caption,visibility,location_kind,posted_at,deleted_at)
 SELECT 'p'||lpad(n::text,3,'0'),'owner','w'||n,'img'||n,'caption '||n,'public','none',
 '2024-09-01'::timestamptz+(n/2)*interval '9 days',CASE WHEN n=99 THEN now() END FROM generate_series(1,100) n;
 UPDATE posts SET author_id='peer' WHERE id='p100';
 INSERT INTO image_objects(id,owner_user_id,data,media_type,created_at)
 SELECT 'noise'||n,'peer',decode(md5(n::text),'hex'),'image/jpeg',
 '2024-09-02'::timestamptz+(n%34)*interval '1 month' FROM generate_series(1,20000) n;
 ANALYZE users; ANALYZE posts; ANALYZE posting_windows; ANALYZE image_objects`)
	files, err := NewImageStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fileBytes := []byte("file-backed-image")
	key, err := files.Put(fileBytes)
	if err != nil {
		t.Fatal(err)
	}
	mustSQL(t, db, `UPDATE image_objects SET storage_key=$1,data='' WHERE id='img2'`, key)
	old := `SELECT ` + postColumns + `,i.data AS image_data,i.storage_key
 FROM posts p JOIN posting_windows w ON w.id=p.posting_window_id JOIN image_objects i ON i.id=p.image_id
 WHERE p.author_id=$1 AND p.deleted_at IS NULL ORDER BY p.posted_at,p.id`
	var want []exportPost
	if err := db.Select(&want, old, "owner"); err != nil {
		t.Fatal(err)
	}
	if len(want) != 98 {
		t.Fatal("unexpected fixture", len(want))
	}
	check := func(stage string) {
		t.Helper()
		got, err := selectExportPosts(t.Context(), db, "owner", true)
		if err != nil {
			t.Fatal(stage, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal(stage, "export rows differ")
		}
		empty, err := selectExportPosts(t.Context(), db, "missing", true)
		if err != nil || len(empty) != 0 {
			t.Fatal(stage, "empty export", err)
		}
	}
	check("no locator table")
	mustSQL(t, db, `CREATE TABLE image_locations(id varchar(36) PRIMARY KEY,created_at timestamptz NOT NULL);
 INSERT INTO image_locations SELECT id,created_at FROM image_objects WHERE id<>'img1'; ANALYZE image_locations`)
	check("missing locator entry")
	mustSQL(t, db, `INSERT INTO image_locations SELECT id,created_at FROM image_objects WHERE id='img1'`)
	check("all locators")
	// A stale locator for an already-deleted image must not create an extra row.
	mustSQL(t, db, `INSERT INTO image_locations VALUES('gone-image','2025-01-01')`)
	check("stale locator")
	// Exercise the branch that does not select storage_key as well.
	withoutStorage, err := selectExportPosts(t.Context(), db, "owner", false)
	if err != nil || len(withoutStorage) != len(want) {
		t.Fatal("legacy columns", err)
	}

	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.GET("/export", func(c *echo.Context) error {
		c.Set("session", &Session{UserID: "owner"})
		return exportPostsHandler(&HandlerContext{DB: db, Files: files, StorageColumn: true})(c)
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/export", nil))
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("export response %d", rec.Code)
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var manifest []exportPostResponse
	images := map[string][]byte{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == "posts.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
		} else {
			images[f.Name] = data
		}
	}
	if len(manifest) != len(want) || len(images) != len(want) {
		t.Fatal("ZIP contents lost")
	}
	for i, p := range want {
		name := fmt.Sprintf("images/%06d.jpg", i+1)
		data := p.ImageData
		if p.StorageKey != nil {
			data = fileBytes
		}
		if manifest[i].ID != p.ID || manifest[i].ImagePath != name || manifest[i].Caption != p.Caption || !bytes.Equal(images[name], data) {
			t.Fatal("ZIP order/content differs", i)
		}
	}
	// Compare selection cost separately from image bytes and ZIP construction.
	var total [2]time.Duration
	for repetition := 0; repetition < 11; repetition++ {
		for variant := 0; variant < 2; variant++ {
			start := time.Now()
			if variant == 0 {
				var rows []exportPost
				err = db.Select(&rows, old, "owner")
			} else {
				_, err = selectExportPosts(t.Context(), db, "owner", true)
			}
			if err != nil {
				t.Fatal(err)
			}
			if repetition > 0 {
				total[variant] += time.Since(start)
			}
		}
	}
	t.Logf("98 posts / 20100 small images: old=%s new=%s (10 alternating samples after warmup, selection only)", total[0]/10, total[1]/10)
	for i, query := range []string{old, `SELECT ` + postColumns + `,i.data AS image_data,i.storage_key FROM posts p JOIN posting_windows w ON w.id=p.posting_window_id` + exportImageJoin + ` WHERE p.author_id=$1 AND p.deleted_at IS NULL ORDER BY p.posted_at,p.id`} {
		var raw []byte
		if err := db.Get(&raw, "EXPLAIN(ANALYZE,BUFFERS,FORMAT JSON) "+query, "owner"); err != nil {
			t.Fatal(err)
		}
		var plans []map[string]any
		if err := json.Unmarshal(raw, &plans); err != nil {
			t.Fatal(err)
		}
		imageLoops, imageRows := float64(0), float64(0)
		var walk func(map[string]any)
		walk = func(n map[string]any) {
			if name, ok := n["Relation Name"].(string); ok && strings.HasPrefix(name, "image_objects_") {
				imageLoops += n["Actual Loops"].(float64)
				imageRows += n["Actual Loops"].(float64) * n["Actual Rows"].(float64)
			}
			if children, ok := n["Plans"].([]any); ok {
				for _, child := range children {
					walk(child.(map[string]any))
				}
			}
		}
		walk(plans[0]["Plan"].(map[string]any))
		t.Logf("variant=%d image partition scans=%g rows=%g execution-ms=%v buffers=%v", i, imageLoops, imageRows, plans[0]["Execution Time"], plans[0]["Plan"].(map[string]any)["Shared Hit Blocks"])
		if i == 1 && imageLoops > float64(len(want)) {
			t.Fatal("known images scanned extra partitions", imageLoops)
		}
	}
	mustSQL(t, db, `ALTER TABLE image_objects DROP COLUMN storage_key`)
	legacy, err := selectExportPosts(t.Context(), db, "owner", false)
	if err != nil || len(legacy) != len(want) {
		t.Fatal("schema without storage_key", err)
	}
}
