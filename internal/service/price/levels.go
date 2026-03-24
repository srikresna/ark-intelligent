package price

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// KeyLevel represents a support or resistance price level derived from daily data.
type KeyLevel struct {
	Price     float64 `json:"price"`
	Type      string  `json:"type"`      // "SUPPORT", "RESISTANCE", "PIVOT"
	Strength  int     `json:"strength"`  // 1-5 (how many times tested)
	Source    string  `json:"source"`    // e.g. "DMA200", "SWING_LOW", "CLUSTER"
	Distance  float64 `json:"distance"`  // % distance from current price
}

// LevelsContext holds computed support/resistance levels for a contract.
type LevelsContext struct {
	ContractCode string      `json:"contract_code"`
	Currency     string      `json:"currency"`
	CurrentPrice float64     `json:"current_price"`
	DailyATR     float64     `json:"daily_atr"`
	Levels       []KeyLevel  `json:"levels"`

	// Key levels summary
	NearestSupport    *KeyLevel `json:"nearest_support,omitempty"`
	NearestResistance *KeyLevel `json:"nearest_resistance,omitempty"`
	DailyPivot        float64   `json:"daily_pivot"`       // (H + L + C) / 3
	PivotR1           float64   `json:"pivot_r1"`          // 2*P - L
	PivotS1           float64   `json:"pivot_s1"`          // 2*P - H
	PivotR2           float64   `json:"pivot_r2"`          // P + (H - L)
	PivotS2           float64   `json:"pivot_s2"`          // P - (H - L)
}

// LevelsBuilder computes support/resistance levels from daily price data.
type LevelsBuilder struct {
	repo DailyPriceStore
}

// NewLevelsBuilder creates a new levels builder.
func NewLevelsBuilder(repo DailyPriceStore) *LevelsBuilder {
	return &LevelsBuilder{repo: repo}
}

// Build computes key levels for a single contract.
func (lb *LevelsBuilder) Build(ctx context.Context, contractCode, currency string) (*LevelsContext, error) {
	records, err := lb.repo.GetDailyHistory(ctx, contractCode, 220)
	if err != nil {
		return nil, fmt.Errorf("get daily history: %w", err)
	}
	if len(records) < 10 {
		return nil, fmt.Errorf("insufficient daily data for %s (need 10+, got %d)", contractCode, len(records))
	}

	// records are newest-first
	current := records[0].Close

	lc := &LevelsContext{
		ContractCode: contractCode,
		Currency:     currency,
		CurrentPrice: current,
	}

	// Compute daily ATR (14-day)
	lc.DailyATR = computeDailyATR(records, 14)

	// 1. Daily pivot points from latest bar
	latest := records[0]
	lc.DailyPivot = roundN((latest.High+latest.Low+latest.Close)/3, 6)
	lc.PivotR1 = roundN(2*lc.DailyPivot-latest.Low, 6)
	lc.PivotS1 = roundN(2*lc.DailyPivot-latest.High, 6)
	lc.PivotR2 = roundN(lc.DailyPivot+(latest.High-latest.Low), 6)
	lc.PivotS2 = roundN(lc.DailyPivot-(latest.High-latest.Low), 6)

	var levels []KeyLevel

	// 2. Moving average levels
	dma20 := computeSMA(records, 20)
	dma50 := computeSMA(records, 50)
	dma200 := computeSMA(records, 200)

	if dma20 > 0 {
		levels = append(levels, KeyLevel{
			Price:    dma20,
			Type:     classifyLevel(current, dma20),
			Strength: 2,
			Source:   "DMA20",
			Distance: pctDistance(current, dma20),
		})
	}
	if dma50 > 0 {
		levels = append(levels, KeyLevel{
			Price:    dma50,
			Type:     classifyLevel(current, dma50),
			Strength: 3,
			Source:   "DMA50",
			Distance: pctDistance(current, dma50),
		})
	}
	if dma200 > 0 {
		levels = append(levels, KeyLevel{
			Price:    dma200,
			Type:     classifyLevel(current, dma200),
			Strength: 4,
			Source:   "DMA200",
			Distance: pctDistance(current, dma200),
		})
	}

	// 3. Swing highs and lows (local extremes in 20-day windows)
	swingLevels := findSwingLevels(records, current)
	levels = append(levels, swingLevels...)

	// 4. Pivot levels
	levels = append(levels,
		KeyLevel{Price: lc.PivotR1, Type: "RESISTANCE", Strength: 1, Source: "PIVOT_R1", Distance: pctDistance(current, lc.PivotR1)},
		KeyLevel{Price: lc.PivotS1, Type: "SUPPORT", Strength: 1, Source: "PIVOT_S1", Distance: pctDistance(current, lc.PivotS1)},
		KeyLevel{Price: lc.PivotR2, Type: "RESISTANCE", Strength: 1, Source: "PIVOT_R2", Distance: pctDistance(current, lc.PivotR2)},
		KeyLevel{Price: lc.PivotS2, Type: "SUPPORT", Strength: 1, Source: "PIVOT_S2", Distance: pctDistance(current, lc.PivotS2)},
	)

	// 5. Recent high/low range
	if len(records) >= 20 {
		rangeHigh, rangeLow := rangeHighLow(records[:20])
		levels = append(levels,
			KeyLevel{Price: rangeHigh, Type: "RESISTANCE", Strength: 3, Source: "20D_HIGH", Distance: pctDistance(current, rangeHigh)},
			KeyLevel{Price: rangeLow, Type: "SUPPORT", Strength: 3, Source: "20D_LOW", Distance: pctDistance(current, rangeLow)},
		)
	}

	// Sort by distance from current price
	sort.Slice(levels, func(i, j int) bool {
		return math.Abs(levels[i].Distance) < math.Abs(levels[j].Distance)
	})

	// Deduplicate close levels (within 0.1% of each other)
	levels = deduplicateLevels(levels, 0.1)

	lc.Levels = levels

	// Find nearest support and resistance
	for i := range levels {
		if levels[i].Type == "SUPPORT" && lc.NearestSupport == nil {
			l := levels[i]
			lc.NearestSupport = &l
		}
		if levels[i].Type == "RESISTANCE" && lc.NearestResistance == nil {
			l := levels[i]
			lc.NearestResistance = &l
		}
		if lc.NearestSupport != nil && lc.NearestResistance != nil {
			break
		}
	}

	return lc, nil
}

// --- helpers ---

func classifyLevel(current, level float64) string {
	if current == 0 {
		return "SUPPORT"
	}
	// Within 0.01% of current price — treat as support (immediate floor)
	if math.Abs((level-current)/current*100) < 0.01 {
		return "SUPPORT"
	}
	if level < current {
		return "SUPPORT"
	}
	return "RESISTANCE"
}

func pctDistance(current, level float64) float64 {
	if current == 0 {
		return 0
	}
	return roundN((level-current)/current*100, 4)
}

func rangeHighLow(records []domain.DailyPrice) (float64, float64) {
	high := records[0].High
	low := records[0].Low
	for _, r := range records[1:] {
		if r.High > high {
			high = r.High
		}
		if r.Low < low {
			low = r.Low
		}
	}
	return high, low
}

// findSwingLevels detects local highs/lows using a 5-bar lookback/lookahead.
func findSwingLevels(records []domain.DailyPrice, current float64) []KeyLevel {
	var levels []KeyLevel
	window := 5

	if len(records) < 2*window+1 {
		return levels
	}

	seen := make(map[float64]bool)

	for i := window; i < len(records)-window && i < 90; i++ {
		// Swing high: records[i] high is higher than surrounding bars
		isSwingHigh := true
		for j := i - window; j <= i+window; j++ {
			if j == i {
				continue
			}
			if records[j].High >= records[i].High {
				isSwingHigh = false
				break
			}
		}

		if isSwingHigh {
			p := roundN(records[i].High, 6)
			if !seen[p] {
				seen[p] = true
				levels = append(levels, KeyLevel{
					Price:    p,
					Type:     classifyLevel(current, p),
					Strength: 2,
					Source:   "SWING_HIGH",
					Distance: pctDistance(current, p),
				})
			}
		}

		// Swing low
		isSwingLow := true
		for j := i - window; j <= i+window; j++ {
			if j == i {
				continue
			}
			if records[j].Low <= records[i].Low {
				isSwingLow = false
				break
			}
		}

		if isSwingLow {
			p := roundN(records[i].Low, 6)
			if !seen[p] {
				seen[p] = true
				levels = append(levels, KeyLevel{
					Price:    p,
					Type:     classifyLevel(current, p),
					Strength: 2,
					Source:   "SWING_LOW",
					Distance: pctDistance(current, p),
				})
			}
		}
	}

	return levels
}

// deduplicateLevels merges levels that are within thresholdPct of each other.
// Keeps the one with higher strength.
func deduplicateLevels(levels []KeyLevel, thresholdPct float64) []KeyLevel {
	if len(levels) <= 1 {
		return levels
	}

	var result []KeyLevel
	used := make([]bool, len(levels))

	for i := range levels {
		if used[i] {
			continue
		}
		best := levels[i]
		for j := i + 1; j < len(levels); j++ {
			if used[j] {
				continue
			}
			if best.Price > 0 && math.Abs((levels[j].Price-best.Price)/best.Price*100) < thresholdPct {
				used[j] = true
				if levels[j].Strength > best.Strength {
					best = levels[j]
				} else if levels[j].Strength == best.Strength && best.Strength < 5 {
					// Cluster confirmation: bump strength (capped at 5)
					best.Strength++
				}
			}
		}
		result = append(result, best)
	}

	return result
}
