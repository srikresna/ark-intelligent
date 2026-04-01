package cot

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// ---------------------------------------------------------------------------
// COT Seasonality Analysis
// Computes week-of-year averages from COT net positioning history.
// Reference: internal/service/price/seasonal.go (same pattern for prices).
// ---------------------------------------------------------------------------

// COTSeasonalPoint holds the net-position average for one ISO week.
type COTSeasonalPoint struct {
	WeekOfYear int     `json:"week_of_year"` // 1–53
	AvgNet     float64 `json:"avg_net"`      // Mean net position across available years
	StdDev     float64 `json:"std_dev"`      // Standard deviation across years
	SampleSize int     `json:"sample_size"`  // Number of data points (≥1)
}

// COTSeasonalResult is the full seasonal report for a single contract.
type COTSeasonalResult struct {
	ContractCode string `json:"contract_code"`
	Currency     string `json:"currency"`
	CurrentWeek  int    `json:"current_week"`  // ISO week of the analysis date
	CurrentNet   float64 `json:"current_net"`  // Latest actual net position

	// Comparison vs historical average for the current week
	SeasonalAvg float64 `json:"seasonal_avg"` // Average net position for this week-of-year
	Deviation   float64 `json:"deviation"`    // CurrentNet − SeasonalAvg
	DeviationZ  float64 `json:"deviation_z"`  // Deviation / StdDev (z-score)

	// Qualitative labels
	Trend        string `json:"trend"`         // "ABOVE_SEASONAL", "BELOW_SEASONAL", "IN_LINE"
	SeasonalBias string `json:"seasonal_bias"` // "SEASONALLY_BULLISH", "SEASONALLY_BEARISH", "NEUTRAL"
	Description  string `json:"description"`   // Human-readable summary

	// Full seasonal curve (one entry per week present in history)
	Curve []COTSeasonalPoint `json:"curve,omitempty"`

	// Metadata
	TotalRecords int `json:"total_records"` // Number of history records used
}

// SeasonalEngine computes COT seasonality analysis.
type SeasonalEngine struct {
	repo ports.COTRepository
}

// NewSeasonalEngine creates a seasonal engine backed by the given COT repository.
func NewSeasonalEngine(repo ports.COTRepository) *SeasonalEngine {
	return &SeasonalEngine{repo: repo}
}

// Analyze computes COT seasonality for a single contract.
// It fetches up to 52 weeks of history and groups net positions by ISO week.
func (e *SeasonalEngine) Analyze(ctx context.Context, contract domain.COTContract) (*COTSeasonalResult, error) {
	history, err := e.repo.GetHistory(ctx, contract.Code, 52)
	if err != nil {
		return nil, fmt.Errorf("fetch history for %s: %w", contract.Code, err)
	}
	if len(history) < 8 {
		return nil, fmt.Errorf("insufficient COT history for seasonal (%s): got %d, need ≥8", contract.Currency, len(history))
	}

	return e.analyzeRecords(contract, history, time.Now())
}

// analyzeRecords is the pure computation, separated for testability.
func (e *SeasonalEngine) analyzeRecords(contract domain.COTContract, history []domain.COTRecord, now time.Time) (*COTSeasonalResult, error) {
	if len(history) < 8 {
		return nil, fmt.Errorf("insufficient COT history for seasonal (%s): got %d, need ≥8", contract.Currency, len(history))
	}
	// 1. Compute net position for each record and group by ISO week.
	weekBuckets := make(map[int][]float64) // week-of-year → list of net positions
	for _, rec := range history {
		net := computeSmartNet(rec, contract.ReportType)
		_, wk := rec.ReportDate.ISOWeek()
		weekBuckets[wk] = append(weekBuckets[wk], net)
	}

	// 2. Build seasonal curve.
	var curve []COTSeasonalPoint
	for wk := 1; wk <= 53; wk++ {
		vals, ok := weekBuckets[wk]
		if !ok {
			continue
		}
		pt := COTSeasonalPoint{
			WeekOfYear: wk,
			AvgNet:     mathutil.Mean(vals),
			StdDev:     mathutil.StdDev(vals),
			SampleSize: len(vals),
		}
		curve = append(curve, pt)
	}

	// 3. Current week's seasonal stats.
	_, currentWeek := now.ISOWeek()

	// Latest record = most recent in history (history is ordered oldest-first).
	latestRec := history[len(history)-1]
	currentNet := computeSmartNet(latestRec, contract.ReportType)

	// Find the seasonal point for the current week (or nearest).
	seasonalAvg, seasonalStd := lookupWeek(curve, currentWeek)

	deviation := currentNet - seasonalAvg
	var deviationZ float64
	if seasonalStd > 0 {
		deviationZ = deviation / seasonalStd
	}

	// 4. Classify trend & bias.
	trend := classifyTrend(deviationZ)
	seasonalBias := classifySeasonalBias(seasonalAvg, curve, currentWeek)

	// 5. Build human-readable description.
	desc := buildDescription(contract.Currency, currentWeek, currentNet, seasonalAvg, deviationZ, trend, seasonalBias, len(history))

	return &COTSeasonalResult{
		ContractCode: contract.Code,
		Currency:     contract.Currency,
		CurrentWeek:  currentWeek,
		CurrentNet:   currentNet,
		SeasonalAvg:  seasonalAvg,
		Deviation:    deviation,
		DeviationZ:   deviationZ,
		Trend:        trend,
		SeasonalBias: seasonalBias,
		Description:  desc,
		Curve:        curve,
		TotalRecords: len(history),
	}, nil
}

// AnalyzeAll returns seasonal analysis for every default COT contract.
func (e *SeasonalEngine) AnalyzeAll(ctx context.Context) ([]COTSeasonalResult, error) {
	var results []COTSeasonalResult
	for _, c := range domain.DefaultCOTContracts {
		res, err := e.Analyze(ctx, c)
		if err != nil {
			log.Warn().Str("contract", c.Currency).Err(err).Msg("seasonal: skipping contract")
			continue
		}
		results = append(results, *res)
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// computeSmartNet returns the "smart money" net position for a record,
// choosing the appropriate category based on report type.
// TFF → AssetManager net (institutional), DISAGGREGATED → ManagedMoney net.
func computeSmartNet(rec domain.COTRecord, reportType string) float64 {
	switch reportType {
	case "TFF":
		return rec.AssetMgrLong - rec.AssetMgrShort
	case "DISAGGREGATED":
		return rec.ManagedMoneyLong - rec.ManagedMoneyShort
	default:
		return rec.AssetMgrLong - rec.AssetMgrShort
	}
}

// lookupWeek returns (avg, stddev) for the given ISO week from the curve.
// Falls back to the nearest week if the exact week has no data.
func lookupWeek(curve []COTSeasonalPoint, week int) (avg, std float64) {
	// Exact match first.
	for _, pt := range curve {
		if pt.WeekOfYear == week {
			return pt.AvgNet, pt.StdDev
		}
	}
	// Nearest neighbour.
	bestDist := 54
	for _, pt := range curve {
		d := abs(pt.WeekOfYear - week)
		if d > 26 {
			d = 53 - d // wrap-around distance
		}
		if d < bestDist {
			bestDist = d
			avg = pt.AvgNet
			std = pt.StdDev
		}
	}
	return avg, std
}

// classifyTrend returns a label based on deviation Z-score.
func classifyTrend(z float64) string {
	switch {
	case z > 1.5:
		return "ABOVE_SEASONAL"
	case z < -1.5:
		return "BELOW_SEASONAL"
	default:
		return "IN_LINE"
	}
}

// classifySeasonalBias determines whether the seasonal tendency around the
// current week is historically bullish, bearish, or neutral.
// It looks at the 4-week forward trend in the seasonal curve.
func classifySeasonalBias(currentAvg float64, curve []COTSeasonalPoint, currentWeek int) string {
	if len(curve) < 4 {
		return "NEUTRAL"
	}

	// Collect the seasonal averages for the next 4 weeks.
	var forwardAvgs []float64
	for offset := 1; offset <= 4; offset++ {
		target := (currentWeek + offset)
		if target > 53 {
			target -= 53
		}
		for _, pt := range curve {
			if pt.WeekOfYear == target {
				forwardAvgs = append(forwardAvgs, pt.AvgNet)
				break
			}
		}
	}

	if len(forwardAvgs) == 0 {
		return "NEUTRAL"
	}

	fwdMean := mathutil.Mean(forwardAvgs)
	delta := fwdMean - currentAvg

	// Use 5% of the absolute average as a significance threshold.
	threshold := math.Abs(currentAvg) * 0.05
	if threshold < 500 {
		threshold = 500 // minimum absolute threshold for contracts
	}

	switch {
	case delta > threshold:
		return "SEASONALLY_BULLISH"
	case delta < -threshold:
		return "SEASONALLY_BEARISH"
	default:
		return "NEUTRAL"
	}
}

// buildDescription creates a human-readable seasonal summary.
func buildDescription(currency string, week int, currentNet, seasonalAvg, z float64, trend, bias string, records int) string {
	var trendDesc string
	switch trend {
	case "ABOVE_SEASONAL":
		trendDesc = fmt.Sprintf("positioning %.0f contracts ABOVE seasonal average (%.1fσ)", currentNet-seasonalAvg, z)
	case "BELOW_SEASONAL":
		trendDesc = fmt.Sprintf("positioning %.0f contracts BELOW seasonal average (%.1fσ)", seasonalAvg-currentNet, math.Abs(z))
	default:
		trendDesc = fmt.Sprintf("positioning in line with seasonal average (%.1fσ)", z)
	}

	var biasNote string
	switch bias {
	case "SEASONALLY_BULLISH":
		biasNote = "seasonal tendency is BULLISH for the next 4 weeks"
	case "SEASONALLY_BEARISH":
		biasNote = "seasonal tendency is BEARISH for the next 4 weeks"
	default:
		biasNote = "no strong seasonal bias for the next 4 weeks"
	}

	confidence := "limited"
	if records >= 40 {
		confidence = "moderate"
	}

	return fmt.Sprintf("%s Week %d: %s. Forward outlook: %s (sample: %d weeks — %s confidence)",
		currency, week, trendDesc, biasNote, records, confidence)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
