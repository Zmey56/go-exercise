package backoff

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestBackoff_Retry_Success(t *testing.T) {
	config := Config{
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		MaxAttempts:  3,
		Multiplier:   2.0,
		JitterFactor: 0.1,
	}

	backoff := New(config)
	attempts := 0

	err := backoff.Retry(context.Background(), "test", func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil // Success on second attempt
	}, func(err error) bool {
		return true // Always retry
	})

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestBackoff_Retry_MaxAttemptsReached(t *testing.T) {
	config := Config{
		BaseDelay:    1 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		MaxAttempts:  3,
		Multiplier:   2.0,
		JitterFactor: 0.0, // No jitter for predictable timing
	}

	backoff := New(config)
	attempts := 0

	start := time.Now()
	err := backoff.Retry(context.Background(), "test", func() error {
		attempts++
		return errors.New("persistent error")
	}, func(err error) bool {
		return true // Always retry
	})

	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error after max attempts")
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	// Should have waited for delays: 1ms + 2ms = 3ms minimum
	// (no sleep after last attempt)
	if elapsed < 3*time.Millisecond {
		t.Errorf("expected at least 3ms elapsed, got %v", elapsed)
	}
}

func TestBackoff_Retry_NonRetryableError(t *testing.T) {
	config := DefaultConfig()
	backoff := New(config)
	attempts := 0

	err := backoff.Retry(context.Background(), "test", func() error {
		attempts++
		return errors.New("non-retryable error")
	}, func(err error) bool {
		return false // Never retry
	})

	if err == nil {
		t.Error("expected error to be returned")
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestBackoff_Retry_ContextCancellation(t *testing.T) {
	config := Config{
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		MaxAttempts:  5,
		Multiplier:   2.0,
		JitterFactor: 0.0,
	}

	backoff := New(config)
	attempts := 0

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := backoff.Retry(ctx, "test", func() error {
		attempts++
		return errors.New("error requiring retry")
	}, func(err error) bool {
		return true
	})

	if err != context.DeadlineExceeded {
		t.Errorf("expected context deadline exceeded, got: %v", err)
	}

	// Should only get 1 attempt before context timeout during sleep
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestBackoff_CalculateDelay(t *testing.T) {
	config := Config{
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		MaxAttempts:  5,
		Multiplier:   2.0,
		JitterFactor: 0.0, // No jitter for predictable results
	}

	backoff := New(config)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond}, // base delay
		{2, 200 * time.Millisecond}, // base * 2^1
		{3, 400 * time.Millisecond}, // base * 2^2
		{4, 800 * time.Millisecond}, // base * 2^3
		{5, 1 * time.Second},        // capped at max delay
		{10, 1 * time.Second},       // still capped at max delay
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := backoff.calculateDelay(tt.attempt)
			if delay != tt.expected {
				t.Errorf("expected delay %v, got %v", tt.expected, delay)
			}
		})
	}
}

func TestBackoff_CalculateDelayWithJitter(t *testing.T) {
	config := Config{
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		MaxAttempts:  3,
		Multiplier:   2.0,
		JitterFactor: 0.5, // 50% jitter
	}

	backoff := New(config)

	// Run multiple times to test jitter variability
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = backoff.calculateDelay(2) // Second attempt
	}

	// Check that delays vary (due to jitter)
	allSame := true
	first := delays[0]
	for _, delay := range delays[1:] {
		if delay != first {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("expected delays to vary due to jitter, but all were the same")
	}

	// Check that delays are within expected range
	// Base delay for attempt 2: 200ms
	// With 50% jitter: 100ms to 300ms
	for i, delay := range delays {
		if delay < 50*time.Millisecond || delay > 350*time.Millisecond {
			t.Errorf("delay %d (%v) outside expected jitter range", i, delay)
		}
	}
}

func TestHTTPError(t *testing.T) {
	err := NewHTTPError(429, "Too Many Requests")

	if err.StatusCode != 429 {
		t.Errorf("expected status code 429, got %d", err.StatusCode)
	}

	if err.Message != "Too Many Requests" {
		t.Errorf("expected message 'Too Many Requests', got %s", err.Message)
	}

	expected := "HTTP 429: Too Many Requests"
	if err.Error() != expected {
		t.Errorf("expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestIsRetryableHTTPError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "429 Too Many Requests",
			err:      NewHTTPError(429, "Too Many Requests"),
			expected: true,
		},
		{
			name:     "500 Internal Server Error",
			err:      NewHTTPError(500, "Internal Server Error"),
			expected: true,
		},
		{
			name:     "502 Bad Gateway",
			err:      NewHTTPError(502, "Bad Gateway"),
			expected: true,
		},
		{
			name:     "503 Service Unavailable",
			err:      NewHTTPError(503, "Service Unavailable"),
			expected: true,
		},
		{
			name:     "400 Bad Request",
			err:      NewHTTPError(400, "Bad Request"),
			expected: false,
		},
		{
			name:     "404 Not Found",
			err:      NewHTTPError(404, "Not Found"),
			expected: false,
		},
		{
			name:     "Non-HTTP error",
			err:      errors.New("network error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableHTTPError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.BaseDelay != 100*time.Millisecond {
		t.Errorf("expected base delay 100ms, got %v", config.BaseDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("expected max delay 30s, got %v", config.MaxDelay)
	}

	if config.MaxAttempts != 5 {
		t.Errorf("expected max attempts 5, got %d", config.MaxAttempts)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %f", config.Multiplier)
	}

	if config.JitterFactor != 0.1 {
		t.Errorf("expected jitter factor 0.1, got %f", config.JitterFactor)
	}
}

// Test that simulates Kraken API 429 responses
func TestBackoff_KrakenThrottling(t *testing.T) {
	config := Config{
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		MaxAttempts:  3,
		Multiplier:   2.0,
		JitterFactor: 0.1,
	}

	backoff := New(config)
	attempts := 0

	err := backoff.Retry(context.Background(), "kraken", func() error {
		attempts++
		if attempts <= 2 {
			// Simulate 429 response
			return NewHTTPError(http.StatusTooManyRequests, "Rate limit exceeded")
		}
		// Success on third attempt
		return nil
	}, IsRetryableHTTPError)

	if err != nil {
		t.Errorf("expected success after retries, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func BenchmarkBackoff_Retry(b *testing.B) {
	config := Config{
		BaseDelay:    1 * time.Microsecond,
		MaxDelay:     1 * time.Millisecond,
		MaxAttempts:  2,
		Multiplier:   2.0,
		JitterFactor: 0.0,
	}

	backoff := New(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backoff.Retry(context.Background(), "test", func() error {
			return nil // Immediate success
		}, func(err error) bool {
			return true
		})
	}
}