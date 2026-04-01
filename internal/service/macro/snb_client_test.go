package macro

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SNB CSV parser tests
// ---------------------------------------------------------------------------

const snbSampleCSV = `"CubeId";"snbbipo"
"PublishingDate";"2026-03-31 09:00"

"Date";"D0";"Value"
"2024-11";"GFG";"60000.00"
"2024-11";"D";"700000.00"
"2024-11";"GB";"460000.00"
"2024-11";"N";"74000.00"
"2024-11";"T0";"810000.00"
"2024-12";"GFG";"61000.00"
"2024-12";"D";"720000.00"
"2024-12";"GB";"462000.00"
"2024-12";"N";"75000.00"
"2024-12";"T0";"825000.00"
`

func TestParseSNBCSV_BasicParsing(t *testing.T) {
	r := strings.NewReader(snbSampleCSV)
	data, err := parseSNBCSV(r)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Latest month should be 2024-12
	assert.Equal(t, 2024, data.LatestDate.Year())
	assert.Equal(t, time.December, data.LatestDate.Month())

	// Previous month should be 2024-11
	assert.Equal(t, time.November, data.PreviousDate.Month())

	// Check FX reserves
	assert.InDelta(t, 720000.0, data.FXReserves, 1.0)
	assert.InDelta(t, 700000.0, data.FXReserves-data.FXReservesMoM, 1.0) // prev month

	// Check MoM calculation: 720000 - 700000 = 20000
	assert.InDelta(t, 20000.0, data.FXReservesMoM, 1.0)

	// Check gold
	assert.InDelta(t, 61000.0, data.GoldHoldings, 1.0)
	assert.InDelta(t, 1000.0, data.GoldHoldingsMoM, 1.0)

	// Check total assets
	assert.InDelta(t, 825000.0, data.TotalAssets, 1.0)

	// Published at
	assert.Equal(t, 2026, data.PublishedAt.Year())
	assert.Equal(t, time.March, data.PublishedAt.Month())
}

func TestParseSNBCSV_InterventionDetection_SellingCHF(t *testing.T) {
	// FX reserves increase > 5B = SNB selling CHF
	csv := `"CubeId";"snbbipo"
"PublishingDate";"2026-01-15 09:00"

"Date";"D0";"Value"
"2025-10";"D";"700000.00"
"2025-10";"GFG";"60000.00"
"2025-10";"GB";"450000.00"
"2025-10";"N";"74000.00"
"2025-10";"T0";"800000.00"
"2025-11";"D";"715000.00"
"2025-11";"GFG";"60100.00"
"2025-11";"GB";"451000.00"
"2025-11";"N";"74100.00"
"2025-11";"T0";"815000.00"
`
	// FX MoM = +15000 (15B CHF) → SELLING_CHF (threshold 5000)
	data, err := parseSNBCSV(strings.NewReader(csv))
	require.NoError(t, err)
	require.NotNil(t, data)

	assert.True(t, data.IsLikelyIntervention)
	assert.Equal(t, "SELLING_CHF", data.InterventionDir)
	assert.Contains(t, data.AlertMessage, "SNB")
}

func TestParseSNBCSV_InterventionDetection_BuyingCHF(t *testing.T) {
	// FX reserves decrease > 5B = SNB buying CHF
	csv := `"CubeId";"snbbipo"
"PublishingDate";"2026-01-15 09:00"

"Date";"D0";"Value"
"2025-10";"D";"700000.00"
"2025-10";"GFG";"60000.00"
"2025-10";"GB";"450000.00"
"2025-10";"N";"74000.00"
"2025-10";"T0";"800000.00"
"2025-11";"D";"685000.00"
"2025-11";"GFG";"60100.00"
"2025-11";"GB";"445000.00"
"2025-11";"N";"74100.00"
"2025-11";"T0";"784000.00"
`
	// FX MoM = -15000 (15B CHF) → BUYING_CHF
	data, err := parseSNBCSV(strings.NewReader(csv))
	require.NoError(t, err)
	require.NotNil(t, data)

	assert.True(t, data.IsLikelyIntervention)
	assert.Equal(t, "BUYING_CHF", data.InterventionDir)
}

func TestParseSNBCSV_NoIntervention(t *testing.T) {
	// FX reserves change < 5B = no intervention
	csv := `"CubeId";"snbbipo"
"PublishingDate";"2026-01-15 09:00"

"Date";"D0";"Value"
"2025-10";"D";"700000.00"
"2025-10";"GFG";"60000.00"
"2025-10";"GB";"450000.00"
"2025-10";"N";"74000.00"
"2025-10";"T0";"800000.00"
"2025-11";"D";"701200.00"
"2025-11";"GFG";"60050.00"
"2025-11";"GB";"450500.00"
"2025-11";"N";"74050.00"
"2025-11";"T0";"801500.00"
`
	// FX MoM = +1200 < 5000 → no intervention
	data, err := parseSNBCSV(strings.NewReader(csv))
	require.NoError(t, err)
	require.NotNil(t, data)

	assert.False(t, data.IsLikelyIntervention)
	assert.Equal(t, "NONE", data.InterventionDir)
}

func TestParseSNBCSV_EmptyBody(t *testing.T) {
	_, err := parseSNBCSV(strings.NewReader(""))
	assert.Error(t, err)
}

func TestFormatSNBData_NilInput(t *testing.T) {
	result := FormatSNBData(nil)
	assert.Contains(t, result, "tidak tersedia")
}

func TestFormatSNBData_WithData(t *testing.T) {
	d := &SNBData{
		FetchedAt:            time.Now(),
		LatestDate:           time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC),
		PreviousDate:         time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC),
		FXReserves:           715000.0,
		GoldHoldings:         60100.0,
		SightDeposits:        451000.0,
		Banknotes:            74100.0,
		TotalAssets:          815000.0,
		FXReservesMoM:        15000.0,
		GoldHoldingsMoM:      100.0,
		SightDepositsMoM:     1000.0,
		TotalAssetsMoM:       15000.0,
		IsLikelyIntervention: true,
		InterventionDir:      "SELLING_CHF",
		AlertMessage:         "⚠️ SNB kemungkinan intervensi: FX reserves naik +15.0B CHF",
	}

	result := FormatSNBData(d)

	assert.Contains(t, result, "SNB Balance Sheet")
	assert.Contains(t, result, "Nov 2025")
	assert.Contains(t, result, "715.0B")
	assert.Contains(t, result, "menjual CHF")
	assert.Contains(t, result, "melemahkan CHF")
	assert.Less(t, len(result), 3000, "output should be <3000 chars")
}
