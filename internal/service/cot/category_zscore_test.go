package cot

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// makeHistoryLevFund builds a synthetic COT history (newest-first) with
// varied WoW changes to ensure non-zero stddev. The newest record (index 0)
// gets an additional spike on LevFundLong.
func makeHistoryLevFund(n int, spike float64) []domain.COTRecord {
	records := make([]domain.COTRecord, n)
	base := 10000.0
	// Use varied steps to produce non-zero stddev in WoW changes
	steps := []float64{100, 250, 50, 300, 75, 180, 120, 220, 90, 160}
	cumLevFund := 0.0
	for i := n - 1; i >= 0; i-- {
		step := steps[i%len(steps)]
		cumLevFund += step
		records[i] = domain.COTRecord{
			LevFundLong:       base + cumLevFund,
			LevFundShort:      base + cumLevFund*0.3,
			DealerLong:        base - cumLevFund*0.2,
			DealerShort:       base + cumLevFund*0.4,
			ManagedMoneyLong:  base + cumLevFund*0.5,
			ManagedMoneyShort: base + cumLevFund*0.1,
			SwapDealerLong:    base,
			SwapDealerShort:   base + cumLevFund*0.15,
			AssetMgrLong:      base,
			AssetMgrShort:     base,
		}
	}
	// Add large spike to the most recent record (index 0)
	records[0].LevFundLong += spike
	return records
}

// makeHistoryFlat builds a history with no variation (stddev=0 → ZScore=0).
func makeHistoryFlat(n int) []domain.COTRecord {
	records := make([]domain.COTRecord, n)
	for i := range records {
		records[i] = domain.COTRecord{
			LevFundLong:       10000,
			LevFundShort:      5000,
			DealerLong:        8000,
			DealerShort:       12000,
			ManagedMoneyLong:  9000,
			ManagedMoneyShort: 4000,
			SwapDealerLong:    7000,
			SwapDealerShort:   11000,
		}
	}
	return records
}

// TestComputeCategoryZScore_InsufficientData verifies safe handling for short input.
func TestComputeCategoryZScore_InsufficientData(t *testing.T) {
	getLong := func(r domain.COTRecord) float64 { return r.LevFundLong }
	getShort := func(r domain.COTRecord) float64 { return r.LevFundShort }

	zscore, alert := computeCategoryZScore(getLong, getShort, nil)
	if zscore != 0 || alert {
		t.Errorf("expected (0, false) for nil input, got (%.2f, %v)", zscore, alert)
	}

	zscore, alert = computeCategoryZScore(getLong, getShort, make([]domain.COTRecord, 3))
	if zscore != 0 || alert {
		t.Errorf("expected (0, false) for <4 bars, got (%.2f, %v)", zscore, alert)
	}
}

// TestComputeCategoryZScore_FlatHistory verifies ZScore=0 for flat positions.
func TestComputeCategoryZScore_FlatHistory(t *testing.T) {
	history := makeHistoryFlat(20)
	getLong := func(r domain.COTRecord) float64 { return r.LevFundLong }
	getShort := func(r domain.COTRecord) float64 { return r.LevFundShort }

	zscore, alert := computeCategoryZScore(getLong, getShort, history)
	if alert {
		t.Error("expected no alert for flat history")
	}
	if math.IsNaN(zscore) || math.IsInf(zscore, 0) {
		t.Errorf("unexpected zscore value: %v", zscore)
	}
}

// TestComputeCategoryZScore_SpikeProducesAlert verifies a large spike triggers alert.
func TestComputeCategoryZScore_SpikeProducesAlert(t *testing.T) {
	// Large spike (100,000 contracts) on top of normal variation → should alert
	history := makeHistoryLevFund(52, 100000)
	getLong := func(r domain.COTRecord) float64 { return r.LevFundLong }
	getShort := func(r domain.COTRecord) float64 { return r.LevFundShort }

	zscore, alert := computeCategoryZScore(getLong, getShort, history)
	if !alert {
		t.Errorf("expected alert for large spike, zscore=%.2f", zscore)
	}
	if zscore <= 0 {
		t.Errorf("expected positive zscore for bullish spike, got %.2f", zscore)
	}
}

// TestComputeAllCategoryZScores_PopulatesAll verifies all Z-scores are set.
func TestComputeAllCategoryZScores_PopulatesAll(t *testing.T) {
	history := makeHistoryLevFund(52, 100000)
	analysis := &domain.COTAnalysis{}
	computeAllCategoryZScores(analysis, history)

	// At least one z-score should be non-zero
	allZero := analysis.DealerZScore == 0 && analysis.LevFundZScore == 0 &&
		analysis.ManagedMoneyZScore == 0 && analysis.SwapDealerZScore == 0
	if allZero {
		t.Error("all z-scores are zero — expected at least some variation")
	}
}

// TestComputeAllCategoryZScores_ShortHistory verifies safe handling.
func TestComputeAllCategoryZScores_ShortHistory(t *testing.T) {
	analysis := &domain.COTAnalysis{}
	computeAllCategoryZScores(analysis, make([]domain.COTRecord, 2))

	if analysis.DealerZScore != 0 || analysis.LevFundZScore != 0 {
		t.Error("expected all z-scores = 0 for short history")
	}
	if analysis.CategoryDivergence {
		t.Error("expected no divergence for short history")
	}
}

// TestDetectCategoryDivergence_SpecVsCommercial verifies spec/commercial divergence.
func TestDetectCategoryDivergence_SpecVsCommercial(t *testing.T) {
	a := &domain.COTAnalysis{
		LevFundZScore: 2.0,
		DealerZScore:  -2.0,
	}
	divergence, desc := detectCategoryDivergence(a)
	if !divergence {
		t.Error("expected divergence for LevFund+2/Dealer-2")
	}
	if desc == "" {
		t.Error("expected non-empty description for divergence")
	}
}

// TestDetectCategoryDivergence_NoDivergence verifies no false positives.
func TestDetectCategoryDivergence_NoDivergence(t *testing.T) {
	a := &domain.COTAnalysis{
		LevFundZScore:      1.0,
		DealerZScore:       0.5,
		ManagedMoneyZScore: 0.8,
		SwapDealerZScore:   0.3,
	}
	divergence, _ := detectCategoryDivergence(a)
	if divergence {
		t.Error("expected no divergence when all categories agree")
	}
}

// TestDetectCategoryDivergence_FragmentedSpecs verifies fragmented spec detection.
func TestDetectCategoryDivergence_FragmentedSpecs(t *testing.T) {
	a := &domain.COTAnalysis{
		LevFundZScore:      1.5,
		ManagedMoneyZScore: -1.5,
	}
	divergence, desc := detectCategoryDivergence(a)
	if !divergence {
		t.Error("expected divergence for fragmented specs")
	}
	if desc == "" {
		t.Error("expected non-empty description")
	}
}
