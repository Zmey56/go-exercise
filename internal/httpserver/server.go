package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Zmey56/go-exercise/internal/health"
	"github.com/Zmey56/go-exercise/internal/ltp"
	"github.com/Zmey56/go-exercise/internal/metrics"
	"github.com/Zmey56/go-exercise/internal/ratelimit"
	"github.com/Zmey56/go-exercise/internal/resp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// LTPProvider — minimal interface for the service
// (easier to mock in tests if needed)
type LTPProvider interface {
	GetLTP(ctx context.Context, pairs []string) ([]ltp.Quote, error)
}

type Server struct {
	addr          string
	svc           LTPProvider
	http          *http.Server
	healthChecker *health.Checker
	rateLimiter   *ratelimit.RateLimiter
}

func New(addr string, svc LTPProvider) *Server {
	mux := http.NewServeMux()
	s := &Server{
		addr:          addr,
		svc:           svc,
		healthChecker: health.NewChecker(),
		rateLimiter:   ratelimit.NewRateLimiter(ratelimit.DefaultConfig()),
	}

	// API endpoints
	mux.HandleFunc("/api/v1/ltp", s.handleLTP)

	// Health endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Add middleware: CORS, logging, recovery, metrics, rate limiting
	handler := cors(logging(s.rateLimitMiddleware(metricsMiddleware(recovery(mux)))))

	s.http = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return s
}

func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.rateLimiter.Stop()
	return s.http.Shutdown(ctx)
}

func (s *Server) AddHealthCheck(check health.Check) {
	s.healthChecker.AddCheck(check)
}

func (s *Server) handleLTP(w http.ResponseWriter, r *http.Request) {
	// Support only GET requests
	if r.Method != http.MethodGet {
		resp.JSON(w, http.StatusMethodNotAllowed, resp.Error{Message: "method not allowed"})
		return
	}

	// Support ?pair=BTC/USD OR ?pairs=BTC/USD,BTC/EUR
	// If parameters are not specified, return all supported pairs
	q := r.URL.Query()
	var pairs []string

	if p := q.Get("pair"); p != "" {
		pairs = []string{p}
	} else if ps := q.Get("pairs"); ps != "" {
		parts := strings.Split(ps, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				pairs = append(pairs, p)
			}
		}
	} else {
		// If parameters are not specified, return all supported pairs
		pairs = []string{"BTC/USD", "BTC/CHF", "BTC/EUR"}
	}

	if len(pairs) == 0 {
		resp.JSON(w, http.StatusBadRequest, resp.Error{Message: "query param 'pair' or 'pairs' is required"})
		return
	}

	ctx := r.Context()
	quotes, err := s.svc.GetLTP(ctx, pairs)
	if err != nil {
		var herr *ltp.HandlerError
		if errors.As(err, &herr) {
			resp.JSON(w, herr.Code, resp.Error{Message: herr.Error()})
			return
		}
		log.Printf("svc error: %v", err)
		resp.JSON(w, http.StatusInternalServerError, resp.Error{Message: "internal error"})
		return
	}

	// Response format {"ltp": [{"pair":"BTC/CHF","amount":...}, ...]}
	out := struct {
		LTP []ltp.Quote `json:"ltp"`
	}{LTP: quotes}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		resp.JSON(w, http.StatusMethodNotAllowed, resp.Error{Message: "method not allowed"})
		return
	}

	response := s.healthChecker.HealthCheck(r.Context())
	statusCode := http.StatusOK
	if response.Status == health.StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		resp.JSON(w, http.StatusMethodNotAllowed, resp.Error{Message: "method not allowed"})
		return
	}

	response := s.healthChecker.ReadinessCheck(r.Context())
	statusCode := http.StatusOK
	if response.Status == health.StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// cors adds CORS headers
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// logging adds request logging
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.String(), time.Since(start))
	})
}

// recovery adds panic handling
func recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v", err)
				resp.JSON(w, http.StatusInternalServerError, resp.Error{Message: "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// metricsMiddleware adds HTTP request metrics
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap ResponseWriter to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)

		metrics.HTTPRequests.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		metrics.HTTPDuration.WithLabelValues(r.Method, r.URL.Path, status).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// rateLimitMiddleware applies rate limiting based on client IP
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for health and metrics endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		ip := ratelimit.ExtractIP(r)
		if !s.rateLimiter.Allow(ip) {
			resp.JSON(w, http.StatusTooManyRequests, resp.Error{
				Message: "rate limit exceeded",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetRateLimitStats returns current rate limiting statistics
func (s *Server) GetRateLimitStats() ratelimit.Stats {
	return s.rateLimiter.GetStats()
}

// UpdateRateLimitMetrics updates Prometheus metrics with current rate limiter stats
func (s *Server) UpdateRateLimitMetrics() {
	stats := s.rateLimiter.GetStats()
	metrics.RateLimitGlobalTokens.Set(float64(stats.GlobalTokens))
	metrics.RateLimitGlobalUtilization.Set(stats.GlobalUtilization)
}
