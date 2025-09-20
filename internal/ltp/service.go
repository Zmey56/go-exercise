package ltp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Zmey56/go-exercise/internal/pairs"
)

// Quote represents a currency pair quote
type Quote struct {
	Pair   string  `json:"pair"`
	Amount float64 `json:"amount"`
}

// HandlerError represents a handler error with HTTP code
type HandlerError struct {
	Code int
	Msg  string
}

func (e *HandlerError) Error() string {
	return e.Msg
}

// KrakenAPI interface for working with Kraken API
type KrakenAPI interface {
	GetTickerPrices(ctx context.Context, symbols []string) (map[string]float64, error)
}

// Cache interface for caching
type Cache interface {
	Get(key string) (float64, bool)
	Set(key string, v float64)
}

// Service represents the service for getting LTP
type Service struct {
	api   KrakenAPI
	cache Cache
}

// NewService creates a new instance of LTP service
func NewService(api KrakenAPI, cache Cache) *Service {
	return &Service{
		api:   api,
		cache: cache,
	}
}

// GetLTP maps to Kraken symbols, gets from cache what's available, fetches missing data from Kraken,
// stores in cache and returns the list of quotes.
func (s *Service) GetLTP(ctx context.Context, userPairs []string) ([]Quote, error) {
	if len(userPairs) == 0 {
		return nil, &HandlerError{Code: 400, Msg: "no pairs provided"}
	}

	// Normalization and deduplication
	normalized := make([]string, 0, len(userPairs))
	seen := map[string]struct{}{}
	for _, p := range userPairs {
		n, err := pairs.NormalizeUserPair(p)
		if err != nil {
			return nil, &HandlerError{Code: 400, Msg: err.Error()}
		}
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			normalized = append(normalized, n)
		}
	}

	// First try cache
	type item struct{ userPair, krakenSym string }
	var needed []item
	results := make(map[string]float64, len(normalized)) // userPair -> price

	for _, up := range normalized {
		sym, err := pairs.ToKrakenSymbol(up)
		if err != nil {
			return nil, &HandlerError{Code: 400, Msg: err.Error()}
		}
		if v, ok := s.cache.Get(sym); ok {
			results[up] = v
			continue
		}
		needed = append(needed, item{userPair: up, krakenSym: sym})
	}

	if len(needed) > 0 {
		// Collect list of symbols for request
		syms := make([]string, 0, len(needed))
		for _, it := range needed {
			syms = append(syms, it.krakenSym)
		}

		// Request to Kraken
		prices, err := s.api.GetTickerPrices(ctx, syms)
		if err != nil {
			return nil, fmt.Errorf("kraken: %w", err)
		}

		// Process results
		for _, it := range needed {
			p, ok := prices[it.krakenSym]
			if !ok {
				// If Kraken didn't return symbol — this is 400 (unknown pair)
				// or temporary issue. Consider as 400 to clearly show to user.
				return nil, &HandlerError{Code: 400, Msg: fmt.Sprintf("pair not found or unsupported: %s", it.userPair)}
			}
			s.cache.Set(it.krakenSym, p)
			results[it.userPair] = p
		}
	}

	// Form alphabetically sorted list for response stability
	out := make([]Quote, 0, len(results))
	keys := make([]string, 0, len(results))
	for k := range results {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return strings.Compare(keys[i], keys[j]) < 0 })

	for _, k := range keys {
		out = append(out, Quote{Pair: k, Amount: results[k]})
	}

	return out, nil
}
