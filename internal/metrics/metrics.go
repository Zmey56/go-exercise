package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request metrics
	HTTPRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status"},
	)

	// Kraken API metrics
	KrakenRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kraken_requests_total",
			Help: "Total number of Kraken API requests",
		},
		[]string{"status"},
	)

	KrakenDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kraken_request_duration_seconds",
			Help:    "Kraken API request latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	// Cache metrics
	CacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	CacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	CacheSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cache_size",
			Help: "Current number of items in cache",
		},
	)

	// Rate limiting metrics
	RateLimitAllows = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "rate_limit_allows_total",
			Help: "Total number of requests allowed by rate limiter",
		},
	)

	RateLimitRejects = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_rejects_total",
			Help: "Total number of requests rejected by rate limiter",
		},
		[]string{"type"}, // "global" or "per_ip"
	)

	RateLimitIPsTracked = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "rate_limit_ips_tracked",
			Help: "Current number of IPs being tracked by rate limiter",
		},
	)

	RateLimitGlobalTokens = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "rate_limit_global_tokens",
			Help: "Current number of available global rate limit tokens",
		},
	)

	RateLimitGlobalUtilization = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "rate_limit_global_utilization",
			Help: "Global rate limit utilization (0.0 to 1.0)",
		},
	)

	// Exponential backoff metrics
	BackoffAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "backoff_attempts_total",
			Help: "Total number of backoff attempts",
		},
		[]string{"service", "attempt"}, // service name and attempt number
	)

	BackoffDelay = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "backoff_delay_seconds",
			Help:    "Backoff delay duration in seconds",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 4, 8, 16, 32},
		},
		[]string{"service", "attempt"},
	)
)