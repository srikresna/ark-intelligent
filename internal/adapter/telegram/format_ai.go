package telegram

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
	"time"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatWeeklyOutlook formats the AI-generated weekly market outlook.
func (f *Formatter) FormatWeeklyOutlook(outlook string, date time.Time) string {
	var b strings.Builder

	b.WriteString("<b>Weekly Market Outlook</b>\n")
	b.WriteString(fmt.Sprintf("<i>Week of %s</i>\n\n", date.Format("Jan 2, 2006")))
	b.WriteString(outlook)
	b.WriteString("\n\n<i>Tip: </i><code>/outlook cot</code> | <code>/outlook news</code> | <code>/outlook combine</code>")

	return b.String()
}

// FormatAIInsight wraps an AI narrative with a labeled section.
func (f *Formatter) FormatAIInsight(label, narrative string) string {
	return fmt.Sprintf("<b>%s Analysis:</b>\n<i>%s</i>", label, narrative)
}

// FormatWeeklyReport formats a WeeklyReport into Telegram HTML.
func (f *Formatter) FormatWeeklyReport(r *domain.WeeklyReport) string {
	var b strings.Builder

	b.WriteString("📋 <b>Weekly Performance Report</b>\n")
	b.WriteString(fmt.Sprintf("<i>%s \xe2\x80\x94 %s</i>\n\n",
		fmtutil.FormatDateShortWIB(r.WeekStart),
		fmtutil.FormatDateWIB(r.WeekEnd),
	))

	if len(r.Signals) == 0 {
		b.WriteString("No signals generated this week.\n")
	} else {
		// Cap displayed signals to avoid exceeding Telegram's 4096-char limit
		// when the <pre> block is too large (each row ~60 chars; 50 rows = 3000).
		maxDisplay := 50
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-5s %-14s %-8s %7s  %s\n", "CCY", "SIGNAL", "DIR", "MOVE", ""))
		b.WriteString(fmt.Sprintf("%-5s %-14s %-8s %7s  %s\n", "---", "-----------", "------", "------", "---"))
		for i, s := range r.Signals {
			if i >= maxDisplay {
				break
			}
			dir := shortDirection(s.Direction)
			result := resultBadge(s.Result)
			move := fmt.Sprintf("%+.2f%%", s.PipsChange)
			if s.Result == domain.OutcomePending {
				move = "   ---"
			}
			sigLabel := truncateStr(s.SignalType, 14)
			b.WriteString(fmt.Sprintf("%-5s %-14s %-8s %7s  %s\n",
				truncateStr(s.Contract, 5), sigLabel, dir, move, result))
		}
		b.WriteString("</pre>")
		if len(r.Signals) > maxDisplay {
			b.WriteString(fmt.Sprintf("\n<i>... +%d more signals (showing top %d)</i>\n", len(r.Signals)-maxDisplay, maxDisplay))
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString(fmt.Sprintf("<b>Weekly Score:</b> %s\n", r.WeeklyScore))

	if r.RunningAverage52W > 0 {
		b.WriteString(fmt.Sprintf("<b>52W Average:</b>  %.1f%%\n", r.RunningAverage52W))
	}

	b.WriteString(fmt.Sprintf("<b>Current Streak:</b> %d wins\n", r.CurrentStreak))
	b.WriteString(fmt.Sprintf("<b>Best Streak:</b>    %d wins\n", r.BestStreak))

	b.WriteString("\n<i>Use /backtest for full historical stats</i>")
	return b.String()
}

// FormatEventImpact formats event impact summaries into a clean Telegram HTML message
// with confidence labels, directional summary, and asymmetry analysis.
func (f *Formatter) FormatEventImpact(eventTitle string, summaries []domain.EventImpactSummary) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8A <b>EVENT IMPACT: %s</b>\n", strings.ToUpper(html.EscapeString(eventTitle))))
	b.WriteString("<i>Historical price reaction by surprise magnitude</i>\n\n")

	if len(summaries) == 0 {
		b.WriteString("No impact data recorded yet for this event.\n")
		b.WriteString("<i>Data builds automatically after each release.</i>")
		return b.String()
	}

	// Group by currency
	byCurrency := make(map[string][]domain.EventImpactSummary)
	var currencies []string
	for _, s := range summaries {
		if _, exists := byCurrency[s.Currency]; !exists {
			currencies = append(currencies, s.Currency)
		}
		byCurrency[s.Currency] = append(byCurrency[s.Currency], s)
	}
	sort.Strings(currencies)

	for _, ccy := range currencies {
		items := byCurrency[ccy]

		// Compute total data points for confidence.
		var totalN int
		for _, item := range items {
			totalN += item.Occurrences
		}

		// Confidence label.
		confidence := "HIGH"
		confIcon := "\xE2\x9C\x85" // ✅
		if totalN < 5 {
			confidence = "LOW"
			confIcon = "\xE2\x9A\xA0\xEF\xB8\x8F" // ⚠️
		} else if totalN < 12 {
			confidence = "MEDIUM"
			confIcon = "\xF0\x9F\x9F\xA1" // 🟡
		}

		b.WriteString(fmt.Sprintf("<b>%s</b> %s <i>%s (N=%d)</i>\n", ccy, confIcon, confidence, totalN))
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-14s %7s %7s %4s\n", "Sigma", "AvgPip", "Median", "N"))
		b.WriteString(strings.Repeat("\xE2\x94\x80", 36) + "\n")

		for _, item := range items {
			b.WriteString(fmt.Sprintf("%-14s %+7.1f %+7.1f %4d\n",
				item.SigmaBucket, item.AvgPriceImpactPips, item.MedianImpact, item.Occurrences))
		}
		b.WriteString("</pre>")

		// Directional summary + asymmetry.
		posAvg, posN, negAvg, negN := impactAsymmetry(items)
		if posN > 0 || negN > 0 {
			b.WriteString("<i>")
			if posN > 0 {
				b.WriteString(fmt.Sprintf("Beat \xE2\x86\x92 avg %+.1f pips (N=%d)", posAvg, posN))
			}
			if posN > 0 && negN > 0 {
				b.WriteString(" | ")
			}
			if negN > 0 {
				b.WriteString(fmt.Sprintf("Miss \xE2\x86\x92 avg %+.1f pips (N=%d)", negAvg, negN))
			}
			b.WriteString("</i>\n")

			// Asymmetry ratio — only show when both sides have meaningful magnitude.
			if posN > 0 && negN > 0 && math.Abs(posAvg) >= 1.0 && math.Abs(negAvg) >= 1.0 {
				ratio := math.Abs(negAvg) / math.Abs(posAvg)
				if ratio > 1.3 {
					b.WriteString(fmt.Sprintf("\xE2\x9A\xA1 <i>Asymmetric: miss moves %.1fx stronger than beat</i>\n", ratio))
				} else if ratio < 0.7 {
					b.WriteString(fmt.Sprintf("\xE2\x9A\xA1 <i>Asymmetric: beat moves %.1fx stronger than miss</i>\n", 1/ratio))
				}
			}
		}

		b.WriteString("\n")
	}

	b.WriteString("<i>+ = currency strengthened | Surprise = Actual vs Forecast</i>")
	return b.String()
}

// impactAsymmetry computes average pips for positive-surprise vs negative-surprise buckets.
func impactAsymmetry(items []domain.EventImpactSummary) (posAvg float64, posN int, negAvg float64, negN int) {
	for _, item := range items {
		switch item.SigmaBucket {
		case ">+2\u03c3", "+1\u03c3 to +2\u03c3":
			posAvg += item.AvgPriceImpactPips * float64(item.Occurrences)
			posN += item.Occurrences
		case "<-2\u03c3", "-1\u03c3 to -2\u03c3":
			negAvg += item.AvgPriceImpactPips * float64(item.Occurrences)
			negN += item.Occurrences
		}
	}
	if posN > 0 {
		posAvg /= float64(posN)
	}
	if negN > 0 {
		negAvg /= float64(negN)
	}
	return
}

// FormatOutlookShareText generates a plain-text version of AI outlook for sharing.
// Strips HTML tags and returns clean text suitable for forwarding.
func (f *Formatter) FormatOutlookShareText(htmlContent string) string {
	// Strip HTML tags for plain text output
	text := htmlContent

	// Replace common HTML entities and tags with plain equivalents
	replacer := strings.NewReplacer(
		"<b>", "", "</b>", "",
		"<i>", "", "</i>", "",
		"<code>", "", "</code>", "",
		"<pre>", "", "</pre>", "",
		"<u>", "", "</u>", "",
		"<s>", "", "</s>", "",
		"<tg-spoiler>", "", "</tg-spoiler>", "",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
	)
	text = replacer.Replace(text)

	// Remove any remaining HTML-like tags
	for {
		start := strings.Index(text, "<")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], ">")
		if end == -1 {
			break
		}
		text = text[:start] + text[start+end+1:]
	}

	// Trim excessive whitespace
	lines := strings.Split(text, "\n")
	var cleaned []string
	blankCount := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "" {
			blankCount++
			if blankCount <= 2 {
				cleaned = append(cleaned, "")
			}
		} else {
			blankCount = 0
			cleaned = append(cleaned, trimmed)
		}
	}

	result := strings.Join(cleaned, "\n")
	result = strings.TrimSpace(result)
	result += "\n\n⚡ ARK Intelligence Terminal"

	return result
}
