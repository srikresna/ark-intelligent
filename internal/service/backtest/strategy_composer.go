package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Multi-Strategy Backtester (TASK-139)
// ---------------------------------------------------------------------------
//
// Runs independent backtests per strategy signal type, computes per-strategy
// metrics (Sharpe, drawdown, win rate, profit factor), analyzes
// inter-strategy return correlations, and combines via optimal weighting.
//
// Strategies are mapped from SignalType values in PersistedSignal:
//   - "SMART_MONEY", "EXTREME_POSITIONING", "DIVERGENCE",
//     "MOMENTUM_SHIFT", "CONCENTRATION", "CROWD_CONTRARIAN",
//     "THIN_MARKET" → all grouped under "COT"
//
// For richer separation, each unique SignalType is treated as its own
// sub-strategy so that per-type performance is preserved.

// StrategyLabel constants — canonical display names.
const (
	StrategyAll = "ALL"
)

// knownStrategyGroups maps individual signal types to a high-level group.
// Signals not listed here are left ungrouped (use their raw SignalType name).
var knownStrategyGroups = map[string]string{ //nolint:gochecknoglobals
	"SMART_MONEY":        "COT",
	"EXTREME_POSITIONING": "COT",
	"DIVERGENCE":         "COT",
	"MOMENTUM_SHIFT":     "COT",
	"CONCENTRATION":      "COT",
	"CROWD_CONTRARIAN":   "COT",
	"THIN_MARKET":        "COT",
}

// StrategyMetrics holds per-strategy performance metrics.
type StrategyMetrics struct {
	Name         string    `json:"name"`           // Display name (e.g. "COT", "SMART_MONEY")
	SignalTypes  []string  `json:"signal_types"`   // Constituent signal types
	SignalCount  int       `json:"signal_count"`   // Total signals evaluated
	WinRate      float64   `json:"win_rate"`       // 1W win rate (0-100%)
	AvgReturn    float64   `json:"avg_return"`     // Average 1W return (%)
	Sharpe       float64   `json:"sharpe"`         // Annualized Sharpe
	MaxDrawdown  float64   `json:"max_drawdown"`   // Max peak-to-trough (%)
	ProfitFactor float64   `json:"profit_factor"`  // Sum(wins) / Sum(losses)
	Volatility   float64   `json:"volatility"`     // Weekly return std dev (%)
	Weekly       []float64 `json:"-"`              // Weekly returns (used for correlation)
}

// StrategyCorrelation holds pairwise strategy return correlations.
type StrategyCorrelation struct {
	StratA string  `json:"strat_a"`
	StratB string  `json:"strat_b"`
	Corr   float64 `json:"corr"` // -1 to +1
}

// PortfolioComposition defines a weighted portfolio of strategies.
type PortfolioComposition struct {
	Name               string             `json:"name"`                // e.g. "Equal-Weight"
	Weights            map[string]float64 `json:"weights"`             // strategy → weight (sums to 1.0)
	CombinedSharpe     float64            `json:"combined_sharpe"`
	CombinedMaxDD      float64            `json:"combined_max_dd"`
	DiversificationRatio float64          `json:"diversification_ratio"` // weighted avg vol / portfolio vol
}

// MultiStrategyResult is the top-level output of StrategyComposer.
type MultiStrategyResult struct {
	Strategies    []StrategyMetrics     `json:"strategies"`
	Correlations  []StrategyCorrelation `json:"correlations"`
	Portfolios    []PortfolioComposition `json:"portfolios"`
	BestStrategy  string                `json:"best_strategy"`   // by Sharpe
	BestSharpe    float64               `json:"best_sharpe"`
	WorstStrategy string                `json:"worst_strategy"`  // by Sharpe
	WorstSharpe   float64               `json:"worst_sharpe"`
}

// StrategyComposer runs multi-strategy analysis.
type StrategyComposer struct {
	signalRepo ports.SignalRepository
}

// NewStrategyComposer creates a new StrategyComposer.
func NewStrategyComposer(signalRepo ports.SignalRepository) *StrategyComposer {
	return &StrategyComposer{signalRepo: signalRepo}
}

// Compose runs the full multi-strategy analysis.
func (sc *StrategyComposer) Compose(ctx context.Context) (*MultiStrategyResult, error) {
	signals, err := sc.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	// Group signals by strategy.
	grouped := groupByStrategy(signals)

	// Compute per-strategy metrics.
	var strategies []StrategyMetrics
	for name, sigs := range grouped {
		types := uniqueTypes(sigs)
		m := computeStrategyMetrics(name, types, sigs)
		if m.SignalCount < 5 {
			continue // skip strategies with too few signals
		}
		strategies = append(strategies, m)
	}

	if len(strategies) == 0 {
		return &MultiStrategyResult{}, nil
	}

	// Sort by Sharpe descending.
	sort.Slice(strategies, func(i, j int) bool {
		return strategies[i].Sharpe > strategies[j].Sharpe
	})

	// Compute inter-strategy correlations.
	correlations := computeCorrelations(strategies)

	// Build portfolio compositions.
	portfolios := buildPortfolios(strategies)

	// Best/worst by Sharpe.
	best := strategies[0]
	worst := strategies[len(strategies)-1]

	return &MultiStrategyResult{
		Strategies:    strategies,
		Correlations:  correlations,
		Portfolios:    portfolios,
		BestStrategy:  best.Name,
		BestSharpe:    best.Sharpe,
		WorstStrategy: worst.Name,
		WorstSharpe:   worst.Sharpe,
	}, nil
}

// groupByStrategy maps signals to their strategy group name.
func groupByStrategy(signals []domain.PersistedSignal) map[string][]domain.PersistedSignal {
	grouped := make(map[string][]domain.PersistedSignal)
	for _, s := range signals {
		name := s.SignalType
		if group, ok := knownStrategyGroups[s.SignalType]; ok {
			name = group
		}
		if name == "" {
			name = "UNKNOWN"
		}
		grouped[name] = append(grouped[name], s)
	}
	return grouped
}

// uniqueTypes returns the distinct signal types within a group.
func uniqueTypes(signals []domain.PersistedSignal) []string {
	seen := make(map[string]struct{})
	for _, s := range signals {
		seen[s.SignalType] = struct{}{}
	}
	types := make([]string, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// computeStrategyMetrics calculates performance metrics for a strategy.
func computeStrategyMetrics(name string, types []string, signals []domain.PersistedSignal) StrategyMetrics {
	m := StrategyMetrics{
		Name:        name,
		SignalTypes: types,
	}

	// Collect weekly returns using same bucketing as PortfolioAnalyzer.
	weekMap := make(map[string]*weekBucket)
	var sumWin, sumLoss float64

	for _, s := range signals {
		if s.Outcome1W != domain.OutcomeWin && s.Outcome1W != domain.OutcomeLoss {
			continue
		}
		m.SignalCount++

		y, w := s.ReportDate.ISOWeek()
		key := fmt.Sprintf("%04d-W%02d", y, w)
		b, ok := weekMap[key]
		if !ok {
			b = &weekBucket{key: key}
			weekMap[key] = b
		}
		b.sumReturn += s.Return1W
		b.count++

		if s.Outcome1W == domain.OutcomeWin {
			sumWin += math.Abs(s.Return1W)
			m.WinRate++
		} else {
			sumLoss += math.Abs(s.Return1W)
		}
		m.AvgReturn += s.Return1W
	}

	if m.SignalCount == 0 {
		return m
	}

	m.WinRate = m.WinRate / float64(m.SignalCount) * 100.0
	m.AvgReturn /= float64(m.SignalCount)
	if sumLoss > 0 {
		m.ProfitFactor = sumWin / sumLoss
	}

	// Build sorted weekly returns.
	weeks := make([]string, 0, len(weekMap))
	for k := range weekMap {
		weeks = append(weeks, k)
	}
	sort.Strings(weeks)

	weekly := make([]float64, 0, len(weeks))
	for _, k := range weeks {
		bk := weekMap[k]
		weekly = append(weekly, bk.sumReturn/float64(bk.count))
	}
	m.Weekly = weekly

	if len(weekly) < 2 {
		return m
	}

	// Sharpe, MaxDD, Volatility.
	m.Sharpe, m.Volatility = sharpAndVol(weekly)
	m.MaxDrawdown = maxDrawdown(weekly)

	return m
}

// sharpAndVol computes annualized Sharpe and weekly volatility from weekly returns.
func sharpAndVol(weekly []float64) (sharpe, vol float64) {
	if len(weekly) == 0 {
		return 0, 0
	}
	var sum float64
	for _, r := range weekly {
		sum += r
	}
	mean := sum / float64(len(weekly))

	var variance float64
	for _, r := range weekly {
		d := r - mean
		variance += d * d
	}
	if len(weekly) > 1 {
		variance /= float64(len(weekly) - 1)
	}
	vol = math.Sqrt(variance)

	if vol == 0 {
		return 0, 0
	}
	// Annualize: multiply by sqrt(52) weeks.
	sharpe = (mean / vol) * math.Sqrt(52)
	return sharpe, vol
}

// maxDrawdown computes max peak-to-trough drawdown (%) from weekly returns.
func maxDrawdown(weekly []float64) float64 {
	equity := 1.0
	peak := 1.0
	maxDD := 0.0
	for _, r := range weekly {
		equity *= 1 + r/100
		if equity > peak {
			peak = equity
		}
		dd := (peak - equity) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// computeCorrelations computes pairwise return correlations between strategies.
func computeCorrelations(strategies []StrategyMetrics) []StrategyCorrelation {
	var corrs []StrategyCorrelation
	for i := 0; i < len(strategies); i++ {
		for j := i + 1; j < len(strategies); j++ {
			c := pearsonCorr(strategies[i].Weekly, strategies[j].Weekly)
			corrs = append(corrs, StrategyCorrelation{
				StratA: strategies[i].Name,
				StratB: strategies[j].Name,
				Corr:   c,
			})
		}
	}
	// Sort by absolute correlation descending.
	sort.Slice(corrs, func(i, j int) bool {
		return math.Abs(corrs[i].Corr) > math.Abs(corrs[j].Corr)
	})
	return corrs
}

// pearsonCorr computes Pearson correlation between two weekly return series.
// Returns 0 if insufficient data.
func pearsonCorr(a, b []float64) float64 {
	// Align on common length (min).
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 3 {
		return 0
	}

	var sumA, sumB, sumAB, sumA2, sumB2 float64
	for i := 0; i < n; i++ {
		sumA += a[i]
		sumB += b[i]
		sumAB += a[i] * b[i]
		sumA2 += a[i] * a[i]
		sumB2 += b[i] * b[i]
	}

	fn := float64(n)
	num := fn*sumAB - sumA*sumB
	den := math.Sqrt((fn*sumA2 - sumA*sumA) * (fn*sumB2 - sumB*sumB))
	if den == 0 {
		return 0
	}
	return math.Round(num/den*1000) / 1000
}

// buildPortfolios generates equal-weight, inverse-vol-weight, and correlation-aware portfolios.
func buildPortfolios(strategies []StrategyMetrics) []PortfolioComposition {
	if len(strategies) == 0 {
		return nil
	}

	return []PortfolioComposition{
		buildEqualWeight(strategies),
		buildInverseVolWeight(strategies),
		buildCorrelationAwareWeight(strategies),
	}
}

// buildEqualWeight creates a 1/N equal-weight portfolio.
func buildEqualWeight(strategies []StrategyMetrics) PortfolioComposition {
	n := len(strategies)
	w := 1.0 / float64(n)
	weights := make(map[string]float64, n)
	for _, s := range strategies {
		weights[s.Name] = w
	}

	combined := combineReturns(strategies, weights)
	sharpe, vol := sharpAndVol(combined)
	dd := maxDrawdown(combined)
	divRatio := diversificationRatio(strategies, weights, vol)

	return PortfolioComposition{
		Name:                 "Equal-Weight",
		Weights:              weights,
		CombinedSharpe:       sharpe,
		CombinedMaxDD:        dd,
		DiversificationRatio: divRatio,
	}
}

// buildInverseVolWeight creates a portfolio weighted inversely to each strategy's volatility.
func buildInverseVolWeight(strategies []StrategyMetrics) PortfolioComposition {
	weights := make(map[string]float64, len(strategies))
	sumInvVol := 0.0
	for _, s := range strategies {
		vol := s.Volatility
		if vol <= 0 {
			vol = 1.0 // fallback to equal if no vol data
		}
		inv := 1.0 / vol
		weights[s.Name] = inv
		sumInvVol += inv
	}
	// Normalize to sum to 1.
	if sumInvVol > 0 {
		for k := range weights {
			weights[k] /= sumInvVol
		}
	}

	combined := combineReturns(strategies, weights)
	sharpe, vol := sharpAndVol(combined)
	dd := maxDrawdown(combined)
	divRatio := diversificationRatio(strategies, weights, vol)

	return PortfolioComposition{
		Name:                 "Inverse-Vol",
		Weights:              weights,
		CombinedSharpe:       sharpe,
		CombinedMaxDD:        dd,
		DiversificationRatio: divRatio,
	}
}

// buildCorrelationAwareWeight uses a simplified minimum-correlation weighting:
// penalize strategies that are highly correlated with others.
func buildCorrelationAwareWeight(strategies []StrategyMetrics) PortfolioComposition {
	n := len(strategies)
	weights := make(map[string]float64, n)

	if n == 1 {
		weights[strategies[0].Name] = 1.0
	} else {
		// Score each strategy by average |correlation| with all others.
		// Lower avg correlation → higher weight.
		avgCorr := make(map[string]float64, n)
		for i := 0; i < n; i++ {
			sumC := 0.0
			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				sumC += math.Abs(pearsonCorr(strategies[i].Weekly, strategies[j].Weekly))
			}
			avgCorr[strategies[i].Name] = sumC / float64(n-1)
		}

		// Weight = (1 - avgCorr) / sum. Clamp to 0 if avgCorr > 1.
		sumW := 0.0
		for _, s := range strategies {
			w := math.Max(0, 1-avgCorr[s.Name])
			weights[s.Name] = w
			sumW += w
		}
		if sumW <= 0 {
			// Fall back to equal weight.
			eq := 1.0 / float64(n)
			for _, s := range strategies {
				weights[s.Name] = eq
			}
		} else {
			for k := range weights {
				weights[k] /= sumW
			}
		}
	}

	combined := combineReturns(strategies, weights)
	sharpe, vol := sharpAndVol(combined)
	dd := maxDrawdown(combined)
	divRatio := diversificationRatio(strategies, weights, vol)

	return PortfolioComposition{
		Name:                 "Corr-Aware",
		Weights:              weights,
		CombinedSharpe:       sharpe,
		CombinedMaxDD:        dd,
		DiversificationRatio: divRatio,
	}
}

// combineReturns produces a combined weekly return series using strategy weights.
// Uses the minimum common length.
func combineReturns(strategies []StrategyMetrics, weights map[string]float64) []float64 {
	if len(strategies) == 0 {
		return nil
	}

	// Find minimum non-empty length.
	minLen := -1
	for _, s := range strategies {
		if len(s.Weekly) == 0 {
			continue
		}
		if minLen < 0 || len(s.Weekly) < minLen {
			minLen = len(s.Weekly)
		}
	}
	if minLen <= 0 {
		return nil
	}

	combined := make([]float64, minLen)
	for _, s := range strategies {
		w := weights[s.Name]
		if w == 0 || len(s.Weekly) == 0 {
			continue
		}
		for i := 0; i < minLen; i++ {
			combined[i] += s.Weekly[i] * w
		}
	}
	return combined
}

// diversificationRatio = weighted avg of individual vols / portfolio vol.
// > 1 means diversification benefit; = 1 means no benefit (perfect correlation).
func diversificationRatio(strategies []StrategyMetrics, weights map[string]float64, portfolioVol float64) float64 {
	if portfolioVol <= 0 {
		return 1.0
	}
	var weightedVol float64
	for _, s := range strategies {
		weightedVol += weights[s.Name] * s.Volatility
	}
	return math.Round(weightedVol/portfolioVol*100) / 100
}
