package price

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var corrLog = logger.Component("price-correlation")

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

	var skippedNoMapping []string
	var skippedFetchErr []string
	var skippedInsufficient []string
	var succeeded []string

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			corrLog.Warn().Str("currency", cur).Msg("skip: no price mapping found")
			skippedNoMapping = append(skippedNoMapping, cur)
			continue
		}

		// Request extra calendar days to account for weekends/holidays.
		// 20 trading days ≈ 28 calendar days; period*2 gives comfortable margin.
		calendarDays := period * 2
		if calendarDays < period+10 {
			calendarDays = period + 10
		}
		records, err := ce.dailyRepo.GetDailyHistory(ctx, mapping.ContractCode, calendarDays)
		if err != nil {
			corrLog.Warn().Str("currency", cur).Err(err).Msg("skip: fetch error")
			skippedFetchErr = append(skippedFetchErr, cur)
			continue
		}
		if len(records) < period {
			corrLog.Warn().Str("currency", cur).Int("have", len(records)).Int("need", period).Msg("skip: insufficient records")
			skippedInsufficient = append(skippedInsufficient, cur)
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
			if records[i].Close > 0 && records[i-1].Close > 0 {
				ret := (records[i-1].Close - records[i].Close) / records[i].Close * 100
				returns = append(returns, ret)
			}
		}

		if len(returns) >= period-2 { // Allow minor gaps
			returnsMap[cur] = returns
			succeeded = append(succeeded, cur)
		} else {
			corrLog.Warn().Str("currency", cur).Int("have", len(returns)).Int("need", period-2).Msg("skip: insufficient valid returns after gap filter")
			skippedInsufficient = append(skippedInsufficient, cur)
		}
	}

	// Summary log
	corrLog.Info().Int("period", period).Strs("succeeded", succeeded).Strs("skipped_no_mapping", skippedNoMapping).Strs("skipped_fetch_err", skippedFetchErr).Strs("skipped_insufficient", skippedInsufficient).Msg("correlation matrix build summary")

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
// by comparing against 60-day baseline. If the 20-day window fails, it falls
// back to a 10-day window before giving up.
func (ce *CorrelationEngine) BuildWithBreakdowns(ctx context.Context) (*domain.CorrelationMatrix, error) {
	// Short-term (20-day) with fallback to 10-day
	shortMatrix, err := ce.BuildMatrix(ctx, 20)
	if err != nil {
		corrLog.Warn().Err(err).Msg("20-day matrix failed, falling back to 10-day")
		shortMatrix, err = ce.BuildMatrix(ctx, 10)
		if err != nil {
			// Build a diagnostic message showing data availability
			return nil, fmt.Errorf("correlation matrix unavailable (tried 20-day and 10-day): %w\n%s",
				err, ce.diagnoseDataAvailability(ctx))
		}
		corrLog.Info().Int("currencies", len(shortMatrix.Currencies)).Msg("10-day fallback succeeded")
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

// diagnoseDataAvailability checks each currency and returns a human-readable
// summary of which have data and which don't.
func (ce *CorrelationEngine) diagnoseDataAvailability(ctx context.Context) string {
	currencies := domain.DefaultCorrelationCurrencies()
	var withData []string
	var withoutData []string

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			withoutData = append(withoutData, cur+" (no mapping)")
			continue
		}
		records, err := ce.dailyRepo.GetDailyHistory(ctx, mapping.ContractCode, 30)
		if err != nil {
			withoutData = append(withoutData, cur+fmt.Sprintf(" (error: %v)", err))
			continue
		}
		if len(records) == 0 {
			withoutData = append(withoutData, cur+" (0 records)")
		} else {
			withData = append(withData, fmt.Sprintf("%s (%d records)", cur, len(records)))
		}
	}

	msg := fmt.Sprintf("Data available: %v | Missing: %v", withData, withoutData)
	corrLog.Info().Str("diagnosis", msg).Msg("data availability diagnosis")
	return msg
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
	counts := map[string]int{}
	for _, m := range members {
		switch m {
		case "AUD", "NZD", "CAD", "BTC", "ETH", "SPX500", "NDX", "DJI", "RUT":
			counts["risk"]++
		case "JPY", "CHF", "XAU", "XAG", "BOND", "BOND30", "BOND5", "BOND2":
			counts["safe"]++
		case "OIL", "ULSD", "RBOB":
			counts["energy"]++
		case "COPPER":
			counts["risk"]++ // Copper is a growth/risk proxy
		}
	}

	// Pick dominant theme.
	best, bestN := "Cluster", 0
	labels := map[string]string{"risk": "Risk-On", "safe": "Safe-Haven", "energy": "Energy"}
	for key, n := range counts {
		if n > bestN {
			bestN = n
			best = labels[key]
		}
	}
	return best
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
