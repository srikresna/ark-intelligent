package factors

// flow_divergence.go — Cross-Asset Flow Divergence Detection (TASK-162)
//
// Detects when normally-correlated asset pairs decouple using:
//   1. Rolling 20-bar Pearson correlation (current state)
//   2. Baseline correlation (60-bar rolling mean + stddev for z-score)
//   3. Divergence score: (current_corr - baseline_mean) / baseline_std
//   4. Lead-lag analysis: cross-correlation at offsets ±1..±5 bars

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

const (
	flowShortWindow    = 20
	flowLongWindow     = 60
	flowMinPoints      = 15
	flowMaxLag         = 5
	flowCacheTTL       = 4 * time.Hour
	flowFetchMultiplier = 2
)

var flowLog = logger.Component("flow_divergence")

// ---------------------------------------------------------------------------
// Pair definitions
// ---------------------------------------------------------------------------

// FlowPair defines one cross-asset relationship to monitor.
type FlowPair struct {
	CurrencyA   string
	CurrencyB   string
	Direction   int    // +1 = normally positive corr, -1 = normally inverse
	Label       string
	Implication string
}

var standardFlowPairs = []FlowPair{
	{CurrencyA: "USD", CurrencyB: "EUR", Direction: -1,
		Label:       "DXY↔EUR",
		Implication: "DXY and EUR inverse relationship breaking — EUR-specific driver or USD dislocation"},
	{CurrencyA: "XAU", CurrencyB: "USD", Direction: -1,
		Label:       "Gold↔DXY",
		Implication: "Gold not tracking USD — unique safe-haven or inflation flow divergence"},
	{CurrencyA: "BTC", CurrencyB: "SPX500", Direction: +1,
		Label:       "BTC↔SPX",
		Implication: "Crypto decoupling from risk-on equities — crypto-specific catalyst or regime shift"},
	{CurrencyA: "OIL", CurrencyB: "CAD", Direction: +1,
		Label:       "Oil↔CAD",
		Implication: "CAD not following Oil — BoC policy divergence or position squeeze"},
	{CurrencyA: "BOND", CurrencyB: "USD", Direction: +1,
		Label:       "Yields↔DXY",
		Implication: "Rate differentials not driving USD — capital flow or risk appetite overriding carry"},
	{CurrencyA: "XAU", CurrencyB: "SPX500", Direction: -1,
		Label:       "Gold↔Equities",
		Implication: "Safe-haven bid breakdown — dollar liquidity or stagflation signal"},
	{CurrencyA: "AUD", CurrencyB: "XAU", Direction: +1,
		Label:       "AUD↔Gold",
		Implication: "AUD not tracking Gold — Australian carry or China demand divergence"},
	{CurrencyA: "OIL", CurrencyB: "SPX500", Direction: +1,
		Label:       "Oil↔Equities",
		Implication: "Oil and equities decoupling — geopolitical premium or growth fear"},
	{CurrencyA: "JPY", CurrencyB: "SPX500", Direction: -1,
		Label:       "JPY↔Equities",
		Implication: "Safe-haven JPY not responding to equities — BoJ intervention risk or carry breakdown"},
	{CurrencyA: "JPY", CurrencyB: "BOND", Direction: -1,
		Label:       "JPY↔Yields",
		Implication: "JPY carry trade stress: yield differential not driving USDJPY as expected"},
}

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

// LeadLagResult describes which asset leads the other.
type LeadLagResult struct {
	BestOffset int     // negative = A leads B, positive = B leads A, 0 = simultaneous
	BestCorr   float64
	LeadAsset  string
}

// PairDivergence holds the flow divergence result for one pair.
type PairDivergence struct {
	Pair         FlowPair
	CurrentCorr  float64
	BaselineMean float64
	BaselineStd  float64
	DivergenceZ  float64
	LeadLag      LeadLagResult
	IsDiverging  bool   // |DivergenceZ| > 2.0
	IsStrong     bool   // |DivergenceZ| > 3.0
	Insufficient bool
	AlertText    string
}

// FlowDivergenceResult is the full output of the engine.
type FlowDivergenceResult struct {
	Pairs           []PairDivergence
	TopDivergences  []PairDivergence // sorted by |DivergenceZ| desc
	RegimeStability float64          // fraction of valid pairs NOT diverging
	ComputedAt      time.Time
}

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

// FlowDivergenceStore is the price data interface required by the engine.
type FlowDivergenceStore interface {
	GetDailyHistory(ctx context.Context, contractCode string, days int) ([]domain.DailyPrice, error)
}

// FlowDivergenceEngine computes cross-asset flow divergences.
type FlowDivergenceEngine struct {
	priceRepo FlowDivergenceStore
}

// NewFlowDivergenceEngine creates a new engine.
func NewFlowDivergenceEngine(priceRepo FlowDivergenceStore) *FlowDivergenceEngine {
	return &FlowDivergenceEngine{priceRepo: priceRepo}
}

// ---------------------------------------------------------------------------
// In-process cache
// ---------------------------------------------------------------------------

type flowCacheEntry struct {
	result    *FlowDivergenceResult
	fetchedAt time.Time
}

var (
	flowGlobalCache *flowCacheEntry
	flowCacheMu     sync.RWMutex
)

// GetCachedOrAnalyze returns cached result within TTL, else re-computes.
func (e *FlowDivergenceEngine) GetCachedOrAnalyze(ctx context.Context) (*FlowDivergenceResult, error) {
	flowCacheMu.RLock()
	if flowGlobalCache != nil && time.Since(flowGlobalCache.fetchedAt) < flowCacheTTL {
		r := flowGlobalCache.result
		flowCacheMu.RUnlock()
		return r, nil
	}
	flowCacheMu.RUnlock()

	result, err := e.Analyze(ctx)
	if err != nil {
		return nil, err
	}

	flowCacheMu.Lock()
	flowGlobalCache = &flowCacheEntry{result: result, fetchedAt: time.Now()}
	flowCacheMu.Unlock()

	return result, nil
}

// InvalidateFlowCache clears the cached result.
func InvalidateFlowCache() {
	flowCacheMu.Lock()
	flowGlobalCache = nil
	flowCacheMu.Unlock()
}

// ---------------------------------------------------------------------------
// Analyze
// ---------------------------------------------------------------------------

// Analyze fetches price series and computes flow divergences for all pairs.
func (e *FlowDivergenceEngine) Analyze(ctx context.Context) (*FlowDivergenceResult, error) {
	needed := make(map[string]bool)
	for _, p := range standardFlowPairs {
		needed[p.CurrencyA] = true
		needed[p.CurrencyB] = true
	}

	fetchDays := (flowLongWindow + flowMaxLag + 10) * flowFetchMultiplier
	seriesMap := make(map[string][]float64)
	for cur := range needed {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			flowLog.Warn().Str("currency", cur).Msg("flow: no price mapping")
			continue
		}
		records, err := e.priceRepo.GetDailyHistory(ctx, mapping.ContractCode, fetchDays)
		if err != nil {
			flowLog.Warn().Str("currency", cur).Err(err).Msg("flow: fetch error")
			continue
		}
		returns := flowComputeReturns(records, flowLongWindow+flowMaxLag+5, mapping.Inverse)
		if len(returns) >= flowMinPoints {
			seriesMap[cur] = returns
		}
	}

	var pairs []PairDivergence
	for _, fp := range standardFlowPairs {
		pairs = append(pairs, evaluateFlowPair(fp, seriesMap))
	}

	return &FlowDivergenceResult{
		Pairs:           pairs,
		TopDivergences:  flowTopDivergences(pairs),
		RegimeStability: flowRegimeStability(pairs),
		ComputedAt:      time.Now(),
	}, nil
}

// ---------------------------------------------------------------------------
// Per-pair evaluation
// ---------------------------------------------------------------------------

func evaluateFlowPair(fp FlowPair, seriesMap map[string][]float64) PairDivergence {
	serA, okA := seriesMap[fp.CurrencyA]
	serB, okB := seriesMap[fp.CurrencyB]
	if !okA || !okB {
		return PairDivergence{Pair: fp, Insufficient: true}
	}

	n := len(serA)
	if len(serB) < n {
		n = len(serB)
	}
	if n < flowMinPoints+flowShortWindow {
		return PairDivergence{Pair: fp, Insufficient: true}
	}
	serA = serA[len(serA)-n:]
	serB = serB[len(serB)-n:]

	currentCorr := pearsonCorrSlice(serA[n-flowShortWindow:], serB[n-flowShortWindow:])
	if math.IsNaN(currentCorr) {
		return PairDivergence{Pair: fp, Insufficient: true}
	}

	baseWindow := flowLongWindow
	if n < baseWindow+flowShortWindow {
		baseWindow = n - flowShortWindow
	}
	rollingCorrs := computeRollingCorrelations(serA, serB, flowShortWindow, baseWindow)
	if len(rollingCorrs) < 5 {
		return PairDivergence{Pair: fp, Insufficient: true}
	}
	bMean := flowMean(rollingCorrs)
	bStd := flowStddev(rollingCorrs, bMean)

	var divZ float64
	if bStd > 1e-9 {
		divZ = (currentCorr - bMean) / bStd
	}

	ll := computeLeadLag(serA, serB, fp.CurrencyA, fp.CurrencyB, flowMaxLag)

	isDiverging := math.Abs(divZ) > 2.0
	isStrong := math.Abs(divZ) > 3.0
	alertText := buildFlowAlertText(fp, currentCorr, divZ, ll, isDiverging)

	return PairDivergence{
		Pair:         fp,
		CurrentCorr:  currentCorr,
		BaselineMean: bMean,
		BaselineStd:  bStd,
		DivergenceZ:  divZ,
		LeadLag:      ll,
		IsDiverging:  isDiverging,
		IsStrong:     isStrong,
		AlertText:    alertText,
	}
}

// ---------------------------------------------------------------------------
// Lead-lag analysis
// ---------------------------------------------------------------------------

func computeLeadLag(serA, serB []float64, labelA, labelB string, maxLag int) LeadLagResult {
	n := len(serA)
	if n < flowShortWindow+maxLag*2 {
		return LeadLagResult{}
	}

	bestCorr := math.NaN()
	bestOffset := 0

	for offset := -maxLag; offset <= maxLag; offset++ {
		var a, b []float64
		switch {
		case offset < 0:
			lag := -offset
			if n-1-lag < flowShortWindow {
				continue
			}
			a = serA[lag : n]
			b = serB[0 : n-lag]
		case offset > 0:
			if n-1-offset < flowShortWindow {
				continue
			}
			a = serA[0 : n-offset]
			b = serB[offset : n]
		default:
			a = serA[n-flowShortWindow:]
			b = serB[n-flowShortWindow:]
		}
		if len(a) > flowShortWindow {
			a = a[len(a)-flowShortWindow:]
			b = b[len(b)-flowShortWindow:]
		}
		if len(a) < flowMinPoints {
			continue
		}
		c := pearsonCorrSlice(a, b)
		if math.IsNaN(c) {
			continue
		}
		if math.IsNaN(bestCorr) || math.Abs(c) > math.Abs(bestCorr) {
			bestCorr = c
			bestOffset = offset
		}
	}

	if math.IsNaN(bestCorr) {
		return LeadLagResult{}
	}

	var leadAsset string
	switch {
	case bestOffset < 0:
		leadAsset = fmt.Sprintf("%s leads by %d bar(s)", labelA, -bestOffset)
	case bestOffset > 0:
		leadAsset = fmt.Sprintf("%s leads by %d bar(s)", labelB, bestOffset)
	default:
		leadAsset = "simultaneous"
	}

	return LeadLagResult{BestOffset: bestOffset, BestCorr: bestCorr, LeadAsset: leadAsset}
}

// ---------------------------------------------------------------------------
// Math helpers
// ---------------------------------------------------------------------------

func pearsonCorrSlice(x, y []float64) float64 {
	n := len(x)
	if n != len(y) || n < 3 {
		return math.NaN()
	}
	var sx, sy float64
	for i := 0; i < n; i++ {
		sx += x[i]
		sy += y[i]
	}
	mx := sx / float64(n)
	my := sy / float64(n)
	var num, dx2, dy2 float64
	for i := 0; i < n; i++ {
		dx := x[i] - mx
		dy := y[i] - my
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	denom := math.Sqrt(dx2 * dy2)
	if denom < 1e-12 {
		return math.NaN()
	}
	return num / denom
}

func computeRollingCorrelations(serA, serB []float64, window, numWindows int) []float64 {
	n := len(serA)
	if n < window+numWindows {
		numWindows = n - window
	}
	if numWindows <= 0 {
		return nil
	}
	result := make([]float64, 0, numWindows)
	for i := 0; i < numWindows; i++ {
		start := n - numWindows - window + i
		end := start + window
		if start < 0 || end > n {
			continue
		}
		c := pearsonCorrSlice(serA[start:end], serB[start:end])
		if !math.IsNaN(c) {
			result = append(result, c)
		}
	}
	return result
}

func flowMean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}

func flowStddev(xs []float64, m float64) float64 {
	if len(xs) < 2 {
		return 1.0
	}
	var s float64
	for _, v := range xs {
		d := v - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func flowComputeReturns(records []domain.DailyPrice, window int, inverse bool) []float64 {
	n := len(records)
	if n < 2 {
		return nil
	}
	end := window
	if end >= n {
		end = n - 1
	}
	var returns []float64
	for i := end; i >= 1; i-- {
		prev := records[i].Close
		curr := records[i-1].Close
		if prev <= 0 || curr <= 0 {
			continue
		}
		var ret float64
		if inverse {
			ret = math.Log(prev / curr)
		} else {
			ret = math.Log(curr / prev)
		}
		returns = append(returns, ret)
	}
	return returns
}

func flowTopDivergences(pairs []PairDivergence) []PairDivergence {
	var divs []PairDivergence
	for _, p := range pairs {
		if !p.Insufficient && p.IsDiverging {
			divs = append(divs, p)
		}
	}
	for i := 1; i < len(divs); i++ {
		for j := i; j > 0 && math.Abs(divs[j].DivergenceZ) > math.Abs(divs[j-1].DivergenceZ); j-- {
			divs[j], divs[j-1] = divs[j-1], divs[j]
		}
	}
	return divs
}

func flowRegimeStability(pairs []PairDivergence) float64 {
	valid, stable := 0, 0
	for _, p := range pairs {
		if p.Insufficient {
			continue
		}
		valid++
		if !p.IsDiverging {
			stable++
		}
	}
	if valid == 0 {
		return 1.0
	}
	return float64(stable) / float64(valid)
}

func buildFlowAlertText(fp FlowPair, corr, zScore float64, ll LeadLagResult, isDiverging bool) string {
	dirExp := "positive"
	if fp.Direction < 0 {
		dirExp = "inverse"
	}
	urgency := ""
	if math.Abs(zScore) > 3.0 {
		urgency = "🚨 STRONG "
	} else if isDiverging {
		urgency = "⚠️ "
	}
	base := fmt.Sprintf("%s%s: r=%.2f (z=%.1f, expected %s)",
		urgency, fp.Label, corr, zScore, dirExp)
	if ll.LeadAsset != "" && ll.LeadAsset != "simultaneous" {
		base += fmt.Sprintf(" | %s", ll.LeadAsset)
	}
	if isDiverging {
		base += fmt.Sprintf("\n   → %s", fp.Implication)
	}
	return base
}
