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

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// datedObs holds a FRED observation with its date preserved.
type datedObs struct {
	Date  time.Time
	Value float64
}

// fetchSeriesWithDates fetches up to `limit` non-missing observations for a
// FRED series, returning values paired with their dates in descending
// chronological order (index 0 = most recent).
func fetchSeriesWithDates(ctx context.Context, client *http.Client, seriesID, apiKey string, limit int) []datedObs {
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

	var out []datedObs
	for _, obs := range result.Observations {
		if obs.Value == "." || obs.Value == "" {
			continue
		}
		v, parseErr := strconv.ParseFloat(obs.Value, 64)
		if parseErr != nil {
			continue
		}
		t, parseErr := time.Parse("2006-01-02", obs.Date)
		if parseErr != nil {
			continue
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		out = append(out, datedObs{Date: t, Value: v})
	}
	return out
}

// FetchHistoricalRegimes fetches ~1 year of historical FRED data for key
// regime-determining series, builds weekly MacroData snapshots, classifies
// the regime at each week, and returns a map of date strings to regime names.
//
// The returned map keys are "2006-01-02" formatted dates (weekly granularity).
// Monthly series are forward-filled to weekly resolution.
func FetchHistoricalRegimes(ctx context.Context, weeks int) (map[string]string, error) {
	if weeks <= 0 {
		weeks = 52
	}

	apiKey := os.Getenv("FRED_API_KEY")
	client := httpclient.New()

	// Fetch key series with dates.
	// Weekly series: fetch `weeks` observations.
	// Monthly series: fetch enough monthly obs to cover the period.
	monthlyLimit := weeks/4 + 2
	if monthlyLimit < 14 {
		monthlyLimit = 14 // need 13 months back for YoY calculation
	}

	dgs2 := fetchSeriesWithDates(ctx, client, "DGS2", apiKey, weeks)
	dgs10 := fetchSeriesWithDates(ctx, client, "DGS10", apiKey, weeks)
	pcepilfe := fetchSeriesWithDates(ctx, client, "PCEPILFE", apiKey, monthlyLimit)
	nfci := fetchSeriesWithDates(ctx, client, "NFCI", apiKey, weeks)
	sahm := fetchSeriesWithDates(ctx, client, "SAHMCURRENT", apiKey, monthlyLimit)

	if len(dgs2) == 0 && len(dgs10) == 0 && len(pcepilfe) == 0 {
		return nil, fmt.Errorf("failed to fetch any FRED historical data for regime classification")
	}

	// Build a set of weekly dates (Mondays) spanning the fetched range.
	// Use DGS2 dates as the weekly backbone since it's the most granular.
	weekDates := collectWeeklyDatesFromSeries(weeks, dgs2, dgs10, nfci)
	if len(weekDates) == 0 {
		return nil, fmt.Errorf("no weekly dates derived from FRED data")
	}

	// For each weekly date, look up the closest observation for each series
	// and classify the regime.
	result := make(map[string]string, len(weekDates))

	for _, wd := range weekDates {
		md := &MacroData{FetchedAt: wd}

		// Yields
		md.Yield2Y = lookupClosest(dgs2, wd)
		md.Yield10Y = lookupClosest(dgs10, wd)
		md.YieldSpread = md.Yield10Y - md.Yield2Y

		// Core PCE YoY% — need current month and 12 months prior
		if pceCurrent, pce12MAgo := lookupPCEYoY(pcepilfe, wd); pceCurrent > 0 && pce12MAgo > 0 {
			md.CorePCE = (pceCurrent - pce12MAgo) / pce12MAgo * 100
		}

		// NFCI
		md.NFCI = lookupClosest(nfci, wd)

		// Sahm Rule
		md.SahmRule = lookupClosest(sahm, wd)

		regime := ClassifyMacroRegime(md)
		if regime.Name != "" {
			dateStr := wd.Format("2006-01-02")
			result[dateStr] = regime.Name
		}
	}

	log.Info().Int("snapshots", len(result)).Msg("built historical regime snapshots from FRED data")
	return result, nil
}

// collectWeeklyDatesFromSeries derives a sorted (ascending) list of weekly
// dates (Mondays) from the fetched observation dates across all provided series.
func collectWeeklyDatesFromSeries(maxWeeks int, allSeries ...[]datedObs) []time.Time {
	seen := make(map[string]time.Time)
	for _, series := range allSeries {
		for _, obs := range series {
			monday := toMonday(obs.Date)
			key := monday.Format("2006-01-02")
			seen[key] = monday
		}
	}

	dates := make([]time.Time, 0, len(seen))
	for _, d := range seen {
		dates = append(dates, d)
	}

	// Sort ascending.
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[j].Before(dates[i]) {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}

	// Limit to maxWeeks most recent.
	if len(dates) > maxWeeks {
		dates = dates[len(dates)-maxWeeks:]
	}

	return dates
}

// toMonday rounds a date to the Monday of its ISO week.
func toMonday(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	return t.AddDate(0, 0, -int(weekday-time.Monday))
}

// lookupClosest finds the observation in a descending-sorted series whose date
// is closest to (but not after) the target date. Falls back to the nearest
// future observation if no past observation exists within 14 days.
func lookupClosest(obs []datedObs, target time.Time) float64 {
	if len(obs) == 0 {
		return 0
	}

	bestVal := 0.0
	bestDist := time.Duration(math.MaxInt64)

	for _, o := range obs {
		dist := target.Sub(o.Date)
		if dist < 0 {
			dist = -dist
		}
		// Prefer observations on or before the target date.
		if o.Date.After(target) {
			dist += 24 * time.Hour // slight penalty for future obs
		}
		if dist < bestDist {
			bestDist = dist
			bestVal = o.Value
		}
	}

	// Only use if within 30 days.
	if bestDist > 30*24*time.Hour {
		return 0
	}
	return bestVal
}

// lookupPCEYoY finds the PCE index value closest to the target date and
// the value ~12 months earlier, for computing YoY%.
func lookupPCEYoY(obs []datedObs, target time.Time) (current, yearAgo float64) {
	if len(obs) < 2 {
		return 0, 0
	}

	// Find closest observation to target.
	bestIdx := -1
	bestDist := time.Duration(math.MaxInt64)
	for i, o := range obs {
		dist := target.Sub(o.Date)
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}
	if bestIdx < 0 || bestDist > 45*24*time.Hour {
		return 0, 0
	}
	current = obs[bestIdx].Value

	// Find observation ~12 months earlier.
	target12 := obs[bestIdx].Date.AddDate(0, -12, 0)
	bestDist = time.Duration(math.MaxInt64)
	for _, o := range obs {
		dist := target12.Sub(o.Date)
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			yearAgo = o.Value
		}
	}
	if bestDist > 45*24*time.Hour {
		return current, 0
	}
	return current, yearAgo
}
