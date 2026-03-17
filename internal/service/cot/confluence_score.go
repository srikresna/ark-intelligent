// Package cot provides Confluence Score v2 — institutional-grade positioning score.
// Formula: FRED regime (30%) + COT positioning (35%) + Calendar surprise (20%) + Financial stress (15%)
package cot

import (
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
