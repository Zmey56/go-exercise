package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FallbackCache implements Cache interface with fallback to in-memory cache
type FallbackCache struct {
	primary   Cache
	fallback  *InMemoryCache
	config    CacheConfig
	refreshCh chan string
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

// NewFallbackCache creates a new fallback cache instance
func NewFallbackCache(primary Cache, config CacheConfig) *FallbackCache {
	fallbackConfig := config
	fallbackConfig.DefaultTTL = config.FallbackTTL

	return &FallbackCache{
		primary:   primary,
		fallback:  NewInMemoryCache(fallbackConfig),
		config:    config,
		refreshCh: make(chan string, 1000),
		stopCh:    make(chan struct{}),
	}
}

// Get retrieves a value from cache (primary first, then fallback)
func (f *FallbackCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	// Try primary cache first
	value, found, err := f.primary.Get(ctx, key)
	if err != nil {
		// Primary failed, try fallback
		value, found, err = f.fallback.Get(ctx, key)
		if err != nil {
			return nil, false, fmt.Errorf("both primary and fallback failed: %w", err)
		}
		return value, found, nil
	}

	if found {
		// Update fallback with fresh data
		if err := f.fallback.Set(ctx, key, value, f.config.FallbackTTL); err != nil {
			fmt.Printf("Warning: failed to update fallback cache for key %s: %v\n", key, err)
		}
		return value, true, nil
	}

	// Not found in primary, try fallback
	value, found, err = f.fallback.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}

	return value, found, nil
}

// Set stores a value in both primary and fallback caches
func (f *FallbackCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Set in primary cache
	if err := f.primary.Set(ctx, key, value, ttl); err != nil {
		// Primary failed, set only in fallback
		return f.fallback.Set(ctx, key, value, f.config.FallbackTTL)
	}

	// Set in fallback cache (log error but don't fail)
	if err := f.fallback.Set(ctx, key, value, f.config.FallbackTTL); err != nil {
		// Log error but don't fail the operation since primary succeeded
		// In a real application, you would use a proper logger here
		fmt.Printf("Warning: failed to set fallback cache for key %s: %v\n", key, err)
	}
	return nil
}

// Delete removes a key from both caches
func (f *FallbackCache) Delete(ctx context.Context, key string) error {
	// Delete from primary
	if err := f.primary.Delete(ctx, key); err != nil {
		// Primary failed, delete from fallback
		return f.fallback.Delete(ctx, key)
	}

	// Delete from fallback
	_ = f.fallback.Delete(ctx, key)
	return nil
}

// Clear clears both caches
func (f *FallbackCache) Clear(ctx context.Context) error {
	// Clear primary
	if err := f.primary.Clear(ctx); err != nil {
		// Primary failed, clear fallback
		return f.fallback.Clear(ctx)
	}

	// Clear fallback
	_ = f.fallback.Clear(ctx)
	return nil
}

// GetMultiple retrieves multiple values from cache
func (f *FallbackCache) GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error) {
	// Try primary first
	result, err := f.primary.GetMultiple(ctx, keys)
	if err != nil {
		// Primary failed, try fallback
		return f.fallback.GetMultiple(ctx, keys)
	}

	// Check for missing keys in fallback
	missingKeys := make([]string, 0)
	for _, key := range keys {
		if _, found := result[key]; !found {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		fallbackResult, err := f.fallback.GetMultiple(ctx, missingKeys)
		if err == nil {
			for key, value := range fallbackResult {
				result[key] = value
			}
		}
	}

	return result, nil
}

// SetMultiple stores multiple values in both caches
func (f *FallbackCache) SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	// Set in primary
	if err := f.primary.SetMultiple(ctx, items, ttl); err != nil {
		// Primary failed, set only in fallback
		return f.fallback.SetMultiple(ctx, items, f.config.FallbackTTL)
	}

	// Set in fallback
	_ = f.fallback.SetMultiple(ctx, items, f.config.FallbackTTL)
	return nil
}

// GetWithMetadata retrieves a value with metadata
func (f *FallbackCache) GetWithMetadata(ctx context.Context, key string) (*CacheEntry, bool, error) {
	// Try primary first
	entry, found, err := f.primary.GetWithMetadata(ctx, key)
	if err != nil {
		// Primary failed, try fallback
		return f.fallback.GetWithMetadata(ctx, key)
	}

	if found {
		// Update fallback with fresh data
		_ = f.fallback.SetWithMetadata(ctx, key, entry)
		return entry, true, nil
	}

	// Not found in primary, try fallback
	return f.fallback.GetWithMetadata(ctx, key)
}

// SetWithMetadata stores a value with metadata
func (f *FallbackCache) SetWithMetadata(ctx context.Context, key string, entry *CacheEntry) error {
	// Set in primary
	if err := f.primary.SetWithMetadata(ctx, key, entry); err != nil {
		// Primary failed, set only in fallback
		return f.fallback.SetWithMetadata(ctx, key, entry)
	}

	// Set in fallback
	_ = f.fallback.SetWithMetadata(ctx, key, entry)
	return nil
}

// Expire sets expiration for a key
func (f *FallbackCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	// Set expiration in primary
	if err := f.primary.Expire(ctx, key, ttl); err != nil {
		// Primary failed, set in fallback
		return f.fallback.Expire(ctx, key, f.config.FallbackTTL)
	}

	// Set expiration in fallback
	_ = f.fallback.Expire(ctx, key, f.config.FallbackTTL)
	return nil
}

// TTL returns time to live for a key
func (f *FallbackCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	// Try primary first
	ttl, err := f.primary.TTL(ctx, key)
	if err != nil {
		// Primary failed, try fallback
		return f.fallback.TTL(ctx, key)
	}

	return ttl, nil
}

// EvictExpired removes expired keys from both caches
func (f *FallbackCache) EvictExpired(ctx context.Context) (int, error) {
	// Evict from primary
	primaryCount, err := f.primary.EvictExpired(ctx)
	if err != nil {
		// Primary failed, evict from fallback
		return f.fallback.EvictExpired(ctx)
	}

	// Evict from fallback
	fallbackCount, _ := f.fallback.EvictExpired(ctx)

	return primaryCount + fallbackCount, nil
}

// Size returns the total number of keys in both caches
func (f *FallbackCache) Size(ctx context.Context) (int64, error) {
	// Get size from primary
	primarySize, err := f.primary.Size(ctx)
	if err != nil {
		// Primary failed, return fallback size
		return f.fallback.Size(ctx)
	}

	// Get size from fallback
	fallbackSize, _ := f.fallback.Size(ctx)

	return primarySize + fallbackSize, nil
}

// StartBackgroundRefresh starts background refresh workers
func (f *FallbackCache) StartBackgroundRefresh(ctx context.Context, refreshFunc RefreshFunc) error {
	if !f.config.EnableFallback {
		return nil
	}

	// Start refresh workers
	for i := 0; i < f.config.RefreshWorkers; i++ {
		f.wg.Add(1)
		go f.refreshWorker(ctx, refreshFunc)
	}

	// Start cleanup worker
	f.wg.Add(1)
	go f.cleanupWorker(ctx)

	return nil
}

// StopBackgroundRefresh stops background refresh workers
func (f *FallbackCache) StopBackgroundRefresh() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-f.stopCh:
		// Already closed
		return nil
	default:
		close(f.stopCh)
	}

	f.wg.Wait()
	return nil
}

// refreshWorker processes refresh requests
func (f *FallbackCache) refreshWorker(ctx context.Context, refreshFunc RefreshFunc) {
	defer f.wg.Done()

	ticker := time.NewTicker(f.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case key := <-f.refreshCh:
			f.refreshKey(ctx, key, refreshFunc)
		case <-ticker.C:
			f.refreshExpiringKeys(ctx, refreshFunc)
		case <-f.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// cleanupWorker periodically cleans up expired keys
func (f *FallbackCache) cleanupWorker(ctx context.Context) {
	defer f.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.EvictExpired(ctx)
		case <-f.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// refreshKey refreshes a specific key
func (f *FallbackCache) refreshKey(ctx context.Context, key string, refreshFunc RefreshFunc) {
	value, ttl, err := refreshFunc(ctx, key)
	if err != nil {
		return
	}

	_ = f.Set(ctx, key, value, ttl)
}

// refreshExpiringKeys refreshes keys that are about to expire
func (f *FallbackCache) refreshExpiringKeys(ctx context.Context, refreshFunc RefreshFunc) {
	// Get keys that are about to expire (within 10% of TTL)
	keys := f.fallback.getExpiringKeys(0.1) // 10% of TTL remaining

	for _, key := range keys {
		select {
		case f.refreshCh <- key:
		default:
			// Channel full, skip this key
		}
	}
}

// Ping checks both cache connections
func (f *FallbackCache) Ping(ctx context.Context) error {
	// Check primary
	if err := f.primary.Ping(ctx); err != nil {
		// Primary failed, check fallback
		return f.fallback.Ping(ctx)
	}

	return nil
}

// Close closes both cache connections
func (f *FallbackCache) Close() error {
	// Stop background refresh
	_ = f.StopBackgroundRefresh()

	// Close primary
	if err := f.primary.Close(); err != nil {
		// Primary failed, close fallback
		return f.fallback.Close()
	}

	// Close fallback
	_ = f.fallback.Close()
	return nil
}

