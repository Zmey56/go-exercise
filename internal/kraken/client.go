package kraken

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Zmey56/go-exercise/internal/backoff"
	"github.com/Zmey56/go-exercise/internal/metrics"
)

type Client struct {
	baseURL string
	http    *http.Client
	backoff *backoff.Backoff
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = NewHTTPClient(3 * time.Second)
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
		backoff: backoff.New(backoff.DefaultConfig()),
	}
}

// GetTickerPrices requests Kraken tickers for multiple symbols (comma-separated)
// symbols: e.g. ["XBTUSD","XBTEUR"]
func (c *Client) GetTickerPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	start := time.Now()
	var status string
	defer func() {
		metrics.KrakenRequests.WithLabelValues(status).Inc()
		metrics.KrakenDuration.WithLabelValues(status).Observe(time.Since(start).Seconds())
	}()

	if len(symbols) == 0 {
		status = "error"
		return nil, fmt.Errorf("no symbols provided")
	}

	var result map[string]float64
	err := c.backoff.Retry(ctx, "kraken", func() error {
		var err error
		result, err = c.makeRequest(ctx, symbols)
		return err
	}, backoff.IsRetryableHTTPError)

	if err != nil {
		status = "error"
		return nil, err
	}

	status = "success"
	return result, nil
}

// makeRequest performs a single HTTP request to Kraken API
func (c *Client) makeRequest(ctx context.Context, symbols []string) (map[string]float64, error) {
	v := url.Values{}
	v.Set("pair", strings.Join(symbols, ","))
	endpoint := c.baseURL + "/0/public/Ticker?" + v.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		// Return an HTTPError that can be checked by the retry logic
		return nil, backoff.NewHTTPError(resp.StatusCode, string(b))
	}

	var tr TickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}

	if len(tr.Error) > 0 {
		// Kraken API errors are not retryable
		return nil, errors.New(strings.Join(tr.Error, "; "))
	}

	out := make(map[string]float64, len(tr.Result))
	for sym, entry := range tr.Result {
		if len(entry.C) == 0 {
			continue
		}
		// Price comes as string, parse as float64
		var price float64
		if _, err := fmt.Sscan(entry.C[0], &price); err == nil {
			out[sym] = price
		}
	}

	return out, nil
}
