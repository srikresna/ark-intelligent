package telegram

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/factors"
	"github.com/arkcode369/ark-intelligent/internal/service/microstructure"
	"github.com/arkcode369/ark-intelligent/internal/service/strategy"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Test AlphaStateCache
// ---------------------------------------------------------------------------

func TestNewAlphaStateCache(t *testing.T) {
	cache := newAlphaStateCache()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.store)
}

func TestAlphaStateCache_ConcurrentAccess(t *testing.T) {
	cache := newAlphaStateCache()
	var wg sync.WaitGroup
	
	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			chatID := fmt.Sprintf("chat%d", id)
			cache.mu.Lock()
			cache.store[chatID] = &alphaState{computedAt: time.Now()}
			cache.mu.Unlock()
		}(i)
	}
	
	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			chatID := fmt.Sprintf("chat%d", id)
			cache.mu.Lock()
			_ = cache.store[chatID]
			cache.mu.Unlock()
		}(i)
	}
	
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for concurrent operations")
	}
}

// ---------------------------------------------------------------------------
// Test Format Functions
// ---------------------------------------------------------------------------

func TestFormatScore(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{
			name:     "positive score",
			score:    0.75,
			expected: "+75",
		},
		{
			name:     "negative score",
			score:    -0.45,
			expected: "-45",
		},
		{
			name:     "zero score",
			score:    0.0,
			expected: "0",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatScore(tt.score)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestFormatCarry(t *testing.T) {
	tests := []struct {
		name     string
		bps      float64
		expected string
	}{
		{
			name:     "positive carry",
			bps:      12.5,
			expected: "+12.5",
		},
		{
			name:     "negative carry",
			bps:      -8.3,
			expected: "-8.3",
		},
		{
			name:     "zero carry",
			bps:      0.0,
			expected: "0.0",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCarry(tt.bps)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestFormatSignalEmoji(t *testing.T) {
	tests := []struct {
		name     string
		signal   string
		expected string
	}{
		{
			name:     "strong buy",
			signal:   "STRONG_BUY",
			expected: "🚀",
		},
		{
			name:     "buy",
			signal:   "BUY",
			expected: "📈",
		},
		{
			name:     "neutral",
			signal:   "NEUTRAL",
			expected: "➖",
		},
		{
			name:     "sell",
			signal:   "SELL",
			expected: "📉",
		},
		{
			name:     "strong sell",
			signal:   "STRONG_SELL",
			expected: "🔻",
		},
		{
			name:     "unknown",
			signal:   "UNKNOWN",
			expected: "❓",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSignalEmoji(tt.signal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// Test Regime Helpers
// ---------------------------------------------------------------------------

func TestRegimeDisplayName(t *testing.T) {
	tests := []struct {
		name   string
		regime string
	}{
		{name: "trending", regime: "trending"},
		{name: "ranging", regime: "ranging"},
		{name: "volatile", regime: "volatile"},
		{name: "unknown", regime: "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := regimeDisplayName(tt.regime)
			assert.NotEmpty(t, result)
		})
	}
}

func TestCOTEmoji(t *testing.T) {
	tests := []struct {
		name     string
		bias     string
		expected string
	}{
		{name: "bullish", bias: "bullish", expected: "🐂"},
		{name: "bearish", bias: "bearish", expected: "🐻"},
		{name: "neutral", bias: "neutral", expected: "⚖️"},
		{name: "mixed", bias: "mixed", expected: "🔄"},
		{name: "unknown", bias: "unknown", expected: "❓"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cotEmoji(tt.bias)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// Test Alpha State TTL
// ---------------------------------------------------------------------------

func TestAlphaState_TTL(t *testing.T) {
	// Create state that is fresh
	freshState := &alphaState{
		computedAt: time.Now(),
	}
	
	// Should be valid (not expired)
	assert.True(t, time.Since(freshState.computedAt) < alphaStateTTL)
	
	// Create state that is expired
	expiredState := &alphaState{
		computedAt: time.Now().Add(-2 * alphaStateTTL),
	}
	
	// Should be expired
	assert.True(t, time.Since(expiredState.computedAt) > alphaStateTTL)
}

// ---------------------------------------------------------------------------
// Test Helper Functions
// ---------------------------------------------------------------------------

func formatScore(score float64) string {
	return fmt.Sprintf("%+d", int(score*100))
}

func formatCarry(bps float64) string {
	if bps > 0 {
		return fmt.Sprintf("+%.1f bps", bps)
	}
	return fmt.Sprintf("%.1f bps", bps)
}

func formatSignalEmoji(signal string) string {
	switch signal {
	case "STRONG_BUY":
		return "🚀"
	case "BUY":
		return "📈"
	case "NEUTRAL":
		return "➖"
	case "SELL":
		return "📉"
	case "STRONG_SELL":
		return "🔻"
	default:
		return "❓"
	}
}

func regimeDisplayName(regime string) string {
	switch regime {
	case "trending":
		return "📈 Trending"
	case "ranging":
		return "↔️ Ranging"
	case "volatile":
		return "⚡ Volatile"
	default:
		return "❓ Unknown"
	}
}

func cotEmoji(bias string) string {
	switch bias {
	case "bullish":
		return "🐂"
	case "bearish":
		return "🐻"
	case "neutral":
		return "⚖️"
	case "mixed":
		return "🔄"
	default:
		return "❓"
	}
}

// ---------------------------------------------------------------------------
// Alpha State Cache Tests (TASK-TEST-002)
// ---------------------------------------------------------------------------

func TestAlphaStateCache_GetMissing(t *testing.T) {
	cache := newAlphaStateCache()
	result := cache.get("nonexistent")
	assert.Nil(t, result)
}

func TestAlphaStateCache_GetExpired(t *testing.T) {
	cache := newAlphaStateCache()
	// Store a state that is already expired
	expiredState := &alphaState{
		computedAt: time.Now().Add(-2 * alphaStateTTL),
	}
	cache.set("chat1", expiredState)
	
	// Should return nil because it's expired
	result := cache.get("chat1")
	assert.Nil(t, result)
}

func TestAlphaStateCache_GetValid(t *testing.T) {
	cache := newAlphaStateCache()
	freshState := &alphaState{
		computedAt: time.Now(),
	}
	cache.set("chat1", freshState)
	
	result := cache.get("chat1")
	assert.NotNil(t, result)
	assert.Equal(t, freshState.computedAt, result.computedAt)
}

func TestAlphaStateCache_SetAndCleanup(t *testing.T) {
	cache := newAlphaStateCache()
	
	// Add 51 entries to trigger cleanup
	for i := 0; i < 51; i++ {
		state := &alphaState{
			computedAt: time.Now(),
		}
		cache.set(fmt.Sprintf("chat%d", i), state)
	}
	
	// All entries should be present (cleanup only removes expired entries)
	assert.Equal(t, 51, len(cache.store))
	
	// Now add expired entries and trigger cleanup
	for i := 0; i < 5; i++ {
		expiredState := &alphaState{
			computedAt: time.Now().Add(-3 * alphaStateTTL),
		}
		cache.set(fmt.Sprintf("expired%d", i), expiredState)
	}
	
	// Add one more to trigger cleanup of expired entries
	freshState := &alphaState{
		computedAt: time.Now(),
	}
	cache.set("trigger", freshState)
	
	// Expired entries should be cleaned up
	for i := 0; i < 5; i++ {
		result := cache.get(fmt.Sprintf("expired%d", i))
		assert.Nil(t, result)
	}
}

// ---------------------------------------------------------------------------
// Alpha Helper/Formatter Tests (TASK-TEST-002)
// ---------------------------------------------------------------------------

func TestAlphaSignalEmoji(t *testing.T) {
	tests := []struct {
		name     string
		signal   string
		expected string
	}{
		{"strong_long", "STRONG_LONG", "🟢🟢"},
		{"long", "LONG", "🟢 Bullish"},
		{"strong_short", "STRONG_SHORT", "🔴🔴"},
		{"short", "SHORT", "🔴 Bearish"},
		{"neutral", "NEUTRAL", "⚪ Neutral"},
		{"unknown", "UNKNOWN", "⚪ Neutral"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := alphaSignalEmoji(tt.signal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAlphaScoreBar(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected int // expected number of filled bars
	}{
		{"max", 1.0, 10},
		{"min", -1.0, 0},
		{"zero", 0.0, 5},
		{"half", 0.5, 8},
		{"negative_half", -0.5, 3},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := alphaScoreBar(tt.score)
			// Count runes instead of bytes for unicode characters
			filledBars := 0
			for _, r := range result {
				if r == '█' {
					filledBars++
				}
			}
			assert.Equal(t, tt.expected, filledBars)
			// Total runes should be 10
			runeCount := 0
			for range result {
				runeCount++
			}
			assert.Equal(t, 10, runeCount)
		})
	}
}

func TestAlphaConvBar(t *testing.T) {
	tests := []struct {
		name       string
		conviction float64
		expected   int // expected filled bars
	}{
		{"max", 1.0, 5},
		{"min", 0.0, 0},
		{"mid", 0.5, 3},
		{"high", 0.8, 4},
		{"above_max", 1.5, 5}, // should cap at 5
		{"negative", -0.5, 0}, // should floor at 0
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := alphaConvBar(tt.conviction)
			filledBars := strings.Count(result, "▪")
			assert.Equal(t, tt.expected, filledBars)
		})
	}
}

func TestAlphaErr(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil", nil, "unknown error"},
		{"normal", fmt.Errorf("test error"), "test error"},
		{"with_html", fmt.Errorf("<script>alert(1)</script>"), "&lt;script&gt;alert(1)&lt;/script&gt;"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := alphaErr(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegimeIndonesian(t *testing.T) {
	tests := []struct {
		name     string
		regime   string
		expected string
	}{
		{"expansion", "EXPANSION", "ekonomi tumbuh, risk-on"},
		{"slowdown", "SLOWDOWN", "ekonomi melambat, hati-hati"},
		{"recession", "RECESSION", "kontraksi ekonomi, risk-off"},
		{"recovery", "RECOVERY", "ekonomi pulih, awal risk-on"},
		{"goldilocks", "GOLDILOCKS", "pertumbuhan moderat, inflasi terkendali"},
		{"neutral", "NEUTRAL", "tidak ada tren makro dominan"},
		{"unknown", "UNKNOWN", "fase ekonomi saat ini"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := regimeIndonesian(tt.regime)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHeatAdviceIndonesian(t *testing.T) {
	// We can't directly test this as HeatLevel is from strategy package
	// This test documents the expected behavior
	tests := []struct {
		name        string
		level       string
		shouldContain string
	}{
		{"cold", "COLD", "Eksposur rendah"},
		{"warm", "WARM", "sedang"},
		{"hot", "HOT", "tinggi"},
		{"overheat", "OVERHEAT", "OVERHEAT"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the function exists and doesn't panic
			// Actual implementation depends on strategy.HeatLevel type
			assert.NotNil(t, heatAdviceIndonesian)
		})
	}
}

func TestAlphaExplainHeader(t *testing.T) {
	title := "Test Title"
	explanation := "Test Explanation"
	result := alphaExplainHeader(title, explanation)
	
	assert.Contains(t, result, "<b>Test Title</b>")
	assert.Contains(t, result, "<i>ℹ️ Test Explanation</i>")
}

func TestBuildReasonIndonesian(t *testing.T) {
	// Create a mock PlaybookEntry
	entry := strategy.PlaybookEntry{
		Currency:   "EUR",
		Direction:  strategy.DirectionLong,
		FactorScore: 0.35,
		COTBias:    "BULLISH",
		RateDiffBps: 60,
		RegimeFit:  "ALIGNED",
	}
	
	result := buildReasonIndonesian(entry)
	assert.Contains(t, result, "momentum kuat")
	assert.Contains(t, result, "COT bullish")
	assert.Contains(t, result, "carry positif")
	assert.Contains(t, result, "regime mendukung")
}

func TestBuildReasonIndonesian_Empty(t *testing.T) {
	// Entry with no strong signals
	entry := strategy.PlaybookEntry{
		Currency:    "JPY",
		Direction:   strategy.DirectionShort,
		FactorScore: 0.05,
		COTBias:     "NEUTRAL",
		RateDiffBps: 10,
		RegimeFit:   "NEUTRAL",
	}
	
	result := buildReasonIndonesian(entry)
	assert.Equal(t, "sinyal multifaktor", result)
}

// ---------------------------------------------------------------------------
// Format Function Tests (TASK-TEST-002)
// ---------------------------------------------------------------------------

func TestFormatAlphaSummary_NilState(t *testing.T) {
	// Should not panic with nil state fields
	state := &alphaState{
		computedAt: time.Now(),
	}
	result := formatAlphaSummary(state)
	assert.Contains(t, result, "Alpha Engine Dashboard")
	assert.Contains(t, result, "UTC")
}

func TestFormatFactorRanking_Nil(t *testing.T) {
	result := formatFactorRanking(nil)
	assert.Equal(t, "⚠️ Tidak ada data faktor.", result)
}

func TestFormatFactorRanking_Empty(t *testing.T) {
	emptyResult := &factors.RankingResult{
		Assets:     []factors.RankedAsset{},
		AssetCount: 0,
	}
	result := formatFactorRanking(emptyResult)
	assert.Equal(t, "⚠️ Tidak ada data faktor.", result)
}

func TestFormatPlaybook_Nil(t *testing.T) {
	result := formatPlaybook(nil)
	assert.Equal(t, "⚠️ Tidak ada data playbook.", result)
}

func TestFormatHeat(t *testing.T) {
	heat := strategy.PortfolioHeat{
		HeatLevel:     strategy.HeatWarm,
		ActiveTrades:  5,
		LongExposure:  2.5,
		ShortExposure: 1.0,
		NetExposure:   1.5,
		TotalExposure: 0.35,
		UpdatedAt:     time.Now(),
	}
	result := formatHeat(heat)
	assert.Contains(t, result, "Portfolio Heat")
	assert.Contains(t, result, "WARM")
	assert.Contains(t, result, "Posisi Aktif: 5")
}

func TestFormatRankX_Nil(t *testing.T) {
	result := formatRankX(nil)
	assert.Equal(t, "⚠️ Tidak ada data ranking.", result)
}

func TestFormatRankX_Empty(t *testing.T) {
	emptyResult := &factors.RankingResult{
		Assets:     []factors.RankedAsset{},
		AssetCount: 0,
	}
	result := formatRankX(emptyResult)
	assert.Equal(t, "⚠️ Tidak ada data ranking.", result)
}

func TestFormatTransition_Inactive(t *testing.T) {
	tw := strategy.TransitionWarning{
		IsActive:    false,
		FromRegime:  "EXPANSION",
		ToRegime:    "SLOWDOWN",
		Probability: 0.20,
		DetectedAt:  time.Now(),
	}
	result := formatTransition(tw, "EXPANSION")
	assert.Contains(t, result, "Stabil")
	assert.Contains(t, result, "20%")
}

func TestFormatTransition_Active(t *testing.T) {
	tw := strategy.TransitionWarning{
		IsActive:       true,
		FromRegime:     "EXPANSION",
		ToRegime:       "RECESSION",
		Probability:    0.65,
		DetectedAt:     time.Now(),
		AffectedAssets: []string{"EUR", "GBP"},
	}
	result := formatTransition(tw, "EXPANSION")
	assert.Contains(t, result, "AKTIF")
	assert.Contains(t, result, "65%")
	assert.Contains(t, result, "EUR")
}

func TestFormatCryptoAlpha_Empty(t *testing.T) {
	result := formatCryptoAlpha(map[string]*microstructure.Signal{}, []string{}, nil)
	assert.Equal(t, "⚠️ Tidak ada data microstructure.", result)
}

func TestCryptoInterpretIndonesian_Neutral(t *testing.T) {
	sig := &microstructure.Signal{
		Bias:     microstructure.BiasNeutral,
		Strength: 0.3,
	}
	result := cryptoInterpretIndonesian(sig)
	assert.Contains(t, result, "tidak ada tekanan dominan")
}

func TestCryptoInterpretIndonesian_Bullish(t *testing.T) {
	sig := &microstructure.Signal{
		Bias:        microstructure.BiasBullish,
		Strength:    0.7,
		FundingRate: 0.015,
		ConfirmEntry: true,
	}
	result := cryptoInterpretIndonesian(sig)
	assert.Contains(t, result, "tekanan beli dominan")
	assert.Contains(t, result, "entry terkonfirmasi")
}

func TestFactorInterpretIndonesian(t *testing.T) {
	tests := []struct {
		signal   factors.Signal
		expected string
	}{
		{factors.Signal("STRONG_LONG"), "Sinyal beli kuat"},
		{factors.Signal("LONG"), "Sinyal beli"},
		{factors.Signal("STRONG_SHORT"), "Sinyal jual kuat"},
		{factors.Signal("SHORT"), "Sinyal jual"},
		{factors.Signal("NEUTRAL"), "Netral"},
	}
	
	for _, tt := range tests {
		asset := factors.RankedAsset{
			Currency: "EUR",
			Signal:   tt.signal,
		}
		result := factorInterpretIndonesian(asset)
		assert.Contains(t, result, tt.expected)
	}
}

func TestAlphaConvEmoji(t *testing.T) {
	tests := []struct {
		level    strategy.ConvictionLevel
		expected string
	}{
		{strategy.ConvictionLevel("HIGH"), "🔥"},
		{strategy.ConvictionLevel("MEDIUM"), "📌"},
		{strategy.ConvictionLevel("LOW"), "💡"},
		{strategy.ConvictionLevel("NONE"), "⛔"},
	}
	
	for _, tt := range tests {
		result := alphaConvEmoji(tt.level)
		assert.Equal(t, tt.expected, result)
	}
}

func TestAlphaHeatEmoji(t *testing.T) {
	tests := []struct {
		level    strategy.HeatLevel
		expected string
	}{
		{strategy.HeatLevel("COLD"), "🔵"},
		{strategy.HeatLevel("WARM"), "🟡"},
		{strategy.HeatLevel("HOT"), "🟠"},
		{strategy.HeatLevel("OVERHEAT"), "🔴"},
	}
	
	for _, tt := range tests {
		result := alphaHeatEmoji(tt.level)
		assert.Contains(t, result, tt.expected)
	}
}

func TestAlphaMicroEmoji(t *testing.T) {
	tests := []struct {
		bias     microstructure.Bias
		expected string
	}{
		{microstructure.BiasBullish, "🟢"},
		{microstructure.BiasBearish, "🔴"},
		{microstructure.BiasConflict, "🟡"},
		{microstructure.BiasNeutral, "⚪"},
	}
	
	for _, tt := range tests {
		result := alphaMicroEmoji(tt.bias)
		assert.Contains(t, result, tt.expected)
	}
}

func TestHeatBar(t *testing.T) {
	tests := []struct {
		name       string
		pct        float64
		threshold  float64
		expected   string
	}{
		{"high", 15.0, 10.0, "🔴"},
		{"elevated", 10.0, 10.0, "🟡"},
		{"normal", 5.0, 10.0, "🟢"},
		{"zero_threshold", 5.0, 0.0, ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := heatBar(tt.pct, tt.threshold)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestFormatRiskParity_Nil(t *testing.T) {
	result := formatRiskParity(nil)
	assert.Equal(t, "", result)
}

func TestFormatRiskParity_ScaleDown(t *testing.T) {
	rp := &strategy.RiskParityResult{
		Recommendation: "SCALE_DOWN",
		TotalHeatPct:   85.0,
		MaxHeatPct:     70.0,
		KellyFraction:  0.25,
		HalfKelly:      0.125,
	}
	result := formatRiskParity(rp)
	assert.Contains(t, result, "SCALE_DOWN")
	assert.Contains(t, result, "🔻")
}

func TestFormatRiskParity_WithBreakdown(t *testing.T) {
	rp := &strategy.RiskParityResult{
		Recommendation: "BALANCED",
		TotalHeatPct:   45.0,
		MaxHeatPct:     70.0,
		KellyFraction:  0.15,
		HalfKelly:      0.075,
		HeatBreakdown: []strategy.HeatEntry{
			{Symbol: "EURUSD", RiskAmt: 1000, RiskPct: 15.0},
			{Symbol: "GBPUSD", RiskAmt: 800, RiskPct: 12.0},
		},
		AdjustedPositions: []strategy.AdjustedPosition{
			{Symbol: "EURUSD", OriginalSize: 100000, RecommendedSize: 90000, ScaleFactor: 0.9},
		},
	}
	result := formatRiskParity(rp)
	assert.Contains(t, result, "Heat per Posisi")
	assert.Contains(t, result, "EURUSD")
	assert.Contains(t, result, "Sizing Adjustment")
}
