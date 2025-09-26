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
)