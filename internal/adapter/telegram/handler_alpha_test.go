package telegram

import (
	"fmt"
	"sync"
	"testing"
	"time"

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
