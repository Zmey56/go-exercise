package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	tests := []struct {
		name       string
		capacity   int64
		refillRate int64
		requests   []int
		expected   []bool
		sleepBetween time.Duration
	}{
		{
			name:       "basic allow/deny",
			capacity:   2,
			refillRate: 1,
			requests:   []int{1, 1, 1}, // third should be denied
			expected:   []bool{true, true, false},
		},
		{
			name:       "refill allows more requests",
			capacity:   1,
			refillRate: 10, // 10 tokens per second
			requests:   []int{1, 1},
			expected:   []bool{true, true},
			sleepBetween: 150 * time.Millisecond, // Allow refill
		},
		{
			name:       "multiple tokens at once",
			capacity:   5,
			refillRate: 1,
			requests:   []int{3, 3}, // second should be denied (only 2 left)
			expected:   []bool{true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket := NewTokenBucket(tt.capacity, tt.refillRate)

			for i, n := range tt.requests {
				if i > 0 && tt.sleepBetween > 0 {
					time.Sleep(tt.sleepBetween)
				}

				var result bool
				if n == 1 {
					result = bucket.Allow()
				} else {
					result = bucket.AllowN(int64(n))
				}

				if result != tt.expected[i] {
					t.Errorf("request %d: expected %v, got %v", i, tt.expected[i], result)
				}
			}
		})
	}
}

func TestTokenBucket_Tokens(t *testing.T) {
	bucket := NewTokenBucket(5, 10)

	if tokens := bucket.Tokens(); tokens != 5 {
		t.Errorf("expected 5 initial tokens, got %d", tokens)
	}

	bucket.Allow()
	if tokens := bucket.Tokens(); tokens != 4 {
		t.Errorf("expected 4 tokens after consuming 1, got %d", tokens)
	}

	// Allow some refill time
	time.Sleep(200 * time.Millisecond)

	tokens := bucket.Tokens()
	if tokens <= 4 {
		t.Errorf("expected tokens to refill, but got %d", tokens)
	}
	if tokens > 5 {
		t.Errorf("expected tokens not to exceed capacity (5), but got %d", tokens)
	}
}

func TestTokenBucket_Capacity(t *testing.T) {
	bucket := NewTokenBucket(100, 50)
	if cap := bucket.Capacity(); cap != 100 {
		t.Errorf("expected capacity 100, got %d", cap)
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	bucket := NewTokenBucket(10, 20) // 20 tokens per second

	// Consume all tokens
	bucket.AllowN(10)
	if tokens := bucket.Tokens(); tokens != 0 {
		t.Errorf("expected 0 tokens after consuming all, got %d", tokens)
	}

	// Wait for refill (should get ~4 tokens in 200ms at 20 tokens/sec)
	time.Sleep(200 * time.Millisecond)

	tokens := bucket.Tokens()
	if tokens < 2 || tokens > 6 {
		t.Errorf("expected 2-6 tokens after refill, got %d", tokens)
	}
}

func BenchmarkTokenBucket_Allow(b *testing.B) {
	bucket := NewTokenBucket(1000000, 1000000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bucket.Allow()
		}
	})
}