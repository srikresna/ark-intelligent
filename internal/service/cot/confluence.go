// Package cot provides confluence scoring between COT positioning and calendar surprises.
package cot

import (
	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
	"github.com/arkcode369/ff-calendar-bot/pkg/mathutil"
)

// AdjustSentimentBySurprise modifies a COT sentiment score based on intra-week
// calendar surprises for the same currency AND FRED macro context.
//
// Gap A implementation: FRED data now quantitatively adjusts the sentiment score.
//
// Logic (in order):
//  1. Apply calendar surprise adjustment: each sigma ±5 points.
//  2. Apply FRED yield-curve penalty: inverted curve → -15.
//  3. Apply FRED financial stress penalty: NFCI > 0.5 → -20.
//  4. Apply FRED inflation regime adjustment per currency:
//     - INFLATIONARY (CorePCE > 3.5): USD → +10, AUD/NZD → -10
//     - DISINFLATIONARY (CorePCE < 2.0): AUD/NZD/CAD → +10
//
// Returns an adjusted sentiment label.
func AdjustSentimentBySurprise(analysis domain.COTAnalysis, surprises []domain.SurpriseRecord, macroData *fred.MacroData) string {
	baseSentiment := analysis.SentimentScore
	currency := analysis.Contract.Currency

	// 1. Calendar surprise adjustment: 5 points per sigma
	newsAdj := 0.0
	for _, s := range surprises {
		if s.Currency == currency {
			newsAdj += s.SigmaValue * 5.0
		}
	}

	// 2-4. FRED adjustments
	fredAdj := 0.0
	if macroData != nil {
		// 2. Yield curve penalty
		if macroData.YieldSpread < 0 {
			fredAdj -= 15.0
		}

		// 3. Financial stress penalty
		if macroData.NFCI > 0.5 {
			fredAdj -= 20.0
		}

		// 4. Inflation regime per-currency adjustment
		switch {
		case macroData.CorePCE > 3.5: // INFLATIONARY
			switch currency {
			case "USD":
				fredAdj += 10.0
			case "AUD", "NZD":
				fredAdj -= 10.0
			}
		case macroData.CorePCE > 0 && macroData.CorePCE < 2.0: // DISINFLATIONARY
			switch currency {
			case "AUD", "NZD", "CAD":
				fredAdj += 10.0
			}
		}
	}

	adjusted := mathutil.Clamp(baseSentiment+newsAdj+fredAdj, -100, 100)
	switch {
	case adjusted > 60:
		return "STRONG BULLISH"
	case adjusted > 30:
		return "BULLISH"
	case adjusted > -30:
		return "NEUTRAL"
	case adjusted > -60:
		return "BEARISH"
	default:
		return "STRONG BEARISH"
	}
}

// ConfluenceType classifies the relationship between COT bias and event surprise.
type ConfluenceType string

const (
	// ConfluenceConfirmed — COT direction and surprise direction agree.
	ConfluenceConfirmed ConfluenceType = "CONFLUENCE"
	// ConfluenceDivergence — COT direction and surprise direction conflict.
	ConfluenceDivergence ConfluenceType = "DIVERGENCE"
	// ConfluenceNeutral — one or both signals are flat/uncertain.
	ConfluenceNeutral ConfluenceType = "NEUTRAL"
)

// ClassifyConfluence returns whether a COT analysis and event surprise are
// CONFLUENCE, DIVERGENCE, or NEUTRAL.
//
// cotBullish: true if COT sentiment is positive (net long / index > 50)
// surpriseSigma: positive = hawkish/bullish release, negative = dovish/bearish
func ClassifyConfluence(cotBullish bool, surpriseSigma float64) ConfluenceType {
	if surpriseSigma > 0.3 && cotBullish {
		return ConfluenceConfirmed
	}
	if surpriseSigma < -0.3 && !cotBullish {
		return ConfluenceConfirmed
	}
	if surpriseSigma > 0.3 && !cotBullish {
		return ConfluenceDivergence
	}
	if surpriseSigma < -0.3 && cotBullish {
		return ConfluenceDivergence
	}
	return ConfluenceNeutral
}

// AdjustSurpriseByFREDContext returns an adjusted surprise sigma considering the FRED macro regime.
//
// Gap C implementation: News surprise scores are now filtered by FRED context.
//
// A hawkish CPI surprise in a STRESS or RECESSION regime is dampened because
// risk-off dynamics override fundamental hawkishness. In INFLATIONARY regimes
// the effect is amplified for USD and dampened for non-USD currencies.
//
// Rules:
//   - STRESS / RECESSION: hawkish surprise × 0.5; dovish surprise × 1.5
//   - INFLATIONARY + USD: hawkish surprise × 1.2
//   - INFLATIONARY + non-USD: hawkish surprise × 0.8
//   - default: return sigma unchanged
func AdjustSurpriseByFREDContext(currency string, surpriseSigma float64, regime fred.MacroRegime) float64 {
	switch regime.Name {
	case "STRESS", "RECESSION":
		if surpriseSigma > 0 {
			// Risk-off dominates — hawkish signals are blunted
			return surpriseSigma * 0.5
		}
		// Dovish surprises are amplified (confirm risk-off)
		return surpriseSigma * 1.5

	case "INFLATIONARY":
		if surpriseSigma > 0 {
			if currency == "USD" {
				// Hawkish surprise + inflationary regime = stronger USD signal
				return surpriseSigma * 1.2
			}
			// Hawkish surprise for non-USD currencies is dampened
			return surpriseSigma * 0.8
		}
	}

	return surpriseSigma
}
