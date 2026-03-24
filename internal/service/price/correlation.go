package price

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// CorrelationEngine computes rolling cross-pair correlation matrices.
type CorrelationEngine struct {
	dailyRepo DailyPriceStore
}

// NewCorrelationEngine creates a new correlation engine.
func NewCorrelationEngine(repo DailyPriceStore) *CorrelationEngine {
	return &CorrelationEngine{dailyRepo: repo}
}

// BuildMatrix computes the full NxN correlation matrix from daily close prices.
// period = rolling window in days (e.g. 20 for short-term, 60 for baseline).
func (ce *CorrelationEngine) BuildMatrix(ctx context.Context, period int) (*domain.CorrelationMatrix, error) {
	currencies := domain.DefaultCorrelationCurrencies()

	// Fetch daily returns for each currency
	returnsMap := make(map[string][]float64) // currency -> daily returns (oldest-first)

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			continue
		}

		records, err := ce.dailyRepo.GetDailyHistory(ctx, mapping.ContractCode, period+5)
		if err != nil || len(records) < period {
			continue
		}

		// records are newest-first; compute returns oldest-first
		returns := make([]float64, 0, period)
		// Take last `period` records, compute returns
		end := period
		if end > len(records)-1 {
			end = len(records) - 1
		}
		for i := end; i >= 1; i-- {
			if records[i].Close > 0 {
				ret := (records[i-1].Close - records[i].Close) / records[i].Close * 100
				returns = append(returns, ret)
			}
		}

		if len(returns) >= period-2 { // Allow minor gaps
			returnsMap[cur] = returns
		}
	}

	// Build valid currencies list (only those with enough data)
	var validCurrencies []string
	for _, cur := range currencies {
		if _, ok := returnsMap[cur]; ok {
			validCurrencies = append(validCurrencies, cur)
		}
	}

	if len(validCurrencies) < 2 {
		return nil, fmt.Errorf("insufficient data for correlation matrix (need ≥2 currencies, got %d)", len(validCurrencies))
	}

	// Compute pairwise Pearson correlation
	matrix := make(map[string]map[string]float64)
	for _, a := range validCurrencies {
		matrix[a] = make(map[string]float64)
		for _, b := range validCurrencies {
			if a == b {
				matrix[a][b] = 1.0
			} else {
				matrix[a][b] = pearsonCorrelation(returnsMap[a], returnsMap[b])
			}
		}
	}

	result := &domain.CorrelationMatrix{
		Currencies: validCurrencies,
		Matrix:     matrix,
		Period:     period,
	}

	return result, nil
}

// BuildWithBreakdowns computes 20-day matrix and detects correlation breakdowns
// by comparing against 60-day baseline.
func (ce *CorrelationEngine) BuildWithBreakdowns(ctx context.Context) (*domain.CorrelationMatrix, error) {
	// Short-term (20-day)
	shortMatrix, err := ce.BuildMatrix(ctx, 20)
	if err != nil {
		return nil, fmt.Errorf("short-term correlation: %w", err)
	}

	// Baseline (60-day)
	longMatrix, err := ce.BuildMatrix(ctx, 60)
	if err != nil {
		// If we can't compute 60-day, return short-term without breakdowns
		return shortMatrix, nil
	}

	// Detect breakdowns
	var breakdowns []domain.CorrelationBreakdown
	for _, a := range shortMatrix.Currencies {
		for _, b := range shortMatrix.Currencies {
			if a >= b { // Skip diagonal and duplicates
				continue
			}

			shortCorr, okS := shortMatrix.Matrix[a][b]
			longCorr, okL := longMatrix.Matrix[a][b]
			if !okS || !okL {
				continue
			}

			delta := shortCorr - longCorr
			absDelta := math.Abs(delta)

			var severity string
			switch {
			case absDelta >= 0.40:
				severity = "HIGH"
			case absDelta >= 0.25:
				severity = "MEDIUM"
			default:
				continue // Skip minor changes
			}

			breakdowns = append(breakdowns, domain.CorrelationBreakdown{
				CurrencyA:      a,
				CurrencyB:      b,
				CurrentCorr:    roundN(shortCorr, 3),
				HistoricalCorr: roundN(longCorr, 3),
				Delta:          roundN(delta, 3),
				Severity:       severity,
			})
		}
	}

	// Sort breakdowns by absolute delta descending
	sort.Slice(breakdowns, func(i, j int) bool {
		return math.Abs(breakdowns[i].Delta) > math.Abs(breakdowns[j].Delta)
	})

	shortMatrix.Breakdowns = breakdowns
	return shortMatrix, nil
}

// DetectClusters finds groups of highly correlated currencies.
func (ce *CorrelationEngine) DetectClusters(matrix *domain.CorrelationMatrix, threshold float64) []domain.CorrelationCluster {
	if threshold == 0 {
		threshold = 0.70 // Default: 70% correlation
	}

	assigned := make(map[string]bool)
	var clusters []domain.CorrelationCluster

	for _, a := range matrix.Currencies {
		if assigned[a] {
			continue
		}

		cluster := []string{a}
		assigned[a] = true

		for _, b := range matrix.Currencies {
			if assigned[b] || a == b {
				continue
			}
			if corr, ok := matrix.Matrix[a][b]; ok && corr >= threshold {
				// Check if b correlates with all existing cluster members
				fitsCluster := true
				for _, member := range cluster {
					if memberCorr, ok := matrix.Matrix[member][b]; ok {
						if memberCorr < threshold {
							fitsCluster = false
							break
						}
					}
				}
				if fitsCluster {
					cluster = append(cluster, b)
					assigned[b] = true
				}
			}
		}

		if len(cluster) >= 2 {
			// Compute average intra-cluster correlation
			var corrSum float64
			pairs := 0
			for i, ci := range cluster {
				for _, cj := range cluster[i+1:] {
					if c, ok := matrix.Matrix[ci][cj]; ok {
						corrSum += c
						pairs++
					}
				}
			}
			avgCorr := 0.0
			if pairs > 0 {
				avgCorr = corrSum / float64(pairs)
			}

			clusters = append(clusters, domain.CorrelationCluster{
				Name:       clusterName(cluster),
				Currencies: cluster,
				AvgCorr:    roundN(avgCorr, 3),
			})
		}
	}

	return clusters
}

// clusterName generates a descriptive name based on cluster members.
func clusterName(members []string) string {
	hasRisk := false
	hasSafe := false
	for _, m := range members {
		switch m {
		case "AUD", "NZD", "CAD", "BTC", "SPX500":
			hasRisk = true
		case "JPY", "CHF", "XAU", "BOND":
			hasSafe = true
		}
	}
	if hasRisk && !hasSafe {
		return "Risk-On"
	}
	if hasSafe && !hasRisk {
		return "Safe-Haven"
	}
	return "Cluster"
}

// --- Math ---

// pearsonCorrelation computes Pearson correlation coefficient between two slices.
func pearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if len(y) < n {
		n = len(y)
	}
	if n < 3 {
		return 0
	}

	// Compute means
	var sumX, sumY float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	// Compute correlation
	var sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		sumXY += dx * dy
		sumX2 += dx * dx
		sumY2 += dy * dy
	}

	denom := math.Sqrt(sumX2 * sumY2)
	if denom == 0 {
		return 0
	}

	return roundN(sumXY/denom, 4)
}
