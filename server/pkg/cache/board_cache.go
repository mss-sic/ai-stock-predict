package cache

import (
	"sync"
	"time"
)

type BoardCache struct {
	mu       sync.RWMutex
	data     interface{}
	dateStr  string
	cachedAt time.Time
	ttl      time.Duration
}

func NewBoardCache(ttl time.Duration) *BoardCache {
	return &BoardCache{ttl: ttl}
}

func (c *BoardCache) Get(dateStr string) (interface{}, string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.dateStr == dateStr && time.Since(c.cachedAt) < c.ttl {
		return c.data, c.dateStr, true
	}
	return nil, "", false
}

func (c *BoardCache) Set(data interface{}, dateStr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.dateStr = dateStr
	c.cachedAt = time.Now()
}
