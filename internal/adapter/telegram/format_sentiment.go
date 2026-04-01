package telegram

import (
	"strings"
)

// sentimentGauge builds a visual gauge bar for Fear & Greed (0-100).
func sentimentGauge(score float64, width int) string {
	pos := int(score / 100 * float64(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}

	bar := make([]byte, width)
	for i := range bar {
		bar[i] = '-'
	}
	bar[pos] = '|'

	return "Fear " + string(bar) + " Greed"
}

// fearGreedEmoji returns an emoji indicator for the CNN F&G score.
func fearGreedEmoji(score float64) string {
	switch {
	case score <= 25:
		return "😱"
	case score <= 45:
		return "😟"
	case score <= 55:
		return "😐"
	case score <= 75:
		return "😏"
	default:
		return "🤑"
	}
}

// sentimentBar builds a compact visual bar for a percentage (0-100).
func sentimentBar(pct float64, emoji string) string {
	const barWidth = 10
	filled := int(pct / 100 * barWidth)
	if filled > barWidth {
		filled = barWidth
	}
	return strings.Repeat(emoji, filled)
}
