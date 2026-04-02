package fmtutil

import "strings"

// BiasIcon returns a colour-coded emoji for a directional bias string.
// Recognised values: "BULLISH", "BEARISH", anything else → "⚪".
func BiasIcon(bias string) string {
	switch strings.ToUpper(bias) {
	case "BULLISH":
		return "🟢 Bullish"
	case "BEARISH":
		return "🔴 Bearish"
	default:
		return "⚪ Neutral"
	}
}

// DirectionIcon returns an arrow emoji for a direction string.
// Recognised values: "UP"/"BULLISH"/"BUY" → "⬆️", "DOWN"/"BEARISH"/"SELL" → "⬇️",
// everything else → "➡️".
func DirectionIcon(dir string) string {
	switch strings.ToUpper(dir) {
	case "UP", "BULLISH", "BUY", "LONG":
		return "⬆️"
	case "DOWN", "BEARISH", "SELL", "SHORT":
		return "⬇️"
	default:
		return "➡️"
	}
}

// RegimeEmoji returns an emoji representing a market regime label.
// Recognised values: "BULL", "BULLISH", "BEAR", "BEARISH", "RANGING", "NEUTRAL".
func RegimeEmoji(regime string) string {
	switch strings.ToUpper(regime) {
	case "BULL", "BULLISH", "TRENDING_UP":
		return "🚀"
	case "BEAR", "BEARISH", "TRENDING_DOWN":
		return "📉"
	case "RANGING", "NEUTRAL", "SIDEWAYS":
		return "↔️"
	default:
		return "❓"
	}
}

// AccumulationDistributionIcon returns an icon for Wyckoff schematic.
func AccumulationDistributionIcon(schematic string) string {
	switch strings.ToUpper(schematic) {
	case "ACCUMULATION":
		return "🟢 Accumulation"
	case "DISTRIBUTION":
		return "🔴 Distribution"
	default:
		return "📊 Mixed"
	}
}
