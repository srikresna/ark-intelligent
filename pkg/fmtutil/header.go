package fmtutil

import "fmt"

// AnalysisHeader returns a bold Telegram HTML header for analysis messages.
// Example: AnalysisHeader("📊", "ICT/SMC ANALYSIS", "EURUSD", "H4")
//
//	=> "📊 <b>ICT/SMC ANALYSIS — EURUSD H4</b>\n"
func AnalysisHeader(emoji, title, symbol, timeframe string) string {
	if symbol == "" && timeframe == "" {
		return fmt.Sprintf("%s <b>%s</b>\n", emoji, title)
	}
	if timeframe == "" {
		return fmt.Sprintf("%s <b>%s — %s</b>\n", emoji, title, symbol)
	}
	return fmt.Sprintf("%s <b>%s — %s %s</b>\n", emoji, title, symbol, timeframe)
}

// SectionDivider returns a thin Unicode divider suitable for separating
// sections inside a Telegram HTML message.
func SectionDivider() string {
	return "─────────────────────"
}
