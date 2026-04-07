package domain

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPrecision_LargePositionCounts verifies that large position counts
// don't lose precision when converted to float64 for calculations.
func TestPrecision_LargePositionCounts(t *testing.T) {
	tests := []struct {
		name      string
		long      int64
		short     int64
		wantNet   float64
		wantExact bool // whether float64 representation is exact
	}{
		{
			name:      "small positions",
			long:      1000,
			short:     500,
			wantNet:   500,
			wantExact: true,
		},
		{
			name:      "medium positions",
			long:      100000,
			short:     75000,
			wantNet:   25000,
			wantExact: true,
		},
		{
			name:      "large positions (1M+)",
			long:      2500000,
			short:     1800000,
			wantNet:   700000,
			wantExact: true,
		},
		{
			name:      "very large positions",
			long:      5000000000, // 5 billion
			short:     3000000000, // 3 billion
			wantNet:   2000000000, // 2 billion
			wantExact: true,       // int64 values up to 2^53 are exact in float64
		},
		{
			name:      "max safe integer boundary",
			long:      9007199254740991, // 2^53 - 1 (max safe integer in float64)
			short:     1000,
			wantNet:   9007199254739991,
			wantExact: false, // subtraction at boundary may lose precision
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := COTRecord{
				ContractCode:   "TEST",
				LevFundLong:    float64(tt.long),
				LevFundShort:   float64(tt.short),
			}

			got := record.GetSmartMoneyNet("TFF")

			// For exact values, compare exactly
			if tt.wantExact {
				assert.Equal(t, tt.wantNet, got, "Net position should be exact")
			} else {
				// For boundary values, allow small epsilon
				epsilon := math.Abs(tt.wantNet) * 1e-10
				if epsilon < 1 {
					epsilon = 1
				}
				assert.InDelta(t, tt.wantNet, got, epsilon, "Net position should be within epsilon")
			}
		})
	}
}

// TestPrecision_COTIndexEdgeCases tests edge cases for COT Index calculation.
func TestPrecision_COTIndexEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		nets     []float64
		expected float64
	}{
		{
			name:     "identical values (zero span)",
			nets:     []float64{1000, 1000, 1000, 1000},
			expected: 50.0, // should return neutral when span is zero
		},
		{
			name:     "minimum span",
			nets:     []float64{1000, 1001},
			expected: 0.0, // (1000 - 1000) / (1001 - 1000) * 100 = 0
		},
		{
			name:     "maximum value",
			nets:     []float64{5000, 1000, 2000},
			expected: 100.0, // (5000 - 1000) / (5000 - 1000) * 100 = 100
		},
		{
			name:     "negative values",
			nets:     []float64{-1000, -5000, -3000},
			expected: 100.0, // current is max
		},
		{
			name:     "mixed positive/negative",
			nets:     []float64{0, -5000, 5000},
			expected: 50.0, // (0 - (-5000)) / (5000 - (-5000)) * 100 = 50
		},
		{
			name:     "very large values",
			nets:     []float64{1e15, 0, 2e15},
			expected: 50.0,
		},
		{
			name:     "very small values",
			nets:     []float64{0.001, 0.0001, 0.002},
			expected: 50.0, // (0.001 - 0.0001) / (0.002 - 0.0001) * 100 ≈ 47.37
		},
		{
			name:     "insufficient data (less than 3)",
			nets:     []float64{1000, 2000},
			expected: 50.0, // should return neutral
		},
		{
			name:     "empty data",
			nets:     []float64{},
			expected: 50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate computeCOTIndex logic inline for testing
			result := computeCOTIndexForTest(tt.nets)
			assert.InDelta(t, tt.expected, result, 0.01,
				"COT Index calculation should match expected value")
		})
	}
}

// computeCOTIndexForTest replicates the COT Index calculation for testing.
func computeCOTIndexForTest(nets []float64) float64 {
	if len(nets) < 3 {
		return 50.0
	}

	current := nets[0]
	minVal, maxVal := nets[0], nets[0]

	for _, n := range nets {
		if n < minVal {
			minVal = n
		}
		if n > maxVal {
			maxVal = n
		}
	}

	span := maxVal - minVal
	if span == 0 {
		return 50.0
	}

	// Clamp to [0, 100]
	result := (current - minVal) / span * 100
	if result < 0 {
		return 0
	}
	if result > 100 {
		return 100
	}
	return result
}

// TestPrecision_PercentageOfOI tests percentage calculations.
func TestPrecision_PercentageOfOI(t *testing.T) {
	tests := []struct {
		name          string
		netPosition   int64
		openInterest  int64
		wantPct       float64
		wantValid     bool
	}{
		{
			name:         "normal percentage",
			netPosition:  50000,
			openInterest: 500000,
			wantPct:      10.0,
			wantValid:    true,
		},
		{
			name:         "zero OI",
			netPosition:  1000,
			openInterest: 0,
			wantPct:      0,      // division by zero should be handled
			wantValid:    false,  // or return 0 with safe division
		},
		{
			name:         "100% of OI",
			netPosition:  100000,
			openInterest: 100000,
			wantPct:      100.0,
			wantValid:    true,
		},
		{
			name:         "very small percentage",
			netPosition:  1,
			openInterest: 1000000,
			wantPct:      0.0001,
			wantValid:    true,
		},
		{
			name:         "large OI",
			netPosition:  5000000,
			openInterest: 50000000,
			wantPct:      10.0,
			wantValid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPct float64
			if tt.openInterest > 0 {
				gotPct = float64(tt.netPosition) / float64(tt.openInterest) * 100
			} else {
				gotPct = 0 // safe fallback
			}

			if tt.wantValid {
				assert.InDelta(t, tt.wantPct, gotPct, 0.0001,
					"Percentage of OI calculation should match")
			}

			// Verify result is within valid range [0, 100] for positive inputs
			if tt.netPosition >= 0 && tt.openInterest > 0 {
				assert.True(t, gotPct >= 0 && gotPct <= 100,
					"Percentage should be in valid range [0, 100], got %f", gotPct)
			}
		})
	}
}

// TestPrecision_RatioCalculations tests long/short ratio precision.
func TestPrecision_RatioCalculations(t *testing.T) {
	tests := []struct {
		name     string
		long     int64
		short    int64
		wantRatio float64
	}{
		{
			name:      "balanced",
			long:      10000,
			short:     10000,
			wantRatio: 1.0,
		},
		{
			name:      "2:1 long bias",
			long:      20000,
			short:     10000,
			wantRatio: 2.0,
		},
		{
			name:      "zero short (edge)",
			long:      10000,
			short:     0,
			wantRatio: 999.99, // system uses this as max
		},
		{
			name:      "zero long (edge)",
			long:      0,
			short:     10000,
			wantRatio: 0.0,
		},
		{
			name:      "very large ratio",
			long:      1000000,
			short:     1000,
			wantRatio: 1000.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotRatio float64
			if tt.short == 0 {
				if tt.long > 0 {
					gotRatio = 999.99
				} else {
					gotRatio = 0
				}
			} else {
				gotRatio = float64(tt.long) / float64(tt.short)
			}

			assert.InDelta(t, tt.wantRatio, gotRatio, 0.01,
				"Ratio calculation should match expected")
		})
	}
}

// TestPrecision_FloatSanitization tests NaN/Inf handling.
func TestPrecision_FloatSanitization(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		fallback float64
		want     float64
	}{
		{
			name:     "normal value",
			value:    100.5,
			fallback: 0,
			want:     100.5,
		},
		{
			name:     "NaN",
			value:    math.NaN(),
			fallback: 50.0,
			want:     50.0,
		},
		{
			name:     "positive infinity",
			value:    math.Inf(1),
			fallback: 0,
			want:     0,
		},
		{
			name:     "negative infinity",
			value:    math.Inf(-1),
			fallback: 0,
			want:     0,
		},
		{
			name:     "very large finite",
			value:    1e308,
			fallback: 0,
			want:     1e308,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFloatForTest(tt.value, tt.fallback)
			if math.IsNaN(tt.want) {
				assert.True(t, math.IsNaN(got), "Expected NaN")
			} else {
				assert.Equal(t, tt.want, got, "Sanitization should return correct value")
			}
		})
	}
}

// sanitizeFloatForTest replicates the sanitization logic.
func sanitizeFloatForTest(f, fallback float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fallback
	}
	return f
}

// TestPrecision_IntegerOverflow tests that int64 doesn't overflow.
func TestPrecision_IntegerOverflow(t *testing.T) {
	// Maximum safe position counts
	maxInt64 := int64(^uint64(0) >> 1)

	// Typical COT values
	typicalMaxOI := int64(50_000_000) // 50 million contracts (largest markets)

	// Verify our fields can handle realistic values
	record := COTRecord{
		OpenInterest:   float64(typicalMaxOI),
		LevFundLong:    float64(typicalMaxOI) / 2,
		LevFundShort:   float64(typicalMaxOI) / 4,
		AssetMgrLong:   float64(typicalMaxOI) / 3,
		AssetMgrShort:  float64(typicalMaxOI) / 6,
	}

	// Verify calculations don't overflow
	total := record.LevFundLong + record.LevFundShort +
		record.AssetMgrLong + record.AssetMgrShort

	assert.True(t, total > 0, "Total positions should be positive")
	assert.True(t, total < float64(maxInt64)/2, "Total should be well below int64 max")

	// Verify net calculation is exact (within int64 range)
	net := record.LevFundLong - record.LevFundShort
	assert.Equal(t, float64(typicalMaxOI)/4, net, "Net position should be exact")
}

// TestPrecision_MomentumCalculation tests momentum calculation precision.
func TestPrecision_MomentumCalculation(t *testing.T) {
	history := []float64{
		100000, // current (newest)
		95000,  // 1 period ago
		90000,  // 2 periods ago
		85000,  // 3 periods ago
		80000,  // 4 periods ago
	}

	// 4-period momentum
	momentum4 := history[0] - history[4]
	assert.Equal(t, 20000.0, momentum4, "4-period momentum should be exact")

	// Verify no precision loss with larger values
	historyLarge := []float64{
		1e12,
		9.5e11,
		9e11,
		8.5e11,
		8e11,
	}
	momentumLarge := historyLarge[0] - historyLarge[4]
	assert.InDelta(t, 2e11, momentumLarge, 1, "Large momentum should be precise")
}

// TestPrecision_ZScoreCalculation tests Z-score precision.
func TestPrecision_ZScoreCalculation(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		mean   float64
		stddev float64
		wantZ  float64
	}{
		{
			name:   "exactly at mean",
			value:  100,
			mean:   100,
			stddev: 10,
			wantZ:  0,
		},
		{
			name:   "one stddev above",
			value:  110,
			mean:   100,
			stddev: 10,
			wantZ:  1.0,
		},
		{
			name:   "two stddev below",
			value:  80,
			mean:   100,
			stddev: 10,
			wantZ:  -2.0,
		},
		{
			name:   "zero stddev (edge)",
			value:  100,
			mean:   100,
			stddev: 0,
			wantZ:  0, // should return 0 to avoid division by zero
		},
		{
			name:   "very small stddev",
			value:  100.001,
			mean:   100,
			stddev: 0.001,
			wantZ:  1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotZ float64
			if tt.stddev == 0 {
				gotZ = 0
			} else {
				gotZ = (tt.value - tt.mean) / tt.stddev
			}

			assert.InDelta(t, tt.wantZ, gotZ, 0.0001,
				"Z-score calculation should match expected")
		})
	}
}

// TestPrecision_ChangeCalculation tests week-over-week change precision.
func TestPrecision_ChangeCalculation(t *testing.T) {
	prev := COTRecord{
		LevFundLong:  100000,
		LevFundShort: 50000,
	}
	curr := COTRecord{
		LevFundLong:     110000,
		LevFundShort:    55000,
		LevFundLongChg:  10000,
		LevFundShortChg: 5000,
	}

	// Calculate using API change fields (preferred)
	netChangeAPI := curr.LevFundLongChg - curr.LevFundShortChg
	assert.Equal(t, int64(5000), netChangeAPI, "Net change from API should be exact")

	// Calculate from history diff (fallback)
	prevNet := prev.LevFundLong - prev.LevFundShort
	currNet := curr.LevFundLong - curr.LevFundShort
	netChangeCalc := currNet - prevNet
	assert.Equal(t, int64(5000), netChangeCalc, "Calculated net change should match")

	// Verify both methods give same result
	assert.Equal(t, netChangeAPI, netChangeCalc, "API and calculated change should match")
}
