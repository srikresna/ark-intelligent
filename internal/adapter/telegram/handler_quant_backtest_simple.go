package telegram

// handler_quant_backtest_simple.go — Simple quant backtest WITHOUT Python dependency
// This provides immediate results using basic statistical analysis on price data

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// SimpleQuantBacktestResult holds basic backtest stats
type SimpleQuantBacktestResult struct {
	Model       string
	Symbol      string
	TotalBars   int
	SignalCount int
	WinRate     float64
	AvgReturn   float64
	Sharpe      float64
	MaxDD       float64
	Confidence  float64
}

// SimpleQuantBacktestAnalyzer runs fast backtest without Python
type SimpleQuantBacktestAnalyzer struct {
	priceRepo price.DailyPriceStore
}

// NewSimpleQuantBacktestAnalyzer creates new simple analyzer
func NewSimpleQuantBacktestAnalyzer(q *QuantServices) *SimpleQuantBacktestAnalyzer {
	return &SimpleQuantBacktestAnalyzer{
		priceRepo: q.DailyPriceRepo,
	}
}

// Analyze runs fast backtest for a specific model
func (a *SimpleQuantBacktestAnalyzer) Analyze(ctx context.Context, symbol, model string) (*SimpleQuantBacktestResult, error) {
	if a.priceRepo == nil {
		return nil, fmt.Errorf("price data not configured")
	}

	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	code := mapping.ContractCode

	// Fetch historical data
	historicalData, err := a.priceRepo.GetDailyHistory(ctx, code, 500)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}

	if len(historicalData) < 100 {
		return nil, fmt.Errorf("insufficient data: %d bars (need 100+)", len(historicalData))
	}

	// Convert to OHLCV (newest-first)
	n := len(historicalData)
	bars := make([]ta.OHLCV, n)
	for i, r := range historicalData {
		bars[n-1-i] = ta.OHLCV{
			Date:  r.Date,
			Open:  r.Open,
			High:  r.High,
			Low:   r.Low,
			Close: r.Close,
		}
	}

	// Run model-specific analysis
	result, err := a.runModelAnalysis(bars, model)
	if err != nil {
		return nil, err
	}

	result.Model = strings.ToUpper(model)
	result.Symbol = symbol
	result.TotalBars = len(bars)

	return result, nil
}

// runModelAnalysis implements basic strategies for each model type
func (a *SimpleQuantBacktestAnalyzer) runModelAnalysis(bars []ta.OHLCV, model string) (*SimpleQuantBacktestResult, error) {
	result := &SimpleQuantBacktestResult{
		Model:  strings.ToUpper(model),
		Confidence: 75, // Default confidence
	}

	n := len(bars)
	if n < 50 {
		return result, fmt.Errorf("not enough bars")
	}

	// Simple moving average crossover strategy as proxy for all models
	// This is a placeholder - in production, each model would have its own logic
	shortPeriod := 10
	longPeriod := 30

	if n < longPeriod+5 {
		return result, fmt.Errorf("not enough data for moving averages")
	}

	// Calculate SMAs
	var signals []signalPoint
	var returns []float64

	for i := longPeriod; i < n-5; i += 5 { // Signal every 5 days
		shortSMA := calculateSMA(bars[i-shortPeriod:i])
		longSMA := calculateSMA(bars[i-longPeriod:i])

		// Generate signal
		direction := ""
		if shortSMA > longSMA {
			direction = "LONG"
		} else if shortSMA < longSMA {
			direction = "SHORT"
		} else {
			continue // No signal
		}

		// Evaluate after 5 days
		if i+5 >= n {
			break
		}

		entryPrice := bars[i].Close
		exitPrice := bars[i+5].Close

		var ret float64
		if direction == "LONG" {
			ret = (exitPrice - entryPrice) / entryPrice
		} else {
			ret = (entryPrice - exitPrice) / entryPrice
		}

		returns = append(returns, ret*100)
		signals = append(signals, signalPoint{
			Direction: direction,
			Return:    ret,
		})
	}

	if len(signals) < 10 {
		return result, fmt.Errorf("too few signals generated: %d", len(signals))
	}

	result.SignalCount = len(signals)

	// Calculate metrics
	var wins int
	var totalReturn float64
	for _, r := range returns {
		if r > 0 {
			wins++
		}
		totalReturn += r
	}

	result.WinRate = float64(wins) / float64(len(returns)) * 100
	result.AvgReturn = totalReturn / float64(len(returns))

	// Calculate Sharpe ratio
	if len(returns) > 1 {
		result.Sharpe = calculateSharpe(returns)
	}

	// Calculate max drawdown
	equity := 100.0
	var equityCurve []float64
	for _, sig := range signals {
		if sig.Direction == "LONG" {
			equity *= (1 + sig.Return)
		} else {
			equity *= (1 - sig.Return)
		}
		equityCurve = append(equityCurve, equity)
	}
	result.MaxDD = calculateMaxDrawdown(equityCurve)

	// Adjust confidence based on sample size
	if result.SignalCount >= 50 {
		result.Confidence = 90
	} else if result.SignalCount >= 30 {
		result.Confidence = 80
	} else if result.SignalCount >= 20 {
		result.Confidence = 70
	} else {
		result.Confidence = 60
	}

	return result, nil
}

// Helper functions
func calculateSMA(prices []ta.OHLCV) float64 {
	if len(prices) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range prices {
		sum += p.Close
	}
	return sum / float64(len(prices))
}

func calculateSharpe(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}

	// Calculate mean
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	// Calculate std dev
	variance := 0.0
	for _, r := range returns {
		diff := r - mean
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(returns)-1))

	if stdDev == 0 {
		return 0
	}

	// Annualize (assuming 5-day periods, 52 weeks/year)
	annualizedReturn := mean * 52
	annualizedStdDev := stdDev * math.Sqrt(52)

	return annualizedReturn / annualizedStdDev
}

func calculateMaxDrawdown(equityCurve []float64) float64 {
	if len(equityCurve) < 2 {
		return 0
	}

	peak := equityCurve[0]
	maxDD := 0.0

	for _, eq := range equityCurve {
		if eq > peak {
			peak = eq
		}
		dd := (peak - eq) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
	}

	return -maxDD
}

type signalPoint struct {
	Direction string
	Return    float64
}
