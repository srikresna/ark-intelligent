package fred

import (
	"testing"
)

func TestClassifyMacroRegime_NilData(t *testing.T) {
	// TASK-173: ClassifyMacroRegime(nil) should return a safe default instead of panicking.
	r := ClassifyMacroRegime(nil)
	if r.Name == "" {
		t.Error("should return a regime name for nil data")
	}
	if r.Name != "UNKNOWN" {
		t.Errorf("expected UNKNOWN regime for nil data, got %q", r.Name)
	}
	if r.Bias != "NEUTRAL" {
		t.Errorf("expected NEUTRAL bias for nil data, got %q", r.Bias)
	}
}

func TestClassifyMacroRegime_Inflationary(t *testing.T) {
	data := &MacroData{
		CPI:          4.5,
		CorePCE:      4.0,
		UnemployRate: 3.5,
		FedFundsRate: 5.0,
		Yield10Y:     4.5,
		Yield2Y:      4.8,
		YieldSpread:  -0.3,
	}
	r := ClassifyMacroRegime(data)
	if r.Name == "" {
		t.Error("should classify a regime")
	}
	t.Logf("Regime: %s", r.Name)
}

func TestClassifyMacroRegime_Stress(t *testing.T) {
	data := &MacroData{
		CPI:          1.0,
		CorePCE:      1.5,
		UnemployRate: 8.0,
		FedFundsRate: 0.25,
		Yield10Y:     1.5,
		Yield2Y:      0.5,
		VIX:          35.0,
		NFCI:         0.5,
	}
	r := ClassifyMacroRegime(data)
	if r.Name == "" {
		t.Error("should classify a regime")
	}
	t.Logf("Regime: %s", r.Name)
}

func TestDeriveTradingImplications(t *testing.T) {
	data := &MacroData{
		CPI:          3.0,
		UnemployRate: 4.0,
		FedFundsRate: 4.5,
	}
	regime := ClassifyMacroRegime(data)
	implications := DeriveTradingImplications(regime, data)
	if len(implications) == 0 {
		t.Error("should produce at least one implication")
	}
	for _, imp := range implications {
		if imp.Asset == "" || imp.Direction == "" {
			t.Errorf("implication missing fields: %+v", imp)
		}
	}
}

func TestRegimePlainName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"INFLATIONARY", "inflasi tinggi"},
		{"DISINFLATIONARY", "inflasi menurun"},
		{"STRESS", "stress finansial"},
	}
	for _, tt := range tests {
		got := regimePlainName(tt.input)
		if got != tt.want {
			t.Errorf("regimePlainName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCheckAlerts_BothNil(t *testing.T) {
	alerts := CheckAlerts(nil, nil)
	t.Logf("Nil inputs: %d alerts", len(alerts))
}

func TestCheckAlerts_MajorShift(t *testing.T) {
	prev := &MacroData{
		YieldSpread: 0.5,
		VIX:         15,
		NFCI:        -0.3,
	}
	curr := &MacroData{
		YieldSpread: -0.3, // inversion
		VIX:         35,   // spike
		NFCI:        0.5,  // tightening
	}
	alerts := CheckAlerts(curr, prev)
	if len(alerts) == 0 {
		t.Error("significant changes should produce alerts")
	}
	for _, a := range alerts {
		t.Logf("Alert: %s — %s", a.Type, a.Title)
	}
}
