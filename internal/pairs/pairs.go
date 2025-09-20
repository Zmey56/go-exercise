package pairs

import (
	"fmt"
	"strings"
)

// NormalizeUserPair normalizes user input like
// "BTC/USD" -> "BTC/USD" (trim, uppercase and slash validation)
func NormalizeUserPair(p string) (string, error) {
	p = strings.TrimSpace(strings.ToUpper(p))
	if p == "" || !strings.Contains(p, "/") {
		return "", fmt.Errorf("invalid pair: %q (expected BASE/QUOTE)", p)
	}
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid pair: %q", p)
	}
	return parts[0] + "/" + parts[1], nil
}

// ToKrakenSymbol maps user pair to Kraken symbol
// Support BTC→XXBT and main crosses: USD→ZUSD, EUR→ZEUR, CHF stays CHF
func ToKrakenSymbol(userPair string) (string, error) {
	n, err := NormalizeUserPair(userPair)
	if err != nil {
		return "", err
	}
	parts := strings.Split(n, "/")
	base, quote := parts[0], parts[1]

	// Currency mapping for Kraken
	switch base {
	case "BTC":
		// For CHF shorter format XBT is used
		if quote == "CHF" {
			base = "XBT"
		} else {
			base = "XXBT"
		}
	}

	switch quote {
	case "USD":
		quote = "ZUSD"
	case "EUR":
		quote = "ZEUR"
	case "CHF":
		// CHF stays CHF in Kraken
	}

	return base + quote, nil
}

// FromKrakenSymbol back to USER format (for response beauty), if needed
func FromKrakenSymbol(sym string) (string, error) {
	if len(sym) < 6 {
		return "", fmt.Errorf("invalid kraken symbol: %q", sym)
	}

	var base, quote string

	// Reverse mapping for different Kraken formats
	switch {
	case strings.HasPrefix(sym, "XXBT"):
		base = "BTC"
		quote = sym[4:] // Remove "XXBT"
	case strings.HasPrefix(sym, "XBT"):
		base = "BTC"
		quote = sym[3:] // Remove "XBT"
	default:
		return "", fmt.Errorf("unsupported kraken symbol: %q", sym)
	}

	// Reverse currency mapping
	switch quote {
	case "ZUSD":
		quote = "USD"
	case "ZEUR":
		quote = "EUR"
	case "CHF":
		// CHF stays CHF
	default:
		return "", fmt.Errorf("unsupported quote currency in symbol: %q", sym)
	}

	return base + "/" + quote, nil
}
