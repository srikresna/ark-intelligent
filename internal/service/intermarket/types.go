package intermarket

import (
	"context"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// DailyPriceStore is a subset of the price repository used by the engine.
type DailyPriceStore interface {
	GetDailyHistory(ctx context.Context, contractCode string, days int) ([]domain.DailyPrice, error)
}

// ---------------------------------------------------------------------------
// Rules
// ---------------------------------------------------------------------------

// IntermarketRule defines a known relationship between two assets.
type IntermarketRule struct {
	Base        string // currency code that drives the Base asset (e.g. "AUD")
	Correlated  string // currency code of the correlated asset (e.g. "XAU")
	Direction   int    // +1 = positive correlation expected, -1 = negative
	Window      int    // rolling correlation window in trading days
	Label       string // human-readable relationship name
	Reliability string // "HIGH" or "MEDIUM"
}

// StandardRules defines well-known FX intermarket relationships.
// Symbols use currency codes that map to domain.FindPriceMappingByCurrency.
var StandardRules = []IntermarketRule{
	// AUD = commodity + risk currency
	{Base: "AUD", Correlated: "XAU", Direction: +1, Window: 20, Label: "AUD-Gold", Reliability: "HIGH"},
	{Base: "AUD", Correlated: "SPX500", Direction: +1, Window: 20, Label: "AUD-Equities (risk-on)", Reliability: "HIGH"},

	// CAD = oil currency (stored as USDCAD with Inverse=true → CADUSD positive with oil)
	{Base: "CAD", Correlated: "OIL", Direction: -1, Window: 20, Label: "CAD-Oil (via USDCAD)", Reliability: "HIGH"},

	// JPY = safe haven
	{Base: "JPY", Correlated: "BOND", Direction: +1, Window: 20, Label: "JPY-Yields (carry)", Reliability: "HIGH"},
	{Base: "JPY", Correlated: "SPX500", Direction: +1, Window: 20, Label: "JPY-Equities (risk-off)", Reliability: "HIGH"},

	// CHF = safe haven
	{Base: "CHF", Correlated: "XAU", Direction: -1, Window: 20, Label: "CHF-Gold (safe haven)", Reliability: "MEDIUM"},

	// DXY/USD relationships
	{Base: "USD", Correlated: "XAU", Direction: +1, Window: 20, Label: "DXY-Gold (inverse)", Reliability: "HIGH"},
	{Base: "USD", Correlated: "EUR", Direction: +1, Window: 20, Label: "DXY-EUR (definitional)", Reliability: "HIGH"},

	// Cross-asset risk regime
	{Base: "XAU", Correlated: "SPX500", Direction: -1, Window: 20, Label: "Gold-Equities (crisis hedge)", Reliability: "MEDIUM"},
}

// ---------------------------------------------------------------------------
// Signals
// ---------------------------------------------------------------------------

// CorrelationStatus classifies the actual vs expected correlation relationship.
type CorrelationStatus string

const (
	StatusAligned   CorrelationStatus = "ALIGNED"   // actual matches expected direction
	StatusDiverging CorrelationStatus = "DIVERGING"  // slightly off
	StatusBroken    CorrelationStatus = "BROKEN"     // strongly opposite to expected
)

// IntermarketSignal holds the computed result for a single rule.
type IntermarketSignal struct {
	Rule        IntermarketRule
	ActualCorr  float64           // rolling 20D Pearson correlation
	Status      CorrelationStatus
	Implication string            // trading implication text
	Strength    float64           // 0–1 confidence in the signal
	Insufficient bool             // true if not enough data points
}

// IntermarketResult is the full output of Engine.Analyze.
type IntermarketResult struct {
	Signals     []IntermarketSignal // all rules evaluated
	Divergences []IntermarketSignal // only DIVERGING / BROKEN
	RiskRegime  string              // "RISK_ON", "RISK_OFF", "MIXED"
	AsOf        time.Time
}
