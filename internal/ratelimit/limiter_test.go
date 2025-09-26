package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	config := Config{
		PerIPCapacity:   2,
		PerIPRefillRate: 1,
		GlobalCapacity:  5,
		GlobalRefillRate: 1,
		IPCleanupInterval: 1 * time.Minute,
		IPIdleTimeout:     2 * time.Minute,
	}

	limiter := NewRateLimiter(config)
	defer limiter.Stop()

	// Test per-IP limits
	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// IP1: should allow 2 requests, then deny
	if !limiter.Allow(ip1) {
		t.Error("expected first request from IP1 to be allowed")
	}
	if !limiter.Allow(ip1) {
		t.Error("expected second request from IP1 to be allowed")
	}
	if limiter.Allow(ip1) {
		t.Error("expected third request from IP1 to be denied")
	}

	// IP2: should allow 2 more requests (different IP)
	if !limiter.Allow(ip2) {
		t.Error("expected first request from IP2 to be allowed")
	}
	if !limiter.Allow(ip2) {
		t.Error("expected second request from IP2 to be allowed")
	}

	// Global limit: only 1 more request allowed (5 total - 4 used)
	if !limiter.Allow("192.168.1.3") {
		t.Error("expected request from IP3 to be allowed (within global limit)")
	}
	if limiter.Allow("192.168.1.4") {
		t.Error("expected request from IP4 to be denied (global limit exceeded)")
	}
}

func TestRateLimiter_GlobalLimitFirst(t *testing.T) {
	config := Config{
		PerIPCapacity:     100, // High per-IP limit
		PerIPRefillRate:   100,
		GlobalCapacity:    2,   // Low global limit
		GlobalRefillRate:  1,
		IPCleanupInterval: 1 * time.Minute,
		IPIdleTimeout:     2 * time.Minute,
	}

	limiter := NewRateLimiter(config)
	defer limiter.Stop()

	// Should be limited by global limit, not per-IP
	ip := "192.168.1.1"

	if !limiter.Allow(ip) {
		t.Error("expected first request to be allowed")
	}
	if !limiter.Allow(ip) {
		t.Error("expected second request to be allowed")
	}
	if limiter.Allow(ip) {
		t.Error("expected third request to be denied (global limit)")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	config := Config{
		PerIPCapacity:     10,
		PerIPRefillRate:   10,
		GlobalCapacity:    100,
		GlobalRefillRate:  100,
		IPCleanupInterval: 100 * time.Millisecond, // Fast cleanup for testing
		IPIdleTimeout:     200 * time.Millisecond, // Short timeout
	}

	limiter := NewRateLimiter(config)
	defer limiter.Stop()

	// Make requests from multiple IPs
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}
	for _, ip := range ips {
		limiter.Allow(ip)
	}

	// Check that IPs are tracked
	stats := limiter.GetStats()
	if stats.TrackedIPs != 3 {
		t.Errorf("expected 3 tracked IPs, got %d", stats.TrackedIPs)
	}

	// Wait for cleanup
	time.Sleep(300 * time.Millisecond)

	// Check that IPs are cleaned up
	stats = limiter.GetStats()
	if stats.TrackedIPs != 0 {
		t.Errorf("expected 0 tracked IPs after cleanup, got %d", stats.TrackedIPs)
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	config := DefaultConfig()
	limiter := NewRateLimiter(config)
	defer limiter.Stop()

	initialStats := limiter.GetStats()
	if initialStats.GlobalTokens != config.GlobalCapacity {
		t.Errorf("expected %d global tokens initially, got %d",
			config.GlobalCapacity, initialStats.GlobalTokens)
	}
	if initialStats.TrackedIPs != 0 {
		t.Errorf("expected 0 tracked IPs initially, got %d", initialStats.TrackedIPs)
	}
	if initialStats.GlobalUtilization != 0.0 {
		t.Errorf("expected 0.0 utilization initially, got %f", initialStats.GlobalUtilization)
	}

	// Make some requests
	for i := 0; i < 5; i++ {
		limiter.Allow("192.168.1.1")
	}

	stats := limiter.GetStats()
	if stats.TrackedIPs != 1 {
		t.Errorf("expected 1 tracked IP, got %d", stats.TrackedIPs)
	}
	if stats.GlobalUtilization <= 0.0 {
		t.Errorf("expected positive utilization, got %f", stats.GlobalUtilization)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remoteAddr string
		expected string
	}{
		{
			name: "X-Forwarded-For header",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			expected:   "192.168.1.100",
		},
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "192.168.1.200",
			},
			remoteAddr: "10.0.0.1:12345",
			expected:   "192.168.1.200",
		},
		{
			name:       "RemoteAddr fallback",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.300:54321",
			expected:   "192.168.1.300",
		},
		{
			name:       "RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.400",
			expected:   "192.168.1.400",
		},
		{
			name: "X-Forwarded-For takes precedence",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.500",
				"X-Real-IP":       "192.168.1.600",
			},
			remoteAddr: "10.0.0.1:12345",
			expected:   "192.168.1.500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header:     make(http.Header),
				RemoteAddr: tt.remoteAddr,
			}

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := ExtractIP(req)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	config := Config{
		PerIPCapacity:     1000000,
		PerIPRefillRate:   1000000,
		GlobalCapacity:    10000000,
		GlobalRefillRate:  10000000,
		IPCleanupInterval: 1 * time.Hour, // Disable cleanup for benchmark
		IPIdleTimeout:     1 * time.Hour,
	}

	limiter := NewRateLimiter(config)
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ip := "192.168.1.1"
		for pb.Next() {
			limiter.Allow(ip)
		}
	})
}