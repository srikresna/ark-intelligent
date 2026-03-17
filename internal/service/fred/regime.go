// Package fred classifies macro regimes from FRED economic data.
package fred

import "fmt"

// MacroRegime holds the classified macro environment and bias.
type MacroRegime struct {
	Name        string // e.g., "DISINFLATIONARY", "INFLATIONARY", "STRESS", "STAGFLATION"
	YieldCurve  string // human-readable yield curve label
	Inflation   string // human-readable inflation label
	FinStress   string // human-readable financial stress label
	Labor       string // human-readable labor market label
	MonPolicy   string // monetary policy stance label
	Growth      string // growth trajectory label
	USDStrength string // USD index label
	Bias        string // directional bias e.g., "USD BEARISH bias, Gold BULLISH"
	Description string // narrative explanation
	Score       int    // composite risk score 0-100 (higher = more risk-off)
}

// ClassifyMacroRegime derives a macro regime and trading bias from FRED data.
// Uses a multi-factor scoring system across 7 dimensions.
func ClassifyMacroRegime(data *MacroData) MacroRegime {
	r := MacroRegime{}
	riskScore := 0 // accumulates bearish/risk-off pressure

	// --- 1. Yield Curve ---
	switch {
	case data.YieldSpread > 0.75:
		r.YieldCurve = fmt.Sprintf("Steepening (%.2f%%) ✅", data.YieldSpread)
	case data.YieldSpread > 0.25:
		r.YieldCurve = fmt.Sprintf("Normal (%.2f%%) ✅", data.YieldSpread)
	case data.YieldSpread > 0:
		r.YieldCurve = fmt.Sprintf("Flat (%.2f%%) ⚠️", data.YieldSpread)
		riskScore += 10
	default:
		r.YieldCurve = fmt.Sprintf("INVERTED (%.2f%%) 🔴", data.YieldSpread)
		riskScore += 25 // Inversion = recession signal
	}

	// --- 2. Inflation (Core PCE) ---
	switch {
	case data.CorePCE > 0 && data.CorePCE < 2.0:
		r.Inflation = fmt.Sprintf("Below Target (%.1f%%) ✅", data.CorePCE)
		r.Name = "DISINFLATIONARY"
	case data.CorePCE < 2.5:
		r.Inflation = fmt.Sprintf("On Target (%.1f%%) ✅", data.CorePCE)
		r.Name = "GOLDILOCKS"
	case data.CorePCE < 3.5:
		r.Inflation = fmt.Sprintf("Elevated (%.1f%%) ⚠️", data.CorePCE)
		r.Name = "NEUTRAL"
		riskScore += 10
	default:
		r.Inflation = fmt.Sprintf("High (%.1f%%) 🔴", data.CorePCE)
		r.Name = "INFLATIONARY"
		riskScore += 20
	}

	// Cross-check with CPI if available
	if data.CPI > 0 {
		r.Inflation += fmt.Sprintf(" | CPI: %.1f%%", data.CPI)
	}

	// --- 3. Financial Stress (NFCI + TED Spread) ---
	nfciStress := 0
	switch {
	case data.NFCI < -0.5:
		r.FinStress = fmt.Sprintf("Very Loose (%.3f) 🟢", data.NFCI)
	case data.NFCI < -0.1:
		r.FinStress = fmt.Sprintf("Loose (%.3f) ✅", data.NFCI)
	case data.NFCI < 0.3:
		r.FinStress = fmt.Sprintf("Neutral (%.3f)", data.NFCI)
		nfciStress = 5
	case data.NFCI < 0.7:
		r.FinStress = fmt.Sprintf("Tight (%.3f) ⚠️", data.NFCI)
		nfciStress = 15
	default:
		r.FinStress = fmt.Sprintf("VERY TIGHT (%.3f) 🔴", data.NFCI)
		nfciStress = 25
	}
	riskScore += nfciStress

	// Add TED spread if available (>50bps = credit stress)
	if data.TedSpread > 0 {
		switch {
		case data.TedSpread > 100:
			r.FinStress += fmt.Sprintf(" | TED: %.0fbps 🔴", data.TedSpread)
			riskScore += 15
		case data.TedSpread > 50:
			r.FinStress += fmt.Sprintf(" | TED: %.0fbps ⚠️", data.TedSpread)
			riskScore += 5
		default:
			r.FinStress += fmt.Sprintf(" | TED: %.0fbps ✅", data.TedSpread)
		}
	}

	// --- 4. Labor Market (Initial Claims + Unemployment Rate) ---
	switch {
	case data.InitialClaims > 0 && data.InitialClaims < 200_000:
		r.Labor = fmt.Sprintf("Very Strong (%.0fK) ✅", data.InitialClaims/1_000)
	case data.InitialClaims < 220_000:
		r.Labor = fmt.Sprintf("Strong (%.0fK) ✅", data.InitialClaims/1_000)
	case data.InitialClaims < 280_000:
		r.Labor = fmt.Sprintf("Moderate (%.0fK) ⚠️", data.InitialClaims/1_000)
		riskScore += 5
	case data.InitialClaims >= 280_000:
		r.Labor = fmt.Sprintf("Weak (%.0fK) 🔴", data.InitialClaims/1_000)
		riskScore += 20
	default:
		r.Labor = "N/A"
	}

	if data.UnemployRate > 0 {
		switch {
		case data.UnemployRate < 4.0:
			r.Labor += fmt.Sprintf(" | U-Rate: %.1f%% ✅", data.UnemployRate)
		case data.UnemployRate < 5.0:
			r.Labor += fmt.Sprintf(" | U-Rate: %.1f%% ⚠️", data.UnemployRate)
			riskScore += 5
		default:
			r.Labor += fmt.Sprintf(" | U-Rate: %.1f%% 🔴", data.UnemployRate)
			riskScore += 15
		}
	}

	// --- 5. Monetary Policy (Fed Funds Rate + Breakeven) ---
	realRate := data.FedFundsRate - data.Breakeven5Y // positive real rate = tight policy
	switch {
	case data.FedFundsRate == 0:
		r.MonPolicy = "N/A"
	case realRate > 1.5:
		r.MonPolicy = fmt.Sprintf("Very Restrictive (FFR: %.2f%% | Real: +%.2f%%) 🔴", data.FedFundsRate, realRate)
		riskScore += 15
	case realRate > 0.5:
		r.MonPolicy = fmt.Sprintf("Restrictive (FFR: %.2f%% | Real: +%.2f%%) ⚠️", data.FedFundsRate, realRate)
		riskScore += 5
	case realRate > -0.5:
		r.MonPolicy = fmt.Sprintf("Neutral (FFR: %.2f%% | Real: %.2f%%)", data.FedFundsRate, realRate)
	default:
		r.MonPolicy = fmt.Sprintf("Accommodative (FFR: %.2f%% | Real: %.2f%%) ✅", data.FedFundsRate, realRate)
	}

	// --- 6. Growth (GDP) ---
	if data.GDPGrowth != 0 {
		switch {
		case data.GDPGrowth > 3.0:
			r.Growth = fmt.Sprintf("Strong (%.1f%% QoQ ann.) ✅", data.GDPGrowth)
		case data.GDPGrowth > 1.5:
			r.Growth = fmt.Sprintf("Moderate (%.1f%% QoQ ann.) ✅", data.GDPGrowth)
		case data.GDPGrowth > 0:
			r.Growth = fmt.Sprintf("Slow (%.1f%% QoQ ann.) ⚠️", data.GDPGrowth)
			riskScore += 10
		default:
			r.Growth = fmt.Sprintf("CONTRACTION (%.1f%% QoQ ann.) 🔴", data.GDPGrowth)
			r.Name = "RECESSION"
			riskScore += 30
		}
	} else {
		r.Growth = "N/A"
	}

	// --- 7. USD Strength ---
	if data.DXY > 0 {
		switch {
		case data.DXY > 110:
			r.USDStrength = fmt.Sprintf("Very Strong (DXY: %.1f) 💪", data.DXY)
		case data.DXY > 103:
			r.USDStrength = fmt.Sprintf("Strong (DXY: %.1f) ✅", data.DXY)
		case data.DXY > 97:
			r.USDStrength = fmt.Sprintf("Neutral (DXY: %.1f)", data.DXY)
		default:
			r.USDStrength = fmt.Sprintf("Weak (DXY: %.1f) ⚠️", data.DXY)
		}
	} else {
		r.USDStrength = "N/A"
	}

	// --- Regime Override Rules ---
	// Financial stress dominates when very elevated
	if data.NFCI > 0.7 {
		r.Name = "STRESS"
	}
	// Stagflation: high inflation + weak growth
	if data.CorePCE > 3.5 && data.GDPGrowth < 1.0 && data.GDPGrowth != 0 {
		r.Name = "STAGFLATION"
		riskScore += 20
	}

	// Cap risk score at 100
	if riskScore > 100 {
		riskScore = 100
	}
	r.Score = riskScore

	// --- Overall Bias (derived from composite signals) ---
	r.Bias, r.Description = deriveBias(data, r)

	return r
}

// deriveBias determines the overall trading bias from the macro regime.
func deriveBias(data *MacroData, r MacroRegime) (string, string) {
	disinflationary := data.CorePCE > 0 && data.CorePCE < 3.0
	steepening := data.YieldSpread > 0.25
	looseConditions := data.NFCI < 0
	strongLabor := data.InitialClaims > 0 && data.InitialClaims < 250_000
	restrictivePolicy := data.FedFundsRate > 0 && (data.FedFundsRate-data.Breakeven5Y) > 0.5
	growthPositive := data.GDPGrowth > 1.5

	switch r.Name {
	case "STRESS":
		return "Safe-haven BID (JPY, CHF, Gold)",
			"Financial stress elevated — defensive positioning favored. Risk-off flow into safe havens."

	case "RECESSION":
		return "USD MIXED | Gold BULLISH | Risk FX BEARISH",
			"GDP contracting — risk-off dominates. Monitor Fed pivot signals for reversal."

	case "STAGFLATION":
		return "Gold BULLISH | USD complex | Equities BEARISH",
			"Stagflation environment: high inflation + weak growth = gold/commodities preferred, risk assets under pressure."

	case "INFLATIONARY":
		if restrictivePolicy {
			return "USD BULLISH bias, Risk FX BEARISH",
				"High inflation + restrictive Fed = dollar strength, emerging market pressure, gold mixed."
		}
		return "USD BULLISH bias, Gold BULLISH (inflation hedge)",
			"High inflation without sufficient rate response = real yields suppressed, gold attractive."
	}

	// Goldilocks / Disinflationary scenarios
	switch {
	case disinflationary && steepening && looseConditions && growthPositive:
		return "USD BEARISH bias | Risk FX BULLISH | Gold BULLISH",
			"Ideal macro backdrop: disinflation + steepening curve + loose conditions + positive growth = risk-on."

	case disinflationary && strongLabor && steepening:
		return "Risk-on bias | AUD/NZD/CAD favored | Gold neutral",
			"Healthy expansion phase: strong labor + steepening curve = commodity FX and equities favored."

	case restrictivePolicy && !steepening:
		return "USD BULLISH bias | Gold BEARISH | EM under pressure",
			"Tight monetary policy + flat/inverted curve = USD strength persists until pivot signal."

	default:
		return "Mixed — selective bias",
			"Conflicting macro signals. Score-based risk-off pressure: " + fmt.Sprintf("%d/100", r.Score) + ". Be selective in positioning."
	}
}
