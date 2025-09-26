package cache

import (
	"sync"
	"time"

	"github.com/Zmey56/go-exercise/internal/metrics"
)

type entry struct {
	v       float64
	expires time.Time
}

type Cache struct {
	ttl  time.Duration
	mu   sync.RWMutex
	data map[string]entry
}

func New(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, data: make(map[string]entry)}
}

func (c *Cache) Get(key string) (float64, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		metrics.CacheMisses.Inc()
		return 0, false
	}
	if time.Now().After(e.expires) {
		c.mu.Lock()
		delete(c.data, key)
		metrics.CacheSize.Dec()
		c.mu.Unlock()
		metrics.CacheMisses.Inc()
		return 0, false
	}
	metrics.CacheHits.Inc()
	return e.v, true
}

func (c *Cache) Set(key string, v float64) {
	c.mu.Lock()
	_, exists := c.data[key]
	c.data[key] = entry{v: v, expires: time.Now().Add(c.ttl)}
	if !exists {
		metrics.CacheSize.Inc()
	}
	c.mu.Unlock()
}
