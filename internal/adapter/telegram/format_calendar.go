package telegram

import (
	"fmt"
	"sort"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatCalendarDay builds a message for a single day of events.
func (f *Formatter) FormatCalendarDay(dateStr string, events []domain.NewsEvent, filter string) string {
	var b strings.Builder

	sort.Slice(events, func(i, j int) bool {
		return events[i].TimeWIB.Before(events[j].TimeWIB)
	})

	// Format title
	b.WriteString(fmt.Sprintf("📅 <b>Economic Calendar</b>\n<i>Date: %s</i>\n\n", dateStr))

	if len(events) == 0 {
		b.WriteString("No events found for this filter.")
		b.WriteString("\n\n<i>Tip: </i><code>/calendar</code> | <code>/calendar week</code>")
		return b.String()
	}

	hasEvents := false
	for _, e := range events {
		// Apply filters before writing lines
		if !matchesFilter(e, filter) {
			continue
		}
		hasEvents = true

		timeDisplay := e.Time
		if !e.TimeWIB.IsZero() {
			timeDisplay = fmtutil.UpdatedAtShort(e.TimeWIB)
		}

		b.WriteString(fmt.Sprintf("%s <b>%s - %s</b>\n", e.FormatImpactColor(), timeDisplay, e.Currency))
		b.WriteString(fmt.Sprintf("↳ <i>%s</i>\n", e.Event))

		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast, e.ImpactDirection)
			line := fmt.Sprintf("   ✅ Actual: <b>%s</b> %s (Fcast: %s | Prev: %s)", e.Actual, arrow, e.Forecast, e.Previous)
			if e.SurpriseLabel != "" {
				line += fmt.Sprintf(" — <i>%s</i>", e.SurpriseLabel)
			}
			b.WriteString(line + "\n")
			if e.OldPrevious != "" && e.OldPrevious != e.Previous {
				b.WriteString(fmt.Sprintf("   ↻ <i>Revised from %s to %s</i>\n", e.OldPrevious, e.Previous))
			}
		} else {
			line := fmt.Sprintf("   Fcast: %s | Prev: %s", e.Forecast, e.Previous)
			if e.OldPrevious != "" && e.OldPrevious != e.Previous {
				line += fmt.Sprintf(" (↻ rev from %s)", e.OldPrevious)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	if !hasEvents {
		b.WriteString("No events match the current filter.")
	}

	b.WriteString("\n<i>Tip: </i><code>/calendar</code> | <code>/calendar week</code>")
	return b.String()
}

// FormatCalendarWeek summarizes all events in a week based on the filter.
func (f *Formatter) FormatCalendarWeek(weekStart string, events []domain.NewsEvent, filter string) string {
	var b strings.Builder

	sort.Slice(events, func(i, j int) bool {
		return events[i].TimeWIB.Before(events[j].TimeWIB)
	})

	b.WriteString(fmt.Sprintf("📅 <b>Weekly Economic Calendar</b>\n<i>Week starting: %s</i>\n\n", weekStart))

	if len(events) == 0 {
		b.WriteString("No events found.")
		b.WriteString("\n\n<i>Tip: </i><code>/calendar</code> | <code>/calendar week</code>")
		return b.String()
	}

	lastDate := ""
	for _, e := range events {
		// Apply filters
		if !matchesFilter(e, filter) {
			continue
		}

		// Print date header if it changed
		if e.Date != lastDate {
			b.WriteString(fmt.Sprintf("<b>--- %s ---</b>\n", e.Date))
			lastDate = e.Date
		}

		timeDisplay := e.Time
		if !e.TimeWIB.IsZero() {
			timeDisplay = fmtutil.UpdatedAtShort(e.TimeWIB)
		}

		line := fmt.Sprintf("%s %s %s: <i>%s</i>", e.FormatImpactColor(), timeDisplay, e.Currency, e.Event)
		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast, e.ImpactDirection)
			line += fmt.Sprintf(" — ✅<b>%s</b>%s", e.Actual, arrow)
			if e.SurpriseLabel != "" {
				line += fmt.Sprintf(" <i>%s</i>", e.SurpriseLabel)
			}
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n<i>Tip: </i><code>/calendar</code> | <code>/calendar week</code>")
	return b.String()
}

// FormatCalendarMonth formats all events for a whole month, grouped by day.
func (f *Formatter) FormatCalendarMonth(monthLabel string, events []domain.NewsEvent, filter string) string {
	var b strings.Builder

	sort.Slice(events, func(i, j int) bool {
		return events[i].TimeWIB.Before(events[j].TimeWIB)
	})

	b.WriteString(fmt.Sprintf("📅 <b>Monthly Economic Calendar</b>\n<i>%s</i>\n\n", monthLabel))

	if len(events) == 0 {
		b.WriteString("No events found.")
		return b.String()
	}

	lastDate := ""
	for _, e := range events {
		if !matchesFilter(e, filter) {
			continue
		}

		if e.Date != lastDate {
			b.WriteString(fmt.Sprintf("<b>--- %s ---</b>\n", e.Date))
			lastDate = e.Date
		}

		timeDisplay := e.Time
		if !e.TimeWIB.IsZero() {
			timeDisplay = fmtutil.UpdatedAtShort(e.TimeWIB)
		}

		line := fmt.Sprintf("%s %s %s: <i>%s</i>", e.FormatImpactColor(), timeDisplay, e.Currency, e.Event)
		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast, e.ImpactDirection)
			line += fmt.Sprintf(" — ✅<b>%s</b>%s", e.Actual, arrow)
		}
		b.WriteString(line + "\n")
	}

	return b.String()
}

// matchesFilter checks if a NewsEvent passes the given filter string.
// filter values:
//   - "all"     → no filtering, show everything
//   - "high"    → only high impact
//   - "med"     → high + medium impact
//   - "cur:USD" → only events for the specified currency (e.g. "cur:USD", "cur:GBP")
func matchesFilter(e domain.NewsEvent, filter string) bool {
	switch {
	case filter == "" || filter == "all":
		return true
	case filter == "high":
		return e.Impact == "high"
	case filter == "med":
		return e.Impact == "high" || e.Impact == "medium"
	case strings.HasPrefix(filter, "cur:"):
		currency := strings.ToUpper(strings.TrimPrefix(filter, "cur:"))
		return strings.ToUpper(e.Currency) == currency
	default:
		return true
	}
}

// FormatUpcomingCatalysts formats upcoming high/medium impact events for a given currency.
// Used in /cot detail to show "Upcoming Catalysts (48h)".
func (f *Formatter) FormatUpcomingCatalysts(currency string, events []domain.NewsEvent) string {
	if len(events) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n📅 <b>Upcoming Catalysts (48h):</b>\n")

	shown := 0
	for _, e := range events {
		if shown >= 5 {
			break
		}
		if !strings.EqualFold(e.Currency, currency) {
			continue
		}
		if strings.ToLower(e.Impact) != "high" && strings.ToLower(e.Impact) != "medium" {
			continue
		}
		if e.Actual != "" {
			continue // already released
		}

		timeStr := "TBA"
		if !e.TimeWIB.IsZero() {
			timeStr = e.TimeWIB.Format("Mon 15:04 WIB")
		}

		forecastStr := ""
		if e.Forecast != "" {
			forecastStr = " (Fcast: " + e.Forecast
			if e.Previous != "" {
				forecastStr += " | Prev: " + e.Previous
			}
			forecastStr += ")"
		}

		b.WriteString(fmt.Sprintf("%s %s — <b>%s</b> %s%s\n",
			e.FormatImpactColor(), timeStr, e.Currency, e.Event, forecastStr))
		shown++
	}

	if shown == 0 {
		return ""
	}

	return b.String()
}
