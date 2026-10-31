package isureal

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/labstack/echo/v5"
)

const edgeUploadTestURL = "/api/images/uploaded-image?exp=9999999999&sig=opaque"

func edgeUploadForm(t *testing.T, field string, data []byte) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	f, err := w.CreateFormFile(field, "image.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = w.WriteField("visibility", "public"); err != nil {
		t.Fatal(err)
	}
	if err = w.WriteField("location_kind", "none"); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), w.FormDataContentType()
}

func edgeUploadTestProxy(t *testing.T, cache *ImageCache, upstream http.HandlerFunc) *echo.Echo {
	t.Helper()
	core := httptest.NewServer(upstream)
	t.Cleanup(core.Close)
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.Any("/api/*", newEdgeProxy(&Config{EdgeUpstream: core.URL, EdgeKey: architectureTestKey}, cache))
	return e
}

func edgeUploadRequest(e *echo.Echo, method, path, contentType string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	// Keep encoded responses encoded so the proxy's Content-Encoding guard is exercised.
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	return w
}

func TestEdgeUploadCacheRevalidatesAndPreservesBytes(t *testing.T) {
	for _, flag := range []string{"", "true", "false"} {
		t.Run("flag="+flag, func(t *testing.T) {
			t.Setenv("ISUREAL_EDGE_UPLOAD_IMAGE_CACHE", flag)
			// Accepted trailing bytes must be preserved, rather than re-encoding the JPEG.
			data := append(jpegFixture(t, 16), []byte("trailing bytes\x00\xff")...)
			raw, contentType := edgeUploadForm(t, "image", data)
			response := []byte(" {\n \"imageUrl\" : \"" + edgeUploadTestURL + "\", \"caption\":\"写真\" }\n")
			cache := NewImageCache(imageCacheLimit(&Config{EdgeUpstream: "http://core"}))
			var bodies, checks atomic.Int64
			var forbidden atomic.Bool
			e := edgeUploadTestProxy(t, cache, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					forwarded, err := io.ReadAll(r.Body)
					if err != nil || !bytes.Equal(forwarded, raw) {
						t.Error("forwarded multipart bytes differ")
					}
					r.Body = io.NopCloser(bytes.NewReader(forwarded))
					// Use the same binder and file read as createPostHandler.
					var input createPostRequest
					if err := echo.New().NewContext(r, w).Bind(&input); err != nil || input.Image == nil {
						t.Error("core could not bind image")
						w.WriteHeader(400)
						return
					}
					defer r.MultipartForm.RemoveAll()
					f, err := input.Image.Open()
					if err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					actual, err := io.ReadAll(f)
					f.Close()
					if err != nil || !bytes.Equal(actual, data) || !validEdgeMAC(architectureTestKey, "jpeg", imageValidationValue(actual, 4096), r.Header.Get(validationHeader)) {
						t.Error("core image bytes or JPEG proof differ")
					}
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Length", strconv.Itoa(len(response)))
					w.WriteHeader(201)
					w.Write(response)
					return
				}
				checks.Add(1)
				w.Header().Set("Cache-Control", "no-store")
				if forbidden.Load() {
					w.WriteHeader(403)
					w.Write([]byte("{\"error\":\"image_forbidden\"}\n"))
					return
				}
				if hasCachedImageProof(r, architectureTestKey) {
					w.WriteHeader(204)
					return
				}
				bodies.Add(1)
				w.Header().Set("Content-Type", "image/jpeg")
				w.Write(data)
			})
			posted := edgeUploadRequest(e, "POST", "/api/posts", contentType, raw)
			if posted.Code != 201 || !bytes.Equal(posted.Body.Bytes(), response) || posted.Header().Get("Content-Length") != strconv.Itoa(len(response)) {
				t.Fatal("post response changed")
			}
			cached := cache.Get("uploaded-image")
			if flag == "false" {
				if cached != nil {
					t.Fatal("disabled upload cache was populated")
				}
			} else if cached == nil || cached.OwnerID != "" || cached.MediaType != "image/jpeg" || !bytes.Equal(cached.Data, data) {
				t.Fatal("upload was not cached with the exact bytes and GET metadata")
			}
			for range 2 {
				got := edgeUploadRequest(e, "GET", edgeUploadTestURL, "", nil)
				if got.Code != 200 || !bytes.Equal(got.Body.Bytes(), data) || got.Header().Get("Content-Type") != "image/jpeg" || got.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("GET did not return identical bytes after core verification")
				}
			}
			wantBodies := int64(0)
			if flag == "false" {
				wantBodies = 1
			}
			if bodies.Load() != wantBodies || checks.Load() != 2 {
				t.Fatalf("full bodies/checks=%d/%d, want %d/2", bodies.Load(), checks.Load(), wantBodies)
			}
			forbidden.Store(true)
			for _, reason := range []string{"deleted", "not visible"} {
				got := edgeUploadRequest(e, "GET", edgeUploadTestURL, "", nil)
				if got.Code != 403 || got.Body.String() != "{\"error\":\"image_forbidden\"}\n" || got.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("core rejection changed: %s", reason)
				}
			}
			if checks.Load() != 4 {
				t.Fatal("cached bytes bypassed core verification")
			}
		})
	}
}

func TestEdgeUploadCacheSkipsFailuresWithoutChangingResponse(t *testing.T) {
	t.Setenv("ISUREAL_EDGE_UPLOAD_IMAGE_CACHE", "")
	valid := jpegFixture(t, 16)
	response := []byte(`{"imageUrl":"` + edgeUploadTestURL + `"}`)
	for _, tc := range []struct {
		name, method, path, field, encoding string
		status                              int
		image, response                     []byte
	}{
		{name: "200", status: 200},
		{name: "400", status: 400},
		{name: "403", status: 403},
		{name: "409", status: 409},
		{name: "500", status: 500},
		{name: "invalid JSON", response: []byte(`{"imageUrl":`)},
		{name: "missing URL", response: []byte(`{"id":"post"}`)},
		{name: "empty URL", response: []byte(`{"imageUrl":""}`)},
		{name: "wrong URL type", response: []byte(`{"imageUrl":123}`)},
		{name: "invalid URL", response: []byte(`{"imageUrl":"%zz"}`)},
		{name: "wrong URL path", response: []byte(`{"imageUrl":"/api/posts/other"}`)},
		{name: "empty image ID", response: []byte(`{"imageUrl":"/api/images/"}`)},
		{name: "nested image path", response: []byte(`{"imageUrl":"/api/images/id/extra"}`)},
		{name: "external URL", response: []byte(`{"imageUrl":"http://example.invalid/api/images/id"}`)},
		{name: "encoded response", encoding: "gzip"},
		{name: "invalid JPEG", image: []byte("not jpeg")},
		{name: "truncated JPEG", image: valid[:len(valid)-15]},
		{name: "oversized dimensions", image: validationJPEG(t, 4097, 1)},
		{name: "missing image field", field: "other"},
		{name: "malformed multipart"},
		{name: "RealMoji", path: "/api/posts/post/realmojis"},
		{name: "profile", method: "PUT", path: "/api/me/profile-image"},
		{name: "other method", method: "PUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.status == 0 {
				tc.status = 201
			}
			if tc.method == "" {
				tc.method = "POST"
			}
			if tc.path == "" {
				tc.path = "/api/posts"
			}
			if tc.field == "" {
				tc.field = "image"
			}
			if tc.image == nil {
				tc.image = valid
			}
			if tc.response == nil {
				tc.response = response
			}
			raw, contentType := edgeUploadForm(t, tc.field, tc.image)
			if tc.name == "malformed multipart" {
				raw = []byte("invalid multipart body")
			}
			if tc.name == "profile" {
				raw, contentType = tc.image, "image/jpeg"
			}
			cache := NewImageCache(imageCacheMaxBytes)
			e := edgeUploadTestProxy(t, cache, func(w http.ResponseWriter, r *http.Request) {
				actual, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(actual, raw) {
					t.Error("request changed")
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", tc.encoding)
				w.Header().Set("Content-Length", strconv.Itoa(len(tc.response)))
				w.WriteHeader(tc.status)
				w.Write(tc.response)
			})
			got := edgeUploadRequest(e, tc.method, tc.path, contentType, raw)
			if got.Code != tc.status || !bytes.Equal(got.Body.Bytes(), tc.response) || got.Header().Get("Content-Encoding") != tc.encoding || got.Header().Get("Content-Length") != strconv.Itoa(len(tc.response)) {
				t.Fatal("response changed")
			}
			if cache.bytes.Load() != 0 || cache.Get("uploaded-image") != nil {
				t.Fatal("unsuitable upload or response populated cache")
			}
		})
	}
}

func TestEdgeUploadCacheSkipsTransparentlyDecodedResponse(t *testing.T) {
	t.Setenv("ISUREAL_EDGE_UPLOAD_IMAGE_CACHE", "")
	response := []byte(`{"imageUrl":"` + edgeUploadTestURL + `"}`)
	var encoded bytes.Buffer
	w := gzip.NewWriter(&encoded)
	if _, err := w.Write(response); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	cache := NewImageCache(imageCacheMaxBytes)
	e := edgeUploadTestProxy(t, cache, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(201)
		w.Write(encoded.Bytes())
	})
	raw, contentType := edgeUploadForm(t, "image", jpegFixture(t, 16))
	r := httptest.NewRequest("POST", "/api/posts", bytes.NewReader(raw))
	r.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	if rec.Code != 201 || !bytes.Equal(rec.Body.Bytes(), response) || cache.Get("uploaded-image") != nil {
		t.Fatal("transparent transport decompression changed response or populated cache")
	}
}

func TestEdgeUploadCacheOversizedResponseRemainsIntact(t *testing.T) {
	data := []byte(`{"imageUrl":"` + edgeUploadTestURL + `","padding":"` + strings.Repeat("x", edgeMaxBody) + `"}`)
	body := io.NopCloser(bytes.NewReader(data))
	resp := &http.Response{Body: body, ContentLength: int64(len(data))}
	cache := NewImageCache(imageCacheMaxBytes)
	cacheUploadedPostImage(resp, cache, edgeCacheSnapshot{upload: jpegFixture(t, 16)})
	got, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || !bytes.Equal(got, data) || resp.ContentLength != int64(len(data)) || cache.Get("uploaded-image") != nil {
		t.Fatal("response inspection limit changed response or populated cache")
	}
}

func TestEdgeUploadCacheInitializeDiscardsInflightPost(t *testing.T) {
	t.Setenv("ISUREAL_EDGE_UPLOAD_IMAGE_CACHE", "")
	cache := NewImageCache(imageCacheMaxBytes)
	started, release := make(chan struct{}, 1), make(chan struct{})
	e := edgeUploadTestProxy(t, cache, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/initialize" {
			w.Write([]byte(`{"language":"go"}`))
			return
		}
		started <- struct{}{}
		<-release
		w.WriteHeader(201)
		w.Write([]byte(`{"imageUrl":"` + edgeUploadTestURL + `"}`))
	})
	raw, contentType := edgeUploadForm(t, "image", jpegFixture(t, 16))
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- edgeUploadRequest(e, "POST", "/api/posts", contentType, raw) }()
	<-started
	initialized := edgeUploadRequest(e, "POST", "/api/initialize", "", nil)
	close(release)
	posted := <-done
	if initialized.Code != 200 || posted.Code != 201 || cache.Get("uploaded-image") != nil {
		t.Fatal("pre-initialize post repopulated cache or response changed")
	}
	if got := edgeUploadRequest(e, "POST", "/api/posts", contentType, raw); got.Code != 201 || cache.Get("uploaded-image") == nil {
		t.Fatal("new-generation post was not cached")
	}
}

func TestImageCacheLimitsForCoreAndEdge(t *testing.T) {
	for _, tc := range []struct {
		config Config
		limit  int64
	}{
		{Config{}, 256 << 20},
		{Config{Region: "europe"}, 256 << 20},
		{Config{ListenRegions: "asia:8080,europe:8081,america:8082"}, 256 << 20},
		{Config{Region: "europe", EdgeUpstream: "http://core:8081"}, 1 << 30},
		{Config{Region: "america", EdgeUpstream: "http://core:8082"}, 1 << 30},
	} {
		cache := NewImageCache(imageCacheLimit(&tc.config))
		if cache.maxBytes != tc.limit {
			t.Fatalf("region=%s limit=%d want=%d", tc.config.Region, cache.maxBytes, tc.limit)
		}
		cache.Reset()
		if cache.maxBytes != tc.limit {
			t.Fatal("reset changed capacity")
		}
	}
	// Exercise the configured limit without allocating a GiB for the test.
	cache := NewImageCache(5)
	cache.Put("a", &cachedImage{Data: []byte("123")})
	cache.Put("b", &cachedImage{Data: []byte("45")})
	if cache.Get("a") == nil || cache.Get("b") == nil {
		t.Fatal("cache evicted at the limit")
	}
	cache.Put("too large", &cachedImage{Data: []byte("123456")})
	if cache.Get("too large") != nil || cache.Get("a") == nil {
		t.Fatal("oversized image changed cache")
	}
	cache.Put("c", &cachedImage{Data: []byte("6")})
	if cache.Get("a") != nil || cache.Get("b") != nil || cache.Get("c") == nil || cache.bytes.Load() != 1 {
		t.Fatal("cache did not clear when configured capacity was exceeded")
	}
}

func TestEdgeUploadCacheWithCoreLocal(t *testing.T) {
	t.Setenv("ISUREAL_EDGE_UPLOAD_IMAGE_CACHE", "")
	for _, fileMode := range []bool{false, true} {
		t.Run(strconv.FormatBool(fileMode), func(t *testing.T) {
			db, _ := architectureDB(t)
			_, token := fixtureOwner(t, db)
			mustSQL(t, db, `INSERT INTO posting_windows(id,region,local_date,starts_at,on_time_deadline) VALUES('window','europe',current_date,clock_timestamp()-interval '1 minute',clock_timestamp()+interval '1 day')`)
			runtime := localRuntime(t, db, t.TempDir())
			if !fileMode {
				runtime.Files = nil
			}
			core := NewRegionServer(&Config{Region: "europe", EdgeKey: architectureTestKey}, runtime)
			statuses := make(chan int, 4)
			postBody := make(chan []byte, 1)
			cache := NewImageCache(edgeImageCacheMaxBytes)
			e := edgeUploadTestProxy(t, cache, func(w http.ResponseWriter, r *http.Request) {
				rec := httptest.NewRecorder()
				core.ServeHTTP(rec, r)
				if r.Method == "GET" {
					statuses <- rec.Code
					if rec.Code == 204 && rec.Body.Len() != 0 {
						t.Error("core transferred bytes on metadata-only GET")
					}
				} else if r.Method == "POST" {
					postBody <- bytes.Clone(rec.Body.Bytes())
				}
				for k, v := range rec.Header() {
					w.Header()[k] = v
				}
				w.WriteHeader(rec.Code)
				w.Write(rec.Body.Bytes())
			})
			data := append(jpegFixture(t, 16), []byte("exact persisted trailing bytes")...)
			raw, contentType := edgeUploadForm(t, "image", data)
			r := httptest.NewRequest("POST", "/api/posts", bytes.NewReader(raw))
			r.Header.Set("Content-Type", contentType)
			r.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, r)
			if rec.Code != 201 || !bytes.Equal(rec.Body.Bytes(), <-postBody) {
				t.Fatal("core post response changed")
			}
			var posted createPostResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &posted); err != nil {
				t.Fatal(err)
			}
			id := posted.ImageURLID()
			if cached := cache.Get(id); cached == nil || !bytes.Equal(cached.Data, data) {
				t.Fatal("successful core post did not populate edge cache")
			}
			var object struct {
				Data []byte  `db:"data"`
				Key  *string `db:"storage_key"`
			}
			if err := db.Get(&object, `SELECT data,storage_key FROM image_objects WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
			stored, err := (&HandlerContext{Files: runtime.Files}).imageBytes(object.Data, object.Key)
			if err != nil || !bytes.Equal(stored, data) {
				t.Fatal("core persisted different bytes")
			}
			// Also exercise the core cache-miss metadata-only query.
			runtime.Images.Reset()
			got := edgeUploadRequest(e, "GET", posted.ImageURL, "", nil)
			if got.Code != 200 || !bytes.Equal(got.Body.Bytes(), data) || <-statuses != 204 {
				t.Fatal("first GET did not use core 204 with identical bytes")
			}
			mustSQL(t, db, `UPDATE posts SET deleted_at=clock_timestamp() WHERE id=$1`, posted.ID)
			got = edgeUploadRequest(e, "GET", posted.ImageURL, "", nil)
			if got.Code != 403 || <-statuses != 403 {
				t.Fatal("deleted post returned cached bytes")
			}
			mustSQL(t, db, `UPDATE posts SET deleted_at=NULL WHERE id=$1`, posted.ID)
			mustSQL(t, db, `UPDATE users SET status='deleted',deleted_at=clock_timestamp() WHERE id='owner'`)
			got = edgeUploadRequest(e, "GET", posted.ImageURL, "", nil)
			if got.Code != 403 || <-statuses != 403 {
				t.Fatal("invisible image returned cached bytes")
			}
		})
	}
}
