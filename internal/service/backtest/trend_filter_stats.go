package backtest

import (
	"context"
	"fmt"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// TrendFilterStats holds aggregate statistics on the daily trend filter's impact.
type TrendFilterStats struct {
	TotalSignals       int     `json:"total_signals"`
	FilteredSignals    int     `json:"filtered_signals"`    // Signals that had adjustment
	AvgAdjustment      float64 `json:"avg_adjustment"`      // Mean adjustment applied
	AvgRawConfidence   float64 `json:"avg_raw_confidence"`  // Before filter
	AvgFinalConfidence float64 `json:"avg_final_confidence"` // After filter

	// Performance comparison: trend-aligned vs trend-opposed
	AlignedCount      int     `json:"aligned_count"`       // Signals where adj > 0
	AlignedWinRate1W  float64 `json:"aligned_win_rate_1w"` // Win rate of aligned signals
	OpposedCount      int     `json:"opposed_count"`       // Signals where adj < 0
	OpposedWinRate1W  float64 `json:"opposed_win_rate_1w"` // Win rate of opposed signals
	NeutralCount      int     `json:"neutral_count"`       // Signals where adj == 0
	NeutralWinRate1W  float64 `json:"neutral_win_rate_1w"` // Win rate of neutral signals

	// By daily trend direction
	ByDailyTrend map[string]*TrendBucket `json:"by_daily_trend"`

	// Edge improvement
	BaselineWinRate1W float64 `json:"baseline_win_rate_1w"` // Overall 1W win rate
	FilteredWinRate1W float64 `json:"filtered_win_rate_1w"` // Win rate of top-boosted signals (adj >= 10)
	EdgeGain          float64 `json:"edge_gain"`            // FilteredWinRate - BaselineWinRate
}

// TrendBucket holds stats for a specific daily trend direction.
type TrendBucket struct {
	Trend    string  `json:"trend"`
	Count    int     `json:"count"`
	WinRate  float64 `json:"win_rate"`
	AvgAdj   float64 `json:"avg_adj"`
}

// TrendFilterAnalyzer computes statistics on the daily trend filter's effectiveness.
type TrendFilterAnalyzer struct {
	signalRepo ports_SignalRepo
}

// NewTrendFilterAnalyzer creates a new analyzer.
func NewTrendFilterAnalyzer(signalRepo ports_SignalRepo) *TrendFilterAnalyzer {
	return &TrendFilterAnalyzer{signalRepo: signalRepo}
}

// Analyze computes trend filter stats across all evaluated signals.
func (a *TrendFilterAnalyzer) Analyze(ctx context.Context) (*TrendFilterStats, error) {
	signals, err := a.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get signals: %w", err)
	}

	stats := &TrendFilterStats{
		ByDailyTrend: make(map[string]*TrendBucket),
	}

	var (
		sumAdj, sumRaw, sumFinal float64
		alignedWins, opposedWins, neutralWins int
		evaluated1W                            int
		totalWins1W                            int
		strongBoostCount, strongBoostWins      int
	)

	for _, sig := range signals {
		if sig.Outcome1W == "" || sig.Outcome1W == domain.OutcomePending {
			continue
		}

		stats.TotalSignals++
		evaluated1W++
		win := sig.Outcome1W == domain.OutcomeWin
		if win {
			totalWins1W++
		}

		adj := sig.DailyTrendAdj
		raw := sig.RawConfidence
		if raw == 0 {
			raw = sig.Confidence // No filter applied — legacy signal
		}

		sumRaw += raw
		sumFinal += sig.Confidence

		if adj != 0 {
			stats.FilteredSignals++
			sumAdj += adj
		}

		// Categorize by alignment
		if adj > 0 {
			stats.AlignedCount++
			if win {
				alignedWins++
			}
		} else if adj < 0 {
			stats.OpposedCount++
			if win {
				opposedWins++
			}
		} else {
			stats.NeutralCount++
			if win {
				neutralWins++
			}
		}

		// Strong boost bucket (adj >= 10)
		if adj >= 10 {
			strongBoostCount++
			if win {
				strongBoostWins++
			}
		}

		// By daily trend
		trend := sig.DailyTrend
		if trend == "" {
			trend = "N/A"
		}
		bucket, ok := stats.ByDailyTrend[trend]
		if !ok {
			bucket = &TrendBucket{Trend: trend}
			stats.ByDailyTrend[trend] = bucket
		}
		bucket.Count++
		bucket.AvgAdj += adj
		if win {
			bucket.WinRate++
		}
	}

	n := float64(stats.TotalSignals)
	if n > 0 {
		stats.AvgRawConfidence = sumRaw / n
		stats.AvgFinalConfidence = sumFinal / n
	}
	if stats.FilteredSignals > 0 {
		stats.AvgAdjustment = sumAdj / float64(stats.FilteredSignals)
	}

	if stats.AlignedCount > 0 {
		stats.AlignedWinRate1W = float64(alignedWins) / float64(stats.AlignedCount) * 100
	}
	if stats.OpposedCount > 0 {
		stats.OpposedWinRate1W = float64(opposedWins) / float64(stats.OpposedCount) * 100
	}
	if stats.NeutralCount > 0 {
		stats.NeutralWinRate1W = float64(neutralWins) / float64(stats.NeutralCount) * 100
	}

	if evaluated1W > 0 {
		stats.BaselineWinRate1W = float64(totalWins1W) / float64(evaluated1W) * 100
	}
	if strongBoostCount > 0 {
		stats.FilteredWinRate1W = float64(strongBoostWins) / float64(strongBoostCount) * 100
	}
	stats.EdgeGain = stats.FilteredWinRate1W - stats.BaselineWinRate1W

	// Finalize per-trend averages
	for _, bucket := range stats.ByDailyTrend {
		if bucket.Count > 0 {
			bucket.WinRate = bucket.WinRate / float64(bucket.Count) * 100
			bucket.AvgAdj = bucket.AvgAdj / float64(bucket.Count)
		}
	}

	return stats, nil
}

// SortedTrends returns trend buckets sorted by count descending.
func (s *TrendFilterStats) SortedTrends() []*TrendBucket {
	var buckets []*TrendBucket
	for _, b := range s.ByDailyTrend {
		buckets = append(buckets, b)
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Count > buckets[j].Count
	})
	return buckets
}
