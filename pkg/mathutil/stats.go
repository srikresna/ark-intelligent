// Package mathutil provides financial mathematics and statistical helpers
// used across the quantitative analysis engine.
package mathutil

import (
	"fmt"
	"math"
	"sort"
)

// ---------------------------------------------------------------------------
// Descriptive Statistics
// ---------------------------------------------------------------------------

// Mean returns the arithmetic mean of a float64 slice.
// Returns 0 for empty input.
func Mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// StdDev returns the population standard deviation.
// Returns 0 for fewer than 2 data points.
func StdDev(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := Mean(data)
	var ss float64
	for _, v := range data {
		d := v - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(data)))
}

// StdDevSample returns the sample standard deviation (Bessel-corrected).
func StdDevSample(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := Mean(data)
	var ss float64
	for _, v := range data {
		d := v - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(data)-1))
}

// Median returns the median value. Input is NOT mutated.
func Median(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// ---------------------------------------------------------------------------
// Percentile & Normalization
// ---------------------------------------------------------------------------

// Percentile returns the p-th percentile (0-100) using linear interpolation.
// Input is NOT mutated.
func Percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	rank := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// Normalize rescales a value into [0, 100] given the min and max range.
// Returns 50 if min == max (avoids division by zero).
func Normalize(value, min, max float64) float64 {
	if max == min {
		return 50
	}
	n := (value - min) / (max - min) * 100
	return Clamp(n, 0, 100)
}

// MinMaxIndex computes the Williams-style COT Index:
//
//	Index = (current - minN) / (maxN - minN) * 100
//
// Returns 50 if maxN == minN.
func MinMaxIndex(current, minN, maxN float64) float64 {
	return Normalize(current, minN, maxN)
}

// ---------------------------------------------------------------------------
// Moving Averages
// ---------------------------------------------------------------------------

// SMA returns the Simple Moving Average of the last n values.
// If len(data) < n, uses all available data.
func SMA(data []float64, n int) float64 {
	if len(data) == 0 || n <= 0 {
		return 0
	}
	if n > len(data) {
		n = len(data)
	}
	var sum float64
	for i := len(data) - n; i < len(data); i++ {
		sum += data[i]
	}
	return sum / float64(n)
}

// EMA returns the Exponential Moving Average of the last n values.
// Uses smoothing factor k = 2 / (n + 1).
func EMA(data []float64, n int) float64 {
	if len(data) == 0 || n <= 0 {
		return 0
	}
	k := 2.0 / (float64(n) + 1)
	ema := data[0]
	for i := 1; i < len(data); i++ {
		ema = data[i]*k + ema*(1-k)
	}
	return ema
}

// ---------------------------------------------------------------------------
// Rate of Change & Momentum
// ---------------------------------------------------------------------------

// RateOfChange returns (current - previous) / |previous| * 100.
// Returns 0 if previous is zero.
func RateOfChange(current, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return (current - previous) / math.Abs(previous) * 100
}

// Momentum returns the simple difference: current - nPeriodsAgo.
// data should be ordered oldest-first. n is the lookback period.
func Momentum(data []float64, n int) float64 {
	if len(data) < n+1 || n <= 0 {
		return 0
	}
	return data[len(data)-1] - data[len(data)-1-n]
}

// ---------------------------------------------------------------------------
// Financial Helpers
// ---------------------------------------------------------------------------

// ZScore computes (value - mean) / stddev. Returns 0 if stddev is zero.
func ZScore(value, mean, stddev float64) float64 {
	if stddev == 0 {
		return 0
	}
	return (value - mean) / stddev
}

// ExponentialDecay returns value * exp(-lambda * t).
// lambda = ln(2) / halfLife (half-life in same units as t).
func ExponentialDecay(value, t, halfLife float64) float64 {
	if halfLife <= 0 {
		return 0
	}
	lambda := math.Ln2 / halfLife
	return value * math.Exp(-lambda*t)
}

// CumulativeDecaySum computes the time-decayed rolling sum of values.
// Each element in values has an associated age (in days) in ages.
// halfLife controls the decay rate in days.
func CumulativeDecaySum(values, ages []float64, halfLife float64) float64 {
	if len(values) != len(ages) {
		return 0
	}
	var sum float64
	for i, v := range values {
		sum += ExponentialDecay(v, ages[i], halfLife)
	}
	return sum
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// Clamp restricts value to [min, max].
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Abs returns the absolute value of a float64.
func Abs(v float64) float64 {
	return math.Abs(v)
}

// Sign returns -1, 0, or +1 based on the sign of v.
func Sign(v float64) float64 {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// MinFloat64 returns the minimum value in a float64 slice.
func MinFloat64(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	min := data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// MaxFloat64 returns the maximum value in a float64 slice.
func MaxFloat64(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	max := data[0]
	for _, v := range data[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// ---------------------------------------------------------------------------
// Risk-Adjusted Performance Metrics
// ---------------------------------------------------------------------------

// SharpeRatio computes the annualized Sharpe ratio from weekly returns.
// riskFreeRate should be the weekly risk-free rate (e.g. annual / 52).
// Annualizes by multiplying by sqrt(52) since input is weekly data.
// Returns 0 if there are fewer than 2 data points or if stddev is zero.
func SharpeRatio(returns []float64, riskFreeRate float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	excess := make([]float64, len(returns))
	for i, r := range returns {
		excess[i] = r - riskFreeRate
	}
	avg := Mean(excess)
	sd := StdDevSample(excess)
	if sd == 0 {
		return 0
	}
	return (avg / sd) * math.Sqrt(52)
}

// MaxDrawdown computes the maximum peak-to-trough drawdown from a series
// of periodic returns (%). It builds a cumulative equity curve (starting at 1.0)
// and finds the largest percentage decline from any peak.
// Returns (maxDD, peakIdx, troughIdx). maxDD is expressed as a positive
// percentage (e.g. 15.0 means a 15% drawdown). Returns (0, 0, 0) for empty input.
func MaxDrawdown(returns []float64) (maxDD float64, peakIdx, troughIdx int) {
	if len(returns) == 0 {
		return 0, 0, 0
	}

	// Build cumulative equity curve: equity[0] = 1.0, equity[i] = equity[i-1] * (1 + returns[i-1]/100)
	equity := make([]float64, len(returns)+1)
	equity[0] = 1.0
	for i, r := range returns {
		equity[i+1] = equity[i] * (1 + r/100)
		if equity[i+1] <= 0 {
			equity[i+1] = 1e-18 // Floor to avoid div-by-zero downstream
		}
	}

	peak := equity[0]
	peakI := 0
	for i := 1; i < len(equity); i++ {
		if equity[i] > peak {
			peak = equity[i]
			peakI = i
		}
		dd := (peak - equity[i]) / peak * 100
		if dd > maxDD {
			maxDD = dd
			peakIdx = peakI
			troughIdx = i
		}
	}
	return math.Round(maxDD*100) / 100, peakIdx, troughIdx
}

// CalmarRatio computes the Calmar ratio: avgAnnualReturn / maxDrawdown.
// Both inputs should be expressed as percentages.
// Returns 0 if maxDrawdown is zero.
func CalmarRatio(avgAnnualReturn, maxDrawdown float64) float64 {
	if maxDrawdown == 0 {
		return 0
	}
	return avgAnnualReturn / maxDrawdown
}

// ProfitFactor computes sum(wins) / |sum(losses)|.
// wins should contain positive return values, losses should contain negative values.
// Returns 0 if there are no losses (infinite would be misleading).
func ProfitFactor(wins, losses []float64) float64 {
	var sumW, sumL float64
	for _, w := range wins {
		sumW += w
	}
	for _, l := range losses {
		sumL += l
	}
	if sumL == 0 {
		return 0
	}
	return sumW / math.Abs(sumL)
}

// ExpectedValue computes the expected return per trade:
//
//	EV = (winRate * avgWin) + ((1 - winRate) * avgLoss)
//
// winRate is expressed as a fraction (0-1). avgWin is positive, avgLoss is negative.
func ExpectedValue(winRate float64, avgWin, avgLoss float64) float64 {
	return (winRate * avgWin) + ((1 - winRate) * avgLoss)
}

// KellyCriterion computes the Kelly fraction for position sizing:
//
//	f* = W - (1-W)/R
//
// where W = winRate (fraction 0-1) and R = winLossRatio (avgWin / |avgLoss|).
// Returns 0 if winLossRatio is zero or result is negative (don't bet).
func KellyCriterion(winRate, winLossRatio float64) float64 {
	if winLossRatio == 0 {
		return 0
	}
	f := winRate - (1-winRate)/winLossRatio
	if f < 0 {
		return 0
	}
	return f
}

// ---------------------------------------------------------------------------
// Statistical Significance Testing
// ---------------------------------------------------------------------------

// TTestOneSample performs a one-sample t-test against a hypothesized mean mu.
// Returns the t-statistic and two-tailed p-value.
// If fewer than 2 values are provided, returns (0, 1).
func TTestOneSample(values []float64, mu float64) (tStat, pValue float64) {
	n := len(values)
	if n < 2 {
		return 0, 1
	}
	m := Mean(values)
	s := StdDevSample(values)
	if s == 0 {
		return 0, 1
	}
	tStat = (m - mu) / (s / math.Sqrt(float64(n)))
	df := float64(n - 1)
	pValue = 2 * tDistCDF(-math.Abs(tStat), df)
	return tStat, pValue
}

// WinRatePValue computes the p-value for a one-sided binomial test:
// is the observed win rate significantly greater than 50%?
// Uses a normal approximation to the binomial distribution.
// Returns 1 if total < 1.
func WinRatePValue(wins, total int) float64 {
	if total < 1 {
		return 1
	}
	p0 := 0.5
	pHat := float64(wins) / float64(total)
	se := math.Sqrt(p0 * (1 - p0) / float64(total))
	if se == 0 {
		return 1
	}
	z := (pHat - p0) / se
	// One-sided: P(Z > z)
	return 1 - normalCDF(z)
}

// ConfidenceInterval returns the (lower, upper) confidence interval for a mean
// using a normal approximation. confidence is expressed as a fraction, e.g. 0.95.
// Returns (0, 0) if n < 2.
func ConfidenceInterval(mean, stddev float64, n int, confidence float64) (lower, upper float64) {
	if n < 2 {
		return 0, 0
	}
	z := normalQuantile((1 + confidence) / 2)
	margin := z * stddev / math.Sqrt(float64(n))
	return mean - margin, mean + margin
}

// MinSampleSize returns the minimum number of samples needed to achieve
// the target precision (half-width of CI) at the given confidence level.
// Uses worst-case proportion variance (p=0.5) for binomial-style metrics.
// Returns at least 2.
func MinSampleSize(targetPrecision, confidence float64) int {
	if targetPrecision <= 0 {
		return 2
	}
	z := normalQuantile((1 + confidence) / 2)
	// For a proportion, sigma = sqrt(p*(1-p)) <= 0.5, use 0.5 for worst case.
	sigma := 0.5
	n := math.Ceil((z * sigma / targetPrecision) * (z * sigma / targetPrecision))
	if n < 2 {
		return 2
	}
	return int(n)
}

// ---------------------------------------------------------------------------
// Internal math helpers for significance testing
// ---------------------------------------------------------------------------

// normalCDF approximates the cumulative distribution function of the
// standard normal distribution.
func normalCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

// normalQuantile approximates the inverse CDF (quantile function) of the
// standard normal distribution using the rational approximation by
// Peter Acklam (accurate to ~1.15e-9).
func normalQuantile(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}
	if p == 0.5 {
		return 0
	}

	const (
		a1 = -3.969683028665376e+01
		a2 = 2.209460984245205e+02
		a3 = -2.759285104469687e+02
		a4 = 1.383577518672690e+02
		a5 = -3.066479806614716e+01
		a6 = 2.506628277459239e+00

		b1 = -5.447609879822406e+01
		b2 = 1.615858368580409e+02
		b3 = -1.556989798598866e+02
		b4 = 6.680131188771972e+01
		b5 = -1.328068155288572e+01

		c1 = -7.784894002430293e-03
		c2 = -3.223964580411365e-01
		c3 = -2.400758277161838e+00
		c4 = -2.549732539343734e+00
		c5 = 4.374664141464968e+00
		c6 = 2.938163982698783e+00

		d1 = 7.784695709041462e-03
		d2 = 3.224671290700398e-01
		d3 = 2.445134137142996e+00
		d4 = 3.754408661907416e+00

		pLow  = 0.02425
		pHigh = 1 - pLow
	)

	var q, r float64
	if p < pLow {
		q = math.Sqrt(-2 * math.Log(p))
		return (((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) /
			((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	} else if p <= pHigh {
		q = p - 0.5
		r = q * q
		return (((((a1*r+a2)*r+a3)*r+a4)*r+a5)*r + a6) * q /
			(((((b1*r+b2)*r+b3)*r+b4)*r+b5)*r + 1)
	} else {
		q = math.Sqrt(-2 * math.Log(1-p))
		return -(((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) /
			((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	}
}

// tDistCDF approximates the CDF of the Student's t-distribution with df
// degrees of freedom using the regularized incomplete beta function.
func tDistCDF(t float64, df float64) float64 {
	if df <= 0 {
		return 0.5
	}
	x := df / (df + t*t)
	beta := regIncBeta(df/2, 0.5, x)
	if t >= 0 {
		return 1 - 0.5*beta
	}
	return 0.5 * beta
}

// regIncBeta computes the regularized incomplete beta function I_x(a, b)
// using a continued fraction expansion (Lentz's method).
func regIncBeta(a, b, x float64) float64 {
	if x < 0 || x > 1 {
		return 0
	}
	if x == 0 {
		return 0
	}
	if x == 1 {
		return 1
	}

	// Use the symmetry relation if x > (a+1)/(a+b+2) for better convergence.
	if x > (a+1)/(a+b+2) {
		return 1 - regIncBeta(b, a, 1-x)
	}

	lnBeta := lgamma(a) + lgamma(b) - lgamma(a+b)
	front := math.Exp(math.Log(x)*a+math.Log(1-x)*b-lnBeta) / a

	// Lentz's continued fraction.
	const maxIter = 200
	const epsilon = 1e-14
	c := 1.0
	d := 1 - (a+b)*x/(a+1)
	if math.Abs(d) < epsilon {
		d = epsilon
	}
	d = 1 / d
	f := d

	for i := 1; i <= maxIter; i++ {
		m := float64(i)
		// Even step
		num := m * (b - m) * x / ((a + 2*m - 1) * (a + 2*m))
		d = 1 + num*d
		if math.Abs(d) < epsilon {
			d = epsilon
		}
		c = 1 + num/c
		if math.Abs(c) < epsilon {
			c = epsilon
		}
		d = 1 / d
		f *= d * c

		// Odd step
		num = -(a + m) * (a + b + m) * x / ((a + 2*m) * (a + 2*m + 1))
		d = 1 + num*d
		if math.Abs(d) < epsilon {
			d = epsilon
		}
		c = 1 + num/c
		if math.Abs(c) < epsilon {
			c = epsilon
		}
		d = 1 / d
		delta := d * c
		f *= delta

		if math.Abs(delta-1) < epsilon {
			break
		}
	}

	return front * f
}

// lgamma wraps math.Lgamma, discarding the sign.
func lgamma(x float64) float64 {
	v, _ := math.Lgamma(x)
	return v
}

// ---------------------------------------------------------------------------
// Platt Scaling — Logistic regression for confidence calibration
// ---------------------------------------------------------------------------

// PlattScaling fits a logistic regression P(win) = 1 / (1 + exp(a*x + b))
// to map raw confidence scores to calibrated probabilities.
// Uses Newton-Raphson optimization (8 iterations, sufficient for convergence).
// confidences are raw values (0-100), outcomes are true=win, false=loss.
// Returns (a, b) coefficients. If input is insufficient, returns (0, 0).
func PlattScaling(confidences []float64, outcomes []bool) (a, b float64) {
	n := len(confidences)
	if n < 2 || n != len(outcomes) {
		return 0, 0
	}

	// Target values with Platt's label smoothing to avoid overfitting:
	// t+ = (N+ + 1) / (N+ + 2), t- = 1 / (N- + 2)
	var nPos int
	for _, o := range outcomes {
		if o {
			nPos++
		}
	}
	nNeg := n - nPos
	if nPos == 0 || nNeg == 0 {
		return 0, 0 // degenerate case — all wins or all losses
	}
	tPos := float64(nPos+1) / float64(nPos+2)
	tNeg := 1.0 / float64(nNeg+2)

	targets := make([]float64, n)
	for i, o := range outcomes {
		if o {
			targets[i] = tPos
		} else {
			targets[i] = tNeg
		}
	}

	// Newton-Raphson iteration to minimize negative log-likelihood:
	// L = sum_i [ t_i * log(p_i) + (1 - t_i) * log(1 - p_i) ]
	// where p_i = 1 / (1 + exp(a * x_i + b))
	a, b = 0.0, 0.0
	const maxIter = 8

	for iter := 0; iter < maxIter; iter++ {
		// Compute gradient and Hessian
		var g1, g2 float64        // gradient components
		var h11, h12, h22 float64 // Hessian components

		for i := 0; i < n; i++ {
			x := confidences[i]
			fApB := a*x + b

			// Numerically stable sigmoid
			var p float64
			if fApB >= 0 {
				p = math.Exp(-fApB) / (1 + math.Exp(-fApB))
			} else {
				p = 1.0 / (1 + math.Exp(fApB))
			}

			d := targets[i] - p // residual
			// For Newton's method on the logistic loss:
			// gradient = sum( (t_i - p_i) * x_i ) for a, sum(t_i - p_i) for b
			// Hessian diagonal = -sum( p_i * (1-p_i) * x_i^2 ) etc.
			q := p * (1 - p)
			if q < 1e-12 {
				q = 1e-12
			}

			g1 += d * x
			g2 += d
			h11 += q * x * x
			h12 += q * x
			h22 += q
		}

		// Negate gradient (we minimize negative log-likelihood)
		// The residual-based gradient already has the right sign for descent.
		// Solve 2x2 system: H * delta = g
		det := h11*h22 - h12*h12
		if math.Abs(det) < 1e-20 {
			break // singular Hessian
		}

		da := (h22*g1 - h12*g2) / det
		db := (h11*g2 - h12*g1) / det

		a += da
		b += db

		// Convergence check
		if math.Abs(da) < 1e-10 && math.Abs(db) < 1e-10 {
			break
		}
	}

	return a, b
}

// PlattCalibrate applies fitted Platt scaling to transform a raw confidence
// score into a calibrated probability (0-100 scale).
// rawConfidence is on 0-100 scale, returns calibrated value on 0-100 scale.
func PlattCalibrate(rawConfidence, a, b float64) float64 {
	fApB := a*rawConfidence + b
	var p float64
	if fApB >= 0 {
		p = math.Exp(-fApB) / (1 + math.Exp(-fApB))
	} else {
		p = 1.0 / (1 + math.Exp(fApB))
	}
	return Clamp(p*100, 0, 100)
}

// BrierScore computes the mean squared error between predicted probabilities
// and actual outcomes. predictions are on 0-1 scale, outcomes are true=1, false=0.
// Lower is better: 0 = perfect, 0.25 = random guessing, 1 = worst possible.
// Returns 0 for empty input.
func BrierScore(predictions []float64, outcomes []bool) float64 {
	n := len(predictions)
	if n == 0 || n != len(outcomes) {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		actual := 0.0
		if outcomes[i] {
			actual = 1.0
		}
		d := predictions[i] - actual
		sum += d * d
	}
	return sum / float64(n)
}

// ConsecutiveDirection counts how many consecutive tail elements share
// the same sign direction. data is ordered oldest-first.
// Returns (count, direction) where direction is +1, -1, or 0.
func ConsecutiveDirection(data []float64) (int, float64) {
	if len(data) == 0 {
		return 0, 0
	}
	dir := Sign(data[len(data)-1])
	count := 0
	for i := len(data) - 1; i >= 0; i-- {
		if Sign(data[i]) != dir {
			break
		}
		count++
	}
	return count, dir
}

// ---------------------------------------------------------------------------
// OLS Linear Regression — Normal Equations
// ---------------------------------------------------------------------------

// RegressionResult holds the output of an OLS regression.
type RegressionResult struct {
	Coefficients []float64 // β coefficients (one per predictor column)
	RSquared     float64   // R² — proportion of variance explained
	AdjRSquared  float64   // Adjusted R² — penalizes for number of predictors
	StdErrors    []float64 // Standard error of each coefficient
	TStats       []float64 // t-statistic for each coefficient
	PValues      []float64 // Two-tailed p-value for each coefficient
	Residuals    []float64 // y - X*β
}

// OLSRegression fits a linear model y = Xβ + ε using the normal equations:
//
//	β = (X'X)^(-1) X'y
//
// X is an n×p design matrix (no intercept column is added — include one in X
// if needed). y is the response vector of length n.
// Returns an error if the matrix is singular or if n < p+1.
func OLSRegression(X [][]float64, y []float64) (*RegressionResult, error) {
	n := len(X)
	if n == 0 {
		return nil, fmt.Errorf("ols: no observations")
	}
	p := len(X[0])
	if p == 0 {
		return nil, fmt.Errorf("ols: no predictors")
	}
	if len(y) != n {
		return nil, fmt.Errorf("ols: X has %d rows but y has %d elements", n, len(y))
	}
	if n < p+1 {
		return nil, fmt.Errorf("ols: need at least %d observations for %d predictors, got %d", p+1, p, n)
	}

	// Compute X'X (p×p)
	XtX := matMul(matTranspose(X), X)

	// Compute X'y (p×1)
	Xty := matVecMul(matTranspose(X), y)

	// Invert X'X
	XtXinv, err := matInvert(XtX)
	if err != nil {
		return nil, fmt.Errorf("ols: %w", err)
	}

	// β = (X'X)^(-1) X'y
	beta := matVecMulFlat(XtXinv, Xty)

	// Residuals: e = y - Xβ
	residuals := make([]float64, n)
	var ssRes, ssTot float64
	yMean := Mean(y)
	for i := 0; i < n; i++ {
		yHat := 0.0
		for j := 0; j < p; j++ {
			yHat += X[i][j] * beta[j]
		}
		residuals[i] = y[i] - yHat
		ssRes += residuals[i] * residuals[i]
		d := y[i] - yMean
		ssTot += d * d
	}

	// R² and adjusted R²
	rSquared := 0.0
	if ssTot > 0 {
		rSquared = 1 - ssRes/ssTot
	}
	adjRSquared := 0.0
	if n > p && ssTot > 0 {
		adjRSquared = 1 - (ssRes/float64(n-p))/(ssTot/float64(n-1))
	}

	// Standard errors, t-stats, p-values
	sigmaSquared := ssRes / float64(n-p)
	stdErrors := make([]float64, p)
	tStats := make([]float64, p)
	pValues := make([]float64, p)
	df := float64(n - p)
	for j := 0; j < p; j++ {
		variance := sigmaSquared * XtXinv[j][j]
		if variance > 0 {
			stdErrors[j] = math.Sqrt(variance)
			tStats[j] = beta[j] / stdErrors[j]
			pValues[j] = 2 * tDistCDF(-math.Abs(tStats[j]), df)
		} else {
			stdErrors[j] = 0
			tStats[j] = 0
			pValues[j] = 1
		}
	}

	return &RegressionResult{
		Coefficients: beta,
		RSquared:     rSquared,
		AdjRSquared:  adjRSquared,
		StdErrors:    stdErrors,
		TStats:       tStats,
		PValues:      pValues,
		Residuals:    residuals,
	}, nil
}

// ---------------------------------------------------------------------------
// Small-matrix operations for OLS (handles matrices up to ~10×10)
// ---------------------------------------------------------------------------

// matTranspose returns the transpose of an n×m matrix.
func matTranspose(A [][]float64) [][]float64 {
	if len(A) == 0 {
		return nil
	}
	n, m := len(A), len(A[0])
	T := make([][]float64, m)
	for j := 0; j < m; j++ {
		T[j] = make([]float64, n)
		for i := 0; i < n; i++ {
			T[j][i] = A[i][j]
		}
	}
	return T
}

// matMul multiplies two matrices A (n×m) and B (m×p) returning n×p.
func matMul(A, B [][]float64) [][]float64 {
	n := len(A)
	m := len(A[0])
	p := len(B[0])
	C := make([][]float64, n)
	for i := 0; i < n; i++ {
		C[i] = make([]float64, p)
		for j := 0; j < p; j++ {
			var s float64
			for k := 0; k < m; k++ {
				s += A[i][k] * B[k][j]
			}
			C[i][j] = s
		}
	}
	return C
}

// matVecMul multiplies A (n×m) by vector v (length m) returning length-n result.
func matVecMul(A [][]float64, v []float64) []float64 {
	n := len(A)
	m := len(A[0])
	r := make([]float64, n)
	for i := 0; i < n; i++ {
		var s float64
		for j := 0; j < m; j++ {
			s += A[i][j] * v[j]
		}
		r[i] = s
	}
	return r
}

// matVecMulFlat is an alias for matVecMul (same behaviour).
func matVecMulFlat(A [][]float64, v []float64) []float64 {
	return matVecMul(A, v)
}

// matInvert inverts a square matrix using Gauss-Jordan elimination.
// Returns an error if the matrix is singular.
func matInvert(A [][]float64) ([][]float64, error) {
	n := len(A)
	// Build augmented matrix [A | I]
	aug := make([][]float64, n)
	for i := 0; i < n; i++ {
		aug[i] = make([]float64, 2*n)
		copy(aug[i], A[i])
		aug[i][n+i] = 1.0
	}

	for col := 0; col < n; col++ {
		// Partial pivoting: find row with largest absolute value in column.
		maxVal := math.Abs(aug[col][col])
		maxRow := col
		for row := col + 1; row < n; row++ {
			if math.Abs(aug[row][col]) > maxVal {
				maxVal = math.Abs(aug[row][col])
				maxRow = row
			}
		}
		if maxVal < 1e-14 {
			return nil, fmt.Errorf("singular matrix (pivot %.2e at col %d)", maxVal, col)
		}
		// Swap rows.
		aug[col], aug[maxRow] = aug[maxRow], aug[col]

		// Scale pivot row.
		pivot := aug[col][col]
		for j := 0; j < 2*n; j++ {
			aug[col][j] /= pivot
		}

		// Eliminate column in all other rows.
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := aug[row][col]
			for j := 0; j < 2*n; j++ {
				aug[row][j] -= factor * aug[col][j]
			}
		}
	}

	// Extract inverse from right half.
	inv := make([][]float64, n)
	for i := 0; i < n; i++ {
		inv[i] = make([]float64, n)
		copy(inv[i], aug[i][n:])
	}
	return inv, nil
}
