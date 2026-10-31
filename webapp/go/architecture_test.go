package isureal

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

const architectureTestKey = "local-test-key-for-image-proof-0123456789"

func jpegFixture(t *testing.T, size int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, size, size)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestImageStoreDurabilityAndChecksum(t *testing.T) {
	dir := t.TempDir()
	store, err := NewImageStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	data := jpegFixture(t, 16)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Put(data); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	key, err := store.Put(data)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewImageStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := reopened.Get(key)
	if err != nil || !bytes.Equal(data, actual) {
		t.Fatalf("restart read: %v", err)
	}
	if _, err = reopened.Get("../../etc/passwd"); err == nil {
		t.Fatal("unsafe key accepted")
	}
	if err = os.WriteFile(filepath.Join(dir, key), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.Get(key); err == nil {
		t.Fatal("corruption undetected")
	}
}

func TestEdgeProofCannotAuthorizeDifferentBytesOrDimensions(t *testing.T) {
	data := jpegFixture(t, 16)
	proof := edgeMAC(architectureTestKey, "jpeg", imageValidationValue(data, 512))
	e := echo.New()
	r := httptest.NewRequest("PUT", "/api/me/profile-image", nil)
	r.Header.Set(validationHeader, proof)
	c := e.NewContext(r, httptest.NewRecorder())
	h := &HandlerContext{Config: &Config{EdgeKey: architectureTestKey}}
	if err := validateJPEG(c, h, data, 512); err != nil {
		t.Fatal(err)
	}
	if err := validateJPEG(c, h, []byte("not jpeg"), 512); err == nil {
		t.Fatal("proof accepted for other bytes")
	}
	if validEdgeMAC(architectureTestKey, "jpeg", imageValidationValue(data, 4096), proof) {
		t.Fatal("proof accepted for other limit")
	}
	if validEdgeMAC("short", "jpeg", imageValidationValue(data, 512), proof) {
		t.Fatal("short key accepted")
	}
}

func architectureDB(t *testing.T) (*sqlx.DB, string) {
	t.Helper()
	dsn := os.Getenv("ISUREAL_ARCH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires disposable local PostgreSQL16")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		t.Fatal("architecture tests require loopback PostgreSQL")
	}
	admin, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "arch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sqlx.Connect("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
	execSQLFile(t, db, "001-schema.sql")
	return db, u.String()
}
func execSQLFile(t *testing.T, db *sqlx.DB, name string) {
	t.Helper()
	s, err := os.ReadFile("../sql/" + name)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../sql/architecture.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(s)
	if name == "001-schema.sql" || name == "reset.sql" {
		text += "\n" + string(migration)
	}
	if _, err = db.Exec(text); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if name == "reset.sql" {
		mustSQL(t, db, "SET session_replication_role=origin")
	}
}
func mustSQL(t *testing.T, db *sqlx.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
func fixtureOwner(t *testing.T, db *sqlx.DB) (User, string) {
	t.Helper()
	u := User{ID: "owner", Username: "owner", DisplayName: "Owner", SecretKeyHash: strings.Repeat("ab", 32), Region: "europe", Status: "active"}
	if err := u.setImageKey(); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, db, `INSERT INTO users(id,username,display_name,secret_key_hash,region,created_at) VALUES($1,$2,$3,$4,$5,'2025-01-01')`, u.ID, u.Username, u.DisplayName, u.SecretKeyHash, u.Region)
	token := "architecture-local-session"
	sum := sha256.Sum256([]byte(token))
	mustSQL(t, db, `INSERT INTO login_sessions(id,user_id,token_hash) VALUES('session','owner',$1)`, hex.EncodeToString(sum[:]))
	return u, token
}
func localRuntime(t *testing.T, db *sqlx.DB, dir string) *Runtime {
	t.Helper()
	files, err := NewImageStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := &Runtime{DB: db, Files: files, StorageColumn: true, Images: NewImageCache(imageCacheMaxBytes), IPBans: &IPBanList{}, Notifications: newNotificationPoller(db)}
	t.Cleanup(r.Notifications.close)
	return r
}
func request(t *testing.T, method, target, token, contentType string, body []byte, want int) []byte {
	t.Helper()
	req, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s: status=%d want=%d body=%s", method, strings.Split(target, "?")[0], resp.StatusCode, want, data)
	}
	return data
}

func TestArchitectureImagesAcrossEdgeRestartAndReset(t *testing.T) {
	db, _ := architectureDB(t)
	owner, token := fixtureOwner(t, db)
	data := jpegFixture(t, 16)
	// An initial image remains a DB blob and survives local file absence/reset.
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline,ended_at,created_at) VALUES('initial-window','europe','2025-01-01','2025-01-01','2025-01-02','2025-01-02','2025-01-01')`)
	mustSQL(t, db, `INSERT INTO image_objects(id,owner_user_id,data,media_type,created_at) VALUES('initial-image','owner',$1,'image/jpeg','2025-01-01')`, data)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at,created_at) VALUES('initial-post','owner','initial-window','initial-image','public','none','2025-01-01','2025-01-01')`)
	execSQLFile(t, db, "003-initial-snapshot.sql")
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('current-window','europe','2026-10-31',clock_timestamp(),clock_timestamp()+interval '1 day')`)
	dir := t.TempDir()
	runtime := localRuntime(t, db, dir)
	cfg := &Config{Region: "europe", EdgeKey: architectureTestKey}
	central := httptest.NewServer(NewRegionServer(cfg, runtime))
	defer central.Close()
	edgeCache := NewImageCache(imageCacheMaxBytes)
	edgeEcho := echo.New()
	edgeEcho.HTTPErrorHandler = APIErrorHandler
	edgeEcho.Any("/api/*", newEdgeProxy(&Config{EdgeUpstream: central.URL, EdgeKey: architectureTestKey}, edgeCache))
	edge := httptest.NewServer(edgeEcho)
	defer edge.Close()
	initialURL := signedImageURL(owner, "initial-image")
	if got := request(t, "GET", edge.URL+initialURL, "", "", nil, 200); !bytes.Equal(got, data) {
		t.Fatal("initial image differs")
	}
	// Create through the overseas path, including JPEG attestation and durable file.
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	file, _ := writer.CreateFormFile("image", "photo.jpg")
	file.Write(data)
	writer.WriteField("caption", "hello")
	writer.WriteField("visibility", "public")
	writer.WriteField("location_kind", "none")
	writer.Close()
	result := request(t, "POST", edge.URL+"/api/posts", token, writer.FormDataContentType(), form.Bytes(), 201)
	var posted createPostResponse
	if err := json.Unmarshal(result, &posted); err != nil {
		t.Fatal(err)
	}
	var blobSize int
	var storageKey string
	if err := db.QueryRow(`SELECT octet_length(data),storage_key FROM image_objects WHERE id=$1`, posted.ImageURLID()).Scan(&blobSize, &storageKey); err != nil {
		t.Fatal(err)
	}
	if blobSize != 0 || !validStorageKey(storageKey) {
		t.Fatal("new blob remained in DB")
	}
	for range 2 {
		if got := request(t, "GET", edge.URL+posted.ImageURL, "", "", nil, 200); !bytes.Equal(got, data) {
			t.Fatal("new image differs")
		}
	}
	exported := request(t, "GET", edge.URL+"/api/me/export", token, "", nil, 200)
	zr, err := zip.NewReader(bytes.NewReader(exported), int64(len(exported)))
	if err != nil {
		t.Fatal(err)
	}
	imageCount := 0
	for _, file := range zr.File {
		if strings.HasPrefix(file.Name, "images/") {
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(reader)
			reader.Close()
			if err != nil || !bytes.Equal(got, data) {
				t.Fatal("exported image differs")
			}
			imageCount++
		}
	}
	if imageCount != 2 {
		t.Fatalf("exported image count=%d", imageCount)
	}
	// RealMoji uses the same durable storage path; replacing it invalidates
	// the old image even when the edge retains its immutable bytes.
	mustSQL(t, db, `INSERT INTO users(id,username,secret_key_hash,display_name,region) VALUES('peer','peer',$1,'Peer','europe')`, owner.SecretKeyHash)
	mustSQL(t, db, `INSERT INTO image_objects(id,owner_user_id,data,media_type) VALUES('peer-image','peer',$1,'image/jpeg')`, data)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at) VALUES('peer-post','peer','current-window','peer-image','public','none',clock_timestamp())`)
	var reactionURL string
	for i := 0; i < 2; i++ {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		f, _ := mw.CreateFormFile("image", "reaction.jpg")
		f.Write(data)
		mw.WriteField("reactionType", "smile")
		mw.Close()
		got := request(t, "POST", edge.URL+"/api/posts/peer-post/realmojis", token, mw.FormDataContentType(), body.Bytes(), 201)
		var reaction replaceRealMojiResponse
		if err := json.Unmarshal(got, &reaction); err != nil {
			t.Fatal(err)
		}
		request(t, "GET", edge.URL+reaction.ImageURL, "", "", nil, 200)
		if reactionURL != "" {
			request(t, "GET", edge.URL+reactionURL, "", "", nil, 403)
		}
		reactionURL = reaction.ImageURL
	}
	// The same sessions and public APIs still enforce visibility, independent
	// of the edge's byte cache and trusted JPEG validation proof.
	request(t, "GET", edge.URL+"/api/posts/peer-post", token, "", nil, 200)
	mustSQL(t, db, `UPDATE posts SET visibility='friends' WHERE id='peer-post'`)
	request(t, "GET", edge.URL+"/api/posts/peer-post", token, "", nil, 404)
	mustSQL(t, db, `UPDATE posts SET visibility='public' WHERE id='peer-post'`)
	mustSQL(t, db, `INSERT INTO block_relations(id,blocker_id,blocked_id) VALUES('block','peer','owner')`)
	request(t, "GET", edge.URL+"/api/posts/peer-post", token, "", nil, 404)
	mustSQL(t, db, `DELETE FROM block_relations WHERE id='block'`)
	// Empty central caches and reopen the store, simulating a process restart.
	runtime.Images.Reset()
	runtime.Files, _ = NewImageStore(dir)
	edgeCache.Reset()
	if got := request(t, "GET", edge.URL+posted.ImageURL, "", "", nil, 200); !bytes.Equal(got, data) {
		t.Fatal("restart image differs")
	}
	// State changes must reject cached bytes immediately; no file deletion needed.
	request(t, "DELETE", edge.URL+"/api/posts/"+posted.ID, token, "", nil, 204)
	request(t, "GET", edge.URL+posted.ImageURL, "", "", nil, 403)
	mustSQL(t, db, `UPDATE posts SET deleted_at=NULL WHERE id=$1`, posted.ID)
	request(t, "GET", edge.URL+posted.ImageURL, "", "", nil, 200)
	request(t, "GET", edge.URL+posted.ImageURL+"&exp=1", "", "", nil, 403)
	bad := strings.Replace(posted.ImageURL, "sig=", "sig=x", 1)
	request(t, "GET", edge.URL+bad, "", "", nil, 403)
	request(t, "PUT", edge.URL+"/api/me/profile-image", token, "image/jpeg", data, 204)
	var profile string
	db.Get(&profile, `SELECT profile_image_id FROM users WHERE id='owner'`)
	profileURL := signedImageURL(owner, profile)
	request(t, "GET", edge.URL+profileURL, "", "", nil, 200)
	request(t, "PUT", edge.URL+"/api/me/profile-image", token, "image/jpeg", jpegFixture(t, 17), 204)
	request(t, "GET", edge.URL+profileURL, "", "", nil, 403)
	request(t, "POST", edge.URL+"/api/me/account-deletion", token, "", nil, 204)
	request(t, "GET", edge.URL+posted.ImageURL, "", "", nil, 403)
	// Correctly encoded but expired signatures must also fail on cache hits.
	exp := time.Now().Add(-time.Second).Unix()
	expired := fmt.Sprintf("/api/images/%s?exp=%d&sig=%s", posted.ImageURLID(), exp, base64.RawURLEncoding.EncodeToString(imageSignature(owner.ImageKey, posted.ImageURLID(), exp)))
	request(t, "GET", edge.URL+expired, "", "", nil, 403)
	// Actual reset script restores DB state; cached new IDs cannot resurrect it.
	execSQLFile(t, db, "reset.sql")
	runtime.Images.Reset()
	edgeCache.Reset()
	request(t, "GET", edge.URL+posted.ImageURL, "", "", nil, 403)
	if got := request(t, "GET", edge.URL+initialURL, "", "", nil, 200); !bytes.Equal(got, data) {
		t.Fatal("initial image lost after reset")
	}
	execSQLFile(t, db, "reset.sql") // idempotency
}

func (r createPostResponse) ImageURLID() string {
	u, _ := url.Parse(r.ImageURL)
	return strings.TrimPrefix(u.Path, "/api/images/")
}

func TestNotificationCommitRollbackAndRegionChange(t *testing.T) {
	db, dsn := architectureDB(t)
	fixtureOwner(t, db)
	p := newNotificationPoller(db)
	p.startListener(dsn)
	defer p.close()
	deadline := time.Now().Add(3 * time.Second)
	for !p.listening.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !p.listening.Load() {
		t.Fatal("LISTEN failed")
	}
	var since time.Time
	db.Get(&since, `SELECT clock_timestamp()`)
	s := p.subscribe("owner", "europe", since)
	defer p.unsubscribe(s)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('w','europe','2026-10-31',clock_timestamp(),clock_timestamp())`)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO notifications(id,user_id,posting_window_id,created_at) VALUES('rolled-back','owner','w',clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	select {
	case <-s.result:
		t.Fatal("rolled back notification delivered")
	case <-time.After(100 * time.Millisecond):
	}
	start := time.Now()
	mustSQL(t, db, `INSERT INTO notifications(id,user_id,posting_window_id,created_at) VALUES('committed','owner','w',clock_timestamp())`)
	select {
	case n := <-s.result:
		if n == nil || n.ID != "committed" {
			t.Fatalf("wrong notification: %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("commit did not wake subscriber")
	}
	t.Logf("commit-to-delivery=%s", time.Since(start))
	db.Get(&since, `SELECT clock_timestamp()`)
	s2 := p.subscribe("owner", "europe", since)
	defer p.unsubscribe(s2)
	mustSQL(t, db, `UPDATE users SET region='america' WHERE id='owner'`)
	select {
	case n := <-s2.result:
		if n != nil {
			t.Fatal("region change delivered event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("region change did not disconnect")
	}
}

func TestSharedRuntimeEnforcesListenerRegion(t *testing.T) {
	db, _ := architectureDB(t)
	_, token := fixtureOwner(t, db)
	r := localRuntime(t, db, t.TempDir())
	eu := httptest.NewServer(NewRegionServer(&Config{Region: "europe"}, r))
	defer eu.Close()
	asia := httptest.NewServer(NewRegionServer(&Config{Region: "asia"}, r))
	defer asia.Close()
	request(t, "GET", eu.URL+"/api/me", token, "", nil, 200)
	request(t, "GET", asia.URL+"/api/me", token, "", nil, 403)
}

func TestListenerConfiguration(t *testing.T) {
	for _, spec := range []string{"asia:8080,europe:8081,america:8082", "asia:8080"} {
		if _, err := (&Config{ListenRegions: spec, EdgeKey: architectureTestKey}).RegionListeners(); err != nil {
			t.Fatal(err)
		}
	}
	for _, spec := range []string{"asia:8080,europe:8080", "asia:8080,asia:8081", "unknown:8080", "asia:0"} {
		if _, err := (&Config{ListenRegions: spec, EdgeKey: architectureTestKey}).RegionListeners(); err == nil {
			t.Fatal(fmt.Sprintf("accepted %q", spec))
		}
	}
}

func TestEdgeCacheRevalidatesWithoutTransferringBodyAndPinsEvictedBytes(t *testing.T) {
	cache := NewImageCache(imageCacheMaxBytes)
	data := jpegFixture(t, 16)
	var mu sync.Mutex
	fullBodies, checks := 0, 0
	deleted := false
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		checks++
		if deleted {
			w.WriteHeader(403)
			return
		}
		if hasCachedImageProof(r, architectureTestKey) {
			// Eviction while the verification is in flight must not leak a 204.
			cache.Reset()
			w.WriteHeader(204)
			return
		}
		fullBodies++
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(data)
	}))
	defer origin.Close()
	e := echo.New()
	e.Any("/api/*", newEdgeProxy(&Config{EdgeUpstream: origin.URL, EdgeKey: architectureTestKey}, cache))
	edge := httptest.NewServer(e)
	defer edge.Close()
	for range 2 {
		got := request(t, "GET", edge.URL+"/api/images/id?exp=123&sig=opaque", "", "", nil, 200)
		if !bytes.Equal(got, data) {
			t.Fatal("cache bytes differ")
		}
	}
	mu.Lock()
	if fullBodies != 1 || checks != 2 {
		t.Fatalf("bodies/checks=%d/%d", fullBodies, checks)
	}
	deleted = true
	mu.Unlock()
	request(t, "GET", edge.URL+"/api/images/id?exp=123&sig=opaque", "", "", nil, 403)
}

func TestCacheResetDiscardsInflightFill(t *testing.T) {
	cache := NewImageCache(imageCacheMaxBytes)
	_, generation := cache.GetGeneration("initial")
	cache.Reset()
	cache.PutGeneration("initial", &cachedImage{Data: []byte("old")}, generation)
	if cache.Get("initial") != nil {
		t.Fatal("late fill survived reset")
	}
}

func TestImageMissingFileCannotBecomeEmptySuccess(t *testing.T) {
	store, err := NewImageStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("0", 64)
	h := &HandlerContext{Files: store}
	if _, err = h.imageBytes([]byte{}, &key); err == nil {
		t.Fatal("missing file became empty success")
	}
	legacy := []byte("original-db-blob")
	if got, err := h.imageBytes(legacy, &key); err != nil || !bytes.Equal(got, legacy) {
		t.Fatal("DB fallback failed")
	}
}

func TestNotificationInitializeGenerationPreservesNewSubscribers(t *testing.T) {
	db, dsn := architectureDB(t)
	fixtureOwner(t, db)
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('w','europe','2026-10-31',clock_timestamp(),clock_timestamp())`)
	p := newNotificationPoller(db)
	p.epochSchema = true
	p.startListener(dsn)
	defer p.close()
	deadline := time.Now().Add(3 * time.Second)
	for !p.listening.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !p.listening.Load() {
		t.Fatal("LISTEN failed")
	}
	var epoch string
	var since time.Time
	if err := db.QueryRow(`SELECT clock_timestamp(),generation::text FROM isureal_runtime_state`).Scan(&since, &epoch); err != nil {
		t.Fatal(err)
	}
	old := p.subscribe("owner", "europe", since, epoch)
	defer p.unsubscribe(old)
	mustSQL(t, db, `UPDATE isureal_runtime_state SET generation=gen_random_uuid()`)
	if err := db.QueryRow(`SELECT clock_timestamp(),generation::text FROM isureal_runtime_state`).Scan(&since, &epoch); err != nil {
		t.Fatal(err)
	}
	fresh := p.subscribe("owner", "europe", since, epoch)
	defer p.unsubscribe(fresh)
	// A delayed reset event must close only subscribers from the old generation.
	mustSQL(t, db, `SELECT pg_notify('isureal_state','reset')`)
	mustSQL(t, db, `INSERT INTO notifications(id,user_id,posting_window_id,created_at) VALUES('new-generation','owner','w',clock_timestamp())`)
	select {
	case n := <-old.result:
		if n != nil {
			t.Fatal("old generation got a notification")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("old generation remained subscribed")
	}
	select {
	case n := <-fresh.result:
		if n == nil || n.ID != "new-generation" {
			t.Fatalf("new generation was lost: %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("new subscriber not notified")
	}
}
