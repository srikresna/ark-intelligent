package vix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// MOVEData holds ICE BofA MOVE Index data — bond market volatility equivalent of VIX.
// Fetched from Yahoo Finance ticker ^MOVE.
type MOVEData struct {
	// Current MOVE index level
	Level float64
	// Previous close
	PreviousClose float64
	// Daily change percent
	DailyChangePct float64
	// VIX/MOVE ratio (equity vol vs bond vol)
	// Normal range: 0.15-0.30
	// High VIX + low MOVE = equity-specific fear
	// Low VIX + high MOVE = bond stress / FX carry risk
	VIXMOVERatio float64
	// Divergence signal
	Divergence string // "EQUITY_FEAR", "BOND_STRESS", "ALIGNED", "UNKNOWN"
	// Whether data was successfully fetched
	Available bool
	// Fetch timestamp
	AsOf time.Time
}

// yahooChartResponse is the minimal Yahoo Finance chart API response structure.
type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
			} `json:"meta"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// FetchMOVE fetches the MOVE index from Yahoo Finance and computes VIX/MOVE divergence.
// vixSpot should be the current VIX level (pass 0 if unavailable).
func FetchMOVE(ctx context.Context, vixSpot float64) (*MOVEData, error) {
	md := &MOVEData{AsOf: time.Now().UTC()}
	client := httpclient.New(httpclient.WithTimeout(10 * time.Second))

	url := "https://query2.finance.yahoo.com/v8/finance/chart/%5EMOVE?interval=1d&range=5d"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return md, fmt.Errorf("move request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return md, fmt.Errorf("move fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return md, fmt.Errorf("move yahoo status %d", resp.StatusCode)
	}

	var chart yahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&chart); err != nil {
		return md, fmt.Errorf("move decode: %w", err)
	}

	if chart.Chart.Error != nil {
		return md, fmt.Errorf("move yahoo error: %s", chart.Chart.Error.Description)
	}

	if len(chart.Chart.Result) == 0 {
		return md, fmt.Errorf("move: no chart results")
	}

	meta := chart.Chart.Result[0].Meta
	if meta.RegularMarketPrice <= 0 {
		return md, fmt.Errorf("move: invalid price %.2f", meta.RegularMarketPrice)
	}

	md.Level = meta.RegularMarketPrice
	md.PreviousClose = meta.PreviousClose
	if md.PreviousClose > 0 {
		md.DailyChangePct = (md.Level - md.PreviousClose) / md.PreviousClose * 100
	}

	// Compute VIX/MOVE ratio and divergence
	if vixSpot > 0 && md.Level > 0 {
		md.VIXMOVERatio = vixSpot / md.Level
		md.Divergence = classifyVIXMOVEDivergence(vixSpot, md.Level, md.VIXMOVERatio)
	} else {
		md.Divergence = "UNKNOWN"
	}

	md.Available = true
	return md, nil
}

// classifyVIXMOVEDivergence detects divergence between equity and bond volatility.
//
// Historical norms:
//   - VIX/MOVE ratio normal range: 0.15-0.30
//   - High VIX + low MOVE → equity-specific event (earnings, tariffs)
//   - Low VIX + high MOVE → bond stress, FX carry unwind risk
//   - Both elevated → broad systemic stress
func classifyVIXMOVEDivergence(vix, move, ratio float64) string {
	switch {
	case ratio > 0.35 && vix > 20:
		// VIX elevated relative to MOVE — equity-specific fear
		return "EQUITY_FEAR"
	case ratio < 0.12 && move > 120:
		// MOVE elevated relative to VIX — bond stress / FX carry risk
		return "BOND_STRESS"
	case vix > 25 && move > 130:
		// Both elevated — systemic stress
		return "SYSTEMIC_STRESS"
	default:
		return "ALIGNED"
	}
}
