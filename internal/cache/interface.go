package cache

import (
	"context"
	"time"
)

// CacheEntry represents a cache entry with metadata
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
	CreatedAt time.Time
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// TTL returns the time until expiration
func (e *CacheEntry) TTL() time.Duration {
	return time.Until(e.ExpiresAt)
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	// Redis configuration
	RedisURL      string
	RedisPassword string
	RedisDB       int

	// TTL settings
	DefaultTTL time.Duration
	MaxTTL     time.Duration

	// Eviction settings
	MaxSize        int64
	EvictionPolicy string // "lru", "lfu", "ttl"

	// Background refresh settings
	RefreshInterval time.Duration
	RefreshWorkers  int

	// Fallback settings
	EnableFallback bool
	FallbackTTL    time.Duration
}

// Cache defines the interface for cache operations
type Cache interface {
	// Basic operations
	Get(ctx context.Context, key string) (interface{}, bool, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error

	// Batch operations
	GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error)
	SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// Metadata operations
	GetWithMetadata(ctx context.Context, key string) (*CacheEntry, bool, error)
	SetWithMetadata(ctx context.Context, key string, entry *CacheEntry) error

	// TTL operations
	Expire(ctx context.Context, key string, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Eviction operations
	EvictExpired(ctx context.Context) (int, error)
	Size(ctx context.Context) (int64, error)

	// Background refresh
	StartBackgroundRefresh(ctx context.Context, refreshFunc RefreshFunc) error
	StopBackgroundRefresh() error

	// Health check
	Ping(ctx context.Context) error

	// Close resources
	Close() error
}

// RefreshFunc defines the function signature for background refresh
type RefreshFunc func(ctx context.Context, key string) (interface{}, time.Duration, error)

// DefaultCacheConfig returns sensible default configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		RedisURL:        "redis://localhost:6379",
		RedisPassword:   "",
		RedisDB:         0,
		DefaultTTL:      60 * time.Second,
		MaxTTL:          24 * time.Hour,
		MaxSize:         10000,
		EvictionPolicy:  "lru",
		RefreshInterval: 30 * time.Second,
		RefreshWorkers:  5,
		EnableFallback:  true,
		FallbackTTL:     5 * time.Minute,
	}
}
