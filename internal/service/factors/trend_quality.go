package factors

import "math"

// scoreTrendQuality computes a trend quality score using:
//  1. ADX proxy (Directional Movement Index strength)
//  2. MA alignment (price vs MA20/50/200)
//  3. R² of linear regression over last 63 bars (trend linearity)
//  4. Consecutive up/down bars streak
//
// Returns raw score (engine normalizes cross-sectionally).
func scoreTrendQuality(closes []float64) float64 {
	if len(closes) < 21 {
		return 0
	}

	score := 0.0
	components := 0.0

	// 1. ADX proxy using smoothed directional movement
	if len(closes) >= 28 {
		adx := computeADXProxy(closes, 14)
		// ADX 25+ = trending, normalize to 0-1 (50 = fully trending)
		adxScore := (adx - 20) / 30 // 20 = flat, 50 = strong trend
		score += clamp1(adxScore)
		components++
	}

	// 2. MA alignment
	if len(closes) >= 200 {
		ma20 := sma(closes, 20)
		ma50 := sma(closes, 50)
		ma200 := sma(closes, 200)
		cur := closes[0]

		maScore := 0.0
		if cur > ma20 {
			maScore += 0.33
		} else {
			maScore -= 0.33
		}
		if ma20 > ma50 {
			maScore += 0.33
		} else {
			maScore -= 0.33
		}
		if ma50 > ma200 {
			maScore += 0.34
		} else {
			maScore -= 0.34
		}
		score += maScore
		components++
	} else if len(closes) >= 50 {
		ma20 := sma(closes, 20)
		ma50 := sma(closes, 50)
		cur := closes[0]
		maScore := 0.0
		if cur > ma20 {
			maScore += 0.5
		} else {
			maScore -= 0.5
		}
		if ma20 > ma50 {
			maScore += 0.5
		} else {
			maScore -= 0.5
		}
		score += maScore
		components++
	}

	// 3. R² of linear regression over 63 bars — measures trend linearity
	if len(closes) >= 63 {
		r2 := computeR2(closes[:63])
		// r2 is 0-1. Transform to [-1,+1] by multiplying by direction
		// Use recent momentum direction for sign
		recentRet := 0.0
		if closes[62] != 0 {
			recentRet = (closes[0] - closes[62]) / closes[62]
		}
		sign := 1.0
		if recentRet < 0 {
			sign = -1.0
		}
		r2Score := sign * r2
		score += r2Score
		components++
	}

	// 4. Consecutive bars streak
	if len(closes) >= 10 {
		streak := computeStreak(closes, 10)
		// streak in [-10, +10], normalize to [-0.5, +0.5]
		streakScore := float64(streak) / 20.0
		score += streakScore
		components++
	}

	if components == 0 {
		return 0
	}
	return clamp1(score / components)
}

// computeADXProxy approximates ADX using Wilder's smoothing on True Range
// and directional movement over n periods. Returns ADX value (0-100).
func computeADXProxy(closes []float64, n int) float64 {
	if len(closes) < n+n {
		return 20 // default neutral
	}

	plusDM := make([]float64, len(closes)-1)
	minusDM := make([]float64, len(closes)-1)
	tr := make([]float64, len(closes)-1)

	// Note: closes[0] = most recent, closes[1] = previous
	for i := 0; i < len(closes)-1; i++ {
		high := closes[i] // simplified — using close as proxy (no H/L data in this function)
		low := closes[i]
		prevHigh := closes[i+1]
		prevLow := closes[i+1]
		prevClose := closes[i+1]

		upMove := high - prevHigh
		downMove := prevLow - low

		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		}
		// TR = max(high-low, abs(high-prevClose), abs(low-prevClose))
		trVal := high - low
		if v := math.Abs(high - prevClose); v > trVal {
			trVal = v
		}
		if v := math.Abs(low - prevClose); v > trVal {
			trVal = v
		}
		tr[i] = trVal
	}

	// Wilder smoothing
	smooth := func(vals []float64, period int) float64 {
		if len(vals) < period {
			return 0
		}
		sum := 0.0
		for i := 0; i < period; i++ {
			sum += vals[i]
		}
		result := sum
		for i := period; i < len(vals) && i < period*2; i++ {
			result = result - (result / float64(period)) + vals[i]
		}
		return result
	}

	smoothTR := smooth(tr, n)
	smoothPlusDM := smooth(plusDM, n)
	smoothMinusDM := smooth(minusDM, n)

	if smoothTR == 0 {
		return 20
	}

	plusDI := (smoothPlusDM / smoothTR) * 100
	minusDI := (smoothMinusDM / smoothTR) * 100

	diSum := plusDI + minusDI
	if diSum == 0 {
		return 20
	}

	dx := math.Abs(plusDI-minusDI) / diSum * 100
	return dx // simplified: returns DX instead of smoothed ADX, good enough as proxy
}

// computeR2 returns the coefficient of determination (R²) for a linear regression
// on the price series. Input is newest-first; we reverse internally.
func computeR2(closes []float64) float64 {
	n := len(closes)
	if n < 3 {
		return 0
	}
	// reverse for chronological order
	y := make([]float64, n)
	for i, v := range closes {
		y[n-1-i] = v
	}

	// x = 0..n-1 time index
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, v := range y {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}
	nf := float64(n)
	// slope and intercept
	denom := nf*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	slope := (nf*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / nf

	// R²
	meanY := sumY / nf
	ssTot, ssRes := 0.0, 0.0
	for i, v := range y {
		pred := slope*float64(i) + intercept
		ssRes += (v - pred) * (v - pred)
		ssTot += (v - meanY) * (v - meanY)
	}
	if ssTot == 0 {
		return 1
	}
	r2 := 1 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0
	}
	return r2
}

// computeStreak counts consecutive up/down bars in the last n bars.
// Returns positive for up streak, negative for down streak.
func computeStreak(closes []float64, n int) int {
	if len(closes) < n+1 {
		n = len(closes) - 1
	}
	streak := 0
	for i := 0; i < n; i++ {
		if closes[i] > closes[i+1] {
			if streak >= 0 {
				streak++
			} else {
				break
			}
		} else if closes[i] < closes[i+1] {
			if streak <= 0 {
				streak--
			} else {
				break
			}
		} else {
			break
		}
	}
	return streak
}
