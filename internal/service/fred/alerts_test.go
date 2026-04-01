package fred

import (
	"testing"
)

func TestCheckAlerts_NilInputs(t *testing.T) {
	if got := CheckAlerts(nil, nil); got != nil {
		t.Errorf("CheckAlerts(nil, nil) = %v, want nil", got)
	}
	if got := CheckAlerts(&MacroData{}, nil); got != nil {
		t.Errorf("CheckAlerts(data, nil) = %v, want nil", got)
	}
	if got := CheckAlerts(nil, &MacroData{}); got != nil {
		t.Errorf("CheckAlerts(nil, data) = %v, want nil", got)
	}
}

func TestCheckAlerts_YieldCurve2Y10Y_Uninvert(t *testing.T) {
	prev := &MacroData{YieldSpread: -0.5}
	curr := &MacroData{YieldSpread: 0.1}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertYieldUninvert)
	if found == nil {
		t.Error("Expected AlertYieldUninvert")
	}
	if found != nil && found.Severity != "HIGH" {
		t.Errorf("Severity = %q, want HIGH", found.Severity)
	}
}

func TestCheckAlerts_YieldCurve2Y10Y_Invert(t *testing.T) {
	prev := &MacroData{YieldSpread: 0.5}
	curr := &MacroData{YieldSpread: -0.1}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertYieldInvert)
	if found == nil {
		t.Error("Expected AlertYieldInvert")
	}
}

func TestCheckAlerts_3M10Y_Uninvert(t *testing.T) {
	prev := &MacroData{Spread3M10Y: -0.3}
	curr := &MacroData{Spread3M10Y: 0.2}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, Alert3MUninvert)
	if found == nil {
		t.Error("Expected Alert3MUninvert")
	}
}

func TestCheckAlerts_3M10Y_Invert(t *testing.T) {
	prev := &MacroData{Spread3M10Y: 0.2}
	curr := &MacroData{Spread3M10Y: -0.1}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, Alert3MInvert)
	if found == nil {
		t.Error("Expected Alert3MInvert")
	}
}

func TestCheckAlerts_NFCIStress(t *testing.T) {
	prev := &MacroData{NFCI: 0.3}
	curr := &MacroData{NFCI: 0.6}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertNFCIStress)
	if found == nil {
		t.Error("Expected AlertNFCIStress")
	}
}

func TestCheckAlerts_NFCILoose(t *testing.T) {
	prev := &MacroData{NFCI: -0.2}
	curr := &MacroData{NFCI: -0.4}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertNFCILoose)
	if found == nil {
		t.Error("Expected AlertNFCILoose")
	}
}

func TestCheckAlerts_SahmTrigger(t *testing.T) {
	prev := &MacroData{SahmRule: 0.4}
	curr := &MacroData{SahmRule: 0.6}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertSahmTrigger)
	if found == nil {
		t.Error("Expected AlertSahmTrigger")
	}
}

func TestCheckAlerts_SahmClear(t *testing.T) {
	prev := &MacroData{SahmRule: 0.5}
	curr := &MacroData{SahmRule: 0.2}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertSahmClear)
	if found == nil {
		t.Error("Expected AlertSahmClear")
	}
}

func TestCheckAlerts_VIXSpike(t *testing.T) {
	prev := &MacroData{VIX: 25}
	curr := &MacroData{VIX: 32}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertVIXSpike)
	if found == nil {
		t.Error("Expected AlertVIXSpike")
	}
}

func TestCheckAlerts_VIXCalm(t *testing.T) {
	prev := &MacroData{VIX: 18}
	curr := &MacroData{VIX: 13}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertVIXCalm)
	if found == nil {
		t.Error("Expected AlertVIXCalm")
	}
}

func TestCheckAlerts_NFPNegative(t *testing.T) {
	prev := &MacroData{NFPChange: 200}
	curr := &MacroData{NFPChange: -50}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertNFPNegative)
	if found == nil {
		t.Error("Expected AlertNFPNegative")
	}
}

func TestCheckAlerts_VIXBackwardation(t *testing.T) {
	prev := &MacroData{VIXTermRatio: 0.9}
	curr := &MacroData{VIXTermRatio: 1.1}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertVIXBackwardation)
	if found == nil {
		t.Error("Expected AlertVIXBackwardation")
	}
}

func TestCheckAlerts_VIXContango(t *testing.T) {
	prev := &MacroData{VIXTermRatio: 0.95}
	curr := &MacroData{VIXTermRatio: 0.85}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertVIXContango)
	if found == nil {
		t.Error("Expected AlertVIXContango")
	}
}

func TestCheckAlerts_LaborWeakening_Claims(t *testing.T) {
	prev := &MacroData{InitialClaims: 250_000}
	curr := &MacroData{InitialClaims: 300_000}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertLaborWeakening)
	if found == nil {
		t.Error("Expected AlertLaborWeakening for claims")
	}
}

func TestCheckAlerts_LaborWeakening_SahmEarlyWarning(t *testing.T) {
	prev := &MacroData{SahmRule: 0.2}
	curr := &MacroData{SahmRule: 0.35}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertLaborWeakening)
	if found == nil {
		t.Error("Expected AlertLaborWeakening for Sahm early warning")
	}
}

func TestCheckAlerts_CreditStress(t *testing.T) {
	prev := &MacroData{TedSpread: 4.5}
	curr := &MacroData{TedSpread: 5.5}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertCreditStress)
	if found == nil {
		t.Error("Expected AlertCreditStress")
	}
}

func TestCheckAlerts_CurveUninversion(t *testing.T) {
	prev := &MacroData{YieldSpread: -0.2}
	curr := &MacroData{YieldSpread: 0.3}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertCurveUninversion)
	if found == nil {
		t.Error("Expected AlertCurveUninversion")
	}
}

func TestCheckAlerts_InflationDivergence_Hawkish(t *testing.T) {
	prev := &MacroData{Breakeven5Y: 2.0, CorePCE: 3.0}
	curr := &MacroData{Breakeven5Y: 2.3, CorePCE: 2.8}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertInflationDivergence)
	if found == nil {
		t.Error("Expected AlertInflationDivergence for hawkish repricing")
	}
}

func TestCheckAlerts_InflationDivergence_Dovish(t *testing.T) {
	prev := &MacroData{Breakeven5Y: 2.5, CorePCE: 3.5}
	curr := &MacroData{Breakeven5Y: 2.2, CorePCE: 3.2}
	alerts := CheckAlerts(curr, prev)
	found := findAlertType(alerts, AlertInflationDivergence)
	if found == nil {
		t.Error("Expected AlertInflationDivergence for dovish over-pricing")
	}
}

func TestCheckAlerts_NoAlerts_NoChange(t *testing.T) {
	same := &MacroData{
		YieldSpread: 1.0,
		Spread3M10Y: 0.5,
		NFCI:        0.0,
		SahmRule:     0.1,
		VIX:         20,
	}
	alerts := CheckAlerts(same, same)
	if len(alerts) != 0 {
		t.Errorf("Expected no alerts for same data, got %d", len(alerts))
	}
}

func TestFormatMacroAlert(t *testing.T) {
	alert := MacroAlert{
		Type:        AlertVIXSpike,
		Title:       "🔴 VIX SPIKE",
		Description: "VIX crossed 30",
		Severity:    "HIGH",
		Value:       32.5,
		Previous:    25.0,
	}
	formatted := FormatMacroAlert(alert)
	if formatted == "" {
		t.Error("Expected non-empty formatted output")
	}
}

// helper
func findAlertType(alerts []MacroAlert, alertType AlertType) *MacroAlert {
	for i := range alerts {
		if alerts[i].Type == alertType {
			return &alerts[i]
		}
	}
	return nil
}
