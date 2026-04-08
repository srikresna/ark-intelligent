package mathutil

import (
	"math"
	"testing"
)

const epsilon = 1e-6

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

// ---------------------------------------------------------------------------
// Descriptive Statistics
// ---------------------------------------------------------------------------

func TestMean(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 5},
		{"positive", []float64{1, 2, 3, 4, 5}, 3},
		{"negative", []float64{-1, -2, -3}, -2},
		{"mixed", []float64{-10, 10}, 0},
		{"all_zeros", []float64{0, 0, 0}, 0},
		{"large_values", []float64{1e12, 2e12, 3e12}, 2e12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mean(tt.data)
			if !floatEqual(got, tt.want) {
				t.Errorf("Mean(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestStdDev(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 0},
		{"identical", []float64{3, 3, 3, 3}, 0},
		{"simple", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 2.0},
		{"two_elements", []float64{0, 10}, 5},
		{"negative", []float64{-2, -4, -4, -4, -5, -5, -7, -9}, 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StdDev(tt.data)
			if !floatEqual(got, tt.want) {
				t.Errorf("StdDev(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestStdDevSample(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 0},
		{"identical", []float64{3, 3, 3}, 0},
		{"two_elements", []float64{0, 10}, math.Sqrt(50)},                    // ss=50+50=100, 100/(2-1)=100, sqrt=10
		{"simple", []float64{2, 4, 4, 4, 5, 5, 7, 9}, math.Sqrt(32.0 / 7.0)}, // population var=4, sample var=32/7
		{"three_elements", []float64{1, 2, 3}, 1.0},                          // mean=2, ss=2, 2/(3-1)=1, sqrt=1
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StdDevSample(tt.data)
			if !floatEqual(got, tt.want) {
				t.Errorf("StdDevSample(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{7}, 7},
		{"odd_count", []float64{3, 1, 2}, 2},
		{"even_count", []float64{4, 1, 3, 2}, 2.5},
		{"already_sorted", []float64{1, 2, 3, 4, 5}, 3},
		{"duplicates", []float64{5, 5, 5, 5}, 5},
		{"negative", []float64{-3, -1, -2}, -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Median(tt.data)
			if !floatEqual(got, tt.want) {
				t.Errorf("Median(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestMedianDoesNotMutateInput(t *testing.T) {
	data := []float64{5, 3, 1, 4, 2}
	orig := make([]float64, len(data))
	copy(orig, data)
	Median(data)
	for i := range data {
		if data[i] != orig[i] {
			t.Fatalf("Median mutated input at index %d: got %v, want %v", i, data[i], orig[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Percentile & Normalization
// ---------------------------------------------------------------------------

func TestPercentile(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		p    float64
		want float64
	}{
		{"empty", nil, 50, 0},
		{"single_p50", []float64{10}, 50, 10},
		{"p0", []float64{1, 2, 3, 4, 5}, 0, 1},
		{"p100", []float64{1, 2, 3, 4, 5}, 100, 5},
		{"p50_odd", []float64{1, 2, 3, 4, 5}, 50, 3},
		{"p25", []float64{1, 2, 3, 4, 5}, 25, 2},
		{"p75", []float64{1, 2, 3, 4, 5}, 75, 4},
		{"negative_p", []float64{1, 2, 3}, -10, 1},
		{"over_100_p", []float64{1, 2, 3}, 110, 3},
		{"unsorted_input", []float64{5, 1, 3, 2, 4}, 50, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Percentile(tt.data, tt.p)
			if !floatEqual(got, tt.want) {
				t.Errorf("Percentile(%v, %v) = %v, want %v", tt.data, tt.p, got, tt.want)
			}
		})
	}
}

func TestPercentileDoesNotMutateInput(t *testing.T) {
	data := []float64{5, 3, 1, 4, 2}
	orig := make([]float64, len(data))
	copy(orig, data)
	Percentile(data, 50)
	for i := range data {
		if data[i] != orig[i] {
			t.Fatalf("Percentile mutated input at index %d: got %v, want %v", i, data[i], orig[i])
		}
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name            string
		value, min, max float64
		want            float64
	}{
		{"midpoint", 50, 0, 100, 50},
		{"at_min", 0, 0, 100, 0},
		{"at_max", 100, 0, 100, 100},
		{"equal_min_max", 5, 5, 5, 50},
		{"below_min_clamped", -10, 0, 100, 0},
		{"above_max_clamped", 200, 0, 100, 100},
		{"quarter", 25, 0, 100, 25},
		{"negative_range", -5, -10, 0, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.value, tt.min, tt.max)
			if !floatEqual(got, tt.want) {
				t.Errorf("Normalize(%v, %v, %v) = %v, want %v", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestMinMaxIndex(t *testing.T) {
	tests := []struct {
		name                string
		current, minN, maxN float64
		want                float64
	}{
		{"midpoint", 50, 0, 100, 50},
		{"at_min", 0, 0, 100, 0},
		{"at_max", 100, 0, 100, 100},
		{"equal_min_max", 10, 10, 10, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MinMaxIndex(tt.current, tt.minN, tt.maxN)
			if !floatEqual(got, tt.want) {
				t.Errorf("MinMaxIndex(%v, %v, %v) = %v, want %v", tt.current, tt.minN, tt.maxN, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Moving Averages
// ---------------------------------------------------------------------------

func TestSMA(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		n    int
		want float64
	}{
		{"empty", nil, 3, 0},
		{"zero_period", []float64{1, 2, 3}, 0, 0},
		{"negative_period", []float64{1, 2, 3}, -1, 0},
		{"single_element", []float64{5}, 1, 5},
		{"n_equals_len", []float64{1, 2, 3, 4, 5}, 5, 3},
		{"n_greater_than_len", []float64{1, 2, 3}, 10, 2},
		{"last_3", []float64{1, 2, 3, 4, 5}, 3, 4},
		{"last_1", []float64{1, 2, 3, 4, 5}, 1, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SMA(tt.data, tt.n)
			if !floatEqual(got, tt.want) {
				t.Errorf("SMA(%v, %d) = %v, want %v", tt.data, tt.n, got, tt.want)
			}
		})
	}
}

func TestEMA(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		n    int
		want float64
	}{
		{"empty", nil, 3, 0},
		{"zero_period", []float64{1, 2, 3}, 0, 0},
		{"negative_period", []float64{1, 2, 3}, -1, 0},
		{"single_element", []float64{5}, 3, 5},
		{"constant", []float64{4, 4, 4, 4}, 3, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EMA(tt.data, tt.n)
			if !floatEqual(got, tt.want) {
				t.Errorf("EMA(%v, %d) = %v, want %v", tt.data, tt.n, got, tt.want)
			}
		})
	}

	// Manual calculation for EMA with n=3 on [1,2,3,4,5]: k=0.5
	// ema=1, then (2*0.5+1*0.5)=1.5, (3*0.5+1.5*0.5)=2.25, (4*0.5+2.25*0.5)=3.125, (5*0.5+3.125*0.5)=4.0625
	t.Run("increasing_sequence", func(t *testing.T) {
		got := EMA([]float64{1, 2, 3, 4, 5}, 3)
		want := 4.0625
		if !floatEqual(got, want) {
			t.Errorf("EMA([1,2,3,4,5], 3) = %v, want %v", got, want)
		}
	})
}

// ---------------------------------------------------------------------------
// Rate of Change & Momentum
// ---------------------------------------------------------------------------

func TestRateOfChange(t *testing.T) {
	tests := []struct {
		name              string
		current, previous float64
		want              float64
	}{
		{"zero_previous", 10, 0, 0},
		{"no_change", 10, 10, 0},
		{"positive_change", 110, 100, 10},
		{"negative_change", 90, 100, -10},
		{"negative_previous", -10, -100, 90},
		{"from_negative_to_positive", 10, -10, 200},
		{"double", 20, 10, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RateOfChange(tt.current, tt.previous)
			if !floatEqual(got, tt.want) {
				t.Errorf("RateOfChange(%v, %v) = %v, want %v", tt.current, tt.previous, got, tt.want)
			}
		})
	}
}

func TestMomentum(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		n    int
		want float64
	}{
		{"empty", nil, 1, 0},
		{"n_zero", []float64{1, 2, 3}, 0, 0},
		{"n_negative", []float64{1, 2, 3}, -1, 0},
		{"insufficient_data", []float64{1, 2}, 3, 0},
		{"lookback_1", []float64{1, 2, 3, 4, 5}, 1, 1},
		{"lookback_4", []float64{1, 2, 3, 4, 5}, 4, 4},
		{"negative_momentum", []float64{10, 8, 6, 4, 2}, 2, -4},
		{"exact_boundary", []float64{1, 2}, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Momentum(tt.data, tt.n)
			if !floatEqual(got, tt.want) {
				t.Errorf("Momentum(%v, %d) = %v, want %v", tt.data, tt.n, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Financial Helpers
// ---------------------------------------------------------------------------

func TestZScore(t *testing.T) {
	tests := []struct {
		name                string
		value, mean, stddev float64
		want                float64
	}{
		{"zero_stddev", 5, 3, 0, 0},
		{"at_mean", 5, 5, 1, 0},
		{"one_above", 6, 5, 1, 1},
		{"one_below", 4, 5, 1, -1},
		{"two_above", 7, 5, 1, 2},
		{"fractional", 5.5, 5, 2, 0.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ZScore(tt.value, tt.mean, tt.stddev)
			if !floatEqual(got, tt.want) {
				t.Errorf("ZScore(%v, %v, %v) = %v, want %v", tt.value, tt.mean, tt.stddev, got, tt.want)
			}
		})
	}
}

func TestExponentialDecay(t *testing.T) {
	tests := []struct {
		name                  string
		value, tVal, halfLife float64
		want                  float64
	}{
		{"zero_halflife", 100, 1, 0, 0},
		{"negative_halflife", 100, 1, -5, 0},
		{"t_zero", 100, 0, 10, 100},
		{"one_halflife", 100, 10, 10, 50},
		{"two_halflives", 100, 20, 10, 25},
		{"value_zero", 0, 5, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExponentialDecay(tt.value, tt.tVal, tt.halfLife)
			if !floatEqual(got, tt.want) {
				t.Errorf("ExponentialDecay(%v, %v, %v) = %v, want %v", tt.value, tt.tVal, tt.halfLife, got, tt.want)
			}
		})
	}
}

func TestCumulativeDecaySum(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		ages     []float64
		halfLife float64
		want     float64
	}{
		{"mismatched_lengths", []float64{1, 2}, []float64{1}, 10, 0},
		{"empty", nil, nil, 10, 0},
		{"single_no_decay", []float64{100}, []float64{0}, 10, 100},
		{"single_one_halflife", []float64{100}, []float64{10}, 10, 50},
		{"two_values", []float64{100, 100}, []float64{0, 10}, 10, 150},
		{"all_zero_age", []float64{10, 20, 30}, []float64{0, 0, 0}, 5, 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CumulativeDecaySum(tt.values, tt.ages, tt.halfLife)
			if !floatEqual(got, tt.want) {
				t.Errorf("CumulativeDecaySum(%v, %v, %v) = %v, want %v", tt.values, tt.ages, tt.halfLife, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Utility Functions
// ---------------------------------------------------------------------------

func TestClamp(t *testing.T) {
	tests := []struct {
		name            string
		value, min, max float64
		want            float64
	}{
		{"within_range", 5, 0, 10, 5},
		{"at_min", 0, 0, 10, 0},
		{"at_max", 10, 0, 10, 10},
		{"below_min", -5, 0, 10, 0},
		{"above_max", 15, 0, 10, 10},
		{"negative_range", -5, -10, -1, -5},
		{"equal_min_max", 5, 3, 3, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Clamp(tt.value, tt.min, tt.max)
			if !floatEqual(got, tt.want) {
				t.Errorf("Clamp(%v, %v, %v) = %v, want %v", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		want float64
	}{
		{"positive", 5.5, 5.5},
		{"negative", -5.5, 5.5},
		{"zero", 0, 0},
		{"large_negative", -1e15, 1e15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Abs(tt.v)
			if !floatEqual(got, tt.want) {
				t.Errorf("Abs(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestSign(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		want float64
	}{
		{"positive", 42, 1},
		{"negative", -42, -1},
		{"zero", 0, 0},
		{"small_positive", 0.0001, 1},
		{"small_negative", -0.0001, -1},
		{"large_positive", 1e15, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sign(tt.v)
			if !floatEqual(got, tt.want) {
				t.Errorf("Sign(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestMinFloat64(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 5},
		{"positive", []float64{3, 1, 4, 1, 5}, 1},
		{"negative", []float64{-3, -1, -4}, -4},
		{"mixed", []float64{-10, 0, 10}, -10},
		{"all_same", []float64{7, 7, 7}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MinFloat64(tt.data)
			if !floatEqual(got, tt.want) {
				t.Errorf("MinFloat64(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestMaxFloat64(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 5},
		{"positive", []float64{3, 1, 4, 1, 5}, 5},
		{"negative", []float64{-3, -1, -4}, -1},
		{"mixed", []float64{-10, 0, 10}, 10},
		{"all_same", []float64{7, 7, 7}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxFloat64(tt.data)
			if !floatEqual(got, tt.want) {
				t.Errorf("MaxFloat64(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestConsecutiveDirection(t *testing.T) {
	tests := []struct {
		name    string
		data    []float64
		wantN   int
		wantDir float64
	}{
		{"empty", nil, 0, 0},
		{"single_positive", []float64{5}, 1, 1},
		{"single_negative", []float64{-5}, 1, -1},
		{"single_zero", []float64{0}, 1, 0},
		{"all_positive", []float64{1, 2, 3}, 3, 1},
		{"all_negative", []float64{-1, -2, -3}, 3, -1},
		{"switch_at_end", []float64{1, 2, -1, -2}, 2, -1},
		{"switch_mid", []float64{-3, -2, 1, 2, 3}, 3, 1},
		{"trailing_zeros", []float64{1, 2, 0, 0}, 2, 0},
		{"mixed_ending_positive", []float64{-1, -2, 3, 4}, 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotN, gotDir := ConsecutiveDirection(tt.data)
			if gotN != tt.wantN || !floatEqual(gotDir, tt.wantDir) {
				t.Errorf("ConsecutiveDirection(%v) = (%d, %v), want (%d, %v)",
					tt.data, gotN, gotDir, tt.wantN, tt.wantDir)
			}
		})
	}
}
