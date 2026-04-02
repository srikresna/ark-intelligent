package defi

import (
	"fmt"
	"math"
)

// analyzeSignals detects market signals from the DeFi report.
func analyzeSignals(r *DeFiReport) []DeFiSignal {
	var signals []DeFiSignal

	// TVL drop >5% in 24h = risk-off
	if r.TVLChange24h < -5 {
		signals = append(signals, DeFiSignal{
			Type:     "risk_off",
			Message:  fmt.Sprintf("⚠️ TVL turun %.1f%% dalam 24 jam — potensi risk-off DeFi", r.TVLChange24h),
			Severity: "alert",
		})
	} else if r.TVLChange24h < -2 {
		signals = append(signals, DeFiSignal{
			Type:     "tvl_decline",
			Message:  fmt.Sprintf("📉 TVL turun %.1f%% dalam 24 jam", r.TVLChange24h),
			Severity: "warning",
		})
	} else if r.TVLChange24h > 5 {
		signals = append(signals, DeFiSignal{
			Type:     "tvl_surge",
			Message:  fmt.Sprintf("📈 TVL naik %.1f%% dalam 24 jam — capital inflow kuat", r.TVLChange24h),
			Severity: "info",
		})
	}

	// Stablecoin supply growth = incoming liquidity
	if r.StablecoinChange7D > 2 {
		signals = append(signals, DeFiSignal{
			Type:     "liquidity_inflow",
			Message:  fmt.Sprintf("💰 Stablecoin supply naik %.1f%% dalam 7 hari — incoming liquidity", r.StablecoinChange7D),
			Severity: "info",
		})
	} else if r.StablecoinChange7D < -2 {
		signals = append(signals, DeFiSignal{
			Type:     "liquidity_outflow",
			Message:  fmt.Sprintf("🔻 Stablecoin supply turun %.1f%% dalam 7 hari — liquidity keluar", r.StablecoinChange7D),
			Severity: "warning",
		})
	}

	// DEX volume surge >50% = high activity
	if r.DEX.Change24h > 50 {
		signals = append(signals, DeFiSignal{
			Type:     "dex_surge",
			Message:  fmt.Sprintf("🔥 DEX volume melonjak %.0f%% — aktivitas trading tinggi", r.DEX.Change24h),
			Severity: "warning",
		})
	} else if r.DEX.Change24h < -50 {
		signals = append(signals, DeFiSignal{
			Type:     "dex_collapse",
			Message:  fmt.Sprintf("💤 DEX volume turun %.0f%% — aktivitas rendah", math.Abs(r.DEX.Change24h)),
			Severity: "info",
		})
	}

	if len(signals) == 0 {
		signals = append(signals, DeFiSignal{
			Type:     "neutral",
			Message:  "✅ DeFi metrics stabil — tidak ada anomali signifikan",
			Severity: "info",
		})
	}

	return signals
}
