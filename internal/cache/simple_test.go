package cache

import (
	"context"
	"testing"
	"time"
)

// TestInMemoryCacheBasic tests basic in-memory cache functionality
func TestInMemoryCacheBasic(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewInMemoryCache(config)
	ctx := context.Background()

	// Test Set and Get
	err := cache.Set(ctx, "key1", "value1", time.Minute)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, found, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !found {
		t.Error("Expected to find key1")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	// Test TTL expiration
	err = cache.Set(ctx, "ttl_key", "ttl_value", 100*time.Millisecond)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Should be available immediately
	value, found, err = cache.Get(ctx, "ttl_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !found {
		t.Error("Expected to find ttl_key")
	}
	if value != "ttl_value" {
		t.Errorf("Expected ttl_value, got %v", value)
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, found, err = cache.Get(ctx, "ttl_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if found {
		t.Error("Expected ttl_key to be expired")
	}
}

// TestLegacyCache tests legacy cache wrapper
func TestLegacyCache(t *testing.T) {
	config := DefaultCacheConfig()
	newCache := NewInMemoryCache(config)
	legacyCache := NewLegacyCache(newCache)

	// Test Set and Get
	legacyCache.Set("key1", 123.45)

	value, found := legacyCache.Get("key1")
	if !found {
		t.Error("Expected to find key1")
	}
	if value != 123.45 {
		t.Errorf("Expected 123.45, got %f", value)
	}

	// Test non-existent key
	_, found = legacyCache.Get("nonexistent")
	if found {
		t.Error("Expected not to find nonexistent key")
	}
}
