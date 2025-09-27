package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Cache interface using Redis
type RedisCache struct {
	client *redis.Client
	config CacheConfig
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(config CacheConfig) (*RedisCache, error) {
	// Parse Redis URL
	addr, password, db, err := parseRedisURL(config.RedisURL, config.RedisPassword, config.RedisDB)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	opts := &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		config: config,
	}, nil
}

// Get retrieves a value from cache
func (r *RedisCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	var entry CacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal cache entry: %w", err)
	}

	if entry.IsExpired() {
		// Clean up expired entry
		_ = r.client.Del(ctx, key)
		return nil, false, nil
	}

	return entry.Value, true, nil
}

// Set stores a value in cache with TTL
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = r.config.DefaultTTL
	}
	if ttl > r.config.MaxTTL {
		ttl = r.config.MaxTTL
	}

	entry := &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a key from cache
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Clear removes all keys from cache
func (r *RedisCache) Clear(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

// GetMultiple retrieves multiple values from cache
func (r *RedisCache) GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for i, val := range vals {
		if val == nil {
			continue
		}

		var entry CacheEntry
		if err := json.Unmarshal([]byte(val.(string)), &entry); err != nil {
			continue
		}

		if !entry.IsExpired() {
			result[keys[i]] = entry.Value
		}
	}

	return result, nil
}

// SetMultiple stores multiple values in cache
func (r *RedisCache) SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()
	for key, value := range items {
		entry := &CacheEntry{
			Value:     value,
			ExpiresAt: time.Now().Add(ttl),
			CreatedAt: time.Now(),
		}

		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal cache entry for key %s: %w", key, err)
		}

		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetWithMetadata retrieves a value with metadata
func (r *RedisCache) GetWithMetadata(ctx context.Context, key string) (*CacheEntry, bool, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	var entry CacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal cache entry: %w", err)
	}

	if entry.IsExpired() {
		_ = r.client.Del(ctx, key)
		return nil, false, nil
	}

	return &entry, true, nil
}

// SetWithMetadata stores a value with metadata
func (r *RedisCache) SetWithMetadata(ctx context.Context, key string, entry *CacheEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	ttl := entry.TTL()
	if ttl <= 0 {
		ttl = r.config.DefaultTTL
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

// Expire sets expiration for a key
func (r *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

// TTL returns time to live for a key
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// EvictExpired removes expired keys
func (r *RedisCache) EvictExpired(ctx context.Context) (int, error) {
	// Redis handles TTL automatically, but we can scan for expired keys
	keys, err := r.client.Keys(ctx, "*").Result()
	if err != nil {
		return 0, err
	}

	var expiredCount int
	pipe := r.client.Pipeline()
	for _, key := range keys {
		entry, exists, err := r.GetWithMetadata(ctx, key)
		if err != nil {
			continue
		}
		if !exists || entry.IsExpired() {
			pipe.Del(ctx, key)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		_, err = pipe.Exec(ctx)
	}

	return expiredCount, err
}

// Size returns the number of keys in cache
func (r *RedisCache) Size(ctx context.Context) (int64, error) {
	return r.client.DBSize(ctx).Result()
}

// StartBackgroundRefresh starts background refresh workers
func (r *RedisCache) StartBackgroundRefresh(ctx context.Context, refreshFunc RefreshFunc) error {
	// Background refresh is handled by the fallback cache
	// Redis doesn't need background refresh as it handles TTL automatically
	return nil
}

// StopBackgroundRefresh stops background refresh workers
func (r *RedisCache) StopBackgroundRefresh() error {
	return nil
}

// Ping checks Redis connection
func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// parseRedisURL parses a Redis URL and extracts connection parameters
func parseRedisURL(redisURL, defaultPassword string, defaultDB int) (addr, password string, db int, err error) {
	// Handle simple host:port format
	if !strings.Contains(redisURL, "://") {
		return redisURL, defaultPassword, defaultDB, nil
	}

	// Parse URL
	u, err := url.Parse(redisURL)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid Redis URL: %w", err)
	}

	// Extract address
	addr = u.Host
	if addr == "" {
		return "", "", 0, fmt.Errorf("missing host in Redis URL")
	}

	// Extract password
	password = defaultPassword
	if u.User != nil {
		if pass, ok := u.User.Password(); ok && pass != "" {
			password = pass
		}
	}

	// Extract database number
	db = defaultDB
	if u.Path != "" && u.Path != "/" {
		dbStr := strings.TrimPrefix(u.Path, "/")
		if dbStr != "" {
			if parsedDB, err := strconv.Atoi(dbStr); err == nil {
				db = parsedDB
			}
		}
	}

	return addr, password, db, nil
}

