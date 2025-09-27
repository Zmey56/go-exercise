package cache

import (
	"context"
	"testing"
	"time"
)

// TestRedisURLParsing tests Redis URL parsing functionality
func TestRedisURLParsing(t *testing.T) {
	tests := []struct {
		name            string
		redisURL        string
		defaultPassword string
		defaultDB       int
		expectedAddr    string
		expectedPass    string
		expectedDB      int
		expectError     bool
	}{
		{
			name:            "Simple host:port",
			redisURL:        "localhost:6379",
			defaultPassword: "",
			defaultDB:       0,
			expectedAddr:    "localhost:6379",
			expectedPass:    "",
			expectedDB:      0,
			expectError:     false,
		},
		{
			name:            "Redis URL without auth",
			redisURL:        "redis://localhost:6379",
			defaultPassword: "",
			defaultDB:       0,
			expectedAddr:    "localhost:6379",
			expectedPass:    "",
			expectedDB:      0,
			expectError:     false,
		},
		{
			name:            "Redis URL with database",
			redisURL:        "redis://localhost:6379/1",
			defaultPassword: "",
			defaultDB:       0,
			expectedAddr:    "localhost:6379",
			expectedPass:    "",
			expectedDB:      1,
			expectError:     false,
		},
		{
			name:            "Redis URL with password",
			redisURL:        "redis://:mypassword@localhost:6379",
			defaultPassword: "",
			defaultDB:       0,
			expectedAddr:    "localhost:6379",
			expectedPass:    "mypassword",
			expectedDB:      0,
			expectError:     false,
		},
		{
			name:            "Redis URL with username and password",
			redisURL:        "redis://user:mypassword@localhost:6379/2",
			defaultPassword: "",
			defaultDB:       0,
			expectedAddr:    "localhost:6379",
			expectedPass:    "mypassword",
			expectedDB:      2,
			expectError:     false,
		},
		{
			name:            "Docker compose format",
			redisURL:        "redis://redis:6379",
			defaultPassword: "",
			defaultDB:       0,
			expectedAddr:    "redis:6379",
			expectedPass:    "",
			expectedDB:      0,
			expectError:     false,
		},
		{
			name:            "Invalid URL",
			redisURL:        "://invalid",
			defaultPassword: "",
			defaultDB:       0,
			expectedAddr:    "",
			expectedPass:    "",
			expectedDB:      0,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, password, db, err := parseRedisURL(tt.redisURL, tt.defaultPassword, tt.defaultDB)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if addr != tt.expectedAddr {
				t.Errorf("Expected addr %s, got %s", tt.expectedAddr, addr)
			}

			if password != tt.expectedPass {
				t.Errorf("Expected password %s, got %s", tt.expectedPass, password)
			}

			if db != tt.expectedDB {
				t.Errorf("Expected DB %d, got %d", tt.expectedDB, db)
			}
		})
	}
}

// TestRedisIntegration tests Redis cache with actual Redis instance (if available)
// This test will be skipped if Redis is not available
func TestRedisIntegration(t *testing.T) {
	config := DefaultCacheConfig()
	config.RedisURL = "redis://localhost:6379"
	config.RedisDB = 1 // Use DB 1 for testing

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available, skipping integration test: %v", err)
		return
	}
	defer cache.Close()

	ctx := context.Background()

	// Clean up any existing test data
	cache.Clear(ctx)

	t.Run("BasicOperations", func(t *testing.T) {
		// Test Set and Get
		err := cache.Set(ctx, "redis_key", "redis_value", time.Minute)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		value, found, err := cache.Get(ctx, "redis_key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !found {
			t.Error("Expected to find redis_key")
		}
		if value != "redis_value" {
			t.Errorf("Expected redis_value, got %v", value)
		}
	})

	t.Run("TTLOperations", func(t *testing.T) {
		// Test TTL
		err := cache.Set(ctx, "ttl_key", "ttl_value", 2*time.Second)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		ttl, err := cache.TTL(ctx, "ttl_key")
		if err != nil {
			t.Fatalf("TTL failed: %v", err)
		}
		if ttl <= 0 || ttl > 2*time.Second {
			t.Errorf("Expected TTL between 0 and 2s, got %v", ttl)
		}

		// Test expire (Redis minimum is 1 second)
		err = cache.Expire(ctx, "ttl_key", 1*time.Second)
		if err != nil {
			t.Fatalf("Expire failed: %v", err)
		}

		// Wait for expiration
		time.Sleep(1100 * time.Millisecond)

		// Should be expired
		_, found, err := cache.Get(ctx, "ttl_key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if found {
			t.Error("Expected ttl_key to be expired")
		}
	})

	t.Run("MultipleOperations", func(t *testing.T) {
		items := map[string]interface{}{
			"redis_multi1": "value1",
			"redis_multi2": "value2",
			"redis_multi3": "value3",
		}

		// Test SetMultiple
		err := cache.SetMultiple(ctx, items, time.Hour)
		if err != nil {
			t.Fatalf("SetMultiple failed: %v", err)
		}

		// Test GetMultiple
		keys := []string{"redis_multi1", "redis_multi2", "redis_multi3", "missing"}
		result, err := cache.GetMultiple(ctx, keys)
		if err != nil {
			t.Fatalf("GetMultiple failed: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 results, got %d", len(result))
		}

		for key, expectedValue := range items {
			if result[key] != expectedValue {
				t.Errorf("Expected %v for key %s, got %v", expectedValue, key, result[key])
			}
		}
	})

	t.Run("Metadata", func(t *testing.T) {
		// Test SetWithMetadata and GetWithMetadata
		entry := &CacheEntry{
			Value:     "metadata_value",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		err := cache.SetWithMetadata(ctx, "metadata_key", entry)
		if err != nil {
			t.Fatalf("SetWithMetadata failed: %v", err)
		}

		retrievedEntry, found, err := cache.GetWithMetadata(ctx, "metadata_key")
		if err != nil {
			t.Fatalf("GetWithMetadata failed: %v", err)
		}
		if !found {
			t.Error("Expected to find metadata_key")
		}
		if retrievedEntry.Value != "metadata_value" {
			t.Errorf("Expected metadata_value, got %v", retrievedEntry.Value)
		}
	})

	t.Run("Size", func(t *testing.T) {
		// Clear and test size
		cache.Clear(ctx)

		size, err := cache.Size(ctx)
		if err != nil {
			t.Fatalf("Size failed: %v", err)
		}
		if size != 0 {
			t.Errorf("Expected size 0 after clear, got %d", size)
		}

		// Add items and check size
		cache.Set(ctx, "size_key1", "value1", time.Hour)
		cache.Set(ctx, "size_key2", "value2", time.Hour)

		size, err = cache.Size(ctx)
		if err != nil {
			t.Fatalf("Size failed: %v", err)
		}
		if size < 2 {
			t.Errorf("Expected size >= 2, got %d", size)
		}
	})

	t.Run("Ping", func(t *testing.T) {
		err := cache.Ping(ctx)
		if err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})
}

// TestFallbackIntegration tests fallback cache with Redis
func TestFallbackIntegration(t *testing.T) {
	config := DefaultCacheConfig()
	config.RedisURL = "redis://localhost:6379"
	config.RedisDB = 2 // Use DB 2 for testing
	config.EnableFallback = true

	// Try to create Redis cache
	redisCache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available, skipping fallback integration test: %v", err)
		return
	}

	fallbackCache := NewFallbackCache(redisCache, config)
	defer fallbackCache.Close()

	ctx := context.Background()

	// Clean up any existing test data
	fallbackCache.Clear(ctx)

	t.Run("FallbackOperations", func(t *testing.T) {
		// Test normal operation (should use Redis)
		err := fallbackCache.Set(ctx, "fallback_key", "fallback_value", time.Hour)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		value, found, err := fallbackCache.Get(ctx, "fallback_key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !found {
			t.Error("Expected to find fallback_key")
		}
		if value != "fallback_value" {
			t.Errorf("Expected fallback_value, got %v", value)
		}

		// Test size (should include both Redis and memory)
		size, err := fallbackCache.Size(ctx)
		if err != nil {
			t.Fatalf("Size failed: %v", err)
		}
		if size < 1 {
			t.Errorf("Expected size >= 1, got %d", size)
		}
	})

	t.Run("Ping", func(t *testing.T) {
		err := fallbackCache.Ping(ctx)
		if err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})
}