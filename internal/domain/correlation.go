package domain

// ---------------------------------------------------------------------------
// Cross-Pair Correlation Matrix
// ---------------------------------------------------------------------------

// CorrelationPair holds the rolling correlation between two instruments.
type CorrelationPair struct {
	CurrencyA   string  `json:"currency_a"`
	CurrencyB   string  `json:"currency_b"`
	Correlation float64 `json:"correlation"` // Pearson r, -1.0 to +1.0
	Period      int     `json:"period"`      // Rolling window in days
}

// CorrelationMatrix holds the full NxN correlation matrix.
type CorrelationMatrix struct {
	Currencies []string                      `json:"currencies"`
	Matrix     map[string]map[string]float64 `json:"matrix"` // [A][B] = r
	Period     int                           `json:"period"` // Rolling window
	Breakdowns []CorrelationBreakdown        `json:"breakdowns,omitempty"`
}

// CorrelationBreakdown flags when a historically correlated pair diverges.
type CorrelationBreakdown struct {
	CurrencyA      string  `json:"currency_a"`
	CurrencyB      string  `json:"currency_b"`
	CurrentCorr    float64 `json:"current_corr"`    // Recent 20-day
	HistoricalCorr float64 `json:"historical_corr"` // 60-day baseline
	Delta          float64 `json:"delta"`           // Current - Historical
	Severity       string  `json:"severity"`        // "HIGH", "MEDIUM", "LOW"
}

// CorrelationCluster groups highly correlated currencies.
type CorrelationCluster struct {
	Name       string   `json:"name"`       // e.g. "Risk-On FX"
	Currencies []string `json:"currencies"` // e.g. ["AUD", "NZD", "CAD"]
	AvgCorr    float64  `json:"avg_corr"`   // Average intra-cluster correlation
}

// DefaultCorrelationCurrencies returns all monitored assets for correlation analysis.
// Covers FX majors, metals, energy, bonds, equity indices, and crypto.
func DefaultCorrelationCurrencies() []string {
	return []string{
		// FX Majors
		"EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF", "USD",
		// Metals
		"XAU", "XAG", "COPPER",
		// Energy
		"OIL", "ULSD", "RBOB",
		// Bonds (full curve)
		"BOND", "BOND30", "BOND5", "BOND2",
		// Equity Indices
		"SPX500", "NDX", "DJI", "RUT",
		// Crypto
		"BTC", "ETH",
	}
}
