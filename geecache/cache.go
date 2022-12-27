package geecache

import (
	"geecache/geecache/lru"
	"sync"
)

// cache 封装 lru.Cache 使其并发安全
type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

// add 添加缓存
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil { //延迟初始化
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

// get 获取缓存
func (c *cache) get(key string) (value ByteView, ok bool) {
	if c.lru == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}

	return
}
