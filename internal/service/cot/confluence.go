// Package cot provides confluence scoring between COT positioning and calendar surprises.
package cot

import (
	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/pkg/mathutil"
)

// AdjustSentimentBySurprise modifies a COT sentiment score based on intra-week
// calendar surprises for the same currency.
//
// Logic: each sigma of surprise contributes ±5 points to the COT sentiment baseline.
// A hawkish surprise (positive sigma) on a bullish-COT currency = CONFLUENCE (upgrade).
// A dovish surprise (negative sigma) on a bullish-COT currency = DIVERGENCE (downgrade).
//
// Returns an adjusted sentiment label.
func AdjustSentimentBySurprise(analysis domain.COTAnalysis, surprises []domain.SurpriseRecord) string {
	baseSentiment := analysis.SentimentScore

	adjustment := 0.0
	for _, s := range surprises {
		if s.Currency == analysis.Contract.Currency {
			adjustment += s.SigmaValue * 5.0 // 5 points per sigma
		}
	}

	adjusted := mathutil.Clamp(baseSentiment+adjustment, -100, 100)
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
