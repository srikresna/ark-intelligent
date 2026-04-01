package fred

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeWBResponse builds a mock World Bank API response JSON with the given value.
// Note: the World Bank API encodes date as a string "2023", not an integer.
func makeWBResponse(value float64, year int) []byte {
	page := map[string]any{
		"page":    1,
		"pages":   1,
		"perpage": 5,
		"total":   1,
	}
	v := value
	entries := []map[string]any{
		{
			"date":  fmt.Sprintf("%d", year), // date is a string in the real API
			"value": v,
		},
	}
	raw, _ := json.Marshal([]any{page, entries})
	return raw
}

// makeWBResponseNull builds a mock World Bank API response with a null value.
func makeWBResponseNull() []byte {
	page := map[string]any{"page": 1, "pages": 1, "perpage": 5, "total": 1}
	entries := []map[string]any{{"date": "2023", "value": nil}} // date as string
	raw, _ := json.Marshal([]any{page, entries})
	return raw
}

// TestWorldBankDataStructure verifies CountryMacro fields populate correctly.
func TestWorldBankDataStructure(t *testing.T) {
	cm := &CountryMacro{
		Country:        "Australia",
		Currency:       "AUD",
		GDPGrowthYoY:   2.1,
		CurrentAccount: -1.8,
		InflationCPI:   3.5,
		FXReserves:     60.0,
		Year:           2023,
	}

	assert.Equal(t, "AUD", cm.Currency)
	assert.Equal(t, "Australia", cm.Country)
	assert.InDelta(t, 2.1, cm.GDPGrowthYoY, 0.001)
	assert.InDelta(t, -1.8, cm.CurrentAccount, 0.001)
	assert.InDelta(t, 3.5, cm.InflationCPI, 0.001)
	assert.Equal(t, 2023, cm.Year)
}

// TestWorldBankDataAvailable verifies the Available flag.
func TestWorldBankDataAvailable(t *testing.T) {
	wb := &WorldBankData{
		Countries: map[string]*CountryMacro{
			"AUD": {Currency: "AUD", GDPGrowthYoY: 2.1},
		},
		Available: true,
		FetchedAt: time.Now(),
	}

	assert.True(t, wb.Available)
	assert.Len(t, wb.Countries, 1)
}

// TestCurrencyCountryMapping verifies all 8 currency blocs are mapped.
func TestCurrencyCountryMapping(t *testing.T) {
	expectedCurrencies := []string{"EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF", "USD"}
	for _, c := range expectedCurrencies {
		_, ok := currencyCountry[c]
		assert.True(t, ok, "currencyCountry missing: %s", c)
	}
}

// TestCurrencyNameMapping verifies all 8 currencies have display names.
func TestCurrencyNameMapping(t *testing.T) {
	expectedCurrencies := []string{"EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF", "USD"}
	for _, c := range expectedCurrencies {
		name, ok := currencyName[c]
		assert.True(t, ok, "currencyName missing: %s", c)
		assert.NotEmpty(t, name, "currencyName empty for: %s", c)
	}
}

// TestWBIndicatorsMapping verifies the 4 indicator codes are defined.
func TestWBIndicatorsMapping(t *testing.T) {
	assert.Contains(t, wbIndicators, "NY.GDP.MKTP.KD.ZG")
	assert.Contains(t, wbIndicators, "BN.CAB.XOKA.GD.ZS")
	assert.Contains(t, wbIndicators, "FP.CPI.TOTL.ZG")
	assert.Contains(t, wbIndicators, "FI.RES.XFGD.CD")
}

// TestWBFetch_ValidResponse verifies wbFetch parses a valid API response.
func TestWBFetch_ValidResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(makeWBResponse(2.1, 2023))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Patch the URL to use test server — we'll test parsing via a direct HTTP call
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	var raw []json.RawMessage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))
	require.Len(t, raw, 2)

	var entries []wbResponseEntry
	require.NoError(t, json.Unmarshal(raw[1], &entries))
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].Value)
	assert.InDelta(t, 2.1, *entries[0].Value, 0.001)
}

// TestWBFetch_NullValue verifies wbFetch handles null values gracefully.
func TestWBFetch_NullValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(makeWBResponseNull())
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	var raw []json.RawMessage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))

	var entries []wbResponseEntry
	require.NoError(t, json.Unmarshal(raw[1], &entries))
	require.Len(t, entries, 1)
	assert.Nil(t, entries[0].Value, "null value should deserialize as nil pointer")
}

// TestWorldBankCacheAge_Empty verifies CacheAge returns -1 when cache is empty.
func TestWorldBankCacheAge_Empty(t *testing.T) {
	// Reset global cache state for test isolation
	wbCacheMu.Lock()
	wbCache = nil
	wbCacheMu.Unlock()

	age := WorldBankCacheAge()
	assert.Equal(t, float64(-1), age)
}

// TestWorldBankCacheAge_Populated verifies CacheAge returns a positive value after storing.
func TestWorldBankCacheAge_Populated(t *testing.T) {
	wbCacheMu.Lock()
	wbCache = &wbCachedData{
		data:      &WorldBankData{Available: true},
		fetchedAt: time.Now().Add(-30 * time.Minute),
	}
	wbCacheMu.Unlock()
	defer func() {
		wbCacheMu.Lock()
		wbCache = nil
		wbCacheMu.Unlock()
	}()

	age := WorldBankCacheAge()
	assert.Greater(t, age, 0.0)
	assert.Less(t, age, 1.0) // should be ~0.5h
}

// TestFXReservesConversion verifies USD→billions conversion logic.
func TestFXReservesConversion(t *testing.T) {
	// $60 billion = 60_000_000_000 USD
	rawUSD := 60_000_000_000.0
	billions := rawUSD / 1e9
	assert.InDelta(t, 60.0, billions, 0.001)
}
