package telegram

// /impact — Event Impact Database

import (
	"context"
	"fmt"
	"html"
	"strings"
)

// ---------------------------------------------------------------------------
// /impact — Event Impact Database
// ---------------------------------------------------------------------------

// cmdImpact handles the /impact command.
// /impact        — lists all tracked events
// /impact <name> — shows historical price impact by sigma bucket
func (h *Handler) cmdImpact(ctx context.Context, chatID string, _ int64, args string) error {
	if h.impactProvider == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Event impact tracking is not available.")
		return err
	}

	query := strings.TrimSpace(args)

	// Resolve common abbreviations to full event names
	query = resolveEventAlias(query)

	// No arguments: show category keyboard
	if query == "" {
		kb := h.kb.ImpactCategoryMenu()
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			"📋 <b>EVENT IMPACT DATABASE</b>\n<i>Select a category to view event impacts:</i>\n\n<i>Or type directly:</i> <code>/impact NFP</code>",
			kb)
		return err
	}

	// Argument provided: look up impact summary for that event
	summaries, err := h.impactProvider.GetEventImpactSummary(ctx, query)
	if err != nil {
		log.Error().Err(err).Str("query", query).Msg("cmdImpact: get summary failed")
		_, sendErr := h.bot.SendHTML(ctx, chatID, "Failed to load impact data.")
		return sendErr
	}

	// If no results, try substring matching against tracked events
	if len(summaries) == 0 {
		events, listErr := h.impactProvider.GetTrackedEvents(ctx)
		if listErr == nil {
			matched := fuzzyMatchEvent(query, events)
			if matched != "" {
				summaries, err = h.impactProvider.GetEventImpactSummary(ctx, matched)
				if err != nil {
					log.Error().Err(err).Str("query", matched).Msg("cmdImpact: fuzzy match summary failed")
					_, sendErr := h.bot.SendHTML(ctx, chatID, "Failed to load impact data.")
					return sendErr
				}
				query = matched
			}
		}
	}

	if len(summaries) == 0 {
		kb := h.kb.ImpactCategoryMenu()
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			fmt.Sprintf("❌ Event <b>%s</b> not found in impact database.\n\n<i>Select from categories below or use known aliases:</i>\n<code>NFP, CPI, FOMC, BOE, GDP, PMI...</code>", html.EscapeString(query)),
			kb)
		return err
	}

	htmlOut := h.fmt.FormatEventImpact(query, summaries)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// eventAliases maps common abbreviations to full event names.
var eventAliases = map[string]string{
	"NFP":         "Non-Farm Employment Change",
	"NONFARM":     "Non-Farm Employment Change",
	"CPI":         "CPI m/m",
	"CORE CPI":    "Core CPI m/m",
	"PPI":         "PPI m/m",
	"FOMC":        "Federal Funds Rate",
	"FED":         "Federal Funds Rate",
	"BOE":         "Official Bank Rate",
	"ECB":         "Main Refinancing Rate",
	"BOJ":         "BOJ Policy Rate",
	"RBA":         "Cash Rate",
	"BOC":         "Overnight Rate",
	"RBNZ":        "Official Cash Rate",
	"SNB":         "SNB Policy Rate",
	"GDP":         "GDP q/q",
	"PMI":         "ISM Manufacturing PMI",
	"RETAIL":      "Core Retail Sales m/m",
	"CLAIMS":      "Unemployment Claims",
	"JOBLESS":     "Unemployment Claims",
	"PCE":         "Core PCE Price Index m/m",
	"ISM":         "ISM Manufacturing PMI",
	"ADP":         "ADP Non-Farm Employment Change",
	"WAGES":       "Average Hourly Earnings m/m",
	"CORE_CPI":    "Core CPI m/m",
	"CB_CONSUMER": "CB Consumer Confidence Index",
	"PRICE_EXP":   "Consumer Price Expectations",
	"HOME_SALES":  "Existing Home Sales",
	"PERMITS":     "Building Permits",
}

// resolveEventAlias resolves a known abbreviation to its full event name.
// Returns the input unchanged if no alias matches.
func resolveEventAlias(query string) string {
	upper := strings.ToUpper(strings.TrimSpace(query))
	if full, ok := eventAliases[upper]; ok {
		return full
	}
	return query
}

// fuzzyMatchEvent finds the first tracked event whose name contains the query (case-insensitive).
// Returns the matched event name, or empty string if no match found.
func fuzzyMatchEvent(query string, events []string) string {
	lower := strings.ToLower(query)
	for _, ev := range events {
		if strings.Contains(strings.ToLower(ev), lower) {
			return ev
		}
	}
	return ""
}

// cbImpact handles "imp:" prefixed callbacks for event impact navigation.
func (h *Handler) cbImpact(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	if h.impactProvider == nil {
		return nil
	}

	action := strings.TrimPrefix(data, "imp:")

	switch {
	case strings.HasPrefix(action, "cat:"):
		// Show events in category
		category := strings.TrimPrefix(action, "cat:")
		kb := h.kb.ImpactEventMenu(category)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID,
			"📋 <b>EVENT IMPACT DATABASE</b>\n<i>Select an event:</i>",
			kb)

	case action == "back":
		// Back to category menu
		kb := h.kb.ImpactCategoryMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID,
			"📋 <b>EVENT IMPACT DATABASE</b>\n<i>Select a category to view event impacts:</i>\n\n<i>Or type directly:</i> <code>/impact NFP</code>",
			kb)

	case strings.HasPrefix(action, "ev:"):
		// Show impact for specific event
		alias := strings.TrimPrefix(action, "ev:")
		// Resolve alias (need to handle underscores -> spaces for multi-word aliases)
		query := strings.ReplaceAll(alias, "_", " ")
		query = resolveEventAlias(query)

		summaries, err := h.impactProvider.GetEventImpactSummary(ctx, query)
		if err != nil {
			return err
		}

		impactHTML := h.fmt.FormatEventImpact(query, summaries)
		// Edit existing message with impact data + back button
		kb := h.kb.ImpactBackMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, impactHTML, kb)
	}

	return nil
}
