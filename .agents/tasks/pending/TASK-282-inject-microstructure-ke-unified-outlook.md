# TASK-282: Inject Microstructure Signals ke UnifiedOutlookData untuk /outlook

**Priority:** medium
**Type:** feature
**Estimated:** S
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 26:00 WIB

## Deskripsi

Microstructure Engine (`internal/service/microstructure/engine.go`) sudah implement analisis Bybit orderbook depth, taker flow, dan OI momentum. Saat ini engine ini hanya dipakai via `/quant` command path, **tidak di-inject ke `UnifiedOutlookData`**, sehingga `/outlook` tidak mendapat sinyal microstructure crypto.

Microstructure signals (bid/ask imbalance, taker buy ratio, OI momentum, funding rate) adalah leading indicators penting untuk short-term crypto directional analysis.

## Perubahan yang Diperlukan

### 1. Tambah field ke `UnifiedOutlookData`

File: `internal/service/ai/unified_outlook.go`

```go
import "github.com/arkcode369/ark-intelligent/internal/service/microstructure"

type UnifiedOutlookData struct {
    // ... existing fields ...
    MicrostructureData []*microstructure.Signal // BTC, ETH microstructure
}
```

### 2. Fetch microstructure di handler.go

File: `internal/adapter/telegram/handler.go` — di sekitar line 1004 (blok assembly)

Handler perlu akses ke microstructure engine. Engine sudah ada di `h.alpha.MicroEngine` (cek `internal/adapter/telegram/handler.go:93` — `alpha` struct).

```go
// Fetch microstructure for top crypto pairs (best-effort)
var microSignals []*microstructure.Signal
if h.alpha != nil && h.alpha.MicroEngine != nil {
    for _, sym := range []string{"BTCUSDT", "ETHUSDT"} {
        if sig, err := h.alpha.MicroEngine.Analyze(ctx, "linear", sym); err == nil {
            microSignals = append(microSignals, sig)
        }
    }
}

unifiedData := aisvc.UnifiedOutlookData{
    // ... existing ...
    MicrostructureData: microSignals,
}
```

### 3. Tambah section microstructure ke `BuildUnifiedOutlookPrompt`

```go
if len(data.MicrostructureData) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. MICROSTRUCTURE (Bybit Orderbook + Flow) ===\n", section))
    section++
    for _, sig := range data.MicrostructureData {
        b.WriteString(fmt.Sprintf("%s: Bias=%s Strength=%.0f%% BidAskImbalance=%.2f TakerBuyRatio=%.0f%% OI_Δ%.1f%% Funding=%.4f%%\n",
            sig.Symbol, sig.Bias, sig.Strength*100,
            sig.BidAskImbalance, sig.TakerBuyRatio*100,
            sig.OIChange, sig.FundingRate*100))
    }
    b.WriteString("\n")
}
```

## File yang Harus Diubah

1. `internal/service/ai/unified_outlook.go` — tambah field + prompt section
2. `internal/adapter/telegram/handler.go` — fetch microstructure dan inject

## Verifikasi

```bash
go build ./...
# Manual: /outlook → cek section Microstructure muncul untuk BTC/ETH
```

## Acceptance Criteria

- [ ] `UnifiedOutlookData` memiliki field `MicrostructureData []*microstructure.Signal`
- [ ] Handler fetch BTCUSDT dan ETHUSDT microstructure sebelum build unified data
- [ ] Error fetch tidak menghentikan /outlook (best-effort, nil-safe)
- [ ] `BuildUnifiedOutlookPrompt` include section Microstructure jika data tersedia
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-26-data-sources-audit-putaran18.md` — GAP-DS3
- `internal/service/microstructure/engine.go` — Signal struct, Analyze()
- `internal/adapter/telegram/handler.go:93` — `alpha` struct yang menyimpan MicroEngine
- `internal/service/ai/unified_outlook.go:14-37` — UnifiedOutlookData struct
