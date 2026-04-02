package price

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Monte Carlo Scenario Generator
// ---------------------------------------------------------------------------
//
// Generates probabilistic price paths using GARCH(1,1) volatility + HMM
// regime-conditional drift. Produces percentile-based price distributions
// and Value-at-Risk estimates for a given forecast horizon.
//
// Algorithm:
//   1. Fit GARCH(1,1) → current σ² estimate + mean-reversion dynamics
//   2. Get HMM regime → drift parameter per regime
//   3. Generate N GBM paths with time-varying GARCH volatility:
//        r(t) = drift_regime + σ_garch(t) * z(t),  z(t) ~ N(0,1)
//        σ²(t+1) = ω + α·r(t)² + β·σ²(t)
//   4. Compute percentile distribution at horizon

const (
	defaultNumPaths   = 1000
	defaultHorizonDay = 30
	minPricesForMC    = 60 // need enough history for GARCH + HMM
	maxHorizonDays    = 90
)

// regimeDrift maps HMM state to annualised drift (log-return per year).
// These are conservative estimates; actual drift comes from historical mean
// within each regime when available.
var regimeDrift = map[string]float64{
	HMMRiskOn:  0.06,  // ~6% annualised in risk-on
	HMMRiskOff: 0.00,  // flat in risk-off
	HMMCrisis:  -0.12, // ~-12% annualised in crisis
}

// ScenarioResult holds the output of a Monte Carlo scenario simulation.
type ScenarioResult struct {
	Symbol       string              `json:"symbol"`
	CurrentPrice float64             `json:"current_price"`
	HorizonDays  int                 `json:"horizon_days"`
	NumPaths     int                 `json:"num_paths"`
	Percentiles  []PricePercentile   `json:"percentiles"`
	VaR95        float64             `json:"var_95"`         // 5th percentile loss (%)
	VaR99        float64             `json:"var_99"`         // 1st percentile loss (%)
	CVaR95       float64             `json:"cvar_95"`        // Expected shortfall below VaR95
	MeanReturn   float64             `json:"mean_return"`    // Mean return across all paths (%)
	Regime       string              `json:"regime"`         // Current HMM regime used
	VolEstimate  float64             `json:"vol_estimate"`   // GARCH current vol (annualised)
	GeneratedAt  time.Time           `json:"generated_at"`
}

// PricePercentile holds a price level at a given percentile.
type PricePercentile struct {
	Percentile float64 `json:"percentile"` // 0.05, 0.25, 0.50, 0.75, 0.95
	Price      float64 `json:"price"`
	Return     float64 `json:"return_pct"` // % change from current price
}

// ScenarioConfig allows callers to override defaults.
type ScenarioConfig struct {
	NumPaths    int
	HorizonDays int
}

// GenerateScenario runs Monte Carlo price simulation using GARCH vol + HMM drift.
// Prices must be newest-first (standard ordering).
func GenerateScenario(prices []domain.PriceRecord, symbol string, cfg *ScenarioConfig) (*ScenarioResult, error) {
	if len(prices) < minPricesForMC {
		return nil, fmt.Errorf("need at least %d price observations, got %d", minPricesForMC, len(prices))
	}

	numPaths := defaultNumPaths
	horizon := defaultHorizonDay
	if cfg != nil {
		if cfg.NumPaths > 0 {
			numPaths = cfg.NumPaths
		}
		if cfg.HorizonDays > 0 && cfg.HorizonDays <= maxHorizonDays {
			horizon = cfg.HorizonDays
		}
	}

	currentPrice := prices[0].Close
	if currentPrice <= 0 {
		return nil, fmt.Errorf("invalid current price: %.6f", currentPrice)
	}

	// 1. Fit GARCH(1,1) for volatility dynamics
	garch, err := EstimateGARCH(prices)
	if err != nil {
		return nil, fmt.Errorf("GARCH estimation failed: %w", err)
	}

	// 2. Fit HMM for regime-conditional drift
	hmm, err := EstimateHMMRegime(prices)
	if err != nil {
		return nil, fmt.Errorf("HMM estimation failed: %w", err)
	}

	// Daily drift from regime (convert annualised to daily)
	dailyDrift := regimeDrift[hmm.CurrentState] / 252.0

	// If we have historical mean return for this regime, use it instead
	historicalDrift := computeRegimeDrift(prices, hmm)
	if !math.IsNaN(historicalDrift) && math.Abs(historicalDrift) < 0.01 {
		dailyDrift = historicalDrift
	}

	// GARCH parameters for variance evolution
	omega := garch.Omega
	alpha := garch.Alpha
	beta := garch.Beta
	currentVar := garch.CurrentVar

	// Guard: if GARCH didn't converge or params are degenerate, use long-run vol
	if !garch.Converged || currentVar <= 0 {
		currentVar = garch.LongRunVar
	}
	if currentVar <= 0 {
		// Fallback: simple historical vol
		returns := logReturnsFromPrices(prices)
		currentVar = sampleVariance(returns)
	}
	if currentVar <= 0 {
		return nil, fmt.Errorf("unable to estimate volatility")
	}

	// 3. Generate N paths
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	terminalPrices := make([]float64, numPaths)

	for i := 0; i < numPaths; i++ {
		price := currentPrice
		variance := currentVar

		for t := 0; t < horizon; t++ {
			z := rng.NormFloat64()
			vol := math.Sqrt(variance)
			ret := dailyDrift + vol*z

			// GBM step
			price *= math.Exp(ret)

			// GARCH variance update: σ²(t+1) = ω + α·r² + β·σ²(t)
			variance = omega + alpha*ret*ret + beta*variance

			// Floor variance to prevent collapse
			if variance < 1e-12 {
				variance = 1e-12
			}
		}

		terminalPrices[i] = price
	}

	// 4. Compute percentiles
	sort.Float64s(terminalPrices)

	pctLevels := []float64{0.01, 0.05, 0.10, 0.25, 0.50, 0.75, 0.90, 0.95, 0.99}
	percentiles := make([]PricePercentile, len(pctLevels))
	for i, p := range pctLevels {
		idx := int(math.Floor(p * float64(numPaths)))
		if idx >= numPaths {
			idx = numPaths - 1
		}
		pPrice := terminalPrices[idx]
		percentiles[i] = PricePercentile{
			Percentile: p,
			Price:      pPrice,
			Return:     (pPrice/currentPrice - 1) * 100,
		}
	}

	// 5. VaR and CVaR
	var95Price := terminalPrices[int(math.Floor(0.05*float64(numPaths)))]
	var99Price := terminalPrices[int(math.Floor(0.01*float64(numPaths)))]

	// CVaR95 = expected value of losses below VaR95
	var95Idx := int(math.Floor(0.05 * float64(numPaths)))
	cvarSum := 0.0
	for i := 0; i < var95Idx; i++ {
		cvarSum += (terminalPrices[i]/currentPrice - 1)
	}
	cvar95 := 0.0
	if var95Idx > 0 {
		cvar95 = (cvarSum / float64(var95Idx)) * 100
	}

	// Mean return
	totalReturn := 0.0
	for _, p := range terminalPrices {
		totalReturn += (p/currentPrice - 1)
	}
	meanReturn := (totalReturn / float64(numPaths)) * 100

	return &ScenarioResult{
		Symbol:       symbol,
		CurrentPrice: currentPrice,
		HorizonDays:  horizon,
		NumPaths:     numPaths,
		Percentiles:  percentiles,
		VaR95:        (var95Price/currentPrice - 1) * 100,
		VaR99:        (var99Price/currentPrice - 1) * 100,
		CVaR95:       cvar95,
		MeanReturn:   meanReturn,
		Regime:       hmm.CurrentState,
		VolEstimate:  garch.CurrentVol * math.Sqrt(252), // annualise
		GeneratedAt:  time.Now(),
	}, nil
}

// computeRegimeDrift computes mean daily return from the most recent stretch of the
// current regime (using Viterbi path).
func computeRegimeDrift(prices []domain.PriceRecord, hmm *HMMResult) float64 {
	if len(hmm.ViterbiPath) == 0 || len(prices) < 2 {
		return math.NaN()
	}

	target := hmm.CurrentState
	returns := logReturnsFromPrices(prices)

	// ViterbiPath is newest-first; returns is newest-first (returns[0] = most recent)
	var sum float64
	var count int
	maxLookback := len(returns)
	if maxLookback > len(hmm.ViterbiPath) {
		maxLookback = len(hmm.ViterbiPath)
	}

	for i := 0; i < maxLookback; i++ {
		if hmm.ViterbiPath[i] != target {
			break // stop at first regime change
		}
		sum += returns[i]
		count++
	}

	if count < 5 {
		return math.NaN()
	}
	return sum / float64(count)
}

// logReturnsFromPrices computes log-returns from newest-first prices.
// Returns are also newest-first.
func logReturnsFromPrices(prices []domain.PriceRecord) []float64 {
	if len(prices) < 2 {
		return nil
	}
	returns := make([]float64, len(prices)-1)
	for i := 0; i < len(prices)-1; i++ {
		if prices[i+1].Close > 0 {
			returns[i] = math.Log(prices[i].Close / prices[i+1].Close)
		}
	}
	return returns
}

// sampleVariance computes the sample variance of a float64 slice.
func sampleVariance(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	var sum, sumSq float64
	n := float64(len(data))
	for _, v := range data {
		sum += v
		sumSq += v * v
	}
	mean := sum / n
	return (sumSq/n - mean*mean)
}
