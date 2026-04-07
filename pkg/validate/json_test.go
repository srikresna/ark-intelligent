package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

// Test struct for CNN Fear & Greed response
type testCNNResponse struct {
	FearAndGreed struct {
		Score          float64 `json:"score" validate:"required"`
		Rating         string  `json:"rating" validate:"required"`
		Timestamp      string  `json:"timestamp"`
		PreviousClose  float64 `json:"previous_close"`
		Previous1Week  float64 `json:"previous_1_week"`
		Previous1Month float64 `json:"previous_1_month"`
		Previous1Year  float64 `json:"previous_1_year"`
	} `json:"fear_and_greed" validate:"required"`
}

func (r *testCNNResponse) Validate() error {
	if err := Required("fear_and_greed.rating", r.FearAndGreed.Rating); err != nil {
		return err
	}
	if err := Range("fear_and_greed.score", r.FearAndGreed.Score, 0, 100); err != nil {
		return err
	}
	return nil
}

func TestUnmarshalStrict(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid JSON",
			json:    `{"score": 50, "rating": "Neutral"}`,
			wantErr: false,
		},
		{
			name:    "unknown field",
			json:    `{"score": 50, "rating": "Neutral", "unknown": "value"}`,
			wantErr: true,
			errMsg:  "unknown",
		},
		{
			name:    "invalid JSON",
			json:    `{"score": 50, "rating":}`,
			wantErr: true,
			errMsg:  "unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := UnmarshalStrict([]byte(tt.json), &result)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalStrict() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("UnmarshalStrict() error message should contain %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

func TestRequired(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		wantErr bool
	}{
		{
			name:    "non-empty string",
			field:   "rating",
			value:   "Neutral",
			wantErr: false,
		},
		{
			name:    "empty string",
			field:   "rating",
			value:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			field:   "rating",
			value:   "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Required(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Required() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRange(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   float64
		min     float64
		max     float64
		wantErr bool
	}{
		{
			name:    "value in range",
			field:   "score",
			value:   50,
			min:     0,
			max:     100,
			wantErr: false,
		},
		{
			name:    "value below min",
			field:   "score",
			value:   -1,
			min:     0,
			max:     100,
			wantErr: true,
		},
		{
			name:    "value above max",
			field:   "score",
			value:   101,
			min:     0,
			max:     100,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Range(tt.field, tt.value, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("Range() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOneOf(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		allowed []string
		wantErr bool
	}{
		{
			name:    "valid option",
			field:   "rating",
			value:   "Neutral",
			allowed: []string{"Fear", "Neutral", "Greed"},
			wantErr: false,
		},
		{
			name:    "invalid option",
			field:   "rating",
			value:   "Unknown",
			allowed: []string{"Fear", "Neutral", "Greed"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := OneOf(tt.field, tt.value, tt.allowed...)
			if (err != nil) != tt.wantErr {
				t.Errorf("OneOf() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnmarshalAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid CNN response",
			json:    `{"fear_and_greed":{"score":50,"rating":"Neutral"}}`,
			wantErr: false,
		},
		{
			name:    "missing required field",
			json:    `{"fear_and_greed":{"score":50}}`,
			wantErr: true,
		},
		{
			name:    "score out of range",
			json:    `{"fear_and_greed":{"score":150,"rating":"Extreme Greed"}}`,
			wantErr: true,
		},
		{
			name:    "unknown field",
			json:    `{"fear_and_greed":{"score":50,"rating":"Neutral","unknown":"value"}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result testCNNResponse
			err := UnmarshalAndValidate([]byte(tt.json), &result)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalAndValidate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	var dummy map[string]interface{}
	err := &ValidationError{
		Field:   "test_field",
		Message: "test message",
		Cause:   json.Unmarshal([]byte(`invalid`), &dummy),
	}

	if err.Error() == "" {
		t.Error("ValidationError.Error() should not be empty")
	}

	if !IsValidationError(err) {
		t.Error("IsValidationError should return true for ValidationError")
	}

	if IsValidationError(nil) {
		t.Error("IsValidationError should return false for nil")
	}
}

func TestValidateStruct(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name" validate:"required"`
		Value int    `json:"value" validate:"required"`
		Opt   string `json:"optional"`
	}

	tests := []struct {
		name    string
		input   testStruct
		wantErr bool
	}{
		{
			name:    "valid struct",
			input:   testStruct{Name: "test", Value: 10},
			wantErr: false,
		},
		{
			name:    "missing required string",
			input:   testStruct{Name: "", Value: 10},
			wantErr: true,
		},
		{
			name:    "missing required int",
			input:   testStruct{Name: "test", Value: 0},
			wantErr: true,
		},
		{
			name:    "optional field can be empty",
			input:   testStruct{Name: "test", Value: 10, Opt: ""},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStruct(&tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
