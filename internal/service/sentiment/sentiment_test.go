package sentiment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test 1: normalizeFearGreedLabel — pure function, all branches
// ---------------------------------------------------------------------------

func TestNormalizeFearGreedLabel(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Extreme Fear", "Extreme Fear"},
		{"extreme fear", "Extreme Fear"},
		{"EXTREME FEAR", "Extreme Fear"},
		{"Fear", "Fear"},
		{"fear", "Fear"},
		{"FEAR", "Fear"},
		{"Neutral", "Neutral"},
		{"neutral", "Neutral"},
		{"  neutral  ", "Neutral"}, // extra whitespace
		{"Greed", "Greed"},
		{"greed", "Greed"},
		{"Extreme Greed", "Extreme Greed"},
		{"extreme greed", "Extreme Greed"},
		{"EXTREME GREED", "Extreme Greed"},
		{"Custom Label", "Custom Label"}, // passthrough for unknown values
		{"", "Unknown"},                 // empty → Unknown
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeFearGreedLabel(tc.input)
			assert.Equal(t, tc.want, got, "input=%q", tc.input)
		})
	}
}

// ---------------------------------------------------------------------------
// Test 2: SentimentData zero value — no panic on field access
// ---------------------------------------------------------------------------

func TestSentimentData_ZeroValue(t *testing.T) {
	var data SentimentData

	// All Available flags should be false
	assert.False(t, data.AAIIAvailable)
	assert.False(t, data.CNNAvailable)
	assert.False(t, data.PutCallAvailable)
	assert.False(t, data.CryptoFearGreedAvailable)

	// Numeric fields should be zero
	assert.Equal(t, float64(0), data.CNNFearGreed)
	assert.Equal(t, float64(0), data.AAIIBullish)
	assert.Equal(t, float64(0), data.AAIIBearish)
	assert.Equal(t, float64(0), data.AAIINeutral)
	assert.Equal(t, float64(0), data.PutCallTotal)
	assert.Equal(t, float64(0), data.CryptoFearGreed)

	// String fields should be empty
	assert.Equal(t, "", data.CNNFearGreedLabel)
	assert.Equal(t, "", data.PutCallSignal)

	// Time field should be zero
	assert.True(t, data.FetchedAt.IsZero())
}

// ---------------------------------------------------------------------------
// Test 3: NewSentimentFetcher — constructs with all circuit breakers non-nil
// ---------------------------------------------------------------------------

func TestNewSentimentFetcher(t *testing.T) {
	f := NewSentimentFetcher()
	require.NotNil(t, f)
	assert.NotNil(t, f.httpClient)
	assert.NotNil(t, f.cbCNN)
	assert.NotNil(t, f.cbAAII)
	assert.NotNil(t, f.cbCBOE)
	assert.NotNil(t, f.cbCrypto)
}

// ---------------------------------------------------------------------------
// Test 4: SentimentFetcher.Fetch — returns non-nil data and no error even when
// all external sources are unreachable (circuit breakers absorb failures)
// ---------------------------------------------------------------------------

func TestSentimentFetcher_Fetch_ReturnsDataOnAllFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: makes network attempts")
	}

	f := NewSentimentFetcher()
	// Use a very short timeout so external calls fail fast
	f.httpClient.Timeout = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	data, err := f.Fetch(ctx)

	// Error must always be nil — individual source failures are swallowed
	assert.NoError(t, err)
	// Data must be non-nil
	require.NotNil(t, data)
	// FetchedAt should be populated
	assert.False(t, data.FetchedAt.IsZero())
}

// ---------------------------------------------------------------------------
// Test 5: SentimentData — field assignment roundtrip (no panic, correct values)
// ---------------------------------------------------------------------------

func TestSentimentData_FieldAssignment(t *testing.T) {
	data := SentimentData{
		CNNFearGreed:             72.5,
		CNNFearGreedLabel:        "Greed",
		CNNPrevClose:             68.0,
		CNNPrev1Week:             55.0,
		CNNPrev1Month:            30.0,
		CNNPrev1Year:             80.0,
		CNNAvailable:             true,
		AAIIBullish:              42.3,
		AAIIBearish:              28.1,
		AAIINeutral:              29.6,
		AAIIBullBear:             1.5,
		AAIIAvailable:            true,
		PutCallTotal:             0.85,
		PutCallEquity:            0.72,
		PutCallIndex:             1.10,
		PutCallSignal:            "NEUTRAL",
		PutCallAvailable:         true,
		CryptoFearGreed:          45.0,
		CryptoFearGreedLabel:     "Fear",
		CryptoFearGreedAvailable: true,
		FetchedAt:                time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
	}

	assert.InDelta(t, 72.5, data.CNNFearGreed, 0.001)
	assert.Equal(t, "Greed", data.CNNFearGreedLabel)
	assert.True(t, data.CNNAvailable)
	assert.True(t, data.AAIIAvailable)
	assert.InDelta(t, 42.3, data.AAIIBullish, 0.001)
	assert.Equal(t, "NEUTRAL", data.PutCallSignal)
	assert.True(t, data.PutCallAvailable)
	assert.InDelta(t, 45.0, data.CryptoFearGreed, 0.001)
	assert.True(t, data.CryptoFearGreedAvailable)
	assert.Equal(t, 2026, data.FetchedAt.Year())
}

// ---------------------------------------------------------------------------
// Test 6: normalizeFearGreedLabel — idempotent (already-normalized input)
// ---------------------------------------------------------------------------

func TestNormalizeFearGreedLabel_Idempotent(t *testing.T) {
	labels := []string{"Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"}
	for _, label := range labels {
		// Calling twice should produce the same result
		first := normalizeFearGreedLabel(label)
		second := normalizeFearGreedLabel(first)
		assert.Equal(t, first, second, "not idempotent for %q", label)
	}
}
