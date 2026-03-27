package factors

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// flatPrices returns a series of n identical prices.
func flatPrices(p float64, n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = p
	}
	return s
}

// trendingPrices returns n prices linearly from high (index 0, newest) down to low (oldest).
func trendingPrices(newest, oldest float64, n int) []float64 {
	s := make([]float64, n)
	for i := 0; i < n; i++ {
		frac := float64(i) / float64(n-1)
		s[i] = newest - frac*(newest-oldest)
	}
	return s
}

// ---------------------------------------------------------------------------
// zscore — validates the actual implementation behaviour
// ---------------------------------------------------------------------------

func TestZscore_AllEqual(t *testing.T) {
	in := []float64{5, 5, 5, 5}
	z := zscore(in)
	for i, v := range z {
		if v != 0 {
			t.Errorf("z[%d] = %f, want 0 (all-equal input)", i, v)
		}
	}
}

// The actual zscore clamps at ±2σ then divides by 2 → range [-1,+1].
// For input {1,2,3,4,5}: mean=3, std=√2≈1.414
// z-values before clamp: -2/√2, -1/√2, 0, +1/√2, +2/√2
// after clamp & /2: -√2/2, -1/(2√2), 0, +1/(2√2), +√2/2
func TestZscore_KnownValues(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5}
	z := zscore(in)

	// Centre value must be 0
	if math.Abs(z[2]) > 1e-9 {
		t.Errorf("centre z = %.6f, want 0", z[2])
	}
	// Symmetry: z[0]==-z[4], z[1]==-z[3]
	if math.Abs(z[0]+z[4]) > 1e-9 {
		t.Errorf("z[0]+z[4] = %.6f, want 0 (symmetry)", z[0]+z[4])
	}
	if math.Abs(z[1]+z[3]) > 1e-9 {
		t.Errorf("z[1]+z[3] = %.6f, want 0 (symmetry)", z[1]+z[3])
	}
	// All values must be in [-1,+1]
	for i, v := range z {
		if v < -1.001 || v > 1.001 {
			t.Errorf("z[%d] = %.4f out of [-1,+1]", i, v)
		}
	}
}

func TestZscore_ClampAt2Sigma(t *testing.T) {
	// With an extreme outlier the output should be exactly ±1.
	in := []float64{0, 0, 0, 0, 1000}
	z := zscore(in)
	for _, v := range z {
		if v > 1.001 || v < -1.001 {
			t.Errorf("zscore out of [-1,+1] after clamp: %.4f", v)
		}
	}
}

// ---------------------------------------------------------------------------
// Momentum
// ---------------------------------------------------------------------------

func TestScoreMomentum_Uptrend(t *testing.T) {
	// newest=200, oldest=100 → positive 1M/3M returns → positive momentum
	closes := trendingPrices(200, 100, 300)
	score := scoreMomentum(closes)
	if score <= 0 {
		t.Errorf("expected positive momentum for uptrend, got %.4f", score)
	}
}

func TestScoreMomentum_Downtrend(t *testing.T) {
	// newest=100, oldest=200 → negative returns → negative momentum
	closes := trendingPrices(100, 200, 300)
	score := scoreMomentum(closes)
	if score >= 0 {
		t.Errorf("expected negative momentum for downtrend, got %.4f", score)
	}
}

func TestScoreMomentum_Flat(t *testing.T) {
	closes := flatPrices(100, 300)
	score := scoreMomentum(closes)
	if math.Abs(score) > 0.001 {
		t.Errorf("expected ~0 momentum for flat prices, got %.4f", score)
	}
}

func TestScoreMomentum_TooFewBars(t *testing.T) {
	closes := flatPrices(100, 10) // < 22 bars
	score := scoreMomentum(closes)
	if score != 0 {
		t.Errorf("expected 0 for insufficient data, got %.4f", score)
	}
}

// ---------------------------------------------------------------------------
// TrendQuality
// ---------------------------------------------------------------------------

func TestScoreTrendQuality_CleanTrend(t *testing.T) {
	closes := trendingPrices(200, 100, 100)
	score := scoreTrendQuality(closes)
	// A clean uptrend should produce a non-zero score (direction is engine-agnostic;
	// what matters is that R² is high → non-trivial score)
	if math.Abs(score) < 0.001 {
		t.Errorf("expected non-zero trend quality for clean uptrend, got %.4f", score)
	}
}

func TestScoreTrendQuality_InsufficientData(t *testing.T) {
	closes := flatPrices(100, 5)
	score := scoreTrendQuality(closes)
	if score != 0 {
		t.Errorf("expected 0 for insufficient data, got %.4f", score)
	}
}

// ---------------------------------------------------------------------------
// LowVol (Sharpe proxy: annualised return / annualised vol)
// ---------------------------------------------------------------------------

func TestScoreLowVol_Uptrend_HighSharpe(t *testing.T) {
	// Strong, smooth uptrend → high Sharpe proxy → positive score
	closes := trendingPrices(200, 100, 300)
	score := scoreLowVol(closes, 63)
	if score <= 0 {
		t.Errorf("expected positive low-vol score for uptrending prices, got %.4f", score)
	}
}

func TestScoreLowVol_Downtrend_NegativeSharpe(t *testing.T) {
	// Strong, smooth downtrend → negative Sharpe → negative score
	closes := trendingPrices(100, 200, 300)
	score := scoreLowVol(closes, 63)
	if score >= 0 {
		t.Errorf("expected negative low-vol score for downtrending prices, got %.4f", score)
	}
}

func TestScoreLowVol_TooFewBars(t *testing.T) {
	closes := flatPrices(100, 10)
	score := scoreLowVol(closes, 63)
	if score != 0 {
		t.Errorf("expected 0 for insufficient data, got %.4f", score)
	}
}

// ---------------------------------------------------------------------------
// Engine.Rank
// ---------------------------------------------------------------------------

func TestEngine_Rank_BasicSort(t *testing.T) {
	up := AssetProfile{
		ContractCode: "001",
		Currency:     "UP",
		Name:         "Uptrend",
		DailyCloses:  trendingPrices(200, 100, 300),
	}
	flat := AssetProfile{
		ContractCode: "002",
		Currency:     "FLAT",
		Name:         "Flat",
		DailyCloses:  flatPrices(100, 300),
	}
	down := AssetProfile{
		ContractCode: "003",
		Currency:     "DOWN",
		Name:         "Downtrend",
		DailyCloses:  trendingPrices(100, 200, 300), // newest<oldest → downtrend
	}

	eng := NewEngine(DefaultWeights())
	result := eng.Rank([]AssetProfile{flat, down, up})

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Assets) != 3 {
		t.Fatalf("expected 3 assets, got %d", len(result.Assets))
	}
	if result.Assets[0].Rank != 1 {
		t.Errorf("first asset should have Rank=1, got %d", result.Assets[0].Rank)
	}

	// Scores must be monotonically non-increasing (sorted best-first)
	for i := 1; i < len(result.Assets); i++ {
		if result.Assets[i].CompositeScore > result.Assets[i-1].CompositeScore+1e-9 {
			t.Errorf("assets not sorted: [%d]=%.4f > [%d]=%.4f",
				i, result.Assets[i].CompositeScore,
				i-1, result.Assets[i-1].CompositeScore)
		}
	}

	// Uptrend should rank first, downtrend last
	if result.Assets[0].Currency != "UP" {
		t.Errorf("expected UP to rank first, got %s", result.Assets[0].Currency)
	}
	if result.Assets[2].Currency != "DOWN" {
		t.Errorf("expected DOWN to rank last, got %s", result.Assets[2].Currency)
	}
}

func TestEngine_Rank_EmptyInput(t *testing.T) {
	eng := NewEngine(DefaultWeights())
	result := eng.Rank(nil)
	if result == nil {
		t.Fatal("expected non-nil result for empty input")
	}
	if len(result.Assets) != 0 {
		t.Errorf("expected 0 assets, got %d", len(result.Assets))
	}
}

func TestEngine_Rank_SingleAsset(t *testing.T) {
	eng := NewEngine(DefaultWeights())
	result := eng.Rank([]AssetProfile{{
		ContractCode: "X",
		Currency:     "X",
		DailyCloses:  flatPrices(100, 200),
	}})
	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(result.Assets))
	}
	if result.Assets[0].Rank != 1 {
		t.Error("single asset should have Rank=1")
	}
}

func TestEngine_Rank_AllCompositesClamped(t *testing.T) {
	profiles := make([]AssetProfile, 5)
	for i := range profiles {
		profiles[i] = AssetProfile{
			ContractCode: string(rune('A' + i)),
			Currency:     string(rune('A' + i)),
			DailyCloses:  trendingPrices(float64(100+i*10), float64(100+i*5), 300),
		}
	}
	eng := NewEngine(DefaultWeights())
	result := eng.Rank(profiles)
	for _, a := range result.Assets {
		if a.CompositeScore > 1.001 || a.CompositeScore < -1.001 {
			t.Errorf("composite score %.4f out of [-1,+1] for %s", a.CompositeScore, a.Currency)
		}
	}
}

// ---------------------------------------------------------------------------
// Signal mapping
// ---------------------------------------------------------------------------

func TestCompositeToSignal(t *testing.T) {
	cases := []struct {
		score float64
		want  Signal
	}{
		{0.80, SignalStrongLong},
		{0.30, SignalLong},
		{0.10, SignalNeutral},
		{-0.30, SignalShort},
		{-0.70, SignalStrongShort},
	}
	for _, tc := range cases {
		got := CompositeToSignal(tc.score)
		if got != tc.want {
			t.Errorf("CompositeToSignal(%.2f) = %s, want %s", tc.score, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Default weights
// ---------------------------------------------------------------------------

func TestDefaultWeights_SumToOne(t *testing.T) {
	w := DefaultWeights()
	sum := w.Momentum + w.TrendQuality + w.CarryAdjusted + w.LowVol + w.ResidualReversal + w.Crowding
	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("DefaultWeights sum = %.4f, want 1.0", sum)
	}
}
