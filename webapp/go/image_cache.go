package isureal

import (
	"sync"
	"sync/atomic"
)

// 画像の本体・所有者・形式は作成後に変わらず、id も UUID で再利用されないため、アプリ内に保持する。
// 閲覧可否（削除・退会・置き換え）は毎回 DB で確認するので、ここには載せない。
const imageCacheMaxBytes = 256 << 20
const edgeImageCacheMaxBytes = 1 << 30

func imageCacheLimit(config *Config) int64 {
	if config.EdgeUpstream != "" {
		return edgeImageCacheMaxBytes
	}
	return imageCacheMaxBytes
}

type cachedImage struct {
	OwnerID   string
	MediaType string
	Data      []byte
}

type ImageCache struct {
	mu         sync.RWMutex
	generation uint64
	images     map[string]*cachedImage
	bytes      atomic.Int64
	maxBytes   int64
}

func NewImageCache(maxBytes int64) *ImageCache {
	return &ImageCache{images: map[string]*cachedImage{}, maxBytes: maxBytes}
}

func (c *ImageCache) GetGeneration(id string) (*cachedImage, uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.images[id], c.generation
}
func (c *ImageCache) PutGeneration(id string, image *cachedImage, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation == generation {
		c.putLocked(id, image)
	}
}
func (c *ImageCache) Get(id string) *cachedImage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.images[id]
}

func (c *ImageCache) Put(id string, image *cachedImage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.putLocked(id, image)
}

func (c *ImageCache) putLocked(id string, image *cachedImage) {
	if int64(len(image.Data)) > c.maxBytes {
		return
	}
	if _, ok := c.images[id]; ok {
		return
	}
	// 上限を超えたら丸ごと捨てる。
	if c.bytes.Load()+int64(len(image.Data)) > c.maxBytes {
		c.images = map[string]*cachedImage{}
		c.bytes.Store(0)
	}
	c.images[id] = image
	c.bytes.Add(int64(len(image.Data)))
}

func (c *ImageCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	c.images = map[string]*cachedImage{}
	c.bytes.Store(0)
}
