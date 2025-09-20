package kraken

// Kraken response for /0/public/Ticker?pair=...
// We only consider the minimum required fields.

type TickerResponse struct {
	Error  []string               `json:"error"`
	Result map[string]TickerEntry `json:"result"`
}

type TickerEntry struct {
	// c — last trade closed [<price>, <lot volume>]
	C []string `json:"c"`
}
