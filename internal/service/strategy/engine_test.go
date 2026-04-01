package strategy_test

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/factors"
	"github.com/arkcode369/ark-intelligent/internal/service/strategy"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeRankedAsset builds a RankedAsset with the given currency, composite score and signal.
func makeRankedAsset(contractCode, currency string, compositeScore float64, signal factors.Signal) factors.RankedAsset {
	return factors.RankedAsset{
		ContractCode:   contractCode,
		Currency:       currency,
		Name:           currency,
		CompositeScore: compositeScore,
		Signal:         signal,
		UpdatedAt:      time.Now(),
	}
}

// makeRanking wraps assets into a RankingResult.
func makeRanking(assets ...factors.RankedAsset) *factors.RankingResult {
	for i := range assets {
		assets[i].Rank = i + 1
	}
	return &factors.RankingResult{
		Assets:     assets,
		AssetCount: len(assets),
		ComputedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// TestEngine_NilRanking — empty input should not panic
// ---------------------------------------------------------------------------

func TestEngine_NilRanking(t *testing.T) {
	eng := strategy.NewEngine()
	result := eng.Generate(strategy.Input{Ranking: nil})
	if result == nil {
		t.Fatal("Generate() returned nil for nil ranking")
	}
	if len(result.Playbook) != 0 {
		t.Errorf("expected empty playbook for nil ranking, got %d entries", len(result.Playbook))
	}
}

func TestEngine_EmptyAssets(t *testing.T) {
	eng := strategy.NewEngine()
	result := eng.Generate(strategy.Input{
		Ranking: &factors.RankingResult{Assets: nil},
	})
	if result == nil {
		t.Fatal("Generate() returned nil for empty ranking")
	}
	if len(result.Playbook) != 0 {
		t.Errorf("expected empty playbook, got %d entries", len(result.Playbook))
	}
}

// ---------------------------------------------------------------------------
// TestEngine_AllFlatSignals — no LONG/SHORT signals → empty playbook
// ---------------------------------------------------------------------------

func TestEngine_AllFlatSignals(t *testing.T) {
	eng := strategy.NewEngine()
	ranking := makeRanking(
		makeRankedAsset("099741", "EUR", 0.10, factors.SignalNeutral),
		makeRankedAsset("096742", "GBP", -0.05, factors.SignalNeutral),
	)
	result := eng.Generate(strategy.Input{Ranking: ranking})
	if len(result.Playbook) != 0 {
		t.Errorf("expected 0 entries for neutral-only signals, got %d", len(result.Playbook))
	}
}

// ---------------------------------------------------------------------------
// TestEngine_AllBullishSignals — strong bullish → LONG playbook entries
// ---------------------------------------------------------------------------

func TestEngine_AllBullishSignals(t *testing.T) {
	eng := strategy.NewEngine()
	ranking := makeRanking(
		makeRankedAsset("099741", "EUR", 0.80, factors.SignalStrongLong),
		makeRankedAsset("096742", "GBP", 0.65, factors.SignalLong),
	)
	result := eng.Generate(strategy.Input{
		Ranking: ranking,
		COTBias: map[string]string{
			"099741": "BULLISH",
			"096742": "BULLISH",
		},
	})

	if len(result.Playbook) == 0 {
		t.Fatal("expected playbook entries for bullish signals, got none")
	}
	for _, entry := range result.Playbook {
		if entry.Direction != strategy.DirectionLong {
			t.Errorf("expected LONG direction for EUR/GBP bullish, got %s for %s", entry.Direction, entry.Currency)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEngine_AllBearishSignals — strong bearish → SHORT playbook entries
// ---------------------------------------------------------------------------

func TestEngine_AllBearishSignals(t *testing.T) {
	eng := strategy.NewEngine()
	ranking := makeRanking(
		makeRankedAsset("099741", "EUR", -0.80, factors.SignalStrongShort),
		makeRankedAsset("096742", "GBP", -0.65, factors.SignalShort),
	)
	result := eng.Generate(strategy.Input{
		Ranking: ranking,
		COTBias: map[string]string{
			"099741": "BEARISH",
			"096742": "BEARISH",
		},
	})

	if len(result.Playbook) == 0 {
		t.Fatal("expected playbook entries for bearish signals, got none")
	}
	for _, entry := range result.Playbook {
		if entry.Direction != strategy.DirectionShort {
			t.Errorf("expected SHORT direction for EUR/GBP bearish, got %s for %s", entry.Direction, entry.Currency)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEngine_COTDivergenceReducesConviction
// ---------------------------------------------------------------------------

func TestEngine_COTDivergenceReducesConviction(t *testing.T) {
	eng := strategy.NewEngine()

	// Same asset, same factor signal — compare conviction with aligned vs divergent COT
	assetBase := makeRankedAsset("099741", "EUR", 0.60, factors.SignalLong)

	resultAligned := eng.Generate(strategy.Input{
		Ranking: makeRanking(assetBase),
		COTBias: map[string]string{"099741": "BULLISH"}, // aligned
	})
	resultDivergent := eng.Generate(strategy.Input{
		Ranking: makeRanking(assetBase),
		COTBias: map[string]string{"099741": "BEARISH"}, // divergent
	})

	if len(resultAligned.Playbook) == 0 || len(resultDivergent.Playbook) == 0 {
		t.Skip("no playbook entries generated — skip conviction comparison")
	}

	alignedConv := resultAligned.Playbook[0].Conviction
	divergentConv := resultDivergent.Playbook[0].Conviction

	if alignedConv <= divergentConv {
		t.Errorf("aligned COT conviction (%f) should be > divergent COT conviction (%f)",
			alignedConv, divergentConv)
	}
}

// ---------------------------------------------------------------------------
// TestEngine_RegimeFitBoostsConviction — EXPANSION + risk-on long → higher conviction
// ---------------------------------------------------------------------------

func TestEngine_RegimeFitBoostsConviction(t *testing.T) {
	eng := strategy.NewEngine()

	asset := makeRankedAsset("13874P", "AUD", 0.50, factors.SignalLong)

	resultAligned := eng.Generate(strategy.Input{
		Ranking:     makeRanking(asset),
		MacroRegime: "EXPANSION", // AUD long is aligned in expansion
	})
	resultNeutral := eng.Generate(strategy.Input{
		Ranking:     makeRanking(asset),
		MacroRegime: "", // no regime context
	})

	if len(resultAligned.Playbook) == 0 || len(resultNeutral.Playbook) == 0 {
		t.Skip("no playbook entries — skip regime fit test")
	}

	if resultAligned.Playbook[0].Conviction < resultNeutral.Playbook[0].Conviction {
		t.Errorf("regime-aligned conviction (%f) should be ≥ neutral conviction (%f)",
			resultAligned.Playbook[0].Conviction, resultNeutral.Playbook[0].Conviction)
	}
}

// ---------------------------------------------------------------------------
// TestEngine_AgainstRegimePenalizesConviction
// ---------------------------------------------------------------------------

func TestEngine_AgainstRegimePenalizesConviction(t *testing.T) {
	eng := strategy.NewEngine()

	// JPY long in EXPANSION is against the regime (JPY is safe-haven)
	asset := makeRankedAsset("097741", "JPY", 0.50, factors.SignalLong)

	resultAgainst := eng.Generate(strategy.Input{
		Ranking:     makeRanking(asset),
		MacroRegime: "EXPANSION",
	})
	resultNeutral := eng.Generate(strategy.Input{
		Ranking:     makeRanking(asset),
		MacroRegime: "",
	})

	if len(resultAgainst.Playbook) == 0 || len(resultNeutral.Playbook) == 0 {
		t.Skip("no entries — skip against-regime test")
	}

	if resultAgainst.Playbook[0].Conviction >= resultNeutral.Playbook[0].Conviction {
		t.Errorf("against-regime conviction (%f) should be < neutral conviction (%f)",
			resultAgainst.Playbook[0].Conviction, resultNeutral.Playbook[0].Conviction)
	}
}

// ---------------------------------------------------------------------------
// TestEngine_VolExpandingReducesConviction
// ---------------------------------------------------------------------------

func TestEngine_VolExpandingReducesConviction(t *testing.T) {
	eng := strategy.NewEngine()

	asset := makeRankedAsset("099741", "EUR", 0.60, factors.SignalLong)

	resultNormal := eng.Generate(strategy.Input{
		Ranking:   makeRanking(asset),
		VolRegime: map[string]string{"099741": "NORMAL"},
	})
	resultExpanding := eng.Generate(strategy.Input{
		Ranking:   makeRanking(asset),
		VolRegime: map[string]string{"099741": "EXPANDING"},
	})

	if len(resultNormal.Playbook) == 0 || len(resultExpanding.Playbook) == 0 {
		t.Skip("no entries — skip vol regime test")
	}

	if resultExpanding.Playbook[0].Conviction >= resultNormal.Playbook[0].Conviction {
		t.Errorf("expanding vol conviction (%f) should be < normal vol conviction (%f)",
			resultExpanding.Playbook[0].Conviction, resultNormal.Playbook[0].Conviction)
	}
}

// ---------------------------------------------------------------------------
// TestEngine_TransitionReducesConviction
// ---------------------------------------------------------------------------

func TestEngine_TransitionReducesConviction(t *testing.T) {
	eng := strategy.NewEngine()

	asset := makeRankedAsset("099741", "EUR", 0.60, factors.SignalLong)

	resultNoTransition := eng.Generate(strategy.Input{
		Ranking:        makeRanking(asset),
		TransitionProb: 0.10, // low transition probability
	})
	resultTransition := eng.Generate(strategy.Input{
		Ranking:        makeRanking(asset),
		TransitionProb: 0.80, // high — regime transition active
		TransitionFrom: "EXPANSION",
		TransitionTo:   "SLOWDOWN",
	})

	if len(resultNoTransition.Playbook) == 0 || len(resultTransition.Playbook) == 0 {
		t.Skip("no entries — skip transition test")
	}

	if resultTransition.Playbook[0].Conviction >= resultNoTransition.Playbook[0].Conviction {
		t.Errorf("transition conviction (%f) should be < no-transition conviction (%f)",
			resultTransition.Playbook[0].Conviction, resultNoTransition.Playbook[0].Conviction)
	}
}

// ---------------------------------------------------------------------------
// TestEngine_ConvictionBoundedAtOne
// ---------------------------------------------------------------------------

func TestEngine_ConvictionBoundedAtOne(t *testing.T) {
	eng := strategy.NewEngine()
	ranking := makeRanking(
		makeRankedAsset("099741", "AUD", 1.0, factors.SignalStrongLong),
	)
	result := eng.Generate(strategy.Input{
		Ranking:     ranking,
		MacroRegime: "EXPANSION", // aligned → boost
		COTBias:     map[string]string{"099741": "BULLISH"}, // confirmed → further boost
	})

	for _, entry := range result.Playbook {
		if entry.Conviction > 1.0 {
			t.Errorf("conviction %f exceeds 1.0 for %s", entry.Conviction, entry.Currency)
		}
		if entry.Conviction < 0 {
			t.Errorf("conviction %f is negative for %s", entry.Conviction, entry.Currency)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEngine_PlaybookSortedByConvictionDesc
// ---------------------------------------------------------------------------

func TestEngine_PlaybookSortedByConvictionDesc(t *testing.T) {
	eng := strategy.NewEngine()
	ranking := makeRanking(
		makeRankedAsset("099741", "EUR", 0.30, factors.SignalLong),
		makeRankedAsset("096742", "GBP", 0.80, factors.SignalStrongLong),
		makeRankedAsset("13874P", "AUD", 0.55, factors.SignalLong),
	)
	result := eng.Generate(strategy.Input{Ranking: ranking})

	for i := 1; i < len(result.Playbook); i++ {
		if result.Playbook[i].Conviction > result.Playbook[i-1].Conviction {
			t.Errorf("playbook not sorted by conviction desc: entry[%d]=%f > entry[%d]=%f",
				i, result.Playbook[i].Conviction,
				i-1, result.Playbook[i-1].Conviction)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEngine_PortfolioHeat_EmptyPlaybook
// ---------------------------------------------------------------------------

func TestEngine_PortfolioHeat_EmptyPlaybook(t *testing.T) {
	eng := strategy.NewEngine()
	result := eng.Generate(strategy.Input{Ranking: &factors.RankingResult{}})

	heat := result.Heat
	if heat.ActiveTrades != 0 {
		t.Errorf("expected 0 active trades for empty playbook, got %d", heat.ActiveTrades)
	}
}

// ---------------------------------------------------------------------------
// TestEngine_TopLong_TopShort helpers
// ---------------------------------------------------------------------------

func TestPlaybookResult_TopLong_TopShort(t *testing.T) {
	eng := strategy.NewEngine()
	ranking := makeRanking(
		makeRankedAsset("099741", "EUR", 0.70, factors.SignalStrongLong),
		makeRankedAsset("096742", "GBP", -0.70, factors.SignalStrongShort),
		makeRankedAsset("13874P", "AUD", 0.55, factors.SignalLong),
	)
	result := eng.Generate(strategy.Input{Ranking: ranking})

	longs := result.TopLong(5)
	for _, l := range longs {
		if l.Direction != strategy.DirectionLong {
			t.Errorf("TopLong() returned non-LONG direction: %s", l.Direction)
		}
	}

	shorts := result.TopShort(5)
	for _, s := range shorts {
		if s.Direction != strategy.DirectionShort {
			t.Errorf("TopShort() returned non-SHORT direction: %s", s.Direction)
		}
	}
}

// ---------------------------------------------------------------------------
// TestEngine_MacroRegimePassedThrough
// ---------------------------------------------------------------------------

func TestEngine_MacroRegimePassedThrough(t *testing.T) {
	eng := strategy.NewEngine()
	ranking := makeRanking(makeRankedAsset("099741", "EUR", 0.60, factors.SignalLong))
	result := eng.Generate(strategy.Input{
		Ranking:     ranking,
		MacroRegime: "RECESSION",
	})
	if result.MacroRegime != "RECESSION" {
		t.Errorf("expected MacroRegime='RECESSION', got '%s'", result.MacroRegime)
	}
}
