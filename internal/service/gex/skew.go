// Package gex — skew.go: IV Skew / Smile analysis with flip detection and historical tracking.
// Builds on top of AnalyzeIVSurface data to provide:
//   - 5-point moneyness smile curve per expiry
//   - Put/Call IV ratio with historical percentile
//   - Skew slope via linear regression
//   - Skew flip detection (bearish→bullish or vice versa)
//   - ATM IV term structure slope analysis
package gex

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// SmilePoint is a single point on the IV smile curve at a specific moneyness.
type SmilePoint struct {
	Moneyness float64 // e.g. 0.80, 0.90, 1.00, 1.10, 1.20
	AvgIV     float64 // average IV of options near this moneyness
	Count     int     // number of options contributing
}

// SkewMetrics holds computed skew analytics for a single expiry.
type SkewMetrics struct {
	Expiry         time.Time
	DTE            int
	SmileCurve     []SmilePoint // 5-point moneyness smile
	PutCallIVRatio float64      // avg_put_IV / avg_call_IV (>1 = bearish skew)
	SkewSlope      float64      // linear regression slope of IV vs moneyness (negative = normal skew)
	SkewDirection  string       // "BEARISH" | "BULLISH" | "NEUTRAL"
}

// SkewAlert represents a detected skew flip event.
type SkewAlert struct {
	Symbol     string
	DTE        int
	Expiry     time.Time
	OldSkew    string  // previous direction
	NewSkew    string  // current direction
	DeltaRatio float64 // absolute change in PutCallIVRatio
	DetectedAt time.Time
}

// SkewResult is the complete skew analysis output for one asset.
type SkewResult struct {
	Symbol    string
	SpotPrice float64

	// Per-expiry skew metrics
	ExpirySkews []SkewMetrics

	// Aggregate put/call IV ratio across nearest liquid expiry
	AggregatePCRatio   float64
	PCRatioPercentile  float64 // 0–100, based on recent history
	PCRatioSignal      string  // "BEARISH" | "BULLISH" | "NEUTRAL"

	// ATM IV term structure slope (annualised change per DTE)
	TermSlopePerDTE    float64
	TermSlopeSignal    string // "CONTANGO" | "BACKWARDATION" | "FLAT"

	// Detected skew flips
	Alerts []SkewAlert

	AnalyzedAt time.Time
}

// ---------------------------------------------------------------------------
// Historical tracking for flip detection & percentile
// ---------------------------------------------------------------------------

const (
	skewHistorySize = 48 // ~24h at 30min intervals
	skewCacheTTL    = 30 * time.Minute
)

type skewSnapshot struct {
	pcRatio   float64
	direction string // "BEARISH" | "BULLISH" | "NEUTRAL"
	ts        time.Time
}

type skewHistoryEntry struct {
	snapshots []skewSnapshot
}

var (
	skewHistMu sync.Mutex
	skewHist   = make(map[string]*skewHistoryEntry) // key: "SYM:DTE"

	skewCacheMu sync.Mutex
	skewCache   = make(map[string]*skewCacheEntry)
)

type skewCacheEntry struct {
	result    *SkewResult
	fetchedAt time.Time
}

func skewFromCache(sym string) *SkewResult {
	skewCacheMu.Lock()
	defer skewCacheMu.Unlock()
	c, ok := skewCache[sym]
	if !ok {
		return nil
	}
	if time.Since(c.fetchedAt) > skewCacheTTL {
		delete(skewCache, sym)
		return nil
	}
	return c.result
}

func skewStoreCache(sym string, r *SkewResult) {
	skewCacheMu.Lock()
	defer skewCacheMu.Unlock()
	skewCache[sym] = &skewCacheEntry{result: r, fetchedAt: time.Now()}
}

func recordSkewSnapshot(sym string, dte int, pcRatio float64, direction string) {
	key := sym + ":" + itoa(dte)
	skewHistMu.Lock()
	defer skewHistMu.Unlock()

	h, ok := skewHist[key]
	if !ok {
		h = &skewHistoryEntry{}
		skewHist[key] = h
	}

	h.snapshots = append(h.snapshots, skewSnapshot{
		pcRatio:   pcRatio,
		direction: direction,
		ts:        time.Now(),
	})

	// Trim old snapshots
	if len(h.snapshots) > skewHistorySize {
		h.snapshots = h.snapshots[len(h.snapshots)-skewHistorySize:]
	}
}

func getSkewHistory(sym string, dte int) []skewSnapshot {
	key := sym + ":" + itoa(dte)
	skewHistMu.Lock()
	defer skewHistMu.Unlock()

	h, ok := skewHist[key]
	if !ok {
		return nil
	}
	out := make([]skewSnapshot, len(h.snapshots))
	copy(out, h.snapshots)
	return out
}

// itoa is a minimal int-to-string helper to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

// ---------------------------------------------------------------------------
// Engine method: AnalyzeSkew
// ---------------------------------------------------------------------------

// AnalyzeSkew performs IV skew/smile analysis on top of IV surface data.
// Results are cached for 30 minutes.
func (e *Engine) AnalyzeSkew(ctx context.Context, symbol string) (*SkewResult, error) {
	sym := normalizeSymbol(symbol)

	if cached := skewFromCache(sym); cached != nil {
		return cached, nil
	}

	// Get IV surface data (reuses its own cache)
	ivResult, err := e.AnalyzeIVSurface(ctx, sym)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var expirySkews []SkewMetrics

	for _, slice := range ivResult.Expiries {
		if slice.DTE < 1 || slice.PointCount < 5 {
			continue // skip very near-term or thin expiries
		}

		// Filter IV points for this expiry
		var expiryPoints []IVPoint
		for _, p := range ivResult.Points {
			if p.Expiry.Equal(slice.Expiry) {
				expiryPoints = append(expiryPoints, p)
			}
		}

		metrics := computeSkewMetrics(slice.Expiry, slice.DTE, expiryPoints)
		expirySkews = append(expirySkews, metrics)
	}

	// Aggregate: use nearest liquid expiry (DTE >= 3, enough data)
	var primary *SkewMetrics
	for i := range expirySkews {
		if expirySkews[i].DTE >= 3 && len(expirySkews[i].SmileCurve) >= 3 {
			primary = &expirySkews[i]
			break
		}
	}

	aggPCRatio := 0.0
	pcPercentile := 50.0
	pcSignal := "NEUTRAL"

	if primary != nil {
		aggPCRatio = primary.PutCallIVRatio
		pcSignal = primary.SkewDirection

		// Record snapshot for history
		recordSkewSnapshot(sym, primary.DTE, aggPCRatio, pcSignal)

		// Compute percentile from history
		history := getSkewHistory(sym, primary.DTE)
		pcPercentile = computePercentile(history, aggPCRatio)
	}

	// Term structure slope
	termSlope, termSignal := computeTermSlope(ivResult.TermStructure)

	// Detect skew flips
	var alerts []SkewAlert
	if primary != nil {
		history := getSkewHistory(sym, primary.DTE)
		if flip := detectSkewFlip(sym, primary.DTE, primary.Expiry, history); flip != nil {
			alerts = append(alerts, *flip)
		}
	}

	result := &SkewResult{
		Symbol:            sym,
		SpotPrice:         ivResult.SpotPrice,
		ExpirySkews:       expirySkews,
		AggregatePCRatio:  aggPCRatio,
		PCRatioPercentile: pcPercentile,
		PCRatioSignal:     pcSignal,
		TermSlopePerDTE:   termSlope,
		TermSlopeSignal:   termSignal,
		Alerts:            alerts,
		AnalyzedAt:        now,
	}

	skewStoreCache(sym, result)
	return result, nil
}

// normalizeSymbol trims and uppercases a symbol.
func normalizeSymbol(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		out = append(out, c)
	}
	return string(out)
}

// ---------------------------------------------------------------------------
// Smile curve computation
// ---------------------------------------------------------------------------

// moneynessTargets are the 5-point moneyness levels for the smile curve.
var moneynessTargets = []float64{0.80, 0.90, 1.00, 1.10, 1.20}

// moneynessWindow defines the ± range around each target for grouping options.
const moneynessWindow = 0.05

func computeSkewMetrics(expiry time.Time, dte int, points []IVPoint) SkewMetrics {
	// Build 5-point smile curve
	smile := make([]SmilePoint, 0, len(moneynessTargets))
	for _, target := range moneynessTargets {
		lo := target - moneynessWindow
		hi := target + moneynessWindow
		var ivs []float64
		for _, p := range points {
			if p.Moneyness >= lo && p.Moneyness < hi && p.MarkIV > 0 {
				ivs = append(ivs, p.MarkIV)
			}
		}
		avgIV := 0.0
		if len(ivs) > 0 {
			sum := 0.0
			for _, v := range ivs {
				sum += v
			}
			avgIV = sum / float64(len(ivs))
		}
		smile = append(smile, SmilePoint{
			Moneyness: target,
			AvgIV:     avgIV,
			Count:     len(ivs),
		})
	}

	// Put/Call IV ratio: avg put IV / avg call IV
	var putIVs, callIVs []float64
	for _, p := range points {
		if p.MarkIV <= 0 {
			continue
		}
		if p.OptionType == "put" {
			putIVs = append(putIVs, p.MarkIV)
		} else {
			callIVs = append(callIVs, p.MarkIV)
		}
	}

	pcRatio := 0.0
	avgPutIV := sliceAvg(putIVs)
	avgCallIV := sliceAvg(callIVs)
	if avgCallIV > 0 {
		pcRatio = avgPutIV / avgCallIV
	}

	// Skew slope: linear regression of IV vs moneyness
	// Use all points with valid IV.
	var xs, ys []float64
	for _, p := range points {
		if p.MarkIV > 0 && p.Moneyness > 0.5 && p.Moneyness < 2.0 {
			xs = append(xs, p.Moneyness)
			ys = append(ys, p.MarkIV)
		}
	}
	slope := linearSlope(xs, ys)

	// Direction from PC ratio
	direction := "NEUTRAL"
	if pcRatio > 1.05 {
		direction = "BEARISH"
	} else if pcRatio < 0.95 {
		direction = "BULLISH"
	}

	return SkewMetrics{
		Expiry:         expiry,
		DTE:            dte,
		SmileCurve:     smile,
		PutCallIVRatio: pcRatio,
		SkewSlope:      slope,
		SkewDirection:  direction,
	}
}

// sliceAvg returns the arithmetic mean of a float64 slice, or 0 if empty.
func sliceAvg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// linearSlope computes the slope of a simple linear regression y = a + b*x.
// Returns 0 if insufficient data.
func linearSlope(xs, ys []float64) float64 {
	n := len(xs)
	if n < 2 || n != len(ys) {
		return 0
	}
	nf := float64(n)
	sumX, sumY, sumXY, sumXX := 0.0, 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		sumX += xs[i]
		sumY += ys[i]
		sumXY += xs[i] * ys[i]
		sumXX += xs[i] * xs[i]
	}
	denom := nf*sumXX - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return 0
	}
	return (nf*sumXY - sumX*sumY) / denom
}

// ---------------------------------------------------------------------------
// Percentile computation
// ---------------------------------------------------------------------------

func computePercentile(history []skewSnapshot, current float64) float64 {
	if len(history) < 2 {
		return 50.0 // insufficient data
	}
	below := 0
	for _, s := range history {
		if s.pcRatio < current {
			below++
		}
	}
	return float64(below) / float64(len(history)) * 100
}

// ---------------------------------------------------------------------------
// Skew flip detection
// ---------------------------------------------------------------------------

// detectSkewFlip checks if the most recent direction differs from the previous.
// A flip requires at least 2 historical snapshots and a meaningful ratio change.
func detectSkewFlip(sym string, dte int, expiry time.Time, history []skewSnapshot) *SkewAlert {
	if len(history) < 2 {
		return nil
	}

	current := history[len(history)-1]
	// Look back for the last snapshot with a *different* direction.
	var prev *skewSnapshot
	for i := len(history) - 2; i >= 0; i-- {
		if history[i].direction != current.direction && history[i].direction != "NEUTRAL" && current.direction != "NEUTRAL" {
			prev = &history[i]
			break
		}
	}

	if prev == nil {
		return nil
	}

	// Only alert if the flip happened recently (within last 2 snapshots)
	if len(history) >= 2 {
		secondLast := history[len(history)-2]
		if secondLast.direction == current.direction {
			return nil // not a fresh flip
		}
	}

	deltaRatio := math.Abs(current.pcRatio - prev.pcRatio)
	if deltaRatio < 0.05 {
		return nil // too small a change
	}

	return &SkewAlert{
		Symbol:     sym,
		DTE:        dte,
		Expiry:     expiry,
		OldSkew:    prev.direction,
		NewSkew:    current.direction,
		DeltaRatio: deltaRatio,
		DetectedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Term structure slope
// ---------------------------------------------------------------------------

func computeTermSlope(pts []TermPoint) (slope float64, signal string) {
	if len(pts) < 2 {
		return 0, "FLAT"
	}

	// Sort by DTE ascending
	sorted := make([]TermPoint, len(pts))
	copy(sorted, pts)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].DTE < sorted[j].DTE })

	// Linear regression of ATMIV vs DTE
	var xs, ys []float64
	for _, p := range sorted {
		if p.ATMIV > 0 {
			xs = append(xs, float64(p.DTE))
			ys = append(ys, p.ATMIV)
		}
	}

	slope = linearSlope(xs, ys)

	if slope > 0.1 {
		signal = "CONTANGO"
	} else if slope < -0.1 {
		signal = "BACKWARDATION"
	} else {
		signal = "FLAT"
	}
	return slope, signal
}
