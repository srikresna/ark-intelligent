package price

import (
	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// PriceCOTDivergence represents a detected divergence between price trend and COT positioning.
type PriceCOTDivergence struct {
	ContractCode string  `json:"contract_code"`
	Currency     string  `json:"currency"`
	PriceTrend   string  `json:"price_trend"`   // "UP", "DOWN", "FLAT"
	COTDirection string  `json:"cot_direction"` // "BULLISH", "BEARISH", "NEUTRAL"
	COTIndex     float64 `json:"cot_index"`
	Description  string  `json:"description"`
	Severity     string  `json:"severity"` // "HIGH", "MEDIUM", "LOW"
}

// DetectPriceCOTDivergences flags contracts where the 4-week price trend
// contradicts the COT smart money direction.
func DetectPriceCOTDivergences(
	priceContexts map[string]*domain.PriceContext,
	analyses []domain.COTAnalysis,
) []PriceCOTDivergence {
	analysisMap := make(map[string]*domain.COTAnalysis, len(analyses))
	for i := range analyses {
		analysisMap[analyses[i].Contract.Code] = &analyses[i]
	}

	var divergences []PriceCOTDivergence

	for code, pc := range priceContexts {
		analysis := analysisMap[code]
		if analysis == nil {
			continue
		}

		trend := pc.Trend4W
		cotDir := cotDirection(analysis.COTIndex)

		// Check for divergence
		var div *PriceCOTDivergence

		if trend == "UP" && cotDir == "BEARISH" {
			severity := "MEDIUM"
			if analysis.COTIndex < cotExtremeLow {
				severity = "HIGH"
			}
			div = &PriceCOTDivergence{
				ContractCode: code,
				Currency:     pc.Currency,
				PriceTrend:   trend,
				COTDirection: cotDir,
				COTIndex:     analysis.COTIndex,
				Description: pc.Currency + ": Price trending UP but COT index at " +
					formatCOTLevel(analysis.COTIndex) + " — smart money is bearish",
				Severity: severity,
			}
		} else if trend == "DOWN" && cotDir == "BULLISH" {
			severity := "MEDIUM"
			if analysis.COTIndex > cotExtremeHigh {
				severity = "HIGH"
			}
			div = &PriceCOTDivergence{
				ContractCode: code,
				Currency:     pc.Currency,
				PriceTrend:   trend,
				COTDirection: cotDir,
				COTIndex:     analysis.COTIndex,
				Description: pc.Currency + ": Price trending DOWN but COT index at " +
					formatCOTLevel(analysis.COTIndex) + " — smart money is bullish",
				Severity: severity,
			}
		}

		if div != nil {
			divergences = append(divergences, *div)
		}
	}

	return divergences
}

// COT threshold constants — mirrors cot package thresholds.
// Defined locally to avoid circular import (price → cot).
const (
	cotDirBullish  = 60
	cotDirBearish  = 40
	cotExtremeHigh = 75
	cotExtremeLow  = 25
)

func cotDirection(cotIndex float64) string {
	if cotIndex > cotDirBullish {
		return "BULLISH"
	} else if cotIndex < cotDirBearish {
		return "BEARISH"
	}
	return "NEUTRAL"
}

func formatCOTLevel(index float64) string {
	if index > 80 {
		return "extreme bullish"
	} else if index > 60 {
		return "bullish"
	} else if index < 20 {
		return "extreme bearish"
	} else if index < 40 {
		return "bearish"
	}
	return "neutral"
}
