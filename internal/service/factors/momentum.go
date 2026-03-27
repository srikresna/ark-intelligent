package factors

import "math"

// scoreMomentum computes a cross-sectional momentum score for an asset.
//
// We use a composite of 3 lookback returns:
//   - 1-month (21-day): short-term
//   - 3-month (63-day): medium-term
//   - 12-month (252-day) skip-1-month: standard TSMOM (skips last 21 days to avoid reversal)
//
// Returns raw momentum score (will be z-score normalized by engine).
func scoreMomentum(closes []float64) float64 {
	if len(closes) < 22 {
		return 0
	}

	ret := func(n, skip int) float64 {
		if len(closes) < n+skip {
			return 0
		}
		past := closes[skip+n-1]
		if past == 0 {
			return 0
		}
		return (closes[skip] - past) / past
	}

	// 1-month
	r1m := ret(21, 0)
	// 3-month
	r3m := ret(63, 0)
	// 12-month skip-1
	r12m := ret(252, 21)

	// Weights: 40% short, 30% medium, 30% long
	// If not enough data, partial weighting
	score := 0.0
	totalW := 0.0

	if r1m != 0 || len(closes) >= 22 {
		score += 0.40 * r1m
		totalW += 0.40
	}
	if len(closes) >= 64 {
		score += 0.30 * r3m
		totalW += 0.30
	}
	if len(closes) >= 273 {
		score += 0.30 * r12m
		totalW += 0.30
	}
	if totalW == 0 {
		return 0
	}
	return score / totalW
}

// returns calculates a slice of daily returns from close series (newest first).
func dailyReturns(closes []float64) []float64 {
	if len(closes) < 2 {
		return nil
	}
	rets := make([]float64, len(closes)-1)
	for i := 0; i < len(closes)-1; i++ {
		prev := closes[i+1]
		if prev == 0 {
			rets[i] = 0
			continue
		}
		rets[i] = (closes[i] - prev) / prev
	}
	return rets
}

// meanStd computes mean and population std of a slice.
func meanStd(vals []float64) (mean, std float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))
	for _, v := range vals {
		diff := v - mean
		std += diff * diff
	}
	std = math.Sqrt(std / float64(len(vals)))
	return
}

// zscore normalizes a slice of raw scores to [-1, +1] range using z-score clamped at ±2σ.
func zscore(raw []float64) []float64 {
	mean, std := meanStd(raw)
	out := make([]float64, len(raw))
	if std == 0 {
		return out
	}
	for i, v := range raw {
		z := (v - mean) / std
		// clamp to [-2, +2] then scale to [-1, +1]
		if z > 2 {
			z = 2
		}
		if z < -2 {
			z = -2
		}
		out[i] = z / 2.0
	}
	return out
}

// clamp1 clamps a float64 to [-1, +1].
func clamp1(v float64) float64 {
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

// sma computes a simple moving average of the first n elements.
func sma(data []float64, n int) float64 {
	if len(data) < n || n == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += data[i]
	}
	return sum / float64(n)
}
