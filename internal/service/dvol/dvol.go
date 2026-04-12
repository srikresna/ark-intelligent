// Package dvol provides Deribit Volatility Index (DVOL) analysis.
// DVOL is the crypto-native VIX equivalent — a 30-day implied volatility index
// computed by Deribit from BTC and ETH options.
//
// Key metrics:
//   - DVOL current level (annualized IV)
//   - 24h change (spike detection)
//   - IV–HV spread (implied vs realized, premium/discount)
//   - Comparison scaffold for cross-asset vol (DVOL vs CBOE VIX)
package dvol

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/deribit"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("dvol")

const cacheTTL = 1 * time.Hour

// CurrencyDVOL holds DVOL analysis for a single currency (BTC or ETH).
type CurrencyDVOL struct {
	Currency     string  // "BTC" or "ETH"
	Current      float64 // Current DVOL level (annualized %)
	High24h      float64 // 24h high
	Low24h       float64 // 24h low
	Open24h      float64 // 24h open
	Change24h    float64 // Absolute change in DVOL points
	Change24hPct float64 // Percentage change from 24h ago
	HV           float64 // Latest realized (historical) vol (annualized %)
	IVHVSpread   float64 // IV - HV (positive = IV premium, negative = IV discount)
	IVHVRatio    float64 // IV / HV (>1 = fear premium, <1 = complacency)
	Spike        bool    // True if |Change24hPct| > 20%
	Available    bool
}

// DVOLResult holds aggregated DVOL data for all currencies.
type DVOLResult struct {
	BTC       CurrencyDVOL
	ETH       CurrencyDVOL
	Available bool
	FetchedAt time.Time
}

type cachedDVOL struct {
	result    *DVOLResult
	fetchedAt time.Time
}

// Fetcher handles DVOL data retrieval with caching.
type Fetcher struct {
	client *deribit.Client

	mu    sync.Mutex
	cache *cachedDVOL
}

// NewFetcher creates a DVOL Fetcher with a fresh Deribit client.
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: deribit.NewClient(),
	}
}

// Fetch retrieves DVOL data for BTC and ETH. Results are cached for 1 hour.
func (f *Fetcher) Fetch(ctx context.Context) (*DVOLResult, error) {
	// Check cache
	f.mu.Lock()
	if f.cache != nil && time.Since(f.cache.fetchedAt) < cacheTTL {
		r := f.cache.result
		f.mu.Unlock()
		return r, nil
	}
	f.mu.Unlock()

	log.Info().Msg("fetching DVOL data from Deribit")

	result := &DVOLResult{
		FetchedAt: time.Now(),
	}

	// Fetch BTC and ETH DVOL in sequence (Deribit rate limit is generous for public)
	result.BTC = f.fetchCurrency(ctx, "BTC")
	result.ETH = f.fetchCurrency(ctx, "ETH")

	result.Available = result.BTC.Available || result.ETH.Available

	// Store cache
	f.mu.Lock()
	f.cache = &cachedDVOL{result: result, fetchedAt: time.Now()}
	f.mu.Unlock()

	return result, nil
}

// fetchCurrency fetches DVOL + HV for a single currency and computes metrics.
func (f *Fetcher) fetchCurrency(ctx context.Context, currency string) CurrencyDVOL {
	cd := CurrencyDVOL{Currency: currency}

	// 1. Fetch DVOL candles (1h resolution, last 24h)
	candles, err := f.client.GetDVOL(ctx, currency, "3600")
	if err != nil {
		log.Warn().Err(err).Str("currency", currency).Msg("DVOL fetch failed")
		return cd
	}
	if len(candles) == 0 {
		log.Warn().Str("currency", currency).Msg("DVOL: no candle data returned")
		return cd
	}

	// Sort by timestamp ascending
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Timestamp < candles[j].Timestamp
	})

	// Latest candle = current DVOL
	latest := candles[len(candles)-1]
	cd.Current = latest.Close

	// Compute 24h range
	cd.Open24h = candles[0].Open
	cd.High24h = candles[0].High
	cd.Low24h = candles[0].Low
	for _, c := range candles {
		if c.High > cd.High24h {
			cd.High24h = c.High
		}
		if c.Low < cd.Low24h || cd.Low24h == 0 {
			cd.Low24h = c.Low
		}
	}

	// 24h change
	cd.Change24h = cd.Current - cd.Open24h
	if cd.Open24h > 0 {
		cd.Change24hPct = (cd.Change24h / cd.Open24h) * 100
	}

	// Spike detection: >20% change in 24h
	cd.Spike = math.Abs(cd.Change24hPct) > 20

	// 2. Fetch historical volatility
	hvs, err := f.client.GetHistoricalVolatility(ctx, currency)
	if err != nil {
		log.Warn().Err(err).Str("currency", currency).Msg("HV fetch failed")
		// DVOL is still valid even without HV
		cd.Available = true
		return cd
	}
	if len(hvs) > 0 {
		// Latest HV reading (Deribit returns sorted by time, last is most recent)
		latestHV := hvs[len(hvs)-1]
		cd.HV = latestHV.Value * 100 // Convert from decimal (0.65) to percentage (65%)

		// IV-HV spread and ratio
		if cd.HV > 0 {
			cd.IVHVSpread = cd.Current - cd.HV
			cd.IVHVRatio = cd.Current / cd.HV
		}
	}

	cd.Available = true

	log.Info().
		Str("currency", currency).
		Float64("dvol", cd.Current).
		Float64("change_24h_pct", cd.Change24hPct).
		Float64("hv", cd.HV).
		Float64("iv_hv_spread", cd.IVHVSpread).
		Bool("spike", cd.Spike).
		Msg("DVOL analysis complete")

	return cd
}

// SpreadSignal returns a human-readable interpretation of the IV-HV spread.
func SpreadSignal(ivhvRatio float64) string {
	switch {
	case ivhvRatio >= 1.5:
		return "EXTREME FEAR PREMIUM"
	case ivhvRatio >= 1.2:
		return "FEAR PREMIUM"
	case ivhvRatio >= 0.9:
		return "NORMAL"
	case ivhvRatio >= 0.7:
		return "COMPLACENT"
	default:
		return "EXTREME COMPLACENCY"
	}
}

// FormatDVOLBrief returns a one-line summary for embedding in other views.
func FormatDVOLBrief(result *DVOLResult) string {
	if !result.Available {
		return ""
	}
	var parts []string
	if result.BTC.Available {
		arrow := "→"
		if result.BTC.Change24hPct > 5 {
			arrow = "↑"
		} else if result.BTC.Change24hPct < -5 {
			arrow = "↓"
		}
		parts = append(parts, fmt.Sprintf("BTC DVOL: %.1f%% %s (%+.1f%%)", result.BTC.Current, arrow, result.BTC.Change24hPct))
	}
	if result.ETH.Available {
		arrow := "→"
		if result.ETH.Change24hPct > 5 {
			arrow = "↑"
		} else if result.ETH.Change24hPct < -5 {
			arrow = "↓"
		}
		parts = append(parts, fmt.Sprintf("ETH DVOL: %.1f%% %s (%+.1f%%)", result.ETH.Current, arrow, result.ETH.Change24hPct))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%s", joinParts(parts))
}

func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += " | " + p
	}
	return result
}
