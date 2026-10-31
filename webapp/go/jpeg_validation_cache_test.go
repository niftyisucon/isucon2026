package isureal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/labstack/echo/v5"
)

func validationJPEG(t testing.TB, width, height int) []byte {
	t.Helper()
	img := image.NewYCbCr(image.Rect(0, 0, width, height), image.YCbCrSubsampleRatio420)
	seed := uint32(1)
	for _, plane := range [][]byte{img.Y, img.Cb, img.Cr} {
		for i := range plane {
			seed = 1664525*seed + 1013904223
			plane[i] = byte(seed >> 24)
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestJPEGValidationCacheMatchesFullDecoder(t *testing.T) {
	valid := validationJPEG(t, 32, 17)
	corpus := [][]byte{nil, []byte("not jpeg"), valid, append(bytes.Clone(valid), []byte("trailing bytes")...), validationJPEG(t, 512, 1), validationJPEG(t, 513, 1), validationJPEG(t, 1, 513), validationJPEG(t, 4097, 1)}
	for _, n := range []int{1, 2, 20, len(valid) / 2, len(valid) - 15, len(valid) - 1} {
		corpus = append(corpus, valid[:n])
	}
	// Header parsing can succeed while the entropy stream is incomplete.
	truncated := valid[:len(valid)-15]
	if _, err := jpeg.DecodeConfig(bytes.NewReader(truncated)); err != nil {
		t.Fatal("fixture must have an intact header")
	}
	if decodeJPEGUncached(truncated, 512) == nil {
		t.Fatal("fixture must fail full decode")
	}
	for i := 0; i < len(valid); i += 31 {
		mutated := bytes.Clone(valid)
		mutated[i] ^= 0xff
		corpus = append(corpus, mutated)
	}
	var cache jpegValidationCache
	for i, data := range corpus {
		for _, limit := range []int{0, 16, 32, 512, 4096} {
			want := decodeJPEGUncached(data, limit)
			for range 2 {
				if got := cache.validate(data, limit); fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("case %d limit %d: got %v, want %v", i, limit, got, want)
				}
			}
		}
	}
}

func TestJPEGValidationCacheRequiresExactBytesAndLimit(t *testing.T) {
	var cache jpegValidationCache
	data := validationJPEG(t, 32, 17)
	original := bytes.Clone(data)
	if err := cache.validate(data, 512); err != nil {
		t.Fatal(err)
	}
	if cache.stats()["bytes"] != 0 {
		t.Fatal("first occurrence copied the image")
	}
	if err := cache.validate(data, 512); err != nil {
		t.Fatal(err)
	}
	if err := cache.validate(bytes.Clone(data), 512); err != nil || cache.hits.Load() != 1 {
		t.Fatal("identical separately allocated input did not hit")
	}
	if err := cache.validate(data, 16); err != errImageDimensions {
		t.Fatal("cached success bypassed a smaller dimension limit")
	}
	data[0] = 0
	key := jpegValidationKey{digest: sha256.Sum256(original), limit: 512}
	if !bytes.Equal(cache.shards[key.digest[0]%jpegValidationShards].data[key], original) {
		t.Fatal("cache retained caller-owned mutable input")
	}
	if err := cache.validate(data, 512); err == nil {
		t.Fatal("mutated input accepted")
	}
	// Simulate a hash collision: a matching digest is not sufficient proof.
	invalid := []byte("invalid JPEG with a simulated colliding digest")
	key = jpegValidationKey{digest: sha256.Sum256(invalid), limit: 512}
	cache.put(key, original, cache.generation.Load())
	cache.put(key, original, cache.generation.Load())
	if err := cache.validate(invalid, 512); err == nil {
		t.Fatal("digest collision accepted without byte equality")
	}
}

func TestJPEGValidationCacheBoundsAndReset(t *testing.T) {
	var cache jpegValidationCache
	for i := range jpegValidationShardEntries + 1 {
		cache.put(jpegValidationKey{limit: i}, []byte{1}, cache.generation.Load())
		cache.put(jpegValidationKey{limit: i}, []byte{1}, cache.generation.Load())
	}
	if n := cache.stats()["entries"]; n > jpegValidationShardEntries {
		t.Fatalf("entry cap exceeded: %d", n)
	}
	large := make([]byte, jpegValidationShardBytes*3/4)
	for i := range 3 {
		cache.put(jpegValidationKey{limit: 1000 + i}, large, cache.generation.Load())
		cache.put(jpegValidationKey{limit: 1000 + i}, large, cache.generation.Load())
	}
	if size := cache.stats()["bytes"]; size > jpegValidationShardBytes {
		t.Fatalf("single-shard byte cap exceeded: %d", size)
	}
	for i := range jpegValidationShardCandidates + 1 {
		cache.put(jpegValidationKey{limit: 2000 + i}, []byte{1}, cache.generation.Load())
	}
	if n := cache.stats()["candidates"]; n > jpegValidationShardCandidates {
		t.Fatalf("candidate cap exceeded: %d", n)
	}
	generation := cache.generation.Load()
	cache.reset()
	cache.put(jpegValidationKey{}, []byte{1}, generation)
	if s := cache.stats(); s["entries"] != 0 || s["bytes"] != 0 || s["candidates"] != 0 {
		t.Fatal("a pre-reset decode repopulated the cache")
	}
	data := append(validationJPEG(t, 1, 1), make([]byte, jpegValidationShardBytes)...)
	if err := cache.validate(data, 512); err != nil || cache.bypasses.Load() != 1 || cache.stats()["entries"] != 0 {
		t.Fatal("large input did not retain the original uncached path")
	}
}

func TestJPEGValidationCacheConcurrentReset(t *testing.T) {
	var cache jpegValidationCache
	data := validationJPEG(t, 64, 64)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 40 {
				if err := cache.validate(data, 512); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Go(func() {
		for range 40 {
			cache.reset()
		}
	})
	wg.Wait()
	cache.reset()
	if cache.stats()["bytes"] != 0 {
		t.Fatal("reset retained data")
	}
}

func TestJPEGValidationCachePreservesProfileAbuseAndAuth(t *testing.T) {
	validatedJPEGs.reset()
	t.Cleanup(validatedJPEGs.reset)
	data := validationJPEG(t, 32, 17)
	if err := decodeJPEG(data, 512); err != nil {
		t.Fatal(err)
	}
	h := &HandlerContext{}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	e.PUT("/api/me/profile-image", putProfileImageHandler(h), SessionMiddleware(h, true))
	req := httptest.NewRequest("PUT", "/api/me/profile-image", bytes.NewReader(data))
	req.Header.Set("Content-Type", "image/jpeg")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("cached JPEG bypassed authentication: %d", rec.Code)
	}
	wide := validationJPEG(t, 513, 1)
	if err := decodeJPEG(wide, 4096); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		data  []byte
		abuse bool
	}{{wide, false}, {data[:len(data)-15], true}, {[]byte("invalid"), true}} {
		req := httptest.NewRequest("PUT", "/api/me/profile-image", bytes.NewReader(tc.data))
		req.Header.Set("Content-Type", "image/jpeg")
		ctx := e.NewContext(req, httptest.NewRecorder())
		if err := putProfileImageHandler(h)(ctx); err == nil || (ctx.Get("abuse") == "invalid_image") != tc.abuse {
			t.Fatal("invalid JPEG/dimension failure changed the abuse classification")
		}
	}
}

func TestJPEGValidationCacheAcrossEdgeAndInitialize(t *testing.T) {
	validatedJPEGs.reset()
	t.Cleanup(validatedJPEGs.reset)
	data := validationJPEG(t, 32, 17)
	beforeHits := validatedJPEGs.hits.Load()
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/initialize" {
			return
		}
		body, _ := io.ReadAll(r.Body)
		e := echo.New()
		ctx := e.NewContext(r, w)
		if err := validateJPEG(ctx, &HandlerContext{Config: &Config{EdgeKey: architectureTestKey}}, body, 512); err != nil {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(204)
	}))
	defer core.Close()
	edge := echo.New()
	edge.Any("/api/*", newEdgeProxy(&Config{EdgeUpstream: core.URL, EdgeKey: architectureTestKey}, NewImageCache(imageCacheMaxBytes)))
	for _, body := range [][]byte{data, bytes.Clone(data), bytes.Clone(data), []byte("invalid")} {
		req := httptest.NewRequest("PUT", "/api/me/profile-image", bytes.NewReader(body))
		req.Header.Set("Content-Type", "image/jpeg")
		// A client-supplied proof must still be stripped by the edge.
		req.Header.Set(validationHeader, edgeMAC(architectureTestKey, "jpeg", imageValidationValue(data, 512)))
		rec := httptest.NewRecorder()
		edge.ServeHTTP(rec, req)
		want := 204
		if bytes.Equal(body, []byte("invalid")) {
			want = 400
		}
		if rec.Code != want {
			t.Fatalf("edge response %d, want %d", rec.Code, want)
		}
	}
	if jpegValidationCacheEnabled && validatedJPEGs.hits.Load() == beforeHits {
		t.Fatal("edge never reused its successful decode")
	}
	rec := httptest.NewRecorder()
	edge.ServeHTTP(rec, httptest.NewRequest("POST", "/api/initialize", nil))
	if rec.Code != 200 || validatedJPEGs.stats()["entries"] != 0 {
		t.Fatal("successful initialize did not clear edge JPEG proofs")
	}
}

func BenchmarkJPEGValidation(b *testing.B) {
	for _, size := range []int{512, 1024} {
		data := validationJPEG(b, size, size)
		for _, repeated := range []bool{true, false} {
			for _, cached := range []bool{false, true} {
				b.Run(fmt.Sprintf("size=%d/repeated=%t/cached=%t", size, repeated, cached), func(b *testing.B) {
					var cache jpegValidationCache
					// A JPEG comment changes exact bytes without changing decoder work.
					input := append([]byte{0xff, 0xd8, 0xff, 0xfe, 0, 10}, make([]byte, 8)...)
					input = append(input, data[2:]...)
					if cached && repeated {
						for range 2 {
							if err := cache.validate(input, 4096); err != nil {
								b.Fatal(err)
							}
						}
					}
					b.ReportAllocs()
					b.SetBytes(int64(len(input)))
					b.ResetTimer()
					for i := range b.N {
						if !repeated {
							binary.BigEndian.PutUint64(input[6:14], uint64(i))
						}
						var err error
						if cached {
							err = cache.validate(input, 4096)
						} else {
							err = decodeJPEGUncached(input, 4096)
						}
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}
