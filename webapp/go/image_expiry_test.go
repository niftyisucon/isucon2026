package isureal

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestImageURLExpiryBoundsAndRollback(t *testing.T) {
	for _, bucket := range []time.Duration{time.Second, time.Minute} {
		for _, base := range []time.Time{
			time.Date(2026, 10, 31, 23, 59, 0, 0, time.UTC),
			time.Date(2026, 12, 31, 23, 59, 0, 0, time.FixedZone("local", 9*3600)),
		} {
			for second := 0; second < 180; second++ {
				now := base.Add(time.Duration(second)*time.Second + 999*time.Millisecond)
				exp := imageURLExpiry(now, bucket)
				remaining := time.Unix(exp, 0).Sub(now)
				if remaining <= imageURLLifetime-bucket || remaining > imageURLLifetime {
					t.Fatalf("lifetime=%s bucket=%s", remaining, bucket)
				}
				if bucket == time.Second && exp != now.Add(imageURLLifetime).Unix() {
					t.Fatal("rollback changed original expiry")
				}
				if exp%int64(bucket/time.Second) != 0 {
					t.Fatal("expiry is not bucket aligned")
				}
			}
		}
	}
	for _, value := range []string{"", "60", "1"} {
		want := time.Minute
		if value == "1" {
			want = time.Second
		}
		if got := configuredImageURLExpiryBucket(value); got != want {
			t.Fatal("configuration", value, got)
		}
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("invalid bucket accepted")
			}
		}()
		configuredImageURLExpiryBucket("0")
	}()
}

func TestImageURLExpiryCacheAcrossSeconds(t *testing.T) {
	key := bytes.Repeat([]byte{0xab}, 32)
	base := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	for _, bucket := range []time.Duration{time.Second, time.Minute} {
		var cache imageURLCache
		start := time.Now()
		for second := 0; second < 61; second++ {
			exp := imageURLExpiry(base.Add(time.Duration(second)*time.Second), bucket)
			for id := 0; id < 20; id++ {
				cache.get(key, fmt.Sprintf("image-%d", id), exp)
			}
		}
		elapsed := time.Since(start)
		stats := cache.stats()
		wantMisses := uint64(1220)
		if bucket == time.Minute {
			wantMisses = 40
		}
		if stats["misses"] != wantMisses || stats["hits"] != 1220-wantMisses {
			t.Fatal("cache reuse", bucket, stats)
		}
		t.Logf("20 images every second for 61 simulated seconds: bucket=%s elapsed=%s hits=%d misses=%d", bucket, elapsed, stats["hits"], stats["misses"])
		for _, offset := range []time.Duration{0, 59 * time.Second, 60 * time.Second} {
			exp := imageURLExpiry(base.Add(offset), bucket)
			actual := cache.get(key, "image-0", exp)
			if actual != buildSignedImageURL(key, "image-0", exp) {
				t.Fatal("signature or URL format changed")
			}
			parsed, err := url.Parse(actual)
			if err != nil {
				t.Fatal(err)
			}
			got, err := strconv.ParseInt(parsed.Query().Get("exp"), 10, 64)
			if err != nil || got != exp || strconv.FormatInt(got, 10) != parsed.Query().Get("exp") {
				t.Fatal("noncanonical expiry")
			}
		}
	}
}
