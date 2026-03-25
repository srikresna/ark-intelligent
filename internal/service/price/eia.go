package price

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

// ---------------------------------------------------------------------------
// EIA API Integration — Energy Information Administration
// Free API: https://api.eia.gov/v2/
// Provides petroleum inventory, refinery utilization, and production data.
// ---------------------------------------------------------------------------

var eiaLog = logger.Component("eia")

// EIAClient fetches energy data from the EIA API v2.
type EIAClient struct {
	apiKey string
	client *http.Client
}

// NewEIAClient creates an EIA API client.
func NewEIAClient(apiKey string) *EIAClient {
	return &EIAClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// EIASeasonalData holds aggregated EIA data for seasonal context.
type EIASeasonalData struct {
	// Weekly crude oil inventories (excluding SPR), million barrels
	CrudeInventory []EIAWeeklyObs
	// Weekly gasoline inventories, million barrels
	GasolineInventory []EIAWeeklyObs
	// Weekly distillate inventories, million barrels
	DistillateInventory []EIAWeeklyObs
	// Weekly refinery utilization (percent)
	RefineryUtilization []EIAWeeklyObs
	// Weekly crude production (thousand barrels/day)
	CrudeProduction []EIAWeeklyObs

	FetchedAt time.Time
}

// EIAWeeklyObs represents a single weekly observation.
type EIAWeeklyObs struct {
	Date  time.Time
	Value float64
}

// eiaResponse represents the EIA API v2 JSON response structure.
type eiaResponse struct {
	Response struct {
		Data []eiaDataPoint `json:"data"`
	} `json:"response"`
}

type eiaDataPoint struct {
	Period string  `json:"period"`
	Value  float64 `json:"value"`
}

// FetchSeasonalData fetches 5 years of key energy series for seasonal analysis.
func (c *EIAClient) FetchSeasonalData(ctx context.Context) (*EIASeasonalData, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("EIA API key not configured")
	}

	data := &EIASeasonalData{FetchedAt: time.Now()}

	// Define series to fetch
	type seriesSpec struct {
		seriesID string
		target   *[]EIAWeeklyObs
		desc     string
	}

	// EIA v2 API series IDs for weekly petroleum data
	series := []seriesSpec{
		{"PET.WCESTUS1.W", &data.CrudeInventory, "crude inventory"},
		{"PET.WGTSTUS1.W", &data.GasolineInventory, "gasoline inventory"},
		{"PET.WDISTUS1.W", &data.DistillateInventory, "distillate inventory"},
		{"PET.WPULEUS3.W", &data.RefineryUtilization, "refinery utilization"},
		{"PET.WCRFPUS2.W", &data.CrudeProduction, "crude production"},
	}

	for _, s := range series {
		obs, err := c.fetchSeries(ctx, s.seriesID)
		if err != nil {
			eiaLog.Warn().Str("series", s.seriesID).Err(err).Msgf("EIA fetch failed for %s", s.desc)
			continue
		}
		*s.target = obs
		eiaLog.Debug().Str("series", s.seriesID).Int("records", len(obs)).Msgf("fetched %s", s.desc)
	}

	if len(data.CrudeInventory) == 0 && len(data.GasolineInventory) == 0 {
		return nil, fmt.Errorf("failed to fetch any EIA data")
	}

	eiaLog.Info().
		Int("crude", len(data.CrudeInventory)).
		Int("gasoline", len(data.GasolineInventory)).
		Int("distillate", len(data.DistillateInventory)).
		Msg("EIA seasonal data fetched")

	return data, nil
}

// fetchSeries fetches a single EIA series (up to 260 weekly observations).
func (c *EIAClient) fetchSeries(ctx context.Context, seriesID string) ([]EIAWeeklyObs, error) {
	// EIA v2 API endpoint
	startDate := time.Now().AddDate(-5, 0, 0).Format("2006-01-02")
	url := fmt.Sprintf(
		"https://api.eia.gov/v2/seriesid/%s?api_key=%s&start=%s&frequency=weekly&sort[0][column]=period&sort[0][direction]=desc&length=260",
		seriesID, c.apiKey, startDate,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EIA API returned status %d", resp.StatusCode)
	}

	var result eiaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode EIA response: %w", err)
	}

	var obs []EIAWeeklyObs
	for _, dp := range result.Response.Data {
		t, err := time.Parse("2006-01-02", dp.Period)
		if err != nil {
			// Try alternate format
			t, err = time.Parse("2006-01-02T15:04:05", dp.Period)
			if err != nil {
				continue
			}
		}
		obs = append(obs, EIAWeeklyObs{Date: t, Value: dp.Value})
	}

	return obs, nil
}

// ComputeEIAContext computes energy-specific seasonal context for a given month.
func ComputeEIAContext(eiaData *EIASeasonalData, currency string, month int) *EIAContext {
	if eiaData == nil || month < 1 || month > 12 {
		return nil
	}

	// Only applies to energy assets
	switch currency {
	case "OIL", "RBOB", "ULSD":
		// continue
	default:
		return nil
	}

	result := &EIAContext{}

	// Select relevant inventory series
	var inventoryObs []EIAWeeklyObs
	switch currency {
	case "OIL":
		inventoryObs = eiaData.CrudeInventory
	case "RBOB":
		inventoryObs = eiaData.GasolineInventory
	case "ULSD":
		inventoryObs = eiaData.DistillateInventory
	}

	// Compute average weekly change for this month
	if len(inventoryObs) > 1 {
		type weeklyChange struct {
			month  int
			change float64
		}
		var changes []weeklyChange
		for i := 1; i < len(inventoryObs); i++ {
			// Use the newer observation's month (i-1 is newer in descending order)
			m := int(inventoryObs[i-1].Date.Month())
			// Obs are descending (newer first), so inventoryObs[i-1] is newer than inventoryObs[i]
			// Weekly change = newer value - older value
			change := inventoryObs[i-1].Value - inventoryObs[i].Value
			changes = append(changes, weeklyChange{month: m, change: change})
		}

		// Filter for target month
		sum := 0.0
		count := 0
		for _, wc := range changes {
			if wc.month == month {
				sum += wc.change
				count++
			}
		}
		if count > 0 {
			result.AvgWeeklyChange = sum / float64(count)
			if result.AvgWeeklyChange > 0.5 {
				result.InventoryTrend = "BUILD"
			} else if result.AvgWeeklyChange < -0.5 {
				result.InventoryTrend = "DRAW"
			} else {
				result.InventoryTrend = "FLAT"
			}
		}
	}

	// Refinery utilization for this month
	if len(eiaData.RefineryUtilization) > 0 {
		sum := 0.0
		count := 0
		for _, obs := range eiaData.RefineryUtilization {
			if int(obs.Date.Month()) == month {
				sum += obs.Value
				count++
			}
		}
		if count > 0 {
			result.RefineryUtil = sum / float64(count)
		}
	}

	// 5-year average comparison: compute avg inventory level for this month across all years
	// vs the most recent year's level for this month
	if len(inventoryObs) > 0 {
		now := time.Now()
		currentYear := now.Year()

		recentLevels := 0.0
		recentCount := 0
		historicalLevels := 0.0
		historicalCount := 0

		for _, obs := range inventoryObs {
			if int(obs.Date.Month()) == month {
				if obs.Date.Year() == currentYear || obs.Date.Year() == currentYear-1 {
					recentLevels += obs.Value
					recentCount++
				} else {
					historicalLevels += obs.Value
					historicalCount++
				}
			}
		}

		if recentCount > 0 && historicalCount > 0 {
			recentAvg := recentLevels / float64(recentCount)
			histAvg := historicalLevels / float64(historicalCount)
			pctDiff := (recentAvg - histAvg) / histAvg * 100
			switch {
			case pctDiff > 3:
				result.CurrentVs5YrAvg = "ABOVE"
			case pctDiff < -3:
				result.CurrentVs5YrAvg = "BELOW"
			default:
				result.CurrentVs5YrAvg = "NEAR"
			}
		}
	}

	// Generate assessment
	result.Assessment = generateEIAAssessment(currency, month, result)

	return result
}

// generateEIAAssessment creates a human-readable assessment.
func generateEIAAssessment(currency string, month int, eia *EIAContext) string {
	if eia == nil || month < 1 || month > 12 {
		return ""
	}

	monthName := monthNames[month-1]
	var parts []string

	if eia.InventoryTrend == "BUILD" {
		parts = append(parts, fmt.Sprintf("%s build season in %s (avg %.1fM bbl/wk)", currency, monthName, eia.AvgWeeklyChange))
	} else if eia.InventoryTrend == "DRAW" {
		parts = append(parts, fmt.Sprintf("%s draw season in %s (avg %.1fM bbl/wk)", currency, monthName, eia.AvgWeeklyChange))
	}

	if eia.RefineryUtil > 0 {
		if eia.RefineryUtil < 88 {
			parts = append(parts, fmt.Sprintf("refinery maintenance period (%.1f%% util)", eia.RefineryUtil))
		} else if eia.RefineryUtil > 93 {
			parts = append(parts, fmt.Sprintf("peak refinery runs (%.1f%% util)", eia.RefineryUtil))
		}
	}

	if eia.CurrentVs5YrAvg == "ABOVE" {
		parts = append(parts, "current storage above 5yr avg → bearish pressure")
	} else if eia.CurrentVs5YrAvg == "BELOW" {
		parts = append(parts, "current storage below 5yr avg → bullish pressure")
	}

	if len(parts) == 0 {
		return fmt.Sprintf("Normal %s seasonal pattern for %s", currency, monthName)
	}

	return joinParts(parts)
}

func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "; " + parts[i]
	}
	return result
}
