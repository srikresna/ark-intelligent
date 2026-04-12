package price

import (
	"math"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Volatility Cone Analysis
// ---------------------------------------------------------------------------
//
// Computes where current realized volatility sits relative to its historical
// distribution. Windows: 20d, 60d, 120d rolling realized vol (annualized).
// Bands: P5, P25, P50, P75, P95. Alert when vol > P95 or < P5.

// VolConeWindow holds the cone metrics for a single lookback window.
type VolConeWindow struct {
	Window     int     `json:"window"`      // Lookback days (20, 60, 120)
	CurrentVol float64 `json:"current_vol"` // Annualized realized vol (%)
	P5         float64 `json:"p5"`
	P25        float64 `json:"p25"`
	P50        float64 `json:"p50"`
	P75        float64 `json:"p75"`
	P95        float64 `json:"p95"`
	Percentile float64 `json:"percentile"` // Position of CurrentVol in distribution (0-100)
	ZScore     float64 `json:"z_score"`    // (CurrentVol - P50) / IQR
	IsAnomaly  bool    `json:"is_anomaly"`
	AnomalyDir string  `json:"anomaly_dir"` // "HIGH", "LOW", or ""
}

// VolCone is the top-level result of volatility cone analysis.
type VolCone struct {
	Symbol    string           `json:"symbol"`
	AsOf      time.Time        `json:"as_of"`
	Month     int              `json:"month"`
	Windows   []*VolConeWindow `json:"windows"`
	IsAnomaly bool             `json:"is_anomaly"`
	Summary   string           `json:"summary"`
}

// ComputeVolCone runs volatility cone analysis on daily price records (newest-first).
// Returns nil if insufficient data (< 150 records).
func ComputeVolCone(records []domain.DailyPrice) *VolCone {
	if len(records) < 150 {
		return nil
	}

	// Reverse to oldest-first for rolling computation.
	n := len(records)
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		closes[i] = records[n-1-i].Close
	}

	// Log returns (oldest-first).
	rets := make([]float64, n-1)
	for i := 1; i < n; i++ {
		if closes[i-1] <= 0 {
			rets[i-1] = 0
			continue
		}
		rets[i-1] = math.Log(closes[i] / closes[i-1])
	}

	cone := &VolCone{
		Symbol: records[0].Symbol,
		AsOf:   records[0].Date,
		Month:  int(records[0].Date.Month()),
	}

	for _, w := range []int{20, 60, 120} {
		cw := computeVolConeWindow(rets, w)
		if cw != nil {
			cone.Windows = append(cone.Windows, cw)
			if cw.IsAnomaly {
				cone.IsAnomaly = true
			}
		}
	}

	if len(cone.Windows) == 0 {
		return nil
	}

	cone.Summary = volConeSummary(cone)
	return cone
}

func computeVolConeWindow(rets []float64, window int) *VolConeWindow {
	if len(rets) < window+1 {
		return nil
	}

	const annFactor = 252.0
	var historical []float64

	for i := window - 1; i < len(rets); i++ {
		rv := rolledVol(rets[i-window+1:i+1], annFactor)
		if rv > 0 && !math.IsNaN(rv) && !math.IsInf(rv, 0) {
			historical = append(historical, rv)
		}
	}

	if len(historical) < 20 {
		return nil
	}

	sort.Float64s(historical)

	cw := &VolConeWindow{Window: window}
	cw.CurrentVol = historical[len(historical)-1]
	cw.P5 = percentileVal(historical, 5)
	cw.P25 = percentileVal(historical, 25)
	cw.P50 = percentileVal(historical, 50)
	cw.P75 = percentileVal(historical, 75)
	cw.P95 = percentileVal(historical, 95)
	cw.Percentile = percentileRank(historical, cw.CurrentVol)

	iqr := cw.P75 - cw.P25
	if iqr > 0 {
		cw.ZScore = roundN((cw.CurrentVol-cw.P50)/iqr, 2)
	}

	if cw.CurrentVol > cw.P95 {
		cw.IsAnomaly = true
		cw.AnomalyDir = "HIGH"
	} else if cw.CurrentVol < cw.P5 {
		cw.IsAnomaly = true
		cw.AnomalyDir = "LOW"
	}

	return cw
}

func rolledVol(rets []float64, annFactor float64) float64 {
	n := len(rets)
	if n < 2 {
		return 0
	}
	var sum, sumSq float64
	for _, r := range rets {
		sum += r
		sumSq += r * r
	}
	mean := sum / float64(n)
	variance := sumSq/float64(n) - mean*mean
	if variance <= 0 {
		return 0
	}
	return math.Sqrt(variance*annFactor) * 100
}

func percentileVal(sorted []float64, pct float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if pct <= 0 {
		return sorted[0]
	}
	if pct >= 100 {
		return sorted[len(sorted)-1]
	}
	idx := pct / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func percentileRank(sorted []float64, v float64) float64 {
	if len(sorted) == 0 {
		return 50
	}
	count := 0
	for _, s := range sorted {
		if s <= v {
			count++
		}
	}
	return roundN(float64(count)/float64(len(sorted))*100, 1)
}

func volConeSummary(c *VolCone) string {
	highCount, lowCount := 0, 0
	for _, w := range c.Windows {
		switch w.AnomalyDir {
		case "HIGH":
			highCount++
		case "LOW":
			lowCount++
		}
	}
	switch {
	case highCount >= 2:
		return "Volatility unusually high — elevated regime across multiple timeframes"
	case lowCount >= 2:
		return "Volatility unusually low — compression phase, potential breakout setup"
	case highCount == 1:
		return "Volatility elevated on short-term window — monitor for regime shift"
	case lowCount == 1:
		return "Volatility compressed on one window — early compression signal"
	}
	for _, w := range c.Windows {
		if w.Window == 20 {
			switch {
			case w.Percentile >= 70:
				return "Volatility above median — leaning elevated"
			case w.Percentile <= 30:
				return "Volatility below median — leaning compressed"
			}
		}
	}
	return "Volatility within normal historical range"
}
