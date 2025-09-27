package health

import (
	"context"
	"errors"
	"testing"
)

func TestHealthChecker_Basic(t *testing.T) {
	checker := NewChecker()

	// Test empty checker (should be healthy)
	result := checker.HealthCheck(context.Background())
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}

	result = checker.ReadinessCheck(context.Background())
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy readiness status, got %s", result.Status)
	}
}

func TestLegacyCacheCheck_Basic(t *testing.T) {
	// Test with working cache
	cache := &mockCacheImpl{data: make(map[string]float64)}
	check := NewLegacyCacheCheck(cache)

	result := check.Check(context.Background())
	if result.Name != "legacy_cache" {
		t.Errorf("Expected name 'legacy_cache', got '%s'", result.Name)
	}
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}

	// Test with nil cache
	nilCheck := NewLegacyCacheCheck(nil)
	result = nilCheck.Check(context.Background())
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status for nil cache, got %s", result.Status)
	}
}

func TestKrakenCheck_Basic(t *testing.T) {
	// Test with working client
	client := &mockKrakenClient{err: nil}
	check := NewKrakenCheck(client)

	result := check.Check(context.Background())
	if result.Name != "kraken_api" {
		t.Errorf("Expected name 'kraken_api', got '%s'", result.Name)
	}
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}

	// Test with failing client
	failingClient := &mockKrakenClient{err: errors.New("API connection failed")}
	failingCheck := NewKrakenCheck(failingClient)

	result = failingCheck.Check(context.Background())
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status for failing client, got %s", result.Status)
	}
}

func TestChecker_WithChecks(t *testing.T) {
	checker := NewChecker()

	// Add a healthy check
	healthyCheck := &mockCheck{name: "test_healthy", healthy: true}
	checker.AddCheck(healthyCheck)

	result := checker.ReadinessCheck(context.Background())
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check result, got %d", len(result.Checks))
	}

	// Add an unhealthy check
	unhealthyCheck := &mockCheck{name: "test_unhealthy", healthy: false}
	checker.AddCheck(unhealthyCheck)

	result = checker.ReadinessCheck(context.Background())
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", result.Status)
	}

	if len(result.Checks) != 2 {
		t.Errorf("Expected 2 check results, got %d", len(result.Checks))
	}
}

// Mock implementations for testing
type mockCacheImpl struct {
	data map[string]float64
	fail bool
}

func (m *mockCacheImpl) Get(key string) (float64, bool) {
	if m.fail {
		return 0, false
	}
	val, ok := m.data[key]
	return val, ok
}

func (m *mockCacheImpl) Set(key string, value float64) {
	if m.fail {
		return
	}
	if m.data == nil {
		m.data = make(map[string]float64)
	}
	m.data[key] = value
}

type mockKrakenClient struct {
	err error
}

func (m *mockKrakenClient) GetTickerPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return map[string]float64{"XBTUSD": 50000.0}, nil
}

type mockCheck struct {
	name    string
	healthy bool
}

func (m *mockCheck) Check(ctx context.Context) CheckResult {
	if m.healthy {
		return CheckResult{
			Name:   m.name,
			Status: StatusHealthy,
		}
	}
	return CheckResult{
		Name:   m.name,
		Status: StatusUnhealthy,
		Error:  "mock check failure",
	}
}