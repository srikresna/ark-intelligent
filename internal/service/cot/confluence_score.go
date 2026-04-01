// Package cot provides Confluence Score v2 — institutional-grade positioning score.
// Formula: FRED regime (30%) + COT positioning (35%) + Calendar surprise (20%) + Financial stress (15%)
package cot

import (
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// computeMacroComponentScore computes a unified macro conditions score from FRED data.
// Combines yield curve, inflation target, financial conditions (NFCI continuous),
// labor market, and GDP into a single -100..+100 score.
// This replaces the separate "stress" and "FRED" components to eliminate NFCI triple-counting.
func computeMacroComponentScore(macroData *fred.MacroData) float64 {
	if macroData == nil {
		return 0
	}

	raw := 0.0

	// Yield curve: positive spread = healthy economy
	switch {
	case macroData.YieldSpread > 0.5:
		raw += 25
	case macroData.YieldSpread > 0:
		raw += 10
	case macroData.YieldSpread > -0.5:
		raw -= 10
	default:
		raw -= 25 // deep inversion
	}

	// Inflation proximity to target (Core PCE)
	if macroData.CorePCE > 0 {
		switch {
		case macroData.CorePCE < 2.5:
			raw += 25 // on/below target
		case macroData.CorePCE < 3.5:
			raw += 5 // slightly elevated
		default:
			raw -= 20 // high inflation
		}
	}

	// Financial conditions (NFCI) — continuous scoring, single entry point
	// NFCI: negative = loose (good), positive = tight (bad)
	nfciScore := mathutil.Clamp(-macroData.NFCI*40, -25, 25)
	raw += nfciScore

	// Labor market
	if macroData.InitialClaims > 0 {
		switch {
		case macroData.InitialClaims < 220_000:
			raw += 15
		case macroData.InitialClaims < 280_000:
			raw += 5
		default:
			raw -= 15
		}
	}

	// GDP growth factor
	if macroData.GDPGrowth != 0 {
		switch {
		case macroData.GDPGrowth > 3.0:
			raw += 10
		case macroData.GDPGrowth > 1.5:
			raw += 5
		case macroData.GDPGrowth > 0:
			// neutral
		default:
			raw -= 15
		}
	}

	return mathutil.Clamp(raw, -100, 100)
}

// ConfluenceScoreV2 computes the institutional-grade confluence score for a currency.
//
// Components (4-factor):
//   - COT positioning   (35%) — based on SentimentScore (-100..+100)
//   - Calendar surprise  (20%) — based on recent sigma surprise for this currency
//   - Macro conditions   (45%) — unified FRED score (yield, PCE, NFCI, labor, GDP)
//
// Returns a score in [-100, +100]. Positive = bullish bias, negative = bearish bias.
func ConfluenceScoreV2(
	analysis domain.COTAnalysis,
	macroData *fred.MacroData,
	surpriseSigma float64,
) float64 {
	cotScore := mathutil.Clamp(analysis.SentimentScore, -100, 100)
	surpriseScore := mathutil.Clamp(surpriseSigma*20, -100, 100)
	macroScore := computeMacroComponentScore(macroData)

	if macroData != nil {
		total := cotScore*0.35 + surpriseScore*0.20 + macroScore*0.45
		return mathutil.Clamp(total, -100, 100)
	}

	// Without FRED data: weight COT (60%) + surprise (40%)
	total := cotScore*0.60 + surpriseScore*0.40
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
	Version      int     // 2 or 3 (formula version)

	// Component breakdown (V3 only)
	COTComponent      float64 // -100..+100 (weighted)
	CalendarComponent float64 // -100..+100 (weighted)
	MacroComponent    float64 // -100..+100 (weighted)
	PriceComponent    float64 // -100..+100 (weighted)
	SourcesAvailable  int     // how many of 4 sources contributed (1-4)
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

	return buildConvictionResult(analysis, conviction, regime.Name, calendarNote, 2)
}

// ---------------------------------------------------------------------------
// V3 — 5-component Confluence Score with Price Data
// ---------------------------------------------------------------------------

// ConfluenceScoreV3 computes a 4-component institutional-grade confluence score.
//
// Components:
//   - COT positioning    (30%) — based on SentimentScore (-100..+100)
//   - Calendar surprise   (15%) — based on recent sigma surprise
//   - Macro conditions    (25%) — per-currency macro differential (US vs counterpart)
//   - Price momentum      (30%) — volatility-normalized MA alignment + momentum + concordance
//
// The macro component uses per-country differentials when composite data is available:
// for non-USD currencies, macro = (US score - counterpart score) / 2, so a positive
// differential (US stronger) is bullish for USD vs that currency.
// Falls back to the unified US-centric macro score if composites cannot be computed.
//
// Returns a score in [-100, +100]. Positive = bullish bias, negative = bearish bias.
func ConfluenceScoreV3(
	analysis domain.COTAnalysis,
	macroData *fred.MacroData,
	surpriseSigma float64,
	priceContext *domain.PriceContext,
) float64 {
	cotScore := mathutil.Clamp(analysis.SentimentScore, -100, 100)
	surpriseScore := mathutil.Clamp(surpriseSigma*20, -100, 100)
	macroScore := computeMacroComponentScore(macroData)

	// Enhanced macro component: use per-currency differential if composites available
	if macroData != nil {
		composites := fred.ComputeComposites(macroData)
		if composites != nil {
			currency := analysis.Contract.Currency
			countryScore := getCountryMacroScore(currency, composites)
			if currency == "USD" || currency == "DXY" {
				// For USD index, use US score directly
				macroScore = mathutil.Clamp(composites.USScore, -100, 100)
			} else {
				// Differential: positive = USD stronger than counterpart = bullish for USD pair
				differential := composites.USScore - countryScore
				// Normalize differential (-200..+200 range) to -100..+100
				macroScore = mathutil.Clamp(differential/2, -100, 100)
			}
		}
	}

	// Price momentum component (30%)
	priceScore := 0.0
	if priceContext != nil {
		// MA alignment: above both MAs = bullish, below both = bearish
		maScore := 0.0
		if priceContext.AboveMA4W {
			maScore += 25
		} else {
			maScore -= 25
		}
		if priceContext.AboveMA13W {
			maScore += 25
		} else {
			maScore -= 25
		}

		// Momentum from weekly/monthly changes — volatility-normalized if ATR available
		weeklyMom := priceContext.WeeklyChgPct
		monthlyMom := priceContext.MonthlyChgPct
		if priceContext.NormalizedATR > 0 {
			// Z-score style: normalize by ATR percentage
			weeklyMom = weeklyMom / priceContext.NormalizedATR * 2
			monthlyMom = monthlyMom / priceContext.NormalizedATR * 2
		} else {
			// Fallback: raw scaling
			weeklyMom = weeklyMom * 10
			monthlyMom = monthlyMom * 5
		}

		momentumScore := mathutil.Clamp(weeklyMom, -25, 25) +
			mathutil.Clamp(monthlyMom, -25, 25)

		priceScore = mathutil.Clamp(maScore+momentumScore, -100, 100)

		// Price-COT concordance: agreement boosts, disagreement dampens
		cotBullish := analysis.SentimentScore > 20
		cotBearish := analysis.SentimentScore < -20
		priceBullish := priceContext.Trend4W == "UP"
		priceBearish := priceContext.Trend4W == "DOWN"

		if (cotBullish && priceBullish) || (cotBearish && priceBearish) {
			priceScore *= 1.2
		} else if (cotBullish && priceBearish) || (cotBearish && priceBullish) {
			priceScore *= 0.7
		}
		priceScore = mathutil.Clamp(priceScore, -100, 100)
	}

	// Weighted combination
	if priceContext != nil && macroData != nil {
		total := cotScore*0.30 + surpriseScore*0.15 + macroScore*0.25 + priceScore*0.30
		return mathutil.Clamp(total, -100, 100)
	} else if priceContext != nil {
		// No FRED: COT 40% + Surprise 15% + Price 45%
		total := cotScore*0.40 + surpriseScore*0.15 + priceScore*0.45
		return mathutil.Clamp(total, -100, 100)
	} else if macroData != nil {
		// No price: COT 35% + Surprise 20% + Macro 45%
		total := cotScore*0.35 + surpriseScore*0.20 + macroScore*0.45
		return mathutil.Clamp(total, -100, 100)
	}

	// Neither: COT 60% + Surprise 40%
	total := cotScore*0.60 + surpriseScore*0.40
	return mathutil.Clamp(total, -100, 100)
}

// ComputeConvictionScoreV3 generates a unified 0-100 conviction score using
// the 4-component V3 formula. Regime multiplier is scaled by regime intensity.
func ComputeConvictionScoreV3(
	analysis domain.COTAnalysis,
	regime fred.MacroRegime,
	surpriseSigma float64,
	calendarNote string,
	macroData *fred.MacroData,
	priceContext *domain.PriceContext,
) ConvictionScore {
	baseScore := ConfluenceScoreV3(analysis, macroData, surpriseSigma, priceContext)

	// Scale regime multiplier by regime intensity (Score 0-100)
	multiplier := FREDRegimeMultiplier(analysis.Contract.Currency, regime)
	regimeIntensity := float64(regime.Score) / 100.0
	scaledMultiplier := multiplier * regimeIntensity
	adjusted := mathutil.Clamp(baseScore+scaledMultiplier, -100, 100)
	conviction := (adjusted + 100) / 2

	cs := buildConvictionResult(analysis, conviction, regime.Name, calendarNote, 3)

	// Populate component breakdown
	cs.COTComponent = mathutil.Clamp(analysis.SentimentScore, -100, 100)
	cs.CalendarComponent = mathutil.Clamp(surpriseSigma*20, -100, 100)
	cs.MacroComponent = computeMacroComponentScore(macroData)

	// Enhanced macro breakdown: use per-currency differential if composites available
	if macroData != nil {
		composites := fred.ComputeComposites(macroData)
		if composites != nil {
			currency := analysis.Contract.Currency
			countryScore := getCountryMacroScore(currency, composites)
			if currency == "USD" || currency == "DXY" {
				cs.MacroComponent = mathutil.Clamp(composites.USScore, -100, 100)
			} else {
				differential := composites.USScore - countryScore
				cs.MacroComponent = mathutil.Clamp(differential/2, -100, 100)
			}
		}
	}
	if priceContext != nil {
		// Recompute price component for breakdown (same logic as V3)
		maScore := 0.0
		if priceContext.AboveMA4W {
			maScore += 25
		} else {
			maScore -= 25
		}
		if priceContext.AboveMA13W {
			maScore += 25
		} else {
			maScore -= 25
		}
		weeklyMom := priceContext.WeeklyChgPct
		monthlyMom := priceContext.MonthlyChgPct
		if priceContext.NormalizedATR > 0 {
			weeklyMom = weeklyMom / priceContext.NormalizedATR * 2
			monthlyMom = monthlyMom / priceContext.NormalizedATR * 2
		} else {
			weeklyMom = weeklyMom * 10
			monthlyMom = monthlyMom * 5
		}
		momentumScore := mathutil.Clamp(weeklyMom, -25, 25) + mathutil.Clamp(monthlyMom, -25, 25)
		cs.PriceComponent = mathutil.Clamp(maScore+momentumScore, -100, 100)
	}

	// Count available sources
	cs.SourcesAvailable = 1 // COT always available
	if surpriseSigma != 0 {
		cs.SourcesAvailable++
	}
	if macroData != nil {
		cs.SourcesAvailable++
	}
	if priceContext != nil {
		cs.SourcesAvailable++
	}

	return cs
}

// ConfluenceWeights defines custom factor weights for the confluence score.
// All values are expressed as fractions (e.g., 0.25 for 25%).
type ConfluenceWeights struct {
	COT      float64 // COT positioning weight
	Calendar float64 // Calendar surprise weight
	Stress   float64 // Financial stress weight
	FRED     float64 // FRED regime weight
	Price    float64 // Price momentum weight
}

// DefaultWeightsV3 returns the V3 weights. Stress is merged into FRED (Macro).
func DefaultWeightsV3() ConfluenceWeights {
	return ConfluenceWeights{
		COT:      0.30,
		Calendar: 0.15,
		Stress:   0.00, // merged into FRED/Macro
		FRED:     0.25, // unified macro component
		Price:    0.30,
	}
}

// ConfluenceScoreWithWeights computes a 4-component confluence score using
// custom weights instead of hardcoded values.
func ConfluenceScoreWithWeights(
	analysis domain.COTAnalysis,
	macroData *fred.MacroData,
	surpriseSigma float64,
	priceContext *domain.PriceContext,
	weights ConfluenceWeights,
) float64 {
	cotScore := mathutil.Clamp(analysis.SentimentScore, -100, 100)
	surpriseScore := mathutil.Clamp(surpriseSigma*20, -100, 100)
	macroScore := computeMacroComponentScore(macroData)

	// Price momentum component
	priceScore := 0.0
	if priceContext != nil {
		maScore := 0.0
		if priceContext.AboveMA4W {
			maScore += 25
		} else {
			maScore -= 25
		}
		if priceContext.AboveMA13W {
			maScore += 25
		} else {
			maScore -= 25
		}
		weeklyMom := priceContext.WeeklyChgPct
		monthlyMom := priceContext.MonthlyChgPct
		if priceContext.NormalizedATR > 0 {
			weeklyMom = weeklyMom / priceContext.NormalizedATR * 2
			monthlyMom = monthlyMom / priceContext.NormalizedATR * 2
		} else {
			weeklyMom = weeklyMom * 10
			monthlyMom = monthlyMom * 5
		}
		momentumScore := mathutil.Clamp(weeklyMom, -25, 25) + mathutil.Clamp(monthlyMom, -25, 25)
		priceScore = mathutil.Clamp(maScore+momentumScore, -100, 100)

		cotBullish := analysis.SentimentScore > 20
		cotBearish := analysis.SentimentScore < -20
		priceBullish := priceContext.Trend4W == "UP"
		priceBearish := priceContext.Trend4W == "DOWN"
		if (cotBullish && priceBullish) || (cotBearish && priceBearish) {
			priceScore *= 1.2
		} else if (cotBullish && priceBearish) || (cotBearish && priceBullish) {
			priceScore *= 0.7
		}
		priceScore = mathutil.Clamp(priceScore, -100, 100)
	}

	// Merge Stress weight into Macro (since they're now unified)
	macroWeight := weights.FRED + weights.Stress

	if priceContext != nil && macroData != nil {
		total := cotScore*weights.COT + surpriseScore*weights.Calendar + macroScore*macroWeight + priceScore*weights.Price
		return mathutil.Clamp(total, -100, 100)
	} else if priceContext != nil {
		sum := weights.COT + weights.Calendar + weights.Price
		if sum > 0 {
			total := cotScore*(weights.COT/sum) + surpriseScore*(weights.Calendar/sum) + priceScore*(weights.Price/sum)
			return mathutil.Clamp(total, -100, 100)
		}
	} else if macroData != nil {
		sum := weights.COT + weights.Calendar + macroWeight
		if sum > 0 {
			total := cotScore*(weights.COT/sum) + surpriseScore*(weights.Calendar/sum) + macroScore*(macroWeight/sum)
			return mathutil.Clamp(total, -100, 100)
		}
	}

	sum := weights.COT + weights.Calendar
	if sum > 0 {
		total := cotScore*(weights.COT/sum) + surpriseScore*(weights.Calendar/sum)
		return mathutil.Clamp(total, -100, 100)
	}
	return 0
}

// getCountryMacroScore returns the macro score for the country associated with
// a given currency. Used by the per-currency macro differential in V3.
func getCountryMacroScore(currency string, c *domain.MacroComposites) float64 {
	if c == nil {
		return 0
	}
	switch currency {
	case "EUR":
		return c.EZScore
	case "GBP":
		return c.UKScore
	case "JPY":
		return c.JPScore
	case "AUD":
		return c.AUScore
	case "CAD":
		return c.CAScore
	case "NZD":
		return c.NZScore
	case "CHF":
		return c.EZScore // CHF closely tied to Eurozone
	default:
		return c.USScore // fallback: no differential
	}
}

// buildConvictionResult creates a ConvictionScore from a normalized 0-100 conviction value.
func buildConvictionResult(analysis domain.COTAnalysis, conviction float64, regimeName, calendarNote string, version int) ConvictionScore {
	direction := "NEUTRAL"
	switch {
	case conviction > 55:
		direction = "LONG"
	case conviction < 45:
		direction = "SHORT"
	}

	cotBias := "NEUTRAL"
	switch {
	case analysis.SentimentScore > 30:
		cotBias = "BULLISH"
	case analysis.SentimentScore < -30:
		cotBias = "BEARISH"
	}

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
		FREDRegime:   regimeName,
		CalendarBias: calendarNote,
		Label:        label,
		Version:      version,
	}
}
