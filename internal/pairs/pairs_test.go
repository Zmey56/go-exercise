package pairs

import (
	"testing"
)

func TestNormalizeUserPair(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "Valid BTC/USD",
			input:       "BTC/USD",
			expected:    "BTC/USD",
			expectError: false,
		},
		{
			name:        "Valid with spaces",
			input:       " btc/usd ",
			expected:    "BTC/USD",
			expectError: false,
		},
		{
			name:        "Valid lowercase",
			input:       "btc/eur",
			expected:    "BTC/EUR",
			expectError: false,
		},
		{
			name:        "Valid CHF",
			input:       "BTC/CHF",
			expected:    "BTC/CHF",
			expectError: false,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "No slash",
			input:       "BTCUSD",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Multiple slashes",
			input:       "BTC/USD/EUR",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty base",
			input:       "/USD",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty quote",
			input:       "BTC/",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeUserPair(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestToKrakenSymbol(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "BTC/USD to XXBTZUSD",
			input:       "BTC/USD",
			expected:    "XXBTZUSD",
			expectError: false,
		},
		{
			name:        "BTC/EUR to XXBTZEUR",
			input:       "BTC/EUR",
			expected:    "XXBTZEUR",
			expectError: false,
		},
		{
			name:        "BTC/CHF to XBTCHF",
			input:       "BTC/CHF",
			expected:    "XBTCHF",
			expectError: false,
		},
		{
			name:        "Invalid pair",
			input:       "INVALID",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToKrakenSymbol(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFromKrakenSymbol(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "XXBTZUSD to BTC/USD",
			input:       "XXBTZUSD",
			expected:    "BTC/USD",
			expectError: false,
		},
		{
			name:        "XXBTZEUR to BTC/EUR",
			input:       "XXBTZEUR",
			expected:    "BTC/EUR",
			expectError: false,
		},
		{
			name:        "XBTCHF to BTC/CHF",
			input:       "XBTCHF",
			expected:    "BTC/CHF",
			expectError: false,
		},
		{
			name:        "Invalid symbol",
			input:       "INVALID",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FromKrakenSymbol(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	// Full conversion cycle test: user pair -> Kraken symbol -> user pair
	userPairs := []string{"BTC/USD", "BTC/EUR", "BTC/CHF"}

	for _, userPair := range userPairs {
		t.Run(userPair, func(t *testing.T) {
			// User pair -> Kraken symbol
			krakenSymbol, err := ToKrakenSymbol(userPair)
			if err != nil {
				t.Errorf("unexpected error converting to Kraken symbol: %v", err)
				return
			}

			// Kraken symbol -> user pair
			convertedBack, err := FromKrakenSymbol(krakenSymbol)
			if err != nil {
				t.Errorf("unexpected error converting from Kraken symbol: %v", err)
				return
			}

			if convertedBack != userPair {
				t.Errorf("round trip conversion failed: %s -> %s -> %s", userPair, krakenSymbol, convertedBack)
			}
		})
	}
}
