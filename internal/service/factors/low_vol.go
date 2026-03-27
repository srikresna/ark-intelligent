package factors

import "math"

// scoreLowVol computes the low-volatility efficiency factor.
//
// Assets with lower realized vol AND acceptable return get rewarded.
// This is a Sharpe-proxy: annualizedReturn / annualizedVol
//
// Score is designed for cross-sectional normalization (higher = better risk efficiency).
// Returns raw score (engine z-scores across the universe).
func scoreLowVol(closes []float64, lookback int) float64 {
	if lookback == 0 {
		lookback = 63 // 3 months
	}
	if len(closes) < lookback+1 {
		return 0
	}

	rets := dailyReturns(closes[:lookback+1])
	if len(rets) == 0 {
		return 0
	}

	mean, std := meanStd(rets)
	if std == 0 {
		return 0
	}

	// Annualize
	annRet := mean * 252
	annVol := std * math.Sqrt(252)

	// Sharpe proxy (no risk-free rate subtraction for cross-sectional ranking)
	sharpe := annRet / annVol

	// For this factor we want to reward:
	//   1. Low vol (negative correlation to vol)
	//   2. Positive return (positive correlation to return)
	// Combined: Sharpe, clamped to [-3, +3] then scaled to [-1, +1]
	return clamp1(sharpe / 3.0)
}

// RealizedVol computes the annualized realized volatility of daily returns
// over the last n bars. Exported for use by other packages (microstructure, strategy).
func RealizedVol(closes []float64, n int) float64 {
	if len(closes) < n+1 {
		n = len(closes) - 1
	}
	if n < 2 {
		return 0
	}
	rets := dailyReturns(closes[:n+1])
	_, std := meanStd(rets)
	return std * math.Sqrt(252)
}
