package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Zmey56/go-exercise/internal/metrics"
)

// Config holds rate limiter configuration
type Config struct {
	// Per-IP limits
	PerIPCapacity   int64 // tokens per IP
	PerIPRefillRate int64 // tokens per second per IP

	// Global limits
	GlobalCapacity   int64 // total tokens across all IPs
	GlobalRefillRate int64 // tokens per second globally

	// Cleanup settings
	IPCleanupInterval time.Duration // how often to clean up inactive IPs
	IPIdleTimeout     time.Duration // how long before an IP is considered inactive
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() Config {
	return Config{
		PerIPCapacity:     100,   // 100 requests per IP
		PerIPRefillRate:   10,    // 10 requests per second per IP
		GlobalCapacity:    1000,  // 1000 requests total
		GlobalRefillRate:  100,   // 100 requests per second globally
		IPCleanupInterval: 5 * time.Minute,
		IPIdleTimeout:     10 * time.Minute,
	}
}

// IPEntry holds per-IP rate limiting data
type IPEntry struct {
	bucket   *TokenBucket
	lastSeen time.Time
}

// RateLimiter manages both per-IP and global rate limiting
type RateLimiter struct {
	config       Config
	globalBucket *TokenBucket
	ipBuckets    map[string]*IPEntry
	mu           sync.RWMutex
	stopCleanup  chan struct{}
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config Config) *RateLimiter {
	rl := &RateLimiter{
		config:       config,
		globalBucket: NewTokenBucket(config.GlobalCapacity, config.GlobalRefillRate),
		ipBuckets:    make(map[string]*IPEntry),
		stopCleanup:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupRoutine()

	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	// Check global limit first (fail fast)
	if !rl.globalBucket.Allow() {
		metrics.RateLimitRejects.WithLabelValues("global").Inc()
		return false
	}

	// Check per-IP limit
	if !rl.allowIP(ip) {
		// Return token to global bucket since IP limit failed
		rl.returnGlobalToken()
		metrics.RateLimitRejects.WithLabelValues("per_ip").Inc()
		return false
	}

	metrics.RateLimitAllows.Inc()
	return true
}

// allowIP checks per-IP rate limit
func (rl *RateLimiter) allowIP(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create IP entry
	entry, exists := rl.ipBuckets[ip]
	if !exists {
		entry = &IPEntry{
			bucket:   NewTokenBucket(rl.config.PerIPCapacity, rl.config.PerIPRefillRate),
			lastSeen: now,
		}
		rl.ipBuckets[ip] = entry
		metrics.RateLimitIPsTracked.Set(float64(len(rl.ipBuckets)))
	}

	entry.lastSeen = now
	return entry.bucket.Allow()
}

// returnGlobalToken adds a token back to the global bucket (used when IP limit fails)
func (rl *RateLimiter) returnGlobalToken() {
	rl.globalBucket.mu.Lock()
	defer rl.globalBucket.mu.Unlock()

	if rl.globalBucket.tokens < rl.globalBucket.capacity {
		rl.globalBucket.tokens++
	}
}

// cleanupRoutine periodically removes inactive IP entries
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.config.IPCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanup removes IP entries that haven't been seen recently
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.config.IPIdleTimeout)

	for ip, entry := range rl.ipBuckets {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.ipBuckets, ip)
		}
	}

	metrics.RateLimitIPsTracked.Set(float64(len(rl.ipBuckets)))
}

// Stop shuts down the rate limiter cleanup routine
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats() Stats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	globalTokens := rl.globalBucket.Tokens()
	ipCount := len(rl.ipBuckets)

	return Stats{
		GlobalTokens:     globalTokens,
		GlobalCapacity:   rl.globalBucket.Capacity(),
		TrackedIPs:       ipCount,
		GlobalUtilization: float64(rl.globalBucket.Capacity()-globalTokens) / float64(rl.globalBucket.Capacity()),
	}
}

// Stats holds rate limiter statistics
type Stats struct {
	GlobalTokens      int64
	GlobalCapacity    int64
	TrackedIPs        int
	GlobalUtilization float64 // 0.0 to 1.0
}

// ExtractIP extracts the real IP address from an HTTP request
func ExtractIP(r *http.Request) string {
	// Check X-Forwarded-For header (most common proxy header)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote address
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}

	return r.RemoteAddr
}