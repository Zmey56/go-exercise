package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Zmey56/go-exercise/internal/cache"
	"github.com/Zmey56/go-exercise/internal/health"
	"github.com/Zmey56/go-exercise/internal/httpserver"
	"github.com/Zmey56/go-exercise/internal/kraken"
	"github.com/Zmey56/go-exercise/internal/ltp"
)

func main() {
	// Config from environment variables (without external libs)
	addr := getenv("HTTP_ADDR", ":8080")
	krakenURL := getenv("KRAKEN_URL", "https://api.kraken.com")
	timeout := getDuration("HTTP_TIMEOUT", 3*time.Second)

	// Create cache using factory
	cacheConfig := cache.LoadConfigFromEnv()
	newCache, err := cache.CreateCacheFromEnv()
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer func() {
		if err := newCache.Close(); err != nil {
			log.Printf("Failed to close cache: %v", err)
		}
	}()

	// Create legacy cache wrapper for backward compatibility
	c := cache.NewLegacyCache(newCache)

	// Infrastructure
	httpClient := kraken.NewHTTPClient(timeout)
	kclient := kraken.NewClient(krakenURL, httpClient)
	svc := ltp.NewService(kclient, c)

	srv := httpserver.New(addr, svc)

	// Configure health checks
	cacheCheck := health.NewLegacyCacheCheck(c)
	krakenCheck := health.NewKrakenCheck(kclient)
	srv.AddHealthCheck(cacheCheck)
	srv.AddHealthCheck(krakenCheck)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start background cache refresh
	refreshFunc := func(ctx context.Context, key string) (interface{}, time.Duration, error) {
		// Refresh by fetching from Kraken API
		pairs := []string{key}
		quotes, err := svc.GetLTP(ctx, pairs)
		if err != nil {
			return nil, 0, err
		}
		if len(quotes) == 0 {
			return nil, 0, fmt.Errorf("no quotes returned for key: %s", key)
		}
		return quotes[0].Amount, cacheConfig.DefaultTTL, nil
	}

	if err := newCache.StartBackgroundRefresh(ctx, refreshFunc); err != nil {
		log.Printf("Failed to start background refresh: %v", err)
	}
	defer func() {
		if err := newCache.StopBackgroundRefresh(); err != nil {
			log.Printf("Failed to stop background refresh: %v", err)
		}
	}()

	// Start metrics updater
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				srv.UpdateRateLimitMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Listening on %s", addr)
		errCh <- srv.Start()
	}()

	select {
	case <-ctx.Done():
		log.Println("shutting down...")
		_ = srv.Shutdown(context.Background())
	case err := <-errCh:
		if err != nil {
			log.Printf("server error: %v\n", err)
		}
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
