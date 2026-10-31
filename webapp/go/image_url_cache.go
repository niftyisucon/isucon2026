package isureal

import (
	"crypto/hmac"
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
	urls map[imageURLCacheKey]imageURLCacheValue
}

type imageURLCacheValue struct {
	url       string
	signature [sha256.Size]byte
}

type imageURLCache struct {
	shards             [imageURLCacheShards]imageURLCacheShard
	hits               atomic.Uint64
	misses             atomic.Uint64
	verificationHits   atomic.Uint64
	verificationMisses atomic.Uint64
}

var signedImageURLs imageURLCache

// A/B evaluation can retain diagnostics while disabling this CPU optimization.
// Read once at process startup, like the trace sampling setting.
var imageURLCacheEnabled = os.Getenv("ISUREAL_IMAGE_URL_CACHE") != "false"
var imageVerificationCacheEnabled = os.Getenv("ISUREAL_IMAGE_VERIFY_CACHE") != "false"

func (c *imageURLCache) get(key []byte, id string, exp int64) string {
	// Production users have a SHA-256 key. Preserve legacy behavior for other
	// key lengths without putting arbitrary-length values in the cache.
	if len(key) != sha256.Size {
		return buildSignedImageURL(key, id, exp)
	}
	value, hit := c.getValue(key, id, exp)
	if hit {
		c.hits.Add(1)
	} else {
		c.misses.Add(1)
	}
	return value.url
}

func (c *imageURLCache) verify(key []byte, id string, exp int64, signature []byte) bool {
	if len(key) != sha256.Size {
		return hmac.Equal(signature, imageSignature(key, id, exp))
	}
	value, hit := c.getValue(key, id, exp)
	if hit {
		c.verificationHits.Add(1)
	} else {
		c.verificationMisses.Add(1)
	}
	// Only the expected mathematical signature is memoized. Never remember an
	// authorization result or the untrusted signature supplied by the caller.
	return hmac.Equal(signature, value.signature[:])
}

// Both callers pass a SHA-256 key read from the current user state.
func (c *imageURLCache) getValue(key []byte, id string, exp int64) (imageURLCacheValue, bool) {
	k := imageURLCacheKey{id: id, exp: exp}
	copy(k.secret[:], key)
	shard := &c.shards[k.secret[0]%imageURLCacheShards]
	shard.mu.RLock()
	value, ok := shard.urls[k]
	shard.mu.RUnlock()
	if ok {
		return value, true
	}
	copy(value.signature[:], imageSignature(key, id, exp))
	value.url = formatSignedImageURL(id, exp, value.signature[:])
	shard.mu.Lock()
	if shard.urls == nil || len(shard.urls) >= imageURLCacheEntriesPerShard {
		shard.urls = make(map[imageURLCacheKey]imageURLCacheValue)
	}
	shard.urls[k] = value
	shard.mu.Unlock()
	return value, false
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
	return map[string]uint64{"hits": c.hits.Load(), "misses": c.misses.Load(), "entries": entries,
		"verification_hits": c.verificationHits.Load(), "verification_misses": c.verificationMisses.Load()}
}
