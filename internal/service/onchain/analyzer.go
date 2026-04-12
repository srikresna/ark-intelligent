package onchain

import (
	"math"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// analyzeAsset processes raw CoinMetrics data points into a summary.
// Points are expected in ascending time order (oldest first) from the CoinMetrics API.
func analyzeAsset(asset string, points []coinMetricsDataPoint) *AssetOnChainSummary {
	summary := &AssetOnChainSummary{
		Asset:     asset,
		FetchedAt: time.Now(),
		Available: true,
	}

	// Parse flows — points come oldest-first (ascending), which is chronological order.
	var flows []ExchangeFlow
	var activeAddrs []ActiveAddressMetric

	for _, dp := range points {

		t, err := time.Parse("2006-01-02T15:04:05.000000000Z", dp.Time)
		if err != nil {
			t, err = time.Parse("2006-01-02", dp.Time)
			if err != nil {
				log.Debug().Str("time", dp.Time).Msg("onchain: skipping unparseable time")
				continue
			}
		}

		flowIn := parseFloat(dp.FlowInExNtv)
		flowOut := parseFloat(dp.FlowOutExNtv)
		adrAct := parseInt(dp.AdrActCnt)
		txCnt := parseInt(dp.TxCnt)

		if flowIn > 0 || flowOut > 0 {
			flows = append(flows, ExchangeFlow{
				Date:       t,
				FlowInNtv:  flowIn,
				FlowOutNtv: flowOut,
				NetFlow:    flowIn - flowOut,
			})
		}

		if adrAct > 0 || txCnt > 0 {
			activeAddrs = append(activeAddrs, ActiveAddressMetric{
				Date:            t,
				ActiveAddresses: adrAct,
				TxCount:         txCnt,
			})
		}
	}

	summary.Flows = flows

	// Compute net flows over windows.
	if n := len(flows); n > 0 {
		summary.NetFlow7D = sumNetFlows(flows, 7)
		summary.NetFlow30D = sumNetFlows(flows, 30)
		summary.ConsecutiveOutflow = countConsecutiveOutflow(flows)
		summary.LargeInflowSpike = detectLargeInflowSpike(flows)
		summary.FlowTrend = classifyFlowTrend(summary)
	}

	// Active address metrics.
	if n := len(activeAddrs); n > 0 {
		latest := activeAddrs[n-1]
		summary.ActiveAddresses = latest.ActiveAddresses
		summary.TxCount = latest.TxCount

		if n >= 8 {
			old := activeAddrs[n-8]
			if old.ActiveAddresses > 0 {
				summary.ActiveAddrChange7D = (float64(latest.ActiveAddresses) - float64(old.ActiveAddresses)) / float64(old.ActiveAddresses) * 100
			}
		}
	}

	return summary
}

// sumNetFlows sums net flow for the last N days from chronologically-ordered flows.
func sumNetFlows(flows []ExchangeFlow, days int) float64 {
	n := len(flows)
	start := n - days
	if start < 0 {
		start = 0
	}
	var total float64
	for i := start; i < n; i++ {
		total += flows[i].NetFlow
	}
	return total
}

// countConsecutiveOutflow counts consecutive days of net outflow from the most recent day.
func countConsecutiveOutflow(flows []ExchangeFlow) int {
	count := 0
	for i := len(flows) - 1; i >= 0; i-- {
		if flows[i].NetFlow < 0 {
			count++
		} else {
			break
		}
	}
	return count
}

// detectLargeInflowSpike checks if the most recent day has inflow > 2x the 7-day average.
func detectLargeInflowSpike(flows []ExchangeFlow) bool {
	n := len(flows)
	if n < 2 {
		return false
	}

	latest := flows[n-1]

	// Compute 7-day average inflow (excluding latest).
	start := n - 8
	if start < 0 {
		start = 0
	}
	var totalIn float64
	count := 0
	for i := start; i < n-1; i++ {
		totalIn += flows[i].FlowInNtv
		count++
	}
	if count == 0 {
		return false
	}
	avgIn := totalIn / float64(count)

	return avgIn > 0 && latest.FlowInNtv > 2*avgIn
}

// classifyFlowTrend determines accumulation/distribution/neutral based on computed metrics.
func classifyFlowTrend(s *AssetOnChainSummary) string {
	// Strong accumulation: 3+ consecutive outflow days AND negative 7D net flow.
	if s.ConsecutiveOutflow >= 3 && s.NetFlow7D < 0 {
		return "ACCUMULATION"
	}
	// Distribution: large inflow spike OR positive 7D net flow.
	if s.LargeInflowSpike || s.NetFlow7D > 0 {
		return "DISTRIBUTION"
	}
	return "NEUTRAL"
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

func parseInt(s string) int64 {
	if s == "" {
		return 0
	}
	// CoinMetrics sometimes returns floats for integer metrics.
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return int64(f)
}
