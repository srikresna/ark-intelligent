package price

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Seasonal Pattern Analysis — Historical monthly return tendencies
// ---------------------------------------------------------------------------

// SeasonalPattern holds monthly return statistics for a contract.
type SeasonalPattern struct {
	ContractCode string
	Currency     string
	Monthly      [12]MonthStats // Index 0 = January
	CurrentMonth int            // 1-12
	CurrentBias  string         // "BULLISH", "BEARISH", "NEUTRAL" based on current month's historical stats
}

// MonthStats holds aggregated return statistics for a single calendar month.
type MonthStats struct {
	Month      string  // "Jan", "Feb", etc.
	AvgReturn  float64 // Average monthly return %
	WinRate    float64 // % of months positive
	SampleSize int     // Number of data points
	Bias       string  // "BULLISH" (>55% WR + positive avg), "BEARISH" (<45% WR + negative avg), "NEUTRAL"
}

var monthNames = [12]string{
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

// SeasonalAnalyzer computes historical seasonal tendencies from stored price data.
type SeasonalAnalyzer struct {
	priceRepo ports.PriceRepository
}

// NewSeasonalAnalyzer creates a new SeasonalAnalyzer.
func NewSeasonalAnalyzer(priceRepo ports.PriceRepository) *SeasonalAnalyzer {
	return &SeasonalAnalyzer{priceRepo: priceRepo}
}

// Analyze computes seasonal patterns for all COT contracts.
// Uses up to 260 weeks (5 years) of history.
func (sa *SeasonalAnalyzer) Analyze(ctx context.Context) ([]SeasonalPattern, error) {
	mappings := domain.COTPriceSymbolMappings()
	patterns := make([]SeasonalPattern, 0, len(mappings))

	for _, m := range mappings {
		p, err := sa.AnalyzeContract(ctx, m.ContractCode, m.Currency)
		if err != nil {
			// Skip contracts with insufficient data; don't fail the whole batch.
			continue
		}
		patterns = append(patterns, *p)
	}

	if len(patterns) == 0 {
		return nil, fmt.Errorf("no seasonal data available for any contract")
	}
	return patterns, nil
}

// AnalyzeContract computes seasonal pattern for a single contract.
func (sa *SeasonalAnalyzer) AnalyzeContract(ctx context.Context, contractCode, currency string) (*SeasonalPattern, error) {
	const maxWeeks = 260 // 5 years

	records, err := sa.priceRepo.GetHistory(ctx, contractCode, maxWeeks)
	if err != nil {
		return nil, fmt.Errorf("get price history for %s: %w", currency, err)
	}
	if len(records) < 8 { // need at least 2 months of data
		return nil, fmt.Errorf("insufficient price data for %s (%d records)", currency, len(records))
	}

	// Records come newest-first; reverse to oldest-first for chronological processing.
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date.Before(records[j].Date)
	})

	// Group weekly closes by year-month, picking first and last close per month.
	type monthBound struct {
		firstClose float64
		lastClose  float64
		firstDate  time.Time
		lastDate   time.Time
	}

	// Key: "YYYY-MM"
	months := make(map[string]*monthBound)
	for _, r := range records {
		if r.Close == 0 {
			continue
		}
		key := r.Date.Format("2006-01")
		mb, ok := months[key]
		if !ok {
			mb = &monthBound{
				firstClose: r.Close,
				lastClose:  r.Close,
				firstDate:  r.Date,
				lastDate:   r.Date,
			}
			months[key] = mb
		} else {
			if r.Date.Before(mb.firstDate) {
				mb.firstClose = r.Close
				mb.firstDate = r.Date
			}
			if r.Date.After(mb.lastDate) {
				mb.lastClose = r.Close
				mb.lastDate = r.Date
			}
		}
	}

	// Compute monthly returns and aggregate by calendar month (1-12).
	type monthReturn struct {
		month  time.Month
		retPct float64
	}

	var returns []monthReturn
	for _, mb := range months {
		if mb.firstClose == 0 {
			continue
		}
		ret := (mb.lastClose - mb.firstClose) / mb.firstClose * 100.0
		returns = append(returns, monthReturn{
			month:  mb.firstDate.Month(),
			retPct: ret,
		})
	}

	// Aggregate per calendar month.
	var sums [12]float64
	var counts [12]int
	var wins [12]int

	for _, r := range returns {
		idx := int(r.month) - 1
		sums[idx] += r.retPct
		counts[idx]++
		if r.retPct > 0 {
			wins[idx]++
		}
	}

	now := time.Now()
	pattern := &SeasonalPattern{
		ContractCode: contractCode,
		Currency:     currency,
		CurrentMonth: int(now.Month()),
	}

	for i := 0; i < 12; i++ {
		ms := MonthStats{
			Month:      monthNames[i],
			SampleSize: counts[i],
		}
		if counts[i] > 0 {
			ms.AvgReturn = sums[i] / float64(counts[i])
			ms.WinRate = float64(wins[i]) / float64(counts[i]) * 100.0
		}
		ms.Bias = classifyBias(ms.AvgReturn, ms.WinRate, ms.SampleSize)
		pattern.Monthly[i] = ms
	}

	// Set current month bias.
	curIdx := int(now.Month()) - 1
	pattern.CurrentBias = pattern.Monthly[curIdx].Bias

	return pattern, nil
}

// classifyBias determines seasonal bias from average return and win rate.
func classifyBias(avgReturn, winRate float64, sampleSize int) string {
	if sampleSize < 2 {
		return "NEUTRAL"
	}
	if avgReturn > 0 && winRate > 55.0 {
		return "BULLISH"
	}
	if avgReturn < 0 && winRate < 45.0 {
		return "BEARISH"
	}
	return "NEUTRAL"
}
