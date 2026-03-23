package price

import (
	"context"
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// DailyContextBuilder computes daily price context from stored daily price records.
type DailyContextBuilder struct {
	repo DailyPriceStore
}

// DailyPriceStore defines the storage interface needed by DailyContextBuilder.
type DailyPriceStore interface {
	GetDailyHistory(ctx context.Context, contractCode string, days int) ([]domain.DailyPrice, error)
}

// NewDailyContextBuilder creates a new daily price context builder.
func NewDailyContextBuilder(repo DailyPriceStore) *DailyContextBuilder {
	return &DailyContextBuilder{repo: repo}
}

// Build computes daily price context for a single contract.
// Requires up to 220 days of history (for 200 DMA).
func (cb *DailyContextBuilder) Build(ctx context.Context, contractCode, currency string) (*domain.DailyPriceContext, error) {
	records, err := cb.repo.GetDailyHistory(ctx, contractCode, 220)
	if err != nil {
		return nil, fmt.Errorf("get daily history: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no daily price data for %s", contractCode)
	}

	// records are newest-first
	dc := &domain.DailyPriceContext{
		ContractCode: contractCode,
		Currency:     currency,
		CurrentPrice: records[0].Close,
	}

	// Daily change (latest vs previous day)
	if len(records) >= 2 && records[1].Close > 0 {
		dc.DailyChgPct = roundN(((records[0].Close-records[1].Close)/records[1].Close)*100, 4)
	}

	// 5-day (weekly) change
	if len(records) >= 6 && records[5].Close > 0 {
		dc.WeeklyChgPct = roundN(((records[0].Close-records[5].Close)/records[5].Close)*100, 4)
	}

	// 20-day (monthly) change
	if len(records) >= 21 && records[20].Close > 0 {
		dc.MonthlyChgPct = roundN(((records[0].Close-records[20].Close)/records[20].Close)*100, 4)
	}

	// Moving averages
	dc.DMA20 = computeSMA(records, 20)
	dc.DMA50 = computeSMA(records, 50)
	dc.DMA200 = computeSMA(records, 200)

	if dc.DMA20 > 0 {
		dc.AboveDMA20 = dc.CurrentPrice > dc.DMA20
	}
	if dc.DMA50 > 0 {
		dc.AboveDMA50 = dc.CurrentPrice > dc.DMA50
	}
	if dc.DMA200 > 0 {
		dc.AboveDMA200 = dc.CurrentPrice > dc.DMA200
	}

	// Daily ATR (14-day)
	dc.DailyATR = computeDailyATR(records, 14)
	if dc.CurrentPrice > 0 && dc.DailyATR > 0 {
		dc.NormalizedATR = roundN(dc.DailyATR/dc.CurrentPrice*100, 4)
	}

	// Daily trend (5-day)
	if len(records) >= 5 {
		dc.DailyTrend = computeDailyTrend(records[:5])
	}

	// Consecutive up/down days
	dc.ConsecDays, dc.ConsecDir = computeConsecutiveDays(records)

	// Momentum (rate of change)
	dc.Momentum5D = computeROC(records, 5)
	dc.Momentum10D = computeROC(records, 10)
	dc.Momentum20D = computeROC(records, 20)

	return dc, nil
}

// BuildAll computes daily price context for all COT contracts.
func (cb *DailyContextBuilder) BuildAll(ctx context.Context) (map[string]*domain.DailyPriceContext, error) {
	result := make(map[string]*domain.DailyPriceContext)

	for _, mapping := range domain.COTPriceSymbolMappings() {
		dc, err := cb.Build(ctx, mapping.ContractCode, mapping.Currency)
		if err != nil {
			log.Debug().Err(err).Str("contract", mapping.Currency).Msg("skipping daily price context")
			continue
		}
		result[mapping.ContractCode] = dc
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no daily price context available for any contract")
	}
	return result, nil
}

// --- Computation helpers ---

// computeSMA calculates Simple Moving Average from newest-first records.
func computeSMA(records []domain.DailyPrice, period int) float64 {
	if len(records) < period {
		return 0
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += records[i].Close
	}
	return roundN(sum/float64(period), 6)
}

// computeDailyATR calculates Average True Range from newest-first records.
func computeDailyATR(records []domain.DailyPrice, period int) float64 {
	if len(records) < period+1 {
		return 0
	}

	var atrSum float64
	for i := 0; i < period; i++ {
		curr := records[i]
		prev := records[i+1]

		tr := math.Max(
			curr.High-curr.Low,
			math.Max(
				math.Abs(curr.High-prev.Close),
				math.Abs(curr.Low-prev.Close),
			),
		)
		atrSum += tr
	}

	return roundN(atrSum/float64(period), 6)
}

// computeDailyTrend determines short-term trend direction from newest-first records.
func computeDailyTrend(records []domain.DailyPrice) string {
	if len(records) < 2 {
		return "FLAT"
	}
	newest := records[0].Close
	oldest := records[len(records)-1].Close
	if oldest == 0 {
		return "FLAT"
	}
	changePct := ((newest - oldest) / oldest) * 100
	if changePct > 0.3 {
		return "UP"
	} else if changePct < -0.3 {
		return "DOWN"
	}
	return "FLAT"
}

// computeConsecutiveDays counts consecutive up or down days from the most recent.
func computeConsecutiveDays(records []domain.DailyPrice) (int, string) {
	if len(records) < 2 {
		return 0, ""
	}

	// Determine initial direction
	firstChange := records[0].Close - records[1].Close
	if firstChange == 0 {
		return 0, ""
	}

	dir := "UP"
	if firstChange < 0 {
		dir = "DOWN"
	}

	count := 1
	for i := 1; i < len(records)-1; i++ {
		change := records[i].Close - records[i+1].Close
		if (dir == "UP" && change > 0) || (dir == "DOWN" && change < 0) {
			count++
		} else {
			break
		}
	}
	return count, dir
}

// computeROC calculates Rate of Change (momentum) as percentage.
func computeROC(records []domain.DailyPrice, period int) float64 {
	if len(records) <= period || records[period].Close == 0 {
		return 0
	}
	return roundN(((records[0].Close-records[period].Close)/records[period].Close)*100, 4)
}
