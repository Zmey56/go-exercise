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
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = NewHTTPClient(3 * time.Second)
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

// GetTickerPrices requests Kraken tickers for multiple symbols (comma-separated)
// symbols: e.g. ["XBTUSD","XBTEUR"]
func (c *Client) GetTickerPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbols provided")
	}

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
		return nil, fmt.Errorf("kraken status %d: %s", resp.StatusCode, string(b))
	}

	var tr TickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}


	if len(tr.Error) > 0 {
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
