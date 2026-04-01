package cot

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestComputeCOTIndex(t *testing.T) {
	tests := []struct {
		name string
		nets []float64
		want float64
	}{
		{"empty", nil, 50},
		{"single value", []float64{100}, 50},
		{"all same", []float64{100, 100, 100}, 50},
		{"min=0 max=100 current=50", []float64{50, 0, 100}, 50},
		{"min=0 max=100 current=100", []float64{100, 0, 50}, 100},
		{"min=0 max=100 current=0", []float64{0, 50, 100}, 0},
		{"negative range", []float64{-50, -100, 0}, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCOTIndex(tt.nets)
			if math.Abs(got-tt.want) > 1.0 {
				t.Errorf("computeCOTIndex(%v) = %v, want ~%v", tt.nets, got, tt.want)
			}
		})
	}
}

func TestClassifySignal(t *testing.T) {
	tests := []struct {
		name         string
		cotIndex     float64
		momentum     float64
		isCommercial bool
	}{
		{"high index spec", 85, 10, false},
		{"low index spec", 15, -10, false},
		{"mid index", 50, 0, false},
		{"commercial high", 85, 10, true},
		{"commercial low", 15, -10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySignal(tt.cotIndex, tt.momentum, tt.isCommercial)
			if got == "" {
				t.Errorf("returned empty signal")
			}
		})
	}
}

func TestComputeCrowding(t *testing.T) {
	r := domain.COTRecord{
		SpecLong:  80000,
		SpecShort: 20000,
		CommLong:  50000,
		CommShort: 50000,
	}
	got := computeCrowding(r, "futures_only")
	if got < 0 || got > 100 {
		t.Errorf("computeCrowding() = %v, want 0-100", got)
	}
}

func TestSafeRatio(t *testing.T) {
	tests := []struct {
		name string
		a, b float64
		want float64
	}{
		{"normal", 10, 5, 2},
		{"zero denominator pos", 10, 0, 999.99},
		{"both zero", 0, 0, 0},
		{"negative", -10, 5, -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeRatio(tt.a, tt.b)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("safeRatio(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestClassifySignalStrength(t *testing.T) {
	tests := []struct {
		name string
		a    domain.COTAnalysis
	}{
		{"neutral", domain.COTAnalysis{COTIndex: 50, COTIndexComm: 50}},
		{"strong", domain.COTAnalysis{COTIndex: 90, COTIndexComm: 10, CrowdingIndex: 80}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySignalStrength(tt.a)
			valid := got == domain.SignalStrong || got == domain.SignalModerate || got == domain.SignalWeak || got == domain.SignalNeutral
			if !valid {
				t.Errorf("got %q, want STRONG/MODERATE/WEAK/NEUTRAL", got)
			}
		})
	}
}

func TestExtractNets(t *testing.T) {
	history := []domain.COTRecord{
		{SpecLong: 100, SpecShort: 50},
		{SpecLong: 200, SpecShort: 80},
		{SpecLong: 150, SpecShort: 70},
	}
	fn := func(r domain.COTRecord) float64 {
		return float64(r.SpecLong - r.SpecShort)
	}
	got := extractNets(history, fn)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != 50 || got[1] != 120 || got[2] != 80 {
		t.Errorf("got %v, want [50 120 80]", got)
	}
}

func TestClassifyMomentumDir(t *testing.T) {
	got := classifyMomentumDir(5000, -5000)
	if got == domain.MomentumStable {
		t.Error("spec up + comm down should not be stable")
	}
	got2 := classifyMomentumDir(0, 0)
	if got2 != domain.MomentumStable {
		t.Errorf("both zero should be stable, got %q", got2)
	}
}

func TestSignF(t *testing.T) {
	if signF(5) != 1 {
		t.Error("signF(5) should be 1")
	}
	if signF(-5) != -1 {
		t.Error("signF(-5) should be -1")
	}
	if signF(0) != 0 {
		t.Error("signF(0) should be 0")
	}
}
