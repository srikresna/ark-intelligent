package backtest

import (
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// DefaultSpreadsBps maps currency to typical one-way spread in basis points.
// Round-trip cost = 2 × spread. Used for estimating transaction costs in backtest analysis.
var DefaultSpreadsBps = map[string]float64{
	"EUR": 1.0, // EUR/USD — tightest major
	"GBP": 1.5,
	"JPY": 1.0,
	"AUD": 2.0,
	"NZD": 3.0,
	"CAD": 2.0,
	"CHF": 2.0,
	"MXN": 5.0,
	"XAU": 3.0, // Gold
	"XAG": 5.0, // Silver
	"OIL": 3.0,
	"BTC": 10.0,
	"ETH": 15.0,
	"DXY": 2.0,
}

// SpreadBps returns the default spread for a currency, defaulting to 3 bps.
func SpreadBps(currency string) float64 {
	if s, ok := DefaultSpreadsBps[currency]; ok {
		return s
	}
	return 3.0
}

// CostAdjustedReturn subtracts round-trip spread cost from a return.
func CostAdjustedReturn(rawReturnPct float64, spreadBps float64) float64 {
	// Convert bps to pct: 3 bps = 0.03%, round-trip = 2x
	costPct := (spreadBps / 100) * 2
	return rawReturnPct - costPct
}

// CostAnalysisResult holds before/after cost comparison for a group of signals.
type CostAnalysisResult struct {
	GroupLabel     string  `json:"group_label"`
	RawAvgReturn1W float64 `json:"raw_avg_return_1w"`
	NetAvgReturn1W float64 `json:"net_avg_return_1w"`
	RawEV          float64 `json:"raw_ev"`
	NetEV          float64 `json:"net_ev"`
	AvgCostPct     float64 `json:"avg_cost_pct"` // average cost per trade (%)
	CostErasesEdge bool    `json:"cost_erases_edge"`
	Evaluated      int     `json:"evaluated"`
}

// ComputeCostAnalysis computes before/after cost returns for a set of signals.
func ComputeCostAnalysis(signals []domain.PersistedSignal, label string) *CostAnalysisResult {
	result := &CostAnalysisResult{GroupLabel: label}

	var rawSum, netSum, costSum float64
	var rawWinSum, rawLossSum, netWinSum, netLossSum float64
	var winCount, lossCount, count int

	for _, s := range signals {
		if s.Outcome1W == "" || s.Outcome1W == domain.OutcomePending || s.Outcome1W == domain.OutcomeExpired {
			continue
		}
		count++
		spread := SpreadBps(s.Currency)
		cost := (spread / 100) * 2 // round-trip

		// Use absolute return magnitude — direction is already captured in Outcome1W.
		// BEARISH wins have negative Return1W; using abs ensures correct bucketing.
		absRet := math.Abs(s.Return1W)
		rawSum += absRet
		netRet := absRet - cost
		netSum += netRet
		costSum += cost

		// Classify by Outcome1W (direction-aware), not by return sign.
		if s.Outcome1W == domain.OutcomeWin {
			winCount++
			rawWinSum += absRet
			if netRet > 0 {
				netWinSum += netRet
			} else {
				// Small win eaten by cost → net loss
				netLossSum += netRet
			}
		} else {
			lossCount++
			rawLossSum -= absRet // negative by convention
			netLossSum -= (absRet + cost)
		}
	}

	if count == 0 {
		return result
	}

	result.Evaluated = count
	result.RawAvgReturn1W = rawSum / float64(count)
	result.NetAvgReturn1W = netSum / float64(count)
	result.AvgCostPct = costSum / float64(count)

	// Expected Value: winRate × avgWin + lossRate × avgLoss
	n := float64(count)
	if winCount > 0 && lossCount > 0 {
		wc := float64(winCount)
		lc := float64(lossCount)
		result.RawEV = (wc/n)*(rawWinSum/wc) + (lc/n)*(rawLossSum/lc)
		// Net EV: re-classify wins/losses after cost deduction.
		netWinCount := 0.0
		netLossCount := 0.0
		netWS := 0.0
		netLS := 0.0
		for _, s := range signals {
			if s.Outcome1W == "" || s.Outcome1W == domain.OutcomePending || s.Outcome1W == domain.OutcomeExpired {
				continue
			}
			absR := math.Abs(s.Return1W)
			sp := SpreadBps(s.Currency)
			c := (sp / 100) * 2
			var nr float64
			if s.Outcome1W == domain.OutcomeWin {
				nr = absR - c
			} else {
				nr = -(absR + c)
			}
			if nr > 0 {
				netWinCount++
				netWS += nr
			} else {
				netLossCount++
				netLS += nr
			}
		}
		if netWinCount > 0 && netLossCount > 0 {
			result.NetEV = (netWinCount/n)*(netWS/netWinCount) + (netLossCount/n)*(netLS/netLossCount)
		} else {
			result.NetEV = result.NetAvgReturn1W
		}
	} else {
		result.RawEV = result.RawAvgReturn1W
		result.NetEV = result.NetAvgReturn1W
	}
	result.CostErasesEdge = result.RawEV > 0 && result.NetEV <= 0

	return result
}
