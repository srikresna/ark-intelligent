package ta

import (
	"math"
	"time"
)

// ---------------------------------------------------------------------------
// Backtest Types
// ---------------------------------------------------------------------------

// BacktestParams configures a walk-forward backtest of the CTA confluence strategy.
type BacktestParams struct {
	Symbol       string
	Timeframe    string  // "daily", "4h", "1h"
	WarmupBars   int     // bars needed to seed indicators (default 200)
	MaxOpenBars  int     // max bars to hold a trade (default 20)
	RiskPerTrade float64 // % of equity per trade (default 2.0)
	StartEquity  float64 // starting equity (default 10000)
	MinGrade     string  // minimum grade to trade ("A","B","C") default "C"
	MinRR        float64 // minimum R:R ratio (default 1.5)
	Slippage     float64 // slippage in price units (default 0)
	Commission   float64 // commission per trade in $ (default 0)
}

// TradeRecord represents a single completed trade in the backtest.
type TradeRecord struct {
	EntryBar   int
	ExitBar    int
	EntryDate  time.Time
	ExitDate   time.Time
	Direction  string // "LONG" or "SHORT"
	EntryPrice float64
	ExitPrice  float64
	StopLoss   float64
	TakeProfit float64
	PnLDollar  float64
	PnLPercent float64
	ExitReason string // "TP1", "SL", "TIMEOUT", "REVERSAL"
	Grade      string
	Score      float64
	RR         float64
}

// BacktestResult holds the aggregated performance metrics from a backtest run.
type BacktestResult struct {
	Params          BacktestParams
	Trades          []TradeRecord
	TotalTrades     int
	WinRate         float64 // 0-100%
	TotalPnLPercent float64
	TotalPnLDollar  float64
	MaxDrawdown     float64 // max peak-to-trough drawdown %
	SharpeRatio     float64 // annualized
	ProfitFactor    float64 // gross profit / gross loss
	AvgWin          float64 // avg winning trade %
	AvgLoss         float64 // avg losing trade %
	BestTrade       float64 // best single trade %
	WorstTrade      float64 // worst single trade %
	ConsecWins      int     // max consecutive wins
	ConsecLosses    int     // max consecutive losses
	ExpectedValue   float64 // (WinRate * AvgWin) - (LossRate * AvgLoss)
	TotalBars       int
	StartDate       time.Time
	EndDate         time.Time
	FinalEquity     float64
	EquityCurve     []float64 // equity at each trade
}

// DefaultBacktestParams returns sensible defaults for backtesting.
func DefaultBacktestParams() BacktestParams {
	return BacktestParams{
		Symbol:       "EUR",
		Timeframe:    "daily",
		WarmupBars:   200,
		MaxOpenBars:  20,
		RiskPerTrade: 2.0,
		StartEquity:  10000,
		MinGrade:     "C",
		MinRR:        1.5,
		Slippage:     0,
		Commission:   0,
	}
}

// ---------------------------------------------------------------------------
// Grade helpers
// ---------------------------------------------------------------------------

// gradeRank returns a numeric rank for a grade string. Higher = better.
func gradeRank(g string) int {
	switch g {
	case "A":
		return 4
	case "B":
		return 3
	case "C":
		return 2
	case "D":
		return 1
	default:
		return 0
	}
}

// meetsMinGrade returns true if grade meets the minimum threshold.
func meetsMinGrade(grade, minGrade string) bool {
	return gradeRank(grade) >= gradeRank(minGrade)
}

// ---------------------------------------------------------------------------
// openTrade — in-flight trade state during backtest walk
// ---------------------------------------------------------------------------

type openTrade struct {
	entryBar   int
	entryDate  time.Time
	direction  string
	entryPrice float64
	stopLoss   float64
	takeProfit float64
	grade      string
	score      float64
	rr         float64
}

// ---------------------------------------------------------------------------
// RunBacktest — Walk-forward backtester
// ---------------------------------------------------------------------------

// RunBacktest simulates the CTA confluence strategy on historical OHLCV data.
// Bars must be in newest-first order (standard OHLCV convention). The function
// reverses them internally so it can walk oldest-to-newest without lookahead.
//
// Returns nil if there is insufficient data (< WarmupBars + 10).
func RunBacktest(bars []OHLCV, params BacktestParams) *BacktestResult {
	// Apply defaults for zero-valued params
	if params.WarmupBars <= 0 {
		params.WarmupBars = 200
	}
	if params.MaxOpenBars <= 0 {
		params.MaxOpenBars = 20
	}
	if params.RiskPerTrade <= 0 {
		params.RiskPerTrade = 2.0
	}
	if params.StartEquity <= 0 {
		params.StartEquity = 10000
	}
	if params.MinGrade == "" {
		params.MinGrade = "C"
	}
	if params.MinRR <= 0 {
		params.MinRR = 1.5
	}

	minBars := params.WarmupBars + 10
	if len(bars) < minBars {
		return nil
	}

	// Convert to oldest-first for sequential walk
	asc := reverseOHLCV(bars)
	n := len(asc)

	engine := NewEngine()
	equity := params.StartEquity
	var trades []TradeRecord
	var equityCurve []float64
	var current *openTrade

	// Walk from warmup point to end
	for i := params.WarmupBars; i < n; i++ {
		// Build the "visible" history as newest-first slice (what the engine expects)
		// visible = asc[0..i] reversed → newest-first
		visibleLen := i + 1
		visible := make([]OHLCV, visibleLen)
		for j := 0; j < visibleLen; j++ {
			visible[visibleLen-1-j] = asc[j]
		}

		// Check if existing trade should be closed using current bar
		if current != nil {
			bar := asc[i]
			exitPrice := 0.0
			exitReason := ""

			barsHeld := i - current.entryBar

			if current.direction == "LONG" {
				// Check SL first (worst case), then TP
				if bar.Low <= current.stopLoss {
					exitPrice = current.stopLoss
					exitReason = "SL"
				} else if bar.High >= current.takeProfit {
					exitPrice = current.takeProfit
					exitReason = "TP1"
				}
			} else {
				// SHORT
				if bar.High >= current.stopLoss {
					exitPrice = current.stopLoss
					exitReason = "SL"
				} else if bar.Low <= current.takeProfit {
					exitPrice = current.takeProfit
					exitReason = "TP1"
				}
			}

			// Timeout check
			if exitReason == "" && barsHeld >= params.MaxOpenBars {
				exitPrice = bar.Close
				exitReason = "TIMEOUT"
			}

			// Reversal check: compute current signal
			if exitReason == "" {
				fullResult := engine.ComputeFull(visible)
				if fullResult != nil && fullResult.Confluence != nil {
					conf := fullResult.Confluence
					// Strong opposite signal (grade B+ in opposite direction)
					if current.direction == "LONG" && conf.Direction == "BEARISH" && gradeRank(conf.Grade) >= gradeRank("B") {
						exitPrice = bar.Close
						exitReason = "REVERSAL"
					} else if current.direction == "SHORT" && conf.Direction == "BULLISH" && gradeRank(conf.Grade) >= gradeRank("B") {
						exitPrice = bar.Close
						exitReason = "REVERSAL"
					}
				}
			}

			// Close the trade if exit triggered
			if exitReason != "" {
				// Apply slippage
				if current.direction == "LONG" {
					if exitReason == "SL" || exitReason == "TIMEOUT" || exitReason == "REVERSAL" {
						exitPrice -= params.Slippage
					}
				} else {
					if exitReason == "SL" || exitReason == "TIMEOUT" || exitReason == "REVERSAL" {
						exitPrice += params.Slippage
					}
				}

				// Position sizing: risk-based
				risk := math.Abs(current.entryPrice - current.stopLoss)
				if risk == 0 {
					risk = current.entryPrice * 0.01 // fallback
				}
				riskAmount := equity * params.RiskPerTrade / 100.0
				posSize := riskAmount / risk

				// PnL calculation
				dirMul := 1.0
				if current.direction == "SHORT" {
					dirMul = -1.0
				}
				rawPnL := posSize * (exitPrice - current.entryPrice) * dirMul
				pnlDollar := rawPnL - params.Commission
				pnlPercent := pnlDollar / equity * 100.0

				equity += pnlDollar

				tr := TradeRecord{
					EntryBar:   current.entryBar,
					ExitBar:    i,
					EntryDate:  current.entryDate,
					ExitDate:   bar.Date,
					Direction:  current.direction,
					EntryPrice: current.entryPrice,
					ExitPrice:  exitPrice,
					StopLoss:   current.stopLoss,
					TakeProfit: current.takeProfit,
					PnLDollar:  pnlDollar,
					PnLPercent: pnlPercent,
					ExitReason: exitReason,
					Grade:      current.grade,
					Score:      current.score,
					RR:         current.rr,
				}
				trades = append(trades, tr)
				equityCurve = append(equityCurve, equity)
				current = nil
			}
		}

		// Try to open a new trade if no position
		if current == nil && i < n-1 { // need at least one bar after entry
			fullResult := engine.ComputeFull(visible)
			if fullResult == nil || fullResult.Confluence == nil || fullResult.Zones == nil {
				continue
			}

			conf := fullResult.Confluence
			zones := fullResult.Zones

			// Check entry criteria
			if !meetsMinGrade(conf.Grade, params.MinGrade) {
				continue
			}
			if !zones.Valid {
				continue
			}
			if zones.RiskReward1 < params.MinRR {
				continue
			}

			// Entry price = midpoint of entry zone
			entryPrice := (zones.EntryHigh + zones.EntryLow) / 2.0

			// Apply slippage to entry
			if zones.Direction == "LONG" {
				entryPrice += params.Slippage
			} else {
				entryPrice -= params.Slippage
			}

			current = &openTrade{
				entryBar:   i,
				entryDate:  asc[i].Date,
				direction:  zones.Direction,
				entryPrice: entryPrice,
				stopLoss:   zones.StopLoss,
				takeProfit: zones.TakeProfit1,
				grade:      conf.Grade,
				score:      conf.Score,
				rr:         zones.RiskReward1,
			}
		}
	}

	// If still holding at end, close at last bar close
	if current != nil {
		lastBar := asc[n-1]
		risk := math.Abs(current.entryPrice - current.stopLoss)
		if risk == 0 {
			risk = current.entryPrice * 0.01
		}
		riskAmount := equity * params.RiskPerTrade / 100.0
		posSize := riskAmount / risk

		dirMul := 1.0
		if current.direction == "SHORT" {
			dirMul = -1.0
		}
		rawPnL := posSize * (lastBar.Close - current.entryPrice) * dirMul
		pnlDollar := rawPnL - params.Commission
		pnlPercent := pnlDollar / equity * 100.0
		equity += pnlDollar

		tr := TradeRecord{
			EntryBar:   current.entryBar,
			ExitBar:    n - 1,
			EntryDate:  current.entryDate,
			ExitDate:   lastBar.Date,
			Direction:  current.direction,
			EntryPrice: current.entryPrice,
			ExitPrice:  lastBar.Close,
			StopLoss:   current.stopLoss,
			TakeProfit: current.takeProfit,
			PnLDollar:  pnlDollar,
			PnLPercent: pnlPercent,
			ExitReason: "TIMEOUT",
			Grade:      current.grade,
			Score:      current.score,
			RR:         current.rr,
		}
		trades = append(trades, tr)
		equityCurve = append(equityCurve, equity)
	}

	// Compute result metrics
	return computeMetrics(trades, equityCurve, params, asc)
}

// ---------------------------------------------------------------------------
// Metrics computation
// ---------------------------------------------------------------------------

func computeMetrics(trades []TradeRecord, equityCurve []float64, params BacktestParams, asc []OHLCV) *BacktestResult {
	result := &BacktestResult{
		Params:      params,
		Trades:      trades,
		TotalTrades: len(trades),
		TotalBars:   len(asc),
		EquityCurve: equityCurve,
	}

	if len(asc) > 0 {
		result.StartDate = asc[0].Date
		result.EndDate = asc[len(asc)-1].Date
	}

	if len(trades) == 0 {
		result.FinalEquity = params.StartEquity
		return result
	}

	result.FinalEquity = equityCurve[len(equityCurve)-1]
	result.TotalPnLDollar = result.FinalEquity - params.StartEquity
	result.TotalPnLPercent = result.TotalPnLDollar / params.StartEquity * 100.0

	// Win/Loss stats
	var wins, losses int
	var grossProfit, grossLoss float64
	var sumWinPct, sumLossPct float64
	result.BestTrade = -math.MaxFloat64
	result.WorstTrade = math.MaxFloat64

	for _, t := range trades {
		if t.PnLDollar > 0 {
			wins++
			grossProfit += t.PnLDollar
			sumWinPct += t.PnLPercent
		} else {
			losses++
			grossLoss += math.Abs(t.PnLDollar)
			sumLossPct += math.Abs(t.PnLPercent)
		}
		if t.PnLPercent > result.BestTrade {
			result.BestTrade = t.PnLPercent
		}
		if t.PnLPercent < result.WorstTrade {
			result.WorstTrade = t.PnLPercent
		}
	}

	total := len(trades)
	if total > 0 {
		result.WinRate = float64(wins) / float64(total) * 100.0
	}
	if wins > 0 {
		result.AvgWin = sumWinPct / float64(wins)
	}
	if losses > 0 {
		result.AvgLoss = sumLossPct / float64(losses)
	}

	// Profit Factor
	if grossLoss > 0 {
		result.ProfitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		result.ProfitFactor = 999.0 // effectively infinite
	}

	// Expected Value
	winRate := result.WinRate / 100.0
	lossRate := 1.0 - winRate
	result.ExpectedValue = (winRate * result.AvgWin) - (lossRate * result.AvgLoss)

	// Max Drawdown
	result.MaxDrawdown = calcMaxDrawdown(equityCurve)

	// Consecutive wins/losses
	result.ConsecWins, result.ConsecLosses = calcStreaks(trades)

	// Sharpe Ratio
	result.SharpeRatio = calcSharpe(equityCurve, params.Timeframe)

	// Fix edge cases for no-trade scenarios
	if result.BestTrade == -math.MaxFloat64 {
		result.BestTrade = 0
	}
	if result.WorstTrade == math.MaxFloat64 {
		result.WorstTrade = 0
	}

	return result
}

// calcMaxDrawdown computes the maximum peak-to-trough drawdown percentage.
func calcMaxDrawdown(equityCurve []float64) float64 {
	if len(equityCurve) == 0 {
		return 0
	}
	peak := equityCurve[0]
	maxDD := 0.0
	for _, eq := range equityCurve {
		if eq > peak {
			peak = eq
		}
		dd := (peak - eq) / peak * 100.0
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// calcStreaks computes max consecutive wins and losses.
func calcStreaks(trades []TradeRecord) (maxWins, maxLosses int) {
	curWins := 0
	curLosses := 0
	for _, t := range trades {
		if t.PnLDollar > 0 {
			curWins++
			if curWins > maxWins {
				maxWins = curWins
			}
			curLosses = 0
		} else {
			curLosses++
			if curLosses > maxLosses {
				maxLosses = curLosses
			}
			curWins = 0
		}
	}
	return
}

// calcSharpe computes the annualized Sharpe ratio from the equity curve.
func calcSharpe(equityCurve []float64, timeframe string) float64 {
	if len(equityCurve) < 3 {
		return 0
	}

	// Compute returns between equity points
	returns := make([]float64, len(equityCurve)-1)
	for i := 1; i < len(equityCurve); i++ {
		if equityCurve[i-1] != 0 {
			returns[i-1] = (equityCurve[i] - equityCurve[i-1]) / equityCurve[i-1]
		}
	}

	if len(returns) == 0 {
		return 0
	}

	// Mean return
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	// Std dev
	sumSq := 0.0
	for _, r := range returns {
		diff := r - mean
		sumSq += diff * diff
	}
	stdDev := math.Sqrt(sumSq / float64(len(returns)))

	if stdDev == 0 {
		return 0
	}

	// Annualization factor
	annFactor := 252.0 // daily
	switch timeframe {
	case "4h":
		annFactor = 1460.0 // 6 bars/day * ~243 days
	case "1h":
		annFactor = 5840.0 // 24 bars/day * ~243 days
	}

	return (mean / stdDev) * math.Sqrt(annFactor)
}
