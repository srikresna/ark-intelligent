package fred

import (
	"context"
	"math"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// RateDifferentialEngine computes interest rate differentials and carry trade scores.
type RateDifferentialEngine struct {
	client *http.Client
	apiKey string
}

// NewRateDifferentialEngine creates a new rate differential engine.
func NewRateDifferentialEngine() *RateDifferentialEngine {
	return &RateDifferentialEngine{
		client: httpclient.New(),
		apiKey: os.Getenv("FRED_API_KEY"),
	}
}

// FetchCarryRanking fetches policy rates for all major currencies and computes
// interest rate differentials relative to USD.
func (e *RateDifferentialEngine) FetchCarryRanking(ctx context.Context) (*domain.CarryRanking, error) {
	rates := make(map[string]float64)

	// Fetch each currency's policy rate from FRED
	for cur, info := range domain.CentralBankRateMapping {
		rate := e.fetchRate(ctx, info)
		if rate != 0 {
			rates[cur] = rate
		} else {
			log.Warn().
				Str("currency", cur).
				Str("fred_series", info.FREDSeries).
				Str("fallback_series", info.FallbackSeries).
				Msg("carry rate returned 0 — FRED series may have failed")
		}
	}

	usRate, ok := rates["USD"]
	if !ok || usRate == 0 {
		// Fallback: use SOFR or assume from FRED MacroData
		usRate = e.fetchRate(ctx, domain.CentralBankRateInfo{
			FREDSeries:     "SOFR",
			FallbackSeries: "DFF",
		})
		if usRate == 0 {
			usRate = 5.25 // Fallback: approximate current Fed Funds rate; should be fetched from config in production
		}
		rates["USD"] = usRate
	}

	// Compute differentials for each FX pair (vs USD)
	fxCurrencies := []string{"EUR", "GBP", "JPY", "CHF", "AUD", "CAD", "NZD"}
	var pairs []domain.RateDifferential

	for _, cur := range fxCurrencies {
		quoteRate, ok := rates[cur]
		if !ok {
			continue
		}

		// For XXX/USD pairs (EUR/USD, GBP/USD, AUD/USD, NZD/USD):
		//   Positive diff = earn carry going LONG (buy XXX, sell USD)
		// For USD/XXX pairs (USD/JPY, USD/CHF, USD/CAD) — handled by Inverse flag
		//   We still express diff as quoteRate - baseRate for consistency

		diff := quoteRate - usRate

		// Carry score: normalized to -100..+100
		// 0 diff = 0 score; ±5% diff maps to ±100
		carryScore := clampFloat(diff*20, -100, 100)

		direction := "LONG"
		if diff < 0 {
			direction = "SHORT"
		}

		pairs = append(pairs, domain.RateDifferential{
			Currency:     cur,
			BaseCurrency: "USD",
			BaseRate:     roundN(usRate, 3),
			QuoteRate:    roundN(quoteRate, 3),
			Differential: roundN(diff, 3),
			CarryScore:   roundN(carryScore, 1),
			Direction:    direction,
		})
	}

	// Sort by carry score descending (most attractive carry first)
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].CarryScore > pairs[j].CarryScore
	})

	ranking := &domain.CarryRanking{
		Pairs:  pairs,
		USRate: roundN(usRate, 3),
		AsOf:   time.Now().Format("2006-01-02"),
	}

	if len(pairs) > 0 {
		ranking.BestCarry = pairs[0].Currency
		ranking.WorstCarry = pairs[len(pairs)-1].Currency
	}

	return ranking, nil
}

// fetchRate fetches a single rate from FRED, trying primary then fallback series.
func (e *RateDifferentialEngine) fetchRate(ctx context.Context, info domain.CentralBankRateInfo) float64 {
	if info.FREDSeries != "" {
		obs := fetchSeries(ctx, e.client, info.FREDSeries, e.apiKey, 5)
		if len(obs) > 0 {
			return obs[0]
		}
	}

	if info.FallbackSeries != "" {
		obs := fetchSeries(ctx, e.client, info.FallbackSeries, e.apiKey, 5)
		if len(obs) > 0 {
			return obs[0]
		}
	}

	return 0
}

// CarryAdjustment returns a confidence adjustment for a signal based on carry trade alignment.
// Positive carry aligned with signal direction boosts confidence; opposed reduces it.
func CarryAdjustment(diff domain.RateDifferential, signalDirection string) float64 {
	// Carry aligned with signal = favorable
	if signalDirection == "BULLISH" && diff.Differential > 0 {
		return math.Min(diff.Differential*2, 5) // Up to +5 boost
	}
	if signalDirection == "BEARISH" && diff.Differential < 0 {
		return math.Min(math.Abs(diff.Differential)*2, 5)
	}

	// Carry opposed to signal = headwind
	if signalDirection == "BULLISH" && diff.Differential < -1.0 {
		return math.Max(diff.Differential*1.5, -5) // Up to -5 penalty
	}
	if signalDirection == "BEARISH" && diff.Differential > 1.0 {
		return -math.Min(diff.Differential*1.5, 5)
	}

	return 0 // Neutral carry
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func roundN(v float64, n int) float64 {
	pow := math.Pow(10, float64(n))
	return math.Round(v*pow) / pow
}
