package scheduler

// scheduler_pair_alerts.go — Per-pair COT alert logic (TASK-052).
//
// When new COT data arrives, check each user's PairAlerts configuration
// and send targeted alerts for currencies that match their criteria
// (conviction delta threshold, bias flip detection).

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// checkPairAlerts evaluates per-pair alerts for all active users.
// It compares current COT analyses with previous analyses to detect
// conviction changes and bias flips that match user criteria.
//
// currAnalyses: current week's analyses (just computed).
// prevAnalyses: previous week's analyses (snapshot taken before fetch).
// activeUsers: map of user ID → prefs for all active users.
func (s *Scheduler) checkPairAlerts(
	ctx context.Context,
	currAnalyses []domain.COTAnalysis,
	prevAnalyses []domain.COTAnalysis,
	activeUsers map[int64]domain.UserPrefs,
) {
	if len(currAnalyses) == 0 {
		return
	}

	// Build analysis maps by currency
	currMap := make(map[string]domain.COTAnalysis, len(currAnalyses))
	for _, a := range currAnalyses {
		currMap[a.Contract.Currency] = a
	}

	prevMap := make(map[string]domain.COTAnalysis, len(prevAnalyses))
	for _, a := range prevAnalyses {
		prevMap[a.Contract.Currency] = a
	}

	sentCount := 0
	for userID, prefs := range activeUsers {
		if len(prefs.PairAlerts) == 0 || prefs.ChatID == "" {
			continue
		}
		if !prefs.COTAlertsEnabled {
			continue
		}
		if s.deps.IsBanned != nil && s.deps.IsBanned(ctx, userID) {
			continue
		}

		var alerts []string
		for _, pa := range prefs.PairAlerts {
			if !pa.Enabled {
				continue
			}

			curr, hasCurr := currMap[pa.Currency]
			if !hasCurr {
				continue
			}

			prev, hasPrev := prevMap[pa.Currency]

			// Determine current and previous bias
			currBias := classifyBias(curr.ShortTermBias)
			prevBias := ""
			if hasPrev {
				prevBias = classifyBias(prev.ShortTermBias)
			}

			// Check bias flip
			if pa.BiasFlip {
				if hasPrev && prevBias != "" && currBias != prevBias &&
					currBias != "NEUTRAL" && prevBias != "NEUTRAL" {
					alerts = append(alerts, fmt.Sprintf(
						"🔄 <b>%s BIAS FLIP</b>: %s%s → %s%s",
						pa.Currency,
						biasEmoji(prevBias), prevBias,
						biasEmoji(currBias), currBias))
				}
				continue // flip-only mode: don't check delta
			}

			// Check conviction delta
			if pa.ConvictionDelta > 0 && hasPrev {
				currScore := curr.SentimentScore
				prevScore := prev.SentimentScore
				delta := math.Abs(currScore - prevScore)

				if delta >= pa.ConvictionDelta {
					direction := "📈"
					if currScore < prevScore {
						direction = "📉"
					}
					alerts = append(alerts, fmt.Sprintf(
						"%s <b>%s</b>: Sentiment %.1f → %.1f (Δ%.1f)",
						direction, pa.Currency, prevScore, currScore, delta))
				}
			} else if pa.ConvictionDelta == 0 && !pa.BiasFlip {
				// Default mode: alert on any meaningful change
				if hasPrev {
					currScore := curr.SentimentScore
					prevScore := prev.SentimentScore
					delta := math.Abs(currScore - prevScore)

					biasChanged := currBias != prevBias &&
						currBias != "NEUTRAL" && prevBias != "NEUTRAL"

					if delta >= 0.5 || biasChanged {
						icon := "📊"
						if biasChanged {
							icon = "🔄"
						}
						detail := fmt.Sprintf("Sentiment %.1f → %.1f", prevScore, currScore)
						if biasChanged {
							detail += fmt.Sprintf(" | Bias: %s → %s", prevBias, currBias)
						}
						alerts = append(alerts, fmt.Sprintf(
							"%s <b>%s</b>: %s", icon, pa.Currency, detail))
					}
				} else {
					// No previous data — just show current state
					alerts = append(alerts, fmt.Sprintf(
						"📊 <b>%s</b>: Bias %s%s (Sentiment: %.1f)",
						pa.Currency, biasEmoji(currBias), currBias, curr.SentimentScore))
				}
			}
		}

		if len(alerts) > 0 {
			html := "🔔 <b>PER-PAIR COT ALERT</b>\n\n"
			html += strings.Join(alerts, "\n")
			html += "\n\n<i>Kelola alert: /setalert list</i>"

			kb := ports.InlineKeyboard{Rows: [][]ports.InlineButton{
				{
					{Text: "📊 Lihat COT", CallbackData: "cmd:cot"},
					{Text: "⚙️ Kelola Alert", CallbackData: "setalert:add"},
				},
			}}

			if _, err := s.deps.Bot.SendWithKeyboard(ctx, prefs.ChatID, html, kb); err == nil {
				sentCount++
			}
			time.Sleep(config.TelegramFloodDelay)
		}
	}

	if sentCount > 0 {
		log.Info().Int("users", sentCount).Msg("sent per-pair COT alerts")
	}
}

// classifyBias normalizes various bias strings into "BULLISH", "BEARISH", or "NEUTRAL".
func classifyBias(bias string) string {
	upper := strings.ToUpper(bias)
	switch {
	case strings.Contains(upper, "BUY") || strings.Contains(upper, "BULL") || strings.Contains(upper, "LONG"):
		return "BULLISH"
	case strings.Contains(upper, "SELL") || strings.Contains(upper, "BEAR") || strings.Contains(upper, "SHORT"):
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

// biasEmoji returns an emoji prefix for the bias direction.
func biasEmoji(bias string) string {
	switch bias {
	case "BULLISH":
		return "🟢 "
	case "BEARISH":
		return "🔴 "
	default:
		return "⚪ "
	}
}
