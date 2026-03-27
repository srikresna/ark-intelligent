// Package factors implements a cross-sectional factor ranking engine.
// Factors: momentum, trend quality, carry-adjusted momentum, low-vol efficiency,
// residual reversal, crowding risk.
// All scores are normalized to [-1, +1] before combination.
package factors

import "time"

// AssetProfile bundles all inputs required to score a single asset.
type AssetProfile struct {
	ContractCode string  // CFTC code or synthetic code
	Currency     string  // Display ticker, e.g. "EUR", "XAU"
	Name         string  // Human-readable, e.g. "Euro FX"
	ReportType   string  // "TFF" or "DISAGGREGATED"
	IsCrypto     bool    // true → use Bybit/CoinGecko carry data
	IsInverse    bool    // true → price is quoted inversely vs USD

	// Daily price series (newest first), at least 200 bars preferred.
	DailyCloses []float64 // close prices, index 0 = most recent

	// COT Positioning — latest values
	COTIndex        float64 // 0-100, commercial positioning percentile
	SmartMoneyNet   float64 // levered fund or managed money net
	CrowdingIndex   float64 // from domain.COTAnalysis.CrowdingIndex
	SpecMomentum4W  float64 // 4-week spec momentum

	// Rate differential (carry proxy) — carry of holding long position.
	// For FX: rate difference vs USD (positive = carry favorable for long).
	CarryBps float64 // in basis points, positive = favorable carry

	// Crypto-specific carry
	FundingRate float64 // Bybit perpetual funding rate (annualized, bps)
}

// ---------------------------------------------------------------------------
// Factor Scores
// ---------------------------------------------------------------------------

// FactorScores holds the normalized score [-1,+1] for each factor.
type FactorScores struct {
	Momentum        float64 // cross-sectional price momentum
	TrendQuality    float64 // ADX/Hurst/MA alignment quality
	CarryAdjusted   float64 // momentum adjusted for carry cost
	LowVol          float64 // low-vol efficiency (inverted vol / Sharpe proxy)
	ResidualReversal float64 // mean-reversion signal from OLS residual
	Crowding        float64 // crowding risk (negative = crowded = avoid)
}

// Combined returns the weighted composite score.
// Weights sum to 1.0 and are tuned for systematic macro:
//   momentum=0.30, trendQuality=0.20, carryAdjusted=0.20,
//   lowVol=0.10, residualReversal=0.10, crowding=0.10 (penalty)
func (f FactorScores) Combined(w Weights) float64 {
	return w.Momentum*f.Momentum +
		w.TrendQuality*f.TrendQuality +
		w.CarryAdjusted*f.CarryAdjusted +
		w.LowVol*f.LowVol +
		w.ResidualReversal*f.ResidualReversal +
		w.Crowding*f.Crowding
}

// Weights holds factor combination weights. Must sum to ~1.0.
type Weights struct {
	Momentum        float64
	TrendQuality    float64
	CarryAdjusted   float64
	LowVol          float64
	ResidualReversal float64
	Crowding        float64
}

// DefaultWeights returns the production default weights.
func DefaultWeights() Weights {
	return Weights{
		Momentum:        0.30,
		TrendQuality:    0.20,
		CarryAdjusted:   0.20,
		LowVol:          0.10,
		ResidualReversal: 0.10,
		Crowding:        0.10,
	}
}

// ---------------------------------------------------------------------------
// Ranked Asset
// ---------------------------------------------------------------------------

// RankedAsset is the output from the engine for one asset.
type RankedAsset struct {
	ContractCode string
	Currency     string
	Name         string

	Scores       FactorScores
	CompositeScore float64  // Combined() output, [-1, +1]
	Rank         int        // 1 = best (highest composite), ascending
	Quartile     int        // 1 (top 25%) to 4 (bottom 25%)

	Signal       Signal
	UpdatedAt    time.Time
}

// Signal is the actionable directional bias derived from composite score.
type Signal string

const (
	SignalStrongLong   Signal = "STRONG_LONG"
	SignalLong         Signal = "LONG"
	SignalNeutral      Signal = "NEUTRAL"
	SignalShort        Signal = "SHORT"
	SignalStrongShort  Signal = "STRONG_SHORT"
)

// CompositeToSignal maps a composite score to a signal.
func CompositeToSignal(score float64) Signal {
	switch {
	case score >= 0.50:
		return SignalStrongLong
	case score >= 0.20:
		return SignalLong
	case score <= -0.50:
		return SignalStrongShort
	case score <= -0.20:
		return SignalShort
	default:
		return SignalNeutral
	}
}

// ---------------------------------------------------------------------------
// Engine Output
// ---------------------------------------------------------------------------

// RankingResult is the full output from the Factor Engine for one computation cycle.
type RankingResult struct {
	Assets    []RankedAsset // sorted by Rank (ascending)
	ComputedAt time.Time
	AssetCount int
}

// Top returns the top n assets (highest ranked).
func (r *RankingResult) Top(n int) []RankedAsset {
	if n > len(r.Assets) {
		n = len(r.Assets)
	}
	return r.Assets[:n]
}

// Bottom returns the bottom n assets (lowest ranked, best shorts).
func (r *RankingResult) Bottom(n int) []RankedAsset {
	total := len(r.Assets)
	if n > total {
		n = total
	}
	return r.Assets[total-n:]
}
