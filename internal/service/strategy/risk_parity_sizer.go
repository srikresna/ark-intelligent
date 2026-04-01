package strategy

import (
	"math"
	"time"
)

// ---------------------------------------------------------------------------
// Risk Parity Position Sizer
// ---------------------------------------------------------------------------
//
// Extends per-pair ATR sizing (price.PositionSizeResult) into portfolio-level
// risk parity. Accounts for total portfolio heat, applies Kelly fraction from
// backtest statistics, and adjusts for volatility regime.

// RiskParityInput bundles positions plus account context.
type RiskParityInput struct {
	Positions      []PositionRisk // Current or planned positions
	AccountBalance float64        // Total account balance in base currency
	MaxHeatPct     float64        // Max portfolio heat as % (e.g. 6.0 = 6%)

	// Backtest-derived (optional — zero values trigger fallback)
	WinRate     float64 // Win rate 0-100 (e.g. 58.0)
	AvgWinLoss  float64 // Average win / average loss ratio (reward-to-risk)
	SharpeRatio float64 // Annualized Sharpe

	// Volatility regime per-pair (optional, key = symbol)
	VolRegimes map[string]string // symbol -> "EXPANDING"/"CONTRACTING"/"NORMAL"
}

// PositionRisk describes a single position for portfolio sizing.
type PositionRisk struct {
	Symbol    string
	Direction Direction // LONG or SHORT
	Entry     float64
	StopLoss  float64
	Size      float64 // Current position size (units)
}

// RiskParityResult is the output of portfolio-level position sizing.
type RiskParityResult struct {
	AdjustedPositions []AdjustedPosition `json:"adjusted_positions"`
	TotalHeatPct      float64            `json:"total_heat_pct"`      // Sum of individual risk as % of account
	KellyFraction     float64            `json:"kelly_fraction"`      // Full Kelly fraction
	HalfKelly         float64            `json:"half_kelly"`          // Half-Kelly (recommended)
	UsedFraction      float64            `json:"used_fraction"`       // Fraction actually applied
	Recommendation    SizingAdvice       `json:"recommendation"`      // SCALE_DOWN / BALANCED / SCALE_UP
	HeatBreakdown     []HeatEntry        `json:"heat_breakdown"`      // Per-position heat
	MaxHeatPct        float64            `json:"max_heat_pct"`
	ComputedAt        time.Time          `json:"computed_at"`
}

// AdjustedPosition holds the sizing recommendation for one position.
type AdjustedPosition struct {
	Symbol          string  `json:"symbol"`
	OriginalSize    float64 `json:"original_size"`
	RecommendedSize float64 `json:"recommended_size"`
	RiskPct         float64 `json:"risk_pct"`     // Individual risk as % of account
	VolAdjustment   float64 `json:"vol_adj"`      // Multiplier from vol regime (0.8-1.1)
	KellyAdjustment float64 `json:"kelly_adj"`    // Multiplier from Kelly
	ScaleFactor     float64 `json:"scale_factor"` // Final combined scale factor
}

// HeatEntry shows one position's contribution to portfolio heat.
type HeatEntry struct {
	Symbol  string  `json:"symbol"`
	RiskAmt float64 `json:"risk_amt"` // Dollar risk
	RiskPct float64 `json:"risk_pct"` // % of account
}

// SizingAdvice is the top-level recommendation.
type SizingAdvice string

const (
	SizingScaleDown SizingAdvice = "SCALE_DOWN"
	SizingBalanced  SizingAdvice = "BALANCED"
	SizingScaleUp   SizingAdvice = "SCALE_UP"
)

// ComputeRiskParity calculates portfolio-level position sizing adjustments.
func ComputeRiskParity(input RiskParityInput) *RiskParityResult {
	result := &RiskParityResult{
		MaxHeatPct: input.MaxHeatPct,
		ComputedAt: time.Now(),
	}

	if input.AccountBalance <= 0 {
		result.Recommendation = SizingBalanced
		return result
	}

	if input.MaxHeatPct <= 0 {
		input.MaxHeatPct = 6.0 // default 6% max portfolio risk
	}
	result.MaxHeatPct = input.MaxHeatPct

	// Step 1: Compute Kelly fraction from backtest stats.
	result.KellyFraction = computeKelly(input.WinRate, input.AvgWinLoss)
	result.HalfKelly = result.KellyFraction / 2
	result.UsedFraction = result.HalfKelly
	if result.UsedFraction <= 0 || result.UsedFraction > 0.25 {
		// Fallback: fixed fractional (2% per trade) if Kelly invalid or too aggressive
		result.UsedFraction = 0.02
	}

	// Step 2: Compute per-position risk.
	var totalRisk float64
	for _, pos := range input.Positions {
		riskPerUnit := math.Abs(pos.Entry - pos.StopLoss)
		riskAmt := riskPerUnit * math.Abs(pos.Size)
		riskPct := 0.0
		if input.AccountBalance > 0 {
			riskPct = riskAmt / input.AccountBalance * 100
		}
		totalRisk += riskPct

		result.HeatBreakdown = append(result.HeatBreakdown, HeatEntry{
			Symbol:  pos.Symbol,
			RiskAmt: roundN(riskAmt, 2),
			RiskPct: roundN(riskPct, 2),
		})
	}
	result.TotalHeatPct = roundN(totalRisk, 2)

	// Step 3: Determine recommendation.
	heatRatio := totalRisk / input.MaxHeatPct
	switch {
	case heatRatio > 1.0:
		result.Recommendation = SizingScaleDown
	case heatRatio < 0.5:
		result.Recommendation = SizingScaleUp
	default:
		result.Recommendation = SizingBalanced
	}

	// Step 4: Compute per-position adjusted sizes.
	scaleFactor := 1.0
	if totalRisk > input.MaxHeatPct && totalRisk > 0 {
		scaleFactor = input.MaxHeatPct / totalRisk
	}

	for _, pos := range input.Positions {
		adj := AdjustedPosition{
			Symbol:       pos.Symbol,
			OriginalSize: pos.Size,
		}

		// Volatility regime adjustment
		adj.VolAdjustment = volRegimeMultiplier(input.VolRegimes[pos.Symbol])

		// Kelly adjustment: scale relative to half-Kelly optimal
		riskPerUnit := math.Abs(pos.Entry - pos.StopLoss)
		if riskPerUnit > 0 && input.AccountBalance > 0 {
			optimalRiskAmt := input.AccountBalance * result.UsedFraction
			optimalSize := optimalRiskAmt / riskPerUnit
			if math.Abs(pos.Size) > 0 {
				adj.KellyAdjustment = roundN(optimalSize/math.Abs(pos.Size), 2)
			} else {
				adj.KellyAdjustment = 1.0
			}
		} else {
			adj.KellyAdjustment = 1.0
		}

		// Combined scale: portfolio heat constraint × vol regime × Kelly guidance
		adj.ScaleFactor = roundN(scaleFactor*adj.VolAdjustment, 2)
		// If Kelly says smaller than heat constraint, use the smaller
		if adj.KellyAdjustment < adj.ScaleFactor {
			adj.ScaleFactor = adj.KellyAdjustment
		}
		// Floor at 10% of original to avoid zeroing out
		if adj.ScaleFactor < 0.1 {
			adj.ScaleFactor = 0.1
		}

		adj.RecommendedSize = roundN(math.Abs(pos.Size)*adj.ScaleFactor, 4)
		adj.RiskPct = roundN(riskPerUnit*adj.RecommendedSize/input.AccountBalance*100, 2)

		result.AdjustedPositions = append(result.AdjustedPositions, adj)
	}

	return result
}

// computeKelly returns the Kelly fraction: f* = p - (1-p)/b
// where p = win probability (0-1), b = avg win / avg loss ratio.
// Capped at 25% to prevent over-leverage.
func computeKelly(winRatePct float64, avgWinLoss float64) float64 {
	p := winRatePct / 100
	if p <= 0 || p >= 1 || avgWinLoss <= 0 {
		return 0
	}
	kelly := p - (1-p)/avgWinLoss
	if kelly < 0 {
		return 0
	}
	if kelly > 0.25 {
		return 0.25
	}
	return roundN(kelly, 4)
}

// volRegimeMultiplier returns a sizing multiplier based on volatility regime.
func volRegimeMultiplier(regime string) float64 {
	switch regime {
	case "EXPANDING":
		return 0.80 // Reduce size 20% in high vol
	case "CONTRACTING":
		return 1.10 // Allow 10% more in low vol
	default:
		return 1.0
	}
}

// roundN rounds val to n decimal places. Duplicated locally to avoid
// importing the price package which would create a circular dep.
func roundN(val float64, n int) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	pow := math.Pow(10, float64(n))
	return math.Round(val*pow) / pow
}
