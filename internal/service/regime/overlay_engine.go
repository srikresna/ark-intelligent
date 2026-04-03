package regime

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	priceSvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// OverlayEngine — unified market regime orchestrator
// ---------------------------------------------------------------------------
//
// Combines HMM (30%), GARCH (25%), ADX (25%), COT (20%) into a single
// UnifiedScore (-100..+100) with graceful degradation when sub-models fail.

// OverlayEngine computes and caches RegimeOverlay results.
type OverlayEngine struct {
	dailyRepo  DailyStore
	cotRepo    ports.COTRepository
	mu         sync.Mutex
	cache      map[string]*cacheEntry // key: "symbol:timeframe"
}

// DailyStore is the minimal interface needed by the overlay engine.
type DailyStore interface {
	GetDailyHistory(ctx context.Context, contractCode string, days int) ([]domain.DailyPrice, error)
}

type cacheEntry struct {
	result    *RegimeOverlay
	expiresAt time.Time
}

// cacheTTL returns the cache TTL based on timeframe.
// Intraday (1h/4h): 1h. Daily+: 4h.
func cacheTTL(timeframe string) time.Duration {
	switch strings.ToLower(timeframe) {
	case "1h", "4h", "15m", "30m":
		return time.Hour
	default:
		return 4 * time.Hour
	}
}

// NewOverlayEngine creates a new OverlayEngine.
// cotRepo may be nil — COT sub-model will be skipped gracefully.
func NewOverlayEngine(dailyRepo DailyStore, cotRepo ports.COTRepository) *OverlayEngine {
	return &OverlayEngine{
		dailyRepo: dailyRepo,
		cotRepo:   cotRepo,
		cache:     make(map[string]*cacheEntry),
	}
}

// ---------------------------------------------------------------------------
// ComputeOverlay — main entry point
// ---------------------------------------------------------------------------

// ComputeOverlay computes the unified regime overlay for a symbol+timeframe.
// contractCode is the CFTC code used to fetch price and COT data.
// symbol is the display name (e.g. "EUR/USD").
func (e *OverlayEngine) ComputeOverlay(ctx context.Context, contractCode, symbol, timeframe string) (*RegimeOverlay, error) {
	cacheKey := symbol + ":" + timeframe
	if r := e.getCached(cacheKey); r != nil {
		return r, nil
	}

	result, err := e.compute(ctx, contractCode, symbol, timeframe)
	if err != nil {
		return nil, err
	}

	e.setCached(cacheKey, result, cacheTTL(timeframe))
	return result, nil
}

func (e *OverlayEngine) compute(ctx context.Context, contractCode, symbol, timeframe string) (*RegimeOverlay, error) {
	overlay := &RegimeOverlay{
		Symbol:     symbol,
		Timeframe:  timeframe,
		ComputedAt: time.Now(),
	}

	// Fetch daily price history (shared across sub-models)
	dailyPrices, err := e.dailyRepo.GetDailyHistory(ctx, contractCode, 250)
	if err != nil || len(dailyPrices) < 30 {
		return nil, fmt.Errorf("regime overlay: insufficient daily price data for %s: %w", symbol, err)
	}

	// Convert DailyPrice → domain.PriceRecord (for HMM/GARCH)
	weeklyRecords := dailyToPriceRecords(dailyPrices)

	// Convert DailyPrice → ta.OHLCV (for ADX)
	ohlcvBars := dailyToOHLCV(dailyPrices)

	// -------------------------------------------------------------------
	// Sub-model 1: HMM Regime (weight 30%)
	// -------------------------------------------------------------------
	hmmScore, hmmOK := e.runHMM(weeklyRecords, overlay)

	// -------------------------------------------------------------------
	// Sub-model 2: GARCH Volatility (weight 25%)
	// -------------------------------------------------------------------
	garchScore, garchOK := e.runGARCH(weeklyRecords, overlay)

	// -------------------------------------------------------------------
	// Sub-model 3: ADX Trend Strength (weight 25%)
	// -------------------------------------------------------------------
	adxScore, adxOK := e.runADX(ohlcvBars, overlay)

	// -------------------------------------------------------------------
	// Sub-model 4: COT Sentiment (weight 20%)
	// -------------------------------------------------------------------
	cotScore, cotOK := e.runCOT(ctx, contractCode, overlay)

	// -------------------------------------------------------------------
	// Weighted aggregation with graceful degradation
	// -------------------------------------------------------------------
	baseWeights := map[string]float64{
		"hmm":   0.30,
		"garch": 0.25,
		"adx":   0.25,
		"cot":   0.20,
	}
	available := map[string]bool{
		"hmm":   hmmOK,
		"garch": garchOK,
		"adx":   adxOK,
		"cot":   cotOK,
	}
	scores := map[string]float64{
		"hmm":   hmmScore,
		"garch": garchScore,
		"adx":   adxScore,
		"cot":   cotScore,
	}

	totalWeight := 0.0
	for k, avail := range available {
		if avail {
			totalWeight += baseWeights[k]
		}
	}
	if totalWeight == 0 {
		return nil, fmt.Errorf("regime overlay: all sub-models failed for %s", symbol)
	}

	unifiedScore := 0.0
	var modelsUsed []string
	for k, avail := range available {
		if avail {
			// Normalize weight proportionally
			adjustedWeight := baseWeights[k] / totalWeight
			unifiedScore += scores[k] * adjustedWeight
			modelsUsed = append(modelsUsed, k)

			// Store effective weights
			switch k {
			case "hmm":
				overlay.WeightHMM = adjustedWeight
			case "garch":
				overlay.WeightGARCH = adjustedWeight
			case "adx":
				overlay.WeightADX = adjustedWeight
			case "cot":
				overlay.WeightCOT = adjustedWeight
			}
		}
	}

	overlay.UnifiedScore = clamp(unifiedScore, -100, 100)
	overlay.ModelsUsed = modelsUsed
	overlay.OverlayColor = scoreToColor(overlay.UnifiedScore)
	overlay.Label = scoreToLabel(overlay.UnifiedScore, overlay.HMMState)
	overlay.Description = buildDescription(overlay)

	return overlay, nil
}

// ---------------------------------------------------------------------------
// Sub-model runners
// ---------------------------------------------------------------------------

func (e *OverlayEngine) runHMM(records []domain.PriceRecord, overlay *RegimeOverlay) (float64, bool) {
	if len(records) < 60 {
		return 0, false
	}
	result, err := priceSvc.EstimateHMMRegime(records)
	if err != nil || result == nil {
		return 0, false
	}

	overlay.HMMState = result.CurrentState
	overlay.HMMConfidence = maxFloat64(result.StateProbabilities[0], result.StateProbabilities[1], result.StateProbabilities[2], result.StateProbabilities[3])

	// Map HMM state to score contribution
	var score float64
	switch result.CurrentState {
	case priceSvc.HMMRiskOn:
		// Risk-on: score proportional to confidence, max +100
		score = overlay.HMMConfidence * 100
	case priceSvc.HMMRiskOff:
		// Risk-off: slightly negative, uncertainty
		score = -(overlay.HMMConfidence * 40)
	case priceSvc.HMMCrisis:
		// Crisis: strongly negative
		score = -(overlay.HMMConfidence * 100)
	case priceSvc.HMMTrending:
		// Trending: strong directional move, low vol — moderately bullish
		score = overlay.HMMConfidence * 70
	}
	overlay.HMMScore = clamp(score, -100, 100)
	return overlay.HMMScore, true
}

func (e *OverlayEngine) runGARCH(records []domain.PriceRecord, overlay *RegimeOverlay) (float64, bool) {
	if len(records) < 30 {
		return 0, false
	}
	result, err := priceSvc.EstimateGARCH(records)
	if err != nil || result == nil || !result.Converged {
		return 0, false
	}

	overlay.VolRatio = result.VolRatio

	// Classify vol regime
	switch {
	case result.VolRatio > 1.5:
		overlay.GARCHVolRegime = "EXPANDING"
	case result.VolRatio < 0.75:
		overlay.GARCHVolRegime = "CONTRACTING"
	default:
		overlay.GARCHVolRegime = "NORMAL"
	}

	// GARCH score: low/contracting vol is friendly (positive), high vol is negative
	// VolRatio 0.5 → +80, 1.0 → 0, 1.5 → -40, 2.0+ → -80
	var score float64
	switch overlay.GARCHVolRegime {
	case "CONTRACTING":
		// Vol below normal = benign environment
		score = 60 * (1 - result.VolRatio/0.75)
		if score < 0 {
			score = 0
		}
		score = clamp(score+20, 0, 80)
	case "NORMAL":
		// Near long-run vol = neutral
		score = 0
	case "EXPANDING":
		// Elevated vol = risk-off signal
		excess := result.VolRatio - 1.0
		score = -clamp(excess*50, 0, 80)
	}
	overlay.GARCHScore = clamp(score, -100, 100)
	return overlay.GARCHScore, true
}

func (e *OverlayEngine) runADX(bars []ta.OHLCV, overlay *RegimeOverlay) (float64, bool) {
	if len(bars) < 30 {
		return 0, false
	}
	result := ta.CalcADX(bars, 14)
	if result == nil {
		return 0, false
	}

	overlay.ADXValue = result.ADX
	overlay.ADXStrength = result.TrendStrength

	// ADX score: strong trend is regime-positive (momentum), weak is neutral/negative
	// Also factor in direction via +DI vs -DI
	directionMult := 1.0
	if result.MinusDI > result.PlusDI {
		directionMult = -1.0 // bearish trend
	}

	var score float64
	switch result.TrendStrength {
	case "STRONG":
		score = directionMult * clamp((result.ADX-25)/25*80+40, 40, 90)
	case "MODERATE":
		score = directionMult * 30
	case "WEAK":
		// Ranging market — slightly negative for directional bias
		score = -10
	}
	overlay.ADXScore = clamp(score, -100, 100)
	return overlay.ADXScore, true
}

func (e *OverlayEngine) runCOT(ctx context.Context, contractCode string, overlay *RegimeOverlay) (float64, bool) {
	if e.cotRepo == nil || contractCode == "" {
		return 0, false
	}

	analysis, err := e.cotRepo.GetLatestAnalysis(ctx, contractCode)
	if err != nil || analysis == nil {
		return 0, false
	}

	overlay.COTSentiment = analysis.SentimentScore

	// Map COT sentiment to bias label
	switch {
	case analysis.SentimentScore > 20:
		overlay.COTBias = "BULLISH"
	case analysis.SentimentScore < -20:
		overlay.COTBias = "BEARISH"
	default:
		overlay.COTBias = "NEUTRAL"
	}

	// Clamp COT score to -100..+100 (SentimentScore already in that range but verify)
	overlay.COTScore = clamp(analysis.SentimentScore, -100, 100)
	return overlay.COTScore, true
}

// ---------------------------------------------------------------------------
// Label / color / description helpers
// ---------------------------------------------------------------------------

func scoreToColor(score float64) string {
	switch {
	case score >= 30:
		return "🟢"
	case score <= -30:
		return "🔴"
	default:
		return "🟡"
	}
}

func scoreToLabel(score float64, hmmState string) string {
	if hmmState == priceSvc.HMMCrisis {
		return "CRISIS"
	}
	if hmmState == priceSvc.HMMTrending {
		return "TRENDING"
	}
	switch {
	case score >= 60:
		return "BULLISH"
	case score >= 30:
		return "MILDLY BULLISH"
	case score > -30:
		return "NEUTRAL"
	case score > -60:
		return "MILDLY BEARISH"
	default:
		return "BEARISH"
	}
}

func buildDescription(o *RegimeOverlay) string {
	parts := []string{}

	// Trend component
	switch o.ADXStrength {
	case "STRONG":
		if o.ADXScore > 0 {
			parts = append(parts, "Trending↑")
		} else {
			parts = append(parts, "Trending↓")
		}
	case "MODERATE":
		parts = append(parts, "Moderate Trend")
	default:
		parts = append(parts, "Ranging")
	}

	// Vol component
	switch o.GARCHVolRegime {
	case "CONTRACTING":
		parts = append(parts, "Low Vol")
	case "EXPANDING":
		parts = append(parts, "High Vol")
	default:
		parts = append(parts, "Normal Vol")
	}

	// COT component
	switch o.COTBias {
	case "BULLISH":
		parts = append(parts, "COT Long")
	case "BEARISH":
		parts = append(parts, "COT Short")
	}

	return strings.Join(parts, ", ")
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

// dailyToPriceRecords converts DailyPrice slice to PriceRecord slice (newest-first).
func dailyToPriceRecords(daily []domain.DailyPrice) []domain.PriceRecord {
	// daily is assumed newest-first from repository
	out := make([]domain.PriceRecord, len(daily))
	for i, d := range daily {
		out[i] = domain.PriceRecord{
			ContractCode: d.ContractCode,
			Symbol:       d.Symbol,
			Date:         d.Date,
			Open:         d.Open,
			High:         d.High,
			Low:          d.Low,
			Close:        d.Close,
			Volume:       d.Volume,
			Source:       d.Source,
		}
	}
	return out
}

// dailyToOHLCV converts DailyPrice slice to ta.OHLCV slice (newest-first).
func dailyToOHLCV(daily []domain.DailyPrice) []ta.OHLCV {
	out := make([]ta.OHLCV, len(daily))
	for i, d := range daily {
		out[i] = ta.OHLCV{
			Open:   d.Open,
			High:   d.High,
			Low:    d.Low,
			Close:  d.Close,
			Volume: d.Volume,
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Cache helpers
// ---------------------------------------------------------------------------

func (e *OverlayEngine) getCached(key string) *RegimeOverlay {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, ok := e.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.result
}

func (e *OverlayEngine) setCached(key string, r *RegimeOverlay, ttl time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache[key] = &cacheEntry{result: r, expiresAt: time.Now().Add(ttl)}
}

// ---------------------------------------------------------------------------
// Math helpers
// ---------------------------------------------------------------------------

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func maxFloat64(vals ...float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// Ensure math import is used (for potential future extensions).
var _ = math.Abs
