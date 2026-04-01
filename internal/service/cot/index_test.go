package cot

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestIndexCalculator_ComputeMultiTimeframe_TooFewRecords(t *testing.T) {
	ic := NewIndexCalculator()
	result := ic.ComputeMultiTimeframe(make([]domain.COTRecord, 5))
	if result != nil {
		t.Error("Expected nil for < 13 records")
	}
}

func TestIndexCalculator_ComputeMultiTimeframe_13Records(t *testing.T) {
	ic := NewIndexCalculator()
	history := make([]domain.COTRecord, 13)
	for i := range history {
		history[i] = domain.COTRecord{
			SpecLong:  float64(100 + i*10),
			SpecShort: float64(50 + i*5),
		}
	}
	result := ic.ComputeMultiTimeframe(history)
	if result == nil {
		t.Fatal("Expected non-nil for 13 records")
	}
	// With 13 records, 26W and 52W should fall back
	if result.Index13W < 0 || result.Index13W > 100 {
		t.Errorf("Index13W = %v, want 0-100", result.Index13W)
	}
}

func TestIndexCalculator_ComputeMultiTimeframe_52Records(t *testing.T) {
	ic := NewIndexCalculator()
	history := make([]domain.COTRecord, 52)
	for i := range history {
		// Increasing net position over time
		history[i] = domain.COTRecord{
			SpecLong:  float64(200 - i*2),
			SpecShort: float64(100),
		}
	}
	result := ic.ComputeMultiTimeframe(history)
	if result == nil {
		t.Fatal("Expected non-nil for 52 records")
	}
	if result.Index52W < 0 || result.Index52W > 100 {
		t.Errorf("Index52W = %v, want 0-100", result.Index52W)
	}
	if result.Average < 0 || result.Average > 100 {
		t.Errorf("Average = %v, want 0-100", result.Average)
	}
	if result.Trend == "" {
		t.Error("Expected non-empty trend")
	}
}

func TestIndexCalculator_ComputeMultiTimeframe_TFFFields(t *testing.T) {
	ic := NewIndexCalculator()
	history := make([]domain.COTRecord, 20)
	for i := range history {
		history[i] = domain.COTRecord{
			LevFundLong:  float64(50000 + i*1000),
			LevFundShort: float64(30000),
		}
	}
	result := ic.ComputeMultiTimeframe(history)
	if result == nil {
		t.Fatal("Expected non-nil result for TFF fields")
	}
}

func TestIndexCalculator_ComputeROC_TooFew(t *testing.T) {
	ic := NewIndexCalculator()
	result := ic.ComputeROC(make([]domain.COTRecord, 5))
	if result != nil {
		t.Error("Expected nil for < 8 records")
	}
}

func TestIndexCalculator_ComputeROC_Normal(t *testing.T) {
	ic := NewIndexCalculator()
	history := make([]domain.COTRecord, 40)
	for i := range history {
		history[i] = domain.COTRecord{
			ContractCode: "099741",
			SpecLong:     float64(100000 - i*1000),
			SpecShort:    float64(50000),
		}
	}
	result := ic.ComputeROC(history)
	if result == nil {
		t.Fatal("Expected non-nil ROC result")
	}
	if result.Signal == "" {
		t.Error("Expected non-empty signal")
	}
	if result.ContractCode != "099741" {
		t.Errorf("ContractCode = %v, want 099741", result.ContractCode)
	}
}

func TestIndexCalculator_ComputeComposite(t *testing.T) {
	ic := NewIndexCalculator()
	analysis := domain.COTAnalysis{
		Contract:          domain.COTContract{Code: "099741", Currency: "EUR"},
		COTIndex:          75,
		SpecMomentum4W:    10000,
		SpecMomentum8W:    8000,
		Top4Concentration: 30,
		CrowdingIndex:     40,
	}
	cs := ic.ComputeComposite(analysis)
	if cs.Score < 0 || cs.Score > 100 {
		t.Errorf("Score = %v, want 0-100", cs.Score)
	}
	if cs.Currency != "EUR" {
		t.Errorf("Currency = %v, want EUR", cs.Currency)
	}
	if len(cs.Components) != 4 {
		t.Errorf("Components count = %d, want 4", len(cs.Components))
	}
}

func TestIndexCalculator_ComputeComposite_ZeroMomentum(t *testing.T) {
	ic := NewIndexCalculator()
	analysis := domain.COTAnalysis{
		COTIndex:      50,
		CrowdingIndex: 50,
	}
	cs := ic.ComputeComposite(analysis)
	if cs.Score < 0 || cs.Score > 100 {
		t.Errorf("Score = %v, want 0-100", cs.Score)
	}
	// With all neutral values, score should be around 50
	if cs.Score < 30 || cs.Score > 70 {
		t.Errorf("Score = %v, expected near 50 for neutral inputs", cs.Score)
	}
}

func TestComputeCOTIndexFromFloats(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"too few", []float64{1, 2}, 50.0},
		{"all same", []float64{5, 5, 5}, 50.0},
		{"at max", []float64{100, 0, 50}, 100.0},
		{"at min", []float64{0, 50, 100}, 0.0},
		{"midpoint", []float64{50, 0, 100}, 50.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCOTIndexFromFloats(tt.values)
			diff := got - tt.want
			if diff > 1 || diff < -1 {
				t.Errorf("computeCOTIndexFromFloats(%v) = %v, want ~%v", tt.values, got, tt.want)
			}
		})
	}
}

func TestClassifyROCSignal(t *testing.T) {
	tests := []struct {
		roc4w, accel float64
		want         string
	}{
		{10, 5, "ACCELERATING_BULL"},
		{10, -5, "DECELERATING_BULL"},
		{-10, -5, "ACCELERATING_BEAR"},
		{-10, 5, "DECELERATING_BEAR"},
	}
	for _, tt := range tests {
		got := classifyROCSignal(tt.roc4w, tt.accel)
		if got != tt.want {
			t.Errorf("classifyROCSignal(%v, %v) = %v, want %v", tt.roc4w, tt.accel, got, tt.want)
		}
	}
}

func TestFormatMultiTimeframe_Nil(t *testing.T) {
	got := FormatMultiTimeframe(nil)
	if got != "Insufficient data" {
		t.Errorf("FormatMultiTimeframe(nil) = %q, want 'Insufficient data'", got)
	}
}

func TestFormatMultiTimeframe_NonNil(t *testing.T) {
	mtf := &MultiTimeframeIndex{
		Currency: "EUR",
		Index13W: 65.5,
		Index26W: 60.0,
		Index52W: 55.0,
		Average:  59.5,
		Trend:    "RISING",
	}
	got := FormatMultiTimeframe(mtf)
	if got == "" || got == "Insufficient data" {
		t.Error("Expected formatted output")
	}
}
