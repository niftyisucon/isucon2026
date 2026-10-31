package isureal

import (
	"crypto/sha256"
	"os"
	"sync"
	"sync/atomic"
)

const imageURLCacheShards = 16
const imageURLCacheEntriesPerShard = 1024

// Only memoize the pure signing operation. Every caller still loads the current
// user/key and checks visibility; image GET still checks current DB state.
// Expiry is part of the key; the caller chooses the issuance expiry bucket.
type imageURLCacheKey struct {
	secret [sha256.Size]byte
	id     string
	exp    int64
}

type imageURLCacheShard struct {
	mu   sync.RWMutex
	urls map[imageURLCacheKey]string
}

type imageURLCache struct {
	shards [imageURLCacheShards]imageURLCacheShard
	hits   atomic.Uint64
	misses atomic.Uint64
}

var signedImageURLs imageURLCache

// A/B evaluation can retain diagnostics while disabling this CPU optimization.
// Read once at process startup, like the trace sampling setting.
var imageURLCacheEnabled = os.Getenv("ISUREAL_IMAGE_URL_CACHE") != "false"

func (c *imageURLCache) get(key []byte, id string, exp int64) string {
	// Production users have a SHA-256 key. Preserve legacy behavior for other
	// key lengths without putting arbitrary-length values in the cache.
	if len(key) != sha256.Size {
		return buildSignedImageURL(key, id, exp)
	}
	k := imageURLCacheKey{id: id, exp: exp}
	copy(k.secret[:], key)
	shard := &c.shards[k.secret[0]%imageURLCacheShards]
	shard.mu.RLock()
	url, ok := shard.urls[k]
	shard.mu.RUnlock()
	if ok {
		c.hits.Add(1)
		return url
	}
	c.misses.Add(1)
	url = buildSignedImageURL(key, id, exp)
	shard.mu.Lock()
	if shard.urls == nil || len(shard.urls) >= imageURLCacheEntriesPerShard {
		shard.urls = make(map[imageURLCacheKey]string)
	}
	shard.urls[k] = url
	shard.mu.Unlock()
	return url
}

func (c *imageURLCache) reset() {
	for i := range c.shards {
		shard := &c.shards[i]
		shard.mu.Lock()
		shard.urls = nil
		shard.mu.Unlock()
	}
}

func (c *imageURLCache) stats() map[string]uint64 {
	var entries uint64
	for i := range c.shards {
		shard := &c.shards[i]
		shard.mu.RLock()
		entries += uint64(len(shard.urls))
		shard.mu.RUnlock()
	}
	return map[string]uint64{"hits": c.hits.Load(), "misses": c.misses.Load(), "entries": entries}
}
