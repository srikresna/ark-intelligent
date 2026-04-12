package cot

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
)

func TestAdjustSentimentBySurprise_NoSurprises(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: 50,
		Contract:       domain.COTContract{Currency: "EUR"},
	}
	result := AdjustSentimentBySurprise(analysis, nil, nil)
	if result != "BULLISH" {
		t.Errorf("got %q, want BULLISH for sentiment=50", result)
	}
}

func TestAdjustSentimentBySurprise_WithPositiveSurprise(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: 20,
		Contract:       domain.COTContract{Currency: "EUR"},
	}
	surprises := []domain.SurpriseRecord{
		{Currency: "EUR", SigmaValue: 3.0},
	}
	result := AdjustSentimentBySurprise(analysis, surprises, nil)
	// 20 + 3*5 = 35 → BULLISH
	if result != "BULLISH" {
		t.Errorf("got %q, want BULLISH", result)
	}
}

func TestAdjustSentimentBySurprise_WithNegativeSurprise(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: -10,
		Contract:       domain.COTContract{Currency: "USD"},
	}
	surprises := []domain.SurpriseRecord{
		{Currency: "USD", SigmaValue: -5.0},
	}
	result := AdjustSentimentBySurprise(analysis, surprises, nil)
	// -10 + (-5)*5 = -35 → BEARISH
	if result != "BEARISH" {
		t.Errorf("got %q, want BEARISH", result)
	}
}

func TestAdjustSentimentBySurprise_DifferentCurrency_NoEffect(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: 50,
		Contract:       domain.COTContract{Currency: "EUR"},
	}
	surprises := []domain.SurpriseRecord{
		{Currency: "JPY", SigmaValue: 10.0}, // different currency, should not affect
	}
	result := AdjustSentimentBySurprise(analysis, surprises, nil)
	if result != "BULLISH" {
		t.Errorf("got %q, want BULLISH (different currency surprise ignored)", result)
	}
}

func TestAdjustSentimentBySurprise_FREDYieldCurvePenalty(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: 20,
		Contract:       domain.COTContract{Currency: "EUR"},
	}
	macro := &fred.MacroData{YieldSpread: -0.5}
	result := AdjustSentimentBySurprise(analysis, nil, macro)
	// 20 - 15 = 5 → NEUTRAL
	if result != "NEUTRAL" {
		t.Errorf("got %q, want NEUTRAL with yield curve penalty", result)
	}
}

func TestAdjustSentimentBySurprise_FREDStressPenalty(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: 40,
		Contract:       domain.COTContract{Currency: "EUR"},
	}
	macro := &fred.MacroData{NFCI: 0.6}
	result := AdjustSentimentBySurprise(analysis, nil, macro)
	// 40 - 20 = 20 → NEUTRAL
	if result != "NEUTRAL" {
		t.Errorf("got %q, want NEUTRAL with NFCI stress penalty", result)
	}
}

func TestAdjustSentimentBySurprise_InflationaryUSD(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: 25,
		Contract:       domain.COTContract{Currency: "USD"},
	}
	macro := &fred.MacroData{CorePCE: 4.0}
	result := AdjustSentimentBySurprise(analysis, nil, macro)
	// 25 + 10 = 35 → BULLISH
	if result != "BULLISH" {
		t.Errorf("got %q, want BULLISH for inflationary USD", result)
	}
}

func TestAdjustSentimentBySurprise_InflationaryAUD(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: 25,
		Contract:       domain.COTContract{Currency: "AUD"},
	}
	macro := &fred.MacroData{CorePCE: 4.0}
	result := AdjustSentimentBySurprise(analysis, nil, macro)
	// 25 - 10 = 15 → NEUTRAL
	if result != "NEUTRAL" {
		t.Errorf("got %q, want NEUTRAL for inflationary AUD", result)
	}
}

func TestAdjustSentimentBySurprise_Disinflationary(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: 25,
		Contract:       domain.COTContract{Currency: "CAD"},
	}
	macro := &fred.MacroData{CorePCE: 1.5}
	result := AdjustSentimentBySurprise(analysis, nil, macro)
	// 25 + 10 = 35 → BULLISH
	if result != "BULLISH" {
		t.Errorf("got %q, want BULLISH for disinflationary CAD", result)
	}
}

func TestAdjustSentimentBySurprise_StrongBearish(t *testing.T) {
	analysis := domain.COTAnalysis{
		SentimentScore: -40,
		Contract:       domain.COTContract{Currency: "EUR"},
	}
	macro := &fred.MacroData{YieldSpread: -0.3, NFCI: 0.6}
	result := AdjustSentimentBySurprise(analysis, nil, macro)
	// -40 - 15 - 20 = -75 → STRONG BEARISH
	if result != "STRONG BEARISH" {
		t.Errorf("got %q, want STRONG BEARISH", result)
	}
}

func TestClassifyConfluence(t *testing.T) {
	tests := []struct {
		name       string
		cotBullish bool
		sigma      float64
		want       ConfluenceType
	}{
		{"bullish + positive surprise", true, 1.0, ConfluenceConfirmed},
		{"bearish + negative surprise", false, -1.0, ConfluenceConfirmed},
		{"bullish + negative surprise", true, -1.0, ConfluenceDivergence},
		{"bearish + positive surprise", false, 1.0, ConfluenceDivergence},
		{"bullish + flat surprise", true, 0.1, ConfluenceNeutral},
		{"bearish + flat surprise", false, 0.0, ConfluenceNeutral},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyConfluence(tt.cotBullish, tt.sigma)
			if got != tt.want {
				t.Errorf("ClassifyConfluence(%v, %v) = %v, want %v", tt.cotBullish, tt.sigma, got, tt.want)
			}
		})
	}
}

func TestAdjustSurpriseByFREDContext(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		sigma    float64
		regime   fred.MacroRegime
		wantMin  float64
		wantMax  float64
	}{
		{
			"stress hawkish dampened",
			"USD", 2.0, fred.MacroRegime{Name: "STRESS"},
			0.9, 1.1,
		},
		{
			"stress dovish amplified",
			"USD", -2.0, fred.MacroRegime{Name: "STRESS"},
			-3.1, -2.9,
		},
		{
			"recession hawkish dampened",
			"EUR", 3.0, fred.MacroRegime{Name: "RECESSION"},
			1.4, 1.6,
		},
		{
			"inflationary USD hawkish amplified",
			"USD", 2.0, fred.MacroRegime{Name: "INFLATIONARY"},
			2.3, 2.5,
		},
		{
			"inflationary non-USD hawkish dampened",
			"EUR", 2.0, fred.MacroRegime{Name: "INFLATIONARY"},
			1.5, 1.7,
		},
		{
			"normal regime unchanged",
			"USD", 2.0, fred.MacroRegime{Name: "NORMAL"},
			1.9, 2.1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AdjustSurpriseByFREDContext(tt.currency, tt.sigma, tt.regime)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("AdjustSurpriseByFREDContext(%q, %v, %q) = %v, want [%v, %v]",
					tt.currency, tt.sigma, tt.regime.Name, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
