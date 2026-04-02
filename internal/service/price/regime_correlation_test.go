package price

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestClassifyDays_Fallback(t *testing.T) {
	// Generate synthetic daily prices (newest-first): alternating calm and volatile.
	n := 40
	recs := make([]domain.DailyPrice, n)
	base := 100.0
	for i := n - 1; i >= 0; i-- {
		delta := 0.1
		if i%5 == 0 {
			delta = 3.0 // every 5th day is volatile
		}
		close := base + delta*float64(n-1-i)
		recs[i] = domain.DailyPrice{
			Date:  time.Now().AddDate(0, 0, -(n - 1 - i)),
			Open:  close - delta/2,
			High:  close + delta,
			Low:   close - delta,
			Close: close,
		}
	}

	tags := classifyDays(recs, nil, n-1)
	if len(tags) != n-1 {
		t.Fatalf("expected %d tags, got %d", n-1, len(tags))
	}

	// Should have at least one of each non-crisis tag.
	counts := map[RegimeTag]int{}
	for _, tg := range tags {
		counts[tg]++
	}
	if counts[RegimeTagTrending] == 0 {
		t.Error("expected at least one TRENDING tag")
	}
	if counts[RegimeTagRanging] == 0 {
		t.Error("expected at least one RANGING tag")
	}
}

func TestClassifyDays_WithViterbi(t *testing.T) {
	recs := make([]domain.DailyPrice, 20)
	for i := 0; i < 20; i++ {
		recs[i] = domain.DailyPrice{Close: 100 + float64(i)*0.1}
	}

	hmm := &HMMResult{
		ViterbiPath: []string{
			HMMRiskOn, HMMRiskOn, HMMRiskOff, HMMCrisis, HMMRiskOn,
			HMMRiskOn, HMMRiskOff, HMMRiskOff, HMMRiskOn, HMMRiskOn,
		},
	}

	tags := classifyDays(recs, hmm, 10)
	if len(tags) != 10 {
		t.Fatalf("expected 10 tags, got %d", len(tags))
	}

	// Viterbi is newest-first; our chronological output reverses it.
	// Last Viterbi element (index 9 = RISK_ON) should be tags[0] (oldest chronological).
	if tags[0] != RegimeTagTrending {
		t.Errorf("tags[0] = %s, want TRENDING (HMM RISK_ON)", tags[0])
	}
}

func TestBuildMatrixFromTagged(t *testing.T) {
	tagged := map[string][]taggedReturn{
		"EUR": {
			{0.5, RegimeTagTrending}, {0.3, RegimeTagTrending}, {-0.2, RegimeTagRanging},
			{0.1, RegimeTagTrending}, {0.4, RegimeTagTrending}, {-0.1, RegimeTagRanging},
			{0.6, RegimeTagTrending}, {0.2, RegimeTagRanging},
		},
		"GBP": {
			{0.4, RegimeTagTrending}, {0.2, RegimeTagTrending}, {-0.3, RegimeTagRanging},
			{0.2, RegimeTagTrending}, {0.5, RegimeTagTrending}, {-0.2, RegimeTagRanging},
			{0.5, RegimeTagTrending}, {0.1, RegimeTagRanging},
		},
	}

	// Overall (all tags).
	m := buildMatrixFromTagged([]string{"EUR", "GBP"}, tagged, "")
	if m == nil {
		t.Fatal("expected non-nil overall matrix")
	}
	if len(m.Currencies) != 2 {
		t.Fatalf("expected 2 currencies, got %d", len(m.Currencies))
	}
	if m.Matrix["EUR"]["EUR"] != 1.0 {
		t.Error("diagonal should be 1.0")
	}
	// EUR and GBP should be positively correlated given co-moving data.
	corr := m.Matrix["EUR"]["GBP"]
	if corr < 0.5 {
		t.Errorf("EUR-GBP overall correlation = %.3f, expected > 0.5", corr)
	}

	// Trending only.
	mt := buildMatrixFromTagged([]string{"EUR", "GBP"}, tagged, RegimeTagTrending)
	if mt == nil {
		t.Fatal("expected non-nil trending matrix")
	}
}

func TestCountTag(t *testing.T) {
	tags := []RegimeTag{RegimeTagTrending, RegimeTagRanging, RegimeTagCrisis, RegimeTagTrending}
	if got := countTag(tags, RegimeTagTrending); got != 2 {
		t.Errorf("countTag TRENDING = %d, want 2", got)
	}
	if got := countTag(tags, RegimeTagCrisis); got != 1 {
		t.Errorf("countTag CRISIS = %d, want 1", got)
	}
}

func TestDetectRegimeDivergences(t *testing.T) {
	currencies := []string{"EUR", "GBP"}
	overall := &domain.CorrelationMatrix{
		Currencies: currencies,
		Matrix: map[string]map[string]float64{
			"EUR": {"EUR": 1.0, "GBP": 0.80},
			"GBP": {"GBP": 1.0, "EUR": 0.80},
		},
	}
	regime := &domain.CorrelationMatrix{
		Currencies: currencies,
		Matrix: map[string]map[string]float64{
			"EUR": {"EUR": 1.0, "GBP": 0.30},
			"GBP": {"GBP": 1.0, "EUR": 0.30},
		},
	}

	divs := detectRegimeDivergences(currencies, overall, regime, RegimeTagCrisis)
	if len(divs) == 0 {
		t.Fatal("expected at least one divergence (0.80 vs 0.30 = delta 0.50)")
	}
	if divs[0].Significance != "HIGH" {
		t.Errorf("significance = %s, want HIGH", divs[0].Significance)
	}
}

func TestFilterCoreCurrencies(t *testing.T) {
	all := domain.DefaultCorrelationCurrencies()
	core := filterCoreCurrencies(all)
	if len(core) == 0 {
		t.Fatal("filterCoreCurrencies returned empty")
	}
	// EUR must be in core.
	found := false
	for _, c := range core {
		if c == "EUR" {
			found = true
		}
	}
	if !found {
		t.Error("EUR not in core currencies")
	}
}
