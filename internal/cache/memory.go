package cache

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// InMemoryCache implements Cache interface using in-memory storage
type InMemoryCache struct {
	config CacheConfig
	data   map[string]*CacheEntry
	mu     sync.RWMutex
	stats  *CacheStats
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Refreshes int64
	mu        sync.RWMutex
}

// NewInMemoryCache creates a new in-memory cache instance
func NewInMemoryCache(config CacheConfig) *InMemoryCache {
	return &InMemoryCache{
		config: config,
		data:   make(map[string]*CacheEntry),
		stats:  &CacheStats{},
	}
}

// Get retrieves a value from cache
func (m *InMemoryCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	m.mu.RLock()
	entry, exists := m.data[key]
	m.mu.RUnlock()

	if !exists {
		m.stats.recordMiss()
		return nil, false, nil
	}

	if entry.IsExpired() {
		m.mu.Lock()
		delete(m.data, key)
		m.mu.Unlock()
		m.stats.recordMiss()
		return nil, false, nil
	}

	m.stats.recordHit()
	return entry.Value, true, nil
}

// Set stores a value in cache with TTL
func (m *InMemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = m.config.DefaultTTL
	}
	if ttl > m.config.MaxTTL {
		ttl = m.config.MaxTTL
	}

	entry := &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we need to evict
	if int64(len(m.data)) >= m.config.MaxSize {
		m.evictLRU()
	}

	m.data[key] = entry
	return nil
}

// Delete removes a key from cache
func (m *InMemoryCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

// Clear removes all keys from cache
func (m *InMemoryCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]*CacheEntry)
	return nil
}

// GetMultiple retrieves multiple values from cache
func (m *InMemoryCache) GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	result := make(map[string]interface{})
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, key := range keys {
		if entry, exists := m.data[key]; exists && !entry.IsExpired() {
			result[key] = entry.Value
		}
	}

	return result, nil
}

// SetMultiple stores multiple values in cache
func (m *InMemoryCache) SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we need to evict
	if int64(len(m.data)+len(items)) >= m.config.MaxSize {
		m.evictLRU()
	}

	for key, value := range items {
		entry := &CacheEntry{
			Value:     value,
			ExpiresAt: time.Now().Add(ttl),
			CreatedAt: time.Now(),
		}
		m.data[key] = entry
	}

	return nil
}

// GetWithMetadata retrieves a value with metadata
func (m *InMemoryCache) GetWithMetadata(ctx context.Context, key string) (*CacheEntry, bool, error) {
	m.mu.RLock()
	entry, exists := m.data[key]
	m.mu.RUnlock()

	if !exists {
		m.stats.recordMiss()
		return nil, false, nil
	}

	if entry.IsExpired() {
		m.mu.Lock()
		delete(m.data, key)
		m.mu.Unlock()
		m.stats.recordMiss()
		return nil, false, nil
	}

	m.stats.recordHit()
	return entry, true, nil
}

// SetWithMetadata stores a value with metadata
func (m *InMemoryCache) SetWithMetadata(ctx context.Context, key string, entry *CacheEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we need to evict
	if int64(len(m.data)) >= m.config.MaxSize {
		m.evictLRU()
	}

	m.data[key] = entry
	return nil
}

// Expire sets expiration for a key
func (m *InMemoryCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.data[key]; exists {
		entry.ExpiresAt = time.Now().Add(ttl)
	}

	return nil
}

// TTL returns time to live for a key
func (m *InMemoryCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entry, exists := m.data[key]; exists {
		if entry.IsExpired() {
			return 0, nil
		}
		return entry.TTL(), nil
	}

	return 0, fmt.Errorf("key not found")
}

// EvictExpired removes expired keys
func (m *InMemoryCache) EvictExpired(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expiredKeys []string
	for key, entry := range m.data {
		if entry.IsExpired() {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(m.data, key)
	}

	m.stats.recordEvictions(int64(len(expiredKeys)))
	return len(expiredKeys), nil
}

// Size returns the number of keys in cache
func (m *InMemoryCache) Size(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return int64(len(m.data)), nil
}

// StartBackgroundRefresh starts background refresh workers
func (m *InMemoryCache) StartBackgroundRefresh(ctx context.Context, refreshFunc RefreshFunc) error {
	// In-memory cache doesn't need background refresh
	// This is handled by the fallback cache
	return nil
}

// StopBackgroundRefresh stops background refresh workers
func (m *InMemoryCache) StopBackgroundRefresh() error {
	return nil
}

// Ping checks cache health
func (m *InMemoryCache) Ping(ctx context.Context) error {
	return nil
}

// Close closes cache resources
func (m *InMemoryCache) Close() error {
	return nil
}

// evictLRU evicts least recently used entries
func (m *InMemoryCache) evictLRU() {
	if len(m.data) == 0 {
		return
	}

	// Sort entries by creation time (LRU)
	type entryWithKey struct {
		key   string
		entry *CacheEntry
	}

	entries := make([]entryWithKey, 0, len(m.data))
	for key, entry := range m.data {
		entries = append(entries, entryWithKey{key, entry})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].entry.CreatedAt.Before(entries[j].entry.CreatedAt)
	})

	// Evict 10% of entries
	evictCount := len(entries) / 10
	if evictCount == 0 {
		evictCount = 1
	}

	for i := 0; i < evictCount; i++ {
		delete(m.data, entries[i].key)
	}

	m.stats.recordEvictions(int64(evictCount))
}

// getExpiringKeys returns keys that are about to expire
func (m *InMemoryCache) getExpiringKeys(threshold float64) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expiringKeys []string

	for key, entry := range m.data {
		if entry.IsExpired() {
			continue
		}

		ttl := entry.TTL()
		totalTTL := entry.ExpiresAt.Sub(entry.CreatedAt)
		remainingRatio := float64(ttl) / float64(totalTTL)

		if remainingRatio <= threshold {
			expiringKeys = append(expiringKeys, key)
		}
	}

	return expiringKeys
}

// GetStats returns cache statistics
func (m *InMemoryCache) GetStats() *CacheStats {
	return m.stats
}

// recordHit records a cache hit
func (s *CacheStats) recordHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Hits++
}

// recordMiss records a cache miss
func (s *CacheStats) recordMiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Misses++
}

// recordEvictions records cache evictions
func (s *CacheStats) recordEvictions(count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Evictions += count
}

// recordRefresh records a cache refresh
func (s *CacheStats) recordRefresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Refreshes++
}

// HitRate returns the cache hit rate
func (s *CacheStats) HitRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}

	return float64(s.Hits) / float64(total)
}
