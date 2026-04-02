package ta

import "time"

// ---------------------------------------------------------------------------
// Full Result (single timeframe)
// ---------------------------------------------------------------------------

// FullResult bundles the complete technical analysis for a single timeframe:
// all indicators, confluence scoring, and entry/exit zones.
type FullResult struct {
	Snapshot    *IndicatorSnapshot
	Confluence  *ConfluenceResult
	Zones       *ZoneResult
	Patterns    []CandlePattern   // from patterns.go
	Divergences []Divergence      // from divergence.go
	ICT         *ICTResult        // from ict.go — nil if insufficient data
	SMC         *SMCResult        // convenience accessor — same as Snapshot.SMC
	Wyckoff     *WyckoffSummary   // populated by caller (avoids circular import with wyckoff pkg)
	ComputedAt  time.Time
}

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

// Engine is the main orchestrator for technical analysis computations.
type Engine struct{}

// NewEngine creates a new TA engine.
func NewEngine() *Engine {
	return &Engine{}
}

// ---------------------------------------------------------------------------
// ComputeSnapshot — all indicators for one set of OHLCV bars
// ---------------------------------------------------------------------------

// ComputeSnapshot calculates all available indicators for the given bars.
// Bars must be newest-first (index 0 = most recent bar).
// Individual indicators that lack sufficient data will be nil.
func (e *Engine) ComputeSnapshot(bars []OHLCV) *IndicatorSnapshot {
	if len(bars) == 0 {
		return &IndicatorSnapshot{}
	}

	snap := &IndicatorSnapshot{}
	snap.CurrentPrice = bars[0].Close

	// ATR(14) — used by zones for position sizing
	snap.ATR = CalcATR(bars, 14)

	// RSI(14)
	snap.RSI = CalcRSI(bars, 14)

	// MACD(12, 26, 9)
	snap.MACD = CalcMACD(bars, 12, 26, 9)

	// Stochastic(14, 3, 3)
	snap.Stochastic = CalcStochastic(bars, 14, 3, 3)

	// Bollinger Bands(20, 2)
	snap.Bollinger = CalcBollinger(bars, 20, 2.0)

	// EMA Ribbon(9, 21, 55, 100, 200)
	snap.EMA = CalcEMARibbon(bars, []int{9, 21, 55, 100, 200})

	// ADX(14)
	snap.ADX = CalcADX(bars, 14)

	// OBV
	snap.OBV = CalcOBV(bars)

	// Williams %R(14)
	snap.WilliamsR = CalcWilliamsR(bars, 14)

	// CCI(20)
	snap.CCI = CalcCCI(bars, 20)

	// MFI(14)
	snap.MFI = CalcMFI(bars, 14)

	// Advanced indicators
	snap.Ichimoku = CalcIchimoku(bars)
	snap.SuperTrend = CalcSuperTrend(bars, 10, 3.0)
	snap.Fibonacci = CalcFibonacci(bars, 50)

	// Killzone: classify current trading session
	kz := ClassifyKillzone(time.Now())
	snap.Killzone = &kz


	// VWAP: anchored volume-weighted average price (needs volume data)
	if hasVolume(bars) {
		snap.VWAP = CalcVWAPSet(bars)
	}

	// Delta: tick-rule estimated cumulative buy/sell pressure
	if len(bars) >= 2 {
		snap.Delta = CalcDelta(bars)
	}

	// SMC: Smart Money Concepts (BOS, CHOCH, premium/discount zones)
	if len(bars) >= 20 && snap.ATR > 0 {
		snap.SMC = CalcSMC(bars, snap.ATR)
	}

	// Wyckoff phase detection (simplified ta-level analysis)
	if len(bars) >= 50 && snap.ATR > 0 {
		snap.Wyckoff = CalcWyckoff(bars, snap.ATR)
	}
	return snap
}

// ---------------------------------------------------------------------------
// ComputeFull — snapshot + confluence + zones for one timeframe
// ---------------------------------------------------------------------------

// ComputeFull calculates the complete technical analysis for a single set of
// OHLCV bars: indicators, confluence scoring, and entry/exit zones.
func (e *Engine) ComputeFull(bars []OHLCV) *FullResult {
	snap := e.ComputeSnapshot(bars)
	conf := CalcConfluence(snap)
	zones := CalcZones(snap, conf)

	// Candlestick patterns
	patterns := DetectPatterns(bars)

	// Divergences (need RSI and MACD series)
	var divergences []Divergence
	rsiSeries := CalcRSISeries(bars, 14)
	macdLine, _, _ := CalcMACDSeries(bars, 12, 26, 9)
	if rsiSeries != nil || macdLine != nil {
		divergences = DetectDivergences(bars, rsiSeries, macdLine)
	}

	return &FullResult{
		Snapshot:    snap,
		Confluence:  conf,
		Zones:       zones,
		Patterns:    patterns,
		Divergences: divergences,
		ICT:         CalcICT(bars, snap.ATR),
		SMC:         snap.SMC,
		ComputedAt:  time.Now(),
	}
}

// ---------------------------------------------------------------------------
// ComputeMTF — multi-timeframe analysis
// ---------------------------------------------------------------------------

// ComputeMTF calculates multi-timeframe analysis from OHLCV data keyed by
// timeframe name. Each timeframe's bars are run through ComputeFull, then
// the resulting confluence scores are aggregated by CalcMTF.
func (e *Engine) ComputeMTF(barsByTF map[string][]OHLCV) *MTFResult {
	snapshots := make(map[string]*IndicatorSnapshot, len(barsByTF))
	for tf, bars := range barsByTF {
		snap := e.ComputeSnapshot(bars)
		snap.Timeframe = tf
		snapshots[tf] = snap
	}
	return CalcMTF(snapshots)
}
