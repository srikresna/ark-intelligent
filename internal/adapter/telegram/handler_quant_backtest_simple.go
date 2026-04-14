package telegram

// handler_quant_backtest_simple.go — Go-native quant backtest without Python dependency.
// Each model implements a genuinely different quantitative strategy.

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// SimpleQuantBacktestResult holds backtest statistics for one model.
type SimpleQuantBacktestResult struct {
	Model       string
	Symbol      string
	Timeframe   string
	TotalBars   int
	SignalCount int

	// Win/loss breakdown
	WinRate      float64
	LongCount    int
	ShortCount   int
	LongWins     int
	ShortWins    int
	LongWinRate  float64
	ShortWinRate float64

	// Returns
	AvgReturn    float64
	AvgWinReturn float64
	AvgLossReturn float64
	BestTrade    float64
	WorstTrade   float64
	ProfitFactor float64

	// Risk
	Sharpe     float64
	MaxDD      float64
	Confidence float64

	// Description of the signal logic
	SignalLogic string
	Criteria    string
}

// signalPoint is an internal trade record.
type signalPoint struct {
	Direction string
	Return    float64
}

// ---------------------------------------------------------------------------
// Analyzer
// ---------------------------------------------------------------------------

// SimpleQuantBacktestAnalyzer runs fast Go-native backtests.
type SimpleQuantBacktestAnalyzer struct {
	priceRepo    price.DailyPriceStore
	intradayRepo price.IntradayStore
}

// NewSimpleQuantBacktestAnalyzer creates a new analyzer from QuantServices.
func NewSimpleQuantBacktestAnalyzer(q *QuantServices) *SimpleQuantBacktestAnalyzer {
	a := &SimpleQuantBacktestAnalyzer{priceRepo: q.DailyPriceRepo}
	if q.IntradayRepo != nil {
		a.intradayRepo = q.IntradayRepo
	}
	return a
}

// Analyze runs a backtest for the given symbol/model/timeframe.
func (a *SimpleQuantBacktestAnalyzer) Analyze(ctx context.Context, symbol, model, timeframe string) (*SimpleQuantBacktestResult, error) {
	if a.priceRepo == nil {
		return nil, fmt.Errorf("price data not configured")
	}
	if timeframe == "" {
		timeframe = "daily"
	}

	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}
	code := mapping.ContractCode

	// --- Fetch bars ---
	bars, usedTF, err := a.fetchBars(ctx, code, timeframe)
	if err != nil {
		return nil, err
	}

	// --- Run model ---
	result, err := a.runModelAnalysis(bars, model, usedTF)
	if err != nil {
		return nil, err
	}
	result.Model = strings.ToUpper(model)
	result.Symbol = symbol
	result.Timeframe = usedTF
	result.TotalBars = len(bars)
	return result, nil
}

// fetchBars returns OHLCV bars oldest→newest, falling back to daily if intraday unavailable.
func (a *SimpleQuantBacktestAnalyzer) fetchBars(ctx context.Context, code, timeframe string) ([]ta.OHLCV, string, error) {
	if timeframe != "daily" && a.intradayRepo != nil {
		count := 500
		if timeframe == "15m" || timeframe == "30m" {
			count = 2000
		}
		intradayBars, iErr := a.intradayRepo.GetHistory(ctx, code, timeframe, count)
		if iErr == nil && len(intradayBars) >= 100 {
			ohlcv := ta.IntradayBarsToOHLCV(intradayBars)
			reverseOHLCV(ohlcv) // ensure oldest→newest
			return ohlcv, timeframe, nil
		}
	}
	// Fall back to daily
	records, err := a.priceRepo.GetDailyHistory(ctx, code, 500)
	if err != nil {
		return nil, "daily", fmt.Errorf("failed to fetch data: %w", err)
	}
	if len(records) < 100 {
		return nil, "daily", fmt.Errorf("insufficient data: %d bars (need 100+)", len(records))
	}
	ohlcv := ta.DailyPricesToOHLCV(records)
	reverseOHLCV(ohlcv) // ensure oldest→newest
	return ohlcv, "daily", nil
}

// reverseOHLCV reverses a slice in-place (converts newest-first to oldest-first).
func reverseOHLCV(bars []ta.OHLCV) {
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}
}

// ---------------------------------------------------------------------------
// Model dispatch
// ---------------------------------------------------------------------------

func (a *SimpleQuantBacktestAnalyzer) runModelAnalysis(bars []ta.OHLCV, model, timeframe string) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	if n < 50 {
		return nil, fmt.Errorf("not enough bars: %d (need 50+)", n)
	}

	var result *SimpleQuantBacktestResult
	var err error

	switch strings.ToLower(model) {
	case "stats":
		result, err = backtestStats(bars)
	case "garch":
		result, err = backtestGARCH(bars)
	case "corr", "correlation":
		result, err = backtestCorrelation(bars)
	case "regime":
		result, err = backtestRegime(bars)
	case "seasonal":
		result, err = backtestSeasonal(bars)
	case "meanrevert":
		result, err = backtestMeanRevert(bars)
	case "granger":
		result, err = backtestGranger(bars)
	case "coint", "cointegration":
		result, err = backtestCointegration(bars)
	case "pca":
		result, err = backtestPCA(bars)
	case "var":
		result, err = backtestVaR(bars)
	case "risk":
		result, err = backtestRisk(bars)
	default:
		return nil, fmt.Errorf("unknown model: %s", model)
	}
	if err != nil {
		return nil, err
	}

	// Attach model description
	result.SignalLogic, result.Criteria = modelDescription(strings.ToLower(model))
	return result, nil
}

// modelDescription returns human-readable description of each model's signal logic.
func modelDescription(model string) (logic, criteria string) {
	switch model {
	case "stats":
		return "Return Percentile Rank Momentum",
			"Signal ketika 5-bar return masuk top 75th percentile (LONG) atau bottom 25th percentile (SHORT) dari distribusi rolling 20-bar. Menangkap momentum persistensi."
	case "garch":
		return "Volatility Regime Breakout",
			"Membandingkan fast vol (5-bar) vs slow vol (20-bar). Fast > slow×1.3 → ikuti trend (volatility expanding). Fast < slow×0.6 → counter-trend (volatility compressing). Hold 5 bar."
	case "corr", "correlation":
		return "Multi-Timeframe Momentum Alignment",
			"Signal ketika momentum 10-bar (>0.2%) DAN momentum 20-bar keduanya searah. LONG jika keduanya positif, SHORT jika keduanya negatif. Konfirmasi alignment. Hold 7 bar."
	case "regime":
		return "SMA 50/200 Golden/Death Cross",
			"Classic trend following. LONG ketika SMA-50 menyebrangi SMA-200 dari bawah (golden cross). SHORT ketika SMA-50 turun melewati SMA-200 (death cross). Hold 10 bar."
	case "seasonal":
		return "Day-of-Week Pattern",
			"Analisis return historis per hari-dalam-seminggu (senin-jumat). Training di 50% data pertama. Signal di 50% data kedua: LONG di hari dengan avg return >0.05%, SHORT di hari <-0.05%. Hold 5 bar."
	case "meanrevert":
		return "Bollinger Band Z-Score Fade",
			"Hitung z-score: (close - 20MA) / 20-bar std. Z > 2 → SHORT (overbought). Z < -2 → LONG (oversold). Mean reversion strategy, berlawanan dengan trend. Hold 5 bar."
	case "granger":
		return "Rate-of-Change Percentile Momentum",
			"Hitung 5-bar ROC lalu ranking-nya dalam distribusi 30-bar terakhir. ROC di top 80th percentile → LONG (strong positive momentum). Bottom 20th → SHORT. Hold 5 bar."
	case "coint", "cointegration":
		return "Long-Term Mean Reversion (60-bar)",
			"Z-score terhadap rata-rata 60-bar (lebih panjang dari mean revert). Threshold 2.5 std. Z > 2.5 → SHORT. Z < -2.5 → LONG. Menangkap long-term price deviation. Hold 10 bar."
	case "pca":
		return "Multi-Factor Composite Signal",
			"Gabungan 3 faktor: 40% momentum (5-bar ROC tanh), 40% trend (distance dari SMA-50), 20% mean-reversion (inverse z-score 20MA). Composite > 0.3 → LONG, < -0.3 → SHORT. Hold 5 bar."
	case "var":
		return "Historical VaR Regime Signal",
			"Hitung 95% VaR dan CVaR dari 30-bar terakhir. LONG jika VaR-95 > -2% dan vol tidak ekstrem (< 70th pctile). SHORT jika VaR < -3% dan vol tinggi (>75th pctile). Hold 5 bar."
	case "risk":
		return "Sortino Ratio Regime",
			"Hitung rasio upside (mean positive return) / downside deviation dari 25-bar. Sortino > 1.2 → LONG (favorable risk env). Sortino < 0.5 dan downside vol tinggi → SHORT. Hold 5 bar."
	}
	return "Custom Model", "Signal logic tidak tersedia."
}

// ---------------------------------------------------------------------------
// Model 1: STATS — Return percentile-rank signal
// Signal when 5-bar return rank breaks into top/bottom quartile.
// ---------------------------------------------------------------------------

func backtestStats(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	window := 20
	holdBars := 5
	if n < window+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for stats model")
	}

	// Pre-compute all 5-bar rolling returns
	rets := make([]float64, n)
	for i := holdBars; i < n; i++ {
		if bars[i-holdBars].Close > 0 {
			rets[i] = (bars[i].Close - bars[i-holdBars].Close) / bars[i-holdBars].Close * 100
		}
	}

	var signals []signalPoint
	for i := window + holdBars; i < n-holdBars; i++ {
		// Percentile rank of current 5-bar return within rolling window
		cur := rets[i]
		window5 := rets[i-window : i]
		rank := percentileRank(window5, cur)

		direction := ""
		if rank >= 75 {
			direction = "LONG" // momentum continuation
		} else if rank <= 25 {
			direction = "SHORT" // momentum continuation short
		}
		if direction == "" {
			continue
		}

		// Entry at close[i], exit after holdBars
		if i+holdBars >= n {
			break
		}
		entry := bars[i].Close
		exit := bars[i+holdBars].Close
		ret := returnPct(entry, exit, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 75.0), nil
}

// ---------------------------------------------------------------------------
// Model 2: GARCH — Volatility regime signal (fast vol vs slow vol)
// Buy breakouts when fast vol expands above slow vol; fade when compressing.
// ---------------------------------------------------------------------------

func backtestGARCH(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	fastPeriod := 5
	slowPeriod := 20
	holdBars := 5
	if n < slowPeriod+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for GARCH model")
	}

	logRets := computeLogReturns(bars)
	var signals []signalPoint

	for i := slowPeriod; i < n-holdBars; i++ {
		fastVol := stdDev(logRets[i-fastPeriod : i])
		slowVol := stdDev(logRets[i-slowPeriod : i])
		if slowVol == 0 {
			continue
		}
		ratio := fastVol / slowVol
		// Trend direction: last 5 bars
		trend := ""
		if bars[i].Close > bars[i-5].Close {
			trend = "LONG"
		} else {
			trend = "SHORT"
		}

		direction := ""
		if ratio > 1.3 {
			direction = trend // vol expanding — ride the trend
		} else if ratio < 0.6 {
			// vol compression — mean reversion
			if trend == "LONG" {
				direction = "SHORT"
			} else {
				direction = "LONG"
			}
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 72.0), nil
}

// ---------------------------------------------------------------------------
// Model 3: CORRELATION — Short/Long momentum alignment
// Signal when 10-bar and 20-bar momentum agree in direction.
// ---------------------------------------------------------------------------

func backtestCorrelation(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	shortPeriod := 10
	longPeriod := 20
	holdBars := 7
	if n < longPeriod+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for correlation model")
	}

	var signals []signalPoint
	for i := longPeriod; i < n-holdBars; i++ {
		mom10 := (bars[i].Close - bars[i-shortPeriod].Close) / bars[i-shortPeriod].Close
		mom20 := (bars[i].Close - bars[i-longPeriod].Close) / bars[i-longPeriod].Close

		// Both momentums agree AND signal is strong enough
		direction := ""
		if mom10 > 0.002 && mom20 > 0 {
			direction = "LONG"
		} else if mom10 < -0.002 && mom20 < 0 {
			direction = "SHORT"
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 70.0), nil
}

// ---------------------------------------------------------------------------
// Model 4: REGIME — Trend following (50 SMA vs 200 SMA)
// Classic golden/death cross with trend confirmation.
// ---------------------------------------------------------------------------

func backtestRegime(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	fastMA := 50
	slowMA := 200
	holdBars := 10
	if n < slowMA+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for regime model (need 200+)")
	}

	var signals []signalPoint
	for i := slowMA; i < n-holdBars; i++ {
		sma50 := sma(bars, i, fastMA)
		sma200 := sma(bars, i, slowMA)
		prevSMA50 := sma(bars, i-1, fastMA)
		prevSMA200 := sma(bars, i-1, slowMA)

		direction := ""
		// Golden cross: SMA50 crosses above SMA200
		if prevSMA50 <= prevSMA200 && sma50 > sma200 {
			direction = "LONG"
		}
		// Death cross: SMA50 crosses below SMA200
		if prevSMA50 >= prevSMA200 && sma50 < sma200 {
			direction = "SHORT"
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 68.0), nil
}

// ---------------------------------------------------------------------------
// Model 5: SEASONAL — Day-of-week / time patterns
// Signal based on historically best-performing weekdays.
// ---------------------------------------------------------------------------

func backtestSeasonal(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	holdBars := 5
	if n < 60 {
		return nil, fmt.Errorf("not enough bars for seasonal model")
	}

	// Compute average return by weekday using first half of data
	midpoint := n / 2
	dayReturns := make(map[int][]float64) // weekday → returns
	for i := 1; i < midpoint; i++ {
		if bars[i-1].Close > 0 {
			ret := (bars[i].Close - bars[i-1].Close) / bars[i-1].Close * 100
			wd := int(bars[i].Date.Weekday())
			dayReturns[wd] = append(dayReturns[wd], ret)
		}
	}

	// Compute mean per weekday
	dayAvg := make(map[int]float64)
	for wd, rets := range dayReturns {
		dayAvg[wd] = qbtMean(rets)
	}

	// Signal on second half: buy on best day of week, sell on worst
	var signals []signalPoint
	for i := midpoint; i < n-holdBars; i++ {
		wd := int(bars[i].Date.Weekday())
		avg, ok := dayAvg[wd]
		if !ok {
			continue
		}

		direction := ""
		if avg > 0.05 { // historically positive day → LONG
			direction = "LONG"
		} else if avg < -0.05 { // historically negative day → SHORT
			direction = "SHORT"
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 65.0), nil
}

// ---------------------------------------------------------------------------
// Model 6: MEANREVERT — Bollinger Band z-score
// Fade extremes: z > 2 → SHORT; z < -2 → LONG.
// ---------------------------------------------------------------------------

func backtestMeanRevert(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	period := 20
	holdBars := 5
	threshold := 2.0
	if n < period+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for mean revert model")
	}

	var signals []signalPoint
	for i := period; i < n-holdBars; i++ {
		window := closePrices(bars, i-period, i)
		mu := qbtMean(window)
		sd := stdDevSlice(window)
		if sd == 0 {
			continue
		}
		z := (bars[i].Close - mu) / sd

		direction := ""
		if z > threshold {
			direction = "SHORT" // overbought → fade
		} else if z < -threshold {
			direction = "LONG" // oversold → buy
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 78.0), nil
}

// ---------------------------------------------------------------------------
// Model 7: GRANGER — Rate-of-change momentum
// Signal when 5-bar ROC is in top/bottom quartile of recent distribution.
// ---------------------------------------------------------------------------

func backtestGranger(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	rocPeriod := 5
	windowSize := 30
	holdBars := 5
	if n < windowSize+rocPeriod+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for granger model")
	}

	// Pre-compute ROC
	roc := make([]float64, n)
	for i := rocPeriod; i < n; i++ {
		if bars[i-rocPeriod].Close > 0 {
			roc[i] = (bars[i].Close-bars[i-rocPeriod].Close)/bars[i-rocPeriod].Close*100
		}
	}

	var signals []signalPoint
	for i := windowSize + rocPeriod; i < n-holdBars; i++ {
		curROC := roc[i]
		windowROC := roc[i-windowSize : i]
		rank := percentileRank(windowROC, curROC)

		direction := ""
		if rank >= 80 { // strong positive momentum
			direction = "LONG"
		} else if rank <= 20 { // strong negative momentum
			direction = "SHORT"
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 73.0), nil
}

// ---------------------------------------------------------------------------
// Model 8: COINTEGRATION — Long-term mean reversion (60-bar mean)
// Fade price when it deviates 2.5+ std from 60-bar mean.
// ---------------------------------------------------------------------------

func backtestCointegration(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	period := 60
	holdBars := 10
	threshold := 2.5
	if n < period+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for cointegration model")
	}

	var signals []signalPoint
	for i := period; i < n-holdBars; i++ {
		window := closePrices(bars, i-period, i)
		mu := qbtMean(window)
		sd := stdDevSlice(window)
		if sd == 0 {
			continue
		}
		z := (bars[i].Close - mu) / sd

		direction := ""
		if z > threshold {
			direction = "SHORT"
		} else if z < -threshold {
			direction = "LONG"
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 71.0), nil
}

// ---------------------------------------------------------------------------
// Model 9: PCA — Multi-factor composite signal
// Combines momentum (ROC), trend (vs 50MA), and mean-reversion (z-score).
// ---------------------------------------------------------------------------

func backtestPCA(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	maPeriod := 50
	zPeriod := 20
	rocPeriod := 5
	holdBars := 5
	if n < maPeriod+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for PCA model")
	}

	var signals []signalPoint
	for i := maPeriod; i < n-holdBars; i++ {
		// Factor 1: ROC momentum (normalised to [-1,1])
		roc := 0.0
		if bars[i-rocPeriod].Close > 0 {
			roc = (bars[i].Close-bars[i-rocPeriod].Close)/bars[i-rocPeriod].Close*100
		}
		f1 := math.Tanh(roc / 0.5) // scale: 0.5% ROC → tanh(1)

		// Factor 2: Trend (above/below 50 MA)
		ma50 := sma(bars, i, maPeriod)
		f2 := 0.0
		if ma50 > 0 {
			f2 = math.Tanh((bars[i].Close - ma50) / ma50 * 100)
		}

		// Factor 3: Mean-reversion (inverted z-score vs 20MA)
		window := closePrices(bars, i-zPeriod, i)
		mu := qbtMean(window)
		sd := stdDevSlice(window)
		f3 := 0.0
		if sd > 0 {
			z := (bars[i].Close - mu) / sd
			f3 = -math.Tanh(z / 2) // invert: overbought → negative score
		}

		// Composite: 40% momentum, 40% trend, 20% mean-rev
		composite := 0.4*f1 + 0.4*f2 + 0.2*f3

		direction := ""
		if composite > 0.3 {
			direction = "LONG"
		} else if composite < -0.3 {
			direction = "SHORT"
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 74.0), nil
}

// ---------------------------------------------------------------------------
// Model 10: VAR — Historical VaR regime signal
// Signal when historical tail-risk environment is favourable.
// ---------------------------------------------------------------------------

func backtestVaR(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	window := 30
	holdBars := 5
	if n < window+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for VaR model")
	}

	logRets := computeLogReturns(bars)
	var signals []signalPoint

	for i := window + 1; i < n-holdBars; i++ {
		recent := logRets[i-window : i]
		sorted := make([]float64, len(recent))
		copy(sorted, recent)
		sortFloat64s(sorted)

		// 95% VaR = 5th percentile of returns
		idx := int(0.05 * float64(len(sorted)))
		var95 := sorted[idx]

		// Expected shortfall (CVaR): avg of worst 5%
		cvar := qbtMean(sorted[:idx+1])

		// Current vol percentile
		curVol := math.Abs(logRets[i])
		volRank := percentileRank(recent, curVol)

		direction := ""
		// If recent VaR is not too bad (> -2%) and vol is not extreme → LONG
		if var95 > -0.02 && cvar > -0.03 && volRank < 70 {
			direction = "LONG"
		} else if var95 < -0.03 && volRank > 75 {
			// High tail risk → SHORT
			direction = "SHORT"
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 76.0), nil
}

// ---------------------------------------------------------------------------
// Model 11: RISK — Sortino / downside-deviation regime
// Signal when downside risk is low relative to recent history.
// ---------------------------------------------------------------------------

func backtestRisk(bars []ta.OHLCV) (*SimpleQuantBacktestResult, error) {
	n := len(bars)
	window := 25
	holdBars := 5
	if n < window+holdBars+5 {
		return nil, fmt.Errorf("not enough bars for risk model")
	}

	logRets := computeLogReturns(bars)
	var signals []signalPoint

	for i := window + 1; i < n-holdBars; i++ {
		recent := logRets[i-window : i]

		// Downside deviation: std of negative returns only
		var negReturns []float64
		for _, r := range recent {
			if r < 0 {
				negReturns = append(negReturns, r)
			}
		}
		downsideVol := 0.0
		if len(negReturns) > 3 {
			downsideVol = stdDevSlice(negReturns)
		}

		// Upside capture: mean of positive returns
		var posReturns []float64
		for _, r := range recent {
			if r > 0 {
				posReturns = append(posReturns, r)
			}
		}
		upsideAvg := 0.0
		if len(posReturns) > 0 {
			upsideAvg = qbtMean(posReturns)
		}

		// Sortino-like ratio: upside avg / downside vol
		sortino := 0.0
		if downsideVol > 0 {
			sortino = upsideAvg / downsideVol
		}

		direction := ""
		if sortino > 1.2 { // favourable upside/downside ratio → LONG
			direction = "LONG"
		} else if sortino < 0.5 && downsideVol > 0.01 { // high downside risk → SHORT
			direction = "SHORT"
		}
		if direction == "" {
			continue
		}

		if i+holdBars >= n {
			break
		}
		ret := returnPct(bars[i].Close, bars[i+holdBars].Close, direction)
		signals = append(signals, signalPoint{direction, ret})
	}

	return computeBacktestStats(signals, 72.0), nil
}

// ---------------------------------------------------------------------------
// Metrics computation
// ---------------------------------------------------------------------------

func computeBacktestStats(signals []signalPoint, baseConf float64) *SimpleQuantBacktestResult {
	r := &SimpleQuantBacktestResult{}
	if len(signals) < 5 {
		r.Confidence = 50
		return r
	}

	r.SignalCount = len(signals)
	returns := make([]float64, len(signals))
	var wins int
	var total, winTotal, lossTotal float64
	var winCount, lossCount int
	best, worst := -1e9, 1e9

	for i, s := range signals {
		returns[i] = s.Return
		total += s.Return
		if s.Return > best {
			best = s.Return
		}
		if s.Return < worst {
			worst = s.Return
		}
		if s.Return > 0 {
			wins++
			winTotal += s.Return
			winCount++
		} else {
			lossTotal += s.Return
			lossCount++
		}
		if s.Direction == "LONG" {
			r.LongCount++
			if s.Return > 0 {
				r.LongWins++
			}
		} else {
			r.ShortCount++
			if s.Return > 0 {
				r.ShortWins++
			}
		}
	}

	r.WinRate = float64(wins) / float64(len(signals)) * 100
	r.AvgReturn = total / float64(len(signals))
	r.BestTrade = best
	r.WorstTrade = worst

	if winCount > 0 {
		r.AvgWinReturn = winTotal / float64(winCount)
	}
	if lossCount > 0 {
		r.AvgLossReturn = lossTotal / float64(lossCount)
	}
	if r.LongCount > 0 {
		r.LongWinRate = float64(r.LongWins) / float64(r.LongCount) * 100
	}
	if r.ShortCount > 0 {
		r.ShortWinRate = float64(r.ShortWins) / float64(r.ShortCount) * 100
	}
	if lossTotal != 0 {
		r.ProfitFactor = math.Abs(winTotal / lossTotal)
	}

	r.Sharpe = calculateSharpe(returns)
	r.MaxDD = calculateMaxDrawdown(buildEquityCurve(signals))

	// Confidence based on sample size
	switch {
	case r.SignalCount >= 50:
		r.Confidence = baseConf
	case r.SignalCount >= 30:
		r.Confidence = baseConf - 5
	case r.SignalCount >= 20:
		r.Confidence = baseConf - 10
	default:
		r.Confidence = baseConf - 15
	}
	if r.Confidence < 50 {
		r.Confidence = 50
	}
	return r
}

func buildEquityCurve(signals []signalPoint) []float64 {
	equity := 100.0
	curve := make([]float64, len(signals))
	for i, s := range signals {
		equity *= (1 + s.Return/100)
		curve[i] = equity
	}
	return curve
}

// ---------------------------------------------------------------------------
// Statistical helpers
// ---------------------------------------------------------------------------

func closePrices(bars []ta.OHLCV, from, to int) []float64 {
	out := make([]float64, to-from)
	for i := from; i < to; i++ {
		out[i-from] = bars[i].Close
	}
	return out
}

func computeLogReturns(bars []ta.OHLCV) []float64 {
	n := len(bars)
	rets := make([]float64, n)
	for i := 1; i < n; i++ {
		if bars[i-1].Close > 0 {
			rets[i] = math.Log(bars[i].Close / bars[i-1].Close)
		}
	}
	return rets
}

func sma(bars []ta.OHLCV, endIdx, period int) float64 {
	if endIdx < period {
		return 0
	}
	sum := 0.0
	for i := endIdx - period; i < endIdx; i++ {
		sum += bars[i].Close
	}
	return sum / float64(period)
}

func qbtMean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stdDevSlice(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := qbtMean(vals)
	sum := 0.0
	for _, v := range vals {
		d := v - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(vals)-1))
}

func stdDev(vals []float64) float64 {
	return stdDevSlice(vals)
}

func percentileRank(data []float64, val float64) float64 {
	if len(data) == 0 {
		return 50
	}
	var below int
	for _, v := range data {
		if v < val {
			below++
		}
	}
	return float64(below) / float64(len(data)) * 100
}

func returnPct(entry, exit float64, direction string) float64 {
	if entry == 0 {
		return 0
	}
	if direction == "LONG" {
		return (exit - entry) / entry * 100
	}
	return (entry - exit) / entry * 100
}

// sortFloat64s is a simple insertion sort for small slices (VaR computation).
func sortFloat64s(vals []float64) {
	for i := 1; i < len(vals); i++ {
		key := vals[i]
		j := i - 1
		for j >= 0 && vals[j] > key {
			vals[j+1] = vals[j]
			j--
		}
		vals[j+1] = key
	}
}

func calculateSharpe(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	m := qbtMean(returns)
	sd := stdDevSlice(returns)
	if sd == 0 {
		return 0
	}
	// Annualise with ~52 periods per year (5-day hold)
	return (m * 52) / (sd * math.Sqrt(52))
}

func calculateMaxDrawdown(equity []float64) float64 {
	if len(equity) < 2 {
		return 0
	}
	peak := equity[0]
	maxDD := 0.0
	for _, e := range equity {
		if e > peak {
			peak = e
		}
		if peak > 0 {
			dd := (peak - e) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return -maxDD
}
