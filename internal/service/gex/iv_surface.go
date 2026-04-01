// Package gex — iv_surface.go: Implied Volatility surface, skew, and term structure analysis
// using Deribit public book-summary data (mark_iv field).
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
)

const ivCacheTTL = 30 * time.Minute

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// IVPoint is a single option data point in the IV surface.
type IVPoint struct {
	InstrumentName string
	Strike         float64
	Expiry         time.Time
	OptionType     string  // "call" | "put"
	MarkIV         float64 // annualised IV in %
	OpenInterest   float64
	Moneyness      float64 // Strike / Spot
}

// ExpirySlice holds IV analytics for a single expiry date.
type ExpirySlice struct {
	Expiry       time.Time
	DTE          int     // days to expiry
	ATMIV        float64 // ATM implied vol (average of calls+puts near moneyness=1)
	PutWingIV    float64 // average IV of OTM puts (moneyness 0.85–0.95)
	CallWingIV   float64 // average IV of OTM calls (moneyness 1.05–1.15)
	Skew25Delta  float64 // PutWingIV - CallWingIV (positive = put skew / fear)
	SmileLabel   string  // "PUT_SKEW" | "CALL_SKEW" | "FLAT"
	PointCount   int
}

// IVSurfaceResult is the complete IV analytics output for one asset.
type IVSurfaceResult struct {
	Symbol    string
	SpotPrice float64

	// Per-expiry slices, sorted ascending by expiry date.
	Expiries []ExpirySlice

	// Term structure: list of (DTE, ATMIV) pairs for charting.
	TermStructure []TermPoint

	// Backwardation: true when near-term IV > far-term IV.
	Backwardation bool

	// Overall market signal derived from skew and term structure.
	MarketSignal string // "FEAR" | "GREED" | "NEUTRAL"
	SignalReason  string

	// Raw IV points (filtered to those with valid MarkIV > 0).
	Points []IVPoint

	AnalyzedAt time.Time
}

// TermPoint is a single point on the IV term structure curve.
type TermPoint struct {
	DTE   int
	ATMIV float64
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

type cachedIVResult struct {
	result    *IVSurfaceResult
	fetchedAt time.Time
}

var (
	ivMu    sync.Mutex
	ivCache = make(map[string]*cachedIVResult)
)

func ivFromCache(sym string) *IVSurfaceResult {
	ivMu.Lock()
	defer ivMu.Unlock()
	c, ok := ivCache[sym]
	if !ok {
		return nil
	}
	if time.Since(c.fetchedAt) > ivCacheTTL {
		delete(ivCache, sym)
		return nil
	}
	return c.result
}

func ivStoreCache(sym string, r *IVSurfaceResult) {
	ivMu.Lock()
	defer ivMu.Unlock()
	ivCache[sym] = &cachedIVResult{result: r, fetchedAt: time.Now()}
}

// ---------------------------------------------------------------------------
// Engine method: AnalyzeIVSurface
// ---------------------------------------------------------------------------

// AnalyzeIVSurface builds the IV surface, skew metrics, and term structure
// for the given symbol (e.g. "BTC", "ETH"). Results are cached for 30 min.
func (e *Engine) AnalyzeIVSurface(ctx context.Context, symbol string) (*IVSurfaceResult, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))

	if cached := ivFromCache(sym); cached != nil {
		return cached, nil
	}

	cfg, ok := supportedAssets[sym]
	if !ok {
		return nil, fmt.Errorf("gex/iv: unsupported symbol %s", sym)
	}

	log.Info().Str("symbol", sym).Msg("fetching IV surface data from Deribit")

	// 1. Get spot price via index.
	spot, err := e.client.GetIndexPriceByName(ctx, cfg.IndexName)
	if err != nil || spot <= 0 {
		// Fallback: try book summary underlying price.
		spot = 0
	}

	// 2. Fetch bulk book summary (all options, single call).
	allSummaries, err := e.client.GetBookSummary(ctx, cfg.APICurrency)
	if err != nil {
		return nil, fmt.Errorf("gex/iv: get book summary: %w", err)
	}

	// Filter by prefix for USDC-settled altcoins.
	summaries := allSummaries
	if cfg.InstrumentPrefix != "" {
		summaries = summaries[:0]
		for _, s := range allSummaries {
			if strings.HasPrefix(s.InstrumentName, cfg.InstrumentPrefix) {
				summaries = append(summaries, s)
			}
		}
	}

	// If spot still zero, derive from summaries.
	if spot <= 0 {
		for _, s := range summaries {
			if s.UnderlyingPrice > 0 {
				spot = s.UnderlyingPrice
				break
			}
		}
	}
	if spot <= 0 {
		return nil, fmt.Errorf("gex/iv: cannot determine spot price for %s", sym)
	}

	// 3. Build IV points from book summary.
	now := time.Now()
	var points []IVPoint
	for _, s := range summaries {
		if s.MarkIV <= 0 {
			continue // skip entries without IV data
		}
		// Parse instrument name: e.g. "BTC-28MAR25-80000-C"
		parts := strings.SplitN(s.InstrumentName, "-", 4)
		if len(parts) < 4 {
			continue
		}
		expiry, err := deribit.ParseExpiryFromInstrument(s.InstrumentName)
		if err != nil {
			continue
		}
		if expiry.Before(now) {
			continue
		}
		strike := 0.0
		fmt.Sscanf(parts[2], "%f", &strike)
		if strike <= 0 {
			continue
		}
		optType := strings.ToLower(parts[3])
		if optType != "c" && optType != "p" {
			continue
		}
		fullType := "call"
		if optType == "p" {
			fullType = "put"
		}
		points = append(points, IVPoint{
			InstrumentName: s.InstrumentName,
			Strike:         strike,
			Expiry:         expiry,
			OptionType:     fullType,
			MarkIV:         s.MarkIV,
			OpenInterest:   s.OpenInterest,
			Moneyness:      strike / spot,
		})
	}

	if len(points) == 0 {
		return nil, fmt.Errorf("gex/iv: no valid IV data for %s (all mark_iv=0)", sym)
	}

	// 4. Group by expiry and compute per-slice metrics.
	expiryMap := make(map[time.Time][]IVPoint)
	for _, p := range points {
		expiryMap[p.Expiry] = append(expiryMap[p.Expiry], p)
	}

	var slices []ExpirySlice
	for expiry, pts := range expiryMap {
		dte := int(math.Round(expiry.Sub(now).Hours() / 24))
		if dte < 0 {
			dte = 0
		}
		slice := computeExpirySlice(expiry, dte, pts)
		slices = append(slices, slice)
	}

	// Sort by expiry ascending.
	sort.Slice(slices, func(i, j int) bool {
		return slices[i].Expiry.Before(slices[j].Expiry)
	})

	// 5. Build term structure (only slices with valid ATMIV).
	var termStructure []TermPoint
	for _, sl := range slices {
		if sl.ATMIV > 0 {
			termStructure = append(termStructure, TermPoint{DTE: sl.DTE, ATMIV: sl.ATMIV})
		}
	}

	// 6. Detect term structure backwardation (front IV > back IV).
	backwardation := false
	if len(termStructure) >= 2 {
		backwardation = termStructure[0].ATMIV > termStructure[len(termStructure)-1].ATMIV
	}

	// 7. Derive overall market signal from skew of nearest liquid expiry.
	signal, reason := deriveMarketSignal(slices, backwardation)

	result := &IVSurfaceResult{
		Symbol:        sym,
		SpotPrice:     spot,
		Expiries:      slices,
		TermStructure: termStructure,
		Backwardation: backwardation,
		MarketSignal:  signal,
		SignalReason:  reason,
		Points:        points,
		AnalyzedAt:    now,
	}

	ivStoreCache(sym, result)
	return result, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// computeExpirySlice computes IV metrics for a single expiry.
func computeExpirySlice(expiry time.Time, dte int, pts []IVPoint) ExpirySlice {
	// ATM: moneyness 0.97–1.03
	var atmIVs []float64
	// Put wing: moneyness 0.85–0.95 (OTM puts)
	var putIVs []float64
	// Call wing: moneyness 1.05–1.15 (OTM calls)
	var callIVs []float64

	for _, p := range pts {
		m := p.Moneyness
		iv := p.MarkIV
		switch {
		case m >= 0.97 && m <= 1.03:
			atmIVs = append(atmIVs, iv)
		case p.OptionType == "put" && m >= 0.85 && m <= 0.95:
			putIVs = append(putIVs, iv)
		case p.OptionType == "call" && m >= 1.05 && m <= 1.15:
			callIVs = append(callIVs, iv)
		}
	}

	atmIV := avg(atmIVs)
	putWingIV := avg(putIVs)
	callWingIV := avg(callIVs)
	skew := 0.0
	if putWingIV > 0 && callWingIV > 0 {
		skew = putWingIV - callWingIV
	}

	smileLabel := "FLAT"
	if skew > 2.0 {
		smileLabel = "PUT_SKEW"
	} else if skew < -2.0 {
		smileLabel = "CALL_SKEW"
	}

	return ExpirySlice{
		Expiry:      expiry,
		DTE:         dte,
		ATMIV:       atmIV,
		PutWingIV:   putWingIV,
		CallWingIV:  callWingIV,
		Skew25Delta: skew,
		SmileLabel:  smileLabel,
		PointCount:  len(pts),
	}
}

// deriveMarketSignal produces a high-level signal from the IV surface.
func deriveMarketSignal(slices []ExpirySlice, backwardation bool) (signal, reason string) {
	// Find first slice with meaningful data (DTE > 3, ATMIV > 0, has skew).
	var primary *ExpirySlice
	for i := range slices {
		sl := &slices[i]
		if sl.DTE >= 3 && sl.ATMIV > 0 && (sl.PutWingIV > 0 || sl.CallWingIV > 0) {
			primary = sl
			break
		}
	}

	if primary == nil {
		return "NEUTRAL", "Insufficient data for signal"
	}

	reasons := []string{}
	fearScore := 0

	if primary.SmileLabel == "PUT_SKEW" {
		fearScore += 2
		reasons = append(reasons, fmt.Sprintf("put skew +%.1f%%", primary.Skew25Delta))
	} else if primary.SmileLabel == "CALL_SKEW" {
		fearScore -= 2
		reasons = append(reasons, fmt.Sprintf("call skew %.1f%%", primary.Skew25Delta))
	}

	if backwardation {
		fearScore += 2
		reasons = append(reasons, "term structure backwardation")
	}

	if primary.ATMIV > 80 {
		fearScore++
		reasons = append(reasons, fmt.Sprintf("high ATM IV %.0f%%", primary.ATMIV))
	} else if primary.ATMIV < 30 {
		fearScore--
		reasons = append(reasons, fmt.Sprintf("low ATM IV %.0f%%", primary.ATMIV))
	}

	switch {
	case fearScore >= 3:
		signal = "FEAR"
	case fearScore <= -2:
		signal = "GREED"
	default:
		signal = "NEUTRAL"
	}

	reason = strings.Join(reasons, "; ")
	if reason == "" {
		reason = "balanced IV structure"
	}
	return signal, reason
}

// avg returns the arithmetic mean of a float64 slice, or 0 if empty.
func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
