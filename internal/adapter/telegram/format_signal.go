package telegram

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/analysis"
)

// FormatUnifiedSignal formats a single-currency UnifiedSignalV2 for Telegram.
func (f *Formatter) FormatUnifiedSignal(sig *analysis.UnifiedSignalV2) string {
	var b strings.Builder

	recEmoji := analysis.RecommendationEmoji(sig.Recommendation)
	recLabel := analysis.RecommendationLabel(sig.Recommendation)
	confLabel := analysis.ConfidenceLabel(sig.Confidence)

	b.WriteString(fmt.Sprintf("🎯 <b>UNIFIED SIGNAL — %s</b>\n", sig.Currency))
	b.WriteString(fmt.Sprintf("<code>%s | Grade: %s | Confidence: %s</code>\n\n",
		sig.AsOf, sig.Grade, confLabel))

	// Main recommendation banner
	b.WriteString(fmt.Sprintf("%s <b>%s</b>  Score: <code>%+.1f</code>\n",
		recEmoji, recLabel, sig.UnifiedScore))

	// Conflict warning
	if sig.ConflictCount > 0 {
		b.WriteString(fmt.Sprintf("⚠️ <i>%d conflict(s) detected — confidence reduced</i>\n", sig.ConflictCount))
	}
	b.WriteString("\n")

	// Component breakdown
	b.WriteString("📊 <b>Component Breakdown</b>\n")
	for _, c := range sig.Components {
		if !c.Available {
			b.WriteString(fmt.Sprintf("  %-10s <code>%s</code> <i>(no data)</i>\n",
				c.Name, "──────"))
			continue
		}
		bar := analysis.FormatComponentBar(c.NormalizedScore)
		voteIcon := voteIcon(c.Vote)
		b.WriteString(fmt.Sprintf("  %-10s %s <code>%+.0f</code> %s (w:%.0f%%)\n",
			c.Name, bar, c.RawScore, voteIcon, c.Weight*100))
	}

	// Voting matrix
	vm := sig.VotingMatrix
	b.WriteString(fmt.Sprintf("\n🗳 <b>Votes:</b> 🟢 %d long  🔴 %d short  ⚪ %d neutral\n",
		vm.LongVotes, vm.ShortVotes, vm.NeutralVotes))
	if len(vm.Dissenting) > 0 {
		b.WriteString(fmt.Sprintf("   <i>Dissenting: %s</i>\n", strings.Join(vm.Dissenting, ", ")))
	}

	// VIX dampening note
	if sig.VIXMultiplier < 1.0 {
		b.WriteString(fmt.Sprintf("\n🔇 <i>VIX dampening applied: ×%.2f</i>\n", sig.VIXMultiplier))
	}

	// Confidence bar
	confPct := int(math.Round(sig.Confidence))
	confBar := buildConfidenceBar(confPct)
	b.WriteString(fmt.Sprintf("\n🎲 <b>Confidence:</b> %s <code>%d%%</code>\n", confBar, confPct))

	return b.String()
}

// FormatUnifiedSignalOverview formats an overview table of unified signals for all currencies.
func (f *Formatter) FormatUnifiedSignalOverview(sigs []*analysis.UnifiedSignalV2) string {
	if len(sigs) == 0 {
		return "❌ No unified signal data available."
	}

	// Sort by absolute score descending (strongest signals first)
	sorted := make([]*analysis.UnifiedSignalV2, len(sigs))
	copy(sorted, sigs)
	sort.Slice(sorted, func(i, j int) bool {
		return math.Abs(sorted[i].UnifiedScore) > math.Abs(sorted[j].UnifiedScore)
	})

	var b strings.Builder
	b.WriteString("🎯 <b>UNIFIED SIGNALS OVERVIEW</b>\n")
	b.WriteString(fmt.Sprintf("<code>%s</code>\n\n", sorted[0].AsOf))

	for _, sig := range sorted {
		recEmoji := analysis.RecommendationEmoji(sig.Recommendation)
		conflictNote := ""
		if sig.ConflictCount > 0 {
			conflictNote = fmt.Sprintf(" ⚠️×%d", sig.ConflictCount)
		}
		confLabel := analysis.ConfidenceLabel(sig.Confidence)
		b.WriteString(fmt.Sprintf("%s <b>%s</b>  <code>%+.1f</code>  [%s] %s%s\n",
			recEmoji, sig.Currency, sig.UnifiedScore, sig.Grade, confLabel, conflictNote))
	}

	b.WriteString("\n<i>Use /signal [CURRENCY] for full breakdown</i>")
	return b.String()
}

// voteIcon returns an emoji for a VoteDirection.
func voteIcon(v analysis.VoteDirection) string {
	switch v {
	case analysis.VoteLong:
		return "🟢"
	case analysis.VoteShort:
		return "🔴"
	default:
		return "⚪"
	}
}

// buildConfidenceBar produces a simple 10-block progress bar for confidence %.
func buildConfidenceBar(pct int) string {
	filled := pct / 10
	if filled > 10 {
		filled = 10
	}
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < 10; i++ {
		bar += "░"
	}
	return bar
}
