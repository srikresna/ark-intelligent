package backtest

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// MonteCarloResult holds the aggregate output of a Monte Carlo bootstrap simulation.
type MonteCarloResult struct {
	NumSimulations    int     `json:"num_simulations"`
	WeeksResampled    int     `json:"weeks_resampled"` // Number of historical weekly returns used
	MedianReturn      float64 `json:"median_return"`
	P5Return          float64 `json:"p5_return"`
	P95Return         float64 `json:"p95_return"`
	MedianMaxDD       float64 `json:"median_max_dd"`
	WorstCaseMaxDD    float64 `json:"worst_case_max_dd"` // 95th percentile of drawdowns
	ProbabilityOfLoss float64 `json:"probability_of_loss"`
	MedianSharpe      float64 `json:"median_sharpe"`
}

// MonteCarloSimulator runs bootstrap simulations on historical signal returns.
type MonteCarloSimulator struct {
	signalRepo ports.SignalRepository
}

// NewMonteCarloSimulator creates a new Monte Carlo simulator.
func NewMonteCarloSimulator(signalRepo ports.SignalRepository) *MonteCarloSimulator {
	return &MonteCarloSimulator{signalRepo: signalRepo}
}

// Simulate runs numSims bootstrap simulations over weekly portfolio returns.
//
// Signals are first aggregated into equal-weighted weekly portfolio returns
// (matching PortfolioAnalyzer logic), then weekly returns are resampled with
// replacement to create simulated 52-week years. This produces realistic
// portfolio-level estimates instead of compounding individual signal returns,
// which would give unrealistically extreme cumulative figures.
func (mc *MonteCarloSimulator) Simulate(ctx context.Context, numSims int) (*MonteCarloResult, error) {
	signals, err := mc.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	// Aggregate into equal-weighted weekly portfolio returns.
	weeklyReturns := aggregateWeeklyReturns(signals)
	if len(weeklyReturns) < 4 {
		return nil, fmt.Errorf("insufficient weekly data: %d weeks (need ≥4)", len(weeklyReturns))
	}

	nWeeks := len(weeklyReturns)
	simYearWeeks := 52 // simulate 52-week years
	simReturns := make([]float64, numSims)
	simMaxDDs := make([]float64, numSims)
	simSharpes := make([]float64, numSims)
	lossCount := 0

	// Use a local RNG seeded from current time for reproducibility control
	// and thread safety (global rand is not safe for concurrent use).
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < numSims; i++ {
		// Bootstrap: resample 52 weekly returns with replacement.
		equity := 1.0
		peak := 1.0
		maxDD := 0.0
		sumRet := 0.0
		sumRetSq := 0.0

		for j := 0; j < simYearWeeks; j++ {
			r := weeklyReturns[rng.Intn(nWeeks)]
			equity *= 1 + r/100
			if equity > peak {
				peak = equity
			}
			dd := (peak - equity) / peak
			if dd > maxDD {
				maxDD = dd
			}
			sumRet += r
			sumRetSq += r * r
		}

		cumReturn := (equity - 1) * 100
		simReturns[i] = cumReturn
		simMaxDDs[i] = maxDD * 100

		if cumReturn < 0 {
			lossCount++
		}

		// Sharpe: mean / stddev * sqrt(52) (weekly to annualized).
		// Use sample variance (n-1 denominator) for consistency with mathutil.SharpeRatio.
		n := float64(simYearWeeks)
		meanRet := sumRet / n
		if n > 1 {
			variance := (sumRetSq - n*meanRet*meanRet) / (n - 1)
			if variance > 0 {
				simSharpes[i] = (meanRet / math.Sqrt(variance)) * math.Sqrt(52)
			}
		}
	}

	sort.Float64s(simReturns)
	sort.Float64s(simMaxDDs)
	sort.Float64s(simSharpes)

	result := &MonteCarloResult{
		NumSimulations:    numSims,
		WeeksResampled:    nWeeks,
		MedianReturn:      round2(percentile(simReturns, 50)),
		P5Return:          round2(percentile(simReturns, 5)),
		P95Return:         round2(percentile(simReturns, 95)),
		MedianMaxDD:       round2(percentile(simMaxDDs, 50)),
		WorstCaseMaxDD:    round2(percentile(simMaxDDs, 95)), // 95th percentile = worst 5% of drawdowns
		ProbabilityOfLoss: round2(float64(lossCount) / float64(numSims) * 100),
		MedianSharpe:      round2(percentile(simSharpes, 50)),
	}

	return result, nil
}

// aggregateWeeklyReturns groups evaluated signals into ISO-week buckets and
// returns equal-weighted weekly portfolio returns, sorted chronologically.
// This mirrors PortfolioAnalyzer.Analyze but returns just the return slice.
func aggregateWeeklyReturns(signals []domain.PersistedSignal) []float64 {
	type bucket struct {
		key       string
		sumReturn float64
		count     int
	}

	weekMap := make(map[string]*bucket)
	for _, s := range signals {
		if s.Outcome1W != domain.OutcomeWin && s.Outcome1W != domain.OutcomeLoss {
			continue
		}
		y, w := s.ReportDate.ISOWeek()
		key := fmt.Sprintf("%04d-W%02d", y, w)
		b, ok := weekMap[key]
		if !ok {
			b = &bucket{key: key}
			weekMap[key] = b
		}
		b.sumReturn += s.Return1W
		b.count++
	}

	weeks := make([]string, 0, len(weekMap))
	for k := range weekMap {
		weeks = append(weeks, k)
	}
	sort.Strings(weeks)

	returns := make([]float64, 0, len(weeks))
	for _, k := range weeks {
		b := weekMap[k]
		returns = append(returns, b.sumReturn/float64(b.count))
	}
	return returns
}

// percentile returns the p-th percentile (0-100) from a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
