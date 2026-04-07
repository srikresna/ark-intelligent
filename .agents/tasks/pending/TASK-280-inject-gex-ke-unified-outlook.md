# TASK-280: Inject GEX Data ke UnifiedOutlookData untuk /outlook

**Priority:** high
**Type:** feature
**Estimated:** S
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 26:00 WIB

## Deskripsi

GEX Engine (`internal/service/gex/engine.go`) sudah implement analisis Gamma Exposure via Deribit public API (gratis, no key). Saat ini hanya dipakai di `/gex` command (`handler_gex.go`), tapi **tidak di-inject ke `UnifiedOutlookData`** sehingga `/outlook` tidak mendapat gamma exposure context.

GEX adalah data kritikal untuk crypto analysis: net GEX positif = gamma dealer hedging mengurangi volatilitas; negatif = dealer hedging memperbesar volatilitas.

## Perubahan yang Diperlukan

### 1. Tambah field ke `UnifiedOutlookData`

File: `internal/service/ai/unified_outlook.go`

```go
import "github.com/arkcode369/ark-intelligent/internal/service/gex"

type UnifiedOutlookData struct {
    // ... existing fields ...
    GEXData map[string]*gex.GEXResult // "BTC" → GEXResult, "ETH" → GEXResult
}
```

### 2. Fetch GEX di handler sebelum build unified data

File: `internal/adapter/telegram/handler.go` — di sekitar line 1004 (blok assembly unified data)

```go
// Fetch GEX for BTC and ETH (best-effort, non-blocking)
var gexData map[string]*gexsvc.GEXResult
if h.gex != nil {
    gexData = make(map[string]*gexsvc.GEXResult)
    for _, sym := range []string{"BTC", "ETH"} {
        if result, err := h.gex.Engine.Analyze(ctx, sym); err == nil {
            gexData[sym] = result
        }
    }
}

unifiedData := aisvc.UnifiedOutlookData{
    // ... existing fields ...
    GEXData: gexData,
}
```

### 3. Tambah section GEX ke `BuildUnifiedOutlookPrompt`

File: `internal/service/ai/unified_outlook.go` — di `BuildUnifiedOutlookPrompt`

```go
if len(data.GEXData) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. GAMMA EXPOSURE (GEX) ===\n", section))
    section++
    for sym, g := range data.GEXData {
        b.WriteString(fmt.Sprintf("%s: Net GEX %+.2fM, Flip Level %.0f, Charm %.2f%%/day\n",
            sym, g.NetGEX/1e6, g.GEXFlip, g.CharmAcceleration))
        if g.NetGEX > 0 {
            b.WriteString("  → Positive GEX: dealer hedging suppresses volatility\n")
        } else {
            b.WriteString("  → Negative GEX: dealer hedging amplifies volatility\n")
        }
    }
    b.WriteString("\n")
}
```

## File yang Harus Diubah

1. `internal/service/ai/unified_outlook.go` — tambah field + prompt section
2. `internal/adapter/telegram/handler.go` — fetch GEX dan inject ke unifiedData

## Verifikasi

```bash
go build ./...
go test ./internal/service/ai/...
# Manual: /outlook → cek apakah ada section GEX di output
```

## Acceptance Criteria

- [ ] `UnifiedOutlookData` memiliki field `GEXData map[string]*gex.GEXResult`
- [ ] Handler fetch BTC dan ETH GEX sebelum build unified data
- [ ] Error fetch GEX tidak menghentikan /outlook (best-effort)
- [ ] `BuildUnifiedOutlookPrompt` include section GEX jika data tersedia
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-26-data-sources-audit-putaran18.md` — GAP-DS2
- `internal/service/gex/engine.go` — GEXResult struct, Analyze()
- `internal/adapter/telegram/handler_gex.go` — contoh penggunaan GEX
- `internal/service/ai/unified_outlook.go:14-37` — UnifiedOutlookData struct
- `internal/adapter/telegram/handler.go:1004` — unified data assembly block
