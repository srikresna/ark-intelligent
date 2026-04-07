# TASK-262: ICT/SMC Analysis Context → UnifiedOutlookData + /outlook AI Prompt

**Priority:** medium
**Type:** data-integration
**Estimated:** S
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

`/ict` command sudah menghasilkan `ICTResult` yang kaya (Fair Value Gaps, Order Blocks, BOS/CHOCH, Liquidity Sweeps, Bias, Killzone) tapi data ini **tidak masuk ke `/outlook` AI prompt**.

`UnifiedOutlookData` struct tidak punya field untuk ICT context. Akibatnya AI di `/outlook` tidak mengetahui struktur pasar ICT/SMC saat merekomendasikan entry/exit.

**Contoh gap:** AI menyebut "EURUSD bullish outlook" tapi tidak tahu ada CHoCH bearish di H4 kemarin dan FVG unmitigated di 1.0820. Dengan ICT context, AI bisa berkata "bullish bias tapi ada CHoCH H4 bearish — tunggu FVG fill sebelum long."

## File yang Harus Diubah

1. `internal/service/ai/unified_outlook.go` — tambah field + section prompt
2. `internal/adapter/telegram/handler.go` — fetch ICT untuk major pairs di cmdOutlook

## Implementasi

### 1. unified_outlook.go — tambah field ke UnifiedOutlookData

Import:
```go
ictsvc "github.com/arkcode369/ark-intelligent/internal/service/ict"
```

Di `UnifiedOutlookData` struct (setelah `MicrostructureSignals` atau setelah `BISData`):
```go
// ICTContexts holds ICT/SMC structure analysis for major symbols (H4 timeframe).
// Key = symbol (e.g. "EURUSD"), Value = ICTResult.
ICTContexts map[string]*ictsvc.ICTResult
```

### 2. unified_outlook.go — tambah section di BuildUnifiedOutlookPrompt()

Tambah setelah section Microstructure atau setelah section BIS REER:
```go
if len(data.ICTContexts) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. ICT/SMC MARKET STRUCTURE (H4) ===\n", section))
    section++
    for sym, r := range data.ICTContexts {
        fvgCount := len(r.FVGZones)
        obCount := 0
        for _, ob := range r.OrderBlocks {
            if !ob.Mitigated { obCount++ }
        }
        sweepCount := len(r.Sweeps)
        b.WriteString(fmt.Sprintf("%s: Bias=%s | UnmitigatedFVG=%d | ActiveOB=%d | Sweeps=%d | Killzone=%s\n",
            sym, r.Bias, fvgCount, obCount, sweepCount, r.Killzone))
        if r.Structure != nil && (r.Structure.LastBOS != "" || r.Structure.LastCHoCH != "") {
            b.WriteString(fmt.Sprintf("  Structure: LastBOS=%s LastCHoCH=%s\n",
                r.Structure.LastBOS, r.Structure.LastCHoCH))
        }
    }
    b.WriteString("NOTE: ICT/SMC data is H4. Use for identifying premium/discount zones and PD Arrays.\n\n")
}
```

### 3. handler.go — fetch ICT context di cmdOutlook

Di `cmdOutlook` (handler.go:836), sebelum construct `unifiedData`, tambah:
```go
var ictContexts map[string]*ictsvc.ICTResult

if h.ict != nil {
    ictContexts = make(map[string]*ictsvc.ICTResult)
    majors := []string{"EURUSD", "GBPUSD", "XAUUSD", "BTCUSD"}
    for _, sym := range majors {
        // Reuse existing bar-fetching pattern dari cmdICT
        mapping := domain.FindPriceMapping(sym)
        if mapping == nil { continue }
        bars, err := h.fetchICTBars(ctx, mapping, "4h") // existing private method
        if err != nil || len(bars) < 15 { continue }
        ictContexts[sym] = h.ict.Engine.Analyze(bars, sym, "4h")
    }
    if len(ictContexts) == 0 {
        ictContexts = nil
    }
}
```

Inject ke unifiedData:
```go
unifiedData := aisvc.UnifiedOutlookData{
    // ... existing fields ...
    ICTContexts: ictContexts,
}
```

**Note:** `h.fetchICTBars()` adalah method private di `handler_ict.go`. Cek apakah sudah ada — jika ya, reuse. Jika tidak, gunakan pola yang sama dengan fetch di `handler_ict.go:78`.

## Acceptance Criteria

- [ ] `UnifiedOutlookData` punya field `ICTContexts map[string]*ictsvc.ICTResult`
- [ ] `/outlook` prompt menyertakan section "ICT/SMC MARKET STRUCTURE" saat data tersedia
- [ ] Section menampilkan: Bias, FVG count, OrderBlock count, Sweep count, Killzone
- [ ] Fetch dilakukan untuk: EURUSD, GBPUSD, XAUUSD, BTCUSD pada H4
- [ ] Jika `h.ict == nil` → skip gracefully, tidak crash
- [ ] Jika fetch gagal untuk satu symbol → symbol di-skip, yang lain tetap jalan
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-10-feature-index-wiring-gaps-unified-outlook-putaran14.md` — Temuan #3
- `internal/service/ict/engine.go:18` — Engine.Analyze() signature
- `internal/service/ict/types.go` — ICTResult struct fields
- `internal/service/ai/unified_outlook.go:20` — UnifiedOutlookData struct
- `internal/service/ai/unified_outlook.go:380` — pola section tambah (BIS REER section)
- `internal/adapter/telegram/handler_ict.go` — fetchICTBars private method
- `internal/adapter/telegram/handler.go:1004` — construction point unifiedData
