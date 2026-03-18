// Package cot provides Confluence Score v2 — institutional-grade positioning score.
// Formula: FRED regime (30%) + COT positioning (35%) + Calendar surprise (20%) + Financial stress (15%)
package cot

import (
	"fmt"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
	"github.com/arkcode369/ff-calendar-bot/pkg/mathutil"
)

// ConfluenceScoreV2 computes the institutional-grade confluence score for a currency.
//
// Components:
//   - COT positioning   (35%) — based on SentimentScore (-100..+100)
//   - Calendar surprise (20%) — based on recent sigma surprise for this currency
//   - Financial stress  (15%) — based on FRED NFCI (negative = loose = bullish risk)
//   - FRED regime       (30%) — composite from yield curve, PCE, NFCI, initial claims
//
// Returns a score in [-100, +100]. Positive = bullish bias, negative = bearish bias.
func ConfluenceScoreV2(
	analysis domain.COTAnalysis,
	macroData *fred.MacroData,
	surpriseSigma float64, // recent sigma surprise for this currency (positive = hawkish)
) float64 {
	// 1. COT component (35%)
	cotScore := mathutil.Clamp(analysis.SentimentScore, -100, 100)

	// 2. Calendar surprise component (20%)
	// Scale: 1 sigma → +20 points; 5 sigma → capped at ±100
	surpriseScore := mathutil.Clamp(surpriseSigma*20, -100, 100)

	// 3. Financial stress component (15%)
	// NFCI negative = loose = bullish; positive = tight = bearish
	stressScore := 0.0
	if macroData != nil {
		stressScore = mathutil.Clamp(-macroData.NFCI*50, -100, 100)
	}

	// 4. FRED regime component (30%)
	// Points: yield curve steepening (+30), disinflationary (+30),
	// loose conditions (+20), strong labor (+20) → raw 0..100 normalized to -100..+100
	fredRaw := 0.0
	if macroData != nil {
		if macroData.YieldSpread > 0 {
			fredRaw += 30
		}
		if macroData.CorePCE > 0 && macroData.CorePCE < 2.5 {
			fredRaw += 30
		}
		if macroData.NFCI < 0 {
			fredRaw += 20
		}
		if macroData.InitialClaims > 0 && macroData.InitialClaims < 250_000 {
			fredRaw += 20
		}
		// Normalize: 0..100 raw → -100..+100 (50 = neutral)
		fredScore := mathutil.Clamp(fredRaw-50, -100, 100)

		total := cotScore*0.35 + surpriseScore*0.20 + stressScore*0.15 + fredScore*0.30
		return mathutil.Clamp(total, -100, 100)
	}

	// Without FRED data: weight COT (50%) + surprise (30%) + no stress/FRED
	total := cotScore*0.50 + surpriseScore*0.30 + stressScore*0.20
	return mathutil.Clamp(total, -100, 100)
}

// ---------------------------------------------------------------------------
// Gap B — FRED Macro Regime as per-currency COT bias multiplier
// ---------------------------------------------------------------------------

// FREDRegimeMultiplier returns a per-currency score adjustment (-20 to +20) based on
// the FRED macro regime. This allows the regime to act as a mathematical multiplier
// on COT signals rather than only influencing them at the AI prompt level.
//
// Regime × Currency matrix:
//
//	                   USD   EUR/GBP/CHF   AUD/NZD/CAD   JPY/XAU
//	INFLATIONARY      +15       -10           -10           +5
//	DISINFLATIONARY    -5       +10           +10            0
//	STRESS            -10         0           -20          +20
//	RECESSION         -15         0           -20          +20
//	STAGFLATION         0        -5           -15          +15
//	GOLDILOCKS         -5        +5           +15           -5
//	default             0         0             0            0
func FREDRegimeMultiplier(currency string, regime fred.MacroRegime) float64 {
	type row struct {
		usd, eurGbpChf, audNzdCad, jpyXau float64
	}

	matrix := map[string]row{
		"INFLATIONARY":   {+15, -10, -10, +5},
		"DISINFLATIONARY": {-5, +10, +10, 0},
		"STRESS":         {-10, 0, -20, +20},
		"RECESSION":      {-15, 0, -20, +20},
		"STAGFLATION":    {0, -5, -15, +15},
		"GOLDILOCKS":     {-5, +5, +15, -5},
	}

	r, ok := matrix[regime.Name]
	if !ok {
		return 0
	}

	switch currency {
	case "USD":
		return r.usd
	case "EUR", "GBP", "CHF":
		return r.eurGbpChf
	case "AUD", "NZD", "CAD":
		return r.audNzdCad
	case "JPY", "XAU":
		return r.jpyXau
	default:
		return 0
	}
}

// ComputeRegimeAdjustedScore computes a sentiment score that incorporates the
// FRED macro regime multiplier for the given currency.
//
// Formula: adjusted = Clamp(SentimentScore + FREDRegimeMultiplier(currency, regime), -100, 100)
//
// This is the mathematical link between FRED regime and COT score (Gap B).
func ComputeRegimeAdjustedScore(analysis domain.COTAnalysis, regime fred.MacroRegime) float64 {
	multiplier := FREDRegimeMultiplier(analysis.Contract.Currency, regime)
	return mathutil.Clamp(analysis.SentimentScore+multiplier, -100, 100)
}

// ---------------------------------------------------------------------------
// Gap D — Conviction Score across all 3 sources
// ---------------------------------------------------------------------------

// ConvictionScore represents the unified cross-source conviction for a currency,
// combining COT positioning, FRED macro regime, and calendar surprise data into
// a single actionable score.
type ConvictionScore struct {
	Currency     string  // e.g. "EUR"
	Score        float64 // 0-100
	Direction    string  // "LONG", "SHORT", "NEUTRAL"
	COTBias      string  // e.g. "BULLISH"
	FREDRegime   string  // e.g. "DISINFLATIONARY"
	CalendarBias string  // e.g. "ECB hawkish"
	Label        string  // e.g. "HIGH CONVICTION LONG"
}

// ComputeConvictionScore generates a unified 0-100 conviction score from all 3 data sources:
// COT positioning, FRED macro regime, and economic calendar surprise.
//
// Algorithm:
//  1. Compute base score via ConfluenceScoreV2 → range -100..+100
//  2. Apply per-currency FREDRegimeMultiplier → adjusted range -100..+100
//  3. Normalize to 0-100: conviction = (adjusted + 100) / 2
//  4. Classify direction: >55 = LONG, <45 = SHORT, else = NEUTRAL
//  5. Classify label: >75 = HIGH CONVICTION, 60-75 = MODERATE, else = LOW
func ComputeConvictionScore(
	analysis domain.COTAnalysis,
	regime fred.MacroRegime,
	surpriseSigma float64,
	calendarNote string,
	macroData *fred.MacroData,
) ConvictionScore {
	// 1. Base score from all 3 sources (-100..+100)
	baseScore := ConfluenceScoreV2(analysis, macroData, surpriseSigma)

	// 2. Apply FRED regime multiplier
	multiplier := FREDRegimeMultiplier(analysis.Contract.Currency, regime)
	adjusted := mathutil.Clamp(baseScore+multiplier, -100, 100)

	// 3. Normalize to 0-100
	conviction := (adjusted + 100) / 2

	// 4. Direction
	direction := "NEUTRAL"
	switch {
	case conviction > 55:
		direction = "LONG"
	case conviction < 45:
		direction = "SHORT"
	}

	// 5. COT bias label from sentiment score
	cotBias := "NEUTRAL"
	switch {
	case analysis.SentimentScore > 30:
		cotBias = "BULLISH"
	case analysis.SentimentScore < -30:
		cotBias = "BEARISH"
	}

	// 6. Conviction label
	var label string
	switch {
	case conviction > 75:
		label = fmt.Sprintf("HIGH CONVICTION %s", direction)
	case conviction > 60:
		label = fmt.Sprintf("MODERATE %s", direction)
	default:
		label = fmt.Sprintf("LOW %s", direction)
	}

	return ConvictionScore{
		Currency:     analysis.Contract.Currency,
		Score:        conviction,
		Direction:    direction,
		COTBias:      cotBias,
		FREDRegime:   regime.Name,
		CalendarBias: calendarNote,
		Label:        label,
	}
}
