# TASK-263: GEX (Gamma Exposure) Context → UnifiedOutlookData + /outlook AI Prompt

**Priority:** medium
**Type:** data-integration
**Estimated:** S
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

`/gex BTC` menghasilkan `GEXResult` kaya dari Deribit public API (TotalGEX, GEXFlipLevel, GammaWall, PutWall, MaxPain, Regime, KeyLevels) tapi data ini **tidak masuk ke `/outlook` AI prompt**.

GEX adalah layer options market yang sangat relevan untuk crypto direction — menentukan apakah market-maker hedging akan meredam (positive GEX) atau amplify (negative GEX) pergerakan harga.

**Nilai untuk AI outlook:**
- Positive GEX → dealer akan jual saat naik, beli saat turun → mean-reversion/range behavior
- Negative GEX → dealer akan beli saat naik, jual saat turun → momentum/trending behavior
- GEX Flip Level → harga kritis di mana regime berganti → level support/resistance kunci
- Gamma Wall → magnet harga ke atas | Put Wall → floor harga ke bawah

**Data source:** Deribit public API (gratis, no key needed) — sudah dipakai `/gex`, cache 15 menit.

## File yang Harus Diubah

1. `internal/service/ai/unified_outlook.go` — tambah field + section prompt
2. `internal/adapter/telegram/handler.go` — fetch GEX untuk BTC+ETH di cmdOutlook

## Implementasi

### 1. unified_outlook.go — tambah field ke UnifiedOutlookData

Import:
```go
gexsvc "github.com/arkcode369/ark-intelligent/internal/service/gex"
```

Di `UnifiedOutlookData` struct:
```go
// GEXResults holds Gamma Exposure analysis for crypto assets (Deribit options data).
// Key = symbol (e.g. "BTC", "ETH"), Value = GEXResult.
GEXResults map[string]*gexsvc.GEXResult
```

### 2. unified_outlook.go — tambah section di BuildUnifiedOutlookPrompt()

```go
if len(data.GEXResults) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. GAMMA EXPOSURE — OPTIONS FLOW (Deribit) ===\n", section))
    section++
    for sym, r := range data.GEXResults {
        b.WriteString(fmt.Sprintf("%s @ $%.0f: Regime=%s | TotalGEX=%.2fM | Flip=$%.0f | GWall=$%.0f | PWall=$%.0f | MaxPain=$%.0f\n",
            sym, r.SpotPrice, r.Regime,
            r.TotalGEX/1e6, r.GEXFlipLevel,
            r.GammaWall, r.PutWall, r.MaxPain))
    }
    b.WriteString("NOTE: POSITIVE_GEX = dealer hedging dampens moves (mean-reversion). " +
        "NEGATIVE_GEX = dealer hedging amplifies moves (trending). " +
        "GEX Flip Level = key price where regime switches.\n\n")
}
```

### 3. handler.go — fetch GEX di cmdOutlook

Di `cmdOutlook`, sebelum construct `unifiedData`:
```go
var gexResults map[string]*gexsvc.GEXResult

if h.gex != nil {
    gexResults = make(map[string]*gexsvc.GEXResult)
    for _, sym := range []string{"BTC", "ETH"} {
        r, err := h.gex.Engine.Analyze(ctx, sym) // 15-min cached, fast
        if err == nil && r != nil {
            gexResults[sym] = r
        }
    }
    if len(gexResults) == 0 {
        gexResults = nil
    }
}
```

Inject ke unifiedData:
```go
unifiedData := aisvc.UnifiedOutlookData{
    // ... existing fields ...
    GEXResults: gexResults,
}
```

**Note:** `h.gex.Engine.Analyze()` sudah di-cache 15 menit (lihat `gex/engine.go:41`). Fetch ini tidak akan hit Deribit API setiap kali `/outlook` dipanggil.

## Acceptance Criteria

- [ ] `UnifiedOutlookData` punya field `GEXResults map[string]*gexsvc.GEXResult`
- [ ] `/outlook` prompt menyertakan section "GAMMA EXPOSURE — OPTIONS FLOW" saat data tersedia
- [ ] Section menampilkan: Regime, TotalGEX (dalam juta), GEXFlipLevel, GammaWall, PutWall, MaxPain
- [ ] Fetch dilakukan untuk BTC dan ETH
- [ ] Jika `h.gex == nil` → skip gracefully, tidak crash
- [ ] Jika fetch gagal satu symbol → symbol di-skip, yang lain tetap jalan
- [ ] Note edukatif tentang POSITIVE_GEX vs NEGATIVE_GEX muncul di prompt
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-10-feature-index-wiring-gaps-unified-outlook-putaran14.md` — Temuan #4
- `internal/service/gex/engine.go:43` — Engine.Analyze(ctx, symbol) signature
- `internal/service/gex/types.go:14` — GEXResult struct (semua fields)
- `internal/service/ai/unified_outlook.go:20` — UnifiedOutlookData struct
- `internal/adapter/telegram/handler_gex.go:86` — pola penggunaan h.gex.Engine.Analyze()
- `internal/adapter/telegram/handler.go:1004` — construction point unifiedData
- TASK-259 (microstructure → /outlook) — pattern referensi
