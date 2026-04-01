package fred

import (
	"math"
	"testing"
)

func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

// --- TestMapRange ---

func TestMapRange(t *testing.T) {
	tests := []struct {
		name                         string
		value, inMin, inMax          float64
		outMin, outMax               float64
		want                         float64
	}{
		{"midpoint", 5, 0, 10, 0, 100, 50},
		{"at_min", 0, 0, 10, 0, 100, 0},
		{"at_max", 10, 0, 10, 0, 100, 100},
		{"below_min_clamped", -5, 0, 10, 0, 100, 0},
		{"above_max_clamped", 15, 0, 10, 0, 100, 100},
		{"inverted_input_range", 250_000, 350_000, 180_000, 0, 100, 58.823529},
		{"inverted_input_at_high", 350_000, 350_000, 180_000, 0, 100, 0},
		{"inverted_input_at_low", 180_000, 350_000, 180_000, 0, 100, 100},
		{"inverted_input_beyond_high", 400_000, 350_000, 180_000, 0, 100, 0},
		{"inverted_input_beyond_low", 100_000, 350_000, 180_000, 0, 100, 100},
		{"equal_inmin_inmax", 5, 5, 5, 0, 100, 50},
		{"negative_output", 2.5, 1.5, 3.5, -1.0, 1.0, 0},
		{"quarter", 2.5, 0, 10, 0, 100, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapRange(tt.value, tt.inMin, tt.inMax, tt.outMin, tt.outMax)
			if !approxEqual(got, tt.want, 0.001) {
				t.Errorf("mapRange(%v, %v, %v, %v, %v) = %v, want %v",
					tt.value, tt.inMin, tt.inMax, tt.outMin, tt.outMax, got, tt.want)
			}
		})
	}
}

// --- TestClamp ---

func TestClamp(t *testing.T) {
	tests := []struct {
		name       string
		v, min, max float64
		want       float64
	}{
		{"within_range", 50, 0, 100, 50},
		{"at_min", 0, 0, 100, 0},
		{"at_max", 100, 0, 100, 100},
		{"below_min", -10, 0, 100, 0},
		{"above_max", 150, 0, 100, 100},
		{"negative_range", -0.5, -1.0, 1.0, -0.5},
		{"below_negative_min", -2.0, -1.0, 1.0, -1.0},
		{"above_negative_max", 2.0, -1.0, 1.0, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp(tt.v, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("clamp(%v, %v, %v) = %v, want %v", tt.v, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

// --- TestComputeLaborHealth ---

func TestComputeLaborHealth(t *testing.T) {
	t.Run("healthy_labor_market", func(t *testing.T) {
		data := &MacroData{
			InitialClaims:    200_000,  // low claims
			ContinuingClaims: 1_600_000,
			UnemployRate:     3.5,      // low unemployment
			U6Unemployment:   7.0,
			JOLTSOpenings:    11_000,   // high openings
			JOLTSQuitRate:    2.8,      // high quit rate
			EmpPopRatio:      61.0,
		}
		got := computeLaborHealth(data)
		if got < 70 {
			t.Errorf("healthy labor market should score >= 70, got %v", got)
		}
		if got > 100 {
			t.Errorf("score should not exceed 100, got %v", got)
		}
	})

	t.Run("weakening_labor_market", func(t *testing.T) {
		data := &MacroData{
			InitialClaims:    340_000,  // high claims
			ContinuingClaims: 2_400_000,
			UnemployRate:     6.5,      // rising unemployment
			U6Unemployment:   11.0,
			JOLTSOpenings:    6_500,    // low openings
			JOLTSQuitRate:    1.6,      // low quit rate
			EmpPopRatio:      56.0,
		}
		got := computeLaborHealth(data)
		if got > 30 {
			t.Errorf("weakening labor market should score <= 30, got %v", got)
		}
		if got < 0 {
			t.Errorf("score should not go below 0, got %v", got)
		}
	})

	t.Run("sahm_rule_override_caps_at_20", func(t *testing.T) {
		data := &MacroData{
			InitialClaims: 200_000,  // otherwise healthy
			JOLTSOpenings: 11_000,
			UnemployRate:  3.5,
			EmpPopRatio:   61.0,
			SahmRule:      0.6, // triggered
		}
		got := computeLaborHealth(data)
		if got > 20 {
			t.Errorf("sahm rule triggered should cap score at 20, got %v", got)
		}
	})

	t.Run("sahm_rule_low_score_passes_through", func(t *testing.T) {
		data := &MacroData{
			InitialClaims: 340_000,  // weak data
			JOLTSOpenings: 6_200,
			UnemployRate:  6.8,
			SahmRule:      0.5, // triggered
		}
		got := computeLaborHealth(data)
		if got > 20 {
			t.Errorf("sahm rule with weak data should be <= 20, got %v", got)
		}
	})

	t.Run("sahm_rule_no_data", func(t *testing.T) {
		data := &MacroData{
			SahmRule: 0.7, // triggered but no indicators
		}
		got := computeLaborHealth(data)
		if got != 20 {
			t.Errorf("sahm rule with no data should return 20, got %v", got)
		}
	})

	t.Run("no_data_returns_neutral", func(t *testing.T) {
		data := &MacroData{}
		got := computeLaborHealth(data)
		if got != 50 {
			t.Errorf("no data should return neutral 50, got %v", got)
		}
	})
}

// --- TestComputeInflationMomentum ---

func TestComputeInflationMomentum(t *testing.T) {
	t.Run("high_inflation", func(t *testing.T) {
		data := &MacroData{
			CorePCE:       3.4,
			MedianCPI:     4.8,
			StickyCPI:     5.5,
			PPICommodities: 8.0,
			Breakeven5Y:   2.9,
			MichInflExp1Y: 4.5,
		}
		got := computeInflationMomentum(data)
		if got < 0.5 {
			t.Errorf("high inflation should produce momentum >= 0.5, got %v", got)
		}
		if got > 1.0 {
			t.Errorf("momentum should not exceed 1.0, got %v", got)
		}
	})

	t.Run("low_inflation", func(t *testing.T) {
		data := &MacroData{
			CorePCE:       1.6,
			MedianCPI:     2.1,
			StickyCPI:     2.1,
			PPICommodities: -4.0,
			Breakeven5Y:   1.6,
			MichInflExp1Y: 2.1,
		}
		got := computeInflationMomentum(data)
		if got > -0.5 {
			t.Errorf("low inflation should produce momentum <= -0.5, got %v", got)
		}
		if got < -1.0 {
			t.Errorf("momentum should not go below -1.0, got %v", got)
		}
	})

	t.Run("no_data_returns_zero", func(t *testing.T) {
		data := &MacroData{}
		got := computeInflationMomentum(data)
		if got != 0 {
			t.Errorf("no data should return 0, got %v", got)
		}
	})

	t.Run("stable_inflation", func(t *testing.T) {
		data := &MacroData{
			CorePCE:       2.5,
			MedianCPI:     3.5,
			Breakeven5Y:   2.25,
			MichInflExp1Y: 3.5,
		}
		got := computeInflationMomentum(data)
		if got < -0.3 || got > 0.3 {
			t.Errorf("moderate inflation should produce momentum near 0, got %v", got)
		}
	})
}

// --- TestComputeYieldCurveSignal ---

func TestComputeYieldCurveSignal(t *testing.T) {
	tests := []struct {
		name string
		data *MacroData
		want string
	}{
		{
			name: "deep_inversion",
			data: &MacroData{
				Spread10Y2Y: -0.8,
				Spread10Y3M: -0.6,
			},
			want: "DEEP_INVERSION",
		},
		{
			name: "inverted_both",
			data: &MacroData{
				Spread10Y2Y: -0.2,
				Spread10Y3M: -0.1,
			},
			want: "INVERTED",
		},
		{
			name: "inverted_one_side",
			data: &MacroData{
				Spread10Y2Y: -0.1,
				Spread10Y3M: 0.5,
			},
			want: "INVERTED",
		},
		{
			name: "flat",
			data: &MacroData{
				Spread10Y2Y: 0.1,
				Spread10Y3M: 0.5,
			},
			want: "FLAT",
		},
		{
			name: "normal",
			data: &MacroData{
				Spread10Y2Y: 0.8,
				Spread10Y3M: 0.9,
			},
			want: "NORMAL",
		},
		{
			name: "steep",
			data: &MacroData{
				Spread10Y2Y: 1.8,
				Spread10Y3M: 1.2,
			},
			want: "STEEP",
		},
		{
			name: "uses_precomputed_spread10y2y",
			data: &MacroData{
				YieldSpread: 0.5,     // would be NORMAL
				Spread10Y2Y: -0.3,    // override: INVERTED
				Spread10Y3M: -0.2,
			},
			want: "INVERTED",
		},
		{
			name: "fallback_to_yield_spread",
			data: &MacroData{
				YieldSpread: 0.1,
				Spread3M10Y: 0.5,
			},
			want: "FLAT",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeYieldCurveSignal(tt.data)
			if got != tt.want {
				t.Errorf("computeYieldCurveSignal() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- TestComputeCreditStress ---

func TestComputeCreditStress(t *testing.T) {
	t.Run("low_stress", func(t *testing.T) {
		data := &MacroData{
			TedSpread:    2.8,   // low HY spread
			BBBSpread:    1.2,
			AAASpread:    0.4,
			NFCI:         -0.4,  // loose
			StLouisStress: -0.8,
			SOFR:         5.30,
			IORB:         5.32,
		}
		got := computeCreditStress(data)
		if got > 25 {
			t.Errorf("low stress should score <= 25, got %v", got)
		}
		if got < 0 {
			t.Errorf("score should not go below 0, got %v", got)
		}
	})

	t.Run("high_stress", func(t *testing.T) {
		data := &MacroData{
			TedSpread:    7.5,   // wide HY spread
			BBBSpread:    3.8,
			AAASpread:    1.8,
			NFCI:         0.6,   // tight
			StLouisStress: 1.8,
			SOFR:         5.50,
			IORB:         5.30,
		}
		got := computeCreditStress(data)
		if got < 70 {
			t.Errorf("high stress should score >= 70, got %v", got)
		}
	})

	t.Run("no_data_returns_neutral_range", func(t *testing.T) {
		data := &MacroData{}
		got := computeCreditStress(data)
		// With NFCI=0 and StLouisStress=0 always included as neutral values,
		// empty MacroData scores ~39 (neutral zone: 20-40).
		if got < 20 || got > 50 {
			t.Errorf("empty data should score in neutral range 20-50, got %v", got)
		}
	})
}

// --- TestComputeHousingPulse ---

func TestComputeHousingPulse(t *testing.T) {
	t.Run("expanding", func(t *testing.T) {
		data := &MacroData{
			BuildingPermits:      1500,
			BuildingPermitsTrend: SeriesTrend{Direction: "UP"},
			HousingStarts:        1400,
			HousingStartsTrend:   SeriesTrend{Direction: "UP"},
			MortgageRate30Y:      4.5, // low rate = positive
		}
		got := computeHousingPulse(data)
		if got != "EXPANDING" {
			t.Errorf("expected EXPANDING, got %q", got)
		}
	})

	t.Run("contracting", func(t *testing.T) {
		data := &MacroData{
			BuildingPermits:      1200,
			BuildingPermitsTrend: SeriesTrend{Direction: "DOWN"},
			HousingStarts:        1100,
			HousingStartsTrend:   SeriesTrend{Direction: "FLAT"},
			MortgageRate30Y:      7.5, // high rate = negative
		}
		got := computeHousingPulse(data)
		if got != "CONTRACTING" {
			t.Errorf("expected CONTRACTING, got %q", got)
		}
	})

	t.Run("collapsing", func(t *testing.T) {
		data := &MacroData{
			BuildingPermits:      1200,
			BuildingPermitsTrend: SeriesTrend{Direction: "DOWN"},
			HousingStarts:        1100,
			HousingStartsTrend:   SeriesTrend{Direction: "DOWN"},
			MortgageRate30Y:      7.5, // high rate = negative
		}
		got := computeHousingPulse(data)
		if got != "COLLAPSING" {
			t.Errorf("expected COLLAPSING, got %q", got)
		}
	})

	t.Run("stable", func(t *testing.T) {
		data := &MacroData{
			BuildingPermits:      1300,
			BuildingPermitsTrend: SeriesTrend{Direction: "UP"},
			HousingStarts:        1200,
			HousingStartsTrend:   SeriesTrend{Direction: "DOWN"},
			MortgageRate30Y:      6.0, // neutral
		}
		got := computeHousingPulse(data)
		if got != "STABLE" {
			t.Errorf("expected STABLE, got %q", got)
		}
	})

	t.Run("no_data_returns_na", func(t *testing.T) {
		data := &MacroData{}
		got := computeHousingPulse(data)
		if got != "N/A" {
			t.Errorf("expected N/A, got %q", got)
		}
	})
}

// --- TestComputeSentimentComposite ---

func TestComputeSentimentComposite(t *testing.T) {
	t.Run("extreme_fear", func(t *testing.T) {
		data := &MacroData{
			CNNFearGreed:      10,    // extreme fear (contrarian bullish)
			AAIIBullBear:      0.4,   // bearish crowd
			PutCallTotal:      1.2,   // high put/call
			VIX:               33,    // high VIX
			ConsumerSentiment: 55,    // low sentiment
		}
		got := computeSentimentComposite(data)
		if got < 50 {
			t.Errorf("extreme fear scenario should produce score >= 50, got %v", got)
		}
	})

	t.Run("extreme_greed", func(t *testing.T) {
		data := &MacroData{
			CNNFearGreed:      90,    // extreme greed (contrarian bearish)
			AAIIBullBear:      1.8,   // bullish crowd
			PutCallTotal:      0.65,  // low put/call
			VIX:               13,    // low VIX
			ConsumerSentiment: 95,    // high sentiment
		}
		got := computeSentimentComposite(data)
		if got > -50 {
			t.Errorf("extreme greed scenario should produce score <= -50, got %v", got)
		}
	})

	t.Run("neutral_sentiment", func(t *testing.T) {
		data := &MacroData{
			CNNFearGreed:      50,
			AAIIBullBear:      1.0,
			PutCallTotal:      0.95,
			VIX:               20,
			ConsumerSentiment: 75,
		}
		got := computeSentimentComposite(data)
		if got < -30 || got > 30 {
			t.Errorf("neutral scenario should produce score near 0, got %v", got)
		}
	})

	t.Run("no_data_returns_zero", func(t *testing.T) {
		data := &MacroData{}
		got := computeSentimentComposite(data)
		if got != 0 {
			t.Errorf("no data should return 0, got %v", got)
		}
	})
}

// --- TestComputeComposites ---

func TestComputeComposites(t *testing.T) {
	t.Run("nil_data_returns_nil", func(t *testing.T) {
		got := ComputeComposites(nil)
		if got != nil {
			t.Error("nil input should return nil")
		}
	})

	t.Run("full_macro_data", func(t *testing.T) {
		data := &MacroData{
			// Labor
			InitialClaims:    210_000,
			ContinuingClaims: 1_700_000,
			UnemployRate:     3.8,
			U6Unemployment:   7.5,
			JOLTSOpenings:    10_000,
			JOLTSQuitRate:    2.5,
			EmpPopRatio:      60.5,
			// Inflation
			CorePCE:       2.8,
			MedianCPI:     3.5,
			StickyCPI:     4.0,
			PPICommodities: 3.0,
			Breakeven5Y:   2.3,
			MichInflExp1Y: 3.2,
			// Yield curve
			Spread10Y2Y: -0.3,
			Spread10Y3M: -0.1,
			// Credit
			TedSpread:    4.0,
			BBBSpread:    2.0,
			NFCI:         -0.2,
			StLouisStress: 0.5,
			// Housing
			BuildingPermits:      1400,
			BuildingPermitsTrend: SeriesTrend{Direction: "UP"},
			HousingStarts:        1300,
			HousingStartsTrend:   SeriesTrend{Direction: "UP"},
			MortgageRate30Y:      6.5,
			// Sentiment
			CNNFearGreed:      45,
			AAIIBullBear:      1.1,
			PutCallTotal:      0.85,
			VIX:               18,
			ConsumerSentiment: 70,
			// Country data
			GDPGrowth:    2.5,
			FedFundsRate: 5.25,
		}

		c := ComputeComposites(data)
		if c == nil {
			t.Fatal("expected non-nil composites")
		}

		// LaborHealth should be in valid range
		if c.LaborHealth < 0 || c.LaborHealth > 100 {
			t.Errorf("LaborHealth out of range: %v", c.LaborHealth)
		}
		if c.LaborLabel == "" {
			t.Error("LaborLabel should not be empty")
		}

		// InflationMomentum should be in range
		if c.InflationMomentum < -1.0 || c.InflationMomentum > 1.0 {
			t.Errorf("InflationMomentum out of range: %v", c.InflationMomentum)
		}
		if c.InflationLabel == "" {
			t.Error("InflationLabel should not be empty")
		}

		// YieldCurveSignal
		if c.YieldCurveSignal != "INVERTED" {
			t.Errorf("expected INVERTED yield curve, got %q", c.YieldCurveSignal)
		}

		// CreditStress should be in range
		if c.CreditStress < 0 || c.CreditStress > 100 {
			t.Errorf("CreditStress out of range: %v", c.CreditStress)
		}
		if c.CreditLabel == "" {
			t.Error("CreditLabel should not be empty")
		}

		// HousingPulse
		if c.HousingPulse == "" || c.HousingPulse == "N/A" {
			t.Error("HousingPulse should have a real value with data present")
		}

		// SentimentComposite range
		if c.SentimentComposite < -100 || c.SentimentComposite > 100 {
			t.Errorf("SentimentComposite out of range: %v", c.SentimentComposite)
		}
		if c.SentimentLabel == "" {
			t.Error("SentimentLabel should not be empty")
		}

		// VIX term regime default
		if c.VIXTermRegime != "N/A" && c.VIXTermRegime == "" {
			t.Error("VIXTermRegime should have a value")
		}

		// ComputedAt should be set
		if c.ComputedAt.IsZero() {
			t.Error("ComputedAt should be set")
		}
	})

	t.Run("empty_data", func(t *testing.T) {
		data := &MacroData{}
		c := ComputeComposites(data)
		if c == nil {
			t.Fatal("expected non-nil composites for empty data")
		}
		// Should get defaults
		if c.LaborHealth != 50 {
			t.Errorf("empty data LaborHealth should be 50, got %v", c.LaborHealth)
		}
		if c.InflationMomentum != 0 {
			t.Errorf("empty data InflationMomentum should be 0, got %v", c.InflationMomentum)
		}
		// CreditStress with zero-value NFCI/StLouisStress always included as neutral
		if c.CreditStress < 20 || c.CreditStress > 50 {
			t.Errorf("empty data CreditStress should be in neutral range 20-50, got %v", c.CreditStress)
		}
		if c.HousingPulse != "N/A" {
			t.Errorf("empty data HousingPulse should be N/A, got %q", c.HousingPulse)
		}
		if c.VIXTermRegime != "N/A" {
			t.Errorf("empty data VIXTermRegime should be N/A, got %q", c.VIXTermRegime)
		}
	})
}

// TestComputeComposites_NilAndEmptyDataGuard verifies that ComputeComposites
// never panics and returns well-defined values for edge-case inputs.
// Covers TASK-173: FRED Composites Nil Pointer Guard.
func TestComputeComposites_NilAndEmptyDataGuard(t *testing.T) {
	t.Run("nil_input_returns_nil", func(t *testing.T) {
		got := ComputeComposites(nil)
		if got != nil {
			t.Error("ComputeComposites(nil) must return nil")
		}
	})

	t.Run("empty_macrodata_no_panic", func(t *testing.T) {
		// Fresh install: all fields zero — should never panic
		got := ComputeComposites(&MacroData{})
		if got == nil {
			t.Fatal("expected non-nil composites for empty MacroData")
		}
		// Country scores must be finite
		scores := map[string]float64{
			"USScore": got.USScore,
			"EZScore": got.EZScore,
			"UKScore": got.UKScore,
			"JPScore": got.JPScore,
			"AUScore": got.AUScore,
			"CAScore": got.CAScore,
			"NZScore": got.NZScore,
		}
		for name, v := range scores {
			if v < -100 || v > 100 {
				t.Errorf("%s out of range [-100,100]: %.2f", name, v)
			}
		}
	})

	t.Run("partial_data_country_scores_finite", func(t *testing.T) {
		// Only US data available; foreign country scores should fall back to 0
		data := &MacroData{
			UnemployRate: 4.1,
			CorePCE:      2.4,
			GDPGrowth:    2.1,
			FedFundsRate: 4.5,
		}
		got := ComputeComposites(data)
		if got == nil {
			t.Fatal("expected non-nil composites for partial MacroData")
		}
		if got.USScore == 0 {
			t.Error("USScore should be non-zero when US data is present")
		}
		// Foreign scores should be 0 (no data)
		if got.EZScore != 0 {
			t.Errorf("EZScore should be 0 (no EZ data), got %.2f", got.EZScore)
		}
	})

	t.Run("vix_term_regime_defaults_to_na", func(t *testing.T) {
		got := ComputeComposites(&MacroData{})
		if got.VIXTermRegime != "N/A" {
			t.Errorf("empty VIXTermRegime should default to N/A, got %q", got.VIXTermRegime)
		}
	})

	t.Run("sanitize_does_not_alter_valid_scores", func(t *testing.T) {
		data := &MacroData{
			UnemployRate: 4.1,
			CorePCE:      2.4,
			GDPGrowth:    2.1,
			FedFundsRate: 4.5,
		}
		got := ComputeComposites(data)
		// sanitizeCompositeScores must preserve valid finite values
		before := got.USScore
		sanitizeCompositeScores(got)
		if got.USScore != before {
			t.Errorf("sanitizeCompositeScores altered a valid USScore: %.2f -> %.2f", before, got.USScore)
		}
	})
}
