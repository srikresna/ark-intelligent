package cot

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestDetectRegime_Empty(t *testing.T) {
	r := DetectRegime(nil)
	if r.Regime != RegimeUncertainty {
		t.Errorf("DetectRegime(nil) = %v, want UNCERTAINTY", r.Regime)
	}
}

func TestDetectRegime_RiskOff(t *testing.T) {
	// Safe havens bullish (JPY, CHF), risk FX bearish
	analyses := []domain.COTAnalysis{
		{Contract: domain.COTContract{Currency: "JPY"}, SentimentScore: 50},
		{Contract: domain.COTContract{Currency: "CHF"}, SentimentScore: 40},
		{Contract: domain.COTContract{Currency: "AUD"}, SentimentScore: -30},
		{Contract: domain.COTContract{Currency: "NZD"}, SentimentScore: -25},
	}
	r := DetectRegime(analyses)
	if r.Regime != RegimeRiskOff {
		t.Errorf("DetectRegime(safe-haven-bullish) = %v, want RISK-OFF", r.Regime)
	}
	if r.Confidence < 50 {
		t.Errorf("Confidence = %v, want >= 50", r.Confidence)
	}
	if len(r.Factors) == 0 {
		t.Error("Expected non-empty factors")
	}
}

func TestDetectRegime_RiskOn(t *testing.T) {
	// Risk FX bullish, safe havens bearish
	analyses := []domain.COTAnalysis{
		{Contract: domain.COTContract{Currency: "JPY"}, SentimentScore: -50},
		{Contract: domain.COTContract{Currency: "CHF"}, SentimentScore: -40},
		{Contract: domain.COTContract{Currency: "AUD"}, SentimentScore: 50},
		{Contract: domain.COTContract{Currency: "CAD"}, SentimentScore: 40},
		{Contract: domain.COTContract{Currency: "NZD"}, SentimentScore: 30},
	}
	r := DetectRegime(analyses)
	if r.Regime != RegimeRiskOn {
		t.Errorf("DetectRegime(risk-on) = %v, want RISK-ON", r.Regime)
	}
}

func TestDetectRegime_Uncertainty_BothBullish(t *testing.T) {
	// Both safe havens and risk FX bullish = confused market
	analyses := []domain.COTAnalysis{
		{Contract: domain.COTContract{Currency: "JPY"}, SentimentScore: 60},
		{Contract: domain.COTContract{Currency: "AUD"}, SentimentScore: 60},
		{Contract: domain.COTContract{Currency: "NZD"}, SentimentScore: 50},
	}
	r := DetectRegime(analyses)
	if r.Regime != RegimeUncertainty {
		t.Errorf("DetectRegime(both-bullish) = %v, want UNCERTAINTY", r.Regime)
	}
}

func TestDetectRegime_MildRiskOn(t *testing.T) {
	analyses := []domain.COTAnalysis{
		{Contract: domain.COTContract{Currency: "JPY"}, SentimentScore: -15},
		{Contract: domain.COTContract{Currency: "AUD"}, SentimentScore: 25},
		{Contract: domain.COTContract{Currency: "CAD"}, SentimentScore: 20},
	}
	r := DetectRegime(analyses)
	if r.Regime != RegimeRiskOn {
		t.Errorf("DetectRegime(mild-risk-on) = %v, want RISK-ON", r.Regime)
	}
	if r.Confidence > 60 {
		t.Errorf("Mild risk-on confidence = %v, want <= 60", r.Confidence)
	}
}

func TestDetectRegime_MildRiskOff(t *testing.T) {
	analyses := []domain.COTAnalysis{
		{Contract: domain.COTContract{Currency: "CHF"}, SentimentScore: 25},
		{Contract: domain.COTContract{Currency: "AUD"}, SentimentScore: -15},
		{Contract: domain.COTContract{Currency: "NZD"}, SentimentScore: -20},
	}
	r := DetectRegime(analyses)
	if r.Regime != RegimeRiskOff {
		t.Errorf("DetectRegime(mild-risk-off) = %v, want RISK-OFF", r.Regime)
	}
}

func TestDetectRegime_GoldRiskOff(t *testing.T) {
	analyses := []domain.COTAnalysis{
		{Contract: domain.COTContract{Currency: "XAU"}, SentimentScore: 50},
		{Contract: domain.COTContract{Currency: "JPY"}, SentimentScore: 30},
		{Contract: domain.COTContract{Currency: "AUD"}, SentimentScore: -20},
	}
	r := DetectRegime(analyses)
	if r.Regime != RegimeRiskOff {
		t.Errorf("DetectRegime(gold-risk-off) = %v, want RISK-OFF", r.Regime)
	}
}

func TestDetectRegime_NoRelevantCurrencies(t *testing.T) {
	analyses := []domain.COTAnalysis{
		{Contract: domain.COTContract{Currency: "EUR"}, SentimentScore: 80},
		{Contract: domain.COTContract{Currency: "GBP"}, SentimentScore: -50},
	}
	r := DetectRegime(analyses)
	// EUR is not tracked, GBP is not safe haven or risk FX in this implementation
	// Only AUD/NZD/CAD are risk FX; only JPY/CHF/XAU are safe havens
	if r.Regime == "" {
		t.Error("Expected a regime result")
	}
}

func TestRegimeResult_HasDescription(t *testing.T) {
	r := DetectRegime(nil)
	if r.Description == "" {
		t.Error("Expected non-empty description")
	}
}
