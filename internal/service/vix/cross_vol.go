package vix

import (
	"context"
	"fmt"
	"net/http"
)

// CrossVolRegime classifies the overall cross-asset volatility environment.
type CrossVolRegime string

const (
	CrossVolNormal        CrossVolRegime = "NORMAL"
	CrossVolEnergyRisk    CrossVolRegime = "ENERGY_RISK"
	CrossVolBroadRiskOff  CrossVolRegime = "BROAD_RISK_OFF"
	CrossVolSmallCapStress CrossVolRegime = "SMALL_CAP_STRESS"
	CrossVolSystemic      CrossVolRegime = "SYSTEMIC"
)

// CrossVolResult holds the cross-asset volatility dashboard analysis.
type CrossVolResult struct {
	// Per-ratio percentiles (0-100, historical rank)
	OVXVIXPercentile  float64
	GVZVIXPercentile  float64
	RVXVIXPercentile  float64
	VIX9D30Percentile float64

	// Overall regime
	Regime      CrossVolRegime
	RegimeLabel string // human-readable label
}

// classifyRegime determines the cross-asset vol regime from current VolSuite data.
func classifyCrossVolRegime(vs *VolSuite, vixSpot float64) (CrossVolRegime, string) {
	if !vs.Available || vixSpot <= 0 {
		return CrossVolNormal, "Data insufficient"
	}

	// Systemic: ALL vol indices elevated simultaneously
	allElevated := vs.OVX > 35 && vs.GVZ > 18 && vs.RVX > 28 && vixSpot > 22
	if allElevated {
		return CrossVolSystemic, "Systemic stress — all vol classes elevated (watch hedges)"
	}

	// Broad risk-off: gold vol + equity vol both high (safe haven + fear)
	if vs.GVZ > 18 && vixSpot > 22 {
		return CrossVolBroadRiskOff, "Broad risk-off — gold + equity vol elevated (de-risking)"
	}

	// Energy-specific: oil vol elevated but equity vol calm
	if vs.OVX > 0 && vixSpot > 0 && vs.OVXVIXRatio > 2.5 && vixSpot < 20 {
		return CrossVolEnergyRisk, "Energy-specific risk — OVX elevated vs calm equity vol"
	}

	// Small-cap stress: RVX premium over VIX
	if vs.RVXVIXRatio > 1.25 {
		return CrossVolSmallCapStress, "Small cap stress — RVX/VIX premium elevated (risk appetite declining)"
	}

	return CrossVolNormal, "Cross-asset vol within normal ranges"
}

// computeCrossVolPercentiles fetches historical data for OVX, GVZ, RVX, VIX9D
// and computes percentile ranks for their ratios vs VIX.
// Extends the existing percentile computation to cover all ratio types.
func (vs *VolSuite) computeCrossVolPercentiles(ctx context.Context, client *http.Client, vixSpot float64) *CrossVolResult {
	result := &CrossVolResult{}

	if vixSpot <= 0 || !vs.Available {
		regime, label := classifyCrossVolRegime(vs, vixSpot)
		result.Regime = regime
		result.RegimeLabel = label
		return result
	}

	// Fetch VIX historical series (shared across all ratios)
	vixSeries := fetchHistoricalCloses(ctx, client, vixEODURL)
	if len(vixSeries) == 0 {
		regime, label := classifyCrossVolRegime(vs, vixSpot)
		result.Regime = regime
		result.RegimeLabel = label
		return result
	}

	// Compute percentile for each cross-asset ratio
	type ratioCalc struct {
		url        string
		current    float64
		target     *float64
		name       string
	}

	calcs := []ratioCalc{
		{ovxEODURL, vs.OVXVIXRatio, &result.OVXVIXPercentile, "OVX/VIX"},
		{gvzEODURL, vs.GVZVIXRatio, &result.GVZVIXPercentile, "GVZ/VIX"},
		{rvxEODURL, vs.RVXVIXRatio, &result.RVXVIXPercentile, "RVX/VIX"},
		{vix9dEODURL, vs.VIX9D30Ratio, &result.VIX9D30Percentile, "VIX9D/VIX"},
	}

	for _, c := range calcs {
		if c.current <= 0 {
			continue
		}
		indexSeries := fetchHistoricalCloses(ctx, client, c.url)
		if len(indexSeries) == 0 {
			continue
		}
		ratioSeries := buildRatioSeries(indexSeries, vixSeries)
		if len(ratioSeries) > 0 {
			*c.target = percentileRank(ratioSeries, c.current)
		}
	}

	regime, label := classifyCrossVolRegime(vs, vixSpot)
	result.Regime = regime
	result.RegimeLabel = label

	return result
}

// VolBar generates a simple text bar chart segment for Telegram <code> blocks.
// width is the max bar width in characters. pct should be 0-100.
func VolBar(pct float64, width int) string {
	if width <= 0 {
		width = 10
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < width; i++ {
		bar += "░"
	}
	return bar
}

// RegimeEmoji returns an appropriate emoji for the cross-vol regime.
func RegimeEmoji(regime CrossVolRegime) string {
	switch regime {
	case CrossVolSystemic:
		return "🔴"
	case CrossVolBroadRiskOff:
		return "🟠"
	case CrossVolEnergyRisk:
		return "🟡"
	case CrossVolSmallCapStress:
		return "🟡"
	default:
		return "🟢"
	}
}

// FormatCrossVolDashboard produces a formatted cross-asset vol dashboard section
// suitable for Telegram HTML output.
func FormatCrossVolDashboard(vs *VolSuite, vixSpot float64, cv *CrossVolResult) string {
	if vs == nil || !vs.Available {
		return ""
	}

	var b []byte

	b = append(b, []byte("\n<b>📊 Cross-Asset Vol Dashboard</b>\n")...)

	// Regime header
	emoji := RegimeEmoji(cv.Regime)
	b = append(b, []byte(fmt.Sprintf("%s <b>Regime:</b> %s\n", emoji, cv.RegimeLabel))...)
	b = append(b, '\n')

	// Visual bar chart for each ratio
	type barEntry struct {
		label   string
		value   float64
		pctile  float64
		fmtVal  string
	}

	entries := []barEntry{}

	if vs.OVXVIXRatio > 0 {
		entries = append(entries, barEntry{
			label:  "OVX/VIX",
			value:  vs.OVXVIXRatio,
			pctile: cv.OVXVIXPercentile,
			fmtVal: fmt.Sprintf("%.1f", vs.OVXVIXRatio),
		})
	}
	if vs.GVZVIXRatio > 0 {
		entries = append(entries, barEntry{
			label:  "GVZ/VIX",
			value:  vs.GVZVIXRatio,
			pctile: cv.GVZVIXPercentile,
			fmtVal: fmt.Sprintf("%.2f", vs.GVZVIXRatio),
		})
	}
	if vs.RVXVIXRatio > 0 {
		entries = append(entries, barEntry{
			label:  "RVX/VIX",
			value:  vs.RVXVIXRatio,
			pctile: cv.RVXVIXPercentile,
			fmtVal: fmt.Sprintf("%.2f", vs.RVXVIXRatio),
		})
	}
	if vs.VIX9D30Ratio > 0 {
		entries = append(entries, barEntry{
			label:  "9D/30D ",
			value:  vs.VIX9D30Ratio,
			pctile: cv.VIX9D30Percentile,
			fmtVal: fmt.Sprintf("%.2f", vs.VIX9D30Ratio),
		})
	}

	for _, e := range entries {
		bar := VolBar(e.pctile, 10)
		pctStr := ""
		if e.pctile > 0 {
			pctStr = fmt.Sprintf(" P%.0f", e.pctile)
		}
		b = append(b, []byte(fmt.Sprintf("<code>%s %s %s%s</code>\n", e.label, bar, e.fmtVal, pctStr))...)
	}

	return string(b)
}
