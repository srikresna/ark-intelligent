// BIS SDMX CSV parser — parses BIS CSV responses into keyed observation maps.
// The BIS CSV format has a header row with column names, and data rows.
// Required columns: REF_AREA (or derived from KEY), TIME_PERIOD, OBS_VALUE.
//
// Example BIS CSV layout:
//   KEY,FREQ,REF_AREA,...,TIME_PERIOD,OBS_VALUE
//   Q.US,Q,US,...,2025-Q3,5.25
//   Q.XM,Q,XM,...,2025-Q3,4.25
package bis

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CSVRow represents one parsed observation from a BIS CSV response.
type CSVRow struct {
	RefArea    string  // BIS country/area code (REF_AREA column, or extracted from KEY)
	TimePeriod string  // Observation period (TIME_PERIOD column)
	OBSValue   float64 // Observation value (OBS_VALUE column)
}

// ParseBISCSV parses a BIS SDMX CSV body and returns all observations.
// Rows with missing or non-numeric OBS_VALUE are silently skipped.
func ParseBISCSV(body []byte) ([]CSVRow, error) {
	r := csv.NewReader(strings.NewReader(string(body)))
	r.TrimLeadingSpace = true
	r.LazyQuotes = true

	// Read header
	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("BIS CSV: empty response")
		}
		return nil, fmt.Errorf("BIS CSV: read header: %w", err)
	}

	// Locate column indices
	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToUpper(h))] = i
	}

	timePeriodIdx, hasTP := colIdx["TIME_PERIOD"]
	obsValueIdx, hasOV := colIdx["OBS_VALUE"]
	if !hasTP || !hasOV {
		return nil, fmt.Errorf("BIS CSV: missing required columns (got: %v)", header)
	}

	refAreaIdx, hasRA := colIdx["REF_AREA"]
	keyIdx, hasKey := colIdx["KEY"]

	var rows []CSVRow
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows
			continue
		}
		if len(rec) <= obsValueIdx || len(rec) <= timePeriodIdx {
			continue
		}

		obsStr := strings.TrimSpace(rec[obsValueIdx])
		if obsStr == "" || obsStr == "NaN" || obsStr == "." || obsStr == "NA" {
			continue
		}
		val, err := strconv.ParseFloat(obsStr, 64)
		if err != nil {
			continue
		}

		period := strings.TrimSpace(rec[timePeriodIdx])

		// Determine REF_AREA
		refArea := ""
		if hasRA && refAreaIdx < len(rec) {
			refArea = strings.TrimSpace(rec[refAreaIdx])
		}
		// Fallback: extract from KEY column (second segment after first dot)
		if refArea == "" && hasKey && keyIdx < len(rec) {
			key := strings.TrimSpace(rec[keyIdx])
			parts := strings.SplitN(key, ".", 3)
			if len(parts) >= 2 {
				refArea = parts[1]
			}
		}

		rows = append(rows, CSVRow{
			RefArea:    refArea,
			TimePeriod: period,
			OBSValue:   val,
		})
	}

	return rows, nil
}

// LatestByRefArea returns the most recent observation per REF_AREA from rows.
// "Most recent" is determined by lexicographic sort of TimePeriod (works for
// YYYY-QN and YYYY-MM formats).
func LatestByRefArea(rows []CSVRow) map[string]CSVRow {
	latest := make(map[string]CSVRow, len(rows))
	for _, row := range rows {
		key := row.RefArea
		if existing, ok := latest[key]; !ok || row.TimePeriod > existing.TimePeriod {
			latest[key] = row
		}
	}
	return latest
}
