package price

import (
	"context"
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// IntradayContextBuilder computes intraday price context from stored bars.
type IntradayContextBuilder struct {
	repo IntradayStore
}

// IntradayStore defines the storage interface needed by IntradayContextBuilder.
type IntradayStore interface {
	GetHistory(ctx context.Context, contractCode, interval string, barCount int) ([]domain.IntradayBar, error)
}

// NewIntradayContextBuilder creates a new intraday context builder.
func NewIntradayContextBuilder(repo IntradayStore) *IntradayContextBuilder {
	return &IntradayContextBuilder{repo: repo}
}

// Build computes intraday context for a single contract.
// Requires ~60 bars of 4H data (≈10 days) for IMA55.
func (cb *IntradayContextBuilder) Build(ctx context.Context, contractCode, currency string) (*domain.IntradayContext, error) {
	bars, err := cb.repo.GetHistory(ctx, contractCode, "4h", 60)
	if err != nil {
		return nil, fmt.Errorf("get intraday history: %w", err)
	}
	if len(bars) < 2 {
		return nil, fmt.Errorf("insufficient intraday data for %s (%d bars)", contractCode, len(bars))
	}

	// bars are newest-first
	ic := &domain.IntradayContext{
		ContractCode: contractCode,
		Currency:     currency,
		Interval:     "4h",
		CurrentPrice: bars[0].Close,
		AsOf:         bars[0].Timestamp,
	}

	// 4H change (latest bar)
	if len(bars) >= 2 && bars[1].Close > 0 {
		ic.Chg4H = roundN(((bars[0].Close - bars[1].Close) / bars[1].Close) * 100, 4)
	}

	// 12H change (3 bars)
	if len(bars) >= 4 && bars[3].Close > 0 {
		ic.Chg12H = roundN(((bars[0].Close - bars[3].Close) / bars[3].Close) * 100, 4)
	}

	// 24H change (6 bars)
	if len(bars) >= 7 && bars[6].Close > 0 {
		ic.Chg24H = roundN(((bars[0].Close - bars[6].Close) / bars[6].Close) * 100, 4)
	}

	// Intraday Moving Averages
	ic.IMA8 = computeIntradaySMA(bars, 8)
	ic.IMA21 = computeIntradaySMA(bars, 21)
	ic.IMA55 = computeIntradaySMA(bars, 55)

	if ic.IMA8 > 0 {
		ic.AboveIMA8 = ic.CurrentPrice > ic.IMA8
	}
	if ic.IMA21 > 0 {
		ic.AboveIMA21 = ic.CurrentPrice > ic.IMA21
	}
	if ic.IMA55 > 0 {
		ic.AboveIMA55 = ic.CurrentPrice > ic.IMA55
	}

	// Intraday ATR (14-bar)
	ic.IntradayATR = computeIntradayATR(bars, 14)
	if ic.CurrentPrice > 0 && ic.IntradayATR > 0 {
		ic.NormalizedIATR = roundN(ic.IntradayATR/ic.CurrentPrice*100, 4)
	}

	// Intraday trend (6-bar / 24h)
	if len(bars) >= 6 {
		ic.IntradayTrend = computeIntradayTrend(bars[:6])
	}

	// Momentum
	ic.Momentum6 = computeIntradayROC(bars, 6)
	ic.Momentum12 = computeIntradayROC(bars, 12)

	// Session high/low (last 6 bars ≈ 24h)
	sessLen := 6
	if len(bars) < sessLen {
		sessLen = len(bars)
	}
	ic.SessionHigh, ic.SessionLow = computeSessionRange(bars[:sessLen])

	return ic, nil
}

// BuildAll computes intraday context for all COT contracts.
func (cb *IntradayContextBuilder) BuildAll(ctx context.Context) (map[string]*domain.IntradayContext, error) {
	result := make(map[string]*domain.IntradayContext)

	for _, mapping := range domain.COTPriceSymbolMappings() {
		ic, err := cb.Build(ctx, mapping.ContractCode, mapping.Currency)
		if err != nil {
			continue
		}
		result[mapping.ContractCode] = ic
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no intraday context available for any contract")
	}
	return result, nil
}

// --- Computation helpers ---

func computeIntradaySMA(bars []domain.IntradayBar, period int) float64 {
	if len(bars) < period {
		return 0
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += bars[i].Close
	}
	return roundN(sum/float64(period), 6)
}

func computeIntradayATR(bars []domain.IntradayBar, period int) float64 {
	if len(bars) < period+1 {
		return 0
	}

	var atrSum float64
	for i := 0; i < period; i++ {
		curr := bars[i]
		prev := bars[i+1]

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

func computeIntradayTrend(bars []domain.IntradayBar) string {
	if len(bars) < 2 {
		return "FLAT"
	}
	newest := bars[0].Close
	oldest := bars[len(bars)-1].Close
	if oldest == 0 {
		return "FLAT"
	}
	changePct := ((newest - oldest) / oldest) * 100
	if changePct > 0.15 {
		return "UP"
	} else if changePct < -0.15 {
		return "DOWN"
	}
	return "FLAT"
}

func computeIntradayROC(bars []domain.IntradayBar, period int) float64 {
	if len(bars) <= period || bars[period].Close == 0 {
		return 0
	}
	return roundN(((bars[0].Close - bars[period].Close) / bars[period].Close) * 100, 4)
}

func computeSessionRange(bars []domain.IntradayBar) (float64, float64) {
	if len(bars) == 0 {
		return 0, 0
	}
	high := bars[0].High
	low := bars[0].Low
	for _, b := range bars[1:] {
		if b.High > high {
			high = b.High
		}
		if b.Low < low {
			low = b.Low
		}
	}
	return high, low
}
