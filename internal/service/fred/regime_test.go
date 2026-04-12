package fred

import (
	"strings"
	"testing"
)

func TestClassifyMacroRegime(t *testing.T) {
	tests := []struct {
		name          string
		data          MacroData
		wantName      string
		wantScoreMin  int
		wantScoreMax  int
		wantSahmAlert bool
		wantBiasKW    []string // keywords expected in Bias
	}{
		{
			name: "GOLDILOCKS regime",
			data: MacroData{
				CorePCE:       2.2,
				YieldSpread:   0.5,
				NFCI:          -0.3,
				InitialClaims: 200_000,
				GDPGrowth:     2.5,
			},
			wantName:     "GOLDILOCKS",
			wantScoreMin: 0,
			wantScoreMax: 20,
		},
		{
			name: "INFLATIONARY regime",
			data: MacroData{
				CorePCE:     4.0,
				YieldSpread: -0.5,
			},
			wantName:     "INFLATIONARY",
			wantScoreMin: 20,
			wantScoreMax: 100,
			wantBiasKW:   []string{"USD BULLISH"},
		},
		{
			name: "RECESSION via Sahm Rule",
			data: MacroData{
				SahmRule:    0.6,
				CorePCE:     2.0,
				YieldSpread: 0.5,
			},
			wantName:      "RECESSION",
			wantSahmAlert: true,
			wantBiasKW:    []string{"DEFENSIVE", "Gold BULLISH"},
		},
		{
			name: "RECESSION via GDP",
			data: MacroData{
				GDPGrowth: -1.0,
				CorePCE:   2.0,
			},
			wantName:   "RECESSION",
			wantBiasKW: []string{"Gold BULLISH"},
		},
		{
			name: "STRESS regime",
			data: MacroData{
				NFCI:    0.8,
				CorePCE: 2.0,
			},
			wantName:     "STRESS",
			wantScoreMin: 25,
			wantScoreMax: 100,
			wantBiasKW:   []string{"Safe-haven"},
		},
		{
			name: "STAGFLATION",
			data: MacroData{
				CorePCE:   4.0,
				GDPGrowth: 0.5,
			},
			wantName:     "STAGFLATION",
			wantScoreMin: 40,
			wantScoreMax: 100,
			wantBiasKW:   []string{"Gold BULLISH"},
		},
		{
			name: "DISINFLATIONARY",
			data: MacroData{
				CorePCE:     1.5,
				YieldSpread: 0.5,
			},
			wantName: "DISINFLATIONARY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMacroRegime(&tt.data)

			if result.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", result.Name, tt.wantName)
			}

			if tt.wantScoreMax > 0 {
				if result.Score < tt.wantScoreMin || result.Score > tt.wantScoreMax {
					t.Errorf("Score = %d, want [%d, %d]", result.Score, tt.wantScoreMin, tt.wantScoreMax)
				}
			}

			if result.SahmAlert != tt.wantSahmAlert {
				t.Errorf("SahmAlert = %v, want %v", result.SahmAlert, tt.wantSahmAlert)
			}

			for _, kw := range tt.wantBiasKW {
				if !strings.Contains(result.Bias, kw) {
					t.Errorf("Bias = %q, want it to contain %q", result.Bias, kw)
				}
			}
		})
	}
}

func TestScoreComponents(t *testing.T) {
	// Each test supplies a MacroData that isolates a single risk factor
	// and verifies the score includes the expected contribution.
	// The baseline uses CorePCE=2.2, YieldSpread=0.5 to avoid extra score
	// from flat/inverted curve or recession risk modifiers.
	tests := []struct {
		name       string
		data       MacroData
		wantMinAdd int
		desc       string
	}{
		{
			name:       "Inverted 2Y-10Y adds 25",
			data:       MacroData{YieldSpread: -0.5, CorePCE: 2.2},
			wantMinAdd: 25,
			desc:       "inverted yield curve",
		},
		{
			// NFCI=0.8 contributes 25 to risk score via the "VERY TIGHT" branch.
			// Baseline NFCI=0 already contributes 5 (Neutral), so net delta is 20.
			name:       "NFCI > 0.7 adds 25 (net 20 over baseline)",
			data:       MacroData{NFCI: 0.8, CorePCE: 2.2, YieldSpread: 0.5},
			wantMinAdd: 20,
			desc:       "very tight NFCI",
		},
		{
			name:       "Sahm triggered adds 35",
			data:       MacroData{SahmRule: 0.6, CorePCE: 2.2, YieldSpread: 0.5},
			wantMinAdd: 35,
			desc:       "Sahm Rule triggered",
		},
		{
			name:       "DXY > 107 adds 10",
			data:       MacroData{DXY: 115, CorePCE: 2.2, YieldSpread: 0.5},
			wantMinAdd: 10,
			desc:       "strong USD",
		},
	}

	baseline := ClassifyMacroRegime(&MacroData{CorePCE: 2.2, YieldSpread: 0.5}).Score

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMacroRegime(&tt.data)
			added := result.Score - baseline
			if added < tt.wantMinAdd {
				t.Errorf("score added for %s = %d (total %d, baseline %d), want at least %d",
					tt.desc, added, result.Score, baseline, tt.wantMinAdd)
			}
		})
	}
}

func TestUSDStrengthLabels(t *testing.T) {
	tests := []struct {
		dxy     float64
		wantSub string
	}{
		{115, "Very Strong"},
		{100, "Neutral"},
		{90, "Weak"},
	}

	for _, tt := range tests {
		t.Run(tt.wantSub, func(t *testing.T) {
			result := ClassifyMacroRegime(&MacroData{DXY: tt.dxy, CorePCE: 2.2})
			if !strings.Contains(result.USDStrength, tt.wantSub) {
				t.Errorf("USDStrength = %q, want it to contain %q", result.USDStrength, tt.wantSub)
			}
		})
	}
}

func TestDeriveBias(t *testing.T) {
	tests := []struct {
		name   string
		data   MacroData
		wantKW []string
	}{
		{
			name:   "RECESSION + Sahm gives DEFENSIVE bias",
			data:   MacroData{SahmRule: 0.6, CorePCE: 2.0, YieldSpread: 0.5},
			wantKW: []string{"DEFENSIVE", "Gold BULLISH"},
		},
		{
			name:   "RECESSION via GDP gives risk-off bias",
			data:   MacroData{GDPGrowth: -1.0, CorePCE: 2.0},
			wantKW: []string{"USD MIXED", "Risk FX BEARISH"},
		},
		{
			name:   "STRESS gives safe-haven bias",
			data:   MacroData{NFCI: 0.8, CorePCE: 2.0},
			wantKW: []string{"Safe-haven"},
		},
		{
			name:   "STAGFLATION gives gold bullish bias",
			data:   MacroData{CorePCE: 4.0, GDPGrowth: 0.5},
			wantKW: []string{"Gold BULLISH", "Equities BEARISH"},
		},
		{
			name:   "INFLATIONARY + restrictive policy gives USD BULLISH",
			data:   MacroData{CorePCE: 4.0, FedFundsRate: 5.0, Breakeven5Y: 2.0},
			wantKW: []string{"USD BULLISH"},
		},
		{
			name: "Goldilocks risk-on with disinflation + steepening + loose + growth",
			data: MacroData{
				CorePCE:       2.2,
				YieldSpread:   0.5,
				NFCI:          -0.3,
				InitialClaims: 200_000,
				GDPGrowth:     2.5,
			},
			wantKW: []string{"USD BEARISH", "Risk FX BULLISH"},
		},
		{
			name: "Healthy expansion with strong labor + steepening but no growth",
			data: MacroData{
				CorePCE:       2.2,
				YieldSpread:   0.5,
				NFCI:          0.1,
				InitialClaims: 190_000,
			},
			wantKW: []string{"Risk-on", "AUD"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMacroRegime(&tt.data)
			for _, kw := range tt.wantKW {
				if !strings.Contains(result.Bias, kw) {
					t.Errorf("Bias = %q, want it to contain %q", result.Bias, kw)
				}
			}
		})
	}
}
