// Package validate_test provides integration tests for JSON schema validation
// of external API responses. These tests verify that validation correctly
// detects unexpected response formats and API changes.
package validate_test

import (
	"encoding/json"
	"testing"

	"github.com/arkcode369/ark-intelligent/pkg/validate"
)

// ---------------------------------------------------------------------------
// Test types that simulate external API response structures
// ---------------------------------------------------------------------------

// twelveDataResponse simulates Twelve Data API response.
type twelveDataResponse struct {
	Meta   twelveDataMeta    `json:"meta"`
	Values []twelveDataValue `json:"values"`
	Status string            `json:"status" validate:"required"`
	Code   int               `json:"code,omitempty"`
	Msg    string            `json:"message,omitempty"`
}

func (r *twelveDataResponse) Validate() error {
	if r.Status != "ok" && r.Status != "error" {
		return &validate.ValidationError{
			Field:   "status",
			Message: "unexpected status value: " + r.Status,
		}
	}
	if r.Status == "ok" && len(r.Values) == 0 {
		return &validate.ValidationError{
			Field:   "values",
			Message: "no data values in response",
		}
	}
	return nil
}

type twelveDataMeta struct {
	Symbol   string `json:"symbol" validate:"required"`
	Interval string `json:"interval"`
	Type     string `json:"type"`
}

type twelveDataValue struct {
	Datetime string `json:"datetime" validate:"required"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
}

// yahooChartResponse simulates Yahoo Finance chart API response.
type yahooChartResponse struct {
	Chart struct {
		Result []yahooChartResult `json:"result"`
		Error  *yahooError        `json:"error"`
	} `json:"chart"`
}

func (r *yahooChartResponse) Validate() error {
	if r.Chart.Error != nil {
		return &validate.ValidationError{
			Field:   "chart.error",
			Message: "Yahoo API error: " + r.Chart.Error.Code + " - " + r.Chart.Error.Description,
		}
	}
	if len(r.Chart.Result) == 0 {
		return &validate.ValidationError{
			Field:   "chart.result",
			Message: "no chart results in response",
		}
	}
	result := r.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return &validate.ValidationError{
			Field:   "chart.result.indicators.quote",
			Message: "no quote data in response",
		}
	}
	return nil
}

type yahooChartResult struct {
	Meta struct {
		Symbol             string  `json:"symbol" validate:"required"`
		Currency           string  `json:"currency"`
		InstrumentType     string  `json:"instrumentType"`
		RegularMarketPrice float64 `json:"regularMarketPrice"`
	} `json:"meta"`
	Timestamp  []int64 `json:"timestamp"`
	Indicators struct {
		Quote []yahooQuote `json:"quote"`
	} `json:"indicators"`
}

type yahooQuote struct {
	Open   []*float64 `json:"open"`
	High   []*float64 `json:"high"`
	Low    []*float64 `json:"low"`
	Close  []*float64 `json:"close"`
	Volume []*float64 `json:"volume"`
}

type yahooError struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// bybitResponse simulates Bybit API response.
type bybitResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Symbol string `json:"symbol" validate:"required"`
		List   []struct {
			Price  string `json:"price"`
			Size   string `json:"size"`
			Side   string `json:"side" validate:"required"`
			Time   string `json:"time"`
		} `json:"list"`
	} `json:"result"`
}

func (r *bybitResponse) Validate() error {
	if r.RetCode != 0 {
		return &validate.ValidationError{
			Field:   "retCode",
			Message: "Bybit API error: retCode=" + string(rune(r.RetCode)) + ", retMsg=" + r.RetMsg,
		}
	}
	if r.Result.Symbol == "" {
		return &validate.ValidationError{
			Field:   "result.symbol",
			Message: "no symbol in response",
		}
	}
	return nil
}

// mql5Event simulates MQL5 economic calendar event.
type mql5Event struct {
	ID          int    `json:"Id"`
	EventName   string `json:"EventName" validate:"required"`
	Importance  string `json:"Importance"`
	CurrencyCode string `json:"CurrencyCode"`
	FullDate    string `json:"FullDate" validate:"required"`
}

// massiveResponse simulates Massive API response.
type massiveResponse struct {
	Status  string `json:"status"`
	Results []struct {
		Ticker string `json:"ticker"`
	} `json:"results"`
}

func (r *massiveResponse) Validate() error {
	if r.Status != "OK" && r.Status != "DELAYED" {
		return &validate.ValidationError{
			Field:   "status",
			Message: "unexpected API status: " + r.Status,
		}
	}
	if len(r.Results) == 0 {
		return &validate.ValidationError{
			Field:   "results",
			Message: "no data in response",
		}
	}
	return nil
}

// claudeResponse simulates Claude API response.
type claudeResponse struct {
	ID         string `json:"id" validate:"required"`
	Type       string `json:"type" validate:"required"`
	Model      string `json:"model" validate:"required"`
	Role       string `json:"role" validate:"required"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (r *claudeResponse) Validate() error {
	if r.Error != nil {
		return &validate.ValidationError{
			Field:   "error",
			Message: "Claude API error: " + r.Error.Type + " - " + r.Error.Message,
		}
	}
	if len(r.Content) == 0 {
		return &validate.ValidationError{
			Field:   "content",
			Message: "no content blocks in response",
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Twelve Data API Tests
// ---------------------------------------------------------------------------

func TestTwelveDataResponse_Valid(t *testing.T) {
	jsonData := `{
		"meta": {"symbol": "AAPL", "interval": "1week", "type": "stock"},
		"values": [{"datetime": "2024-01-01", "open": "150.0", "high": "155.0", "low": "149.0", "close": "153.0", "volume": "1000000"}],
		"status": "ok"
	}`

	var resp twelveDataResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err != nil {
		t.Errorf("expected valid response, got error: %v", err)
	}
}

func TestTwelveDataResponse_MissingStatus(t *testing.T) {
	jsonData := `{
		"meta": {"symbol": "AAPL", "interval": "1week"},
		"values": [{"datetime": "2024-01-01", "open": "150.0", "high": "155.0", "low": "149.0", "close": "153.0", "volume": "1000000"}]
	}`

	var resp twelveDataResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for missing status, got nil")
	}
}

func TestTwelveDataResponse_EmptyValues(t *testing.T) {
	jsonData := `{
		"meta": {"symbol": "AAPL", "interval": "1week"},
		"values": [],
		"status": "ok"
	}`

	var resp twelveDataResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for empty values with ok status, got nil")
	}
}

func TestTwelveDataResponse_UnknownFields(t *testing.T) {
	jsonData := `{
		"meta": {"symbol": "AAPL", "interval": "1week"},
		"values": [{"datetime": "2024-01-01", "open": "150.0", "high": "155.0", "low": "149.0", "close": "153.0", "volume": "1000000", "unknown_field": "value"}],
		"status": "ok"
	}`

	var resp twelveDataResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for unknown fields, got nil")
	}
}

// ---------------------------------------------------------------------------
// Yahoo Finance API Tests
// ---------------------------------------------------------------------------

func TestYahooChartResponse_Valid(t *testing.T) {
	jsonData := `{
		"chart": {
			"result": [{
				"meta": {"symbol": "AAPL", "currency": "USD", "instrumentType": "EQUITY", "regularMarketPrice": 150.0},
				"timestamp": [1704067200],
				"indicators": {"quote": [{"open": [150.0], "high": [155.0], "low": [149.0], "close": [153.0], "volume": [1000000]}]}
			}]
		}
	}`

	var resp yahooChartResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err != nil {
		t.Errorf("expected valid response, got error: %v", err)
	}
}

func TestYahooChartResponse_APIError(t *testing.T) {
	jsonData := `{
		"chart": {
			"result": [],
			"error": {"code": "Not Found", "description": "No data found for symbol"}
		}
	}`

	var resp yahooChartResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for API error, got nil")
	}
}

func TestYahooChartResponse_EmptyResult(t *testing.T) {
	jsonData := `{
		"chart": {
			"result": []
		}
	}`

	var resp yahooChartResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for empty result, got nil")
	}
}

// ---------------------------------------------------------------------------
// Bybit API Tests
// ---------------------------------------------------------------------------

func TestBybitResponse_Valid(t *testing.T) {
	jsonData := `{
		"retCode": 0,
		"retMsg": "OK",
		"result": {
			"symbol": "BTCUSDT",
			"list": [
				{"price": "50000.00", "size": "1.5", "side": "Buy", "time": "1704067200000"}
			]
		}
	}`

	var resp bybitResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err != nil {
		t.Errorf("expected valid response, got error: %v", err)
	}
}

func TestBybitResponse_APIError(t *testing.T) {
	jsonData := `{
		"retCode": 10001,
		"retMsg": "Invalid symbol",
		"result": {"symbol": "", "list": []}
	}`

	var resp bybitResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for API error, got nil")
	}
}

// ---------------------------------------------------------------------------
// MQL5 API Tests
// ---------------------------------------------------------------------------

func TestMQL5Event_Valid(t *testing.T) {
	jsonData := `[
		{"Id": 1, "EventName": "Non-Farm Payrolls", "Importance": "high", "CurrencyCode": "USD", "FullDate": "2024-01-05T08:30:00"},
		{"Id": 2, "EventName": "GDP", "Importance": "medium", "CurrencyCode": "EUR", "FullDate": "2024-01-10T09:00:00"}
	]`

	var events []mql5Event
	if err := validate.UnmarshalStrict([]byte(jsonData), &events); err != nil {
		t.Errorf("expected valid response, got error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestMQL5Event_MissingRequiredFields(t *testing.T) {
	jsonData := `[
		{"Id": 1, "Importance": "high", "CurrencyCode": "USD", "FullDate": "2024-01-05T08:30:00"}
	]`

	var events []mql5Event
	if err := validate.UnmarshalStrict([]byte(jsonData), &events); err != nil {
		t.Errorf("strict unmarshal should allow missing optional fields: %v", err)
	}
}

func TestMQL5Event_UnknownFields(t *testing.T) {
	jsonData := `[
		{"Id": 1, "EventName": "Non-Farm Payrolls", "Importance": "high", "CurrencyCode": "USD", "FullDate": "2024-01-05T08:30:00", "unknown_field": "value"}
	]`

	var events []mql5Event
	if err := validate.UnmarshalStrict([]byte(jsonData), &events); err == nil {
		t.Error("expected error for unknown fields in strict mode, got nil")
	}
}

// ---------------------------------------------------------------------------
// Massive API Tests
// ---------------------------------------------------------------------------

func TestMassiveResponse_Valid(t *testing.T) {
	jsonData := `{
		"status": "OK",
		"results": [{"ticker": "AAPL"}]
	}`

	var resp massiveResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err != nil {
		t.Errorf("expected valid response, got error: %v", err)
	}
}

func TestMassiveResponse_DelayedStatus(t *testing.T) {
	jsonData := `{
		"status": "DELAYED",
		"results": [{"ticker": "AAPL"}]
	}`

	var resp massiveResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err != nil {
		t.Errorf("expected valid response for DELAYED status, got error: %v", err)
	}
}

func TestMassiveResponse_InvalidStatus(t *testing.T) {
	jsonData := `{
		"status": "ERROR",
		"results": [{"ticker": "AAPL"}]
	}`

	var resp massiveResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for invalid status, got nil")
	}
}

func TestMassiveResponse_EmptyResults(t *testing.T) {
	jsonData := `{
		"status": "OK",
		"results": []
	}`

	var resp massiveResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for empty results, got nil")
	}
}

// ---------------------------------------------------------------------------
// Claude API Tests
// ---------------------------------------------------------------------------

func TestClaudeResponse_Valid(t *testing.T) {
	jsonData := `{
		"id": "msg_123",
		"type": "message",
		"model": "claude-opus-4",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello!"}],
		"stop_reason": "end_turn"
	}`

	var resp claudeResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err != nil {
		t.Errorf("expected valid response, got error: %v", err)
	}
}

func TestClaudeResponse_APIError(t *testing.T) {
	jsonData := `{
		"id": "msg_123",
		"type": "error",
		"model": "claude-opus-4",
		"role": "assistant",
		"content": [],
		"error": {"type": "rate_limit_error", "message": "Rate limit exceeded"}
	}`

	var resp claudeResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for API error, got nil")
	}
}

func TestClaudeResponse_EmptyContent(t *testing.T) {
	jsonData := `{
		"id": "msg_123",
		"type": "message",
		"model": "claude-opus-4",
		"role": "assistant",
		"content": []
	}`

	var resp claudeResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for empty content, got nil")
	}
}

func TestClaudeResponse_MissingRequiredFields(t *testing.T) {
	jsonData := `{
		"type": "message",
		"model": "claude-opus-4",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello!"}]
	}`

	var resp claudeResponse
	if err := validate.UnmarshalAndValidate([]byte(jsonData), &resp); err == nil {
		t.Error("expected validation error for missing required fields, got nil")
	}
}

// ---------------------------------------------------------------------------
// Generic Validation Tests
// ---------------------------------------------------------------------------

func TestUnmarshalStrict_UnknownFields(t *testing.T) {
	type simpleStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	jsonData := `{"name": "test", "value": 42, "unknown": "field"}`

	var s simpleStruct
	if err := validate.UnmarshalStrict([]byte(jsonData), &s); err == nil {
		t.Error("expected error for unknown fields, got nil")
	}
}

func TestUnmarshalStrict_Valid(t *testing.T) {
	type simpleStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	jsonData := `{"name": "test", "value": 42}`

	var s simpleStruct
	if err := validate.UnmarshalStrict([]byte(jsonData), &s); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if s.Name != "test" || s.Value != 42 {
		t.Errorf("expected Name=test, Value=42, got Name=%s, Value=%d", s.Name, s.Value)
	}
}

func TestIsValidationError(t *testing.T) {
	vErr := &validate.ValidationError{
		Field:   "test",
		Message: "test error",
	}

	if !validate.IsValidationError(vErr) {
		t.Error("expected IsValidationError to return true for ValidationError")
	}

	if validate.IsValidationError(nil) {
		t.Error("expected IsValidationError to return false for nil")
	}

	var dummy map[string]interface{}
	regularErr := json.Unmarshal([]byte("invalid"), &dummy)
	if validate.IsValidationError(regularErr) {
		t.Error("expected IsValidationError to return false for regular error")
	}
}
