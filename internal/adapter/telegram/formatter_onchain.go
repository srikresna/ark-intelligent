package telegram

import (
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/onchain"
)

// formatOnChainReport builds the Telegram HTML output for the /onchain command.
func formatOnChainReport(report *onchain.OnChainReport, btcHealth *onchain.BTCNetworkHealth) string {
	var sb strings.Builder
	sb.WriteString("⛓ <b>On-Chain Metrics Dashboard</b>\n\n")

	// Section 1: BTC Network Health (Blockchain.com).
	if btcHealth != nil && btcHealth.Available {
		sb.WriteString(formatBTCNetworkHealth(btcHealth))
		sb.WriteString("\n")
	}

	// Section 2: Exchange flows (CoinMetrics).
	if report != nil && report.Available && len(report.Assets) > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("📊 <b>Exchange Flows</b>\n")
		sb.WriteString(fmt.Sprintf("📅 %s\n\n", report.FetchedAt.UTC().Format("2006-01-02 15:04 UTC")))

		assetOrder := []string{"btc", "eth"}
		for _, asset := range assetOrder {
			s, ok := report.Assets[asset]
			if !ok || !s.Available {
				continue
			}
			sb.WriteString(formatAssetOnChain(s))
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("\n📊 Exchange flow data not available.\n")
	}

	sb.WriteString("💡 <i>Negative net flow = coins leaving exchanges (accumulation)\nPositive net flow = coins entering exchanges (sell pressure)</i>\n")
	sb.WriteString("📊 Data: Blockchain.com + CoinMetrics Community API")

	return truncateMsg(sb.String())
}

// formatBTCNetworkHealth formats the BTC network health section.
func formatBTCNetworkHealth(h *onchain.BTCNetworkHealth) string {
	var sb strings.Builder

	sb.WriteString("₿ <b>BTC Network Health</b>\n")
	sb.WriteString(fmt.Sprintf("📅 %s\n\n", h.FetchedAt.UTC().Format("2006-01-02 15:04 UTC")))

	// Price.
	if h.MarketPriceUSD > 0 {
		sb.WriteString(fmt.Sprintf("  💰 Price: $%s\n", formatCompactNumber(h.MarketPriceUSD)))
	}

	// Hash rate.
	if h.HashRate > 0 {
		hashStr := formatHashRate(h.HashRate)
		sb.WriteString(fmt.Sprintf("  ⛏ Hash Rate: %s", hashStr))
		if h.HashRateChange != 0 {
			arrow := "📈"
			if h.HashRateChange < 0 {
				arrow = "📉"
			}
			sb.WriteString(fmt.Sprintf(" (%s %.1f%% 7d)", arrow, h.HashRateChange))
		}
		sb.WriteString("\n")
	}

	// Miner capitulation alert.
	if h.MinerCapitulation {
		sb.WriteString("  ⚠️ <b>Miner Capitulation Signal</b> (hash rate -10%+ 7d)\n")
	}

	// Difficulty.
	if h.Difficulty > 0 {
		sb.WriteString(fmt.Sprintf("  🔐 Difficulty: %sT\n", formatCompactNumber(h.Difficulty/1e12)))
	}

	// Mempool.
	if h.MempoolBytes > 0 || h.MempoolTxCount > 0 {
		mempoolMB := float64(h.MempoolBytes) / (1024 * 1024)
		sb.WriteString(fmt.Sprintf("  📦 Mempool: %.1f MB", mempoolMB))
		if h.MempoolTxCount > 0 {
			sb.WriteString(fmt.Sprintf(" (%s txs)", formatCompactNumber(float64(h.MempoolTxCount))))
		}
		sb.WriteString("\n")
		if h.MempoolCongested {
			sb.WriteString("  🚨 <b>Mempool Congested</b> (>100 MB)\n")
		}
	}

	// Fees.
	if h.TotalFeesBTC > 0 {
		sb.WriteString(fmt.Sprintf("  💸 Fees (24h): %.2f BTC", h.TotalFeesBTC))
		if h.FeeSpike {
			sb.WriteString(" ⚠️ <b>Fee Surge</b> (>2x avg)")
		}
		sb.WriteString("\n")
	}

	// Transactions.
	if h.NTx24H > 0 {
		sb.WriteString(fmt.Sprintf("  📝 Transactions (24h): %s\n", formatCompactNumber(float64(h.NTx24H))))
	}

	return sb.String()
}

func formatAssetOnChain(s *onchain.AssetOnChainSummary) string {
	var sb strings.Builder

	assetEmoji := "₿"
	if s.Asset == "eth" {
		assetEmoji = "Ξ"
	}

	trendEmoji := "➖"
	switch s.FlowTrend {
	case "ACCUMULATION":
		trendEmoji = "🟢 Accumulation"
	case "DISTRIBUTION":
		trendEmoji = "🔴 Distribution"
	case "NEUTRAL":
		trendEmoji = "⚪ Neutral"
	}

	sb.WriteString(fmt.Sprintf("%s <b>%s Exchange Flows</b>\n", assetEmoji, strings.ToUpper(s.Asset)))
	sb.WriteString(fmt.Sprintf("  Trend: %s\n", trendEmoji))
	sb.WriteString(fmt.Sprintf("  Net Flow 7D: %s\n", formatFlowValue(s.NetFlow7D, s.Asset)))
	sb.WriteString(fmt.Sprintf("  Net Flow 30D: %s\n", formatFlowValue(s.NetFlow30D, s.Asset)))

	if s.ConsecutiveOutflow > 0 {
		sb.WriteString(fmt.Sprintf("  🔄 %d consecutive outflow days\n", s.ConsecutiveOutflow))
	}
	if s.LargeInflowSpike {
		sb.WriteString("  ⚠️ Large inflow spike detected (>2x avg)\n")
	}

	if s.ActiveAddresses > 0 {
		sb.WriteString(fmt.Sprintf("  👥 Active Addresses: %s", formatCompactNumber(float64(s.ActiveAddresses))))
		if s.ActiveAddrChange7D != 0 {
			arrow := "📈"
			if s.ActiveAddrChange7D < 0 {
				arrow = "📉"
			}
			sb.WriteString(fmt.Sprintf(" (%s %.1f%%)", arrow, s.ActiveAddrChange7D))
		}
		sb.WriteString("\n")
	}

	if s.TxCount > 0 {
		sb.WriteString(fmt.Sprintf("  📝 Transactions: %s\n", formatCompactNumber(float64(s.TxCount))))
	}

	return sb.String()
}

func formatFlowValue(val float64, asset string) string {
	sign := "+"
	if val < 0 {
		sign = ""
	}
	absVal := math.Abs(val)

	unit := strings.ToUpper(asset)
	if absVal >= 1000 {
		return fmt.Sprintf("%s%.1fK %s", sign, val/1000, unit)
	}
	return fmt.Sprintf("%s%.1f %s", sign, val, unit)
}

func formatCompactNumber(n float64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", n/1_000)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}

// formatHashRate formats hash rate in human-readable units.
func formatHashRate(thps float64) string {
	switch {
	case thps >= 1e6:
		return fmt.Sprintf("%.1f EH/s", thps/1e6)
	case thps >= 1e3:
		return fmt.Sprintf("%.1f PH/s", thps/1e3)
	default:
		return fmt.Sprintf("%.1f TH/s", thps)
	}
}
