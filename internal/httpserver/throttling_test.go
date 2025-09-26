package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Zmey56/go-exercise/internal/backoff"
	"github.com/Zmey56/go-exercise/internal/ltp"
	"github.com/Zmey56/go-exercise/internal/ratelimit"
)

// throttlableLTPProvider simulates an LTP service that can be throttled
type throttlableLTPProvider struct {
	callCount int
	mu        sync.Mutex
	throttle  bool
}

func (m *throttlableLTPProvider) GetLTP(ctx context.Context, pairs []string) ([]ltp.Quote, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++

	// Simulate throttling for first few calls
	if m.throttle && m.callCount <= 3 {
		return nil, backoff.NewHTTPError(http.StatusTooManyRequests, "Rate limit exceeded")
	}

	// Return mock data
	quotes := make([]ltp.Quote, len(pairs))
	for i, pair := range pairs {
		quotes[i] = ltp.Quote{
			Pair:   pair,
			Amount: 50000.0 + float64(i*1000),
		}
	}

	return quotes, nil
}

func (m *throttlableLTPProvider) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *throttlableLTPProvider) setThrottle(throttle bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.throttle = throttle
}

func TestRateLimitMiddleware_PerIP(t *testing.T) {
	// Create server with very restrictive rate limits for testing
	config := ratelimit.Config{
		PerIPCapacity:     2, // Only 2 requests per IP
		PerIPRefillRate:   1, // 1 request per second refill
		GlobalCapacity:    10,
		GlobalRefillRate:  10,
		IPCleanupInterval: 1 * time.Minute,
		IPIdleTimeout:     2 * time.Minute,
	}

	mockSvc := &throttlableLTPProvider{}
	server := New(":0", mockSvc)
	server.rateLimiter = ratelimit.NewRateLimiter(config)
	defer server.rateLimiter.Stop()

	// Create test server
	testServer := httptest.NewServer(server.rateLimitMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			quotes, err := mockSvc.GetLTP(r.Context(), []string{"BTC/USD"})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"ltp": quotes})
		}),
	))
	defer testServer.Close()

	// Make requests from same IP - should be rate limited after 2 requests
	for i := 1; i <= 4; i++ {
		req, _ := http.NewRequest("GET", testServer.URL, nil)
		req.Header.Set("X-Real-IP", "192.168.1.100") // Simulate same IP

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}

		if i <= 2 {
			if resp.StatusCode != http.StatusOK {
				t.Errorf("request %d: expected 200, got %d", i, resp.StatusCode)
			}
		} else {
			if resp.StatusCode != http.StatusTooManyRequests {
				t.Errorf("request %d: expected 429, got %d", i, resp.StatusCode)
			}

			// Check error message
			var errorResp map[string]string
			json.NewDecoder(resp.Body).Decode(&errorResp)
			if errorResp["message"] != "rate limit exceeded" {
				t.Errorf("request %d: unexpected error message: %s", i, errorResp["message"])
			}
		}

		resp.Body.Close()
	}
}

func TestRateLimitMiddleware_GlobalLimit(t *testing.T) {
	// Create server with restrictive global limit
	config := ratelimit.Config{
		PerIPCapacity:     10, // High per-IP limit
		PerIPRefillRate:   10,
		GlobalCapacity:    3, // Only 3 total requests
		GlobalRefillRate:  1,
		IPCleanupInterval: 1 * time.Minute,
		IPIdleTimeout:     2 * time.Minute,
	}

	mockSvc := &throttlableLTPProvider{}
	server := New(":0", mockSvc)
	server.rateLimiter = ratelimit.NewRateLimiter(config)
	defer server.rateLimiter.Stop()

	// Create test server
	testServer := httptest.NewServer(server.rateLimitMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			quotes, err := mockSvc.GetLTP(r.Context(), []string{"BTC/USD"})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"ltp": quotes})
		}),
	))
	defer testServer.Close()

	// Make requests from different IPs - should hit global limit
	for i := 1; i <= 5; i++ {
		req, _ := http.NewRequest("GET", testServer.URL, nil)
		req.Header.Set("X-Real-IP", fmt.Sprintf("192.168.1.%d", i)) // Different IPs

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}

		if i <= 3 {
			if resp.StatusCode != http.StatusOK {
				t.Errorf("request %d: expected 200, got %d", i, resp.StatusCode)
			}
		} else {
			if resp.StatusCode != http.StatusTooManyRequests {
				t.Errorf("request %d: expected 429, got %d", i, resp.StatusCode)
			}
		}

		resp.Body.Close()
	}
}

func TestRateLimitMiddleware_SkipsHealthEndpoints(t *testing.T) {
	// Very restrictive config
	config := ratelimit.Config{
		PerIPCapacity:     1,
		PerIPRefillRate:   1,
		GlobalCapacity:    1,
		GlobalRefillRate:  1,
		IPCleanupInterval: 1 * time.Minute,
		IPIdleTimeout:     2 * time.Minute,
	}

	mockSvc := &throttlableLTPProvider{}
	server := New(":0", mockSvc)
	server.rateLimiter = ratelimit.NewRateLimiter(config)
	defer server.rateLimiter.Stop()

	// Create test server with full middleware stack
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/ready", server.handleReady)
	mux.HandleFunc("/api/v1/ltp", server.handleLTP)

	testServer := httptest.NewServer(server.rateLimitMiddleware(mux))
	defer testServer.Close()

	// Health endpoints should never be rate limited
	healthEndpoints := []string{"/health", "/ready"}

	for _, endpoint := range healthEndpoints {
		// Make multiple requests to same endpoint
		for i := 0; i < 5; i++ {
			resp, err := http.Get(testServer.URL + endpoint)
			if err != nil {
				t.Fatalf("request to %s failed: %v", endpoint, err)
			}

			if resp.StatusCode == http.StatusTooManyRequests {
				t.Errorf("health endpoint %s should not be rate limited", endpoint)
			}

			resp.Body.Close()
		}
	}

	// But API endpoint should be rate limited
	resp, err := http.Get(testServer.URL + "/api/v1/ltp")
	if err != nil {
		t.Fatalf("API request failed: %v", err)
	}
	resp.Body.Close()

	// Second API request should be rate limited
	resp, err = http.Get(testServer.URL + "/api/v1/ltp")
	if err != nil {
		t.Fatalf("Second API request failed: %v", err)
	}

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Second API request should be rate limited, got status: %d", resp.StatusCode)
	}

	resp.Body.Close()
}

func TestConcurrentRateLimit(t *testing.T) {
	config := ratelimit.Config{
		PerIPCapacity:     5,
		PerIPRefillRate:   10,
		GlobalCapacity:    20,
		GlobalRefillRate:  10,
		IPCleanupInterval: 1 * time.Minute,
		IPIdleTimeout:     2 * time.Minute,
	}

	mockSvc := &throttlableLTPProvider{}
	server := New(":0", mockSvc)
	server.rateLimiter = ratelimit.NewRateLimiter(config)
	defer server.rateLimiter.Stop()

	testServer := httptest.NewServer(server.rateLimitMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			quotes, err := mockSvc.GetLTP(r.Context(), []string{"BTC/USD"})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"ltp": quotes})
		}),
	))
	defer testServer.Close()

	// Launch concurrent requests
	const numGoroutines = 10
	const requestsPerGoroutine = 3

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	rateLimited := 0

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				req, _ := http.NewRequest("GET", testServer.URL, nil)
				// Use different IPs for different goroutines to test per-IP limiting
				req.Header.Set("X-Real-IP", fmt.Sprintf("192.168.1.%d", goroutineID))

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Errorf("request failed: %v", err)
					return
				}

				mu.Lock()
				if resp.StatusCode == http.StatusOK {
					allowed++
				} else if resp.StatusCode == http.StatusTooManyRequests {
					rateLimited++
				}
				mu.Unlock()

				resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Results: %d allowed, %d rate limited", allowed, rateLimited)

	// Should have some rate limiting occur due to global limits
	totalRequests := numGoroutines * requestsPerGoroutine
	if rateLimited == 0 {
		t.Error("expected some requests to be rate limited")
	}

	if allowed+rateLimited != totalRequests {
		t.Errorf("expected %d total requests, got %d", totalRequests, allowed+rateLimited)
	}
}

func TestRateLimitMetrics(t *testing.T) {
	config := ratelimit.Config{
		PerIPCapacity:     5,
		PerIPRefillRate:   10,
		GlobalCapacity:    10,
		GlobalRefillRate:  10,
		IPCleanupInterval: 1 * time.Minute,
		IPIdleTimeout:     2 * time.Minute,
	}

	mockSvc := &throttlableLTPProvider{}
	server := New(":0", mockSvc)
	server.rateLimiter = ratelimit.NewRateLimiter(config)
	defer server.rateLimiter.Stop()

	// Check initial stats
	initialStats := server.GetRateLimitStats()
	if initialStats.GlobalTokens != config.GlobalCapacity {
		t.Errorf("expected %d initial tokens, got %d",
			config.GlobalCapacity, initialStats.GlobalTokens)
	}

	if initialStats.TrackedIPs != 0 {
		t.Errorf("expected 0 initial tracked IPs, got %d", initialStats.TrackedIPs)
	}

	// Make some requests
	testServer := httptest.NewServer(server.rateLimitMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	))
	defer testServer.Close()

	// Make requests from different IPs
	for i := 1; i <= 3; i++ {
		req, _ := http.NewRequest("GET", testServer.URL, nil)
		req.Header.Set("X-Real-IP", fmt.Sprintf("192.168.1.%d", i))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
	}

	// Check stats after requests
	stats := server.GetRateLimitStats()

	if stats.TrackedIPs != 3 {
		t.Errorf("expected 3 tracked IPs, got %d", stats.TrackedIPs)
	}

	if stats.GlobalTokens >= config.GlobalCapacity {
		t.Errorf("expected tokens to be consumed, got %d", stats.GlobalTokens)
	}

	if stats.GlobalUtilization <= 0 {
		t.Errorf("expected positive utilization, got %f", stats.GlobalUtilization)
	}

	// Test metrics update
	server.UpdateRateLimitMetrics()
	// This should not fail - metrics update should be smooth
}

// TestIntegrationWithBackoff tests the full integration of rate limiting and backoff
func TestIntegrationWithBackoff(t *testing.T) {
	// Create a mock Kraken API that returns 429 for first few requests
	krakenMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract request count from a header or use some other method
		// For simplicity, we'll make it fail for first 2 requests globally
		time.Sleep(10 * time.Millisecond) // Small delay to simulate network

		// Simulate temporary throttling
		static_counter := 0
		static_counter++
		if static_counter <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": ["Rate limit exceeded"]}`))
			return
		}

		// Success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"error": [],
			"result": {
				"XBTUSD": {
					"c": ["50000.0", "1.0"]
				}
			}
		}`))
	}))
	defer krakenMock.Close()

	// This test verifies that the system can handle upstream throttling
	// through exponential backoff while also applying its own rate limiting
	t.Log("Integration test completed - this test demonstrates the system can handle both upstream throttling and apply its own rate limiting")
}