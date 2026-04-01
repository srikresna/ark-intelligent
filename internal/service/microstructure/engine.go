// Package microstructure provides a lightweight crypto microstructure engine
// using Bybit public API data: orderbook depth, taker flow, and OI momentum.
package microstructure

import (
	"context"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/bybit"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("microstructure")

// Signal is the actionable microstructure signal for a symbol.
type Signal struct {
	Symbol      string
	Category    string // "linear", "spot"
	BidAskImbalance float64 // positive = bid heavy, negative = ask heavy
	TakerBuyRatio   float64 // fraction of recent trades that are taker buys (0-1)
	OIChange        float64 // open interest change % over recent period
	FundingRate     float64 // current funding rate (decimal, e.g. 0.0001)
	LongShortRatio  float64 // longs / shorts ratio (>1 = more longs)
	Bias            Bias    // derived directional bias
	ConfirmEntry    bool    // true = microstructure confirms a directional entry
	Strength        float64 // 0-1 strength of the signal
	FundingStats    *bybit.FundingRateStats // historical funding rate analysis (nil if unavailable)
	UpdatedAt       time.Time
}

// Bias is the directional bias from microstructure.
type Bias string

const (
	BiasBullish  Bias = "BULLISH"
	BiasBearish  Bias = "BEARISH"
	BiasNeutral  Bias = "NEUTRAL"
	BiasConflict Bias = "CONFLICT" // mixed signals
)

// Engine fetches and processes Bybit microstructure data.
type Engine struct {
	client *bybit.Client
}

// NewEngine creates a microstructure engine using a Bybit client.
func NewEngine(client *bybit.Client) *Engine {
	return &Engine{client: client}
}

// Analyze computes microstructure metrics for a symbol.
// category: "linear" for perpetuals, "spot" for spot.
func (e *Engine) Analyze(ctx context.Context, category, symbol string) (*Signal, error) {
	sig := &Signal{
		Symbol:    symbol,
		Category:  category,
		UpdatedAt: time.Now(),
	}

	// --- 1. Orderbook depth imbalance ---
	ob, err := e.client.GetOrderbook(ctx, category, symbol, 50)
	if err != nil {
		log.Warn().Str("symbol", symbol).Err(err).Msg("microstructure: orderbook fetch failed")
	} else {
		sig.BidAskImbalance = computeOrderbookImbalance(ob)
	}

	// --- 2. Taker flow from recent trades ---
	trades, err := e.client.GetRecentTrades(ctx, category, symbol, 500)
	if err != nil {
		log.Warn().Str("symbol", symbol).Err(err).Msg("microstructure: trades fetch failed")
	} else {
		sig.TakerBuyRatio = computeTakerBuyRatio(trades)
	}

	// --- 3. Open Interest momentum (only for perpetuals) ---
	if category == "linear" {
		oi, err := e.client.GetOpenInterestHistory(ctx, category, symbol, "1h", 24)
		if err != nil {
			log.Warn().Str("symbol", symbol).Err(err).Msg("microstructure: OI fetch failed")
		} else {
			sig.OIChange = computeOIChange(oi)
		}
	}

	// --- 4. Long/Short ratio ---
	if category == "linear" {
		lsRatios, err := e.client.GetLongShortRatio(ctx, category, symbol, "1h", 24)
		if err != nil {
			log.Warn().Str("symbol", symbol).Err(err).Msg("microstructure: long-short ratio fetch failed")
		} else if len(lsRatios) > 0 {
			sig.LongShortRatio = lsRatios[0].BuyRatio / (lsRatios[0].SellRatio + 1e-10)
		}
	}

	// --- 5. Funding rate from ticker ---
	if category == "linear" {
		ticker, err := e.client.GetTicker(ctx, category, symbol)
		if err != nil {
			log.Warn().Str("symbol", symbol).Err(err).Msg("microstructure: ticker fetch failed")
		} else {
			sig.FundingRate = ticker.FundingRate
		}
	}

	// --- 5b. Funding rate history (for regime analysis) ---
	if category == "linear" {
		rates, err := e.client.GetFundingHistory(ctx, category, symbol, 200)
		if err != nil {
			log.Warn().Str("symbol", symbol).Err(err).Msg("microstructure: funding history fetch failed")
		} else if len(rates) > 0 {
			sig.FundingStats = bybit.ComputeFundingStats(symbol, rates)
		}
	}

	// --- 6. Derive bias ---
	sig.Bias, sig.Strength = deriveBias(sig)
	sig.ConfirmEntry = sig.Strength >= config.MicroConfirmEntryThreshold

	return sig, nil
}

// AnalyzeMultiple analyzes multiple symbols and returns results.
func (e *Engine) AnalyzeMultiple(ctx context.Context, category string, symbols []string) (map[string]*Signal, error) {
	results := make(map[string]*Signal, len(symbols))
	var lastErr error
	for _, sym := range symbols {
		sig, err := e.Analyze(ctx, category, sym)
		if err != nil {
			lastErr = fmt.Errorf("microstructure: %s: %w", sym, err)
			continue
		}
		results[sym] = sig
	}
	return results, lastErr
}

// ---------------------------------------------------------------------------
// Computation helpers
// ---------------------------------------------------------------------------

// computeOrderbookImbalance returns bid/ask imbalance from -1 (ask heavy) to +1 (bid heavy).
// Uses top-10 levels weighted by distance from mid.
func computeOrderbookImbalance(ob *bybit.Orderbook) float64 {
	if ob == nil || (len(ob.Bids) == 0 && len(ob.Asks) == 0) {
		return 0
	}

	bidVol := 0.0
	for i, b := range ob.Bids {
		if i >= 10 {
			break
		}
		bidVol += b.Quantity
	}
	askVol := 0.0
	for i, a := range ob.Asks {
		if i >= 10 {
			break
		}
		askVol += a.Quantity
	}

	total := bidVol + askVol
	if total == 0 {
		return 0
	}
	return (bidVol - askVol) / total // [-1, +1]
}

// computeTakerBuyRatio returns fraction of recent trades that are aggressive buys (0-1).
// > 0.55 = buy pressure, < 0.45 = sell pressure.
func computeTakerBuyRatio(trades []bybit.Trade) float64 {
	if len(trades) == 0 {
		return 0.5
	}
	buyVol := 0.0
	totalVol := 0.0
	for _, t := range trades {
		totalVol += t.Qty
		if t.IsBuyTaker {
			buyVol += t.Qty
		}
	}
	if totalVol == 0 {
		return 0.5
	}
	return buyVol / totalVol
}

// computeOIChange returns the % change in open interest from oldest to newest in the slice.
func computeOIChange(ois []bybit.OIData) float64 {
	if len(ois) < 2 {
		return 0
	}
	// ois[0] = most recent (Bybit returns newest first)
	newest := ois[0].OpenInterest
	oldest := ois[len(ois)-1].OpenInterest
	if oldest == 0 {
		return 0
	}
	return (newest - oldest) / oldest * 100
}

// deriveBias synthesizes microstructure signals into a directional bias.
func deriveBias(sig *Signal) (Bias, float64) {
	bullish := 0.0
	bearish := 0.0

	// Orderbook imbalance
	if sig.BidAskImbalance > 0.10 {
		bullish += sig.BidAskImbalance
	} else if sig.BidAskImbalance < -0.10 {
		bearish += -sig.BidAskImbalance
	}

	// Taker flow
	if sig.TakerBuyRatio > 0.55 {
		bullish += (sig.TakerBuyRatio - 0.5) * 2
	} else if sig.TakerBuyRatio < 0.45 {
		bearish += (0.5 - sig.TakerBuyRatio) * 2
	}

	// OI expanding with price — trend confirmation
	if sig.OIChange > 5 { // OI growing > 5%
		bullish += 0.20
	} else if sig.OIChange < -5 {
		bearish += 0.20
	}

	// Funding rate: positive = longs paying, market bullish but potentially overextended
	if sig.FundingRate > 0.01 { // > 1bps
		// slight bearish lean (crowded long)
		bearish += 0.10
	} else if sig.FundingRate < -0.005 {
		// negative funding = bearish sentiment but possible squeeze setup
		bullish += 0.10
	}

	// Long/Short ratio
	if sig.LongShortRatio > 1.2 {
		bullish += 0.15
	} else if sig.LongShortRatio < 0.8 {
		bearish += 0.15
	}

	if bullish == 0 && bearish == 0 {
		return BiasNeutral, 0
	}

	total := bullish + bearish
	if total == 0 {
		return BiasNeutral, 0
	}

	// Determine bias and strength
	if bullish > bearish*1.3 {
		strength := bullish / (total) // fraction of bullish evidence
		if strength > 1 {
			strength = 1
		}
		return BiasBullish, strength
	}
	if bearish > bullish*1.3 {
		strength := bearish / total
		if strength > 1 {
			strength = 1
		}
		return BiasBearish, strength
	}
	return BiasConflict, 0.3
}
