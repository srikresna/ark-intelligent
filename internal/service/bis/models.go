// Package bis — model types for the BIS Statistics API (SDMX REST).
// Datasets: WS_CBPOL (policy rates), WS_CREDIT_GAP, WS_GLI (global liquidity).
package bis

import "time"

// PolicyRate holds the latest central bank policy rate for one country.
type PolicyRate struct {
	Country   string  // BIS country code: "US", "XM", "GB", "JP", "CH", "AU", "CA", "NZ"
	Label     string  // Central bank label: "Fed", "ECB", "BOE", etc.
	Rate      float64 // Latest rate in percent (e.g. 5.25)
	Period    string  // Observation period: "2025-Q4" or "2025-12"
	Available bool    // false if fetch or parse failed
}

// CreditGap holds the BIS credit-to-GDP gap for one country.
// Positive gap = credit expansion above trend (financial stability warning).
// Negative gap = credit below trend (deleveraging / contraction).
type CreditGap struct {
	Country   string  // BIS country code
	Label     string  // Display name: "United States", "Eurozone", etc.
	Gap       float64 // Credit-to-GDP gap in percentage points
	Signal    string  // "WARNING" (>2pp), "ELEVATED" (0..2pp), "NEUTRAL" (<0pp)
	Period    string  // Observation period
	Available bool
}

// GlobalLiquidity holds one global liquidity indicator from WS_GLI.
type GlobalLiquidity struct {
	Label     string  // "USD to Non-US Residents", "EUR Credit", etc.
	ValueBn   float64 // USD billions (or original currency billions)
	YoYPct    float64 // Year-over-year % change (0 if unavailable)
	Period    string
	Available bool
}

// BISSummaryData aggregates all BIS datasets into a single result.
type BISSummaryData struct {
	PolicyRates  []PolicyRate
	CreditGaps   []CreditGap
	GLIndicators []GlobalLiquidity
	FetchedAt    time.Time
}
