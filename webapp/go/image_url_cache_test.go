package isureal

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestImageURLCachePreservesSigningInputs(t *testing.T) {
	var cache imageURLCache
	keys := [][]byte{nil, bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32), bytes.Repeat([]byte{1}, 33)}
	for _, key := range keys {
		for _, id := range []string{"", "image-1", "image-2", "画像 /?&"} {
			for _, exp := range []int64{0, 1793425500, 1793425501} {
				want := buildSignedImageURL(key, id, exp)
				for range 2 {
					if got := cache.get(key, id, exp); got != want {
						t.Fatal("cached URL changed when signing key, image ID or expiry differed")
					}
				}
			}
		}
	}
}

func TestImageURLCacheBoundedAndConcurrentReset(t *testing.T) {
	var cache imageURLCache
	key := bytes.Repeat([]byte{1}, 32)
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Go(func() {
			for i := range 160 {
				id := fmt.Sprintf("%d-%d", worker, i)
				got := cache.get(key, id, 1793425500)
				if got != buildSignedImageURL(key, id, 1793425500) {
					t.Error("concurrent cache changed signature")
				}
			}
		})
	}
	wg.Wait()
	if n := cache.stats()["entries"]; n > imageURLCacheEntriesPerShard {
		t.Fatalf("single shard grew past limit: %d", n)
	}
	for range 4 {
		wg.Go(func() {
			for range 20 {
				cache.get(key, "same-image", 1793425500)
				cache.reset()
			}
		})
	}
	wg.Wait()
	cache.reset()
	if n := cache.stats()["entries"]; n != 0 {
		t.Fatalf("reset left %d entries", n)
	}
}

// Same initial-history/profile image URLs are repeated in feed/memory responses.
// Explicit expiry inputs still miss across a second boundary; production callers
// can share a coarser issuance expiry without changing cache-key semantics.
func BenchmarkImageURLRepeated(b *testing.B) {
	key := bytes.Repeat([]byte{1}, 32)
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("01234567-89ab-cdef-0123-%012d", i)
	}
	for _, cached := range []bool{false, true} {
		b.Run(fmt.Sprintf("cached=%t", cached), func(b *testing.B) {
			var cache imageURLCache
			b.ReportAllocs()
			for i := range b.N {
				id := ids[i%len(ids)]
				exp := int64(1793425500 + i/10000)
				if cached {
					cache.get(key, id, exp)
				} else {
					buildSignedImageURL(key, id, exp)
				}
			}
		})
	}
}

func BenchmarkImageURLUnique(b *testing.B) {
	key := bytes.Repeat([]byte{1}, 32)
	for _, cached := range []bool{false, true} {
		b.Run(fmt.Sprintf("cached=%t", cached), func(b *testing.B) {
			var cache imageURLCache
			b.ReportAllocs()
			for i := range b.N {
				if cached {
					cache.get(key, "image", int64(i))
				} else {
					buildSignedImageURL(key, "image", int64(i))
				}
			}
		})
	}
}
