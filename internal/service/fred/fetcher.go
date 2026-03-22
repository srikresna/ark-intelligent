// Package fred provides integration with the FRED (Federal Reserve Economic Data) API.
// FRED is operated by the St. Louis Fed and provides free access to thousands of
// macroeconomic data series via a public REST API.
//
// Free API key available at: https://fred.stlouisfed.org/docs/api/api_key.html
// Set FRED_API_KEY environment variable. Without a key, the API still works for
// basic requests but may be rate-limited.
package fred

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("fred")

// SeriesTrend holds a time-series value with trend direction.
type SeriesTrend struct {
	Latest    float64
	Previous  float64
	Delta     float64
	Direction string // "UP", "DOWN", "FLAT"
}

// trendArrow returns a display arrow for a trend direction.
func (t SeriesTrend) Arrow() string {
	switch t.Direction {
	case "UP":
		return "↑"
	case "DOWN":
		return "↓"
	default:
		return "→"
	}
}

// computeTrend calculates trend direction given two values and a threshold.
func computeTrend(latest, previous, threshold float64) SeriesTrend {
	delta := latest - previous
	direction := "FLAT"
	if math.Abs(delta) >= threshold {
		if delta > 0 {
			direction = "UP"
		} else {
			direction = "DOWN"
		}
	}
	return SeriesTrend{Latest: latest, Previous: previous, Delta: delta, Direction: direction}
}

// MacroData holds the latest values for all tracked FRED series.
type MacroData struct {
	// Yield curve
	Yield2Y         float64     // DGS2  — 2-Year Treasury Constant Maturity Rate
	Yield5Y         float64     // DGS5  — 5-Year Treasury Constant Maturity Rate
	Yield10Y        float64     // DGS10 — 10-Year Treasury Constant Maturity Rate
	Yield30Y        float64     // DGS30 — 30-Year Treasury Constant Maturity Rate
	Yield3M         float64     // DGS3MO — 3-Month Treasury (for 3M-10Y spread)
	YieldSpread     float64     // DGS10 - DGS2 (positive = normal, negative = inverted)
	Spread3M10Y     float64     // DGS10 - DGS3MO (better recession predictor)
	Spread2Y30Y     float64     // DGS30 - DGS2 (long-end term premium)
	YieldSpreadTrend SeriesTrend // trend: is spread steepening or flattening?

	// Inflation
	Breakeven5Y  float64     // T10YIE — 10-Year Breakeven Inflation Rate
	CorePCE      float64     // PCEPILFE — Core PCE Price Index (YoY %)
	CPI          float64     // CPIAUCSL — Consumer Price Index (YoY %)
	CorePCETrend SeriesTrend // trend: inflation rising or falling?
	CPITrend     SeriesTrend

	// Financial stress & liquidity
	NFCI      float64     // NFCI — National Financial Conditions Index (negative = loose)
	TedSpread float64     // TEDRATE — TED Spread (credit risk proxy, bps)
	NFCITrend SeriesTrend

	// Short-term rates & liquidity
	SOFR float64 // SOFR — Secured Overnight Financing Rate (%)
	IORB float64 // IORB — Interest on Reserve Balances (Fed's true policy floor)

	// Labor market
	InitialClaims float64     // ICSA — Initial Jobless Claims (raw units)
	UnemployRate  float64     // UNRATE — Civilian Unemployment Rate (%)
	ClaimsTrend   SeriesTrend // trend: claims rising or falling?

	// Monetary policy
	FedFundsRate float64 // FEDFUNDS — Effective Federal Funds Rate (%)
	M2Growth     float64 // M2SL — computed YoY growth % (NOT level)
	M2GrowthTrend SeriesTrend

	// Growth
	GDPGrowth float64 // A191RL1Q225SBEA — Real GDP Growth Rate (QoQ annualized %)

	// Recession indicators
	SahmRule float64 // SAHMCURRENT — Sahm Rule Recession Indicator (>0.5 = recession)

	// Fed balance sheet
	FedBalSheet      float64     // WALCL — Fed Total Assets (billions USD)
	FedBalSheetTrend SeriesTrend // trend: QE (expanding) or QT (contracting)

	// USD strength
	DXY float64 // DTWEXBGS — Nominal Broad U.S. Dollar Index

	// Sentiment surveys (populated separately via sentiment package)
	CNNFearGreed float64 // 0-100 (0=Extreme Fear, 100=Extreme Greed)
	AAIIBullBear float64 // Bull/Bear ratio (>1 = bullish sentiment)

	FetchedAt time.Time
}

// fredResponse is the JSON structure returned by the FRED observations endpoint.
type fredResponse struct {
	Observations []struct {
		Date  string `json:"date"`
		Value string `json:"value"`
	} `json:"observations"`
}

// parsedObs holds parsed non-missing FRED observations in descending order.
type parsedObs []float64

// FetchMacroData fetches the latest values for all tracked series from FRED.
// It makes one HTTP request per series and is resilient to individual failures.
// If FRED_API_KEY is not set, it uses an empty string (some endpoints work without a key).
func FetchMacroData(ctx context.Context) (*MacroData, error) {
	apiKey := os.Getenv("FRED_API_KEY")

	data := &MacroData{FetchedAt: time.Now()}

	client := &http.Client{Timeout: 15 * time.Second}

	// --- Single-value series (latest point only) ---
	type singleTarget struct {
		id     string
		limit  int
		target *float64
	}

	singles := []singleTarget{
		// Yield curve
		{"DGS2", 5, &data.Yield2Y},
		{"DGS5", 5, &data.Yield5Y},
		{"DGS10", 5, &data.Yield10Y},
		{"DGS30", 5, &data.Yield30Y},
		{"DGS3MO", 5, &data.Yield3M},
		// Inflation
		{"T10YIE", 5, &data.Breakeven5Y},
		// Financial stress
		// Financial stress — TED Spread was discontinued Jan 2023.
		// Use ICE BofA HY Corporate OAS spread as credit stress proxy instead.
		{"BAMLH0A0HYM2", 5, &data.TedSpread},
		// Short-term rates
		{"SOFR", 5, &data.SOFR},
		{"IORB", 5, &data.IORB},
		// Labor
		{"UNRATE", 5, &data.UnemployRate},
		// Monetary policy
		{"FEDFUNDS", 5, &data.FedFundsRate},
		// Growth
		{"A191RL1Q225SBEA", 5, &data.GDPGrowth},
		// Recession
		{"SAHMCURRENT", 5, &data.SahmRule},
		// USD
		{"DTWEXBGS", 5, &data.DXY},
	}

	for _, t := range singles {
		obs := fetchSeries(ctx, client, t.id, apiKey, t.limit)
		if len(obs) > 0 {
			*t.target = obs[0]
		}
	}

	// --- Trend series (need latest + previous) ---

	// Core PCE — PCEPILFE is a raw price index; compute YoY% manually
	if obs := fetchSeries(ctx, client, "PCEPILFE", apiKey, 14); len(obs) >= 13 {
		if obs[12] != 0 {
			data.CorePCE = (obs[0] - obs[12]) / obs[12] * 100
		}
		data.CorePCETrend = computeTrend(obs[0], obs[1], 0.05)
	} else if len(obs) >= 1 {
		// Fallback: not enough history for YoY, use raw (will be labeled as index)
		data.CorePCE = obs[0]
	}

	// CPI — CPIAUCSL is a raw price index; compute YoY% manually
	if obs := fetchSeries(ctx, client, "CPIAUCSL", apiKey, 14); len(obs) >= 13 {
		if obs[12] != 0 {
			data.CPI = (obs[0] - obs[12]) / obs[12] * 100
		}
		data.CPITrend = computeTrend(obs[0], obs[1], 0.05)
	} else if len(obs) >= 1 {
		data.CPI = obs[0]
	}

	// NFCI — weekly
	if obs := fetchSeries(ctx, client, "NFCI", apiKey, 3); len(obs) >= 1 {
		data.NFCI = obs[0]
		if len(obs) >= 2 {
			data.NFCITrend = computeTrend(obs[0], obs[1], 0.02)
		}
	}

	// Initial Claims — weekly
	if obs := fetchSeries(ctx, client, "ICSA", apiKey, 3); len(obs) >= 1 {
		data.InitialClaims = obs[0]
		if len(obs) >= 2 {
			data.ClaimsTrend = computeTrend(obs[0], obs[1], 5_000)
		}
	}

	// M2 YoY growth — fetch 14 monthly observations, compute YoY%
	if obs := fetchSeries(ctx, client, "M2SL", apiKey, 14); len(obs) >= 2 {
		latest := obs[0]
		var yoyBase float64
		if len(obs) >= 13 {
			yoyBase = obs[12] // ~12 months ago
		} else {
			yoyBase = obs[len(obs)-1]
		}
		if yoyBase != 0 {
			data.M2Growth = (latest - yoyBase) / yoyBase * 100
		}
		// M2 trend: compare latest month to previous month (annualized MoM direction)
		data.M2GrowthTrend = computeTrend(obs[0], obs[1], 50) // $50B threshold for M2 level
	}

	// Fed Balance Sheet — weekly, limit=3 for trend
	if obs := fetchSeries(ctx, client, "WALCL", apiKey, 3); len(obs) >= 1 {
		data.FedBalSheet = obs[0]
		if len(obs) >= 2 {
			data.FedBalSheetTrend = computeTrend(obs[0], obs[1], 50) // $50B threshold
		}
	}

	// --- Derived metrics ---
	data.YieldSpread = data.Yield10Y - data.Yield2Y
	if data.Yield3M > 0 && data.Yield10Y > 0 {
		data.Spread3M10Y = data.Yield10Y - data.Yield3M
	}
	if data.Yield2Y > 0 && data.Yield30Y > 0 {
		data.Spread2Y30Y = data.Yield30Y - data.Yield2Y
	}
	// Yield spread trend (steepening vs flattening)
	if data.YieldSpread != 0 && data.YieldSpreadTrend.Latest == 0 {
		// We'll compute this after we have prev spread — use current as latest only
		data.YieldSpreadTrend = SeriesTrend{Latest: data.YieldSpread, Direction: "FLAT"}
	}

	// Sanitize: replace any NaN/Inf with 0 to prevent propagation through
	// regime classification, conviction scoring, and AI prompts.
	sanitizeFloat(&data.Yield2Y)
	sanitizeFloat(&data.Yield5Y)
	sanitizeFloat(&data.Yield10Y)
	sanitizeFloat(&data.Yield30Y)
	sanitizeFloat(&data.Yield3M)
	sanitizeFloat(&data.YieldSpread)
	sanitizeFloat(&data.Spread3M10Y)
	sanitizeFloat(&data.Spread2Y30Y)
	sanitizeFloat(&data.CorePCE)
	sanitizeFloat(&data.CPI)
	sanitizeFloat(&data.Breakeven5Y)
	sanitizeFloat(&data.FedFundsRate)
	sanitizeFloat(&data.SOFR)
	sanitizeFloat(&data.IORB)
	sanitizeFloat(&data.NFCI)
	sanitizeFloat(&data.InitialClaims)
	sanitizeFloat(&data.UnemployRate)
	sanitizeFloat(&data.SahmRule)
	sanitizeFloat(&data.GDPGrowth)
	sanitizeFloat(&data.M2Growth)
	sanitizeFloat(&data.FedBalSheet)
	sanitizeFloat(&data.DXY)
	sanitizeFloat(&data.TedSpread)

	return data, nil
}

// fetchSeries fetches up to `limit` non-missing observations for a FRED series.
// Returns values in descending chronological order (obs[0] = most recent).
func fetchSeries(ctx context.Context, client *http.Client, seriesID, apiKey string, limit int) parsedObs {
	url := buildFREDURL(seriesID, apiKey, limit)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Error().Str("series", seriesID).Err(err).Msg("failed to build request")
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Str("series", seriesID).Err(err).Msg("request failed")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error().Str("series", seriesID).Int("status", resp.StatusCode).Msg("FRED API non-2xx response")
		return nil
	}

	var result fredResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Error().Str("series", seriesID).Err(err).Msg("decode failed")
		return nil
	}

	var values []float64
	for _, obs := range result.Observations {
		if obs.Value == "." || obs.Value == "" {
			continue
		}
		v, parseErr := strconv.ParseFloat(obs.Value, 64)
		if parseErr != nil {
			continue
		}
		values = append(values, v)
	}
	return values
}

// buildFREDURL constructs the FRED API observations URL for a series.
func buildFREDURL(seriesID, apiKey string, limit int) string {
	base := fmt.Sprintf(
		"https://api.stlouisfed.org/fred/series/observations?series_id=%s&file_type=json&limit=%d&sort_order=desc",
		seriesID,
		limit,
	)
	if apiKey != "" {
		base += "&api_key=" + apiKey
	}
	return base
}

// sanitizeFloat replaces NaN or Inf with 0 to prevent propagation.
func sanitizeFloat(v *float64) {
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		*v = 0
	}
}
