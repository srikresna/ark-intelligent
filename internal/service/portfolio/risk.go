// Package portfolio provides portfolio risk analysis including cross-correlation
// analysis, concentration scoring, and regime-aware risk classification.
package portfolio

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

var log = logger.Component("portfolio")

// ---------------------------------------------------------------------------
// Correlation Matrix
// ---------------------------------------------------------------------------

// ComputeCorrelationMatrix computes the Pearson correlation of weekly returns
// between all pairs of currencies present in priceHistory.
// priceHistory maps currency code -> newest-first price records.
func ComputeCorrelationMatrix(priceHistory map[string][]domain.PriceRecord) map[string]map[string]float64 {
	// Compute weekly returns for each currency.
	returns := make(map[string][]float64)
	for currency, records := range priceHistory {
		if len(records) < 2 {
			continue
		}
		// records are newest-first; compute returns oldest->newest for alignment.
		var ret []float64
		for i := len(records) - 1; i > 0; i-- {
			if records[i].Close == 0 {
				continue
			}
			r := (records[i-1].Close - records[i].Close) / records[i].Close * 100
			ret = append(ret, r)
		}
		if len(ret) >= 4 { // Require at least 4 data points for meaningful correlation
			returns[currency] = ret
		}
	}

	// Build correlation matrix.
	matrix := make(map[string]map[string]float64)
	currencies := make([]string, 0, len(returns))
	for c := range returns {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)

	for _, c1 := range currencies {
		matrix[c1] = make(map[string]float64)
		for _, c2 := range currencies {
			if c1 == c2 {
				matrix[c1][c2] = 1.0
				continue
			}
			corr := pearsonCorrelation(returns[c1], returns[c2])
			matrix[c1][c2] = corr
		}
	}

	return matrix
}

// pearsonCorrelation computes the Pearson correlation coefficient between
// two return series. Uses the minimum overlapping length.
func pearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if len(y) < n {
		n = len(y)
	}
	if n < 4 {
		return 0
	}

	// Trim to overlapping length.
	x = x[:n]
	y = y[:n]

	mx := mathutil.Mean(x)
	my := mathutil.Mean(y)

	var sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		dx := x[i] - mx
		dy := y[i] - my
		sumXY += dx * dy
		sumX2 += dx * dx
		sumY2 += dy * dy
	}

	denom := math.Sqrt(sumX2 * sumY2)
	if denom == 0 {
		return 0
	}
	return sumXY / denom
}

// ---------------------------------------------------------------------------
// Portfolio Risk Analysis
// ---------------------------------------------------------------------------

// AnalyzePortfolioRisk performs full risk analysis on a set of positions.
// corrMatrix maps currency -> currency -> correlation coefficient.
// currentRegime is the FRED macro regime name (e.g. "INFLATIONARY", "STRESS").
func AnalyzePortfolioRisk(
	positions []domain.Position,
	corrMatrix map[string]map[string]float64,
	currentRegime string,
) *domain.PortfolioRisk {
	risk := &domain.PortfolioRisk{
		Positions:         positions,
		CorrelationMatrix: corrMatrix,
	}

	if len(positions) == 0 {
		risk.OverallRiskLevel = "LOW"
		return risk
	}

	// --- Correlation warnings ---
	risk.HighCorrelationWarnings = findHighCorrelations(positions, corrMatrix)

	// --- Concentration score ---
	risk.ConcentrationScore = computeConcentrationScore(positions)

	// --- Regime risk score ---
	risk.RegimeRiskScore = computeRegimeRiskScore(currentRegime, len(positions))

	// --- Overall risk level ---
	risk.OverallRiskLevel = classifyOverallRisk(risk)

	return risk
}

// findHighCorrelations identifies position pairs with |correlation| > 0.7.
// Considers direction: two LONG positions in highly correlated currencies
// are risky; a LONG and SHORT in highly correlated currencies partially hedge.
func findHighCorrelations(
	positions []domain.Position,
	corrMatrix map[string]map[string]float64,
) []string {
	var warnings []string

	for i := 0; i < len(positions); i++ {
		for j := i + 1; j < len(positions); j++ {
			p1 := positions[i]
			p2 := positions[j]

			corr := getCorrelation(corrMatrix, p1.Currency, p2.Currency)
			absCorr := math.Abs(corr)

			if absCorr <= 0.7 {
				continue
			}

			sameDirection := p1.Direction == p2.Direction

			if sameDirection && corr > 0.7 {
				// Same direction + high positive correlation = concentrated risk
				warnings = append(warnings,
					fmt.Sprintf("%s %s + %s %s: corr %.2f (concentrated risk)",
						p1.Currency, p1.Direction, p2.Currency, p2.Direction, corr))
			} else if !sameDirection && corr > 0.7 {
				// Opposite direction + high positive correlation = hedged (informational)
				warnings = append(warnings,
					fmt.Sprintf("%s %s + %s %s: corr %.2f (offsetting positions)",
						p1.Currency, p1.Direction, p2.Currency, p2.Direction, corr))
			} else if sameDirection && corr < -0.7 {
				// Same direction + high negative correlation = internally conflicting
				warnings = append(warnings,
					fmt.Sprintf("%s %s + %s %s: corr %.2f (conflicting signals)",
						p1.Currency, p1.Direction, p2.Currency, p2.Direction, corr))
			} else if !sameDirection && corr < -0.7 {
				// Opposite direction + high negative correlation = concentrated risk
				warnings = append(warnings,
					fmt.Sprintf("%s %s + %s %s: corr %.2f (concentrated risk via inverse)",
						p1.Currency, p1.Direction, p2.Currency, p2.Direction, corr))
			}
		}
	}

	return warnings
}

// getCorrelation safely retrieves a correlation value from the matrix.
func getCorrelation(matrix map[string]map[string]float64, c1, c2 string) float64 {
	if row, ok := matrix[c1]; ok {
		if val, ok := row[c2]; ok {
			return val
		}
	}
	return 0
}

// computeConcentrationScore measures how concentrated the portfolio is.
// Score 0-100: higher = more concentrated.
// Factors: direction imbalance, size distribution, number of positions.
func computeConcentrationScore(positions []domain.Position) float64 {
	if len(positions) == 0 {
		return 0
	}

	// --- Factor 1: Direction imbalance (0-40 points) ---
	var longSize, shortSize float64
	for _, p := range positions {
		if strings.EqualFold(p.Direction, "LONG") {
			longSize += p.Size
		} else {
			shortSize += p.Size
		}
	}
	totalSize := longSize + shortSize
	var directionScore float64
	if totalSize > 0 {
		imbalance := math.Abs(longSize-shortSize) / totalSize // 0 = balanced, 1 = all one direction
		directionScore = imbalance * 40
	}

	// --- Factor 2: Size concentration / Herfindahl index (0-40 points) ---
	// HHI: sum of (share_i)^2. If all equal: 1/N, if one dominates: ~1.
	var hhi float64
	for _, p := range positions {
		if totalSize > 0 {
			share := p.Size / totalSize
			hhi += share * share
		}
	}
	// Normalize: min HHI = 1/N, max HHI = 1
	minHHI := 1.0 / float64(len(positions))
	var sizeScore float64
	if len(positions) > 1 {
		sizeScore = mathutil.Normalize(hhi, minHHI, 1.0) * 0.4
	} else {
		sizeScore = 40 // single position = maximum concentration
	}

	// --- Factor 3: Diversification penalty for few positions (0-20 points) ---
	var diversScore float64
	switch {
	case len(positions) == 1:
		diversScore = 20
	case len(positions) == 2:
		diversScore = 15
	case len(positions) == 3:
		diversScore = 10
	case len(positions) <= 5:
		diversScore = 5
	default:
		diversScore = 0
	}

	score := directionScore + sizeScore + diversScore
	return mathutil.Clamp(score, 0, 100)
}

// computeRegimeRiskScore assigns a risk score modifier based on the current
// macro regime. Higher = more danger in current environment.
func computeRegimeRiskScore(regime string, positionCount int) float64 {
	if positionCount == 0 {
		return 0
	}

	baseScore := 30.0 // Default moderate
	switch strings.ToUpper(regime) {
	case "RECESSION":
		baseScore = 90
	case "STRESS":
		baseScore = 85
	case "STAGFLATION":
		baseScore = 80
	case "INFLATIONARY":
		baseScore = 60
	case "NEUTRAL":
		baseScore = 40
	case "GOLDILOCKS":
		baseScore = 20
	case "DISINFLATIONARY":
		baseScore = 25
	case "":
		baseScore = 50 // Unknown regime
	}

	// Scale slightly by number of positions (more positions = more exposure)
	positionMultiplier := 1.0 + math.Min(float64(positionCount-1)*0.05, 0.3)
	return mathutil.Clamp(baseScore*positionMultiplier, 0, 100)
}

// classifyOverallRisk determines the overall risk level from component scores.
func classifyOverallRisk(risk *domain.PortfolioRisk) string {
	// Weighted composite: concentration 40%, correlation warnings 30%, regime 30%
	corrWarningScore := math.Min(float64(len(risk.HighCorrelationWarnings))*25, 100)
	composite := risk.ConcentrationScore*0.4 + corrWarningScore*0.3 + risk.RegimeRiskScore*0.3

	switch {
	case composite >= 75:
		return "CRITICAL"
	case composite >= 50:
		return "HIGH"
	case composite >= 30:
		return "MEDIUM"
	default:
		return "LOW"
	}
}
