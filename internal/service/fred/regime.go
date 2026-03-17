// Package fred classifies macro regimes from FRED economic data.
package fred

import "fmt"

// MacroRegime holds the classified macro environment and bias.
type MacroRegime struct {
	Name        string // e.g., "DISINFLATIONARY", "INFLATIONARY", "STRESS"
	YieldCurve  string // human-readable yield curve label
	Inflation   string // human-readable inflation label
	FinStress   string // human-readable financial stress label
	Labor       string // human-readable labor market label
	Bias        string // directional bias e.g., "USD BEARISH bias, Gold BULLISH"
	Description string // narrative explanation
}

// ClassifyMacroRegime derives a macro regime and trading bias from FRED data.
func ClassifyMacroRegime(data *MacroData) MacroRegime {
	r := MacroRegime{}

	// --- Yield Curve ---
	switch {
	case data.YieldSpread > 0.5:
		r.YieldCurve = fmt.Sprintf("Steepening (%.2f%%) ✅", data.YieldSpread)
	case data.YieldSpread > 0:
		r.YieldCurve = fmt.Sprintf("Flat (%.2f%%) ⚠️", data.YieldSpread)
	default:
		r.YieldCurve = fmt.Sprintf("INVERTED (%.2f%%) 🔴", data.YieldSpread)
	}

	// --- Inflation (Core PCE) ---
	switch {
	case data.CorePCE < 2.5:
		r.Inflation = fmt.Sprintf("Low (%.1f%%) ✅", data.CorePCE)
		r.Name = "DISINFLATIONARY"
	case data.CorePCE < 3.5:
		r.Inflation = fmt.Sprintf("Moderate (%.1f%%)", data.CorePCE)
		r.Name = "NEUTRAL"
	default:
		r.Inflation = fmt.Sprintf("High (%.1f%%) 🔴", data.CorePCE)
		r.Name = "INFLATIONARY"
	}

	// --- Financial Stress (NFCI) ---
	// NFCI: negative = loose conditions (bullish risk), positive = tight (bearish risk)
	switch {
	case data.NFCI < -0.3:
		r.FinStress = "GREEN (loose conditions)"
	case data.NFCI > 0.3:
		r.FinStress = "RED (tight conditions) 🔴"
		r.Name = "STRESS" // override — financial stress dominates
	default:
		r.FinStress = "NEUTRAL"
	}

	// --- Labor Market (Initial Claims) ---
	// Initial Claims in raw units (thousands for ICSA)
	// < 220K = strong, > 300K = weak
	switch {
	case data.InitialClaims > 0 && data.InitialClaims < 220_000:
		r.Labor = fmt.Sprintf("Strong (%.0fK) ✅", data.InitialClaims/1_000)
	case data.InitialClaims >= 300_000:
		r.Labor = fmt.Sprintf("Weak (%.0fK) 🔴", data.InitialClaims/1_000)
	case data.InitialClaims > 0:
		r.Labor = fmt.Sprintf("Moderate (%.0fK)", data.InitialClaims/1_000)
	default:
		r.Labor = "N/A"
	}

	// --- Overall Bias ---
	disinflationary := data.CorePCE > 0 && data.CorePCE < 3.0
	steepening := data.YieldSpread > 0
	looseConditions := data.NFCI < 0
	strongLabor := data.InitialClaims > 0 && data.InitialClaims < 250_000

	switch {
	case disinflationary && steepening && looseConditions:
		r.Bias = "USD BEARISH bias, Gold BULLISH"
		r.Description = "Disinflationary + steepening curve + loose conditions = risk-on"
	case !disinflationary && !steepening:
		r.Bias = "USD BULLISH bias, Gold mixed"
		r.Description = "High inflation + inverted curve = dollar strength favored"
	case r.Name == "STRESS":
		r.Bias = "Safe-haven BID (JPY, CHF, Gold)"
		r.Description = "Financial stress elevated — defensive positioning favored"
	case steepening && strongLabor:
		r.Bias = "Risk-on bias — equities and risk FX favored"
		r.Description = "Steepening curve + strong labor = healthy expansion"
	default:
		r.Bias = "Mixed — selective bias"
		r.Description = "Conflicting macro signals, be selective in positioning"
	}

	return r
}
