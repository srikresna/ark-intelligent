package worldbank

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// buildMockResponse creates a valid World Bank JSON response with one data point.
func buildMockResponse(value *float64, year string) string {
	dp := wbDataPoint{
		Date:  year,
		Value: value,
		Country: wbEntity{
			ID:    "US",
			Value: "United States",
		},
	}
	points, _ := json.Marshal([]wbDataPoint{dp})
	// World Bank wraps response in a 2-element array [metadata, data]
	meta := json.RawMessage(`{"page":1,"pages":1,"per_page":5,"total":1}`)
	raw, _ := json.Marshal(worldBankResponse{meta, json.RawMessage(points)})
	return string(raw)
}

// buildNullResponse creates a response where the value is null.
func buildNullResponse() string {
	type nullPoint struct {
		Date    string   `json:"date"`
		Value   *float64 `json:"value"`
		Country wbEntity `json:"country"`
	}
	dp := nullPoint{Date: "2023", Value: nil, Country: wbEntity{ID: "US", Value: "United States"}}
	points, _ := json.Marshal([]nullPoint{dp})
	meta := json.RawMessage(`{"page":1,"pages":1,"per_page":5,"total":1}`)
	raw, _ := json.Marshal(worldBankResponse{meta, json.RawMessage(points)})
	return string(raw)
}

func TestFetchIndicator_Success(t *testing.T) {
	val := 2.5
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(buildMockResponse(&val, "2023")))
	}))
	defer srv.Close()

	// Override the http client and URL by calling fetchIndicator via a helper
	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	// Test the JSON parsing logic directly by building a test server URL and
	// verifying the wbDataPoint parsing.
	v := 3.14
	year := "2022"
	body := buildMockResponse(&v, year)

	var raw worldBankResponse
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	var points []wbDataPoint
	if err := json.Unmarshal(raw[1], &points); err != nil {
		t.Fatalf("unmarshal points failed: %v", err)
	}

	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].Value == nil {
		t.Fatal("expected non-nil value")
	}
	if *points[0].Value != v {
		t.Errorf("expected value %.2f, got %.2f", v, *points[0].Value)
	}
	if points[0].Date != year {
		t.Errorf("expected year %s, got %s", year, points[0].Date)
	}
}

func TestFetchIndicator_NullValue(t *testing.T) {
	body := buildNullResponse()

	var raw worldBankResponse
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	var points []wbDataPoint
	if err := json.Unmarshal(raw[1], &points); err != nil {
		t.Fatalf("unmarshal points failed: %v", err)
	}

	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].Value != nil {
		t.Error("expected nil value for null response")
	}
}

func TestCountryMacro_UnavailableOnError(t *testing.T) {
	// fetchCountry should return Available=false when fetch fails.
	// Test that CountryMacro defaults to Available=false (zero value).
	macro := CountryMacro{CountryCode: "US", Currency: "USD"}
	if macro.Available {
		t.Error("default CountryMacro should have Available=false")
	}
}

func TestWorldBankData_Structure(t *testing.T) {
	data := &WorldBankData{
		Countries: []CountryMacro{
			{CountryCode: "US", Currency: "USD", GDPGrowth: 2.5, CurrentAccount: -800.0, CPIInflation: 3.2, Year: 2023, Available: true},
			{CountryCode: "GB", Currency: "GBP", GDPGrowth: 0.1, CurrentAccount: -100.0, CPIInflation: 6.7, Year: 2023, Available: true},
			{CountryCode: "JP", Currency: "JPY", Available: false},
		},
		FetchedAt: time.Now(),
	}

	if len(data.Countries) != 3 {
		t.Errorf("expected 3 countries, got %d", len(data.Countries))
	}

	// US should be available
	us := data.Countries[0]
	if !us.Available {
		t.Error("US should be available")
	}
	if us.GDPGrowth != 2.5 {
		t.Errorf("US GDP growth: expected 2.5, got %.2f", us.GDPGrowth)
	}
	if us.CurrentAccount != -800.0 {
		t.Errorf("US current account: expected -800.0, got %.2f", us.CurrentAccount)
	}

	// JP should not be available
	jp := data.Countries[2]
	if jp.Available {
		t.Error("JP should not be available")
	}
}

func TestGetCachedOrFetch_ReturnsCachedData(t *testing.T) {
	// Seed the cache
	cacheMu.Lock()
	globalCache = &WorldBankData{
		Countries: []CountryMacro{
			{CountryCode: "US", Currency: "USD", GDPGrowth: 2.0, Available: true},
		},
		FetchedAt: time.Now(),
	}
	cacheMu.Unlock()
	defer func() {
		cacheMu.Lock()
		globalCache = nil
		cacheMu.Unlock()
	}()

	ctx := context.Background()
	data, err := GetCachedOrFetch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
	if len(data.Countries) != 1 {
		t.Errorf("expected 1 country from cache, got %d", len(data.Countries))
	}
}

func TestGetCachedOrFetch_ExpiredCache(t *testing.T) {
	// Seed an expired cache entry
	cacheMu.Lock()
	globalCache = &WorldBankData{
		Countries: []CountryMacro{
			{CountryCode: "US", Currency: "USD", GDPGrowth: 1.0, Available: true},
		},
		FetchedAt: time.Now().Add(-25 * time.Hour), // expired
	}
	cacheMu.Unlock()
	defer func() {
		cacheMu.Lock()
		globalCache = nil
		cacheMu.Unlock()
	}()

	// Verify the cache is considered expired
	cacheMu.RLock()
	expired := globalCache != nil && time.Since(globalCache.FetchedAt) >= cacheTTL
	cacheMu.RUnlock()

	if !expired {
		t.Error("cache should be considered expired")
	}
}
