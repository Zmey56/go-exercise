package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCache_GetSet(t *testing.T) {
	cache := New(1 * time.Minute)

	// Test setting and getting value
	cache.Set("key1", 100.0)

	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find key1 in cache")
	}
	if value != 100.0 {
		t.Errorf("expected value 100.0, got %f", value)
	}

	// Test getting non-existent key
	_, ok = cache.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent key")
	}
}

func TestCache_Expiration(t *testing.T) {
	cache := New(100 * time.Millisecond)

	// Set value
	cache.Set("key1", 100.0)

	// Check that value exists
	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find key1 in cache")
	}
	if value != 100.0 {
		t.Errorf("expected value 100.0, got %f", value)
	}

	// Wait for TTL expiration
	time.Sleep(150 * time.Millisecond)

	// Check that value has expired
	_, ok = cache.Get("key1")
	if ok {
		t.Error("expected key1 to be expired")
	}
}

func TestCache_Concurrency(t *testing.T) {
	cache := New(1 * time.Minute)

	// Concurrent access test
	var wg sync.WaitGroup
	numGoroutines := 100

	// Start goroutines for writing
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			cache.Set(key, float64(i))
		}(i)
	}

	// Start goroutines for reading
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			// Don't check result as write might not be completed yet
			cache.Get(key)
		}(i)
	}

	wg.Wait()

	// Check that all values were written
	for i := 0; i < numGoroutines; i++ {
		key := fmt.Sprintf("key%d", i)
		value, ok := cache.Get(key)
		if !ok {
			t.Errorf("expected to find %s in cache", key)
		}
		if value != float64(i) {
			t.Errorf("expected value %f for %s, got %f", float64(i), key, value)
		}
	}
}

func TestCache_Overwrite(t *testing.T) {
	cache := New(1 * time.Minute)

	// Set value
	cache.Set("key1", 100.0)

	// Overwrite value
	cache.Set("key1", 200.0)

	// Check new value
	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find key1 in cache")
	}
	if value != 200.0 {
		t.Errorf("expected value 200.0, got %f", value)
	}
}

func TestCache_ZeroValue(t *testing.T) {
	cache := New(1 * time.Minute)

	// Set zero value
	cache.Set("key1", 0.0)

	// Check that zero value is preserved
	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find key1 in cache")
	}
	if value != 0.0 {
		t.Errorf("expected value 0.0, got %f", value)
	}
}

func TestCache_NegativeValue(t *testing.T) {
	cache := New(1 * time.Minute)

	// Set negative value
	cache.Set("key1", -100.0)

	// Check negative value
	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find key1 in cache")
	}
	if value != -100.0 {
		t.Errorf("expected value -100.0, got %f", value)
	}
}

func TestCache_MultipleKeys(t *testing.T) {
	cache := New(1 * time.Minute)

	// Set multiple keys
	cache.Set("key1", 100.0)
	cache.Set("key2", 200.0)
	cache.Set("key3", 300.0)

	// Check all keys
	expected := map[string]float64{
		"key1": 100.0,
		"key2": 200.0,
		"key3": 300.0,
	}

	for key, expectedValue := range expected {
		value, ok := cache.Get(key)
		if !ok {
			t.Errorf("expected to find %s in cache", key)
		}
		if value != expectedValue {
			t.Errorf("expected value %f for %s, got %f", expectedValue, key, value)
		}
	}
}
