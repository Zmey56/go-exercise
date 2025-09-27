package cache

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// CacheType represents the type of cache to use
type CacheType string

const (
	CacheTypeMemory   CacheType = "memory"
	CacheTypeRedis    CacheType = "redis"
	CacheTypeFallback CacheType = "fallback"
)

// CacheFactory creates cache instances based on configuration
type CacheFactory struct{}

// NewCacheFactory creates a new cache factory
func NewCacheFactory() *CacheFactory {
	return &CacheFactory{}
}

// CreateCache creates a cache instance based on configuration
func (f *CacheFactory) CreateCache(config CacheConfig) (Cache, error) {
	cacheType := f.getCacheTypeFromConfig(config)

	switch cacheType {
	case CacheTypeMemory:
		return NewInMemoryCache(config), nil
	case CacheTypeRedis:
		return NewRedisCache(config)
	case CacheTypeFallback:
		return f.createFallbackCache(config)
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cacheType)
	}
}

// createFallbackCache creates a fallback cache with Redis primary and memory fallback
func (f *CacheFactory) createFallbackCache(config CacheConfig) (Cache, error) {
	// Create Redis cache as primary
	redisConfig := config
	redisCache, err := NewRedisCache(redisConfig)
	if err != nil {
		// Redis failed, fallback to memory only
		return NewInMemoryCache(config), nil
	}

	// Create fallback cache
	return NewFallbackCache(redisCache, config), nil
}

// getCacheTypeFromConfig determines cache type from configuration
func (f *CacheFactory) getCacheTypeFromConfig(config CacheConfig) CacheType {
	// Check environment variable first
	if cacheType := os.Getenv("CACHE_TYPE"); cacheType != "" {
		switch CacheType(cacheType) {
		case CacheTypeMemory, CacheTypeRedis, CacheTypeFallback:
			return CacheType(cacheType)
		}
	}

	// Default to fallback if Redis URL is provided
	if config.RedisURL != "" && config.RedisURL != "redis://localhost:6379" {
		return CacheTypeFallback
	}

	// Default to memory
	return CacheTypeMemory
}

// LoadConfigFromEnv loads cache configuration from environment variables
func LoadConfigFromEnv() CacheConfig {
	config := DefaultCacheConfig()

	// Redis configuration
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		config.RedisURL = redisURL
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		config.RedisPassword = redisPassword
	}
	if redisDB := os.Getenv("REDIS_DB"); redisDB != "" {
		if db, err := strconv.Atoi(redisDB); err == nil {
			config.RedisDB = db
		}
	}

	// TTL configuration
	if defaultTTL := os.Getenv("CACHE_DEFAULT_TTL"); defaultTTL != "" {
		if ttl, err := time.ParseDuration(defaultTTL); err == nil {
			config.DefaultTTL = ttl
		}
	}
	if maxTTL := os.Getenv("CACHE_MAX_TTL"); maxTTL != "" {
		if ttl, err := time.ParseDuration(maxTTL); err == nil {
			config.MaxTTL = ttl
		}
	}

	// Eviction configuration
	if maxSize := os.Getenv("CACHE_MAX_SIZE"); maxSize != "" {
		if size, err := strconv.ParseInt(maxSize, 10, 64); err == nil {
			config.MaxSize = size
		}
	}
	if evictionPolicy := os.Getenv("CACHE_EVICTION_POLICY"); evictionPolicy != "" {
		config.EvictionPolicy = evictionPolicy
	}

	// Background refresh configuration
	if refreshInterval := os.Getenv("CACHE_REFRESH_INTERVAL"); refreshInterval != "" {
		if interval, err := time.ParseDuration(refreshInterval); err == nil {
			config.RefreshInterval = interval
		}
	}
	if refreshWorkers := os.Getenv("CACHE_REFRESH_WORKERS"); refreshWorkers != "" {
		if workers, err := strconv.Atoi(refreshWorkers); err == nil {
			config.RefreshWorkers = workers
		}
	}

	// Fallback configuration
	if enableFallback := os.Getenv("CACHE_ENABLE_FALLBACK"); enableFallback != "" {
		if enable, err := strconv.ParseBool(enableFallback); err == nil {
			config.EnableFallback = enable
		}
	}
	if fallbackTTL := os.Getenv("CACHE_FALLBACK_TTL"); fallbackTTL != "" {
		if ttl, err := time.ParseDuration(fallbackTTL); err == nil {
			config.FallbackTTL = ttl
		}
	}

	return config
}

// CreateCacheFromEnv creates a cache instance from environment variables
func CreateCacheFromEnv() (Cache, error) {
	config := LoadConfigFromEnv()
	factory := NewCacheFactory()
	return factory.CreateCache(config)
}

