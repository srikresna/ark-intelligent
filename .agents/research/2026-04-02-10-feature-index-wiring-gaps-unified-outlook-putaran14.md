# Research Siklus 3 / Putaran 14: Feature Index — Wiring Gaps & Unified Outlook Integration

**Tanggal:** 2026-04-02 10:00 WIB
**Siklus:** 3/5 (FEATURE_INDEX.md)
**Putaran:** 14
**Fokus:** Gap antara service yang sudah diimplementasi vs yang sudah tersambung ke user + peluang integrasi data ke `/outlook` AI prompt

---

## Metodologi

1. Baca `FEATURE_INDEX.md` command list secara menyeluruh
2. Cross-check setiap handler `With*()` di `cmd/bot/main.go`
3. Audit `UnifiedOutlookData` struct dan `BuildUnifiedOutlookPrompt()` untuk data yang "ada tapi tidak masuk AI"
4. Cek semua existing tasks (pending + claimed + done) untuk menghindari duplikasi
5. Identifikasi quick wins (S-M effort, HIGH value)

---

## Temuan 1: `/wyckoff` TIDAK ter-register di main.go (KRITIS)

**Status:** GAP KRITIS — User tidak bisa akses fitur ini sama sekali

### Detail:
- `internal/service/wyckoff/` → engine lengkap (Accumulation/Distribution, 50+ bar, Phase A-E, SC/AR/ST/Spring/SOS/LPS)
- `internal/adapter/telegram/handler_wyckoff.go` → cmdWyckoff, WithWyckoff() — LENGKAP
- `cmd/bot/main.go` → **TIDAK ADA** call ke `handler.WithWyckoff()`

### Bukti di main.go (baris ~390-410):
```go
// ICT wired ✅
handler.WithICT(ictServices)
// GEX wired ✅
handler.WithGEX(gexServices)
// Wyckoff — MISSING ❌
```

### Fix (trivial):
```go
// Tambahkan di blok service wiring, setelah WithGEX:
wyckoffServices := tgbot.WyckoffServices{
    DailyPriceRepo: dailyPriceRepo,
    IntradayRepo:   intradayRepo,
    WyckoffEngine:  wyckoff.NewEngine(),
}
handler.WithWyckoff(wyckoffServices)
log.Info().Msg("Wyckoff commands registered (/wyckoff)")
```

Import tambahan: `wyckoffsvc "github.com/arkcode369/ark-intelligent/internal/service/wyckoff"` (meskipun Engine diinstansiasi di handler, jadi mungkin cukup `wyckoff.NewEngine()` yang sudah ada di handler_wyckoff.go).

**Catatan:** TASK-011 di claimed oleh Dev-B tapi itu untuk membangun engine (sudah done). TASK-226 (keyboard navigation) dan TASK-187 (integrate to ta/engine) belum menyentuh wiring di main.go.

---

## Temuan 2: `/smc` TIDAK ter-register di main.go (KRITIS)

**Status:** GAP KRITIS — User tidak bisa akses fitur ini sama sekali

### Detail:
- `internal/service/ta/smc.go` → CalcSMC(), SMCResult (BOS/CHOCH/Zones) — LENGKAP
- `internal/adapter/telegram/handler_smc.go` → cmdSMC, WithSMC() — LENGKAP
- `cmd/bot/main.go` → **TIDAK ADA** call ke `handler.WithSMC()`

### Fix (trivial):
```go
// Tambahkan setelah WithICT:
smcServices := &tgbot.SMCServices{
    ICTEngine:      ictsvc.NewEngine(), // reuse pattern dari WithICT
    TAEngine:       taEngine,           // taEngine sudah ada (dipakai CTA)
    DailyPriceRepo: dailyPriceRepo,
    IntradayRepo:   intradayRepo,
}
handler.WithSMC(smcServices)
log.Info().Msg("SMC commands registered (/smc)")
```

`taEngine` sudah dibuat di baris 355 (`taEngine := ta.NewEngine()`), dapat direuse.

---

## Temuan 3: ICT/SMC analysis TIDAK masuk ke `/outlook` AI prompt

**Status:** GAP MODERAT — Kualitas AI analysis tidak optimal

### Situasi:
- `/ict EURUSD` → menghasilkan `ICTResult` lengkap (FVG, OrderBlocks, BOS/CHOCH, Sweeps, Bias, Killzone)
- `UnifiedOutlookData` struct (unified_outlook.go:20) → **TIDAK ADA field ICTContexts**
- `BuildUnifiedOutlookPrompt()` → tidak ada section ICT/SMC
- AI (`/outlook`) tidak tahu tentang fair value gaps atau order blocks di major pairs

### Nilai ICT dalam /outlook:
Saat AI menganalisis EURUSD, mengetahui:
- Ada FVG di 1.0820–1.0840 yang unmitigated
- Order Block bullish di 1.0780 belum ditest
- CHoCH bearish terjadi 3 hari lalu di H4
→ Meningkatkan spesifisitas rekomendasi AI secara signifikan

### Implementasi outline:
```go
// 1. UnifiedOutlookData — tambah field:
ICTContexts map[string]*ictsvc.ICTResult // symbol → ICTResult

// 2. cmdOutlook — fetch untuk major pairs sebelum build:
if h.ict != nil {
    ictContexts = make(map[string]*ictsvc.ICTResult)
    for _, sym := range []string{"EURUSD", "GBPUSD", "XAUUSD", "BTCUSD"} {
        bars, err := fetchBarsForSymbol(ctx, sym, "4h") // reuse existing fetch
        if err == nil {
            result := h.ict.Engine.Analyze(bars, sym, "4h")
            ictContexts[sym] = result
        }
    }
}

// 3. BuildUnifiedOutlookPrompt() — tambah section:
if len(data.ICTContexts) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. ICT/SMC STRUCTURE (4H) ===\n", section))
    for sym, r := range data.ICTContexts {
        b.WriteString(fmt.Sprintf("%s: Bias=%s | FVGs=%d | OBs=%d | Killzone=%s\n",
            sym, r.Bias, len(r.FVGZones), len(r.OrderBlocks), r.Killzone))
    }
}
```

---

## Temuan 4: GEX (Gamma Exposure) TIDAK masuk ke `/outlook` AI prompt

**Status:** GAP MODERAT — Options flow context tidak ada di AI analysis

### Situasi:
- `/gex BTC` → `GEXResult` lengkap (TotalGEX, FlipLevel, GammaWall, PutWall, MaxPain, Regime, KeyLevels)
- `UnifiedOutlookData` → **TIDAK ADA field GEXResults**
- AI tidak mengetahui GEX flip level atau gamma wall saat membahas BTCUSD/ETHUSD

### Nilai GEX dalam /outlook:
- Positive GEX (>0): dealer hedging meredam volatilitas → range-bound bias
- Negative GEX (<0): dealer hedging amplifies moves → trending/volatile bias
- GEX Flip Level: harga di mana regime berganti → kunci level support/resistance
- Gamma Wall: level yang bertindak sebagai price magnet

### Data source: Deribit public API (gratis, sudah dipakai `/gex`)

### Implementasi outline:
```go
// UnifiedOutlookData — tambah field:
GEXResults map[string]*gexsvc.GEXResult // "BTC" → GEXResult, "ETH" → GEXResult

// cmdOutlook — fetch saat h.gex != nil:
if h.gex != nil {
    gexResults = make(map[string]*gexsvc.GEXResult)
    for _, sym := range []string{"BTC", "ETH"} {
        r, err := h.gex.Engine.Analyze(ctx, sym) // 15-min cached
        if err == nil { gexResults[sym] = r }
    }
}

// BuildUnifiedOutlookPrompt() — section baru:
if len(data.GEXResults) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. OPTIONS GAMMA EXPOSURE (Deribit) ===\n", section))
    for sym, r := range data.GEXResults {
        b.WriteString(fmt.Sprintf("%s: Regime=%s | TotalGEX=%.2fM | Flip=$%.0f | Wall=$%.0f | PutWall=$%.0f\n",
            sym, r.Regime, r.TotalGEX/1e6, r.GEXFlipLevel, r.GammaWall, r.PutWall))
    }
}
```

---

## Temuan 5: Wyckoff phase TIDAK masuk ke `/outlook` AI prompt

**Status:** GAP RENDAH-MODERAT

### Situasi:
- `/wyckoff XAUUSD` → `WyckoffResult` (Schematic: Accumulation/Distribution, Phase, Events)
- `UnifiedOutlookData` → **TIDAK ADA field WyckoffContexts**
- Informasi "XAUUSD sedang di Phase C Accumulation (Spring detected)" sangat relevan untuk directional bias AI

### Nilai Wyckoff dalam /outlook:
- Phase C Spring detection → high-probability reversal setup
- Distribution UTAD → bearish shift signal
- SOS/LPS → accumulation confirmed

### Implementasi outline:
```go
// UnifiedOutlookData — tambah field:
WyckoffContexts map[string]*wyckoff.WyckoffResult // symbol → WyckoffResult

// cmdOutlook — fetch untuk major pairs:
if h.wyckoff != nil {
    wyckoffContexts = make(map[string]*wyckoff.WyckoffResult)
    for _, sym := range []string{"EURUSD", "XAUUSD", "BTCUSD"} {
        bars, err := fetchDailyBarsForSymbol(ctx, sym, 200)
        if err == nil {
            r := h.wyckoff.WyckoffEngine.Analyze(sym, "daily", bars)
            if r.Confidence != "LOW" { // hanya include jika confident
                wyckoffContexts[sym] = r
            }
        }
    }
}

// BuildUnifiedOutlookPrompt() — section baru:
if len(data.WyckoffContexts) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. WYCKOFF STRUCTURE (Daily) ===\n", section))
    for sym, r := range data.WyckoffContexts {
        b.WriteString(fmt.Sprintf("%s: %s Phase=%s Confidence=%s | Events: %s\n",
            sym, r.Schematic, r.CurrentPhase, r.Confidence, r.Summary))
    }
}
```

---

## Summary: 5 Tasks Baru (TASK-260 s/d TASK-264)

| # | Task | Priority | Effort | Nilai |
|---|------|----------|--------|-------|
| TASK-260 | Wire `/wyckoff` di main.go | HIGH | XS | Aktivasi fitur untuk users |
| TASK-261 | Wire `/smc` di main.go | HIGH | XS | Aktivasi fitur untuk users |
| TASK-262 | ICT analysis → UnifiedOutlookData + /outlook | MEDIUM | S | Kualitas AI meningkat |
| TASK-263 | GEX → UnifiedOutlookData + /outlook | MEDIUM | S | Options flow context AI |
| TASK-264 | Wyckoff → UnifiedOutlookData + /outlook | LOW | S | Structural bias context AI |

---

## Verified Free Data Sources

| Data | Source | Status |
|------|--------|--------|
| Wyckoff analysis | Internal (dailyPriceRepo) | ✅ Gratis, sudah ada |
| SMC/BOS/CHOCH | Internal (ta/smc.go) | ✅ Gratis, sudah ada |
| ICT FVG/OB | Internal (ict engine) | ✅ Gratis, sudah ada |
| GEX | Deribit public API | ✅ Gratis, sudah ada (15-min cache) |

Semua task menggunakan data yang sudah ada — tidak perlu API baru atau biaya tambahan.
