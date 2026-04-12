package telegram

// handler_quant_backtest.go — Backtest untuk Quant/Econometric models
//   /qbacktest [SYMBOL] [MODEL] — Run backtest untuk model quant tertentu

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
)

// QuantBacktestResult menyimpan hasil backtest untuk satu model
type QuantBacktestResult struct {
	Model         string
	Symbol        string
	TotalSignals  int
	WinRate1W     float64
	WinRate2W     float64
	WinRate4W     float64
	AvgReturn1W   float64
	AvgReturn2W   float64
	AvgReturn4W   float64
	SharpeRatio   float64
	MaxDrawdown   float64
	ProfitFactor  float64
	SampleSize    int
	Confidence    float64
}

// QuantBacktestStats agregat semua model
type QuantBacktestStats struct {
	Models []QuantBacktestResult
	Symbol string
}

// QuantBacktestAnalyzer untuk compute backtest stats
type QuantBacktestAnalyzer struct {
	priceRepo      price.DailyPriceStore
	intradayRepo   price.IntradayStore
	quantServices  *QuantServices
}

// NewQuantBacktestAnalyzer create new analyzer
func NewQuantBacktestAnalyzer(q *QuantServices) *QuantBacktestAnalyzer {
	return &QuantBacktestAnalyzer{
		quantServices: q,
	}
}

// Analyze run backtest untuk semua model atau model spesifik
func (a *QuantBacktestAnalyzer) Analyze(ctx context.Context, symbol, model string) (*QuantBacktestStats, error) {
	if a.quantServices == nil || a.quantServices.DailyPriceRepo == nil {
		return nil, fmt.Errorf("price data not configured")
	}

	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	code := mapping.ContractCode

	// Fetch historical data (2 tahun untuk backtest)
	historicalData, err := a.quantServices.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
	if err != nil || len(historicalData) < 100 {
		return nil, fmt.Errorf("insufficient historical data for %s (%d bars)", symbol, len(historicalData))
	}

	// Models to backtest
	models := []string{"stats", "garch", "correlation", "regime", "meanrevert", "granger", "cointegration", "pca", "var", "risk"}
	if model != "" {
		models = []string{model}
	}

	results := make([]QuantBacktestResult, 0, len(models))

	for _, m := range models {
		result, err := a.backtestModel(ctx, code, symbol, m, historicalData)
		if err != nil {
			// Continue with other models
			continue
		}
		results = append(results, result)
	}

	return &QuantBacktestStats{
		Models: results,
		Symbol: symbol,
	}, nil
}

// backtestModel run backtest untuk satu model
func (a *QuantBacktestAnalyzer) backtestModel(ctx context.Context, code, symbol, model string, historicalData []domain.DailyPriceRecord) (QuantBacktestResult, error) {
	result := QuantBacktestResult{
		Model:  strings.ToUpper(model),
		Symbol: symbol,
	}

	// Convert to OHLCV
	n := len(historicalData)
	bars := make([]ta.OHLCV, n)
	for i, r := range historicalData {
		bars[n-1-i] = ta.OHLCV{
			Date:   r.Date,
			Open:   r.Open,
			High:   r.High,
			Low:    r.Low,
			Close:  r.Close,
			Volume: r.Volume,
		}
	}

	// Run quant model on rolling windows
	// Use 120-bar lookback, generate signal every 20 bars
	windowSize := 120
	stepSize := 20
	minSample := 30

	var signals []quantSignalPoint
	for i := windowSize; i < len(bars); i += stepSize {
		window := bars[i-windowSize : i]
		signal, err := a.runQuantModelAtPoint(ctx, symbol, model, window)
		if err != nil {
			continue
		}
		signals = append(signals, signal)
	}

	if len(signals) < minSample {
		return result, fmt.Errorf("insufficient signals (%d < %d)", len(signals), minSample)
	}

	result.TotalSignals = len(signals)

	// Compute performance metrics
	// For each signal, check return at 1W, 2W, 4W horizons
	var wins1W, wins2W, wins4W int
	var returns1W, returns2W, returns4W []float64
	var equityCurve []float64
	capital := 1000.0

	for _, sig := range signals {
		horizon := 20 // 20 trading days ~ 4 weeks
		if sig.Index+horizon >= len(bars) {
			continue
		}

		entryPrice := bars[sig.Index].Close
		exitPrice := bars[sig.Index+horizon].Close
		return4W := (exitPrice - entryPrice) / entryPrice * 100

		// 2W horizon
		horizon2W := 10
		exitPrice2W := bars[sig.Index+horizon2W].Close
		return2W := (exitPrice2W - entryPrice) / entryPrice * 100

		// 1W horizon
		horizon1W := 5
		exitPrice1W := bars[sig.Index+horizon1W].Close
		return1W := (exitPrice1W - entryPrice) / entryPrice * 100

		returns1W = append(returns1W, return1W)
		returns2W = append(returns2W, return2W)
		returns4W = append(returns4W, return4W)

		// Count wins (directional accuracy)
		if sig.Direction == "LONG" && return1W > 0 {
			wins1W++
		} else if sig.Direction == "SHORT" && return1W < 0 {
			wins1W++
		}

		if sig.Direction == "LONG" && return2W > 0 {
			wins2W++
		} else if sig.Direction == "SHORT" && return2W < 0 {
			wins2W++
		}

		if sig.Direction == "LONG" && return4W > 0 {
			wins4W++
		} else if sig.Direction == "SHORT" && return4W < 0 {
			wins4W++
		}

		// Equity curve
		if sig.Direction == "LONG" {
			capital *= (1 + return4W/100)
		} else if sig.Direction == "SHORT" {
			capital *= (1 - return4W/100)
		}
		equityCurve = append(equityCurve, capital)
	}

	totalEvaluated := len(returns1W)
	if totalEvaluated > 0 {
		result.WinRate1W = float64(wins1W) / float64(totalEvaluated) * 100
		result.WinRate2W = float64(wins2W) / float64(totalEvaluated) * 100
		result.WinRate4W = float64(wins4W) / float64(totalEvaluated) * 100
		result.AvgReturn1W = mean(returns1W)
		result.AvgReturn2W = mean(returns2W)
		result.AvgReturn4W = mean(returns4W)
	}

	result.SampleSize = totalEvaluated

	// Compute Sharpe ratio (annualized)
	if len(returns4W) > 1 {
		result.SharpeRatio = computeSharpe(returns4W)
	}

	// Compute max drawdown
	if len(equityCurve) > 1 {
		result.MaxDrawdown = computeMaxDrawdown(equityCurve)
	}

	// Compute profit factor
	grossProfit, grossLoss := computeGrossPL(returns4W)
	if grossLoss != 0 {
		result.ProfitFactor = grossProfit / -grossLoss
	}

	// Confidence based on sample size
	if result.SampleSize >= 100 {
		result.Confidence = 95
	} else if result.SampleSize >= 50 {
		result.Confidence = 85
	} else if result.SampleSize >= 30 {
		result.Confidence = 75
	} else {
		result.Confidence = 60
	}

	return result, nil
}

// quantSignalPoint represents a signal generated at a point in time
type quantSignalPoint struct {
	Index     int
	Direction string // LONG, SHORT, FLAT
	Confidence float64
}

// runQuantModelAtPoint execute quant model at specific historical point
func (a *QuantBacktestAnalyzer) runQuantModelAtPoint(ctx context.Context, symbol, model string, bars []ta.OHLCV) (quantSignalPoint, error) {
	signal := quantSignalPoint{
		Confidence: 50,
	}

	// Convert bars to JSON for Python script
	n := len(bars)
	chartBars := make([]chartBar, n)
	for i, b := range bars {
		chartBars[i] = chartBar{
			Date:   b.Date.Format("2006-01-02"),
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		}
	}

	input := quantEngineInput{
		Mode:      model,
		Symbol:    symbol,
		Timeframe: "daily",
		Bars:      chartBars,
		Params: map[string]any{
			"lookback":         120,
			"forecast_horizon": 5,
			"confidence_level": 0.95,
		},
	}

	jsonData, err := json.Marshal(input)
	if err != nil {
		return signal, fmt.Errorf("marshal input: %w", err)
	}

	tmpDir := os.TempDir()
	ts := time.Now().UnixNano()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("qbacktest_input_%d.json", ts))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("qbacktest_output_%d.json", ts))
	chartPath := filepath.Join(tmpDir, fmt.Sprintf("qbacktest_chart_%d.png", ts))

	defer func() {
		os.Remove(inputPath)
		os.Remove(outputPath)
		os.Remove(chartPath)
	}()

	if err = os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return signal, fmt.Errorf("write input: %w", err)
	}

	scriptPath, findErr := findQuantScript()
	if findErr != nil {
		return signal, findErr
	}

	// Short timeout for backtest (10s per model point)
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath, chartPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		return signal, fmt.Errorf("quant model failed: %w", err)
	}

	// Parse output
	outData, err := os.ReadFile(outputPath)
	if err != nil {
		return signal, fmt.Errorf("read output: %w", err)
	}

	var res quantEngineResult
	if err = json.Unmarshal(outData, &res); err != nil {
		return signal, fmt.Errorf("parse output: %w", err)
	}

	if !res.Success {
		return signal, fmt.Errorf("model error: %s", res.Error)
	}

	// Extract signal direction from result
	signal.Direction = extractSignalDirection(res.Result)
	if conf, ok := res.Result["confidence"].(float64); ok {
		signal.Confidence = conf
	}

	return signal, nil
}

// extractSignalDirection extracts LONG/SHORT/FLAT from model result
func extractSignalDirection(result map[string]any) string {
	// Check various fields that might contain signal direction
	if dir, ok := result["signal"].(string); ok {
		return strings.ToUpper(dir)
	}
	if dir, ok := result["direction"].(string); ok {
		return strings.ToUpper(dir)
	}
	if action, ok := result["action"].(string); ok {
		return strings.ToUpper(action)
	}

	// Fallback: check if result has bullish/bearish indicator
	if bullish, ok := result["bullish"].(bool); ok && bullish {
		return "LONG"
	}
	if bearish, ok := result["bearish"].(bool); ok && bearish {
		return "SHORT"
	}

	return "FLAT"
}

// Statistical helpers
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func computeSharpe(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	m := mean(returns)
	variance := 0.0
	for _, r := range returns {
		variance += (r - m) * (r - m)
	}
	stdDev := sqrt(variance / float64(len(returns)-1))
	if stdDev == 0 {
		return 0
	}
	// Annualize (assuming 252 trading days, 4-week returns)
	annualizedReturn := m * (252.0 / 20)
	annualizedStdDev := stdDev * sqrt(252.0/20)
	return annualizedReturn / annualizedStdDev
}

func computeMaxDrawdown(equityCurve []float64) float64 {
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

func computeGrossPL(returns []float64) (grossProfit, grossLoss float64) {
	for _, r := range returns {
		if r > 0 {
			grossProfit += r
		} else {
			grossLoss += r
		}
	}
	return
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return x * 0.5
}
