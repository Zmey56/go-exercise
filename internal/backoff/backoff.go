package backoff

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/Zmey56/go-exercise/internal/metrics"
)

// Config holds backoff configuration
type Config struct {
	BaseDelay    time.Duration // initial delay
	MaxDelay     time.Duration // maximum delay
	MaxAttempts  int           // maximum number of attempts
	Multiplier   float64       // delay multiplier (e.g., 2.0 for doubling)
	JitterFactor float64       // jitter factor (0.0 to 1.0)
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() Config {
	return Config{
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		MaxAttempts:  5,
		Multiplier:   2.0,
		JitterFactor: 0.1, // 10% jitter
	}
}

// Backoff implements exponential backoff with jitter
type Backoff struct {
	config Config
	rng    *rand.Rand
}

// New creates a new backoff instance
func New(config Config) *Backoff {
	return &Backoff{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// ShouldRetry determines if an error should trigger a retry
type ShouldRetry func(error) bool

// Retry executes a function with exponential backoff
func (b *Backoff) Retry(ctx context.Context, serviceName string, fn RetryableFunc, shouldRetry ShouldRetry) error {
	var lastErr error

	for attempt := 1; attempt <= b.config.MaxAttempts; attempt++ {
		attemptStr := strconv.Itoa(attempt)
		metrics.BackoffAttempts.WithLabelValues(serviceName, attemptStr).Inc()

		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if we should retry
		if !shouldRetry(err) {
			return err // Don't retry this error
		}

		// Don't sleep after the last attempt
		if attempt == b.config.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff and jitter
		delay := b.calculateDelay(attempt)

		// Record the delay
		metrics.BackoffDelay.WithLabelValues(serviceName, attemptStr).Observe(delay.Seconds())

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("max attempts (%d) reached, last error: %w", b.config.MaxAttempts, lastErr)
}

// calculateDelay computes the delay for a given attempt with jitter
func (b *Backoff) calculateDelay(attempt int) time.Duration {
	// Calculate base exponential delay
	delay := float64(b.config.BaseDelay) * math.Pow(b.config.Multiplier, float64(attempt-1))

	// Apply maximum delay limit
	if delay > float64(b.config.MaxDelay) {
		delay = float64(b.config.MaxDelay)
	}

	// Add jitter: ±jitterFactor of the delay
	if b.config.JitterFactor > 0 {
		jitter := delay * b.config.JitterFactor
		delay += (b.rng.Float64()*2 - 1) * jitter
	}

	// Ensure delay is not negative
	if delay < 0 {
		delay = float64(b.config.BaseDelay)
	}

	return time.Duration(delay)
}

// IsRetryableHTTPError checks if an HTTP error should be retried
func IsRetryableHTTPError(err error) bool {
	// Check for HTTP status codes that should be retried
	if httpErr, ok := err.(HTTPError); ok {
		code := httpErr.StatusCode
		// Retry on 429 (Too Many Requests) and 5xx server errors
		return code == http.StatusTooManyRequests || (code >= 500 && code < 600)
	}
	return false
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, message string) HTTPError {
	return HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}