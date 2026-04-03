package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// <b>, <i>, <code>, <pre>, <a>, <s>, <u>, <tg-spoiler>
type Formatter struct{}

// NewFormatter creates a new Formatter.
func NewFormatter() *Formatter {
	return &Formatter{}
}

// parseNumeric strips common suffixes and parses a numeric value from a string.
func parseNumeric(s string) *float64 {
	s = strings.TrimSpace(s)
	// Remove trailing %, K, M, B, and common suffixes
	s = strings.TrimRight(s, "%KMBkmb")
	s = strings.ReplaceAll(s, ",", "")
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return &f
	}
	return nil
}

// directionArrow checks if Actual beats Forecast using numeric comparison,
// respecting ImpactDirection from MQL5 so inverted indicators (unemployment,
// CPI miss, trade deficit) show the correct color for the currency.
//
// impactDirection semantics (MQL5):
//   0 = neutral/unknown  → fall back to raw numeric comparison
//   1 = higher actual is bullish for the currency (e.g. NFP, GDP)
//   2 = higher actual is bearish for the currency (e.g. Unemployment Claims, CPI when above target)
func directionArrow(actual, forecast string, impactDirection ...int) string {
	if actual == "" || forecast == "" {
		return "⚪ Pending"
	}
	aVal := parseNumeric(actual)
	fVal := parseNumeric(forecast)
	if aVal == nil || fVal == nil {
		return "⚪ N/A"
	}

	diff := *aVal - *fVal

	// Determine effective direction using ImpactDirection when provided
	dir := 0
	if len(impactDirection) > 0 {
		dir = impactDirection[0]
	}

	var effectiveDiff float64
	switch dir {
	case 1:
		// Higher actual = bullish for currency (normal indicator)
		effectiveDiff = diff
	case 2:
		// Higher actual = bearish for currency (inverted indicator: unemployment, deficits, etc.)
		effectiveDiff = -diff
	default:
		// Unknown direction: use raw diff
		effectiveDiff = diff
	}

	if effectiveDiff > 0 {
		return "🟢 Beat"
	} else if effectiveDiff < 0 {
		return "🔴 Miss"
	}
	return "⚪ In-line"
}

// FormatSettings formats the user preferences display.
func (f *Formatter) FormatSettings(prefs domain.UserPrefs) string {
	var b strings.Builder

	aiReports := "OFF"
	if prefs.AIReportsEnabled {
		aiReports = "ON"
	}

	cotAlerts := "OFF"
	if prefs.COTAlertsEnabled {
		cotAlerts = "ON"
	}

	b.WriteString("🦅 <b>ARK Intelligence Settings</b>\n\n")
	b.WriteString(fmt.Sprintf("<code>[COT] Release Alerts: %s</code>\n", cotAlerts))
	b.WriteString(fmt.Sprintf("<code>[AI] Weekly Reports : %s</code>\n", aiReports))

	langDisplay := "Indonesian 🇮🇩"
	if prefs.Language == "en" {
		langDisplay = "English 🇬🇧"
	}
	b.WriteString(fmt.Sprintf("<code>[AI] Output Language: %s</code>\n", langDisplay))

	modelDisplay := "Claude 🤖"
	if prefs.PreferredModel == "gemini" {
		modelDisplay = "Gemini ✨"
	}
	b.WriteString(fmt.Sprintf("<code>[AI] Chat Model    : %s</code>\n", modelDisplay))

	// Show active Claude model variant (only when using Claude)
	if prefs.PreferredModel != "gemini" {
		claudeVariant := "Server Default"
		if prefs.ClaudeModel != "" {
			claudeVariant = domain.ClaudeModelLabel(prefs.ClaudeModel)
		}
		b.WriteString(fmt.Sprintf("<code>[AI] Claude Variant : %s</code>\n", claudeVariant))
	}

	// Output format mode
	b.WriteString(fmt.Sprintf("<code>[UI] Format Output: %s</code>\n", domain.OutputModeLabel(prefs.OutputMode)))

	// Alert minutes display
	if len(prefs.AlertMinutes) > 0 {
		parts := make([]string, len(prefs.AlertMinutes))
		for i, m := range prefs.AlertMinutes {
			parts[i] = fmt.Sprintf("%d", m)
		}
		b.WriteString(fmt.Sprintf("<code>Alert Minutes      : %s</code>\n", strings.Join(parts, "/")))
	} else {
		b.WriteString("<code>Alert Minutes      : -</code>\n")
	}

	// Currency filter display
	if len(prefs.CurrencyFilter) > 0 {
		b.WriteString(fmt.Sprintf("<code>Alert Currencies   : %s</code>\n", strings.Join(prefs.CurrencyFilter, ", ")))
	} else {
		b.WriteString("<code>Alert Currencies   : All Currencies</code>\n")
	}

	// Quiet hours status (TASK-202)
	if prefs.QuietHoursEnabled {
		b.WriteString(fmt.Sprintf("<code>🌙 Quiet Hours  : %02d:00–%02d:00 WIB</code>\n", prefs.QuietHoursStart, prefs.QuietHoursEnd))
	}
	if prefs.MaxAlertsPerDay > 0 {
		b.WriteString(fmt.Sprintf("<code>📋 Daily Cap    : %d alerts/day</code>\n", prefs.MaxAlertsPerDay))
	}

	b.WriteString("\n<i>Use the buttons below to adjust preferences</i>")

	return b.String()
}


// FormatAlertManagement formats the alert management sub-menu display (TASK-202).
func (f *Formatter) FormatAlertManagement(prefs domain.UserPrefs) string {
	var b strings.Builder

	b.WriteString("🔔 <b>Alert Management</b>\n\n")

	// Quiet hours status
	if prefs.QuietHoursEnabled {
		b.WriteString(fmt.Sprintf("<code>🌙 Quiet Hours : ON (%02d:00–%02d:00 WIB)</code>\n",
			prefs.QuietHoursStart, prefs.QuietHoursEnd))
	} else {
		b.WriteString("<code>🌙 Quiet Hours : OFF</code>\n")
	}

	// Alert types status
	b.WriteString("\n<b>Alert Types:</b>\n")
	for _, key := range domain.ValidAlertTypes() {
		status := "✅ ON"
		if !prefs.IsAlertTypeEnabled(key) {
			status = "❌ OFF"
		}
		b.WriteString(fmt.Sprintf("<code>  %s: %s</code>\n", domain.AlertTypeLabel(key), status))
	}

	// Daily cap
	b.WriteString("\n")
	if prefs.MaxAlertsPerDay > 0 {
		b.WriteString(fmt.Sprintf("<code>📋 Daily Cap   : %d alerts/day</code>\n", prefs.MaxAlertsPerDay))
	} else {
		b.WriteString("<code>📋 Daily Cap   : Unlimited</code>\n")
	}

	b.WriteString("\n<i>Use the buttons below to adjust alert preferences</i>")

	return b.String()
}

// formatProgressBar creates a text-based progress bar for COT Index.
func (f *Formatter) formatProgressBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("#", filled) + strings.Repeat(".", width-filled)

	// Mark extreme zones
	label := ""
	if pct >= 80 {
		label = " EXTREME LONG"
	} else if pct <= 20 {
		label = " EXTREME SHORT"
	}

	return fmt.Sprintf("<code>  [%s] %.0f%%%s</code>\n", bar, pct, label)
}

// contractCodeToFriendly maps CFTC numeric contract codes to user-friendly currency shortcuts.
// Returns the input unchanged if no mapping exists.
func contractCodeToFriendly(code string) string {
	m := map[string]string{
		string(domain.ContractEUR):    "EUR",
		string(domain.ContractGBP):    "GBP",
		string(domain.ContractJPY):    "JPY",
		string(domain.ContractCHF):    "CHF",
		string(domain.ContractAUD):    "AUD",
		string(domain.ContractCAD):    "CAD",
		string(domain.ContractNZD):    "NZD",
		string(domain.ContractDXY):    "USD",
		string(domain.ContractGold):   "GOLD",
		string(domain.ContractSilver): "SILVER",
		string(domain.ContractCopper): "COPPER",
		string(domain.ContractOil):    "OIL",
		"022651": "ULSD",
		"111659": "RBOB",
		"043602": "BOND10",
		"020601": "BOND30",
		"044601": "BOND5",
		"042601": "BOND2",
		"13874A": "SPX",
		string(domain.ContractNasdaq): "NDX",
		"124601": "DJI",
		"239742": "RUT",
		string(domain.ContractBTC):    "BTC",
		"146021": "ETH",
	}
	if friendly, ok := m[code]; ok {
		return friendly
	}
	return code
}

// scoreArrow returns directional arrows for a sentiment score.
func scoreArrow(score float64) string {
	switch {
	case score > 60:
		return "↑↑"
	case score > 30:
		return "↑"
	case score > -30:
		return "→"
	case score > -60:
		return "↓"
	default:
		return "↓↓↓"
	}
}

// scoreDot returns a colored dot based on score direction.
func scoreDot(score float64) string {
	if score > 15 {
		return "🟢 Bullish"
	} else if score < -15 {
		return "🔴 Bearish"
	}
	return "⚪ Neutral"
}

// trendLabel converts a direction string to a human-readable trend label.
func trendLabel(direction string) string {
	switch direction {
	case "UP":
		return "RISING"
	case "DOWN":
		return "FALLING"
	default:
		return "STABLE"
	}
}

// shortDirection returns a compact direction label.
func shortDirection(d string) string {
	switch d {
	case "BULLISH":
		return "\xF0\x9F\x9F\xA2 Bullish BULL"
	case "BEARISH":
		return "\xF0\x9F\x94\xB4 Bearish BEAR"
	default:
		return d
	}
}

// resultBadge returns an emoji badge for a signal outcome.
func resultBadge(r string) string {
	switch r {
	case domain.OutcomeWin:
		return "\xE2\x9C\x85"
	case domain.OutcomeLoss:
		return "\xE2\x9D\x8C"
	default:
		return "\xE2\x8F\xB3"
	}
}

// truncateStr shortens a string to maxLen, adding ".." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

// FormatTrackedEvents formats a list of tracked event names for the /impact help message.
func (f *Formatter) FormatTrackedEvents(events []string) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x8B <b>EVENT IMPACT DATABASE</b>\n")
	b.WriteString("<i>Historical price reaction tracking</i>\n\n")

	if len(events) == 0 {
		b.WriteString("No events tracked yet. Impact data builds automatically\n")
		b.WriteString("after each economic release with price data available.\n\n")
		b.WriteString("Usage: <code>/impact NFP</code> or <code>/impact CPI</code>")
		return b.String()
	}

	b.WriteString("<b>Tracked Events:</b>\n")
	for i, ev := range events {
		if i >= 20 {
			b.WriteString(fmt.Sprintf("\n<i>... and %d more</i>", len(events)-20))
			break
		}
		b.WriteString(fmt.Sprintf("\xE2\x80\xA2 %s\n", ev))
	}

	b.WriteString("\nUsage: <code>/impact Event Name</code>\n")
	b.WriteString("Example: <code>/impact Non-Farm Employment Change</code>")
	return b.String()
}

// FormatRegimeOverlayHeader formats a one-line regime overlay header for embedding
// at the top of analysis output (e.g. /cta, /quant).
// Returns empty string if overlay is nil.
func (f *Formatter) FormatRegimeOverlayHeader(overlay interface{ HeaderLine() string }) string {
	if overlay == nil {
		return ""
	}
	return overlay.HeaderLine() + "\n"
}
