package backtest

import (
	"context"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var dtfLog = logger.Component("daily-trend-filter")

// DailyTrendFilter adjusts signal confidence based on alignment between
// the COT signal direction and the short-term daily price trend.
//
// Rationale: COT data is weekly and captures institutional positioning shifts.
// Daily price data reveals if price action already confirms or contradicts
// the positioning signal. Signals aligned with the daily trend have higher
// probability; opposed signals face short-term headwind.
type DailyTrendFilter struct {
	dailyCtxProvider DailyContextProvider
}

// DailyContextProvider defines the interface for building daily price context.
type DailyContextProvider interface {
	Build(ctx context.Context, contractCode, currency string) (*domain.DailyPriceContext, error)
}

// NewDailyTrendFilter creates a new filter.
func NewDailyTrendFilter(provider DailyContextProvider) *DailyTrendFilter {
	return &DailyTrendFilter{dailyCtxProvider: provider}
}

// TrendAdjustment holds the result of a daily trend check for a signal.
type TrendAdjustment struct {
	AdjustedConfidence float64 // Final confidence after adjustment
	RawConfidence      float64 // Original confidence
	Adjustment         float64 // Delta applied (positive = boost, negative = penalty)
	DailyTrend         string  // "UP", "DOWN", "FLAT"
	MATrend            string  // "BULLISH", "BEARISH", "MIXED"
	Reason             string  // Human-readable explanation
}

// Adjust computes a confidence adjustment for a signal based on daily price trend.
// Returns the adjustment result. Never modifies the signal directly.
func (f *DailyTrendFilter) Adjust(ctx context.Context, contractCode, currency, direction string, confidence float64) *TrendAdjustment {
	result := &TrendAdjustment{
		AdjustedConfidence: confidence,
		RawConfidence:      confidence,
	}

	if f.dailyCtxProvider == nil {
		result.Reason = "no daily data provider"
		return result
	}

	dc, err := f.dailyCtxProvider.Build(ctx, contractCode, currency)
	if err != nil {
		dtfLog.Debug().Err(err).Str("contract", contractCode).Msg("daily context unavailable — no adjustment")
		result.Reason = "daily data unavailable"
		return result
	}

	result.DailyTrend = dc.DailyTrend
	result.MATrend = dc.MATrendDaily()

	bullish := direction == "BULLISH"
	adj := computeTrendAdjustment(dc, bullish)

	result.Adjustment = adj
	result.AdjustedConfidence = clampConfidence(confidence + adj)
	result.Reason = explainAdjustment(dc, bullish, adj)

	return result
}

// computeTrendAdjustment calculates the confidence delta based on daily indicators.
//
// Scoring logic (additive):
//   - Daily trend alignment:    +5 aligned, -5 opposed
//   - MA trend alignment:       +7 aligned, -5 opposed
//   - Price above/below DMA20:  +3 aligned, -3 opposed
//   - Momentum confirmation:    +3 if 5D momentum matches direction
//   - Consecutive day streak:   +2 if ≥3 days in signal direction
//
// Maximum boost: ~+20, maximum penalty: ~-16
// These are intentionally modest — daily data is supplementary, not primary.
func computeTrendAdjustment(dc *domain.DailyPriceContext, bullish bool) float64 {
	var adj float64

	// 1. Daily trend alignment (5-day)
	trendAligned := (bullish && dc.DailyTrend == "UP") || (!bullish && dc.DailyTrend == "DOWN")
	trendOpposed := (bullish && dc.DailyTrend == "DOWN") || (!bullish && dc.DailyTrend == "UP")

	if trendAligned {
		adj += 5
	} else if trendOpposed {
		adj -= 5
	}

	// 2. MA alignment (DMA20 > DMA50 > DMA200 for bullish, reversed for bearish)
	maTrend := dc.MATrendDaily()
	maAligned := (bullish && maTrend == "BULLISH") || (!bullish && maTrend == "BEARISH")
	maOpposed := (bullish && maTrend == "BEARISH") || (!bullish && maTrend == "BULLISH")

	if maAligned {
		adj += 7
	} else if maOpposed {
		adj -= 5
	}

	// 3. Price vs DMA20 (short-term support/resistance)
	if bullish && dc.AboveDMA20 {
		adj += 3
	} else if bullish && !dc.AboveDMA20 {
		adj -= 3
	} else if !bullish && !dc.AboveDMA20 {
		adj += 3
	} else if !bullish && dc.AboveDMA20 {
		adj -= 3
	}

	// 4. Momentum confirmation (5-day ROC)
	if bullish && dc.Momentum5D > 0.3 {
		adj += 3
	} else if !bullish && dc.Momentum5D < -0.3 {
		adj += 3
	}

	// 5. Consecutive day streak (≥3 days in signal direction)
	if dc.ConsecDays >= 3 {
		streakAligned := (bullish && dc.ConsecDir == "UP") || (!bullish && dc.ConsecDir == "DOWN")
		if streakAligned {
			adj += 2
		}
	}

	return adj
}

// explainAdjustment generates a human-readable reason for the adjustment.
func explainAdjustment(dc *domain.DailyPriceContext, bullish bool, adj float64) string {
	dir := "bullish"
	if !bullish {
		dir = "bearish"
	}

	if adj > 10 {
		return dir + " signal strongly confirmed by daily trend + MA alignment"
	} else if adj > 5 {
		return dir + " signal supported by daily trend"
	} else if adj > 0 {
		return dir + " signal weakly confirmed by daily data"
	} else if adj < -10 {
		return dir + " signal strongly opposed by daily trend + MA alignment"
	} else if adj < -5 {
		return dir + " signal opposed by daily trend"
	} else if adj < 0 {
		return dir + " signal weakly opposed by daily data"
	}
	return "daily trend neutral — no adjustment"
}

// clampConfidence ensures confidence stays within [5, 98] range.
func clampConfidence(c float64) float64 {
	return math.Max(5, math.Min(98, c))
}
