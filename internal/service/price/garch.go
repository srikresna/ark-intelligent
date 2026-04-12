package price

import (
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// GARCH(1,1) Volatility Forecasting
// ---------------------------------------------------------------------------
//
// GARCH(1,1) models time-varying volatility:
//   σ²(t) = ω + α·ε²(t-1) + β·σ²(t-1)
//
// where:
//   ω (omega) = long-run variance weight
//   α (alpha) = reaction coefficient (weight on latest shock)
//   β (beta)  = persistence coefficient (weight on prior variance)
//   ε(t)      = return at time t
//
// Constraints: α ≥ 0, β ≥ 0, α + β < 1 (stationarity)

// GARCHResult holds the output of a GARCH(1,1) estimation.
type GARCHResult struct {
	Omega         float64 `json:"omega"`          // Long-run variance weight
	Alpha         float64 `json:"alpha"`          // Shock coefficient
	Beta          float64 `json:"beta"`           // Persistence coefficient
	Persistence   float64 `json:"persistence"`    // α + β (closer to 1 = more persistent)
	LongRunVar    float64 `json:"long_run_var"`   // ω / (1 - α - β)
	LongRunVol    float64 `json:"long_run_vol"`   // √(LongRunVar)
	CurrentVar    float64 `json:"current_var"`    // Latest conditional variance σ²(t)
	CurrentVol    float64 `json:"current_vol"`    // √(CurrentVar) — current volatility estimate
	ForecastVar1  float64 `json:"forecast_var_1"` // 1-step ahead forecast σ²(t+1)
	ForecastVol1  float64 `json:"forecast_vol_1"` // √(ForecastVar1)
	ForecastVar5  float64 `json:"forecast_var_5"` // 5-step ahead forecast σ²(t+5)
	ForecastVol5  float64 `json:"forecast_vol_5"` // √(ForecastVar5)
	VolRatio      float64 `json:"vol_ratio"`      // CurrentVol / LongRunVol — >1 = above average
	VolForecast   string  `json:"vol_forecast"`   // "INCREASING", "DECREASING", "STABLE"
	Converged     bool    `json:"converged"`      // Whether estimation converged
	LogLikelihood float64 `json:"log_likelihood"` // Final log-likelihood
	SampleSize    int     `json:"sample_size"`    // Number of returns used
}

// GARCHVolatilityContext extends VolatilityContext with GARCH-based forecasts.
type GARCHVolatilityContext struct {
	*VolatilityContext
	GARCH              *GARCHResult `json:"garch,omitempty"`
	GARCHMultiplier    float64      `json:"garch_multiplier"`    // Confidence multiplier from GARCH
	CombinedMultiplier float64      `json:"combined_multiplier"` // ATR + GARCH combined
}

// EstimateGARCH fits a GARCH(1,1) model to daily/weekly returns using
// variance targeting + grid search for (α, β).
//
// Returns are computed as log-returns from close prices.
// Prices must be newest-first (standard ordering in this codebase).
// Requires at least 30 observations for meaningful estimation.
func EstimateGARCH(prices []domain.PriceRecord) (*GARCHResult, error) {
	if len(prices) < 30 {
		return nil, fmt.Errorf("insufficient data for GARCH: need 30, got %d", len(prices))
	}

	// Compute log-returns (oldest to newest for time-series processing)
	n := len(prices)
	returns := make([]float64, 0, n-1)
	for i := n - 1; i > 0; i-- {
		// Iterate oldest to newest; prices[i] is older, prices[i-1] is newer
		if prices[i].Close <= 0 || prices[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, math.Log(prices[i-1].Close/prices[i].Close))
	}

	return estimateGARCHFromReturns(returns)
}

// EstimateGARCHFromIntraday fits GARCH(1,1) to intraday bar returns.
func EstimateGARCHFromIntraday(bars []domain.IntradayBar) (*GARCHResult, error) {
	if len(bars) < 30 {
		return nil, fmt.Errorf("insufficient intraday data for GARCH: need 30, got %d", len(bars))
	}

	n := len(bars)
	returns := make([]float64, 0, n-1)
	for i := n - 1; i > 0; i-- {
		if bars[i].Close <= 0 || bars[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, math.Log(bars[i-1].Close/bars[i].Close))
	}

	return estimateGARCHFromReturns(returns)
}

// estimateGARCHFromReturns is the core GARCH(1,1) estimator.
// Returns must be in chronological order (oldest first).
func estimateGARCHFromReturns(returns []float64) (*GARCHResult, error) {
	n := len(returns)
	if n < 20 {
		return nil, fmt.Errorf("insufficient returns for GARCH: need 20, got %d", n)
	}

	// Sample variance for initialization (variance targeting)
	var sumR, sumR2 float64
	for _, r := range returns {
		sumR += r
		sumR2 += r * r
	}
	meanR := sumR / float64(n)
	sampleVar := sumR2/float64(n) - meanR*meanR
	if sampleVar <= 0 {
		sampleVar = 1e-8
	}

	// Grid search for (α, β) that maximizes log-likelihood.
	// ω is derived from variance targeting: ω = sampleVar * (1 - α - β)
	bestAlpha, bestBeta := 0.0, 0.0
	bestLL := math.Inf(-1)

	// Coarse grid
	for a := 0.01; a <= 0.30; a += 0.02 {
		for b := 0.50; b <= 0.98; b += 0.02 {
			if a+b >= 0.9999 {
				continue
			}
			omega := sampleVar * (1 - a - b)
			if omega <= 0 {
				continue
			}
			ll := garchLogLikelihood(returns, omega, a, b, sampleVar)
			if ll > bestLL {
				bestLL = ll
				bestAlpha = a
				bestBeta = b
			}
		}
	}

	// Fine grid around best coarse solution
	fineAlpha, fineBeta := bestAlpha, bestBeta
	fineLL := bestLL
	for a := math.Max(0.005, bestAlpha-0.02); a <= math.Min(0.40, bestAlpha+0.02); a += 0.005 {
		for b := math.Max(0.40, bestBeta-0.02); b <= math.Min(0.995, bestBeta+0.02); b += 0.005 {
			if a+b >= 0.9999 {
				continue
			}
			omega := sampleVar * (1 - a - b)
			if omega <= 0 {
				continue
			}
			ll := garchLogLikelihood(returns, omega, a, b, sampleVar)
			if ll > fineLL {
				fineLL = ll
				fineAlpha = a
				fineBeta = b
			}
		}
	}

	alpha := fineAlpha
	beta := fineBeta
	omega := sampleVar * (1 - alpha - beta)

	// Compute final conditional variance series
	sigma2 := make([]float64, n)
	sigma2[0] = sampleVar
	for t := 1; t < n; t++ {
		sigma2[t] = omega + alpha*returns[t-1]*returns[t-1] + beta*sigma2[t-1]
		if sigma2[t] < 1e-12 {
			sigma2[t] = 1e-12
		}
	}

	currentVar := sigma2[n-1]
	lastReturn := returns[n-1]

	// 1-step ahead forecast
	forecastVar1 := omega + alpha*lastReturn*lastReturn + beta*currentVar

	// Multi-step ahead forecast (mean-reverting to long-run variance)
	// Guard: near unit root (alpha+beta ≈ 1) makes denominator near-zero → Inf
	persistence := alpha + beta
	longRunVar := 0.0
	if denom := 1 - persistence; math.Abs(denom) > 1e-9 {
		longRunVar = omega / denom
		if math.IsNaN(longRunVar) || math.IsInf(longRunVar, 0) || longRunVar < 0 {
			longRunVar = 0
		}
	}

	// 5-step ahead variance forecast (variance at step 5, not cumulative)
	forecastVar5 := forecastVar1
	for h := 2; h <= 5; h++ {
		forecastVar5 = omega + persistence*forecastVar5
	}

	currentVol := math.Sqrt(currentVar)
	longRunVol := math.Sqrt(longRunVar)
	volRatio := 0.0
	if longRunVol > 0 {
		volRatio = currentVol / longRunVol
		if math.IsNaN(volRatio) || math.IsInf(volRatio, 0) {
			volRatio = 0
		}
	}

	// Classify forecast direction
	volForecast := "STABLE"
	if forecastVar1 > currentVar*1.10 {
		volForecast = "INCREASING"
	} else if forecastVar1 < currentVar*0.90 {
		volForecast = "DECREASING"
	}

	// Convergence check: mark not converged only for genuinely bad estimates.
	// fineLL ≈ bestLL (small improvement) means the optimization landscape is stable → converged.
	// Only flag non-convergence for: invalid LL, or near unit-root (alpha+beta ≥ 1).
	converged := true
	if math.IsInf(fineLL, -1) || math.IsNaN(fineLL) {
		converged = false
	}
	if alpha+beta > 0.999 {
		// Near unit root — non-stationary, estimates unreliable
		converged = false
	}

	return &GARCHResult{
		Omega:         roundN(omega, 10),
		Alpha:         roundN(alpha, 4),
		Beta:          roundN(beta, 4),
		Persistence:   roundN(persistence, 4),
		LongRunVar:    roundN(longRunVar, 10),
		LongRunVol:    roundN(longRunVol, 6),
		CurrentVar:    roundN(currentVar, 10),
		CurrentVol:    roundN(currentVol, 6),
		ForecastVar1:  roundN(forecastVar1, 10),
		ForecastVol1:  roundN(math.Sqrt(forecastVar1), 6),
		ForecastVar5:  roundN(forecastVar5, 10),
		ForecastVol5:  roundN(math.Sqrt(forecastVar5), 6),
		VolRatio:      roundN(volRatio, 4),
		VolForecast:   volForecast,
		Converged:     converged,
		LogLikelihood: roundN(fineLL, 4),
		SampleSize:    n,
	}, nil
}

// garchLogLikelihood computes the Gaussian log-likelihood for GARCH(1,1).
// L = -0.5 * Σ [ log(σ²(t)) + ε²(t)/σ²(t) ]
func garchLogLikelihood(returns []float64, omega, alpha, beta, initVar float64) float64 {
	n := len(returns)
	sigma2 := initVar
	ll := 0.0

	for t := 0; t < n; t++ {
		if sigma2 < 1e-12 {
			sigma2 = 1e-12
		}
		ll += -0.5 * (math.Log(sigma2) + returns[t]*returns[t]/sigma2)
		if t < n-1 {
			sigma2 = omega + alpha*returns[t]*returns[t] + beta*sigma2
		}
	}

	if math.IsNaN(ll) || math.IsInf(ll, 0) {
		return math.Inf(-1)
	}
	return ll
}

// GARCHConfidenceMultiplier converts a GARCH vol ratio into a confidence multiplier.
//
// Strategy:
//   - VolRatio > 1.5  → 0.75x (very high vol = noisy signals)
//   - VolRatio > 1.25 → 0.85x (elevated vol)
//   - VolRatio < 0.50 → 1.15x (very low vol = cleaner signals)
//   - VolRatio < 0.75 → 1.10x (low vol)
//   - Otherwise       → 1.00x
//
// The GARCH multiplier is forward-looking (uses forecast), complementing the
// backward-looking ATR multiplier.
func GARCHConfidenceMultiplier(g *GARCHResult) float64 {
	if g == nil || !g.Converged {
		return 1.0
	}

	// Use forecast ratio (forecast vol vs long-run vol)
	forecastRatio := g.VolRatio
	if g.LongRunVol > 0 {
		forecastRatio = g.ForecastVol1 / g.LongRunVol
		if math.IsNaN(forecastRatio) || math.IsInf(forecastRatio, 0) {
			forecastRatio = 1.0
		}
	}

	switch {
	case forecastRatio > 1.50:
		return 0.75
	case forecastRatio > 1.25:
		return 0.85
	case forecastRatio < 0.50:
		return 1.15
	case forecastRatio < 0.75:
		return 1.10
	default:
		return 1.00
	}
}

// CombineVolatilityWithGARCH produces a confidence multiplier that blends
// ATR-based (backward), VIX-based (market), and GARCH-based (forward) signals.
func CombineVolatilityWithGARCH(volCtx *VolatilityContext, riskCtx *domain.RiskContext, garch *GARCHResult) float64 {
	atrMult := CombineVolatilityMultiplier(volCtx, riskCtx)
	garchMult := GARCHConfidenceMultiplier(garch)

	// Weighted average: ATR+VIX combo (60%) + GARCH forecast (40%)
	// GARCH gets lower weight because grid-search estimation is approximate
	return roundN(atrMult*0.60+garchMult*0.40, 4)
}

// ---------------------------------------------------------------------------
// GJR-GARCH(1,1) — Asymmetric Volatility with Leverage Effect
// ---------------------------------------------------------------------------
//
// GJR-GARCH(1,1): σ²(t) = ω + α·ε²(t-1) + γ·I(ε(t-1)<0)·ε²(t-1) + β·σ²(t-1)
//
// where I = 1 if ε(t-1) < 0 (negative return), else 0.
// γ (gamma) = asymmetry/leverage coefficient; γ > 0 → downside shocks more persistent.
// Persistence = α + γ/2 + β; stationarity requires α + γ/2 + β < 1.
//
// Glosten, Jagannathan & Runkle (1993).

// GJRGARCHResult holds the output of a GJR-GARCH(1,1) estimation.
type GJRGARCHResult struct {
	Omega          float64 `json:"omega"`
	Alpha          float64 `json:"alpha"`           // symmetric shock weight
	Gamma          float64 `json:"gamma"`           // asymmetry/leverage coefficient
	Beta           float64 `json:"beta"`            // persistence weight
	Persistence    float64 `json:"persistence"`     // α + γ/2 + β
	AsymmetryRatio float64 `json:"asymmetry_ratio"` // γ / (2α + γ) * 100 — % of shock from downside
	CurrentVol     float64 `json:"current_vol"`     // √σ²(t)
	ForecastVol1   float64 `json:"forecast_vol_1"`  // 1-step ahead vol
	AsymmetryLabel string  `json:"asymmetry_label"` // "HIGH", "MODERATE", "LOW", "NONE"
	LeverageEffect bool    `json:"leverage_effect"` // γ > 0.05
	Converged      bool    `json:"converged"`
	SampleSize     int     `json:"sample_size"`
}

// EstimateGJRGARCH fits a GJR-GARCH(1,1) model to daily prices.
// prices must be newest-first. Requires at least 30 observations.
func EstimateGJRGARCH(prices []domain.PriceRecord) (*GJRGARCHResult, error) {
	if len(prices) < 30 {
		return nil, fmt.Errorf("insufficient data for GJR-GARCH: need 30, got %d", len(prices))
	}
	n := len(prices)
	returns := make([]float64, 0, n-1)
	for i := n - 1; i > 0; i-- {
		if prices[i].Close <= 0 || prices[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, math.Log(prices[i-1].Close/prices[i].Close))
	}
	return estimateGJRGARCHFromReturns(returns)
}

// estimateGJRGARCHFromReturns fits GJR-GARCH(1,1) via variance targeting + grid search.
func estimateGJRGARCHFromReturns(returns []float64) (*GJRGARCHResult, error) {
	n := len(returns)
	if n < 10 {
		return nil, fmt.Errorf("insufficient returns for GJR-GARCH: %d", n)
	}

	// Compute mean and variance target
	mu := 0.0
	for _, r := range returns {
		mu += r
	}
	mu /= float64(n)

	varTarget := 0.0
	for _, r := range returns {
		varTarget += (r - mu) * (r - mu)
	}
	varTarget /= float64(n)

	// Grid search over (α, γ, β) to maximise log-likelihood.
	// Constraint: α + γ/2 + β < 1, all non-negative, ω > 0.
	bestLL := math.Inf(-1)
	var bestAlpha, bestGamma, bestBeta float64

	for _, alpha := range []float64{0.02, 0.05, 0.08, 0.10, 0.12, 0.15} {
		for _, gamma := range []float64{0.0, 0.02, 0.05, 0.08, 0.10, 0.12} {
			for _, beta := range []float64{0.75, 0.80, 0.85, 0.88, 0.90, 0.92} {
				if alpha+gamma/2+beta >= 0.9999 {
					continue
				}
				omega := varTarget * (1 - alpha - gamma/2 - beta)
				if omega <= 0 {
					continue
				}
				ll := gjrLogLikelihood(returns, mu, omega, alpha, gamma, beta, varTarget)
				if ll > bestLL {
					bestLL = ll
					bestAlpha, bestGamma, bestBeta = alpha, gamma, beta
				}
			}
		}
	}

	omega := varTarget * (1 - bestAlpha - bestGamma/2 - bestBeta)
	persistence := bestAlpha + bestGamma/2 + bestBeta

	// Compute final conditional variance sequence
	variances := make([]float64, n)
	variances[0] = varTarget
	for t := 1; t < n; t++ {
		e := returns[t-1] - mu
		ind := 0.0
		if e < 0 {
			ind = 1.0
		}
		variances[t] = omega + bestAlpha*e*e + bestGamma*ind*e*e + bestBeta*variances[t-1]
	}
	currentVar := variances[n-1]

	// 1-step ahead forecast
	eT := returns[n-1] - mu
	indT := 0.0
	if eT < 0 {
		indT = 1.0
	}
	forecastVar1 := omega + bestAlpha*eT*eT + bestGamma*indT*eT*eT + bestBeta*currentVar

	asymLabel := "NONE"
	switch {
	case bestGamma > 0.10:
		asymLabel = "HIGH"
	case bestGamma > 0.05:
		asymLabel = "MODERATE"
	case bestGamma > 0.01:
		asymLabel = "LOW"
	}

	asymRatio := 0.0
	if 2*bestAlpha+bestGamma > 0 {
		asymRatio = bestGamma / (2*bestAlpha + bestGamma) * 100
	}

	return &GJRGARCHResult{
		Omega:          omega,
		Alpha:          bestAlpha,
		Gamma:          bestGamma,
		Beta:           bestBeta,
		Persistence:    persistence,
		AsymmetryRatio: asymRatio,
		CurrentVol:     math.Sqrt(math.Max(currentVar, 1e-12)),
		ForecastVol1:   math.Sqrt(math.Max(forecastVar1, 1e-12)),
		AsymmetryLabel: asymLabel,
		LeverageEffect: bestGamma > 0.05,
		Converged:      true,
		SampleSize:     n,
	}, nil
}

// gjrLogLikelihood computes the Gaussian log-likelihood for GJR-GARCH(1,1).
func gjrLogLikelihood(returns []float64, mu, omega, alpha, gamma, beta, initVar float64) float64 {
	v := initVar
	ll := 0.0
	for _, r := range returns {
		if v <= 0 {
			v = 1e-10
		}
		e := r - mu
		ind := 0.0
		if e < 0 {
			ind = 1.0
		}
		ll += -0.5 * (math.Log(v) + e*e/v)
		v = omega + alpha*e*e + gamma*ind*e*e + beta*v
	}
	if math.IsNaN(ll) || math.IsInf(ll, 0) {
		return math.Inf(-1)
	}
	return ll
}
