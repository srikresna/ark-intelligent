package regime

import "time"

// ---------------------------------------------------------------------------
// Market Regime Overlay Types
// ---------------------------------------------------------------------------
//
// RegimeOverlay is a unified market health score combining:
//   HMM state (30%) + GARCH volatility regime (25%) + ADX trend (25%) + COT sentiment (20%)
//
// Score range: -100 (extreme bearish/risk-off) to +100 (extreme bullish/risk-on)

// RegimeOverlay holds the unified market regime assessment.
type RegimeOverlay struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`

	// Sub-model results
	HMMState      string  `json:"hmm_state"`       // RISK_ON | RISK_OFF | CRISIS
	HMMConfidence float64 `json:"hmm_confidence"`  // 0..1 — highest state probability
	HMMScore      float64 `json:"hmm_score"`       // -100..+100 contribution

	GARCHVolRegime string  `json:"garch_vol_regime"` // EXPANDING | NORMAL | CONTRACTING
	VolRatio       float64 `json:"vol_ratio"`        // current/long-run vol
	GARCHScore     float64 `json:"garch_score"`      // -100..+100 contribution

	ADXStrength string  `json:"adx_strength"` // STRONG | MODERATE | WEAK
	ADXValue    float64 `json:"adx_value"`
	ADXScore    float64 `json:"adx_score"` // -100..+100 contribution

	COTSentiment float64 `json:"cot_sentiment"` // raw COT score (can be outside -100..100)
	COTBias      string  `json:"cot_bias"`      // BULLISH | BEARISH | NEUTRAL
	COTScore     float64 `json:"cot_score"`     // -100..+100 clamped contribution

	// Effective weights used (adjusted if sub-models unavailable)
	WeightHMM   float64 `json:"weight_hmm"`
	WeightGARCH float64 `json:"weight_garch"`
	WeightADX   float64 `json:"weight_adx"`
	WeightCOT   float64 `json:"weight_cot"`

	// Composite output
	UnifiedScore float64 `json:"unified_score"`  // -100..+100
	OverlayColor string  `json:"overlay_color"`  // 🟢 | 🟡 | 🔴
	Label        string  `json:"label"`          // BULLISH | NEUTRAL | BEARISH | RISK-OFF | CRISIS
	Description  string  `json:"description"`    // one-line human summary

	// Diagnostics
	ModelsUsed []string  `json:"models_used"`  // which sub-models contributed
	ComputedAt time.Time `json:"computed_at"`
}

// HeaderLine returns a compact one-line header for embedding in Telegram messages.
// Example: "📊 Regime: 🟢 BULLISH (+67) | Trending, Low Vol, COT Long"
func (r *RegimeOverlay) HeaderLine() string {
	return "📊 Regime: " + r.OverlayColor + " " + r.Label +
		" (" + formatScore(r.UnifiedScore) + ") | " + r.Description
}

// formatScore formats a score as "+67" or "-34" or "0".
func formatScore(s float64) string {
	v := int(s + 0.5)
	if s < 0 {
		v = int(s - 0.5)
	}
	if v >= 0 {
		return "+" + itoa(v)
	}
	return itoa(v)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	buf := [10]byte{}
	pos := len(buf)
	for i >= 10 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	pos--
	buf[pos] = byte('0' + i)
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
