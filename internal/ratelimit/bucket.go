package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	mu           sync.Mutex
	capacity     int64
	tokens       int64
	refillRate   int64 // tokens per second
	lastRefill   time.Time
	refillPeriod time.Duration
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(capacity, refillRate int64) *TokenBucket {
	now := time.Now()
	return &TokenBucket{
		capacity:     capacity,
		tokens:       capacity,
		refillRate:   refillRate,
		lastRefill:   now,
		refillPeriod: time.Second / time.Duration(refillRate),
	}
}

// Allow checks if a request can proceed and consumes a token if so
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN checks if n tokens are available and consumes them if so
func (tb *TokenBucket) AllowN(n int64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

// Tokens returns the current number of available tokens
func (tb *TokenBucket) Tokens() int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}

// Capacity returns the bucket capacity
func (tb *TokenBucket) Capacity() int64 {
	return tb.capacity
}

// refill adds tokens based on elapsed time (must be called with lock held)
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	if elapsed <= 0 {
		return
	}

	// Calculate tokens to add based on elapsed time and refill rate
	tokensToAdd := int64(elapsed.Seconds() * float64(tb.refillRate))

	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}
}