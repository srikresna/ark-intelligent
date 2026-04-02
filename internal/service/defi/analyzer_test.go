package defi

import (
	"testing"
)

func TestAnalyzeSignals_TVLDrop(t *testing.T) {
	r := &DeFiReport{
		TotalTVL:     50e9,
		TVLChange24h: -6.5,
		Available:    true,
	}
	signals := analyzeSignals(r)

	found := false
	for _, s := range signals {
		if s.Type == "risk_off" {
			found = true
			if s.Severity != "alert" {
				t.Errorf("expected severity=alert, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected risk_off signal for TVL drop >5%")
	}
}

func TestAnalyzeSignals_TVLSurge(t *testing.T) {
	r := &DeFiReport{
		TotalTVL:     50e9,
		TVLChange24h: 7.0,
		Available:    true,
	}
	signals := analyzeSignals(r)

	found := false
	for _, s := range signals {
		if s.Type == "tvl_surge" {
			found = true
		}
	}
	if !found {
		t.Error("expected tvl_surge signal for TVL increase >5%")
	}
}

func TestAnalyzeSignals_DEXSurge(t *testing.T) {
	r := &DeFiReport{
		TotalTVL:     50e9,
		TVLChange24h: 0,
		DEX:          DEXVolume{TotalVolume24h: 5e9, Change24h: 80},
		Available:    true,
	}
	signals := analyzeSignals(r)

	found := false
	for _, s := range signals {
		if s.Type == "dex_surge" {
			found = true
		}
	}
	if !found {
		t.Error("expected dex_surge signal for volume increase >50%")
	}
}

func TestAnalyzeSignals_StablecoinGrowth(t *testing.T) {
	r := &DeFiReport{
		TotalTVL:              50e9,
		TVLChange24h:          0,
		StablecoinChange7D:    3.5,
		TotalStablecoinSupply: 130e9,
		Available:             true,
	}
	signals := analyzeSignals(r)

	found := false
	for _, s := range signals {
		if s.Type == "liquidity_inflow" {
			found = true
		}
	}
	if !found {
		t.Error("expected liquidity_inflow signal for stablecoin growth >2%")
	}
}

func TestAnalyzeSignals_Neutral(t *testing.T) {
	r := &DeFiReport{
		TotalTVL:     50e9,
		TVLChange24h: 0.5,
		DEX:          DEXVolume{TotalVolume24h: 3e9, Change24h: 5},
		Available:    true,
	}
	signals := analyzeSignals(r)

	if len(signals) != 1 || signals[0].Type != "neutral" {
		t.Errorf("expected single neutral signal, got %d signals", len(signals))
	}
}
