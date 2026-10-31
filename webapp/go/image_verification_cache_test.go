package isureal

import (
	"bytes"
	"crypto/hmac"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

func TestImageVerificationSharesOnlyExpectedSignature(t *testing.T) {
	var cache imageURLCache
	key := bytes.Repeat([]byte{1}, 32)
	id, exp := "image /?画像", int64(1793437001)
	issued := cache.get(key, id, exp)
	u, err := url.Parse(issued)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(u.Query().Get("sig"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sig, referenceSignature(key, id, exp)) {
		t.Fatal("issued signature changed")
	}
	if !cache.verify(key, id, exp, sig) {
		t.Fatal("issued signature rejected")
	}
	if stats := cache.stats(); stats["verification_hits"] != 1 || stats["verification_misses"] != 0 || stats["hits"] != 0 || stats["misses"] != 1 {
		t.Fatalf("verification did not reuse issuance or mixed counters: %v", stats)
	}
	bad := bytes.Clone(sig)
	bad[0] ^= 1
	for _, candidate := range [][]byte{nil, bad, append(bytes.Clone(sig), 0)} {
		if cache.verify(key, id, exp, candidate) {
			t.Fatal("invalid signature accepted on warm cache")
		}
	}
	for _, input := range []struct {
		key []byte
		id  string
		exp int64
	}{
		{bytes.Repeat([]byte{2}, 32), id, exp}, {key, id + "other", exp}, {key, id, exp + 1},
		{nil, id, exp}, {bytes.Repeat([]byte{3}, 33), id, exp},
	} {
		if cache.verify(input.key, input.id, input.exp, sig) {
			t.Fatal("signature crossed key/id/expiry")
		}
		want := referenceSignature(input.key, input.id, input.exp)
		if !cache.verify(input.key, input.id, input.exp, want) {
			t.Fatal("failed verification poisoned expected value")
		}
		if got := cache.get(input.key, input.id, input.exp); got != buildSignedImageURL(input.key, input.id, input.exp) {
			t.Fatal("GET changed later issuance")
		}
	}
	cache.reset()
	if !cache.verify(key, id, exp, sig) {
		t.Fatal("cold cache changed verification")
	}
	if cache.get(key, id, exp) != issued {
		t.Fatal("verification cache changed issued URL")
	}
}

func TestImageVerificationConcurrentIssuanceAndReset(t *testing.T) {
	var cache imageURLCache
	key := bytes.Repeat([]byte{1}, 32)
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Go(func() {
			for i := range 32 {
				id := fmt.Sprintf("image-%d-%d", worker, i)
				exp := int64(1793437001 + i)
				sig := imageSignature(key, id, exp)
				cache.get(key, id, exp)
				if !cache.verify(key, id, exp, sig) {
					t.Error("concurrent verification changed signature")
				}
				if i%8 == 0 {
					cache.reset()
				}
			}
		})
	}
	wg.Wait()
	if n := cache.stats()["entries"]; n > imageURLCacheEntriesPerShard {
		t.Fatal("single shard exceeded cap", n)
	}
}

func TestImageVerificationUsesCurrentDatabaseKeyAndState(t *testing.T) {
	db, _ := architectureDB(t)
	owner, _ := fixtureOwner(t, db)
	const id = "verification-image"
	data := jpegFixture(t, 16)
	mustSQL(t, db, `INSERT INTO image_objects(id,owner_user_id,data,media_type) VALUES($1,$2,$3,'image/jpeg')`, id, owner.ID, data)
	mustSQL(t, db, `UPDATE users SET profile_image_id=$1 WHERE id=$2`, id, owner.ID)
	h := &HandlerContext{DB: db, Images: NewImageCache(imageCacheMaxBytes), Config: &Config{}}
	e := echo.New()
	e.HTTPErrorHandler = APIErrorHandler
	var abuse any
	e.GET("/api/images/:imageId", func(c *echo.Context) error {
		err := getImageHandler(h)(c)
		abuse = c.Get("abuse")
		return err
	})
	get := func(path string, want int, wantAbuse any) {
		t.Helper()
		w := httptest.NewRecorder()
		e.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want || abuse != wantAbuse {
			t.Fatalf("GET status=%d abuse=%v; want %d/%v", w.Code, abuse, want, wantAbuse)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cache header changed")
		}
		if want == 200 && !bytes.Equal(w.Body.Bytes(), data) {
			t.Fatal("image bytes changed")
		}
	}
	issued := signedImageURL(owner, id)
	before := signedImageURLs.stats()["verification_hits"]
	get(issued, 200, nil)
	if imageURLCacheEnabled && imageVerificationCacheEnabled && signedImageURLs.stats()["verification_hits"] <= before {
		t.Fatal("image GET did not use issued signature")
	}
	u, _ := url.Parse(issued)
	q := u.Query()
	q.Set("sig", base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	u.RawQuery = q.Encode()
	get(u.String(), 403, "invalid_signature")
	get(issued, 200, nil)
	// The body cache retains the image, but the key is read afresh from users.
	mustSQL(t, db, `UPDATE users SET secret_key_hash=repeat('cd',32) WHERE id=$1`, owner.ID)
	get(issued, 403, "invalid_signature")
	owner.SecretKeyHash = strings.Repeat("cd", 32)
	if err := owner.setImageKey(); err != nil {
		t.Fatal(err)
	}
	rotated := signedImageURL(owner, id)
	get(rotated, 200, nil)
	mustSQL(t, db, `UPDATE users SET secret_key_hash='invalid' WHERE id=$1`, owner.ID)
	get(rotated, 500, nil)
	mustSQL(t, db, `UPDATE users SET secret_key_hash=$2 WHERE id=$1`, owner.ID, owner.SecretKeyHash)
	get(rotated+"&exp=1", 403, nil)
	// Client-signed non-bucket expiries remain valid, even without prior issuance.
	exp := time.Now().Add(time.Hour).Unix() + 17
	get(buildSignedImageURL(owner.ImageKey, id, exp), 200, nil)
	get(buildSignedImageURL(owner.ImageKey, id, exp), 200, nil)
	expired := signedImageURLs.get(owner.ImageKey, id, time.Now().Add(-time.Second).Unix())
	get(expired, 403, nil)
	mustSQL(t, db, `UPDATE users SET profile_image_id=NULL WHERE id=$1`, owner.ID)
	get(rotated, 403, nil)
	mustSQL(t, db, `UPDATE users SET profile_image_id=$2,status='deleted' WHERE id=$1`, owner.ID, id)
	get(rotated, 403, nil)
}

func BenchmarkImageVerificationReuse(b *testing.B) {
	key := bytes.Repeat([]byte{1}, 32)
	const id = "01234567-89ab-cdef-0123-456789abcdef"
	const exp = int64(1793437001)
	sig := imageSignature(key, id, exp)
	for _, mode := range []string{"original", "issued", "unique-miss"} {
		b.Run(mode, func(b *testing.B) {
			var cache imageURLCache
			cache.get(key, id, exp)
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				switch mode {
				case "original":
					if !hmac.Equal(sig, imageSignature(key, id, exp)) {
						b.Fatal("mismatch")
					}
				case "issued":
					if !cache.verify(key, id, exp, sig) {
						b.Fatal("mismatch")
					}
				case "unique-miss":
					cache.verify(key, id, exp+int64(i)+1, sig)
				}
			}
		})
	}
}
