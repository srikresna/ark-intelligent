package price

import (
	"fmt"
	"math"
)

// PositionSizeInput holds the parameters for position sizing calculation.
type PositionSizeInput struct {
	AccountBalance float64 // Account size in base currency
	RiskPercent    float64 // Risk per trade (e.g. 1.0 for 1%)
	EntryPrice     float64 // Planned entry price
	StopLoss       float64 // Stop loss price
	DailyATR       float64 // Current daily ATR
	NormalizedATR  float64 // ATR as % of price
}

// PositionSizeResult holds the computed position sizing.
type PositionSizeResult struct {
	// ATR-based stop distance
	ATRStopDistance float64 `json:"atr_stop_distance"` // 1.5x ATR from entry
	ATRStopPrice   float64 `json:"atr_stop_price"`    // Entry - 1.5*ATR (for longs)
	ATRStopPct     float64 `json:"atr_stop_pct"`      // Stop distance as %

	// Position sizing
	RiskAmount     float64 `json:"risk_amount"`      // $ risked per trade
	PositionSize   float64 `json:"position_size"`    // Units/lots to trade
	PositionValue  float64 `json:"position_value"`   // Total position value

	// Volatility context
	VolatilityTier string  `json:"volatility_tier"` // "LOW", "NORMAL", "HIGH", "EXTREME"
	ATRMultiplier  float64 `json:"atr_multiplier"`  // Applied multiplier (1.5-2.5)

	// Risk/reward framework
	RR1Target      float64 `json:"rr1_target"`      // 1:1 R:R target price
	RR2Target      float64 `json:"rr2_target"`      // 1:2 R:R target price
	RR3Target      float64 `json:"rr3_target"`      // 1:3 R:R target price
}

// ComputePositionSize calculates ATR-based position sizing.
// Uses volatility-adaptive stop placement: tighter in low vol, wider in high vol.
func ComputePositionSize(input PositionSizeInput, bullish bool) *PositionSizeResult {
	result := &PositionSizeResult{}

	if input.EntryPrice <= 0 || input.DailyATR <= 0 {
		return result
	}

	// Classify volatility tier by normalized ATR
	result.VolatilityTier, result.ATRMultiplier = classifyVolatility(input.NormalizedATR)

	// ATR-based stop distance
	result.ATRStopDistance = input.DailyATR * result.ATRMultiplier

	if bullish {
		result.ATRStopPrice = input.EntryPrice - result.ATRStopDistance
	} else {
		result.ATRStopPrice = input.EntryPrice + result.ATRStopDistance
	}

	result.ATRStopPct = roundN(result.ATRStopDistance/input.EntryPrice*100, 4)

	// Position sizing based on risk budget
	if input.AccountBalance > 0 && input.RiskPercent > 0 && result.ATRStopDistance > 0 {
		result.RiskAmount = input.AccountBalance * input.RiskPercent / 100
		result.PositionSize = roundN(result.RiskAmount/result.ATRStopDistance, 4)
		result.PositionValue = roundN(result.PositionSize*input.EntryPrice, 2)
	}

	// R:R targets
	if bullish {
		result.RR1Target = input.EntryPrice + result.ATRStopDistance
		result.RR2Target = input.EntryPrice + 2*result.ATRStopDistance
		result.RR3Target = input.EntryPrice + 3*result.ATRStopDistance
	} else {
		result.RR1Target = input.EntryPrice - result.ATRStopDistance
		result.RR2Target = input.EntryPrice - 2*result.ATRStopDistance
		result.RR3Target = input.EntryPrice - 3*result.ATRStopDistance
	}

	return result
}

// classifyVolatility returns volatility tier and ATR multiplier for stop placement.
// Low vol → tighter stops (1.5x ATR), high vol → wider stops (2.5x ATR).
func classifyVolatility(normalizedATR float64) (string, float64) {
	switch {
	case normalizedATR < 0.3:
		return "LOW", 1.5
	case normalizedATR < 0.8:
		return "NORMAL", 2.0
	case normalizedATR < 1.5:
		return "HIGH", 2.5
	default:
		return "EXTREME", 3.0
	}
}

// ComputeEntryTiming provides entry timing suggestions based on daily price context.
type EntryTimingResult struct {
	Recommendation string  `json:"recommendation"` // "ENTER_NOW", "WAIT_PULLBACK", "WAIT_BREAKOUT", "AVOID"
	Reasoning      string  `json:"reasoning"`
	IdealEntry     float64 `json:"ideal_entry"`     // Suggested entry price
	MaxEntry       float64 `json:"max_entry"`       // Don't enter above this (for longs)
	Urgency        int     `json:"urgency"`         // 1-5 (5 = most urgent)
}

// ComputeEntryTiming analyzes daily price context to suggest optimal entry timing.
func ComputeEntryTiming(dc *LevelsContext, bullish bool, dailyATR float64) *EntryTimingResult {
	result := &EntryTimingResult{Urgency: 3}
	current := dc.CurrentPrice

	if current <= 0 || dailyATR <= 0 {
		result.Recommendation = "AVOID"
		result.Reasoning = "insufficient data"
		return result
	}

	nearSupport := dc.NearestSupport != nil && math.Abs(dc.NearestSupport.Distance) < 0.5
	nearResistance := dc.NearestResistance != nil && math.Abs(dc.NearestResistance.Distance) < 0.5

	if bullish {
		switch {
		case nearSupport:
			// Price near support — good entry for longs
			result.Recommendation = "ENTER_NOW"
			result.Reasoning = "price near support — favorable long entry"
			result.IdealEntry = current
			result.MaxEntry = current + dailyATR*0.5
			result.Urgency = 5

		case nearResistance:
			// Price at resistance — wait for breakout confirmation
			result.Recommendation = "WAIT_BREAKOUT"
			result.Reasoning = "price at resistance — wait for breakout above " + fmt.Sprintf("%.5f", dc.NearestResistance.Price)
			result.IdealEntry = dc.NearestResistance.Price + dailyATR*0.2
			result.MaxEntry = dc.NearestResistance.Price + dailyATR
			result.Urgency = 2

		default:
			// Mid-range — wait for pullback to support
			result.Recommendation = "WAIT_PULLBACK"
			result.Reasoning = "price in mid-range — wait for pullback"
			if dc.NearestSupport != nil {
				result.IdealEntry = dc.NearestSupport.Price + dailyATR*0.3
			} else {
				result.IdealEntry = current - dailyATR*0.5
			}
			result.MaxEntry = current
			result.Urgency = 3
		}
	} else {
		// Bearish mirror
		switch {
		case nearResistance:
			result.Recommendation = "ENTER_NOW"
			result.Reasoning = "price near resistance — favorable short entry"
			result.IdealEntry = current
			result.MaxEntry = current - dailyATR*0.5
			result.Urgency = 5

		case nearSupport:
			result.Recommendation = "WAIT_BREAKOUT"
			result.Reasoning = "price at support — wait for breakdown below " + fmt.Sprintf("%.5f", dc.NearestSupport.Price)
			result.IdealEntry = dc.NearestSupport.Price - dailyATR*0.2
			result.MaxEntry = dc.NearestSupport.Price - dailyATR
			result.Urgency = 2

		default:
			result.Recommendation = "WAIT_PULLBACK"
			result.Reasoning = "price in mid-range — wait for rally to resistance"
			if dc.NearestResistance != nil {
				result.IdealEntry = dc.NearestResistance.Price - dailyATR*0.3
			} else {
				result.IdealEntry = current + dailyATR*0.5
			}
			result.MaxEntry = current
			result.Urgency = 3
		}
	}

	return result
}
