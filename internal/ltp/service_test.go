package ltp

import (
	"context"
	"errors"
	"testing"
)

// mockKrakenAPI mock for Kraken API
type mockKrakenAPI struct {
	prices map[string]float64
	err    error
}

func (m *mockKrakenAPI) GetTickerPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.prices, nil
}

// mockCache mock for cache
type mockCache struct {
	data map[string]float64
}

func (m *mockCache) Get(key string) (float64, bool) {
	val, ok := m.data[key]
	return val, ok
}

func (m *mockCache) Set(key string, v float64) {
	if m.data == nil {
		m.data = make(map[string]float64)
	}
	m.data[key] = v
}

func TestService_GetLTP(t *testing.T) {
	tests := []struct {
		name           string
		pairs          []string
		mockAPI        *mockKrakenAPI
		mockCache      *mockCache
		expectedQuotes []Quote
		expectedError  string
	}{
		{
			name:  "Single pair from cache",
			pairs: []string{"BTC/USD"},
			mockAPI: &mockKrakenAPI{
				prices: map[string]float64{"XXBTZUSD": 50000.0},
			},
			mockCache: &mockCache{
				data: map[string]float64{"XXBTZUSD": 50000.0},
			},
			expectedQuotes: []Quote{
				{Pair: "BTC/USD", Amount: 50000.0},
			},
		},
		{
			name:  "Multiple pairs from API",
			pairs: []string{"BTC/USD", "BTC/EUR"},
			mockAPI: &mockKrakenAPI{
				prices: map[string]float64{
					"XXBTZUSD": 50000.0,
					"XXBTZEUR": 45000.0,
				},
			},
			mockCache: &mockCache{},
			expectedQuotes: []Quote{
				{Pair: "BTC/EUR", Amount: 45000.0},
				{Pair: "BTC/USD", Amount: 50000.0},
			},
		},
		{
			name:  "Mixed cache and API",
			pairs: []string{"BTC/USD", "BTC/EUR"},
			mockAPI: &mockKrakenAPI{
				prices: map[string]float64{"XXBTZEUR": 45000.0},
			},
			mockCache: &mockCache{
				data: map[string]float64{"XXBTZUSD": 50000.0},
			},
			expectedQuotes: []Quote{
				{Pair: "BTC/EUR", Amount: 45000.0},
				{Pair: "BTC/USD", Amount: 50000.0},
			},
		},
		{
			name:          "Empty pairs",
			pairs:         []string{},
			mockAPI:       &mockKrakenAPI{},
			mockCache:     &mockCache{},
			expectedError: "no pairs provided",
		},
		{
			name:  "Invalid pair format",
			pairs: []string{"INVALID"},
			mockAPI: &mockKrakenAPI{
				prices: map[string]float64{},
			},
			mockCache:     &mockCache{},
			expectedError: "invalid pair: \"INVALID\" (expected BASE/QUOTE)",
		},
		{
			name:  "API error",
			pairs: []string{"BTC/USD"},
			mockAPI: &mockKrakenAPI{
				err: errors.New("kraken API error"),
			},
			mockCache:     &mockCache{},
			expectedError: "kraken: kraken API error",
		},
		{
			name:  "Pair not found in API response",
			pairs: []string{"BTC/USD"},
			mockAPI: &mockKrakenAPI{
				prices: map[string]float64{}, // Empty response
			},
			mockCache:     &mockCache{},
			expectedError: "pair not found or unsupported: BTC/USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.mockAPI, tt.mockCache)

			quotes, err := service.GetLTP(context.Background(), tt.pairs)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("expected error %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(quotes) != len(tt.expectedQuotes) {
				t.Errorf("expected %d quotes, got %d", len(tt.expectedQuotes), len(quotes))
				return
			}

			// Check that all expected quotes are present
			quoteMap := make(map[string]float64)
			for _, q := range quotes {
				quoteMap[q.Pair] = q.Amount
			}

			for _, expected := range tt.expectedQuotes {
				if amount, ok := quoteMap[expected.Pair]; !ok {
					t.Errorf("missing quote for pair %s", expected.Pair)
				} else if amount != expected.Amount {
					t.Errorf("expected amount %f for pair %s, got %f", expected.Amount, expected.Pair, amount)
				}
			}
		})
	}
}

func TestService_GetLTP_ContextCancellation(t *testing.T) {
	// Context cancellation test
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context immediately

	service := NewService(&mockKrakenAPI{}, &mockCache{})

	_, err := service.GetLTP(ctx, []string{"BTC/USD"})
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

func TestService_GetLTP_Deduplication(t *testing.T) {
	// Pair deduplication test
	service := NewService(
		&mockKrakenAPI{
			prices: map[string]float64{"XXBTZUSD": 50000.0},
		},
		&mockCache{},
	)

	// Pass duplicate pairs
	quotes, err := service.GetLTP(context.Background(), []string{"BTC/USD", "btc/usd", "BTC/USD"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// Should be only one quote
	if len(quotes) != 1 {
		t.Errorf("expected 1 quote after deduplication, got %d", len(quotes))
	}

	if quotes[0].Pair != "BTC/USD" || quotes[0].Amount != 50000.0 {
		t.Errorf("expected BTC/USD:50000, got %s:%f", quotes[0].Pair, quotes[0].Amount)
	}
}

func TestService_GetLTP_CacheExpiration(t *testing.T) {
	// Cache expiration test
	cache := &mockCache{data: make(map[string]float64)}
	service := NewService(
		&mockKrakenAPI{
			prices: map[string]float64{"XXBTZUSD": 50000.0},
		},
		cache,
	)

	// First request - data from API
	quotes1, err := service.GetLTP(context.Background(), []string{"BTC/USD"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// Second request - data from cache
	quotes2, err := service.GetLTP(context.Background(), []string{"BTC/USD"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if len(quotes1) != 1 || len(quotes2) != 1 {
		t.Error("expected 1 quote in both requests")
		return
	}

	if quotes1[0].Amount != quotes2[0].Amount {
		t.Error("expected same amount from cache")
	}
}
