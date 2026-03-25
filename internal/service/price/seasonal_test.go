package price

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Phase 1 Tests: Statistical Foundation
// ---------------------------------------------------------------------------

func TestClassifyBias_MinSampleSize(t *testing.T) {
	tests := []struct {
		name       string
		avgReturn  float64
		winRate    float64
		sampleSize int
		want       string
	}{
		{"sample=0 returns NEUTRAL", 5.0, 80.0, 0, "NEUTRAL"},
		{"sample=1 returns NEUTRAL despite strong stats", 5.0, 100.0, 1, "NEUTRAL"},
		{"sample=2 returns NEUTRAL despite strong stats", 5.0, 100.0, 2, "NEUTRAL"},
		{"sample=3 bullish", 1.0, 66.7, 3, "BULLISH"},
		{"sample=3 bearish", -1.0, 33.3, 3, "BEARISH"},
		{"sample=3 neutral (WR in dead zone)", 0.5, 50.0, 3, "NEUTRAL"},
		{"sample=5 bullish", 2.0, 60.0, 5, "BULLISH"},
		{"sample=5 bearish", -2.0, 40.0, 5, "BEARISH"},
		{"positive avg but low WR = NEUTRAL", 0.5, 50.0, 5, "NEUTRAL"},
		{"negative avg but high WR = NEUTRAL", -0.5, 50.0, 5, "NEUTRAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyBias(tt.avgReturn, tt.winRate, tt.sampleSize)
			if got != tt.want {
				t.Errorf("classifyBias(%.1f, %.1f, %d) = %q, want %q",
					tt.avgReturn, tt.winRate, tt.sampleSize, got, tt.want)
			}
		})
	}
}

func TestClassifyConfidence(t *testing.T) {
	tests := []struct {
		name       string
		avgReturn  float64
		winRate    float64
		stdDev     float64
		sampleSize int
		want       ConfidenceTier
	}{
		{"insufficient data", 5.0, 80.0, 1.0, 2, ConfidenceNone},
		{"strong: high WR + low noise + n>=4", 3.0, 75.0, 2.0, 4, ConfidenceStrong},
		{"strong: very low WR (bearish)", -3.0, 25.0, 2.0, 4, ConfidenceStrong},
		{"moderate: meets threshold", 1.0, 60.0, 3.0, 3, ConfidenceModerate},
		{"moderate: bearish threshold", -1.0, 40.0, 3.0, 3, ConfidenceModerate},
		{"weak: directional but noisy", 0.5, 52.0, 5.0, 3, ConfidenceWeak},
		{"none: no direction", 0.0, 50.0, 5.0, 3, ConfidenceNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyConfidence(tt.avgReturn, tt.winRate, tt.stdDev, tt.sampleSize)
			if got != tt.want {
				t.Errorf("classifyConfidence(%.1f, %.1f, %.1f, %d) = %q, want %q",
					tt.avgReturn, tt.winRate, tt.stdDev, tt.sampleSize, got, tt.want)
			}
		})
	}
}

func TestComputeRecencyWeighted(t *testing.T) {
	// 3 years of data: most recent year weighted highest
	yearRets := []YearReturn{
		{Year: 2025, Return: 3.0},  // weight 1.0
		{Year: 2024, Return: 2.0},  // weight 0.8
		{Year: 2023, Return: -1.0}, // weight 0.64
	}

	avgW, wrW := computeRecencyWeighted(yearRets, 2025)

	// Weighted avg = (3*1 + 2*0.8 + (-1)*0.64) / (1+0.8+0.64) = (3+1.6-0.64)/2.44 = 3.96/2.44 ≈ 1.623
	if avgW < 1.5 || avgW > 1.7 {
		t.Errorf("weighted avg = %.3f, want ~1.623", avgW)
	}

	// Weighted WR: 2025 win (w=1), 2024 win (w=0.8), 2023 loss (w=0)
	// = (1+0.8)/2.44*100 ≈ 73.8%
	if wrW < 70 || wrW > 78 {
		t.Errorf("weighted WR = %.1f%%, want ~73.8%%", wrW)
	}
}

func TestComputeRecencyWeighted_Empty(t *testing.T) {
	avg, wr := computeRecencyWeighted(nil, 2025)
	if avg != 0 || wr != 0 {
		t.Errorf("expected 0,0 for empty input, got %.2f, %.2f", avg, wr)
	}
}

func TestComputeMonthStats_Median(t *testing.T) {
	// Odd number of returns
	rets := []float64{-2.0, 1.0, 3.0, 5.0, 10.0}
	yrs := []YearReturn{
		{2025, -2.0}, {2024, 1.0}, {2023, 3.0}, {2022, 5.0}, {2021, 10.0},
	}
	ms := computeMonthStats(0, rets, yrs, 2025) // January

	// Median of sorted [-2, 1, 3, 5, 10] = 3.0
	if ms.MedianRet != 3.0 {
		t.Errorf("median = %.2f, want 3.0", ms.MedianRet)
	}

	// Even number of returns
	rets2 := []float64{1.0, 3.0, 5.0, 7.0}
	yrs2 := []YearReturn{{2025, 1.0}, {2024, 3.0}, {2023, 5.0}, {2022, 7.0}}
	ms2 := computeMonthStats(0, rets2, yrs2, 2025)

	// Median of sorted [1, 3, 5, 7] = (3+5)/2 = 4.0
	if ms2.MedianRet != 4.0 {
		t.Errorf("median = %.2f, want 4.0", ms2.MedianRet)
	}
}

func TestComputeMonthStats_StdDev(t *testing.T) {
	// All same returns → StdDev = 0
	rets := []float64{2.0, 2.0, 2.0}
	yrs := []YearReturn{{2025, 2.0}, {2024, 2.0}, {2023, 2.0}}
	ms := computeMonthStats(0, rets, yrs, 2025)

	if ms.StdDev != 0 {
		t.Errorf("stddev = %.4f, want 0 for identical values", ms.StdDev)
	}

	// Known stddev: [1, 3, 5] → mean=3, var=((1-3)^2+(3-3)^2+(5-3)^2)/2 = 8/2 = 4, std=2
	rets2 := []float64{1.0, 3.0, 5.0}
	yrs2 := []YearReturn{{2025, 1.0}, {2024, 3.0}, {2023, 5.0}}
	ms2 := computeMonthStats(0, rets2, yrs2, 2025)

	if ms2.StdDev < 1.99 || ms2.StdDev > 2.01 {
		t.Errorf("stddev = %.4f, want 2.0", ms2.StdDev)
	}
}

func TestComputeMonthStats_Empty(t *testing.T) {
	ms := computeMonthStats(5, nil, nil, 2025)
	if ms.Bias != "NEUTRAL" {
		t.Errorf("bias = %q, want NEUTRAL for empty data", ms.Bias)
	}
	if ms.Confidence != ConfidenceNone {
		t.Errorf("confidence = %q, want NONE for empty data", ms.Confidence)
	}
	if ms.Month != "Jun" {
		t.Errorf("month = %q, want Jun for index 5", ms.Month)
	}
}

// ---------------------------------------------------------------------------
// Phase 2 Tests: Regime Matching
// ---------------------------------------------------------------------------

func TestFindRegimeAtDate(t *testing.T) {
	regimes := map[string]string{
		"2024-03-11": "DISINFLATIONARY",
		"2024-06-10": "GOLDILOCKS",
		"2025-01-06": "INFLATIONARY",
	}

	tests := []struct {
		name string
		year int
		month int
		day  int
		want string
	}{
		{"exact match", 2024, 3, 11, "DISINFLATIONARY"},
		{"close to March", 2024, 3, 15, "DISINFLATIONARY"},
		{"close to June", 2024, 6, 8, "GOLDILOCKS"},
		{"too far from any date", 2023, 1, 1, ""}, // >30 days from all
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := seasonalDate(tt.year, tt.month, tt.day)
			got := findRegimeAtDate(regimes, target)
			if got != tt.want {
				t.Errorf("findRegimeAtDate(%v) = %q, want %q", target, got, tt.want)
			}
		})
	}
}

func TestFindRegimeAtDate_Empty(t *testing.T) {
	got := findRegimeAtDate(nil, seasonalDate(2024, 3, 15))
	if got != "" {
		t.Errorf("expected empty for nil regimes, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Phase 3c Tests: Volatility Context
// ---------------------------------------------------------------------------

func TestComputeVolatilityContext_VIXRegimes(t *testing.T) {
	p := &SeasonalPattern{CurrentMonth: 1}
	// Give month 0 (Jan) some data
	p.Monthly[0] = MonthStats{StdDev: 2.0, SampleSize: 5}
	// Give other months data too for avg calc
	for i := 1; i < 12; i++ {
		p.Monthly[i] = MonthStats{StdDev: 2.0, SampleSize: 3}
	}

	driver := AssetDriver{VIXSensitivity: "HIGH"}

	// Test VIX regimes
	result := computeVolatilityContext(p, 0, 30.0, driver)
	if result.CurrentVIXRegime != "ELEVATED" {
		t.Errorf("VIX=30 → regime=%q, want ELEVATED", result.CurrentVIXRegime)
	}

	result = computeVolatilityContext(p, 0, 20.0, driver)
	if result.CurrentVIXRegime != "NORMAL" {
		t.Errorf("VIX=20 → regime=%q, want NORMAL", result.CurrentVIXRegime)
	}

	result = computeVolatilityContext(p, 0, 12.0, driver)
	if result.CurrentVIXRegime != "LOW" {
		t.Errorf("VIX=12 → regime=%q, want LOW", result.CurrentVIXRegime)
	}
}

// ---------------------------------------------------------------------------
// Phase 4 Tests: EIA Context
// ---------------------------------------------------------------------------

func TestComputeEIAContext_NonEnergy(t *testing.T) {
	eiaData := &EIASeasonalData{}
	result := ComputeEIAContext(eiaData, "EUR", 3)
	if result != nil {
		t.Error("expected nil EIA context for non-energy asset")
	}
}

func TestComputeEIAContext_Nil(t *testing.T) {
	result := ComputeEIAContext(nil, "OIL", 3)
	if result != nil {
		t.Error("expected nil for nil EIA data")
	}
}

// ---------------------------------------------------------------------------
// Phase 5 Tests: Confluence Scoring
// ---------------------------------------------------------------------------

func TestComputeConfluence_AllAligned(t *testing.T) {
	p := &SeasonalPattern{
		CurrentMonth: 3,
		RegimeStats: &RegimeMonthStats{
			DriverAlignment: "SUPPORTIVE",
			RegimeName:      "DISINFLATIONARY",
			PrimaryFREDDriver: "test",
			SampleSize:      3,
		},
		COTAlignment: &COTAlignmentResult{
			CurrentAligned: true,
			CurrentCOTBias: "BULLISH",
		},
		EventDensity: &EventDensityResult{
			Rating: "LOW",
		},
		CrossAsset: &CrossAssetResult{
			Assessment:     "CONSISTENT",
			Contradictions: 0,
		},
	}

	curStats := MonthStats{
		Bias:       "BULLISH",
		AvgReturn:  2.0,
		WinRate:    70.0,
		SampleSize: 5,
		Confidence: ConfidenceStrong,
	}

	result := computeConfluence(p, curStats)
	if result == nil {
		t.Fatal("expected non-nil confluence result")
	}
	if result.Score != 5 {
		t.Errorf("all aligned → score=%d, want 5", result.Score)
	}
	if result.Level != "HIGH" {
		t.Errorf("all aligned → level=%q, want HIGH", result.Level)
	}
}

func TestComputeConfluence_NoneAligned(t *testing.T) {
	p := &SeasonalPattern{
		CurrentMonth: 3,
		RegimeStats: &RegimeMonthStats{
			DriverAlignment: "HEADWIND",
			RegimeName:      "INFLATIONARY",
			PrimaryFREDDriver: "test",
		},
		COTAlignment: &COTAlignmentResult{
			CurrentAligned: false,
			CurrentCOTBias: "BEARISH",
		},
		EventDensity: &EventDensityResult{
			Rating: "HIGH",
			KeyEvents: "FOMC, NFP",
		},
		CrossAsset: &CrossAssetResult{
			Assessment:     "CONTRADICTORY",
			Contradictions: 2,
		},
	}

	curStats := MonthStats{
		Bias:       "NEUTRAL",
		SampleSize: 3,
		Confidence: ConfidenceNone,
	}

	result := computeConfluence(p, curStats)
	if result == nil {
		t.Fatal("expected non-nil confluence result")
	}
	if result.Score > 1 {
		t.Errorf("nothing aligned → score=%d, want <=1", result.Score)
	}
}

func TestComputeConfluence_NeutralBias(t *testing.T) {
	p := &SeasonalPattern{CurrentMonth: 6}
	curStats := MonthStats{
		Bias:       "NEUTRAL",
		SampleSize: 2, // below threshold
		Confidence: ConfidenceNone,
	}
	result := computeConfluence(p, curStats)
	if result != nil {
		t.Error("expected nil confluence for insufficient neutral data")
	}
}

// ---------------------------------------------------------------------------
// Asset Driver Tests
// ---------------------------------------------------------------------------

func TestGetAssetDriver_Known(t *testing.T) {
	known := []string{"EUR", "GBP", "JPY", "CHF", "AUD", "CAD", "NZD", "USD",
		"XAU", "XAG", "COPPER", "OIL", "ULSD", "RBOB",
		"BOND", "BOND30", "BOND5", "BOND2",
		"SPX500", "NDX", "DJI", "RUT", "BTC", "ETH"}

	for _, ccy := range known {
		d := GetAssetDriver(ccy)
		if d.AssetClass == "OTHER" {
			t.Errorf("GetAssetDriver(%q) returned OTHER, expected a known class", ccy)
		}
		if d.PrimaryFREDMetric == "unknown" {
			t.Errorf("GetAssetDriver(%q) has unknown FRED metric", ccy)
		}
	}
}

func TestGetAssetDriver_Unknown(t *testing.T) {
	d := GetAssetDriver("FAKE")
	if d.AssetClass != "OTHER" {
		t.Errorf("expected OTHER for unknown currency, got %q", d.AssetClass)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func seasonalDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
