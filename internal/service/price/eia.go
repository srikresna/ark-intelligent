package price

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
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
		client: httpclient.New(httpclient.WithTimeout(15 * time.Second)),
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

	// Weekly natural gas storage (billion cubic feet)
	NaturalGasStorage []EIAWeeklyObs
	// Daily Henry Hub natural gas spot price (USD/MMBtu)
	HenryHubPrice []EIAWeeklyObs

	FetchedAt time.Time
}

// NaturalGasData holds processed natural gas market data.
type NaturalGasData struct {
	// Latest storage level in BCF
	LatestStorageBCF float64
	// Week-over-week storage change in BCF
	WeekOverWeekChange float64
	// Storage vs 5-year average (percent deviation)
	StorageVs5YrAvgPct float64
	// Storage trend: "INJECTION", "WITHDRAWAL", "FLAT"
	StorageTrend string
	// Henry Hub latest price USD/MMBtu
	HenryHubLatest float64
	// Henry Hub 7-day average
	HenryHub7dAvg float64
	// Henry Hub 30-day average
	HenryHub30dAvg float64
	// Season context: "INJECTION" (Apr-Oct) or "WITHDRAWAL" (Nov-Mar)
	SeasonContext string
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

	// EIA v2 API series IDs for weekly petroleum + natural gas data
	series := []seriesSpec{
		{"PET.WCESTUS1.W", &data.CrudeInventory, "crude inventory"},
		{"PET.WGTSTUS1.W", &data.GasolineInventory, "gasoline inventory"},
		{"PET.WDISTUS1.W", &data.DistillateInventory, "distillate inventory"},
		{"PET.WPULEUS3.W", &data.RefineryUtilization, "refinery utilization"},
		{"PET.WCRFPUS2.W", &data.CrudeProduction, "crude production"},
		{"NG.NW2_EPG0_SWO_R48_BCF.W", &data.NaturalGasStorage, "natural gas storage"},
		{"NG.RNGWHHD.D", &data.HenryHubPrice, "henry hub price"},
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
	case "OIL", "RBOB", "ULSD", "NG", "NATGAS":
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
	case "NG", "NATGAS":
		inventoryObs = eiaData.NaturalGasStorage
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

	// Natural gas uses BCF, petroleum uses M bbl
	unit := "M bbl/wk"
	if currency == "NG" || currency == "NATGAS" {
		unit = "BCF/wk"
	}

	if eia.InventoryTrend == "BUILD" {
		parts = append(parts, fmt.Sprintf("%s build season in %s (avg %.1f %s)", currency, monthName, eia.AvgWeeklyChange, unit))
	} else if eia.InventoryTrend == "DRAW" {
		parts = append(parts, fmt.Sprintf("%s draw season in %s (avg %.1f %s)", currency, monthName, eia.AvgWeeklyChange, unit))
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

// ComputeNaturalGasData processes raw EIA natural gas data into actionable metrics.
func ComputeNaturalGasData(eiaData *EIASeasonalData) *NaturalGasData {
	if eiaData == nil || len(eiaData.NaturalGasStorage) == 0 {
		return nil
	}

	ng := &NaturalGasData{}

	// Storage data (sorted descending — newest first)
	storage := eiaData.NaturalGasStorage
	ng.LatestStorageBCF = storage[0].Value

	// Week-over-week change
	if len(storage) >= 2 {
		ng.WeekOverWeekChange = storage[0].Value - storage[1].Value
	}

	// Determine storage trend from last 4 weeks
	if len(storage) >= 4 {
		netChange := storage[0].Value - storage[3].Value
		if netChange > 5 {
			ng.StorageTrend = "INJECTION"
		} else if netChange < -5 {
			ng.StorageTrend = "WITHDRAWAL"
		} else {
			ng.StorageTrend = "FLAT"
		}
	}

	// Storage vs 5-year average for current week
	now := time.Now()
	currentWeek := now.YearDay() / 7
	currentYear := now.Year()

	var historicalLevels []float64
	for _, obs := range storage {
		week := obs.Date.YearDay() / 7
		year := obs.Date.Year()
		if week == currentWeek && year < currentYear {
			historicalLevels = append(historicalLevels, obs.Value)
		}
	}
	if len(historicalLevels) > 0 {
		sum := 0.0
		for _, v := range historicalLevels {
			sum += v
		}
		avg := sum / float64(len(historicalLevels))
		if avg > 0 {
			ng.StorageVs5YrAvgPct = (ng.LatestStorageBCF - avg) / avg * 100
		}
	}

	// Season context: injection (Apr-Oct) or withdrawal (Nov-Mar)
	month := int(now.Month())
	if month >= 4 && month <= 10 {
		ng.SeasonContext = "INJECTION"
	} else {
		ng.SeasonContext = "WITHDRAWAL"
	}

	// Henry Hub price metrics
	if len(eiaData.HenryHubPrice) > 0 {
		ng.HenryHubLatest = eiaData.HenryHubPrice[0].Value

		// 7-day average
		sum7 := 0.0
		count7 := 0
		for i := 0; i < len(eiaData.HenryHubPrice) && i < 7; i++ {
			sum7 += eiaData.HenryHubPrice[i].Value
			count7++
		}
		if count7 > 0 {
			ng.HenryHub7dAvg = sum7 / float64(count7)
		}

		// 30-day average
		sum30 := 0.0
		count30 := 0
		for i := 0; i < len(eiaData.HenryHubPrice) && i < 30; i++ {
			sum30 += eiaData.HenryHubPrice[i].Value
			count30++
		}
		if count30 > 0 {
			ng.HenryHub30dAvg = sum30 / float64(count30)
		}
	}

	return ng
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

// ---------------------------------------------------------------------------
// Package-level EIA cache (TTL: 12h — EIA data updates weekly)
// ---------------------------------------------------------------------------

const eiaDataCacheTTL = 12 * time.Hour

var (
	eiaGlobalCacheMu  sync.Mutex
	eiaGlobalCache    *EIASeasonalData
	eiaGlobalFetchedAt time.Time
)

// GetCachedOrFetchEIA returns cached EIASeasonalData if within TTL, else fetches fresh data.
// Returns nil without error if EIA_API_KEY is not set.
func GetCachedOrFetchEIA(ctx context.Context) (*EIASeasonalData, error) {
	eiaGlobalCacheMu.Lock()
	defer eiaGlobalCacheMu.Unlock()

	if eiaGlobalCache != nil && time.Since(eiaGlobalFetchedAt) < eiaDataCacheTTL {
		return eiaGlobalCache, nil
	}

	apiKey := os.Getenv("EIA_API_KEY")
	if apiKey == "" {
		return nil, nil // graceful: no key configured
	}

	client := NewEIAClient(apiKey)
	data, err := client.FetchSeasonalData(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch EIA data: %w", err)
	}

	eiaGlobalCache = data
	eiaGlobalFetchedAt = time.Now()
	return data, nil
}
