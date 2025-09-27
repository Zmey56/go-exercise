package kraken

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestClient_NetworkTimeouts tests network timeout scenarios
func TestClient_NetworkTimeouts(t *testing.T) {
	// Create server that sleeps longer than client timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TickerResponse{})
	}))
	defer server.Close()

	// Client with very short timeout
	httpClient := &http.Client{Timeout: 50 * time.Millisecond}
	client := NewClient(server.URL, httpClient)

	_, err := client.GetTickerPrices(context.Background(), []string{"XXBTZUSD"})
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

// TestClient_BadJSONResponses tests handling of invalid JSON responses
func TestClient_BadJSONResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	client := NewClient(server.URL, httpClient)

	_, err := client.GetTickerPrices(context.Background(), []string{"XXBTZUSD"})
	if err == nil {
		t.Fatal("Expected JSON decode error, got nil")
	}
}

// TestClient_HTTPErrorStatuses tests HTTP error status handling
func TestClient_HTTPErrorStatuses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"429 Too Many Requests", http.StatusTooManyRequests},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"error":["Test error"]}`))
			}))
			defer server.Close()

			httpClient := &http.Client{Timeout: 5 * time.Second}
			client := NewClient(server.URL, httpClient)

			_, err := client.GetTickerPrices(context.Background(), []string{"XXBTZUSD"})
			if err == nil {
				t.Fatalf("Expected error for HTTP %d, got nil", tt.statusCode)
			}
		})
	}
}

// TestClient_EmptySymbols tests empty symbols handling
func TestClient_EmptySymbols(t *testing.T) {
	httpClient := &http.Client{Timeout: 5 * time.Second}
	client := NewClient("http://example.com", httpClient)

	_, err := client.GetTickerPrices(context.Background(), []string{})
	if err == nil {
		t.Error("Expected error for empty symbols")
	}

	_, err = client.GetTickerPrices(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil symbols")
	}
}

// TestClient_PartialSymbolResults tests scenarios where only some symbols are returned
func TestClient_PartialSymbolResults(t *testing.T) {
	mockResponse := TickerResponse{
		Error: []string{},
		Result: map[string]TickerEntry{
			"XXBTZUSD": {C: []string{"50000.0", "0.1"}},
			// XXBTZEUR missing from response
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	client := NewClient(server.URL, httpClient)

	// Request both symbols but only one is in response
	prices, err := client.GetTickerPrices(context.Background(), []string{"XXBTZUSD", "XXBTZEUR"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have one result
	if len(prices) != 1 {
		t.Errorf("Expected 1 price, got %d", len(prices))
	}

	if prices["XXBTZUSD"] != 50000.0 {
		t.Errorf("Expected XXBTZUSD price 50000.0, got %f", prices["XXBTZUSD"])
	}
}

// TestClient_ContextCancellation tests context cancellation
func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TickerResponse{})
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	client := NewClient(server.URL, httpClient)

	// Cancel context before request completes
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.GetTickerPrices(ctx, []string{"XXBTZUSD"})
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}
}

// TestClient_KrakenAPIErrors tests Kraken API error responses
func TestClient_KrakenAPIErrors(t *testing.T) {
	mockResponse := TickerResponse{
		Error: []string{"EQuery:Invalid asset pair"},
		Result: map[string]TickerEntry{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	client := NewClient(server.URL, httpClient)

	_, err := client.GetTickerPrices(context.Background(), []string{"INVALID"})
	if err == nil {
		t.Fatal("Expected Kraken API error, got nil")
	}

	// Should contain the Kraken error message
	if !strings.Contains(err.Error(), "Invalid asset pair") {
		t.Errorf("Expected Kraken error message, got: %v", err)
	}
}