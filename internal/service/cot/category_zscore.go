package cot

// category_zscore.go — Per-category Z-Score analysis for COT positioning.
//
// Computes Z-scores for Dealer, LevFund, ManagedMoney, and SwapDealer
// categories, plus a cross-category divergence signal.

import (
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// computeCategoryZScore computes a Z-score for a given category's
// WoW net position change vs historical distribution.
//
// history is newest-first, index 0 = current week.
// getLong/getShort extract the category's long/short positions from a record.
//
// Returns (zscore, alert) where alert=true if |zscore| >= 2.0.
func computeCategoryZScore(getLong, getShort func(r domain.COTRecord) float64, history []domain.COTRecord) (zscore float64, alert bool) {
	if len(history) < 4 {
		return 0, false
	}

	// Collect WoW net position changes, skipping history[0] vs history[1]
	// (the current week's change) to avoid biasing the distribution.
	var changes []float64
	for i := 2; i < len(history); i++ {
		prev := getLong(history[i-1]) - getShort(history[i-1])
		curr := getLong(history[i]) - getShort(history[i])
		changes = append(changes, prev-curr)
	}

	if len(changes) < 2 {
		return 0, false
	}

	// Current week's WoW change
	currentNet := getLong(history[0]) - getShort(history[0])
	prevNet := getLong(history[1]) - getShort(history[1])
	currentChange := currentNet - prevNet

	avg := mathutil.Mean(changes)
	stdDev := mathutil.StdDevSample(changes)

	if stdDev <= 0 {
		return 0, false
	}

	zscore = (currentChange - avg) / stdDev
	alert = math.Abs(zscore) >= 2.0
	return zscore, alert
}

// computeAllCategoryZScores populates all per-category Z-scores on the analysis.
// Call this after the basic COT analysis fields are set.
func computeAllCategoryZScores(analysis *domain.COTAnalysis, history []domain.COTRecord) {
	if len(history) < 4 {
		return
	}

	// Dealer
	analysis.DealerZScore, analysis.DealerAlert = computeCategoryZScore(
		func(r domain.COTRecord) float64 { return r.DealerLong },
		func(r domain.COTRecord) float64 { return r.DealerShort },
		history,
	)

	// LevFund
	analysis.LevFundZScore, analysis.LevFundAlert = computeCategoryZScore(
		func(r domain.COTRecord) float64 { return r.LevFundLong },
		func(r domain.COTRecord) float64 { return r.LevFundShort },
		history,
	)

	// ManagedMoney
	analysis.ManagedMoneyZScore, analysis.ManagedMoneyAlert = computeCategoryZScore(
		func(r domain.COTRecord) float64 { return r.ManagedMoneyLong },
		func(r domain.COTRecord) float64 { return r.ManagedMoneyShort },
		history,
	)

	// SwapDealer
	analysis.SwapDealerZScore, analysis.SwapDealerAlert = computeCategoryZScore(
		func(r domain.COTRecord) float64 { return r.SwapDealerLong },
		func(r domain.COTRecord) float64 { return r.SwapDealerShort },
		history,
	)

	// Cross-category divergence
	analysis.CategoryDivergence, analysis.CategoryDivergenceDesc = detectCategoryDivergence(analysis)
}

// detectCategoryDivergence identifies significant disagreement between categories.
//
// Logic:
//   - Compare LevFund (speculative) vs Dealer (commercial) net direction.
//   - If LevFund Z > +1.5 and Dealer Z < -1.5 → "specs buying, dealers selling"
//   - If LevFund Z < -1.5 and Dealer Z > +1.5 → "specs selling, dealers buying"
//   - If ManagedMoney and LevFund diverge (opposite signs, |z| > 1.0) → fragmented spec camp
//
// Returns (hasDivergence, description).
func detectCategoryDivergence(a *domain.COTAnalysis) (bool, string) {
	const threshold = 1.5
	const fragThreshold = 1.0

	// Spec vs Commercial divergence (strongest institutional signal)
	if a.LevFundZScore > threshold && a.DealerZScore < -threshold {
		return true, fmt.Sprintf("Specs buying (LevFund z=%.1f) vs Dealers selling (Dealer z=%.1f) — risk-on divergence",
			a.LevFundZScore, a.DealerZScore)
	}
	if a.LevFundZScore < -threshold && a.DealerZScore > threshold {
		return true, fmt.Sprintf("Specs selling (LevFund z=%.1f) vs Dealers buying (Dealer z=%.1f) — risk-off divergence",
			a.LevFundZScore, a.DealerZScore)
	}

	// Fragmented speculative camp (LevFund vs ManagedMoney disagree)
	if a.LevFundZScore > fragThreshold && a.ManagedMoneyZScore < -fragThreshold {
		return true, fmt.Sprintf("Fragmented specs: LevFund bullish (z=%.1f), ManagedMoney bearish (z=%.1f)",
			a.LevFundZScore, a.ManagedMoneyZScore)
	}
	if a.LevFundZScore < -fragThreshold && a.ManagedMoneyZScore > fragThreshold {
		return true, fmt.Sprintf("Fragmented specs: LevFund bearish (z=%.1f), ManagedMoney bullish (z=%.1f)",
			a.LevFundZScore, a.ManagedMoneyZScore)
	}

	// SwapDealer extreme vs LevFund extreme
	if a.SwapDealerZScore > threshold && a.LevFundZScore < -threshold {
		return true, fmt.Sprintf("SwapDealers buying (z=%.1f) vs LevFunds selling (z=%.1f) — structural divergence",
			a.SwapDealerZScore, a.LevFundZScore)
	}
	if a.SwapDealerZScore < -threshold && a.LevFundZScore > threshold {
		return true, fmt.Sprintf("SwapDealers selling (z=%.1f) vs LevFunds buying (z=%.1f) — structural divergence",
			a.SwapDealerZScore, a.LevFundZScore)
	}

	return false, ""
}
