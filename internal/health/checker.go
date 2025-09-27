package health

import (
	"context"
	"time"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
)

type CheckResult struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Response struct {
	Status Status        `json:"status"`
	Checks []CheckResult `json:"checks,omitempty"`
}

type Check interface {
	Check(ctx context.Context) CheckResult
}

type Checker struct {
	checks []Check
}

func NewChecker() *Checker {
	return &Checker{checks: make([]Check, 0)}
}

func (c *Checker) AddCheck(check Check) {
	c.checks = append(c.checks, check)
}

func (c *Checker) HealthCheck(ctx context.Context) Response {
	return Response{Status: StatusHealthy}
}

func (c *Checker) ReadinessCheck(ctx context.Context) Response {
	if len(c.checks) == 0 {
		return Response{Status: StatusHealthy}
	}

	results := make([]CheckResult, 0, len(c.checks))
	overallStatus := StatusHealthy

	for _, check := range c.checks {
		result := check.Check(ctx)
		results = append(results, result)
		if result.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		}
	}

	return Response{
		Status: overallStatus,
		Checks: results,
	}
}

type CacheCheck struct {
	cache interface {
		Get(key string) (float64, bool)
	}
}

func NewCacheCheck(cache interface {
	Get(key string) (float64, bool)
}) *CacheCheck {
	return &CacheCheck{cache: cache}
}

func (c *CacheCheck) Check(ctx context.Context) CheckResult {
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

type KrakenCheck struct {
	client interface {
		GetTickerPrices(ctx context.Context, symbols []string) (map[string]float64, error)
	}
}

func NewKrakenCheck(client interface {
	GetTickerPrices(ctx context.Context, symbols []string) (map[string]float64, error)
}) *KrakenCheck {
	return &KrakenCheck{client: client}
}

func (k *KrakenCheck) Check(ctx context.Context) CheckResult {
	if k.client == nil {
		return CheckResult{
			Name:   "kraken",
			Status: StatusUnhealthy,
			Error:  "kraken client is nil",
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := k.client.GetTickerPrices(checkCtx, []string{"XBTUSD"})
	if err != nil {
		return CheckResult{
			Name:   "kraken",
			Status: StatusUnhealthy,
			Error:  err.Error(),
		}
	}

	return CheckResult{
		Name:   "kraken",
		Status: StatusHealthy,
	}
}
