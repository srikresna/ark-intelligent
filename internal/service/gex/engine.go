// Package gex implements Gamma Exposure (GEX) analysis using Deribit options data.
package gex

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/deribit"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("gex")

const (
	cacheTTL = 15 * time.Minute
)

// cachedResult wraps a GEXResult with a timestamp for cache invalidation.
type cachedResult struct {
	result    *GEXResult
	fetchedAt time.Time
}

// Engine performs GEX analysis against Deribit's public options API.
// Results are cached for cacheTTL to avoid hammering the API.
type Engine struct {
	client *deribit.Client

	mu    sync.Mutex
	cache map[string]*cachedResult // "BTC" | "ETH" → cached result
}

// NewEngine creates a GEX Engine with a fresh Deribit client.
func NewEngine() *Engine {
	return &Engine{
		client: deribit.NewClient(),
		cache:  make(map[string]*cachedResult),
	}
}

// Analyze fetches Deribit options data for symbol (e.g. "BTC" or "ETH"),
// computes the full gamma exposure profile, and returns a GEXResult.
// Results are cached for 15 minutes.
func (e *Engine) Analyze(ctx context.Context, symbol string) (*GEXResult, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))

	// Cache check
	if cached := e.fromCache(sym); cached != nil {
		return cached, nil
	}

	log.Info().Str("symbol", sym).Msg("fetching GEX data from Deribit")

	// 1. Instruments (for contract metadata)
	instruments, err := e.client.GetInstruments(ctx, sym)
	if err != nil {
		return nil, fmt.Errorf("gex: get instruments: %w", err)
	}
	if len(instruments) == 0 {
		return nil, fmt.Errorf("gex: no active instruments for %s", sym)
	}

	// 2. Book summary (OI + underlying price)
	summaries, err := e.client.GetBookSummary(ctx, sym)
	if err != nil {
		return nil, fmt.Errorf("gex: get book summary: %w", err)
	}

	// Index by instrument name for fast lookup
	instrMap := make(map[string]deribit.Instrument, len(instruments))
	for _, inst := range instruments {
		instrMap[inst.InstrumentName] = inst
	}
	summaryMap := make(map[string]deribit.BookSummary, len(summaries))
	for _, s := range summaries {
		summaryMap[s.InstrumentName] = s
	}

	// Derive spot price from the first book summary entry that has one
	spot := 0.0
	for _, s := range summaries {
		if s.UnderlyingPrice > 0 {
			spot = s.UnderlyingPrice
			break
		}
	}
	if spot <= 0 {
		// Fallback 1: try to get the Deribit index price directly.
		// This is the canonical spot source — far more reliable than MarkPrice.
		log.Warn().Str("symbol", sym).Msg("no UnderlyingPrice in book summaries, fetching index price as fallback")
		indexSpot, indexErr := e.client.GetIndexPrice(ctx, sym)
		if indexErr == nil && indexSpot > 0 {
			spot = indexSpot
			log.Info().Str("symbol", sym).Float64("spot", spot).Msg("using Deribit index price as spot fallback")
		} else {
			log.Warn().Str("symbol", sym).Err(indexErr).Msg("index price fallback failed, will retry from ticker data")
		}
	}

	// 3. Fetch per-instrument gammas (batch with reasonable limit)
	// Deribit allows 10 req/s; we sample at most 200 instruments to stay safe.
	callGamma := make(map[float64]float64) // strike → weighted avg gamma
	callOI := make(map[float64]float64)    // strike → total call OI
	putGamma := make(map[float64]float64)
	putOI := make(map[float64]float64)
	callCount := make(map[float64]int)
	putCount := make(map[float64]int)

	contractSize := 1.0 // Deribit BTC/ETH options: 1 contract = 1 underlying

	fetched := 0
	for name, inst := range instrMap {
		if fetched >= 200 {
			break
		}
		sum, ok := summaryMap[name]
		if !ok || sum.OpenInterest <= 0 {
			continue
		}

		ticker, err := e.client.GetTicker(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("instrument", name).Msg("ticker fetch failed, skipping")
			continue
		}
		fetched++

		if ticker.Gamma <= 0 {
			continue
		}

		// Update spot from ticker if better (ticker UnderlyingPrice is the most reliable source)
		if ticker.UnderlyingPrice > 0 && spot <= 0 {
			spot = ticker.UnderlyingPrice
			log.Info().Str("symbol", sym).Float64("spot", spot).Str("instrument", name).Msg("spot price recovered from ticker UnderlyingPrice")
		} else if ticker.UnderlyingPrice > 0 {
			// Keep refreshing spot with the latest value for accuracy
			spot = ticker.UnderlyingPrice
		}

		k := inst.Strike
		if inst.OptionType == "call" {
			callGamma[k] += ticker.Gamma
			callOI[k] += sum.OpenInterest
			callCount[k]++
		} else {
			putGamma[k] += ticker.Gamma
			putOI[k] += sum.OpenInterest
			putCount[k]++
		}
	}

	// Average gamma across multiple expiries at same strike
	for k, cnt := range callCount {
		if cnt > 1 {
			callGamma[k] /= float64(cnt)
		}
	}
	for k, cnt := range putCount {
		if cnt > 1 {
			putGamma[k] /= float64(cnt)
		}
	}

	// Collect unique strikes
	strikeSet := make(map[float64]struct{})
	for k := range callGamma {
		strikeSet[k] = struct{}{}
	}
	for k := range putGamma {
		strikeSet[k] = struct{}{}
	}
	strikes := make([]float64, 0, len(strikeSet))
	for k := range strikeSet {
		strikes = append(strikes, k)
	}
	sort.Float64s(strikes)

	if len(strikes) == 0 {
		return nil, fmt.Errorf("gex: no strikes with gamma data for %s", sym)
	}

	// 4. Validate spot price before calculation
	if spot <= 0 {
		return nil, fmt.Errorf("gex: invalid spot price (%.4f) for %s — no usable underlying or mark price from Deribit", spot, sym)
	}

	// 5. Calculate GEX profile
	levels, err := calculateGEX(strikes, callGamma, callOI, putGamma, putOI, contractSize, spot)
	if err != nil {
		return nil, fmt.Errorf("gex: calculate: %w", err)
	}

	// Filter to ±20% around spot for the display profile
	nearStrikes := filterNearSpot(levels, spot, 0.20)

	// 6. Aggregate stats
	totalGEX := 0.0
	for _, l := range levels {
		totalGEX += l.NetGEX
	}

	flipLevel := findFlipLevel(nearStrikes, spot)
	maxPain := findMaxPain(strikes, callOI, putOI)
	keys := topKeyLevels(nearStrikes, 5)
	gwall := gammaWall(nearStrikes)
	pwall := putWall(nearStrikes)

	regime, impl := regimeAndImplication(totalGEX, flipLevel)

	result := &GEXResult{
		Symbol:      sym,
		SpotPrice:   spot,
		TotalGEX:    totalGEX,
		GEXFlipLevel: flipLevel,
		Levels:      nearStrikes,
		KeyLevels:   keys,
		GammaWall:   gwall,
		PutWall:     pwall,
		MaxPain:     maxPain,
		Regime:      regime,
		Implication: impl,
		AnalyzedAt:  time.Now().UTC(),
	}

	e.storeCache(sym, result)
	return result, nil
}

// filterNearSpot filters GEXLevel entries within ±pct of spot.
func filterNearSpot(levels []GEXLevel, spot, pct float64) []GEXLevel {
	lo := spot * (1 - pct)
	hi := spot * (1 + pct)
	var out []GEXLevel
	for _, l := range levels {
		if l.Strike >= lo && l.Strike <= hi {
			out = append(out, l)
		}
	}
	// If too many, keep the 30 closest to spot
	if len(out) > 30 {
		sort.Slice(out, func(i, j int) bool {
			return math.Abs(out[i].Strike-spot) < math.Abs(out[j].Strike-spot)
		})
		out = out[:30]
		sort.Slice(out, func(i, j int) bool { return out[i].Strike < out[j].Strike })
	}
	return out
}

// fromCache returns a cached result if still valid, otherwise nil.
func (e *Engine) fromCache(sym string) *GEXResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.cache[sym]
	if !ok {
		return nil
	}
	if time.Since(c.fetchedAt) > cacheTTL {
		delete(e.cache, sym)
		return nil
	}
	return c.result
}

// storeCache saves a result to the cache.
func (e *Engine) storeCache(sym string, r *GEXResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache[sym] = &cachedResult{result: r, fetchedAt: time.Now()}
}
