package vix

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// computeHistoricalPercentile fetches the full SKEW and VIX EOD CSVs,
// builds a historical SKEW/VIX ratio series, and computes where the current
// ratio sits relative to history. This provides context for tail risk alerts.
//
// Non-fatal: if fetching fails, percentile fields remain zero.
func (vs *VolSuite) computeHistoricalPercentile(ctx context.Context, client *http.Client, vixSpot float64) {
	if vs.SKEW <= 0 || vixSpot <= 0 {
		return
	}

	// Fetch historical close series (last column of CBOE EOD CSV).
	skewSeries := fetchHistoricalCloses(ctx, client, skewEODURL)
	vixSeries := fetchHistoricalCloses(ctx, client, vixEODURL)

	// SKEW percentile (standalone)
	if len(skewSeries) > 0 {
		vals := make([]float64, len(skewSeries))
		for i, s := range skewSeries {
			vals[i] = s.close
		}
		vs.SKEWPercentile = percentileRank(vals, vs.SKEW)
	}

	// SKEW/VIX ratio percentile
	if len(skewSeries) > 0 && len(vixSeries) > 0 {
		ratioSeries := buildRatioSeries(skewSeries, vixSeries)
		if len(ratioSeries) > 0 {
			vs.SKEWVIXPercentile = percentileRank(ratioSeries, vs.SKEWVIXRatio)
		}
	}
}

// historicalClose holds a date and close value from a CBOE EOD CSV.
type historicalClose struct {
	date  string
	close float64
}

// fetchHistoricalCloses fetches a CBOE EOD CSV and returns the full historical
// close price series. Returns nil on error (non-fatal).
func fetchHistoricalCloses(ctx context.Context, client *http.Client, url string) []historicalClose {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	r := csv.NewReader(resp.Body)
	r.FieldsPerRecord = -1

	var series []historicalClose
	header := true
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if header {
			header = false
			continue
		}
		// CSV format: Date, Open, High, Low, Close, ...
		if len(row) < 5 {
			continue
		}
		date := strings.TrimSpace(row[0])
		v, parseErr := strconv.ParseFloat(strings.TrimSpace(row[4]), 64)
		if parseErr != nil || v <= 0 {
			continue
		}
		series = append(series, historicalClose{date: date, close: v})
	}
	return series
}

// buildRatioSeries aligns SKEW and VIX series by date and computes SKEW/VIX
// for each matching date.
func buildRatioSeries(skewSeries, vixSeries []historicalClose) []float64 {
	vixByDate := make(map[string]float64, len(vixSeries))
	for _, v := range vixSeries {
		vixByDate[v.date] = v.close
	}

	var ratios []float64
	for _, s := range skewSeries {
		vixVal, ok := vixByDate[s.date]
		if !ok || vixVal <= 0 {
			continue
		}
		ratio := s.close / vixVal
		if !math.IsNaN(ratio) && !math.IsInf(ratio, 0) {
			ratios = append(ratios, ratio)
		}
	}
	return ratios
}

// percentileRank computes the percentile rank (0-100) of value within the series.
// Uses interpolated percentile rank formula: (B + 0.5*E) / N * 100.
func percentileRank(series []float64, value float64) float64 {
	if len(series) == 0 {
		return 0
	}

	sorted := make([]float64, len(series))
	copy(sorted, series)
	sort.Float64s(sorted)

	below := 0
	equal := 0
	for _, v := range sorted {
		if v < value {
			below++
		} else if v == value {
			equal++
		}
	}

	n := float64(len(sorted))
	pct := (float64(below) + 0.5*float64(equal)) / n * 100
	return math.Round(pct*10) / 10
}

// TailRiskContext provides human-readable historical context for the current
// SKEW/VIX ratio. Used by the formatter to enrich the /vix output.
func (vs *VolSuite) TailRiskContext() string {
	if vs.SKEWVIXRatio <= 0 {
		return ""
	}

	switch {
	case vs.SKEWVIXPercentile >= 95:
		return "Current SKEW/VIX ratio is in the top 5% of all historical readings. " +
			"Similar extremes preceded Feb 2018 Volmageddon, Mar 2020 crash, Aug 2024 yen carry unwind."
	case vs.SKEWVIXPercentile >= 85:
		return "Current SKEW/VIX ratio is elevated (top 15%). " +
			"Market pricing significant tail risk with low realized volatility — classic complacency setup."
	case vs.SKEWVIXPercentile >= 70:
		return "SKEW/VIX ratio above average — moderate tail risk premium being priced in."
	default:
		return "SKEW/VIX ratio within normal historical range."
	}
}

// AlertLevel returns the severity string for the current tail risk state.
func (vs *VolSuite) AlertLevel() string {
	switch vs.TailRisk {
	case "EXTREME":
		return "HIGH"
	case "ELEVATED":
		return "MEDIUM"
	default:
		return ""
	}
}

// ShouldAlert returns true if the current SKEW/VIX state warrants an alert broadcast.
func (vs *VolSuite) ShouldAlert() bool {
	return vs.TailRisk == "EXTREME" || vs.TailRisk == "ELEVATED"
}

// AlertSummary returns a one-line summary suitable for alert titles.
func (vs *VolSuite) AlertSummary() string {
	switch vs.TailRisk {
	case "EXTREME":
		return "🔴 EXTREME TAIL RISK — SKEW/VIX ratio historically dangerous"
	case "ELEVATED":
		return "⚠️ ELEVATED TAIL RISK — SKEW high relative to VIX"
	default:
		return ""
	}
}

// FormatAlertDetail returns a multi-line alert description with historical context.
func (vs *VolSuite) FormatAlertDetail(vixSpot float64) string {
	if !vs.ShouldAlert() {
		return ""
	}

	pctStr := ""
	if vs.SKEWVIXPercentile > 0 {
		pctStr = fmt.Sprintf(" (%.0fth percentile)", vs.SKEWVIXPercentile)
	}

	return fmt.Sprintf(
		"SKEW: %.1f | VIX: %.2f | SKEW/VIX ratio: %.1f%s\n"+
			"SKEW level at %.0fth percentile of historical range.\n\n"+
			"%s\n\n"+
			"Signals: High SKEW = options market pricing large tail moves. "+
			"Low VIX = equity market complacent. "+
			"This divergence historically precedes sharp selloffs.",
		vs.SKEW, vixSpot, vs.SKEWVIXRatio, pctStr,
		vs.SKEWPercentile,
		vs.TailRiskContext(),
	)
}
