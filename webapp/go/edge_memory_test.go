package isureal

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
)

func memoryEnvelopeFixture() edgeMemoryData {
	profile := "private-profile-image"
	user := User{ID: "owner", Username: "owner", SecretKeyHash: strings.Repeat("ab", 32), DisplayName: "名前<&>", Bio: "bio", ProfileImageID: &profile, Region: "europe", Status: "active"}
	_ = user.setImageKey()
	return edgeMemoryData{Posts: []PostWithAuthor{{Post: Post{ID: "post", AuthorID: user.ID, PostingWindowID: "window", ImageID: "image", Visibility: "private", Caption: "hello<&>", PostLocation: PostLocation{Kind: "none"}, PostedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}, Author: user}}}
}

func TestEdgeMemoryEnvelopeAuthenticatesRequestAndPreservesPrivateFields(t *testing.T) {
	data := memoryEnvelopeFixture()
	body, err := sealMemoryData(data, architectureTestKey, "request-a")
	if err != nil || body == nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Contains(body, []byte(data.Posts[0].Author.SecretKeyHash)) || bytes.Contains(body, data.Posts[0].Author.ImageKey) || bytes.Contains(body, []byte("private-profile-image")) {
		t.Fatal("private model sent in plaintext")
	}
	decoded, err := openMemoryData(body, architectureTestKey, "request-a")
	if err != nil || !reflect.DeepEqual(data, decoded) {
		t.Fatalf("roundtrip: %v", err)
	}
	for _, tc := range []struct {
		key, binding string
		body         []byte
	}{
		{architectureTestKey, "request-b", body}, {architectureTestKey + "wrong", "request-a", body}, {"short", "request-a", body},
		{architectureTestKey, "request-a", body[:10]}, {architectureTestKey, "request-a", append(bytes.Clone(body), 0)},
	} {
		if _, err := openMemoryData(tc.body, tc.key, tc.binding); err == nil {
			t.Fatal("invalid envelope accepted")
		}
	}
	corrupt := bytes.Clone(body)
	corrupt[len(corrupt)-1] ^= 1
	if _, err := openMemoryData(corrupt, architectureTestKey, "request-a"); err == nil {
		t.Fatal("tampered envelope accepted")
	}
	second, err := sealMemoryData(data, architectureTestKey, "request-a")
	if err != nil || bytes.Equal(body, second) {
		t.Fatal("nonce reused")
	}
	data.Posts[0].Author.Bio = strings.Repeat("x", edgeMemoryMaxBytes)
	if body, err := sealMemoryData(data, architectureTestKey, "request-a"); err != nil || body != nil {
		t.Fatalf("large model must use ordinary response: %v", err)
	}
}

func TestEdgeMemoryProofIsBoundAndFeatureIsOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "true"} {
		t.Setenv("ISUREAL_EDGE_MEMORY_RENDERING", value)
		if edgeMemoryRenderingEnabled() != (value == "true") {
			t.Fatal("unexpected default")
		}
	}
	r := httptest.NewRequest("GET", "/api/memories?month=2025-01", nil)
	r.Header.Set("Authorization", "Bearer session-a")
	r.Header.Set(edgeMemoryHeader, edgeMAC(architectureTestKey, "memory-rendering-v1", memoryRequestBinding(r)))
	if !wantsEdgeMemory(r, architectureTestKey) {
		t.Fatal("valid proof rejected")
	}
	for _, change := range []func(*http.Request){
		func(r *http.Request) { r.URL.RawQuery = "month=2025-02" },
		func(r *http.Request) { r.Header.Set("Authorization", "Bearer session-b") },
		func(r *http.Request) { r.Method = "POST" },
		func(r *http.Request) { r.URL.Path = "/api/users" },
	} {
		copy := r.Clone(r.Context())
		change(copy)
		if wantsEdgeMemory(copy, architectureTestKey) {
			t.Fatal("proof used for different request/session")
		}
	}
}

func seedEdgeMemory(t *testing.T, db *sqlx.DB) (User, string) {
	owner, token := fixtureOwner(t, db)
	mustSQL(t, db, `UPDATE users SET profile_image_id='owner-profile',bio='owner bio' WHERE id='owner';
 INSERT INTO users(id,username,secret_key_hash,display_name,bio,region,status,profile_image_id)
 SELECT 'actor'||n,'actor'||n,repeat('cd',32),'名前<&>'||n,'bio'||n,'america',CASE WHEN n=53 THEN 'deleted' ELSE 'active' END,'profile-'||n FROM generate_series(1,55)n;
 INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline,ended_at)
 VALUES('memory-window','europe','2025-01-01','2025-01-01','2025-01-02','2025-01-02');
 INSERT INTO posts(id,author_id,posting_window_id,image_id,caption,visibility,location_kind,posted_at,deleted_at)
 SELECT 'p'||lpad(n::text,3,'0'),'owner','memory-window','image-'||n,'caption'||n,'friends','none',TIMESTAMPTZ '2025-01-01'+n*interval '1 second',CASE WHEN n=54 THEN now() END FROM generate_series(1,54)n;
 INSERT INTO realmojis(id,post_id,reactor_id,image_id,reaction_type,created_at,deleted_at,replaced_at)
 SELECT 'r'||lpad(n::text,3,'0'),'p053','actor'||n,'reaction-image-'||n,'smile',TIMESTAMPTZ '2025-01-01'+(n/3)*interval '1 second',CASE WHEN n=54 THEN now() END,CASE WHEN n=55 THEN now() END FROM generate_series(1,55)n;
 INSERT INTO comments(id,post_id,author_id,body,created_at,deleted_at)
 SELECT 'c'||lpad(n::text,3,'0'),'p053','actor'||n,'body<&>'||n,TIMESTAMPTZ '2025-01-01'+(n/3)*interval '1 second',CASE WHEN n=54 THEN now() END FROM generate_series(1,55)n`)
	return owner, token
}

func memoryGet(t *testing.T, base, path, token, proof string) (int, string, []byte) {
	t.Helper()
	r, err := http.NewRequest("GET", base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if proof == "valid" {
		r.Header.Set(edgeMemoryHeader, edgeMAC(architectureTestKey, "memory-rendering-v1", memoryRequestBinding(r)))
	} else if proof != "" {
		r.Header.Set(edgeMemoryHeader, proof)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), body
}

func TestEdgeMemoryMatchesOriginalThroughProxy(t *testing.T) {
	t.Setenv("ISUREAL_EDGE_MEMORY_RENDERING", "true")
	db, _ := architectureDB(t)
	_, token := seedEdgeMemory(t, db)
	runtime := localRuntime(t, db, t.TempDir())
	core := httptest.NewServer(NewRegionServer(&Config{Region: "europe", EdgeKey: architectureTestKey}, runtime))
	defer core.Close()
	edgeEcho := echo.New()
	edgeEcho.HTTPErrorHandler = APIErrorHandler
	edgeEcho.Any("/api/*", newEdgeProxy(&Config{EdgeUpstream: core.URL, EdgeKey: architectureTestKey}, NewImageCache(edgeImageCacheMaxBytes)))
	edge := httptest.NewServer(edgeEcho)
	defer edge.Close()
	paths := []string{"/api/memories?month=2025-01", "/api/memories/2025-01-01", "/api/memories?month=2025-02", "/api/memories?month=bad", "/api/memories?month=2025-01&beforeId=x", "/api/memories?month=2025-01&beforePostedAt=2025-01-01T00%3A00%3A10Z&beforeId=p010"}
	compare := func(path string, wantToken string) []byte {
		t.Helper()
		status, ctype, want := memoryGet(t, core.URL, path, wantToken, "")
		gotStatus, gotType, got := memoryGet(t, edge.URL, path, wantToken, "forged")
		if status != gotStatus || ctype != gotType {
			t.Fatalf("response differs for %s: %d/%s vs %d/%s", path, status, ctype, gotStatus, gotType)
		}
		var a, b any
		if err := json.Unmarshal(want, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(got, &b); err != nil {
			t.Fatalf("not public JSON: %v", err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("response differs for %s", path)
		}
		if bytes.Contains(got, []byte(strings.Repeat("ab", 32))) || bytes.Contains(got, []byte("ImageKey")) || bytes.Contains(got, []byte("SecretKeyHash")) {
			t.Fatal("private fields exposed")
		}
		return got
	}
	for _, path := range paths {
		compare(path, token)
	}
	compare(paths[0], "")
	compare(paths[0], "invalid")
	// The internal core path does no HMAC signing, including profile images.
	before := signedImageURLs.hits.Load() + signedImageURLs.misses.Load()
	status, ctype, sealed := memoryGet(t, core.URL, paths[0], token, "valid")
	if status != 200 || ctype != edgeMemoryContentType {
		t.Fatalf("offload path not used: %d %s", status, ctype)
	}
	if after := signedImageURLs.hits.Load() + signedImageURLs.misses.Load(); after != before {
		t.Fatal("core signed images while offloading")
	}
	r := httptest.NewRequest("GET", paths[0], nil)
	r.Header.Set("Authorization", "Bearer "+token)
	data, err := openMemoryData(sealed, architectureTestKey, memoryRequestBinding(r))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Posts) != 50 || !data.HasMore || len(data.Reactions) != 50 || len(data.Comments) != 50 {
		t.Fatalf("pagination/interaction limits differ: posts=%d reactions=%d comments=%d", len(data.Posts), len(data.Reactions), len(data.Comments))
	}
	// Invalid public proof must retain the ordinary API, with no private envelope.
	if status, ctype, _ := memoryGet(t, core.URL, paths[0], token, "forged"); status != 200 || ctype == edgeMemoryContentType {
		t.Fatal("public request obtained model")
	}
	// Current DB changes are observed on every request; no response/model cache.
	mustSQL(t, db, `UPDATE realmojis SET deleted_at=clock_timestamp() WHERE id='r051'; UPDATE comments SET deleted_at=clock_timestamp() WHERE id='c001'; UPDATE posts SET deleted_at=clock_timestamp() WHERE id='p052'; UPDATE users SET profile_image_id=NULL,secret_key_hash=repeat('ef',32) WHERE id='actor1'`)
	compare(paths[0], token)
	mustSQL(t, db, `UPDATE users SET region='asia' WHERE id='owner'`)
	compare(paths[0], token)
	mustSQL(t, db, `UPDATE users SET region='europe',status='deleted' WHERE id='owner'`)
	compare(paths[0], token)
	mustSQL(t, db, `UPDATE users SET status='active' WHERE id='owner'`)
	t.Setenv("ISUREAL_EDGE_MEMORY_RENDERING", "false")
	if status, ctype, _ := memoryGet(t, core.URL, paths[0], token, "valid"); status != 200 || ctype == edgeMemoryContentType {
		t.Fatal("disabled core returned internal model")
	}
	compare(paths[0], token)
}

func TestEdgeMemoryNeverForwardsInvalidInternalEnvelope(t *testing.T) {
	t.Setenv("ISUREAL_EDGE_MEMORY_RENDERING", "true")
	for _, mode := range []string{"tampered", "wrong-session", "unmarked"} {
		t.Run(mode, func(t *testing.T) {
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				binding := memoryRequestBinding(r)
				if mode == "wrong-session" {
					binding += "wrong"
				}
				body, err := sealMemoryData(memoryEnvelopeFixture(), architectureTestKey, binding)
				if err != nil {
					t.Error(err)
					return
				}
				if mode == "tampered" {
					body[len(body)-1] ^= 1
				}
				w.Header().Set("Content-Type", edgeMemoryContentType)
				w.Write(body)
			}))
			defer origin.Close()
			e := echo.New()
			e.Any("/api/*", newEdgeProxy(&Config{EdgeUpstream: origin.URL, EdgeKey: architectureTestKey}, NewImageCache(imageCacheMaxBytes)))
			edge := httptest.NewServer(e)
			defer edge.Close()
			path := "/api/memories?month=2025-01"
			if mode == "unmarked" {
				path = "/api/users"
			}
			status, ctype, body := memoryGet(t, edge.URL, path, "token", "")
			if status != 502 || ctype == edgeMemoryContentType || !bytes.Contains(body, []byte("upstream unavailable")) {
				t.Fatal("invalid internal envelope escaped")
			}
		})
	}
}

func TestEdgeMemoryKeepsDatabaseRoundTrips(t *testing.T) {
	db, dsn := architectureDB(t)
	_, _ = seedEdgeMemory(t, db)
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	counter := &readQueryCounter{}
	config.Tracer = counter
	traced := sqlx.NewDb(stdlib.OpenDB(*config), "pgx")
	t.Cleanup(func() { traced.Close() })
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, empty := range []bool{false, true} {
		if empty {
			start = start.AddDate(0, 1, 0)
		}
		counter.count.Store(0)
		if _, err := listMemories(t.Context(), traced, "owner", start, start.AddDate(0, 1, 0), nil, ""); err != nil {
			t.Fatal(err)
		}
		old := counter.count.Load()
		counter.count.Store(0)
		if _, err := loadMemoryData(t.Context(), traced, "owner", start, start.AddDate(0, 1, 0), nil, ""); err != nil {
			t.Fatal(err)
		}
		want := uint64(3)
		if empty {
			want = 1
		}
		if old != want || counter.count.Load() != want {
			t.Fatalf("empty=%v: SQL count old=%d model=%d want=%d", empty, old, counter.count.Load(), want)
		}
	}
}

func benchmarkMemoryData() edgeMemoryData {
	data := edgeMemoryData{HasMore: true}
	for i := 0; i < 30; i++ {
		p := memoryEnvelopeFixture().Posts[0]
		p.ID = fmt.Sprintf("p-%d", i)
		p.ImageID = fmt.Sprintf("image-%d", i)
		data.Posts = append(data.Posts, p)
		for j := 0; j < 10; j++ {
			u := p.Author
			u.ID = fmt.Sprintf("actor-%d-%d", i, j)
			sum := sha256.Sum256([]byte(u.ID))
			u.SecretKeyHash = hex.EncodeToString(sum[:])
			u.ImageKey = bytes.Clone(sum[:])
			profile := "profile-" + u.ID
			u.ProfileImageID = &profile
			data.Reactions = append(data.Reactions, RealMojiWithReactor{RealMoji: RealMoji{ID: u.ID, PostID: p.ID, ReactorID: u.ID, ImageID: "reaction-" + u.ID, ReactionType: "smile", CreatedAt: p.PostedAt}, Reactor: u})
			data.Comments = append(data.Comments, CommentWithAuthor{Comment: Comment{ID: u.ID, PostID: p.ID, AuthorID: u.ID, Body: "comment", CreatedAt: p.PostedAt}, Author: u})
		}
	}
	return data
}

func BenchmarkEdgeMemoryCoreWork(b *testing.B) {
	data := benchmarkMemoryData()
	for _, mode := range []string{"normal_cold", "envelope", "normal_warm"} {
		b.Run(mode, func(b *testing.B) {
			signedImageURLs.reset()
			renderMemoryData(data)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if mode == "normal_cold" {
					signedImageURLs.reset()
				}
				var body []byte
				var err error
				if mode == "envelope" {
					body, err = sealMemoryData(data, architectureTestKey, "binding")
				} else {
					body, err = json.Marshal(renderMemoryData(data))
				}
				if err != nil {
					b.Fatal(err)
				}
				b.SetBytes(int64(len(body)))
			}
		})
	}
}

func BenchmarkEdgeMemoryReceivingWork(b *testing.B) {
	data := benchmarkMemoryData()
	body, err := sealMemoryData(data, architectureTestKey, "binding")
	if err != nil {
		b.Fatal(err)
	}
	for _, mode := range []string{"cold", "warm"} {
		b.Run(mode, func(b *testing.B) {
			signedImageURLs.reset()
			renderMemoryData(data)
			b.ReportAllocs()
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for range b.N {
				if mode == "cold" {
					signedImageURLs.reset()
				}
				decoded, err := openMemoryData(body, architectureTestKey, "binding")
				if err != nil {
					b.Fatal(err)
				}
				if _, err := json.Marshal(renderMemoryData(decoded)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Verify that signed URLs remain ordinary canonical URLs after rendering.
func TestEdgeMemoryURLsUseOriginalSignature(t *testing.T) {
	data := memoryEnvelopeFixture()
	// Deliberately different from the edge clock. The core's expiry must win.
	data.ExpiresAt = imageURLExpiry(time.Now().Add(-2*time.Minute), imageURLExpiryBucket)
	user := data.Posts[0].Author
	data.Reactions = []RealMojiWithReactor{{RealMoji: RealMoji{ID: "reaction", PostID: "post", ImageID: "reaction-image"}, Reactor: user}}
	data.Comments = []CommentWithAuthor{{Comment: Comment{ID: "comment", PostID: "post"}, Author: user}}
	response := renderMemoryData(data)
	item := response.Items[0]
	for _, raw := range []string{item.ImageURL, *item.Author.ProfileImageURL, item.RealMojis[0].ImageURL, *item.RealMojis[0].Reactor.ProfileImageURL, *item.Comments[0].Author.ProfileImageURL} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(u.Path, "/api/images/") || len(u.Query()["sig"]) != 1 || len(u.Query()["exp"]) != 1 {
			t.Fatalf("noncanonical image URL: %q", raw)
		}
		exp, err := strconv.ParseInt(u.Query().Get("exp"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if exp != data.ExpiresAt {
			t.Fatal("edge replaced core expiry")
		}
		sig, err := base64.RawURLEncoding.Strict().DecodeString(u.Query().Get("sig"))
		if err != nil {
			t.Fatal(err)
		}
		want := imageSignature(data.Posts[0].Author.ImageKey, strings.TrimPrefix(u.Path, "/api/images/"), exp)
		if !hmac.Equal(sig, want) {
			t.Fatal("edge signature differs from original HMAC")
		}
	}
}
