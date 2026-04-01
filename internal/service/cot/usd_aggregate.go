// Package cot — usd_aggregate.go synthesises a single USD directional
// signal by combining net positions from all major FX-pair COT analyses.
package cot

import (
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// USDAggregate is the synthesised USD positioning signal derived from
// cross-currency COT analyses.
type USDAggregate struct {
	// Score: +100 = extreme USD-bullish, −100 = extreme USD-bearish.
	Score float64 `json:"score"`
	// Direction: "BULLISH", "BEARISH", or "NEUTRAL".
	Direction string `json:"direction"`

	// Per-currency contribution (currency → normalised USD-direction score).
	Contributions map[string]float64 `json:"contributions"`

	// DX direct positioning (from DX futures COT, if available).
	DXDirectScore float64 `json:"dx_direct_score"`
	DXDirectDir   string  `json:"dx_direct_dir"`

	// Divergence flags.
	Divergence     bool   `json:"divergence"`
	DivergenceDesc string `json:"divergence_desc,omitempty"`

	// ConvictionPct: percentage of pairs that agree with the aggregate direction.
	ConvictionPct  float64 `json:"conviction_pct"`
	HighConviction bool    `json:"high_conviction"`
}

// fxCurrencies are the major FX currencies whose COT positioning
// contributes to the USD aggregate (excluding DX itself).
var fxCurrencies = map[string]bool{
	"EUR": true, "GBP": true, "JPY": true, "CHF": true,
	"AUD": true, "CAD": true, "NZD": true,
}

// ComputeUSDAggregate synthesises a USD positioning signal from all
// FX-pair COT analyses. analyses should come from AnalyzeAll(); contracts
// provides the full contract list for Inverse/Currency lookup.
func ComputeUSDAggregate(analyses []domain.COTAnalysis, contracts []domain.COTContract) USDAggregate {
	contractMap := make(map[string]domain.COTContract, len(contracts))
	for _, c := range contracts {
		contractMap[c.Code] = c
	}

	agg := USDAggregate{
		Contributions: make(map[string]float64, len(fxCurrencies)),
	}

	var sumScore float64
	var count int
	var alignedWithBull, alignedWithBear int

	for i := range analyses {
		a := &analyses[i]
		contract, ok := contractMap[a.Contract.Code]
		if !ok {
			contract = a.Contract
		}

		// --- Handle DX (US Dollar Index) separately ---
		if contract.Currency == "USD" && contract.Inverse {
			// DX: long = USD long (already in USD direction)
			dxNorm := normalisedNet(a)
			agg.DXDirectScore = dxNorm
			agg.DXDirectDir = directionLabelNorm(dxNorm)
			continue
		}

		// Only include FX majors in the aggregate.
		if !fxCurrencies[contract.Currency] {
			continue
		}

		// OI-normalised net position.
		norm := normalisedNet(a)

		// Flip sign: for non-inverse FX pairs, long EUR = short USD.
		if !contract.Inverse {
			norm = -norm
		}

		agg.Contributions[contract.Currency] = norm
		sumScore += norm
		count++

		if norm > 0 {
			alignedWithBull++
		} else if norm < 0 {
			alignedWithBear++
		}
	}

	if count == 0 {
		agg.Direction = "NEUTRAL"
		return agg
	}

	// Average to normalise (−100 to +100 range).
	agg.Score = clamp(sumScore/float64(count)*100, -100, 100)
	agg.Direction = directionLabel(agg.Score)

	// Conviction: % of pairs aligned with aggregate direction.
	if agg.Score > 0 {
		agg.ConvictionPct = float64(alignedWithBull) / float64(count) * 100
	} else if agg.Score < 0 {
		agg.ConvictionPct = float64(alignedWithBear) / float64(count) * 100
	} else {
		agg.ConvictionPct = 50
	}
	agg.HighConviction = agg.ConvictionPct >= 70

	// Detect DX vs aggregate divergence.
	detectUSDDivergence(&agg)

	return agg
}

// normalisedNet returns the OI-normalised net position for an analysis,
// ranging roughly −1 to +1 (net / OI). Falls back to raw NetPosition
// clamped to [−1, +1] if OI is missing.
func normalisedNet(a *domain.COTAnalysis) float64 {
	if a.OpenInterest > 0 {
		return a.NetPosition / a.OpenInterest
	}
	// Fallback: use PctOfOI (already 0-100) / 100.
	if a.PctOfOI != 0 {
		return a.PctOfOI / 100
	}
	// Last resort: clamp raw value.
	if a.NetPosition > 0 {
		return 1
	} else if a.NetPosition < 0 {
		return -1
	}
	return 0
}

// detectDivergence flags when DX direct positioning disagrees with
// the cross-currency aggregate.
func detectUSDDivergence(agg *USDAggregate) {
	if agg.DXDirectDir == "" || agg.DXDirectDir == "NEUTRAL" || agg.Direction == "NEUTRAL" {
		return
	}
	if agg.DXDirectDir != agg.Direction {
		agg.Divergence = true
		agg.DivergenceDesc = fmt.Sprintf(
			"DX says %s (%.1f), cross-pairs say %s (%.1f) — low conviction USD call, await resolution",
			agg.DXDirectDir, agg.DXDirectScore*100,
			agg.Direction, agg.Score)
	}
}

// directionLabel maps a signed score (−100 to +100) to a direction label.
func directionLabel(score float64) string {
	switch {
	case score > 10:
		return "BULLISH"
	case score < -10:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

// directionLabelNorm maps a normalised score (−1 to +1) to a direction label.
func directionLabelNorm(score float64) string {
	switch {
	case score > 0.05:
		return "BULLISH"
	case score < -0.05:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

// clamp limits v to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// FormatUSDAggregate returns a Telegram-ready HTML snippet for the
// USD aggregate signal. Suitable for embedding in /bias output.
func FormatUSDAggregate(agg USDAggregate) string {
	if len(agg.Contributions) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n💵 <b>USD Aggregate COT Signal</b>\n\n")

	// Per-currency contributions.
	b.WriteString("  <b>Pair Contributions:</b>\n")
	for _, cur := range []string{"EUR", "GBP", "JPY", "CHF", "AUD", "CAD", "NZD"} {
		v, ok := agg.Contributions[cur]
		if !ok {
			continue
		}
		emoji := "🔸"
		if v > 0.05 {
			emoji = "🟢" // USD-bullish via this pair
		} else if v < -0.05 {
			emoji = "🔴" // USD-bearish via this pair
		}
		desc := "short USD"
		if v > 0 {
			desc = "long USD"
		}
		b.WriteString(fmt.Sprintf("  %s %s: %+.2f (%s via %s)\n", emoji, cur, v, desc, cur))
	}

	// Aggregate score.
	b.WriteString(fmt.Sprintf("\n  <b>Aggregate Score:</b> %+.1f → <b>%s USD</b>\n", agg.Score, agg.Direction))

	// DX comparison.
	if agg.DXDirectDir != "" {
		b.WriteString(fmt.Sprintf("  <b>DX Direct:</b> %+.1f → %s USD\n", agg.DXDirectScore*100, agg.DXDirectDir))
	}

	// Divergence.
	if agg.Divergence {
		b.WriteString(fmt.Sprintf("  ⚠️ <b>DIVERGENCE:</b> %s\n", agg.DivergenceDesc))
	}

	// Conviction.
	convEmoji := "🟡"
	if agg.HighConviction {
		convEmoji = "🟢"
	}
	b.WriteString(fmt.Sprintf("  %s <b>Conviction:</b> %.0f%% pairs aligned", convEmoji, agg.ConvictionPct))
	if !agg.HighConviction {
		b.WriteString(" (below 70% threshold)")
	}
	b.WriteString("\n")

	return b.String()
}
