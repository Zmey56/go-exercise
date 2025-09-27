package health

import (
	"context"
)

// LegacyCacheCheck provides health check for legacy cache interface
type LegacyCacheCheck struct {
	cache interface {
		Get(key string) (float64, bool)
	}
}

// NewLegacyCacheCheck creates a new legacy cache health check
func NewLegacyCacheCheck(cache interface {
	Get(key string) (float64, bool)
}) *LegacyCacheCheck {
	return &LegacyCacheCheck{cache: cache}
}

// Check performs health check on legacy cache
func (c *LegacyCacheCheck) Check(ctx context.Context) CheckResult {
	if c.cache == nil {
		return CheckResult{
			Name:   "cache",
			Status: StatusUnhealthy,
			Error:  "cache is nil",
		}
	}
	return CheckResult{
		Name:   "cache",
		Status: StatusHealthy,
	}
}

