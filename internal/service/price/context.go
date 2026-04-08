package price

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// COT threshold constants — mirrors cot.COTBullishThreshold / cot.COTBearishThreshold.
// Defined locally to avoid circular import (price → cot → domain, price → domain).
const (
	cotBullishThreshold = 60
	cotBearishThreshold = 40
)

// ContextBuilder computes price context from stored price records.
type ContextBuilder struct {
	priceRepo ports.PriceRepository
}

// NewContextBuilder creates a new price context builder.
func NewContextBuilder(priceRepo ports.PriceRepository) *ContextBuilder {
	return &ContextBuilder{priceRepo: priceRepo}
}

// Build computes price context for a single contract.
func (cb *ContextBuilder) Build(ctx context.Context, contractCode, currency string) (*domain.PriceContext, error) {
	// Get 21 weeks of history for MA (13W) and ATR (20W) calculations
	records, err := cb.priceRepo.GetHistory(ctx, contractCode, 21)
	if err != nil {
		return nil, fmt.Errorf("get price history: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no price data for %s", contractCode)
	}

	// records are newest-first
	pc := &domain.PriceContext{
		ContractCode: contractCode,
		Currency:     currency,
		CurrentPrice: records[0].Close,
	}

	// Weekly change (latest vs previous week)
	if len(records) >= 2 && records[1].Close > 0 {
		pc.WeeklyChgPct = roundN(((records[0].Close-records[1].Close)/records[1].Close)*100, 4)
	}

	// Monthly change (latest vs ~4 weeks ago)
	if len(records) >= 5 && records[4].Close > 0 {
		pc.MonthlyChgPct = roundN(((records[0].Close-records[4].Close)/records[4].Close)*100, 4)
	}

	// 4-week moving average
	if len(records) >= 4 {
		sum := 0.0
		for i := 0; i < 4; i++ {
			sum += records[i].Close
		}
		pc.PriceMA4W = roundN(sum/4, 6)
		pc.AboveMA4W = pc.CurrentPrice > pc.PriceMA4W
	}

	// 13-week moving average
	if len(records) >= 13 {
		sum := 0.0
		for i := 0; i < 13; i++ {
			sum += records[i].Close
		}
		pc.PriceMA13W = roundN(sum/13, 6)
		pc.AboveMA13W = pc.CurrentPrice > pc.PriceMA13W
	}

	// 4-week trend (linear regression slope direction)
	if len(records) >= 4 {
		pc.Trend4W = computeTrend(records[:4])
	}

	// 13-week trend
	if len(records) >= 13 {
		pc.Trend13W = computeTrend(records[:13])
	}

	// ATR-based volatility context (requires at least 2 bars for True Range)
	if vc := ComputeVolatilityContext(records); vc != nil {
		pc.VolatilityRegime = vc.Regime
		pc.ATR = vc.ATR20W
		pc.NormalizedATR = vc.NormalizedATR
		pc.VolatilityMultiplier = vc.ConfidenceMultiplier
	}

	// Price regime classification (TRENDING / RANGING / CRISIS)
	if regime := ClassifyPriceRegime(records, pc); regime != nil {
		pc.PriceRegime = regime.Regime
		pc.ADX = regime.ADX
	}

	return pc, nil
}

// BuildAll computes price context for all default contracts.
func (cb *ContextBuilder) BuildAll(ctx context.Context) (map[string]*domain.PriceContext, error) {
	result := make(map[string]*domain.PriceContext)

	// Use COTPriceSymbolMappings to exclude risk-only instruments (VIX, SPX).
	// VIX/SPX context is built separately via RiskContextBuilder.
	for _, mapping := range domain.COTPriceSymbolMappings() {
		pc, err := cb.Build(ctx, mapping.ContractCode, mapping.Currency)
		if err != nil {
			log.Debug().Err(err).Str("contract", mapping.Currency).Msg("skipping price context")
			continue
		}
		result[mapping.ContractCode] = pc
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no price context available for any contract")
	}
	return result, nil
}

// CurrencyStrength represents a currency's combined ranking.
type CurrencyStrength struct {
	ContractCode  string  `json:"contract_code"`
	Currency      string  `json:"currency"`
	PriceScore    float64 `json:"price_score"`    // Momentum-based score
	COTScore      float64 `json:"cot_score"`      // COT positioning score
	CombinedScore float64 `json:"combined_score"` // Weighted average
	PriceRank     int     `json:"price_rank"`     // 1 = strongest
	COTRank       int     `json:"cot_rank"`
	CombinedRank  int     `json:"combined_rank"`
	Divergence    bool    `json:"divergence"` // Price trend contradicts COT
	DivergenceMsg string  `json:"divergence_msg,omitempty"`
}

// ComputeCurrencyStrengthIndex produces a dual ranking by price momentum + COT positioning.
func ComputeCurrencyStrengthIndex(
	priceContexts map[string]*domain.PriceContext,
	analyses []domain.COTAnalysis,
) []CurrencyStrength {
	// Build analysis lookup
	analysisMap := make(map[string]*domain.COTAnalysis, len(analyses))
	for i := range analyses {
		analysisMap[analyses[i].Contract.Code] = &analyses[i]
	}

	var strengths []CurrencyStrength

	for code, pc := range priceContexts {
		analysis := analysisMap[code]
		if analysis == nil {
			continue
		}

		cs := CurrencyStrength{
			ContractCode: code,
			Currency:     pc.Currency,
		}

		// Price score: unified formula (same as V3 confluence)
		// MA alignment: ±25 per MA
		maScore := 0.0
		if pc.AboveMA4W {
			maScore += 25
		} else {
			maScore -= 25
		}
		if pc.AboveMA13W {
			maScore += 25
		} else {
			maScore -= 25
		}
		// Momentum: volatility-normalized if ATR available
		weeklyMom := pc.WeeklyChgPct
		monthlyMom := pc.MonthlyChgPct
		if pc.NormalizedATR > 0 {
			weeklyMom = weeklyMom / pc.NormalizedATR * 2
			monthlyMom = monthlyMom / pc.NormalizedATR * 2
		} else {
			weeklyMom = weeklyMom * 10
			monthlyMom = monthlyMom * 5
		}
		momentumScore := clampF(weeklyMom, -25, 25) + clampF(monthlyMom, -25, 25)
		cs.PriceScore = clampF(maScore+momentumScore, -100, 100)

		// COT score: use COTIndex (0-100) centered at 50
		cs.COTScore = analysis.COTIndex - 50

		// Combined: 40% price + 60% COT
		cs.CombinedScore = cs.PriceScore*0.4 + cs.COTScore*0.6

		// Divergence detection
		priceBullish := pc.Trend4W == "UP"
		cotBullish := analysis.COTIndex > cotBullishThreshold
		priceBearish := pc.Trend4W == "DOWN"
		cotBearish := analysis.COTIndex < cotBearishThreshold

		if priceBullish && cotBearish {
			cs.Divergence = true
			cs.DivergenceMsg = "Price rising but smart money reducing longs"
		} else if priceBearish && cotBullish {
			cs.Divergence = true
			cs.DivergenceMsg = "Price falling but smart money accumulating"
		}

		strengths = append(strengths, cs)
	}

	// Rank by price score (descending)
	sort.Slice(strengths, func(i, j int) bool {
		return strengths[i].PriceScore > strengths[j].PriceScore
	})
	for i := range strengths {
		strengths[i].PriceRank = i + 1
	}

	// Rank by COT score (descending)
	sort.Slice(strengths, func(i, j int) bool {
		return strengths[i].COTScore > strengths[j].COTScore
	})
	for i := range strengths {
		strengths[i].COTRank = i + 1
	}

	// Rank by combined score (descending)
	sort.Slice(strengths, func(i, j int) bool {
		return strengths[i].CombinedScore > strengths[j].CombinedScore
	})
	for i := range strengths {
		strengths[i].CombinedRank = i + 1
	}

	return strengths
}

// computeTrend determines trend direction from newest-first price records.
// Uses simple comparison: if most recent > oldest, UP; otherwise DOWN.
func computeTrend(records []domain.PriceRecord) string {
	if len(records) < 2 {
		return "FLAT"
	}
	// records[0] is newest, records[len-1] is oldest
	newest := records[0].Close
	oldest := records[len(records)-1].Close
	if oldest == 0 {
		return "FLAT"
	}

	changePct := ((newest - oldest) / oldest) * 100
	if changePct > 0.5 {
		return "UP"
	} else if changePct < -0.5 {
		return "DOWN"
	}
	return "FLAT"
}

func roundN(v float64, n int) float64 {
	pow := math.Pow(10, float64(n))
	return math.Round(v*pow) / pow
}

func clampF(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
