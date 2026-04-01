package fmtutil

import (
	"fmt"
	"math"
	"strings"
)

// ProgressBar renders a horizontal progress bar representing value/max.
//
//	ProgressBar(75, 100, 10, "▓", "░") => "▓▓▓▓▓▓▓░░░"
func ProgressBar(value, max float64, width int, fillChar, emptyChar string) string {
	if width <= 0 {
		width = 10
	}
	if max == 0 {
		return strings.Repeat(emptyChar, width)
	}
	ratio := math.Max(0, math.Min(1, value/max))
	filled := int(math.Round(ratio * float64(width)))
	return strings.Repeat(fillChar, filled) + strings.Repeat(emptyChar, width-filled)
}

// BarChart renders a labelled bar for a single named value, scaled to maxVal.
// Each row:  <label> [<bar>] <formatted-value>
//
// Example:
//
//	BarChart("EURUSD", 75, 100, 8, "▓", "░", "75.0")
//	=> "EURUSD [▓▓▓▓▓▓░░] 75.0"
func BarChart(label string, value, maxVal float64, width int, fillChar, emptyChar, fmtValue string) string {
	bar := ProgressBar(value, maxVal, width, fillChar, emptyChar)
	return fmt.Sprintf("%s [%s] %s", label, bar, fmtValue)
}
