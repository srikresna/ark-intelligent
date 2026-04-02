package telegram

// format_briefing.go — Formatter for /briefing daily morning summary

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
)

// BriefingData holds the pre-fetched data needed to render a daily briefing.
type BriefingData struct {
	Now         time.Time
	Events      []domain.NewsEvent    // Today's calendar events (High + Medium)
	Convictions []cot.ConvictionScore // Sorted by Score desc
	BiasSummary string                // One-liner bias summary (e.g. "USD Strong · EUR Neutral")
}

// FormatBriefing renders the daily briefing message in Telegram HTML.
// Compact format — max 15 lines as per task spec.
func (f *Formatter) FormatBriefing(data BriefingData) string {
	var b strings.Builder

	// ── Header ──────────────────────────────────────────────────────────────
	dayLabel := data.Now.Format("Monday, 2 January 2006")
	timeLabel := data.Now.Format("15:04") + " WIB"
	b.WriteString(fmt.Sprintf("🌅 <b>ARK Daily Briefing</b>\n"))
	b.WriteString(fmt.Sprintf("<i>📅 %s · %s</i>\n", dayLabel, timeLabel))

	// ── Calendar Events ──────────────────────────────────────────────────────
	highMed := filterBriefingEvents(data.Events)
	b.WriteString("\n📅 <b>Events Hari Ini</b>\n")
	if len(highMed) == 0 {
		b.WriteString("<i>Tidak ada event High/Medium impact hari ini</i>\n")
	} else {
		// Sort by time
		sort.Slice(highMed, func(i, j int) bool {
			return highMed[i].TimeWIB.Before(highMed[j].TimeWIB)
		})
		// Show max 5 events
		shown := highMed
		if len(shown) > 5 {
			shown = shown[:5]
		}
		for _, e := range shown {
			impactIcon := "⚠️"
			if e.Impact == "medium" {
				impactIcon = "📊"
			}
			timeStr := e.Time
			if !e.TimeWIB.IsZero() {
				timeStr = e.TimeWIB.Format("15:04")
			}
			b.WriteString(fmt.Sprintf("%s <b>%s</b> • %s <i>%s</i>\n",
				impactIcon, e.Currency, timeStr, e.Event))
		}
		if len(highMed) > 5 {
			b.WriteString(fmt.Sprintf("<i>  +%d events lagi</i>\n", len(highMed)-5))
		}
	}

	// ── Top 3 COT Conviction Scores ──────────────────────────────────────────
	b.WriteString("\n🎯 <b>Top 3 COT Signals</b>\n")
	top := topConvictions(data.Convictions, 3)
	if len(top) == 0 {
		b.WriteString("<i>Data COT belum tersedia</i>\n")
	} else {
		for _, cs := range top {
			icon := "🟢"
			dirLabel := "BULLISH"
			if cs.Direction == "SHORT" {
				icon = "🔴"
				dirLabel = "BEARISH"
			} else if cs.Direction == "NEUTRAL" {
				icon = "⚪"
				dirLabel = "NEUTRAL"
			}
			b.WriteString(fmt.Sprintf("%s <b>%s</b>: %s (Conv: %.0f%%)\n",
				icon, cs.Currency, dirLabel, cs.Score))
		}
	}

	// ── Bias Summary ─────────────────────────────────────────────────────────
	if data.BiasSummary != "" {
		b.WriteString("\n📊 <b>Bias Summary</b>\n")
		b.WriteString(data.BiasSummary + "\n")
	}

	// ── Footer ───────────────────────────────────────────────────────────────
	b.WriteString(fmt.Sprintf("\n<code>Updated: %s</code>", timeLabel))

	return b.String()
}

// filterBriefingEvents returns only High and Medium impact events from a list.
func filterBriefingEvents(events []domain.NewsEvent) []domain.NewsEvent {
	var out []domain.NewsEvent
	for _, e := range events {
		if e.Impact == "high" || e.Impact == "medium" {
			out = append(out, e)
		}
	}
	return out
}

// topConvictions returns the top n conviction scores sorted by absolute Score descending.
// NEUTRAL scores are included only when there aren't enough LONG/SHORT signals.
func topConvictions(scores []cot.ConvictionScore, n int) []cot.ConvictionScore {
	if len(scores) == 0 {
		return nil
	}

	sorted := make([]cot.ConvictionScore, len(scores))
	copy(sorted, scores)

	// Sort by score descending (highest conviction first, regardless of direction)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	// Prefer LONG/SHORT over NEUTRAL
	var directional, neutral []cot.ConvictionScore
	for _, cs := range sorted {
		if cs.Direction == "NEUTRAL" {
			neutral = append(neutral, cs)
		} else {
			directional = append(directional, cs)
		}
	}

	result := directional
	if len(result) < n {
		result = append(result, neutral...)
	}

	if len(result) > n {
		return result[:n]
	}
	return result
}

// FormatBriefingBiasSummary builds a one-liner bias string from conviction scores.
// Example: "USD Strong · EUR Neutral · GBP Weak"
func FormatBriefingBiasSummary(scores []cot.ConvictionScore) string {
	if len(scores) == 0 {
		return ""
	}

	// Show top 5 by absolute score
	sorted := make([]cot.ConvictionScore, len(scores))
	copy(sorted, scores)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})
	if len(sorted) > 5 {
		sorted = sorted[:5]
	}

	parts := make([]string, 0, len(sorted))
	for _, cs := range sorted {
		strength := "Neutral"
		if cs.Score >= 70 {
			strength = "Strong"
		} else if cs.Score >= 50 {
			strength = "Moderate"
		} else if cs.Score < 30 && cs.Direction != "NEUTRAL" {
			strength = "Weak"
		}

		dirStr := ""
		if cs.Direction == "LONG" {
			dirStr = "↑"
		} else if cs.Direction == "SHORT" {
			dirStr = "↓"
		}

		parts = append(parts, fmt.Sprintf("%s%s %s", cs.Currency, dirStr, strength))
	}
	return strings.Join(parts, " · ")
}
