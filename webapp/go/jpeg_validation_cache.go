package isureal

import (
	"bytes"
	"crypto/sha256"
	"os"
	"sync"
	"sync/atomic"
)

const jpegValidationShards = 16
const jpegValidationShardBytes = 2 << 20
const jpegValidationShardEntries = 64
const jpegValidationShardCandidates = 256

type jpegValidationKey struct {
	digest [sha256.Size]byte
	limit  int
}

type jpegValidationShard struct {
	mu    sync.RWMutex
	data  map[jpegValidationKey][]byte
	seen  map[jpegValidationKey]struct{}
	bytes int
}

// Only successful full decodes are memoized. Keep an owned copy and compare the
// exact bytes on every hit: even a digest collision cannot validate other data.
// No ownership, authorization, image IDs or DB state are kept here.
type jpegValidationCache struct {
	shards     [jpegValidationShards]jpegValidationShard
	generation atomic.Uint64
	hits       atomic.Uint64
	misses     atomic.Uint64
	bypasses   atomic.Uint64
}

var validatedJPEGs jpegValidationCache
var jpegValidationCacheEnabled = os.Getenv("ISUREAL_JPEG_VALIDATION_CACHE") != "false"

func (c *jpegValidationCache) validate(data []byte, limit int) error {
	// Large images retain the original path, without hashing or copying them.
	if len(data) > jpegValidationShardBytes {
		c.bypasses.Add(1)
		return decodeJPEGUncached(data, limit)
	}
	key := jpegValidationKey{digest: sha256.Sum256(data), limit: limit}
	shard := &c.shards[key.digest[0]%jpegValidationShards]
	generation := c.generation.Load()
	shard.mu.RLock()
	cached, ok := shard.data[key]
	hit := ok && bytes.Equal(cached, data)
	shard.mu.RUnlock()
	if hit {
		c.hits.Add(1)
		return nil
	}
	c.misses.Add(1)
	if err := decodeJPEGUncached(data, limit); err != nil {
		return err
	}
	c.put(key, data, generation)
	return nil
}

func (c *jpegValidationCache) put(key jpegValidationKey, data []byte, generation uint64) {
	shard := &c.shards[key.digest[0]%jpegValidationShards]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if generation != c.generation.Load() || len(data) > jpegValidationShardBytes {
		return
	}
	if cached, ok := shard.data[key]; ok {
		if bytes.Equal(cached, data) {
			return
		}
		shard.bytes -= len(cached)
		delete(shard.data, key)
	}
	// Admit after two successful full decodes. Unique uploads retain only a
	// bounded digest entry, avoiding an extra image-sized allocation per miss.
	if _, seen := shard.seen[key]; !seen {
		if shard.seen == nil || len(shard.seen) >= jpegValidationShardCandidates {
			shard.seen = make(map[jpegValidationKey]struct{})
		}
		shard.seen[key] = struct{}{}
		return
	}
	if shard.data == nil || len(shard.data) >= jpegValidationShardEntries || shard.bytes+len(data) > jpegValidationShardBytes {
		shard.data = make(map[jpegValidationKey][]byte)
		shard.bytes = 0
	}
	shard.data[key] = bytes.Clone(data)
	shard.bytes += len(data)
}

func (c *jpegValidationCache) reset() {
	// A decode that started before initialize must not repopulate the cache.
	c.generation.Add(1)
	for i := range c.shards {
		shard := &c.shards[i]
		shard.mu.Lock()
		shard.data = nil
		shard.seen = nil
		shard.bytes = 0
		shard.mu.Unlock()
	}
}

func (c *jpegValidationCache) stats() map[string]uint64 {
	var entries, candidates, size uint64
	for i := range c.shards {
		shard := &c.shards[i]
		shard.mu.RLock()
		entries += uint64(len(shard.data))
		candidates += uint64(len(shard.seen))
		size += uint64(shard.bytes)
		shard.mu.RUnlock()
	}
	return map[string]uint64{"hits": c.hits.Load(), "misses": c.misses.Load(), "bypasses": c.bypasses.Load(), "entries": entries, "candidates": candidates, "bytes": size}
}
