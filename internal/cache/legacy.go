package cache

import (
	"context"
	"time"
)

// LegacyCache provides backward compatibility with the old cache interface
type LegacyCache struct {
	cache Cache
}

// NewLegacyCache creates a legacy cache wrapper
func NewLegacyCache(cache Cache) *LegacyCache {
	return &LegacyCache{cache: cache}
}

// Get implements the old cache interface
func (l *LegacyCache) Get(key string) (float64, bool) {
	ctx := context.Background()
	value, found, err := l.cache.Get(ctx, key)
	if err != nil || !found {
		return 0, false
	}
	if price, ok := value.(float64); ok {
		return price, true
	}
	return 0, false
}

// Set implements the old cache interface
func (l *LegacyCache) Set(key string, value float64) {
	ctx := context.Background()
	_ = l.cache.Set(ctx, key, value, 60*time.Second)
}

