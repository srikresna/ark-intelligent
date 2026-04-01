// Package fred classifies macro regimes from FRED economic data.
package fred

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// MacroRegime holds the classified macro environment and bias.
type MacroRegime struct {
	Name         string // e.g., "DISINFLATIONARY", "INFLATIONARY", "STRESS", "STAGFLATION"
	YieldCurve   string // human-readable yield curve label (2Y-10Y)
	Yield3M10Y   string // human-readable 3M-10Y spread label
	Yield2Y30Y   string // human-readable 2Y-30Y spread label
	Inflation    string // human-readable inflation label
	M2Label      string // M2 YoY growth label
	FinStress    string // human-readable financial stress label
	Labor        string // human-readable labor market label
	SahmLabel    string // Sahm Rule label
	SahmAlert    bool   // true if Sahm Rule is triggered (≥0.5)
	MonPolicy    string // monetary policy stance label
	SOFRLabel    string // SOFR vs IORB spread label
	Growth       string // growth trajectory label
	USDStrength  string // USD index label
	FedBalance   string // Fed balance sheet QT/QE status
	TGALabel      string // TGA balance status label
	LiquidityLabel string // Net liquidity regime label
	RecessionRisk string // "LOW", "ELEVATED", "HIGH — SAHM TRIGGERED"
	Bias         string // directional bias e.g., "USD BEARISH bias, Gold BULLISH"
	Description  string // narrative explanation
	Score        int    // composite risk score 0-100 (higher = more risk-off)
}

// TradingImplication represents a structured per-asset trading signal derived from macro regime.
type TradingImplication struct {
	Asset     string // e.g., "Gold", "USD", "AUD/NZD/CAD", "JPY/CHF", "Equities"
	Direction string // "BULLISH", "BEARISH", "NEUTRAL", "MIXED"
	Icon      string // emoji indicator
	Reason    string // short plain-language reason
}

// ClassifyMacroRegime derives a macro regime and trading bias from FRED data.
// Uses a multi-factor scoring system across 9 dimensions.
func ClassifyMacroRegime(data *MacroData, composites ...*domain.MacroComposites) MacroRegime {
	if data == nil {
		return MacroRegime{
			Name:        "UNKNOWN",
			YieldCurve:  "N/A",
			Bias:        "NEUTRAL",
			Description: "Insufficient data for regime classification",
		}
	}
	r := MacroRegime{}
	riskScore := 0 // accumulates bearish/risk-off pressure

	// --- 1. Yield Curve 2Y-10Y ---
	// data.YieldSpread is now authoritative (fetcher prefers FRED pre-computed T10Y2Y)
	switch {
	case data.YieldSpread > 0.75:
		r.YieldCurve = fmt.Sprintf("Steepening (%.2f%%) ✅ %s", data.YieldSpread, data.YieldSpreadTrend.Arrow())
	case data.YieldSpread > 0.25:
		r.YieldCurve = fmt.Sprintf("Normal (%.2f%%) ✅ %s", data.YieldSpread, data.YieldSpreadTrend.Arrow())
	case data.YieldSpread > 0:
		r.YieldCurve = fmt.Sprintf("Flat (%.2f%%) ⚠️ %s", data.YieldSpread, data.YieldSpreadTrend.Arrow())
		riskScore += 10
	default:
		r.YieldCurve = fmt.Sprintf("INVERTED (%.2f%%) 🔴 %s", data.YieldSpread, data.YieldSpreadTrend.Arrow())
		riskScore += 25
	}

	// --- 2. Yield Curve 3M-10Y (better recession predictor) ---
	// data.Spread3M10Y is now authoritative (fetcher prefers FRED pre-computed T10Y3M)
	if data.Spread3M10Y != 0 {
		switch {
		case data.Spread3M10Y > 0.5:
			r.Yield3M10Y = fmt.Sprintf("Normal (%.2f%%) ✅", data.Spread3M10Y)
		case data.Spread3M10Y > 0:
			r.Yield3M10Y = fmt.Sprintf("Flat (%.2f%%) ⚠️", data.Spread3M10Y)
			riskScore += 10
		default:
			r.Yield3M10Y = fmt.Sprintf("INVERTED (%.2f%%) 🔴", data.Spread3M10Y)
			riskScore += 20
		}
	} else {
		r.Yield3M10Y = "N/A"
	}

	// --- 2b. Yield Curve 2Y-30Y (long-end term premium) ---
	if data.Spread2Y30Y != 0 {
		switch {
		case data.Spread2Y30Y > 0.75:
			r.Yield2Y30Y = fmt.Sprintf("Steep (%.2f%%) ✅", data.Spread2Y30Y)
		case data.Spread2Y30Y > 0:
			r.Yield2Y30Y = fmt.Sprintf("Normal (%.2f%%) ✅", data.Spread2Y30Y)
		default:
			r.Yield2Y30Y = fmt.Sprintf("INVERTED (%.2f%%) 🔴", data.Spread2Y30Y)
			riskScore += 10
		}
	} else {
		r.Yield2Y30Y = "N/A"
	}

	// --- 3. Inflation (Core PCE with trend) ---
	trendSuffix := ""
	if data.CorePCETrend.Direction != "" {
		trendSuffix = " " + data.CorePCETrend.Arrow()
	}
	switch {
	case data.CorePCE > 0 && data.CorePCE < 2.0:
		r.Inflation = fmt.Sprintf("Below Target (%.1f%%)%s ✅", data.CorePCE, trendSuffix)
		r.Name = "DISINFLATIONARY"
	case data.CorePCE < 2.5:
		r.Inflation = fmt.Sprintf("On Target (%.1f%%)%s ✅", data.CorePCE, trendSuffix)
		r.Name = "GOLDILOCKS"
	case data.CorePCE < 3.5:
		r.Inflation = fmt.Sprintf("Elevated (%.1f%%)%s ⚠️", data.CorePCE, trendSuffix)
		r.Name = "NEUTRAL"
		riskScore += 10
	default:
		r.Inflation = fmt.Sprintf("High (%.1f%%)%s 🔴", data.CorePCE, trendSuffix)
		r.Name = "INFLATIONARY"
		riskScore += 20
	}

	// Add CPI cross-check
	if data.CPI > 0 {
		cpiArrow := ""
		if data.CPITrend.Direction != "" {
			cpiArrow = " " + data.CPITrend.Arrow()
		}
		r.Inflation += fmt.Sprintf(" | CPI: %.1f%%%s", data.CPI, cpiArrow)
	}

	// Wage Growth cross-check — sticky inflation indicator
	if data.WageGrowth > 0 {
		wageArrow := ""
		if data.WageGrowthTrend.Direction != "" {
			wageArrow = " " + data.WageGrowthTrend.Arrow()
		}
		r.Inflation += fmt.Sprintf(" | Wage: %.1f%%%s", data.WageGrowth, wageArrow)

		// Wage-price spiral override: block DISINFLATIONARY if wages still hot
		if data.WageGrowth > 5.0 && data.CorePCE > 3.0 {
			riskScore += 10 // wage-price spiral risk
		}
		if data.WageGrowth > 5.0 && r.Name == "DISINFLATIONARY" {
			r.Name = "NEUTRAL" // wages too hot for disinflationary classification
		}
	}

	// 5Y5Y Forward Inflation — inflation expectations anchoring
	if data.ForwardInflation > 0 {
		switch {
		case data.ForwardInflation > 2.8:
			r.Inflation += fmt.Sprintf(" | 5Y5Y: %.2f%% ⚠️ (De-anchoring)", data.ForwardInflation)
			riskScore += 10
			// Block DISINFLATIONARY if market expects inflation return
			if r.Name == "DISINFLATIONARY" && data.ForwardInflation > 2.5 {
				r.Name = "NEUTRAL"
			}
		case data.ForwardInflation < 2.0:
			r.Inflation += fmt.Sprintf(" | 5Y5Y: %.2f%% ⚠️ (Deflation risk)", data.ForwardInflation)
		default:
			r.Inflation += fmt.Sprintf(" | 5Y5Y: %.2f%% ✅ (Anchored)", data.ForwardInflation)
		}
	}

	// M2 YoY growth label
	if data.M2Growth != 0 {
		m2Arrow := data.M2GrowthTrend.Arrow()
		switch {
		case data.M2Growth > 8:
			r.M2Label = fmt.Sprintf("Loose +%.1f%% YoY%s 🟢", data.M2Growth, m2Arrow)
		case data.M2Growth > 3:
			r.M2Label = fmt.Sprintf("Moderate +%.1f%% YoY%s", data.M2Growth, m2Arrow)
		case data.M2Growth >= 0:
			r.M2Label = fmt.Sprintf("Tight +%.1f%% YoY%s ⚠️", data.M2Growth, m2Arrow)
		default:
			r.M2Label = fmt.Sprintf("CONTRACTING %.1f%% YoY%s 🔴", data.M2Growth, m2Arrow)
			riskScore += 5
		}
	} else {
		r.M2Label = "N/A"
	}

	// --- 4. Financial Stress (NFCI + HY Credit Spread) ---
	// TedSpread field now holds ICE BofA HY OAS spread (BAMLH0A0HYM2).
	// Normal range: <3.5% = tight credit, 3.5-6% = elevated, >6% = stress.
	nfciArrow := ""
	if data.NFCITrend.Direction != "" {
		nfciArrow = " " + data.NFCITrend.Arrow()
	}
	nfciStress := 0
	switch {
	case data.NFCI < -0.5:
		r.FinStress = fmt.Sprintf("Very Loose (%.3f)%s 🟢", data.NFCI, nfciArrow)
	case data.NFCI < -0.1:
		r.FinStress = fmt.Sprintf("Loose (%.3f)%s ✅", data.NFCI, nfciArrow)
	case data.NFCI < 0.3:
		r.FinStress = fmt.Sprintf("Neutral (%.3f)%s", data.NFCI, nfciArrow)
		nfciStress = 5
	case data.NFCI < 0.7:
		r.FinStress = fmt.Sprintf("Tight (%.3f)%s ⚠️", data.NFCI, nfciArrow)
		nfciStress = 15
	default:
		r.FinStress = fmt.Sprintf("VERY TIGHT (%.3f)%s 🔴", data.NFCI, nfciArrow)
		nfciStress = 25
	}
	riskScore += nfciStress

	// HY Credit Spread (BAMLH0A0HYM2) — in percent, not bps
	if data.TedSpread > 0 {
		switch {
		case data.TedSpread > 6.0:
			r.FinStress += fmt.Sprintf(" | HY Spread: %.2f%% 🔴", data.TedSpread)
			riskScore += 15
		case data.TedSpread > 4.0:
			r.FinStress += fmt.Sprintf(" | HY Spread: %.2f%% ⚠️", data.TedSpread)
			riskScore += 5
		default:
			r.FinStress += fmt.Sprintf(" | HY Spread: %.2f%% ✅", data.TedSpread)
		}
	}

	// VIX — real-time volatility/fear gauge
	if data.VIX > 0 {
		vixArrow := ""
		if data.VIXTrend.Direction != "" {
			vixArrow = " " + data.VIXTrend.Arrow()
		}
		switch {
		case data.VIX > 30:
			r.FinStress += fmt.Sprintf(" | VIX: %.1f%s 🔴 (Fear)", data.VIX, vixArrow)
			riskScore += 15
		case data.VIX > 20:
			r.FinStress += fmt.Sprintf(" | VIX: %.1f%s ⚠️ (Elevated)", data.VIX, vixArrow)
			riskScore += 5
		default:
			r.FinStress += fmt.Sprintf(" | VIX: %.1f%s ✅ (Calm)", data.VIX, vixArrow)
		}
	}

	// --- 5. Labor Market (Initial Claims + Unemployment Rate + Sahm Rule) ---
	claimsArrow := ""
	if data.ClaimsTrend.Direction != "" {
		claimsArrow = " " + data.ClaimsTrend.Arrow()
	}
	switch {
	case data.InitialClaims > 0 && data.InitialClaims < 200_000:
		r.Labor = fmt.Sprintf("Very Strong (%.0fK)%s ✅", data.InitialClaims/1_000, claimsArrow)
	case data.InitialClaims < 220_000:
		r.Labor = fmt.Sprintf("Strong (%.0fK)%s ✅", data.InitialClaims/1_000, claimsArrow)
	case data.InitialClaims < 280_000:
		r.Labor = fmt.Sprintf("Moderate (%.0fK)%s ⚠️", data.InitialClaims/1_000, claimsArrow)
		riskScore += 5
	case data.InitialClaims >= 280_000:
		r.Labor = fmt.Sprintf("Weak (%.0fK)%s 🔴", data.InitialClaims/1_000, claimsArrow)
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

	// Sahm Rule
	if data.SahmRule > 0 {
		switch {
		case data.SahmRule >= 0.5:
			r.SahmLabel = fmt.Sprintf("TRIGGERED (%.2f) 🚨", data.SahmRule)
			r.SahmAlert = true
			riskScore += 35
		case data.SahmRule >= 0.3:
			r.SahmLabel = fmt.Sprintf("Elevated (%.2f) ⚠️", data.SahmRule)
			riskScore += 10
		default:
			r.SahmLabel = fmt.Sprintf("Normal (%.2f) ✅", data.SahmRule)
		}
	} else {
		r.SahmLabel = "N/A"
	}

	// Nonfarm Payrolls — actual job creation
	if data.NFP > 0 {
		nfpArrow := ""
		if data.NFPTrend.Direction != "" {
			nfpArrow = " " + data.NFPTrend.Arrow()
		}
		switch {
		case data.NFPChange < 0:
			r.Labor += fmt.Sprintf(" | NFP: %.0fK chg%s 🔴 (Job Losses!)", data.NFPChange, nfpArrow)
			riskScore += 20
		case data.NFPChange < 100:
			r.Labor += fmt.Sprintf(" | NFP: +%.0fK chg%s ⚠️ (Slowdown)", data.NFPChange, nfpArrow)
			riskScore += 5
		default:
			r.Labor += fmt.Sprintf(" | NFP: +%.0fK chg%s ✅", data.NFPChange, nfpArrow)
		}
	}

	// --- 6. Monetary Policy (Fed Funds Rate + Real Rate + SOFR/IORB) ---
	realRate := data.FedFundsRate - data.Breakeven5Y
	switch {
	case data.FedFundsRate == 0:
		r.MonPolicy = "N/A"
	case realRate > 1.5:
		r.MonPolicy = fmt.Sprintf("Very Restrictive (FFR: %.2f%% | Real: %+.2f%%) 🔴", data.FedFundsRate, realRate)
		riskScore += 15
	case realRate > 0.5:
		r.MonPolicy = fmt.Sprintf("Restrictive (FFR: %.2f%% | Real: %+.2f%%) ⚠️", data.FedFundsRate, realRate)
		riskScore += 5
	case realRate > -0.5:
		r.MonPolicy = fmt.Sprintf("Neutral (FFR: %.2f%% | Real: %+.2f%%)", data.FedFundsRate, realRate)
	default:
		r.MonPolicy = fmt.Sprintf("Accommodative (FFR: %.2f%% | Real: %+.2f%%) ✅", data.FedFundsRate, realRate)
	}

	// SOFR vs IORB spread (SOFR > IORB = repo/liquidity stress)
	if data.SOFR > 0 && data.IORB > 0 {
		sofrSpread := data.SOFR - data.IORB
		switch {
		case sofrSpread > 0.1:
			r.SOFRLabel = fmt.Sprintf("SOFR: %.2f%% | IORB: %.2f%% | Spread: +%.2f%% ⚠️ (liquidity stress)", data.SOFR, data.IORB, sofrSpread)
			riskScore += 5
		case sofrSpread < -0.1:
			r.SOFRLabel = fmt.Sprintf("SOFR: %.2f%% | IORB: %.2f%% | Spread: %.2f%% ✅", data.SOFR, data.IORB, sofrSpread)
		default:
			r.SOFRLabel = fmt.Sprintf("SOFR: %.2f%% | IORB: %.2f%% | Spread: %+.2f%% (normal)", data.SOFR, data.IORB, sofrSpread)
		}
	} else if data.SOFR > 0 {
		r.SOFRLabel = fmt.Sprintf("SOFR: %.2f%%", data.SOFR)
	} else {
		r.SOFRLabel = "N/A"
	}

	// --- 7. Growth (GDP + ISM + Consumer Sentiment) ---
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
			riskScore += 30
		}
	} else {
		r.Growth = "N/A"
	}

	// ISM New Orders — leading manufacturing indicator
	if data.ISMNewOrders > 0 {
		ismArrow := ""
		if data.ISMNewOrdersTrend.Direction != "" {
			ismArrow = " " + data.ISMNewOrdersTrend.Arrow()
		}
		switch {
		case data.ISMNewOrders < 45:
			r.Growth += fmt.Sprintf(" | ISM Orders: %.1f%s 🔴 (Deep Contraction)", data.ISMNewOrders, ismArrow)
			riskScore += 15
		case data.ISMNewOrders < 50:
			r.Growth += fmt.Sprintf(" | ISM Orders: %.1f%s ⚠️ (Contraction)", data.ISMNewOrders, ismArrow)
			riskScore += 10
		case data.ISMNewOrders < 55:
			r.Growth += fmt.Sprintf(" | ISM Orders: %.1f%s (Expansion)", data.ISMNewOrders, ismArrow)
		default:
			r.Growth += fmt.Sprintf(" | ISM Orders: %.1f%s ✅ (Strong)", data.ISMNewOrders, ismArrow)
		}
	}

	// Consumer Sentiment — monthly leading growth proxy
	if data.ConsumerSentiment > 0 {
		csArrow := ""
		if data.ConsumerSentimentTrend.Direction != "" {
			csArrow = " " + data.ConsumerSentimentTrend.Arrow()
		}
		switch {
		case data.ConsumerSentiment < 60:
			r.Growth += fmt.Sprintf(" | Consumer: %.1f%s 🔴 (Pessimistic)", data.ConsumerSentiment, csArrow)
			riskScore += 10
		case data.ConsumerSentiment < 80:
			r.Growth += fmt.Sprintf(" | Consumer: %.1f%s ⚠️", data.ConsumerSentiment, csArrow)
		default:
			r.Growth += fmt.Sprintf(" | Consumer: %.1f%s ✅ (Confident)", data.ConsumerSentiment, csArrow)
		}

		// GDP override: if GDP still positive but consumer sentiment crashing, warn
		if data.GDPGrowth > 1.5 && data.ConsumerSentiment < 60 {
			// Consumer leading indicator suggests GDP will weaken
			riskScore += 5
		}
	}

	// --- 8. USD Strength ---
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

	// DXY score contribution to risk assessment
	if data.DXY > 0 {
		switch {
		case data.DXY > 107:
			riskScore += 10 // Very strong USD = pressure on risk assets
		case data.DXY < 95:
			riskScore -= 5 // Weak USD = loose conditions
		}
	}

	// --- 9. Fed Balance Sheet (QT/QE) ---
	if data.FedBalSheet > 0 {
		balDir := data.FedBalSheetTrend.Arrow()
		balTrillion := data.FedBalSheet / 1_000 // WALCL is in billions
		switch data.FedBalSheetTrend.Direction {
		case "UP":
			r.FedBalance = fmt.Sprintf("$%.2fT %s Expanding (QE) 🟢", balTrillion, balDir)
		case "DOWN":
			r.FedBalance = fmt.Sprintf("$%.2fT %s Contracting (QT) 🔴", balTrillion, balDir)
			riskScore += 5
		default:
			r.FedBalance = fmt.Sprintf("$%.2fT %s Stable", balTrillion, balDir)
		}
	} else {
		r.FedBalance = "N/A"
	}

	// --- 10. Treasury General Account (TGA) ---
	if data.TGABalance > 0 {
		tgaDir := data.TGABalanceTrend.Arrow()
		switch data.TGABalanceTrend.Direction {
		case "UP":
			r.TGALabel = fmt.Sprintf("$%.0fB %s Rising (Liquidity Drain) 🔴", data.TGABalance, tgaDir)
			riskScore += 3
		case "DOWN":
			r.TGALabel = fmt.Sprintf("$%.0fB %s Falling (Liquidity Inject) 🟢", data.TGABalance, tgaDir)
		default:
			r.TGALabel = fmt.Sprintf("$%.0fB %s Stable", data.TGABalance, tgaDir)
		}
	} else {
		r.TGALabel = "N/A"
	}

	// --- 11. Net Liquidity Regime ---
	if data.LiquidityRegime != "" {
		switch data.LiquidityRegime {
		case "EASY":
			r.LiquidityLabel = "💧 EASY — TGA drawdown + declining RRP + expanding Fed BS"
		case "TIGHT":
			r.LiquidityLabel = "🏜️ TIGHT — TGA rising + high RRP + QT"
			riskScore += 5
		default:
			r.LiquidityLabel = "⚖️ NEUTRAL — mixed liquidity signals"
		}
	}

	// --- Recession Risk Classification ---
	switch {
	case r.SahmAlert:
		r.RecessionRisk = "HIGH — SAHM TRIGGERED 🚨"
	case data.Spread3M10Y < 0 && data.YieldSpread < 0:
		r.RecessionRisk = "ELEVATED — Both curves inverted 🔴"
		riskScore += 10
	case data.Spread3M10Y < 0 || data.YieldSpread < 0:
		r.RecessionRisk = "MODERATE — Curve inversion signal ⚠️"
		riskScore += 5
	case data.SahmRule > 0.3:
		r.RecessionRisk = "MODERATE — Sahm Rule rising ⚠️"
	default:
		r.RecessionRisk = "LOW ✅"
	}

	// --- Regime Override Rules ---
	// Priority: RECESSION > STRESS > STAGFLATION > DETERIORATING > base classification.
	if r.SahmAlert {
		r.Name = "RECESSION"
	} else if data.NFCI > 0.7 || data.VIX > 35 {
		r.Name = "STRESS"
	} else if data.CorePCE > 3.5 && data.GDPGrowth < 1.0 && data.GDPGrowth != 0 {
		r.Name = "STAGFLATION"
		riskScore += 20
	} else if data.GDPGrowth < 0 && data.GDPGrowth != 0 {
		r.Name = "RECESSION"
		riskScore += 30
	} else if (r.Name == "GOLDILOCKS" || r.Name == "DISINFLATIONARY") &&
		(data.YieldSpread < 0 || data.Spread3M10Y < 0) && data.NFCI > 0.3 {
		// Yield curve inverted + financial conditions tightening = NOT goldilocks.
		// The label would mislead traders into risk-on positioning despite structural warnings.
		r.Name = "DETERIORATING"
		riskScore += 10
	}

	// --- Composite Score Enhancements (when available) ---
	if len(composites) > 0 && composites[0] != nil {
		comp := composites[0]

		// Labor Health composite can override labor-based scoring
		if comp.LaborHealth < 30 {
			riskScore += 10 // composite confirms weakening
		} else if comp.LaborHealth > 80 {
			if riskScore >= 5 {
				riskScore -= 5 // healthy labor reduces risk
			}
		}

		// Credit Stress composite refinement
		if comp.CreditStress > 70 {
			riskScore += 10 // composite confirms stress
		}

		// VIX term structure backwardation — only add if VIX not already
		// contributing maximum stress (avoid double-counting VIX signal)
		if comp.VIXTermRegime == "BACKWARDATION" && data.VIX <= 30 {
			riskScore += 10 // backwardation without extreme VIX = hidden stress
		}

		// Housing pulse warning
		if comp.HousingPulse == "COLLAPSING" {
			riskScore += 10
		} else if comp.HousingPulse == "CONTRACTING" {
			riskScore += 5
		}

		// Inflation momentum enhancement
		if comp.InflationMomentum > 0.5 && r.Name != "INFLATIONARY" {
			riskScore += 5 // inflation re-accelerating but regime hasn't caught it yet
		}
	}

	// Cap risk score at 100
	if riskScore > 100 {
		riskScore = 100
	}
	r.Score = riskScore

	// --- Overall Bias ---
	r.Bias, r.Description = deriveBias(data, r)

	return r
}

// deriveBias determines the overall trading bias from the macro regime.
func deriveBias(data *MacroData, r MacroRegime) (string, string) {
	disinflationary := data.CorePCE > 0 && data.CorePCE < 3.0
	steepening := data.YieldSpread > 0.25
	looseConditions := data.NFCI < 0
	vixElevated := data.VIX > 25
	strongLabor := data.InitialClaims > 0 && data.InitialClaims < 250_000 && (data.NFPChange == 0 || data.NFPChange > 100)
	restrictivePolicy := data.FedFundsRate > 0 && (data.FedFundsRate-data.Breakeven5Y) > 0.5
	growthPositive := data.GDPGrowth > 1.5

	switch r.Name {
	case "RECESSION":
		if r.SahmAlert {
			return "DEFENSIVE — Gold BULLISH | JPY/CHF BID | Risk FX AVOID",
				"Sahm Rule triggered — historically reliable recession signal. Defensive positioning strongly recommended."
		}
		return "USD MIXED | Gold BULLISH | Risk FX BEARISH",
			"GDP contracting — risk-off dominates. Monitor Fed pivot signals for reversal."

	case "STRESS":
		if vixElevated {
			return "Safe-haven BID (JPY, CHF, Gold) — VIX elevated",
				fmt.Sprintf("Financial stress + VIX at %.1f — defensive positioning strongly favored.", data.VIX)
		}
		return "Safe-haven BID (JPY, CHF, Gold)",
			"Financial stress elevated — defensive positioning favored. Risk-off flow into safe havens."

	case "STAGFLATION":
		return "Gold BULLISH | USD complex | Equities BEARISH",
			"Stagflation: high inflation + weak growth = gold/commodities preferred, risk assets under pressure."

	case "DETERIORATING":
		return "CAUTION — Selective risk, favor safe havens",
			"Inflation controlled but yield curve inverted + conditions tightening. Historically precedes recession. Reduce risk exposure."

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
			"Ideal backdrop: disinflation + steepening curve + loose conditions + positive growth = risk-on."

	case disinflationary && strongLabor && steepening:
		return "Risk-on bias | AUD/NZD/CAD favored | Gold neutral",
			"Healthy expansion: strong labor + steepening curve = commodity FX and equities favored."

	case restrictivePolicy && !steepening:
		return "USD BULLISH bias | Gold BEARISH | EM under pressure",
			"Tight monetary policy + flat/inverted curve = USD strength persists until pivot signal."

	default:
		return "Mixed — selective bias",
			fmt.Sprintf("Conflicting macro signals. Risk-off pressure: %d/100. Be selective.", r.Score)
	}
}

// DeriveTradingImplications returns structured per-asset signals from the macro regime.
func DeriveTradingImplications(regime MacroRegime, data *MacroData) []TradingImplication {
	var implications []TradingImplication

	// Gold
	switch {
	case regime.Name == "RECESSION" || regime.Name == "STRESS" || regime.Name == "STAGFLATION":
		implications = append(implications, TradingImplication{"Gold", "BULLISH", "🟢", "Safe haven demand tinggi di kondisi " + regimePlainName(regime.Name)})
	case regime.Name == "DETERIORATING":
		implications = append(implications, TradingImplication{"Gold", "BULLISH", "🟢", "Kurva yield terbalik + kondisi ketat — lindung nilai disarankan"})
	case regime.Name == "INFLATIONARY" && data.FedFundsRate > 0 && (data.FedFundsRate-data.Breakeven5Y) < 0:
		implications = append(implications, TradingImplication{"Gold", "BULLISH", "🟢", "Real yield negatif — gold sebagai lindung inflasi"})
	case regime.Name == "GOLDILOCKS" || regime.Name == "DISINFLATIONARY":
		if data.YieldSpread > 0.25 && data.NFCI < 0 {
			implications = append(implications, TradingImplication{"Gold", "BULLISH", "🟢", "Inflasi turun + yield curve normal — ruang untuk naik"})
		} else {
			implications = append(implications, TradingImplication{"Gold", "NEUTRAL", "🟡", "Kondisi stabil, gold cenderung sideways"})
		}
	default:
		implications = append(implications, TradingImplication{"Gold", "NEUTRAL", "🟡", "Sinyal macro campur aduk untuk gold"})
	}

	// USD
	switch {
	case regime.Name == "INFLATIONARY" && data.FedFundsRate > 0 && (data.FedFundsRate-data.Breakeven5Y) > 0.5:
		implications = append(implications, TradingImplication{"USD", "BULLISH", "🟢", "Suku bunga tinggi + inflasi tinggi = USD kuat"})
	case regime.Name == "RECESSION" || (regime.Name == "DISINFLATIONARY" && data.YieldSpread > 0.25 && data.NFCI < 0):
		implications = append(implications, TradingImplication{"USD", "BEARISH", "🔴", "Ekspektasi pemotongan suku bunga melemahkan USD"})
	case regime.Name == "STRESS":
		implications = append(implications, TradingImplication{"USD", "MIXED", "🟡", "USD bisa menguat (safe haven) atau melemah (resesi AS)"})
	default:
		implications = append(implications, TradingImplication{"USD", "NEUTRAL", "🟡", "Belum ada sinyal kuat untuk arah USD"})
	}

	// Risk FX (AUD, NZD, CAD)
	switch {
	case regime.Score < 30 && data.NFCI < 0:
		implications = append(implications, TradingImplication{"AUD/NZD/CAD", "BULLISH", "🟢", "Risk-on — kondisi finansial longgar, risiko rendah"})
	case regime.Score >= 60:
		implications = append(implications, TradingImplication{"AUD/NZD/CAD", "BEARISH", "🔴", "Risk-off — tekanan tinggi, hindari risk currency"})
	default:
		implications = append(implications, TradingImplication{"AUD/NZD/CAD", "NEUTRAL", "🟡", "Kondisi campuran — selektif, ikuti data terbaru"})
	}

	// Safe Haven (JPY, CHF)
	switch {
	case regime.Score >= 60 || regime.SahmAlert:
		implications = append(implications, TradingImplication{"JPY/CHF", "BULLISH", "🟢", "Safe haven diminati saat risiko tinggi"})
	case regime.Score < 30:
		implications = append(implications, TradingImplication{"JPY/CHF", "BEARISH", "🔴", "Risk-on — safe haven kurang diminati"})
	default:
		implications = append(implications, TradingImplication{"JPY/CHF", "NEUTRAL", "🟡", "Belum ada tekanan signifikan"})
	}

	// Equities
	switch {
	case regime.Name == "GOLDILOCKS" && regime.Score < 30:
		implications = append(implications, TradingImplication{"Equities", "BULLISH", "🟢", "Goldilocks — pertumbuhan baik, inflasi terjaga"})
	case regime.Name == "RECESSION" || regime.Name == "STAGFLATION":
		implications = append(implications, TradingImplication{"Equities", "BEARISH", "🔴", "Pertumbuhan lemah — ekuitas tertekan"})
	case regime.Name == "STRESS":
		implications = append(implications, TradingImplication{"Equities", "BEARISH", "🔴", "Stress finansial — volatilitas tinggi"})
	default:
		implications = append(implications, TradingImplication{"Equities", "NEUTRAL", "🟡", "Hati-hati, pantau data selanjutnya"})
	}

	return implications
}

// regimePlainName converts regime constant to plain Indonesian.
func regimePlainName(name string) string {
	switch name {
	case "RECESSION":
		return "resesi"
	case "STRESS":
		return "stress finansial"
	case "STAGFLATION":
		return "stagflasi"
	case "INFLATIONARY":
		return "inflasi tinggi"
	case "DISINFLATIONARY":
		return "inflasi menurun"
	case "GOLDILOCKS":
		return "ekonomi ideal"
	case "NEUTRAL":
		return "netral"
	case "DETERIORATING":
		return "memburuk"
	default:
		return strings.ToLower(name)
	}
}
