package price

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// --- GARCH Tests ---

func TestEstimateGARCH_InsufficientData(t *testing.T) {
	prices := make([]domain.PriceRecord, 10)
	for i := range prices {
		prices[i] = domain.PriceRecord{Close: 100 + float64(i)}
	}
	_, err := EstimateGARCH(prices)
	if err == nil {
		t.Fatal("expected error for insufficient data")
	}
}

func TestEstimateGARCH_SyntheticData(t *testing.T) {
	// Generate 100 daily prices with known volatility clustering
	rng := rand.New(rand.NewSource(42))
	prices := make([]domain.PriceRecord, 100)
	price := 100.0
	now := time.Now()

	// Generate oldest-first then reverse to newest-first
	for i := 99; i >= 0; i-- {
		prices[i] = domain.PriceRecord{
			Date:  now.AddDate(0, 0, -(99 - i)),
			Close: price,
			High:  price * 1.01,
			Low:   price * 0.99,
			Open:  price,
		}
		// Simulate vol clustering: larger moves followed by larger moves
		ret := rng.NormFloat64() * 0.01
		if math.Abs(ret) > 0.01 {
			ret *= 1.5 // amplify large moves
		}
		price *= (1 + ret)
	}

	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Basic sanity checks
	if result.Alpha < 0 || result.Alpha > 0.5 {
		t.Errorf("alpha out of range: %f", result.Alpha)
	}
	if result.Beta < 0 || result.Beta > 1 {
		t.Errorf("beta out of range: %f", result.Beta)
	}
	if result.Persistence >= 1 {
		t.Errorf("persistence should be < 1: %f", result.Persistence)
	}
	if result.CurrentVol <= 0 {
		t.Errorf("current vol should be > 0: %f", result.CurrentVol)
	}
	if result.ForecastVol1 <= 0 {
		t.Errorf("forecast vol should be > 0: %f", result.ForecastVol1)
	}
	if result.LongRunVol <= 0 {
		t.Errorf("long-run vol should be > 0: %f", result.LongRunVol)
	}
	if result.SampleSize != 99 {
		t.Errorf("expected 99 returns, got %d", result.SampleSize)
	}
	if result.VolForecast == "" {
		t.Error("vol forecast should be set")
	}
}

func TestGARCHConfidenceMultiplier(t *testing.T) {
	tests := []struct {
		name     string
		garch    *GARCHResult
		expected float64
	}{
		{"nil result", nil, 1.0},
		{"not converged", &GARCHResult{Converged: false}, 1.0},
		{"high vol", &GARCHResult{Converged: true, VolRatio: 1.6, ForecastVol1: 0.02, LongRunVol: 0.01}, 0.75},
		{"elevated vol", &GARCHResult{Converged: true, VolRatio: 1.3, ForecastVol1: 0.015, LongRunVol: 0.01}, 0.85},
		{"normal vol", &GARCHResult{Converged: true, VolRatio: 1.0, ForecastVol1: 0.01, LongRunVol: 0.01}, 1.0},
		{"low vol", &GARCHResult{Converged: true, VolRatio: 0.7, ForecastVol1: 0.007, LongRunVol: 0.01}, 1.10},
		{"very low vol", &GARCHResult{Converged: true, VolRatio: 0.4, ForecastVol1: 0.004, LongRunVol: 0.01}, 1.15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GARCHConfidenceMultiplier(tt.garch)
			if got != tt.expected {
				t.Errorf("expected %.2f, got %.2f", tt.expected, got)
			}
		})
	}
}

// --- Hurst Tests ---

func TestComputeHurstExponent_InsufficientData(t *testing.T) {
	prices := make([]domain.PriceRecord, 20)
	for i := range prices {
		prices[i] = domain.PriceRecord{Close: 100 + float64(i)}
	}
	_, err := ComputeHurstExponent(prices)
	if err == nil {
		t.Fatal("expected error for insufficient data")
	}
}

func TestComputeHurstExponent_TrendingData(t *testing.T) {
	// Generate a strong trend with noise — should produce H > 0.5
	rng := rand.New(rand.NewSource(77))
	prices := make([]domain.PriceRecord, 200)
	price := 100.0
	now := time.Now()

	for i := 199; i >= 0; i-- {
		prices[i] = domain.PriceRecord{
			Date:  now.AddDate(0, 0, -(199 - i)),
			Close: price,
			High:  price * 1.005,
			Low:   price * 0.995,
			Open:  price,
		}
		// Consistent upward drift + noise
		price *= (1 + 0.003 + rng.NormFloat64()*0.005)
	}

	result, err := ComputeHurstExponent(prices)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.H <= 0 || result.H >= 1 {
		t.Errorf("H out of range: %f", result.H)
	}
	if result.Classification == "" {
		t.Error("classification should be set")
	}
	if result.SampleSize != 199 {
		t.Errorf("expected 199 returns, got %d", result.SampleSize)
	}
	if result.RSquared < 0 || result.RSquared > 1 {
		t.Errorf("R² out of range: %f", result.RSquared)
	}
}

func TestComputeHurstExponent_RandomWalk(t *testing.T) {
	// Random walk should give H near 0.5
	rng := rand.New(rand.NewSource(123))
	prices := make([]domain.PriceRecord, 500)
	price := 100.0
	now := time.Now()

	for i := 499; i >= 0; i-- {
		prices[i] = domain.PriceRecord{
			Date:  now.AddDate(0, 0, -(499 - i)),
			Close: price,
			High:  price * 1.005,
			Low:   price * 0.995,
			Open:  price,
		}
		price *= (1 + rng.NormFloat64()*0.01)
	}

	result, err := ComputeHurstExponent(prices)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Random walk H should be roughly 0.4-0.65 (not exact 0.5 due to finite sample)
	if result.H < 0.3 || result.H > 0.7 {
		t.Errorf("random walk H expected near 0.5, got %f", result.H)
	}
}

func TestHurstToRegime(t *testing.T) {
	tests := []struct {
		classification string
		expected       string
	}{
		{"TRENDING", RegimeTrending},
		{"MEAN_REVERTING", RegimeRanging},
		{"RANDOM_WALK", RegimeRanging},
	}

	for _, tt := range tests {
		h := &HurstResult{Classification: tt.classification}
		got := HurstToRegime(h)
		if got != tt.expected {
			t.Errorf("%s: expected %s, got %s", tt.classification, tt.expected, got)
		}
	}
}

func TestCombineRegimeClassification_Agreement(t *testing.T) {
	adx := &PriceRegime{Regime: RegimeTrending, TrendStrength: 60}
	hurst := &HurstResult{H: 0.65, Classification: "TRENDING", Confidence: 30}

	result := CombineRegimeClassification(adx, hurst)
	if !result.RegimeAgreement {
		t.Error("should agree when both say TRENDING")
	}
	if result.CombinedConfidence <= 0 {
		t.Error("combined confidence should be positive")
	}
}

func TestCombineRegimeClassification_Disagreement(t *testing.T) {
	adx := &PriceRegime{Regime: RegimeTrending, TrendStrength: 60}
	hurst := &HurstResult{H: 0.35, Classification: "MEAN_REVERTING", Confidence: 30, RSquared: 0.85}

	result := CombineRegimeClassification(adx, hurst)
	if result.RegimeAgreement {
		t.Error("should disagree when ADX=TRENDING, Hurst=MEAN_REVERTING")
	}
}

func TestSimpleLinearRegression(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10} // y = 2x, perfect fit
	slope, intercept, r2 := simpleLinearRegression(x, y)
	if math.Abs(slope-2.0) > 0.001 {
		t.Errorf("expected slope 2.0, got %f", slope)
	}
	if math.Abs(intercept) > 0.001 {
		t.Errorf("expected intercept 0, got %f", intercept)
	}
	if math.Abs(r2-1.0) > 0.001 {
		t.Errorf("expected R²=1.0, got %f", r2)
	}
}
