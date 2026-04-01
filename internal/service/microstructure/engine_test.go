package microstructure

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/bybit"
)

// ---------------------------------------------------------------------------
// computeOrderbookImbalance
// ---------------------------------------------------------------------------

func TestComputeOrderbookImbalance_NilOrderbook(t *testing.T) {
	got := computeOrderbookImbalance(nil)
	if got != 0 {
		t.Errorf("expected 0 for nil orderbook, got %f", got)
	}
}

func TestComputeOrderbookImbalance_EmptyOrderbook(t *testing.T) {
	ob := &bybit.Orderbook{}
	got := computeOrderbookImbalance(ob)
	if got != 0 {
		t.Errorf("expected 0 for empty orderbook, got %f", got)
	}
}

func TestComputeOrderbookImbalance_BidHeavy(t *testing.T) {
	ob := &bybit.Orderbook{
		Bids: []bybit.OrderbookLevel{
			{Price: 100, Quantity: 80},
			{Price: 99, Quantity: 20},
		},
		Asks: []bybit.OrderbookLevel{
			{Price: 101, Quantity: 10},
		},
	}
	got := computeOrderbookImbalance(ob)
	// Bids=100, Asks=10 → imbalance = (100-10)/110 = 0.818...
	if got <= 0 {
		t.Errorf("expected positive imbalance for bid-heavy book, got %f", got)
	}
	if got > 1.0 || got < -1.0 {
		t.Errorf("imbalance out of [-1,1] range: %f", got)
	}
}

func TestComputeOrderbookImbalance_AskHeavy(t *testing.T) {
	ob := &bybit.Orderbook{
		Bids: []bybit.OrderbookLevel{
			{Price: 100, Quantity: 5},
		},
		Asks: []bybit.OrderbookLevel{
			{Price: 101, Quantity: 95},
		},
	}
	got := computeOrderbookImbalance(ob)
	if got >= 0 {
		t.Errorf("expected negative imbalance for ask-heavy book, got %f", got)
	}
}

func TestComputeOrderbookImbalance_Balanced(t *testing.T) {
	ob := &bybit.Orderbook{
		Bids: []bybit.OrderbookLevel{{Price: 100, Quantity: 50}},
		Asks: []bybit.OrderbookLevel{{Price: 101, Quantity: 50}},
	}
	got := computeOrderbookImbalance(ob)
	if got != 0 {
		t.Errorf("expected 0 for balanced book, got %f", got)
	}
}

func TestComputeOrderbookImbalance_OnlyTop10Levels(t *testing.T) {
	// Build 15 bid levels and 1 ask level — only top 10 bids should be counted.
	bids := make([]bybit.OrderbookLevel, 15)
	for i := range bids {
		bids[i] = bybit.OrderbookLevel{Price: float64(100 - i), Quantity: 1}
	}
	asks := []bybit.OrderbookLevel{{Price: 101, Quantity: 5}}
	ob := &bybit.Orderbook{Bids: bids, Asks: asks}

	got := computeOrderbookImbalance(ob)
	// Only 10 bid units (not 15) should be counted → bidVol=10, askVol=5
	want := (10.0 - 5.0) / (10.0 + 5.0) // = 0.333...
	const tol = 1e-9
	if got < want-tol || got > want+tol {
		t.Errorf("expected %f (top-10 only), got %f", want, got)
	}
}

// ---------------------------------------------------------------------------
// computeTakerBuyRatio
// ---------------------------------------------------------------------------

func TestComputeTakerBuyRatio_EmptyTrades(t *testing.T) {
	got := computeTakerBuyRatio(nil)
	if got != 0.5 {
		t.Errorf("expected 0.5 for empty trades, got %f", got)
	}
}

func TestComputeTakerBuyRatio_AllBuys(t *testing.T) {
	trades := []bybit.Trade{
		{Qty: 10, IsBuyTaker: true},
		{Qty: 5, IsBuyTaker: true},
	}
	got := computeTakerBuyRatio(trades)
	if got != 1.0 {
		t.Errorf("expected 1.0 for all buys, got %f", got)
	}
}

func TestComputeTakerBuyRatio_AllSells(t *testing.T) {
	trades := []bybit.Trade{
		{Qty: 10, IsBuyTaker: false},
		{Qty: 20, IsBuyTaker: false},
	}
	got := computeTakerBuyRatio(trades)
	if got != 0.0 {
		t.Errorf("expected 0.0 for all sells, got %f", got)
	}
}

func TestComputeTakerBuyRatio_Mixed(t *testing.T) {
	trades := []bybit.Trade{
		{Qty: 60, IsBuyTaker: true},
		{Qty: 40, IsBuyTaker: false},
	}
	got := computeTakerBuyRatio(trades)
	want := 0.6
	const tol = 1e-9
	if got < want-tol || got > want+tol {
		t.Errorf("expected %f, got %f", want, got)
	}
}

// ---------------------------------------------------------------------------
// computeOIChange
// ---------------------------------------------------------------------------

func TestComputeOIChange_EmptySlice(t *testing.T) {
	got := computeOIChange(nil)
	if got != 0 {
		t.Errorf("expected 0 for nil OI data, got %f", got)
	}
}

func TestComputeOIChange_SingleItem(t *testing.T) {
	ois := []bybit.OIData{{OpenInterest: 1000}}
	got := computeOIChange(ois)
	if got != 0 {
		t.Errorf("expected 0 for single OI point, got %f", got)
	}
}

func TestComputeOIChange_Increasing(t *testing.T) {
	// ois[0] = newest, ois[N-1] = oldest (Bybit convention)
	ois := []bybit.OIData{
		{OpenInterest: 1100}, // newest
		{OpenInterest: 1000}, // oldest
	}
	got := computeOIChange(ois)
	want := 10.0 // (1100-1000)/1000 * 100 = 10%
	const tol = 1e-9
	if got < want-tol || got > want+tol {
		t.Errorf("expected %f%%, got %f%%", want, got)
	}
}

func TestComputeOIChange_Decreasing(t *testing.T) {
	ois := []bybit.OIData{
		{OpenInterest: 900},  // newest
		{OpenInterest: 1000}, // oldest
	}
	got := computeOIChange(ois)
	want := -10.0
	const tol = 1e-9
	if got < want-tol || got > want+tol {
		t.Errorf("expected %f%%, got %f%%", want, got)
	}
}

func TestComputeOIChange_ZeroOldest(t *testing.T) {
	ois := []bybit.OIData{
		{OpenInterest: 1000},
		{OpenInterest: 0}, // would cause division by zero
	}
	got := computeOIChange(ois)
	if got != 0 {
		t.Errorf("expected 0 when oldest OI is zero, got %f", got)
	}
}

// ---------------------------------------------------------------------------
// deriveBias
// ---------------------------------------------------------------------------

func TestDeriveBias_NeutralSignal(t *testing.T) {
	sig := &Signal{
		BidAskImbalance: 0,
		TakerBuyRatio:   0.5,
		OIChange:        0,
		FundingRate:     0,
		LongShortRatio:  1.0,
	}
	bias, strength := deriveBias(sig)
	if bias != BiasNeutral {
		t.Errorf("expected NEUTRAL bias, got %s", bias)
	}
	if strength != 0 {
		t.Errorf("expected 0 strength for neutral, got %f", strength)
	}
}

func TestDeriveBias_StrongBullish(t *testing.T) {
	sig := &Signal{
		BidAskImbalance: 0.80, // strong bid side
		TakerBuyRatio:   0.75, // heavy buy taker
		OIChange:        10,   // OI expanding
		FundingRate:     0,
		LongShortRatio:  1.5, // more longs
	}
	bias, strength := deriveBias(sig)
	if bias != BiasBullish {
		t.Errorf("expected BULLISH, got %s", bias)
	}
	if strength <= 0 || strength > 1 {
		t.Errorf("strength %f out of (0,1] for bullish signal", strength)
	}
}

func TestDeriveBias_StrongBearish(t *testing.T) {
	sig := &Signal{
		BidAskImbalance: -0.80, // heavy ask side
		TakerBuyRatio:   0.25,  // mostly sell takers
		OIChange:        -10,   // OI contracting
		FundingRate:     0,
		LongShortRatio:  0.6, // more shorts
	}
	bias, strength := deriveBias(sig)
	if bias != BiasBearish {
		t.Errorf("expected BEARISH, got %s", bias)
	}
	if strength <= 0 || strength > 1 {
		t.Errorf("strength %f out of (0,1] for bearish signal", strength)
	}
}

func TestDeriveBias_StrengthBoundedAtOne(t *testing.T) {
	sig := &Signal{
		BidAskImbalance: 0.99,
		TakerBuyRatio:   0.99,
		OIChange:        50,
		FundingRate:     -0.02, // negative funding → bullish boost
		LongShortRatio:  2.0,
	}
	_, strength := deriveBias(sig)
	if strength > 1.0 {
		t.Errorf("strength must be ≤1.0, got %f", strength)
	}
}

func TestDeriveBias_ConflictWhenBalanced(t *testing.T) {
	// Slight bid imbalance (bullish) countered by taker sell pressure (bearish)
	sig := &Signal{
		BidAskImbalance: 0.50, // bullish
		TakerBuyRatio:   0.20, // bearish
		OIChange:        0,
		FundingRate:     0,
		LongShortRatio:  1.0,
	}
	bias, _ := deriveBias(sig)
	// Should be CONFLICT or one side winning — just ensure no panic and valid bias
	validBiases := map[Bias]bool{
		BiasBullish:  true,
		BiasBearish:  true,
		BiasNeutral:  true,
		BiasConflict: true,
	}
	if !validBiases[bias] {
		t.Errorf("unexpected bias value: %s", bias)
	}
}
