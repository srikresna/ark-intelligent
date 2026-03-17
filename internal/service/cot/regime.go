// Package cot provides rule-based market regime detection from COT data.
package cot

import (
	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// RegimeType classifies the macro risk environment.
type RegimeType string

const (
	// RegimeRiskOn — risk assets bid, safe havens sold.
	RegimeRiskOn RegimeType = "RISK-ON"
	// RegimeRiskOff — safe havens bid, risk assets sold.
	RegimeRiskOff RegimeType = "RISK-OFF"
	// RegimeUncertainty — mixed or conflicting signals.
	RegimeUncertainty RegimeType = "UNCERTAINTY"
)

// RegimeResult holds the output of the regime detection engine.
type RegimeResult struct {
	Regime      RegimeType
	Confidence  float64  // 0–100
	Factors     []string // contributing signals
	Description string
}

// DetectRegime uses rule-based logic from COT positioning data to classify
// the current macro risk environment.
//
// Safe havens: JPY, CHF, XAU (Gold)
// Risk FX:     AUD, NZD, CAD, GBP
//
// Safe haven BEARISH positioning → specs selling JPY/CHF/Gold → RISK-ON
// Safe haven BULLISH positioning → specs buying JPY/CHF/Gold → RISK-OFF
// Risk FX BULLISH positioning → specs buying AUD/NZD/CAD → RISK-ON
func DetectRegime(analyses []domain.COTAnalysis) RegimeResult {
	safehavenScore := 0.0
	riskFXScore := 0.0
	factors := []string{}

	for _, a := range analyses {
		switch a.Contract.Currency {
		case "JPY", "CHF":
			// Safe haven: bearish specs = risk-on, bullish specs = risk-off
			if a.SentimentScore < -20 {
				safehavenScore -= a.SentimentScore * 0.5 // contributes positively to risk-on
				factors = append(factors, a.Contract.Currency+" bearish (risk-on signal)")
			} else if a.SentimentScore > 20 {
				safehavenScore += a.SentimentScore * 0.5 // contributes to risk-off
				factors = append(factors, a.Contract.Currency+" bullish (risk-off signal)")
			}
		case "XAU":
			if a.SentimentScore > 30 {
				safehavenScore += a.SentimentScore * 0.3
				factors = append(factors, "Gold bullish (risk-off signal)")
			} else if a.SentimentScore < -30 {
				safehavenScore += a.SentimentScore * 0.3 // negative contribution
				factors = append(factors, "Gold bearish (risk-on signal)")
			}
		case "AUD", "NZD", "CAD":
			riskFXScore += a.SentimentScore * 0.4
			if a.SentimentScore > 20 {
				factors = append(factors, a.Contract.Currency+" bullish (risk-on signal)")
			} else if a.SentimentScore < -20 {
				factors = append(factors, a.Contract.Currency+" bearish (risk-off signal)")
			}
		}
	}

	// Classification rules
	if safehavenScore > 30 && riskFXScore < 0 {
		return RegimeResult{
			Regime:      RegimeRiskOff,
			Confidence:  70,
			Factors:     factors,
			Description: "Safe havens bid, risk FX sold — defensive positioning",
		}
	}
	if riskFXScore > 30 && safehavenScore < 0 {
		return RegimeResult{
			Regime:      RegimeRiskOn,
			Confidence:  70,
			Factors:     factors,
			Description: "Risk FX bid, safe havens sold — appetite for risk",
		}
	}
	if safehavenScore > 15 && riskFXScore > 15 {
		return RegimeResult{
			Regime:      RegimeUncertainty,
			Confidence:  40,
			Factors:     factors,
			Description: "Both safe havens and risk FX bid — confused market",
		}
	}
	if safehavenScore < -10 && riskFXScore > 10 {
		return RegimeResult{
			Regime:      RegimeRiskOn,
			Confidence:  55,
			Factors:     factors,
			Description: "Mild risk-on: risk FX favored over safe havens",
		}
	}
	if safehavenScore > 10 && riskFXScore < -10 {
		return RegimeResult{
			Regime:      RegimeRiskOff,
			Confidence:  55,
			Factors:     factors,
			Description: "Mild risk-off: safe havens favored over risk FX",
		}
	}

	return RegimeResult{
		Regime:      RegimeUncertainty,
		Confidence:  50,
		Factors:     factors,
		Description: "Mixed signals across asset classes — no clear regime",
	}
}
