package vix

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// CBOE EOD CSV URLs for additional volatility indices.
const (
	skewEODURL  = "https://cdn.cboe.com/api/global/us_indices/daily_prices/SKEW_EOD.csv"
	ovxEODURL   = "https://cdn.cboe.com/api/global/us_indices/daily_prices/OVX_EOD.csv"
	gvzEODURL   = "https://cdn.cboe.com/api/global/us_indices/daily_prices/GVZ_EOD.csv"
	rvxEODURL   = "https://cdn.cboe.com/api/global/us_indices/daily_prices/RVX_EOD.csv"
	vix9dEODURL = "https://cdn.cboe.com/api/global/us_indices/daily_prices/VIX9D_EOD.csv"
	cor3mEODURL = "https://cdn.cboe.com/api/global/us_indices/daily_prices/COR3M_EOD.csv"
)

// VolSuite holds cross-asset volatility index levels from CBOE.
type VolSuite struct {
	// Core indices
	SKEW  float64 // S&P 500 tail risk (>140 = crash warning)
	OVX   float64 // CBOE crude oil volatility
	GVZ   float64 // CBOE gold volatility
	RVX   float64 // CBOE Russell 2000 volatility
	VIX9D float64 // 9-day VIX (event-driven pricing)
	COR3M float64 // CBOE 3-month implied correlation (dispersion signal; low = high dispersion)

	// Ratios (computed from VIX spot)
	SKEWVIXRatio float64 // SKEW/VIX — >8 historically dangerous
	OVXVIXRatio  float64 // OVX/VIX — oil vol vs equity vol
	GVZVIXRatio  float64 // GVZ/VIX — gold vol vs equity vol
	RVXVIXRatio  float64 // RVX/VIX — small cap vol premium (>1.3 = risk appetite declining)
	VIX9D30Ratio float64 // VIX9D/VIX — <1 normal, >1 = near-term event priced

	// Analysis
	TailRisk    string // "NORMAL", "ELEVATED", "EXTREME"
	SKEWVIXPercentile float64 // Historical percentile of current SKEW/VIX ratio (0-100)
	SKEWPercentile    float64 // Historical percentile of current SKEW level (0-100)
	Divergences []string // detected vol divergences

	// Cross-asset vol dashboard result (percentiles + regime)
	CrossVol *CrossVolResult // nil if not computed

	Available bool
	FetchedAt time.Time
}

// FetchVolSuite fetches additional CBOE volatility indices and computes
// cross-asset ratios against VIX spot. Failures on individual indices are
// non-fatal — the suite reports whatever data it can obtain.
func FetchVolSuite(ctx context.Context, vixSpot float64) *VolSuite {
	vs := &VolSuite{FetchedAt: time.Now().UTC()}
	client := httpclient.New(httpclient.WithTimeout(15 * time.Second))

	type indexFetch struct {
		url    string
		target *float64
		name   string
	}

	fetches := []indexFetch{
		{skewEODURL, &vs.SKEW, "SKEW"},
		{ovxEODURL, &vs.OVX, "OVX"},
		{gvzEODURL, &vs.GVZ, "GVZ"},
		{rvxEODURL, &vs.RVX, "RVX"},
		{vix9dEODURL, &vs.VIX9D, "VIX9D"},
		{cor3mEODURL, &vs.COR3M, "COR3M"},
	}

	fetched := 0
	for _, f := range fetches {
		val, err := fetchSingleIndexCSV(ctx, client, f.url)
		if err != nil {
			log.Debug().Str("index", f.name).Err(err).Msg("vol suite: fetch failed (non-fatal)")
			continue
		}
		*f.target = val
		fetched++
	}

	if fetched == 0 {
		return vs // Available = false
	}
	vs.Available = true

	// Compute ratios (guard against zero VIX)
	if vixSpot > 0 {
		if vs.SKEW > 0 {
			vs.SKEWVIXRatio = vs.SKEW / vixSpot
		}
		if vs.OVX > 0 {
			vs.OVXVIXRatio = vs.OVX / vixSpot
		}
		if vs.GVZ > 0 {
			vs.GVZVIXRatio = vs.GVZ / vixSpot
		}
		if vs.RVX > 0 {
			vs.RVXVIXRatio = vs.RVX / vixSpot
		}
		if vs.VIX9D > 0 {
			vs.VIX9D30Ratio = vs.VIX9D / vixSpot
		}
	}

	// Guard against NaN/Inf
	vs.SKEWVIXRatio = sanitizeFloat(vs.SKEWVIXRatio)
	vs.OVXVIXRatio = sanitizeFloat(vs.OVXVIXRatio)
	vs.GVZVIXRatio = sanitizeFloat(vs.GVZVIXRatio)
	vs.RVXVIXRatio = sanitizeFloat(vs.RVXVIXRatio)
	vs.VIX9D30Ratio = sanitizeFloat(vs.VIX9D30Ratio)

	// Tail risk classification
	vs.classifyTailRisk(vixSpot)

	// Divergence detection
	vs.detectDivergences(vixSpot)

	// Historical percentile (uses fetched SKEW+VIX CSVs)
	vs.computeHistoricalPercentile(ctx, client, vixSpot)

	// Cross-asset vol dashboard: percentiles + regime classification
	vs.CrossVol = vs.computeCrossVolPercentiles(ctx, client, vixSpot)

	return vs
}

// sanitizeFloat returns 0 for NaN or Inf values.
func sanitizeFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// classifyTailRisk uses SKEW and VIX levels to assess tail risk.
func (vs *VolSuite) classifyTailRisk(vixSpot float64) {
	switch {
	case vs.SKEW > 140 && vixSpot > 0 && vixSpot < 15:
		vs.TailRisk = "EXTREME" // High SKEW + low VIX = complacent market with hidden tail risk
	case vs.SKEW > 130 && vixSpot > 0 && vixSpot < 18:
		vs.TailRisk = "ELEVATED"
	case vs.SKEWVIXRatio > 8.0:
		vs.TailRisk = "EXTREME" // Historically preceded major crashes
	case vs.SKEW > 140:
		vs.TailRisk = "ELEVATED"
	default:
		vs.TailRisk = "NORMAL"
	}
}

// detectDivergences identifies notable cross-asset vol divergences.
func (vs *VolSuite) detectDivergences(vixSpot float64) {
	vs.Divergences = nil

	// OVX rising relative to VIX → energy-specific risk
	if vs.OVX > 0 && vixSpot > 0 && vs.OVXVIXRatio > 3.0 {
		vs.Divergences = append(vs.Divergences,
			fmt.Sprintf("OVX/VIX ratio %.1f — oil vol elevated vs equity vol (geopolitical/supply risk)", vs.OVXVIXRatio))
	}

	// GVZ + VIX both elevated → broad risk-off
	if vs.GVZ > 20 && vixSpot > 25 {
		vs.Divergences = append(vs.Divergences,
			"GVZ + VIX both elevated — broad risk-off (safe haven + equity fear)")
	}

	// RVX/VIX ratio high → small cap underperforming
	if vs.RVXVIXRatio > 1.3 {
		vs.Divergences = append(vs.Divergences,
			fmt.Sprintf("RVX/VIX ratio %.2f — small cap vol premium elevated (risk appetite declining)", vs.RVXVIXRatio))
	}

	// VIX9D > VIX → near-term event pricing
	if vs.VIX9D30Ratio > 1.1 {
		vs.Divergences = append(vs.Divergences,
			fmt.Sprintf("VIX9D/VIX ratio %.2f — near-term event priced in (>1.0 = event imminent)", vs.VIX9D30Ratio))
	}

	// All elevated → systemic stress
	if vs.OVX > 40 && vs.GVZ > 20 && vixSpot > 25 && vs.RVX > 30 {
		vs.Divergences = append(vs.Divergences,
			"All vol indices elevated — systemic stress pattern (2020/2022 pattern)")
	}

	// COR3M: low implied correlation = high dispersion (stock-picking environment)
	// High COR3M + low VIX = macro-driven melt-up with hidden correlation risk
	if vs.COR3M > 0 {
		switch {
		case vs.COR3M < 20:
			vs.Divergences = append(vs.Divergences,
				fmt.Sprintf("COR3M %.0f — high dispersion (low correlation; stock-picking environment)", vs.COR3M))
		case vs.COR3M > 70 && vixSpot < 20:
			vs.Divergences = append(vs.Divergences,
				fmt.Sprintf("COR3M %.0f high + low VIX — macro correlation spike with low fear (watch for correlation unwind)", vs.COR3M))
		case vs.COR3M > 60 && vixSpot > 25:
			vs.Divergences = append(vs.Divergences,
				fmt.Sprintf("COR3M %.0f + VIX elevated — broad correlation regime (stocks moving together in risk-off)", vs.COR3M))
		}
	}
}
