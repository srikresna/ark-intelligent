package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// ---------------------------------------------------------------------------
// Factor Weight Optimization — OLS regression on signal outcomes
// ---------------------------------------------------------------------------

// Factor names used as map keys.
const (
	FactorCOT    = "COT"
	FactorStress = "Stress"
	FactorFRED   = "FRED"
	FactorPrice  = "Price"
)

// factorNames is the canonical order of factors in the regression.
// NOTE: Calendar is excluded because PersistedSignal does not store calendar
// surprise data, so the column would be all-zeros (making X'X singular).
var factorNames = []string{FactorCOT, FactorStress, FactorFRED, FactorPrice}

// currentV3Weights are the hardcoded V3 weights (4-factor model).
// Calendar is excluded from the regression (not stored on PersistedSignal).
// The original V3 Calendar weight (15%) is redistributed proportionally.
var currentV3Weights = map[string]float64{
	FactorCOT:   29, // 25 / 85 * 100 ≈ 29
	FactorStress: 12, // 10 / 85 * 100 ≈ 12
	FactorFRED:  24, // 20 / 85 * 100 ≈ 24
	FactorPrice: 35, // 30 / 85 * 100 ≈ 35
}

// WeightResult holds the output of the weight optimization analysis.
type WeightResult struct {
	// OptimizedWeights maps factor name to optimized weight percentage (sums to 100%).
	OptimizedWeights map[string]float64

	// CurrentWeights maps factor name to current hardcoded weight percentage.
	CurrentWeights map[string]float64

	// RSquared is the proportion of return variance explained by the factors.
	RSquared float64

	// AdjRSquared penalizes for number of predictors.
	AdjRSquared float64

	// FactorSignificance maps factor name to whether it is statistically significant (p < 0.05).
	FactorSignificance map[string]bool

	// FactorPValues maps factor name to its p-value.
	FactorPValues map[string]float64

	// FactorCoefficients maps factor name to its raw OLS coefficient.
	FactorCoefficients map[string]float64

	// PerContractWeights maps contract currency to per-currency optimal weights.
	// Only populated when a currency has >= 10 evaluated signals.
	PerContractWeights map[string]map[string]float64

	// SampleSize is the number of signals used in the regression.
	SampleSize int

	// Recommendation is a human-readable summary.
	Recommendation string
}

// WeightOptimizer uses OLS regression to determine data-driven factor weights.
type WeightOptimizer struct {
	signalRepo ports.SignalRepository
}

// NewWeightOptimizer creates a new WeightOptimizer.
func NewWeightOptimizer(signalRepo ports.SignalRepository) *WeightOptimizer {
	return &WeightOptimizer{signalRepo: signalRepo}
}

// OptimizeWeights runs OLS regression: Return1W ~ COT + Calendar + Stress + FRED + Price.
// Factor scores are reconstructed from the persisted signal metadata.
func (wo *WeightOptimizer) OptimizeWeights(ctx context.Context) (*WeightResult, error) {
	signals, err := wo.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals for weight optimization: %w", err)
	}

	// Keep only signals with a 1W outcome and non-zero entry price.
	var evaluated []domain.PersistedSignal
	for i := range signals {
		if signals[i].Outcome1W != "" && signals[i].Outcome1W != domain.OutcomePending && signals[i].EntryPrice != 0 {
			evaluated = append(evaluated, signals[i])
		}
	}

	minRequired := len(factorNames) + 1 // p+1
	if len(evaluated) < minRequired {
		return &WeightResult{
			CurrentWeights: currentV3Weights,
			SampleSize:     len(evaluated),
			Recommendation: fmt.Sprintf("Insufficient data: need at least %d evaluated signals, have %d. Using hardcoded V3 weights.", minRequired, len(evaluated)),
		}, nil
	}

	// Build design matrix X and response vector y.
	X, y := buildDesignMatrix(evaluated)

	// Run global OLS regression.
	result, err := runWeightRegression(X, y, evaluated)
	if err != nil {
		return &WeightResult{
			CurrentWeights: currentV3Weights,
			SampleSize:     len(evaluated),
			Recommendation: fmt.Sprintf("Regression failed: %s. Using hardcoded V3 weights.", err.Error()),
		}, nil
	}

	// Per-contract regressions.
	result.PerContractWeights = wo.perContractWeights(evaluated)

	return result, nil
}

// buildDesignMatrix constructs the feature matrix and response vector from signals.
// Each signal contributes one row with 5 factor scores and the 1W return as response.
func buildDesignMatrix(signals []domain.PersistedSignal) (X [][]float64, y []float64) {
	X = make([][]float64, 0, len(signals))
	y = make([]float64, 0, len(signals))

	for _, s := range signals {
		row := extractFactorScores(s)
		X = append(X, row)
		y = append(y, s.Return1W)
	}
	return X, y
}

// extractFactorScores reconstructs the 4 factor scores for a signal from its metadata.
// Returns [COT, Stress, FRED, Price] each normalized to [-1, +1].
// Calendar is excluded because PersistedSignal does not store surprise data.
func extractFactorScores(s domain.PersistedSignal) []float64 {
	// 1. COT score: prefer SentimentScore if populated; otherwise derive from
	//    COTIndex (0-100, always stored by bootstrap and scheduler).
	//    Re-center COTIndex to -100..+100: (index - 50) * 2.
	cotScore := s.SentimentScore
	if cotScore == 0 && s.COTIndex != 0 {
		cotScore = (s.COTIndex - 50) * 2
	}
	cotScore = mathutil.Clamp(cotScore, -100, 100)

	// 2. Financial stress: approximate from FRED regime label.
	// STRESS/RECESSION regimes imply high stress (bearish risk), others neutral/positive.
	stressScore := fredRegimeToStressScore(s.FREDRegime)

	// 3. FRED regime score: map regime label to a numeric score.
	fredScore := fredRegimeToScore(s.FREDRegime)

	// 4. Price momentum: derive from signal direction and conviction.
	//    Prefer ConvictionScore if populated; otherwise derive from
	//    Confidence (0-100, always stored) + Direction as a proxy.
	priceScore := 0.0
	if s.ConvictionScore > 0 {
		// ConvictionScore is 0-100 where 50 = neutral.
		priceScore = mathutil.Clamp((s.ConvictionScore-50)*2, -100, 100)
		// Adjust sign based on direction.
		if s.Direction == "BEARISH" {
			priceScore = -math.Abs(priceScore)
		} else if s.Direction == "BULLISH" {
			priceScore = math.Abs(priceScore)
		}
	} else if s.Confidence > 0 {
		// Fallback: use Confidence as magnitude, Direction as sign.
		// Confidence is 0-100; re-center to -100..+100.
		priceScore = mathutil.Clamp((s.Confidence-50)*2, -100, 100)
		if s.Direction == "BEARISH" {
			priceScore = -math.Abs(priceScore)
		} else if s.Direction == "BULLISH" {
			priceScore = math.Abs(priceScore)
		}
	}

	// Normalize all to [-1, +1] for regression stability.
	return []float64{
		cotScore / 100.0,
		stressScore / 100.0,
		fredScore / 100.0,
		priceScore / 100.0,
	}
}

// fredRegimeToStressScore maps a FRED regime label to a stress score (-100 to +100).
func fredRegimeToStressScore(regime string) float64 {
	switch regime {
	case "STRESS":
		return -80
	case "RECESSION":
		return -60
	case "STAGFLATION":
		return -40
	case "INFLATIONARY":
		return -20
	case "DISINFLATIONARY":
		return 20
	case "GOLDILOCKS":
		return 60
	default:
		return 0
	}
}

// fredRegimeToScore maps a FRED regime label to a macro favorability score (-100 to +100).
func fredRegimeToScore(regime string) float64 {
	switch regime {
	case "GOLDILOCKS":
		return 80
	case "DISINFLATIONARY":
		return 40
	case "INFLATIONARY":
		return -20
	case "STAGFLATION":
		return -60
	case "RECESSION":
		return -80
	case "STRESS":
		return -100
	default:
		return 0
	}
}

// runWeightRegression executes OLS and builds a WeightResult.
func runWeightRegression(X [][]float64, y []float64, signals []domain.PersistedSignal) (*WeightResult, error) {
	reg, err := mathutil.OLSRegression(X, y)
	if err != nil {
		return nil, err
	}

	// Convert absolute coefficients to normalized weights (% summing to 100).
	optimized := normalizeCoefficients(reg.Coefficients)

	// Build significance and p-value maps.
	significance := make(map[string]bool, len(factorNames))
	pValues := make(map[string]float64, len(factorNames))
	coefficients := make(map[string]float64, len(factorNames))
	for i, name := range factorNames {
		significance[name] = reg.PValues[i] < 0.05
		pValues[name] = math.Round(reg.PValues[i]*10000) / 10000
		coefficients[name] = math.Round(reg.Coefficients[i]*10000) / 10000
	}

	recommendation := buildWeightRecommendation(optimized, currentV3Weights, significance, reg.RSquared, len(signals))

	return &WeightResult{
		OptimizedWeights:   optimized,
		CurrentWeights:     currentV3Weights,
		RSquared:           math.Round(reg.RSquared*10000) / 10000,
		AdjRSquared:        math.Round(reg.AdjRSquared*10000) / 10000,
		FactorSignificance: significance,
		FactorPValues:      pValues,
		FactorCoefficients: coefficients,
		SampleSize:         len(signals),
		Recommendation:     recommendation,
	}, nil
}

// normalizeCoefficients converts raw OLS coefficients to percentage weights.
// Uses absolute values of coefficients (magnitude = importance) and normalizes to sum to 100.
func normalizeCoefficients(coefficients []float64) map[string]float64 {
	absSum := 0.0
	for _, c := range coefficients {
		absSum += math.Abs(c)
	}

	weights := make(map[string]float64, len(factorNames))
	if absSum == 0 {
		// Equal weights if all coefficients are zero.
		w := 100.0 / float64(len(factorNames))
		for _, name := range factorNames {
			weights[name] = math.Round(w*100) / 100
		}
		return weights
	}

	for i, name := range factorNames {
		weights[name] = math.Round(math.Abs(coefficients[i])/absSum*10000) / 100
	}
	return weights
}

// perContractWeights runs per-currency regressions for currencies with enough data.
func (wo *WeightOptimizer) perContractWeights(signals []domain.PersistedSignal) map[string]map[string]float64 {
	// Group by currency.
	grouped := make(map[string][]domain.PersistedSignal)
	for _, s := range signals {
		if s.Currency != "" {
			grouped[s.Currency] = append(grouped[s.Currency], s)
		}
	}

	result := make(map[string]map[string]float64)
	minPerContract := len(factorNames) + 5 // need more than p+1 for meaningful per-currency results

	for currency, sigs := range grouped {
		if len(sigs) < minPerContract {
			continue
		}
		X, y := buildDesignMatrix(sigs)
		reg, err := mathutil.OLSRegression(X, y)
		if err != nil {
			continue
		}
		result[currency] = normalizeCoefficients(reg.Coefficients)
	}

	return result
}

// buildWeightRecommendation generates a human-readable recommendation.
func buildWeightRecommendation(
	optimized, current map[string]float64,
	significance map[string]bool,
	rSquared float64,
	sampleSize int,
) string {
	// Count significant factors.
	sigCount := 0
	var sigFactors []string
	for _, name := range factorNames {
		if significance[name] {
			sigCount++
			sigFactors = append(sigFactors, name)
		}
	}

	// Find biggest weight change.
	maxDelta := 0.0
	maxDeltaFactor := ""
	for _, name := range factorNames {
		delta := math.Abs(optimized[name] - current[name])
		if delta > maxDelta {
			maxDelta = delta
			maxDeltaFactor = name
		}
	}

	if sampleSize < 30 {
		return fmt.Sprintf(
			"Small sample (%d signals). Results are directional only. "+
				"Recommend waiting for 30+ evaluated signals before adjusting weights.",
			sampleSize,
		)
	}

	if rSquared < 0.01 {
		return "R-squared near zero: factor scores explain very little return variance. " +
			"Current hardcoded weights may be as good as any. Consider adding more predictive features."
	}

	if sigCount == 0 {
		return fmt.Sprintf(
			"No factors are statistically significant (n=%d, R²=%.3f). "+
				"Hardcoded weights remain reasonable. Monitor as sample size grows.",
			sampleSize, rSquared,
		)
	}

	sort.Strings(sigFactors)
	return fmt.Sprintf(
		"Significant factors: %s (n=%d, R²=%.3f). "+
			"Largest weight shift: %s (%.0f%% -> %.0f%%). "+
			"Consider applying optimized weights when R² > 0.05 and n > 50.",
		joinStrings(sigFactors), sampleSize, rSquared,
		maxDeltaFactor, current[maxDeltaFactor], optimized[maxDeltaFactor],
	)
}

// joinStrings joins a slice with commas.
func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return "none"
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += ", " + s
	}
	return result
}
