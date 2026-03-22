package fred

import (
	"math"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// ---------------------------------------------------------------------------
// Regime-Asset Performance Matrix
// ---------------------------------------------------------------------------

// RegimeSnapshot captures the FRED regime classification at a point in time.
type RegimeSnapshot struct {
	Date   time.Time
	Regime string // e.g. "INFLATIONARY", "GOLDILOCKS", "STRESS", etc.
}

// PerformanceStats holds statistical summaries of asset returns within a regime.
type PerformanceStats struct {
	AvgWeeklyReturn     float64 // mean weekly return (%)
	AvgAnnualizedReturn float64 // annualized from avg weekly: (1+r/100)^52 - 1 expressed as %
	Occurrences         int     // number of weeks observed
	BestWeek            float64 // maximum single-week return (%)
	WorstWeek           float64 // minimum single-week return (%)
}

// AssetPerf pairs a currency label with its annualized return in a regime.
type AssetPerf struct {
	Currency        string
	AnnualizedReturn float64
	Occurrences     int
}

// RegimeInsight summarises the best and worst performing assets for a given regime.
type RegimeInsight struct {
	Regime     string
	BestAssets  []AssetPerf
	WorstAssets []AssetPerf
}

// RegimeAssetMatrix maps regime -> contract currency -> PerformanceStats.
type RegimeAssetMatrix struct {
	Data map[string]map[string]PerformanceStats
}

// ---------------------------------------------------------------------------
// Computation
// ---------------------------------------------------------------------------

// ComputeRegimeAssetMatrix groups weekly price returns by FRED regime and
// contract, then computes summary statistics for each cell.
//
// regimeHistory must be sorted chronologically (oldest first).
// priceHistory maps contract code to a slice of PriceRecords (any order).
func ComputeRegimeAssetMatrix(
	regimeHistory []RegimeSnapshot,
	priceHistory map[string][]domain.PriceRecord,
) *RegimeAssetMatrix {
	if len(regimeHistory) == 0 || len(priceHistory) == 0 {
		return &RegimeAssetMatrix{Data: make(map[string]map[string]PerformanceStats)}
	}

	// Sort regime snapshots chronologically for binary search.
	sort.Slice(regimeHistory, func(i, j int) bool {
		return regimeHistory[i].Date.Before(regimeHistory[j].Date)
	})

	// returns[regime][currency] = []float64 of weekly returns
	returns := make(map[string]map[string][]float64)

	for code, records := range priceHistory {
		mapping := domain.FindPriceMapping(code)
		if mapping == nil || mapping.RiskOnly {
			continue
		}
		currency := mapping.Currency

		// Sort records chronologically (oldest first) so consecutive pairs
		// represent sequential weeks.
		sorted := make([]domain.PriceRecord, len(records))
		copy(sorted, records)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Date.Before(sorted[j].Date)
		})

		for i := 1; i < len(sorted); i++ {
			prev := sorted[i-1]
			cur := sorted[i]
			if prev.Close == 0 {
				continue
			}
			weeklyReturn := (cur.Close - prev.Close) / prev.Close * 100

			// Find the regime active at the current week's date.
			regime := regimeAt(regimeHistory, cur.Date)
			if regime == "" {
				continue
			}

			if returns[regime] == nil {
				returns[regime] = make(map[string][]float64)
			}
			returns[regime][currency] = append(returns[regime][currency], weeklyReturn)
		}
	}

	// Build the matrix from collected returns.
	matrix := &RegimeAssetMatrix{
		Data: make(map[string]map[string]PerformanceStats),
	}

	for regime, currencies := range returns {
		matrix.Data[regime] = make(map[string]PerformanceStats)
		for currency, rets := range currencies {
			if len(rets) == 0 {
				continue
			}
			avgW := mathutil.Mean(rets)
			matrix.Data[regime][currency] = PerformanceStats{
				AvgWeeklyReturn:     avgW,
				AvgAnnualizedReturn: annualizeWeekly(avgW),
				Occurrences:         len(rets),
				BestWeek:            mathutil.MaxFloat64(rets),
				WorstWeek:           mathutil.MinFloat64(rets),
			}
		}
	}

	return matrix
}

// regimeAt finds the regime in effect at date using binary search on the
// sorted (oldest-first) snapshot slice. Returns the regime of the latest
// snapshot whose date is <= date.
func regimeAt(snapshots []RegimeSnapshot, date time.Time) string {
	// Binary search: find the rightmost snapshot with Date <= date.
	lo, hi := 0, len(snapshots)-1
	idx := -1
	for lo <= hi {
		mid := (lo + hi) / 2
		if !snapshots[mid].Date.After(date) {
			idx = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if idx < 0 {
		return ""
	}
	return snapshots[idx].Regime
}

// annualizeWeekly converts a mean weekly return (%) to annualized return (%).
func annualizeWeekly(weeklyPct float64) float64 {
	r := weeklyPct / 100
	annual := math.Pow(1+r, 52) - 1
	return annual * 100
}

// ---------------------------------------------------------------------------
// Current Regime Insight
// ---------------------------------------------------------------------------

// GetCurrentRegimeInsight returns the top 3 best and worst performing assets
// for the given regime, based on historical annualized returns in the matrix.
func GetCurrentRegimeInsight(currentRegime string, matrix *RegimeAssetMatrix) RegimeInsight {
	insight := RegimeInsight{Regime: currentRegime}

	if matrix == nil || matrix.Data == nil {
		return insight
	}

	currencies, ok := matrix.Data[currentRegime]
	if !ok || len(currencies) == 0 {
		return insight
	}

	// Collect all assets with at least 4 weeks of data (minimum significance).
	var perfs []AssetPerf
	for currency, stats := range currencies {
		if stats.Occurrences < 4 {
			continue
		}
		perfs = append(perfs, AssetPerf{
			Currency:         currency,
			AnnualizedReturn: stats.AvgAnnualizedReturn,
			Occurrences:      stats.Occurrences,
		})
	}

	if len(perfs) == 0 {
		return insight
	}

	// Sort descending by annualized return.
	sort.Slice(perfs, func(i, j int) bool {
		return perfs[i].AnnualizedReturn > perfs[j].AnnualizedReturn
	})

	top := 3
	if top > len(perfs) {
		top = len(perfs)
	}

	insight.BestAssets = perfs[:top]

	// Worst: take from the tail, but only if there are enough assets
	// to avoid overlapping with BestAssets.
	if len(perfs) > top {
		bottom := 3
		remaining := len(perfs) - top
		if bottom > remaining {
			bottom = remaining
		}
		insight.WorstAssets = perfs[len(perfs)-bottom:]
	}

	// Reverse worst so the worst-performing is first.
	for i, j := 0, len(insight.WorstAssets)-1; i < j; i, j = i+1, j-1 {
		insight.WorstAssets[i], insight.WorstAssets[j] = insight.WorstAssets[j], insight.WorstAssets[i]
	}

	return insight
}

// ---------------------------------------------------------------------------
// Regime History Builder
// ---------------------------------------------------------------------------

// BuildRegimeHistory reconstructs weekly regime snapshots from price dates
// and a regime classifier. It creates one snapshot per unique week by
// classifying the macro data at each point in time.
//
// This is a simplified builder that creates regime snapshots from the
// current FRED data — in production, you would persist these weekly.
// For bootstrap, we reclassify from the current data and assign the same
// regime to all historical weeks (conservative approach).
func BuildRegimeHistoryFromCurrent(data *MacroData, weeks int) []RegimeSnapshot {
	if data == nil || weeks <= 0 {
		return nil
	}

	regime := ClassifyMacroRegime(data)
	snapshots := make([]RegimeSnapshot, weeks)
	now := time.Now()

	for i := 0; i < weeks; i++ {
		snapshots[i] = RegimeSnapshot{
			Date:   now.AddDate(0, 0, -i*7),
			Regime: regime.Name,
		}
	}

	// Return oldest-first.
	for i, j := 0, len(snapshots)-1; i < j; i, j = i+1, j-1 {
		snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
	}

	return snapshots
}
