package isureal

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type edgeReadQueryCounter struct {
	readQueryCounter
	prepares atomic.Uint64
}

func (q *edgeReadQueryCounter) TracePrepareStart(ctx context.Context, _ *pgx.Conn, _ pgx.TracePrepareStartData) context.Context {
	q.prepares.Add(1)
	return ctx
}

func (q *edgeReadQueryCounter) TracePrepareEnd(context.Context, *pgx.Conn, pgx.TracePrepareEndData) {}

type edgeReadLogs struct {
	sync.Mutex
	bytes.Buffer
}

func (b *edgeReadLogs) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}

func (b *edgeReadLogs) take(t *testing.T) []map[string]any {
	t.Helper()
	b.Lock()
	defer b.Unlock()
	var out []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(b.Bytes()), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatal(err)
		}
		out = append(out, entry)
	}
	b.Reset()
	return out
}

type edgeReadFixture struct {
	db, edgeDB         *sqlx.DB
	runtime            *Runtime
	core, edge         *httptest.Server
	edgeRuntime        *Runtime
	coreLogs, edgeLogs edgeReadLogs
	calls              atomic.Int64
	queries            *edgeReadQueryCounter
	owner              User
	token, imageURL    string
	data               []byte
}

func newEdgeReadFixture(t *testing.T, imageFlag, readFlag string) *edgeReadFixture {
	t.Helper()
	t.Setenv("ISUREAL_EDGE_IMAGE_VERIFY", imageFlag)
	t.Setenv("ISUREAL_EDGE_READS", readFlag)
	db, dsn := architectureDB(t)
	owner, token := fixtureOwner(t, db)
	f := &edgeReadFixture{db: db, owner: owner, token: token, data: jpegFixture(t, 16), queries: &edgeReadQueryCounter{}}
	mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline)
 VALUES('window','europe','2025-01-01','2025-01-01','2025-01-01 01:00')`)
	mustSQL(t, db, `INSERT INTO image_objects(id,owner_user_id,data,media_type,created_at)
 VALUES('image','owner',$1,'image/jpeg','2025-01-01'),('profile','owner',$1,'image/jpeg','2025-01-01')`, f.data)
	mustSQL(t, db, `UPDATE users SET profile_image_id='profile',bio='日本語 <>&' WHERE id='owner'`)
	mustSQL(t, db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,caption,visibility,location_kind,latitude,longitude,posted_at)
 VALUES('post','owner','window','image','写真 <>&','friends_of_friends','exact',35.5,139.25,'2025-01-01 01:00:03.123456')`)
	// Production creates this mapping and trigger in reset.sql; do not run reset
	// over the independent fixture. Missing mappings are tested below as fallback.
	mustSQL(t, db, `CREATE TABLE image_locations AS SELECT id,created_at FROM image_objects;
 CREATE UNIQUE INDEX image_locations_id ON image_locations(id)`)
	f.runtime = localRuntime(t, db, t.TempDir())
	coreEcho := NewRegionServer(&Config{Region: "europe", EdgeKey: architectureTestKey, LogLevel: "error"}, f.runtime)
	coreEcho.Pre(ActionLogMiddleware("debug", &f.coreLogs))
	f.core = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		coreEcho.ServeHTTP(w, r)
	}))
	t.Cleanup(f.core.Close)
	pc, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pc.Tracer = f.queries
	f.edgeDB = sqlx.NewDb(stdlib.OpenDB(*pc), "pgx")
	configureDatabasePool(f.edgeDB.DB)
	t.Cleanup(func() { f.edgeDB.Close() })
	f.edgeRuntime = localRuntime(t, f.edgeDB, t.TempDir())
	f.startEdge(t)
	f.imageURL = buildSignedImageURL(owner.ImageKey, "image", time.Now().Add(time.Hour).Unix())
	return f
}

func (f *edgeReadFixture) startEdge(t *testing.T) {
	t.Helper()
	e := NewRegionServer(&Config{Region: "europe", EdgeUpstream: f.core.URL, EdgeKey: architectureTestKey, LogLevel: "error"}, f.edgeRuntime)
	e.Pre(ActionLogMiddleware("debug", &f.edgeLogs))
	f.edge = httptest.NewServer(e)
	t.Cleanup(f.edge.Close)
}

func edgeReadHTTP(t *testing.T, base, path, auth, ip string) (int, http.Header, []byte) {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", auth)
	r.Header.Set("X-Client-IP", ip)
	// Publicly supplied internal proofs must never turn a fast GET into 204.
	r.Header.Set(cachedImageHeader, "untrusted")
	r.Header.Set(validationHeader, "untrusted")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	// Date is generated independently by net/http on the two connections.
	resp.Header.Del("Date")
	return resp.StatusCode, resp.Header, body
}

func (f *edgeReadFixture) compare(t *testing.T, path, auth, ip string, status int, fast bool, queries uint64) []byte {
	t.Helper()
	// IP-ban cache is core behavior; use a fresh snapshot when fixtures mutate DB.
	f.runtime.IPBans.snapshot.Store(nil)
	f.coreLogs.take(t)
	f.edgeLogs.take(t)
	// Keep byte comparisons literal even at an image URL expiry-bucket boundary.
	for attempt := 0; attempt < 3; attempt++ {
		bucket := imageURLExpiry(time.Now(), imageURLExpiryBucket)
		wantStatus, wantHeader, wantBody := edgeReadHTTP(t, f.core.URL, path, auth, ip)
		wantLogs := f.coreLogs.take(t)
		f.calls.Store(0)
		f.queries.count.Store(0)
		f.queries.prepares.Store(0)
		gotStatus, gotHeader, gotBody := edgeReadHTTP(t, f.edge.URL, path, auth, ip)
		coreLogs, edgeLogs := f.coreLogs.take(t), f.edgeLogs.take(t)
		if imageURLExpiry(time.Now(), imageURLExpiryBucket) != bucket {
			continue
		}
		if !fast && strings.HasPrefix(path, "/api/images/") && gotStatus == 200 {
			// The existing proxy removes Content-Length when it substitutes a
			// cached body for core's 204. Preserve that existing chunked response.
			wantHeader.Del("Content-Length")
			gotHeader.Del("Content-Length")
		}
		if wantStatus != status || gotStatus != wantStatus || !bytes.Equal(gotBody, wantBody) || !reflect.DeepEqual(gotHeader, wantHeader) {
			t.Fatalf("%s: core=%d %v (%d bytes) edge=%d %v (%d bytes), equal body=%v", path, wantStatus, wantHeader, len(wantBody), gotStatus, gotHeader, len(gotBody), bytes.Equal(wantBody, gotBody))
		}
		wantCalls := int64(1)
		if fast {
			wantCalls = 0
		}
		if f.calls.Load() != wantCalls || f.queries.count.Load() != queries {
			t.Fatalf("%s: core calls=%d want=%d, edge queries=%d want=%d", path, f.calls.Load(), wantCalls, f.queries.count.Load(), queries)
		}
		if f.queries.prepares.Load() != 0 {
			t.Fatal("edge query performed an extra prepare/describe round trip")
		}
		logs := coreLogs
		if fast {
			logs = edgeLogs
		} else if strings.HasPrefix(path, "/api/images/") && gotStatus == 200 && len(logs) == 1 && logs[0]["status"] == float64(204) {
			// Existing cached-image fallback logs the internal 204 at core.
			wantLogs[0]["status"] = float64(204)
		}
		if !reflect.DeepEqual(logs, wantLogs) || len(logs) != 1 {
			t.Fatalf("%s: action logs differ: core=%v actual=%v", path, wantLogs, logs)
		}
		return gotBody
	}
	t.Fatal("response comparisons repeatedly crossed expiry buckets")
	return nil
}

func (f *edgeReadFixture) cacheImage(id string) {
	// Overseas GET/upload caches have no OwnerID.
	f.edgeRuntime.Images.Put(id, &cachedImage{MediaType: "image/jpeg", Data: f.data})
}

func TestEdgeReadsFlags(t *testing.T) {
	for _, flags := range [][2]string{{"", ""}, {"false", "false"}, {"TRUE", "1"}, {"true", "false"}, {"false", "true"}} {
		t.Run(strings.Join(flags[:], "/"), func(t *testing.T) {
			f := newEdgeReadFixture(t, flags[0], flags[1])
			f.cacheImage("image")
			for _, path := range []string{f.imageURL, "/api/posting-window/current", "/api/posts/me/current"} {
				on := flags[1] == "true"
				if strings.HasPrefix(path, "/api/images/") {
					on = flags[0] == "true"
				}
				q := uint64(0)
				if on {
					q = 1
				}
				f.compare(t, path, "Bearer "+f.token, "198.51.100.1", 200, on, q)
			}
		})
	}
}

func TestEdgeImageVerify(t *testing.T) {
	f := newEdgeReadFixture(t, "true", "true")
	const ip = "198.51.100.2"
	check := func(path string, status int, fast bool, queries uint64) {
		t.Helper()
		f.compare(t, path, "", ip, status, fast, queries)
	}
	check(f.imageURL, 200, false, 0) // no body: existing proxy populates cache
	check(f.imageURL, 200, true, 1)
	f.runtime.Images.Reset() // neither core cache nor its OwnerID is required
	check(f.imageURL, 200, true, 1)
	profile := buildSignedImageURL(f.owner.ImageKey, "profile", time.Now().Add(time.Hour).Unix())
	f.cacheImage("profile")
	check(profile, 200, true, 1)
	// Cached metadata disagreement is never sufficient to authorize a GET.
	f.edgeRuntime.Images.Reset()
	f.edgeRuntime.Images.Put("image", &cachedImage{OwnerID: "other", MediaType: "image/jpeg", Data: f.data})
	check(f.imageURL, 200, false, 1)
	f.edgeRuntime.Images.Reset()
	f.cacheImage("image")
	f.cacheImage("profile")
	for _, visibility := range []string{"friends", "friends_of_friends", "public"} {
		mustSQL(t, f.db, `UPDATE posts SET visibility=$1 WHERE id='post'`, visibility)
		check(f.imageURL, 200, true, 1) // image GET itself uses the signed capability
	}
	for _, tc := range []struct {
		suffix string
		q      uint64
	}{
		{"?exp=0&sig=x", 0},
		{"?exp=01&sig=x", 0},
		{"?exp=9999999999&sig=x", 0},
		{"?exp=9999999999&sig=" + strings.Repeat("A", 43), 1}, // valid shape, bad HMAC
		{"?exp=9999999999&exp=1&sig=x", 0},
		{"?exp=9999999999&sig=x&sig=y", 0},
		{"?exp=9999999999&sig=%zz", 0},
		{"", 0},
	} {
		check("/api/images/image"+tc.suffix, 403, false, tc.q)
	}
	expired := buildSignedImageURL(f.owner.ImageKey, "image", time.Now().Add(-time.Second).Unix())
	check(expired, 403, false, 0)
	mustSQL(t, f.db, `UPDATE posts SET deleted_at=clock_timestamp() WHERE id='post'`)
	check(f.imageURL, 403, false, 1)
	mustSQL(t, f.db, `UPDATE posts SET deleted_at=NULL WHERE id='post'; UPDATE users SET status='deleted' WHERE id='owner'`)
	check(f.imageURL, 403, false, 1)
	mustSQL(t, f.db, `UPDATE users SET status='active',profile_image_id=NULL WHERE id='owner'`)
	check(profile, 403, false, 1)
	mustSQL(t, f.db, `INSERT INTO ip_bans(ip,banned_until) VALUES($1,clock_timestamp()+interval '1 hour')`, ip)
	check(f.imageURL, 403, false, 1)
	mustSQL(t, f.db, `UPDATE ip_bans SET banned_until=clock_timestamp()-interval '1 second'`)
	check(f.imageURL, 200, true, 1)
	mustSQL(t, f.db, `DELETE FROM image_locations WHERE id='image'`)
	check(f.imageURL, 200, false, 1)
	mustSQL(t, f.db, `DROP TABLE image_locations`)
	check(f.imageURL, 200, false, 1) // undefined table: no edge retry
}

func TestEdgeCurrentReads(t *testing.T) {
	f := newEdgeReadFixture(t, "true", "true")
	const ip = "198.51.100.3"
	auth := "Bearer " + f.token
	window, post := "/api/posting-window/current", "/api/posts/me/current"
	check := func(path string, status int, fast bool) []byte {
		t.Helper()
		return f.compare(t, path, auth, ip, status, fast, 1)
	}
	check(window, 200, true)
	check(post, 200, true)
	for _, location := range []string{"location_kind='none',latitude=NULL,longitude=NULL,locality=NULL", "location_kind='approximate',latitude=NULL,longitude=NULL,locality='東京'"} {
		mustSQL(t, f.db, `UPDATE posts SET `+location+` WHERE id='post'`)
		check(post, 200, true)
	}
	// The current window's local date includes a post made in another region,
	// even after that original window has ended.
	mustSQL(t, f.db, `UPDATE posting_windows SET region='asia',ended_at=clock_timestamp() WHERE id='window';
 INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline)
 VALUES('europe-window','europe','2025-01-01','2025-01-01','2025-01-01 01:00')`)
	check(window, 200, true)
	check(post, 200, true)
	mustSQL(t, f.db, `DELETE FROM posting_windows WHERE id='europe-window'; UPDATE posting_windows SET region='europe',ended_at=NULL WHERE id='window'`)
	for _, badAuth := range []string{"", "bearer " + f.token, "Basic x", "Bearer invalid"} {
		q := uint64(0)
		if strings.HasPrefix(badAuth, "Bearer ") {
			q = 1
		}
		for _, path := range []string{window, post} {
			f.compare(t, path, badAuth, ip, 401, false, q)
		}
	}
	for _, state := range []struct {
		sql    string
		status int
	}{
		{`UPDATE users SET region='asia' WHERE id='owner'`, 403},
		{`UPDATE users SET region='europe',status='deleted' WHERE id='owner'`, 401},
		{`UPDATE users SET status='active' WHERE id='owner'; UPDATE login_sessions SET active=false`, 401},
	} {
		mustSQL(t, f.db, state.sql)
		check(window, state.status, false)
		check(post, state.status, false)
	}
	mustSQL(t, f.db, `UPDATE login_sessions SET active=true`)
	mustSQL(t, f.db, `INSERT INTO ip_bans(ip,banned_until) VALUES($1,clock_timestamp()+interval '1 hour')`, ip)
	check(window, 403, false)
	check(post, 403, false)
	mustSQL(t, f.db, `UPDATE ip_bans SET banned_until=clock_timestamp()-interval '1 second'`)
	check(window, 200, true)
	check(post, 200, true)
	// A successful core write is visible on the very next overseas read.
	request(t, "PATCH", f.core.URL+"/api/posts/post", f.token, "application/json", []byte(`{"caption":"変更後","visibility":"public"}`), 200)
	if body := check(post, 200, true); !bytes.Contains(body, []byte("変更後")) {
		t.Fatal("core write was not visible at edge")
	}
	request(t, "DELETE", f.core.URL+"/api/posts/post", f.token, "", nil, 204)
	check(post, 404, false)
	check(window, 200, true)
	mustSQL(t, f.db, `INSERT INTO posts(id,author_id,posting_window_id,image_id,visibility,location_kind,posted_at)
 VALUES('post2','owner','window','image','public','none','2025-01-01 02:00')`)
	check(post, 200, true)
	request(t, "DELETE", f.core.URL+"/api/posts/post2", f.token, "", nil, 204)
	check(post, 404, false)
	var result getCurrentPostingWindowResponse
	if err := json.Unmarshal(check(window, 200, true), &result); err != nil || result.CanPost || result.DeletionCount != 2 || result.HasPosted {
		t.Fatal("two deletions did not prevent posting", result, err)
	}
	mustSQL(t, f.db, `UPDATE posting_windows SET ended_at=clock_timestamp() WHERE id='window'`)
	check(window, 200, true)
	check(post, 404, false)
	mustSQL(t, f.db, `UPDATE posting_windows SET ended_at=NULL,starts_at=clock_timestamp()+interval '1 hour' WHERE id='window'`)
	check(window, 200, true)
	request(t, "POST", f.core.URL+"/api/auth/logout", f.token, "", nil, 204)
	check(window, 401, false)
	check(post, 401, false)
}

func TestEdgeCurrentReadCanPostAndRestart(t *testing.T) {
	f := newEdgeReadFixture(t, "true", "true")
	mustSQL(t, f.db, `DELETE FROM posts`)
	for _, start := range []string{"2025-01-01", "2099-01-01"} {
		mustSQL(t, f.db, `UPDATE posting_windows SET starts_at=$1`, start)
		body := f.compare(t, "/api/posting-window/current", "Bearer "+f.token, "198.51.100.4", 200, true, 1)
		var result getCurrentPostingWindowResponse
		if err := json.Unmarshal(body, &result); err != nil || result.CanPost != (start == "2025-01-01") {
			t.Fatal("start time comparison differs", result, err)
		}
	}
	// Existing cache bodies are optional. Restarting an edge needs no warm state.
	f.edge.Close()
	f.edgeRuntime.Images.Reset()
	f.startEdge(t)
	f.compare(t, "/api/posting-window/current", "Bearer "+f.token, "198.51.100.4", 200, true, 1)
	f.compare(t, f.imageURL, "", "198.51.100.4", 403, false, 0)
}

func TestEdgeReadsSQLErrorFallsBack(t *testing.T) {
	f := newEdgeReadFixture(t, "true", "true")
	f.cacheImage("image")
	// Only edge's SQL connections fail; the same core request must still succeed.
	if err := f.edgeDB.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{f.imageURL, "/api/posting-window/current", "/api/posts/me/current"} {
		f.compare(t, path, "Bearer "+f.token, "198.51.100.5", 200, false, 0)
	}
}

func TestEdgeImageExpiryRecheckedAfterSQL(t *testing.T) {
	f := newEdgeReadFixture(t, "true", "true")
	f.cacheImage("image")
	// Keep the edge SELECT waiting past expiry without changing core behavior.
	tx := f.db.MustBegin()
	defer tx.Rollback()
	tx.MustExec(`LOCK TABLE image_locations IN ACCESS EXCLUSIVE MODE`)
	exp := time.Now().Unix() + 1
	path := buildSignedImageURL(f.owner.ImageKey, "image", exp)
	f.queries.count.Store(0)
	done := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest("GET", f.edge.URL+path, nil)
		req.Header.Set("X-Client-IP", "198.51.100.6")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			done <- 0
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		done <- resp.StatusCode
	}()
	deadline := time.NewTimer(time.Until(time.Unix(exp+1, 0)))
	defer deadline.Stop()
	<-deadline.C
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if status := <-done; status != 403 || f.calls.Load() != 1 || f.queries.count.Load() != 1 {
		t.Fatalf("expired during SQL: status=%d core calls=%d", status, f.calls.Load())
	}
}

// Ensure we test canonical signature formatting rather than a normalized URL.
func TestEdgeImageSignatureFormattingFallback(t *testing.T) {
	f := newEdgeReadFixture(t, "true", "false")
	f.cacheImage("image")
	u, _ := url.Parse(f.imageURL)
	q := u.Query()
	exp, _ := strconv.ParseInt(q.Get("exp"), 10, 64)
	for _, value := range []string{"+" + strconv.FormatInt(exp, 10), "0" + strconv.FormatInt(exp, 10)} {
		q.Set("exp", value)
		u.RawQuery = q.Encode()
		f.compare(t, u.String(), "", "198.51.100.7", 403, false, 0)
	}
	q = u.Query()
	q.Set("exp", strconv.FormatInt(exp, 10))
	q.Set("sig", q.Get("sig")+"=")
	u.RawQuery = q.Encode()
	f.compare(t, u.String(), "", "198.51.100.7", 403, false, 0)
}
