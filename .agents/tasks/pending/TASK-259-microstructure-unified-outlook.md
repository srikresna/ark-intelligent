# TASK-259: Microstructure Bybit (BTC/ETH) → UnifiedOutlookData Integration

**Priority:** low
**Type:** data-source
**Estimated:** S
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

Package `internal/service/microstructure/` sudah sepenuhnya diimplementasi:
- Orderbook depth imbalance (bid/ask heavy)
- Taker buy ratio dari recent trades
- Open Interest momentum (24h OI change %)
- Funding rate
- Long/Short ratio
- Derived directional `Bias` (BULLISH/BEARISH/NEUTRAL/CONFLICT)

**Masalah:** Service ini hanya dipakai di `/alpha` command (`handler_alpha.go`),
tidak masuk ke `UnifiedOutlookData` untuk `/outlook` command.

Saat ini unified_outlook untuk crypto pair (BTCUSD, ETHUSD) hanya pakai price context
(OHLCV + TA indicators). Microstructure menambahkan layer "real-time order flow" yang
sangat relevan sebagai confirming signal untuk direction AI recommendation.

**Data source:** Bybit public API (gratis, no key needed) — sudah dipakai di `/alpha`.

## File yang Harus Diubah

1. `internal/service/ai/unified_outlook.go` — tambah field + section
2. `internal/adapter/telegram/handler.go` — fetch microstructure BTC/ETH, inject

## Implementasi

### 1. unified_outlook.go — tambah field

Import:
```go
microsvc "github.com/arkcode369/ark-intelligent/internal/service/microstructure"
```

Di `UnifiedOutlookData` struct:
```go
// MicrostructureSignals holds Bybit real-time crypto microstructure signals.
// Key = symbol (e.g. "BTCUSDT"), Value = microstructure signal.
MicrostructureSignals map[string]*microsvc.Signal
```

Di `BuildUnifiedOutlookPrompt()`, tambah section setelah Price section atau Sentiment:
```go
if len(data.MicrostructureSignals) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. CRYPTO MICROSTRUCTURE (Bybit Real-time) ===\n", section))
    section++
    for sym, sig := range data.MicrostructureSignals {
        b.WriteString(fmt.Sprintf("%s: Bias=%s | BidAsk=%.3f | TakerBuy=%.1f%% | " +
            "OI Δ=%.2f%% | Funding=%.4f | L/S=%.2f\n",
            sym, sig.Bias,
            sig.BidAskImbalance,
            sig.TakerBuyRatio*100,
            sig.OIChange,
            sig.FundingRate,
            sig.LongShortRatio))
    }
    b.WriteString("NOTE: Microstructure signals are real-time; use as short-term " +
        "confirming signal only.\n\n")
}
```

### 2. handler.go — fetch microstructure BTC/ETH saat /outlook

Di `cmdOutlook`, sebelum construct unifiedData:
```go
var microSignals map[string]*microsvc.Signal

// Fetch microstructure for BTC and ETH (crypto pairs in user's watchlist or default)
if h.microEngine != nil {
    microSignals = make(map[string]*microsvc.Signal)
    cryptoSymbols := []string{"BTCUSDT", "ETHUSDT"}
    for _, sym := range cryptoSymbols {
        sig, err := h.microEngine.Analyze(ctx, "linear", sym)
        if err == nil && sig != nil {
            microSignals[sym] = sig
        }
    }
    if len(microSignals) == 0 {
        microSignals = nil // don't include empty section
    }
}
```

Inject ke unifiedData:
```go
unifiedData := aisvc.UnifiedOutlookData{
    // ... existing ...
    MicrostructureSignals: microSignals,
}
```

**Note:** `h.microEngine` sudah ada di Handler melalui `handler_alpha.go` infrastructure.
Cek apakah `h.alpha` sudah punya `MicroEngine` yang bisa diakses, atau perlu expose
field terpisah di Handler.

Dari `handler_alpha.go:36`:
```go
type AlphaServices struct {
    MicroEngine    *microstructure.Engine
    ...
}
```
Jadi bisa diakses via `h.alpha.MicroEngine` — tidak perlu field baru.

## Acceptance Criteria

- [ ] `UnifiedOutlookData` punya field `MicrostructureSignals map[string]*microsvc.Signal`
- [ ] `/outlook` prompt includes "CRYPTO MICROSTRUCTURE" section saat signals tersedia
- [ ] BTCUSDT dan ETHUSDT diambil saat cmdOutlook
- [ ] Section shows: Bias, BidAskImbalance, TakerBuyRatio, OI Δ, Funding, L/S ratio
- [ ] Jika `h.alpha == nil` atau `MicroEngine == nil` → skip gracefully, tidak crash
- [ ] Jika Bybit fetch gagal untuk satu symbol → symbol di-skip, yang lain tetap jalan
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-09-data-sources-audit-gaps-putaran13.md` — Temuan #2
- `internal/service/microstructure/engine.go:1` — Engine struct, Analyze() method
- `internal/service/microstructure/engine.go:29` — Signal struct (fields)
- `internal/adapter/telegram/handler_alpha.go:36` — AlphaServices.MicroEngine
- `internal/adapter/telegram/handler.go:139` — h.gex (pola referensi untuk optional service)
- `internal/service/ai/unified_outlook.go:22` — UnifiedOutlookData struct
- `internal/adapter/telegram/handler.go:1004` — unifiedData construction point
