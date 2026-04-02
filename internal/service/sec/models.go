// Package sec integrates the SEC EDGAR API to track institutional 13F holdings.
// 13F filings are quarterly mandatory disclosures by institutional investment
// managers with >$100M AUM. Tracking what Berkshire, Bridgewater, Renaissance,
// and Citadel are buying/selling provides a powerful signal for equity positioning.
//
// API: https://data.sec.gov/ (free, no auth — User-Agent header only)
// Rate limit: 10 requests/second.
// Cache TTL: 7 days (quarterly data, infrequent updates).
package sec

import "time"

// Institution represents a tracked institutional filer.
type Institution struct {
	Name string // human-readable name
	CIK  string // SEC Central Index Key (zero-padded to 10 digits)
}

// Holding represents a single position from a 13F filing.
type Holding struct {
	Issuer     string  // company name (nameOfIssuer)
	CUSIPClass string  // CUSIP number (cusip)
	TitleClass string  // class of security (titleOfClass)
	Value      float64 // market value in thousands USD
	Shares     float64 // number of shares (or principal amount)
	PutCall    string  // "PUT", "CALL", or "" for shares
}

// Filing represents a single 13F-HR filing.
type Filing struct {
	AccessionNumber string
	FilingDate      time.Time
	ReportDate      time.Time // period of report (quarter end)
	Holdings        []Holding
	TotalValue      float64 // sum of all holding values (thousands USD)
}

// PortfolioChange represents a quarter-over-quarter change for a holding.
type PortfolioChange struct {
	Issuer      string
	TitleClass  string
	ChangeType  string  // "NEW", "EXIT", "INCREASE", "DECREASE", "UNCHANGED"
	CurrValue   float64 // current quarter value (thousands USD)
	PrevValue   float64 // previous quarter value (thousands USD)
	CurrShares  float64
	PrevShares  float64
	ValueChange float64 // absolute change in thousands
	PctChange   float64 // percentage change
}

// InstitutionReport holds the analysis for one institution.
type InstitutionReport struct {
	Institution    Institution
	LatestFiling   *Filing
	PreviousFiling *Filing
	TopHoldings    []Holding         // top holdings by value (current quarter)
	Changes        []PortfolioChange // significant QoQ changes
	NewPositions   []PortfolioChange // new entries
	Exits          []PortfolioChange // complete exits
}

// EdgarData is the top-level result returned by the SEC service.
type EdgarData struct {
	Reports   []InstitutionReport
	FetchedAt time.Time
}
