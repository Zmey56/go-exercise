package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Zmey56/go-exercise/internal/ltp"
)

// mockLTPProvider mock for testing
type mockLTPProvider struct {
	quotes []ltp.Quote
	err    error
}

func (m *mockLTPProvider) GetLTP(ctx context.Context, pairs []string) ([]ltp.Quote, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.quotes, nil
}

func TestServer_handleLTP(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		url            string
		mockProvider   *mockLTPProvider
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "GET request with single pair",
			method: "GET",
			url:    "/api/v1/ltp?pair=BTC/USD",
			mockProvider: &mockLTPProvider{
				quotes: []ltp.Quote{
					{Pair: "BTC/USD", Amount: 50000.0},
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"ltp":[{"pair":"BTC/USD","amount":50000}]}`,
		},
		{
			name:   "GET request with multiple pairs",
			method: "GET",
			url:    "/api/v1/ltp?pairs=BTC/USD,BTC/EUR",
			mockProvider: &mockLTPProvider{
				quotes: []ltp.Quote{
					{Pair: "BTC/USD", Amount: 50000.0},
					{Pair: "BTC/EUR", Amount: 45000.0},
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"ltp":[{"pair":"BTC/USD","amount":50000},{"pair":"BTC/EUR","amount":45000}]}`,
		},
		{
			name:           "GET request without parameters - should return all pairs",
			method:         "GET",
			url:            "/api/v1/ltp",
			mockProvider:   &mockLTPProvider{quotes: []ltp.Quote{}},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST request - should return method not allowed",
			method:         "POST",
			url:            "/api/v1/ltp",
			mockProvider:   &mockLTPProvider{},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   `{"message":"method not allowed"}`,
		},
		{
			name:   "Service error",
			method: "GET",
			url:    "/api/v1/ltp?pair=BTC/USD",
			mockProvider: &mockLTPProvider{
				err: &ltp.HandlerError{Code: 400, Msg: "invalid pair"},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"message":"invalid pair"}`,
		},
		{
			name:   "Internal service error",
			method: "GET",
			url:    "/api/v1/ltp?pair=BTC/USD",
			mockProvider: &mockLTPProvider{
				err: fmt.Errorf("internal error"),
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"message":"internal error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := New(":8080", tt.mockProvider)

			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			server.handleLTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" {
				var expected, actual interface{}
				json.Unmarshal([]byte(tt.expectedBody), &expected)
				json.Unmarshal(w.Body.Bytes(), &actual)

				expectedJSON, _ := json.Marshal(expected)
				actualJSON, _ := json.Marshal(actual)

				if string(expectedJSON) != string(actualJSON) {
					t.Errorf("expected body %s, got %s", string(expectedJSON), string(actualJSON))
				}
			}
		})
	}
}

func TestServer_CORS(t *testing.T) {
	server := New(":8080", &mockLTPProvider{quotes: []ltp.Quote{}})

	req := httptest.NewRequest("OPTIONS", "/api/v1/ltp", nil)
	w := httptest.NewRecorder()

	server.http.Handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS headers not set")
	}
}

func TestServer_Integration(t *testing.T) {
	// Integration test with httptest.Server
	provider := &mockLTPProvider{
		quotes: []ltp.Quote{
			{Pair: "BTC/USD", Amount: 50000.0},
			{Pair: "BTC/EUR", Amount: 45000.0},
			{Pair: "BTC/CHF", Amount: 48000.0},
		},
	}

	server := New(":8080", provider)
	testServer := httptest.NewServer(server.http.Handler)
	defer testServer.Close()

	// Test various endpoints
	testCases := []struct {
		url            string
		expectedStatus int
	}{
		{"/api/v1/ltp", http.StatusOK},
		{"/api/v1/ltp?pair=BTC/USD", http.StatusOK},
		{"/api/v1/ltp?pairs=BTC/USD,BTC/EUR", http.StatusOK},
	}

	for _, tc := range testCases {
		resp, err := http.Get(testServer.URL + tc.url)
		if err != nil {
			t.Errorf("request failed for %s: %v", tc.url, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != tc.expectedStatus {
			t.Errorf("expected status %d for %s, got %d", tc.expectedStatus, tc.url, resp.StatusCode)
		}
	}
}
