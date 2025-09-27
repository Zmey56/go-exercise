package health

import (
	"context"
)

// LegacyCacheCheck provides health check for legacy cache interface
type LegacyCacheCheck struct {
	cache interface {
		Get(key string) (float64, bool)
		Set(key string, value float64)
	}
}

// NewLegacyCacheCheck creates a new legacy cache health check
func NewLegacyCacheCheck(cache interface {
	Get(key string) (float64, bool)
	Set(key string, value float64)
}) *LegacyCacheCheck {
	return &LegacyCacheCheck{cache: cache}
}

// Check performs health check on legacy cache
func (c *LegacyCacheCheck) Check(ctx context.Context) CheckResult {
	if c.cache == nil {
		return CheckResult{
			Name:   "legacy_cache",
			Status: StatusUnhealthy,
			Error:  "cache is nil",
		}
	}

	// Test write and read operations
	testKey := "health_check"
	testValue := 123.45

	// Try to write test data
	c.cache.Set(testKey, testValue)

	// Try to read test data back
	value, found := c.cache.Get(testKey)
	if !found || value != testValue {
		return CheckResult{
			Name:   "legacy_cache",
			Status: StatusUnhealthy,
			Error:  "cache read/write test failed",
		}
	}

	return CheckResult{
		Name:   "legacy_cache",
		Status: StatusHealthy,
	}
}

