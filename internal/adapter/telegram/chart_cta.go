package telegram

// chart_cta.go — CTA chart generation (extracted from handler_cta.go)
// Chart types, Python chart renderer, and data-preparation utilities.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// Chart Generation
// ---------------------------------------------------------------------------

// chartInput is the JSON structure passed to the Python chart renderer.
type chartInput struct {
	Symbol     string            `json:"symbol"`
	Timeframe  string            `json:"timeframe"`
	Mode       string            `json:"mode,omitempty"`
	Bars       []chartBar        `json:"bars"`
	Indicators chartIndicators   `json:"indicators"`
	Fibonacci  chartFib          `json:"fibonacci"`
	Patterns   []chartPattern    `json:"patterns"`
	Ichimoku   *chartIchimoku    `json:"ichimoku,omitempty"`
	Zones      *chartZones       `json:"zones,omitempty"`
}

type chartIchimoku struct {
	TenkanSen  []float64 `json:"tenkan_sen"`
	KijunSen   []float64 `json:"kijun_sen"`
	SenkouSpanA []float64 `json:"senkou_span_a"`
	SenkouSpanB []float64 `json:"senkou_span_b"`
	ChikouSpan []float64 `json:"chikou_span"`
}

type chartZones struct {
	Direction   string  `json:"direction"`
	EntryHigh   float64 `json:"entry_high"`
	EntryLow    float64 `json:"entry_low"`
	StopLoss    float64 `json:"stop_loss"`
	TakeProfit1 float64 `json:"take_profit_1"`
	TakeProfit2 float64 `json:"take_profit_2"`
	RiskReward1 float64 `json:"risk_reward_1"`
	RiskReward2 float64 `json:"risk_reward_2"`
	Confidence  string  `json:"confidence"`
}

type chartBar struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type chartIndicators struct {
	EMA9          []float64 `json:"ema9,omitempty"`
	EMA21         []float64 `json:"ema21,omitempty"`
	EMA55         []float64 `json:"ema55,omitempty"`
	BBUpper       []float64 `json:"bb_upper,omitempty"`
	BBMiddle      []float64 `json:"bb_middle,omitempty"`
	BBLower       []float64 `json:"bb_lower,omitempty"`
	SuperTrend    []float64 `json:"supertrend,omitempty"`
	SuperTrendDir []string  `json:"supertrend_dir,omitempty"`
	RSI           []float64 `json:"rsi,omitempty"`
	MACD          []float64 `json:"macd,omitempty"`
	MACDSignal    []float64 `json:"macd_signal,omitempty"`
	MACDHistogram []float64 `json:"macd_histogram,omitempty"`
}

type chartFib struct {
	Levels       map[string]float64 `json:"levels,omitempty"`
	TrendDir     string             `json:"trend_dir,omitempty"`
	SwingHigh    float64            `json:"swing_high,omitempty"`
	SwingLow     float64            `json:"swing_low,omitempty"`
	SwingHighIdx int                `json:"swing_high_idx,omitempty"`
	SwingLowIdx  int                `json:"swing_low_idx,omitempty"`
}

type chartPattern struct {
	Name      string `json:"name"`
	BarIndex  int    `json:"bar_index"`
	Direction string `json:"direction"`
}

func (h *Handler) generateCTAChart(state *ctaState, timeframe string) ([]byte, error) {
	ctx := context.Background()
	bars, ok := state.bars[timeframe]
	if !ok || len(bars) == 0 {
		return nil, fmt.Errorf("no bars for timeframe %s", timeframe)
	}

	result := h.getCTAResult(state, timeframe)

	// Convert bars to chart format (bars are newest-first, chart expects oldest-first)
	n := len(bars)
	chartBars := make([]chartBar, n)
	for i, b := range bars {
		chartBars[n-1-i] = chartBar{
			Date:   b.Date.Format(time.RFC3339),
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		}
	}

	// Build indicator arrays
	ind := chartIndicators{}

	// Compute indicator series from bars
	// EMA series
	closes := make([]float64, n)
	for i, b := range bars {
		closes[i] = b.Close
	}

	ema9Series := ta.CalcEMA(closes, 9)
	ema21Series := ta.CalcEMA(closes, 21)
	ema55Series := ta.CalcEMA(closes, 55)
	if ema9Series != nil {
		ind.EMA9 = reverseToOldestFirst(ema9Series)
	}
	if ema21Series != nil {
		ind.EMA21 = reverseToOldestFirst(ema21Series)
	}
	if ema55Series != nil {
		ind.EMA55 = reverseToOldestFirst(ema55Series)
	}

	// Bollinger Bands
	bbU, bbM, bbL := ta.CalcBollingerSeries(bars, 20, 2.0)
	if bbU != nil {
		ind.BBUpper = reverseToOldestFirst(bbU)
		ind.BBMiddle = reverseToOldestFirst(bbM)
		ind.BBLower = reverseToOldestFirst(bbL)
	}

	// SuperTrend
	if result != nil && result.Snapshot != nil && result.Snapshot.SuperTrend != nil {
		st := result.Snapshot.SuperTrend
		if st.Series != nil {
			ind.SuperTrend = reverseToOldestFirst(st.Series) // NaN sanitized by reverseToOldestFirst
		}
		if st.DirectionSeries != nil {
			ind.SuperTrendDir = reverseStringsToOldestFirst(st.DirectionSeries)
		}
	}

	// RSI
	rsiSeries := ta.CalcRSISeries(bars, 14)
	if rsiSeries != nil {
		ind.RSI = reverseToOldestFirst(rsiSeries)
	}

	// MACD
	macdLine, signalLine, histogram := ta.CalcMACDSeries(bars, 12, 26, 9)
	if macdLine != nil {
		ind.MACD = reverseToOldestFirst(macdLine)
		ind.MACDSignal = reverseToOldestFirst(signalLine)
		ind.MACDHistogram = reverseToOldestFirst(histogram)
	}

	// Fibonacci levels
	fibData := chartFib{}
	if result != nil && result.Snapshot != nil && result.Snapshot.Fibonacci != nil {
		// Sanitize Fibonacci levels (remove NaN/Inf)
		sanitizedLevels := make(map[string]float64)
		for k, v := range result.Snapshot.Fibonacci.Levels {
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				sanitizedLevels[k] = v
			}
		}
		fibData.Levels = sanitizedLevels
	}

	// Patterns
	var chartPatterns []chartPattern
	if result != nil && result.Patterns != nil {
		for _, p := range result.Patterns {
			chartPatterns = append(chartPatterns, chartPattern{
				Name:      p.Name,
				BarIndex:  p.BarIndex,
				Direction: p.Direction,
			})
		}
	}

	// Build input JSON
	input := chartInput{
		Symbol:     state.symbol,
		Timeframe:  timeframe,
		Bars:       chartBars,
		Indicators: ind,
		Fibonacci:  fibData,
		Patterns:   chartPatterns,
	}

	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal chart input: %w", err)
	}

	// Write to temp file
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("cta_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("cta_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write chart input: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	// Find the script path relative to the binary
	scriptPath := findCTAScript()

	// Execute Python script with 90s timeout to prevent goroutine leaks if Python hangs.
	cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("chart renderer failed (timeout 90s): %w", err)
	}

	// Read output PNG
	pngData, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read chart output: %w", err)
	}
	if len(pngData) == 0 {
		return nil, fmt.Errorf("chart renderer produced empty output (0 bytes)")
	}

	return pngData, nil
}

// runChartScript marshals input to JSON, runs cta_chart.py, and returns PNG bytes.
func runChartScript(ctx context.Context, input any) ([]byte, error) {
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal chart input: %w", err)
	}

	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("cta_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("cta_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write chart input: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	scriptPath := findCTAScript()
	cmdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("chart renderer failed: %w", err)
	}

	pngBytes, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		return nil, fmt.Errorf("read chart output: %w", readErr)
	}
	if len(pngBytes) == 0 {
		return nil, fmt.Errorf("chart renderer produced empty output (0 bytes)")
	}
	return pngBytes, nil
}

// generateCTADetailChart creates a mode-specific chart (ichimoku, fibonacci, zones).
func (h *Handler) generateCTADetailChart(ctx context.Context, state *ctaState, timeframe string, mode string) ([]byte, error) {
	bars, ok := state.bars[timeframe]
	if !ok || len(bars) == 0 {
		// Fallback to daily
		bars, ok = state.bars["daily"]
		if !ok || len(bars) == 0 {
			return nil, fmt.Errorf("no bars available")
		}
		timeframe = "daily"
	}

	result := h.getCTAResult(state, timeframe)

	// Convert bars to chart format (newest-first → oldest-first)
	n := len(bars)
	chartBars := make([]chartBar, n)
	for i, b := range bars {
		chartBars[n-1-i] = chartBar{
			Date:   b.Date.Format(time.RFC3339),
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		}
	}

	input := chartInput{
		Symbol:    state.symbol,
		Timeframe: timeframe,
		Mode:      mode,
		Bars:      chartBars,
	}

	switch mode {
	case "ichimoku":
		series := ta.CalcIchimokuSeries(bars)
		if series != nil {
			input.Ichimoku = &chartIchimoku{
				TenkanSen:   reverseToOldestFirst(series.Tenkan),
				KijunSen:    reverseToOldestFirst(series.Kijun),
				SenkouSpanA: reverseToOldestFirst(series.SenkouA),
				SenkouSpanB: reverseToOldestFirst(series.SenkouB),
				ChikouSpan:  reverseToOldestFirst(series.Chikou),
			}
		}

	case "fibonacci":
		if result != nil && result.Snapshot != nil && result.Snapshot.Fibonacci != nil {
			fib := result.Snapshot.Fibonacci
			sanitizedLevels := make(map[string]float64)
			for k, v := range fib.Levels {
				if !math.IsNaN(v) && !math.IsInf(v, 0) {
					sanitizedLevels[k] = v
				}
			}
			input.Fibonacci = chartFib{
				Levels:       sanitizedLevels,
				TrendDir:     fib.TrendDir,
				SwingHigh:    fib.SwingHigh,
				SwingLow:     fib.SwingLow,
				SwingHighIdx: fib.SwingHighIdx,
				SwingLowIdx:  fib.SwingLowIdx,
			}
		}

	case "zones":
		if result != nil && result.Zones != nil && result.Zones.Valid {
			z := result.Zones
			input.Zones = &chartZones{
				Direction:   z.Direction,
				EntryHigh:   z.EntryHigh,
				EntryLow:    z.EntryLow,
				StopLoss:    z.StopLoss,
				TakeProfit1: z.TakeProfit1,
				TakeProfit2: z.TakeProfit2,
				RiskReward1: z.RiskReward1,
				RiskReward2: z.RiskReward2,
				Confidence:  z.Confidence,
			}
		}
	}

	return runChartScript(ctx, input)
}

// findCTAScript locates the cta_chart.py script.
func findCTAScript() string {
	candidates := []string{
		"scripts/cta_chart.py",
		"../scripts/cta_chart.py",
		"/home/mulerun/.openclaw/workspace/ark-intelligent/scripts/cta_chart.py",
	}

	// Check relative to current working dir
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	// Check relative to executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		rel := filepath.Join(execDir, "scripts", "cta_chart.py")
		if _, err := os.Stat(rel); err == nil {
			return rel
		}
		rel = filepath.Join(execDir, "..", "scripts", "cta_chart.py")
		if _, err := os.Stat(rel); err == nil {
			abs, _ := filepath.Abs(rel)
			return abs
		}
	}

	// Fallback
	return "scripts/cta_chart.py"
}

// sanitizeFloats replaces NaN and Inf with 0 so JSON marshaling works.
func sanitizeFloats(s []float64) []float64 {
	for i, v := range s {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			s[i] = 0
		}
	}
	return s
}

// reverseToOldestFirst converts a newest-first slice to oldest-first and sanitizes NaN/Inf.
func reverseToOldestFirst(s []float64) []float64 {
	n := len(s)
	out := make([]float64, n)
	for i, v := range s {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			out[n-1-i] = 0
		} else {
			out[n-1-i] = v
		}
	}
	return out
}

// reverseStringsToOldestFirst converts a newest-first string slice to oldest-first.
func reverseStringsToOldestFirst(s []string) []string {
	n := len(s)
	out := make([]string, n)
	for i, v := range s {
		out[n-1-i] = v
	}
	return out
}

// ---------------------------------------------------------------------------

// CheckPythonChartDeps verifies that the required Python packages for chart
// rendering are importable. Should be called at startup; logs a warning but
// does not fail — chart commands will gracefully degrade to text-only output
// when dependencies are missing.
//
// Required packages: mplfinance, matplotlib, numpy, pandas
func CheckPythonChartDeps() error {
	cmd := exec.Command("python3", "-c",
		"import mplfinance; import matplotlib; import numpy; import pandas")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("python chart dependencies missing (mplfinance/matplotlib/numpy/pandas): %w", err)
	}
	return nil
}
