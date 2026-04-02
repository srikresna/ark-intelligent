package fmtutil

import "fmt"

// Accessibility helpers — pair bare emoji with meaningful text labels
// so screen readers convey trading information instead of "GREEN CIRCLE".

// SentimentLabel returns "🟢 <pos>" or "🔴 <neg>" based on the boolean flag.
func SentimentLabel(positive bool, posText, negText string) string {
	if positive {
		return fmt.Sprintf("🟢 %s", posText)
	}
	return fmt.Sprintf("🔴 %s", negText)
}

// SignalDot returns a colored dot with a contextual label.
//
//	positive (score > threshold)  → "🟢 <label>"
//	negative (score < -threshold) → "🔴 <label>"
//	neutral                       → "⚪ <label>"
func SignalDot(score, threshold float64, posLabel, negLabel, neutralLabel string) string {
	if score > threshold {
		return fmt.Sprintf("🟢 %s", posLabel)
	}
	if score < -threshold {
		return fmt.Sprintf("🔴 %s", negLabel)
	}
	return fmt.Sprintf("⚪ %s", neutralLabel)
}

// BullBearNeutral returns emoji + label for directional bias strings.
//
//	"BULLISH" → "🟢 Bullish"
//	"BEARISH" → "🔴 Bearish"
//	other     → "⚪ Neutral"
func BullBearNeutral(bias string) string {
	switch bias {
	case "BULLISH":
		return "🟢 Bullish"
	case "BEARISH":
		return "🔴 Bearish"
	default:
		return "⚪ Neutral"
	}
}

// ChangeLabel returns emoji + label based on numeric sign.
//
//	v > 0 → "🟢 Up"
//	v < 0 → "🔴 Down"
//	v == 0 → "⚪ Flat"
func ChangeLabel(v float64) string {
	if v > 0 {
		return "🟢 Up"
	}
	if v < 0 {
		return "🔴 Down"
	}
	return "⚪ Flat"
}

// ConfidenceLabel returns emoji + label for confidence levels.
//
//	"HIGH"   → "🟢 High"
//	"MEDIUM" → "🟡 Medium"
//	"LOW"    → "🔴 Low"
func ConfidenceLabel(level string) string {
	switch level {
	case "HIGH":
		return "🟢 High"
	case "MEDIUM":
		return "🟡 Medium"
	case "LOW":
		return "🔴 Low"
	default:
		return ""
	}
}

// StabilityLabel returns emoji + label based on a percentage stability score.
func StabilityLabel(pct float64) string {
	if pct >= 70 {
		return "🟢 Stable"
	}
	if pct >= 50 {
		return "🟡 Moderate"
	}
	return "🔴 Unstable"
}

// RiskOnOffLabel returns emoji + label for HMM-style regime states.
func RiskOnOffLabel(state string) string {
	switch state {
	case "RISK_ON":
		return "🟢 Risk-On"
	case "RISK_OFF":
		return "🟡 Risk-Off"
	case "CRISIS":
		return "🔴 Crisis"
	default:
		return "⚪ Unknown"
	}
}

// AnomalyLabel returns emoji + label for anomaly detection.
func AnomalyLabel(isAnomaly bool) string {
	if isAnomaly {
		return "🔴 Anomaly"
	}
	return "🟢 Normal"
}
