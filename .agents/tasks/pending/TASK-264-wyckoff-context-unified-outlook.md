# TASK-264: Wyckoff Phase Context → UnifiedOutlookData + /outlook AI Prompt

**Priority:** low
**Type:** data-integration
**Estimated:** S
**Area:** internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

`/wyckoff` engine menghasilkan `WyckoffResult` yang mencakup Schematic (Accumulation/Distribution), Phase saat ini (A-E), events (SC, AR, Spring, SOS, LPS, dll), dan Confidence level. Data ini **tidak masuk ke `/outlook` AI prompt**.

Informasi Wyckoff phase adalah konteks struktural jangka menengah yang sangat relevan untuk directional bias AI. Contoh: "XAUUSD sedang di Phase C Accumulation dengan Spring detected" → AI bisa merekomendasikan long dengan lebih confident.

**Catatan:** TASK-260 (wire /wyckoff in main.go) sebaiknya diselesaikan lebih dulu agar endpoint user juga aktif, tapi task ini independen dari TASK-260 (UnifiedOutlookData integration bisa dilakukan tanpa command user-facing terdaftar, karena engine diinstansiasi secara internal di cmdOutlook).

**Data source:** Internal — dailyPriceRepo (sudah ada, gratis, tidak perlu API baru).

## File yang Harus Diubah

1. `internal/service/ai/unified_outlook.go` — tambah field + section prompt
2. `internal/adapter/telegram/handler.go` — fetch Wyckoff untuk major pairs di cmdOutlook

## Implementasi

### 1. unified_outlook.go — tambah field ke UnifiedOutlookData

Import:
```go
wyckoffsvc "github.com/arkcode369/ark-intelligent/internal/service/wyckoff"
```

Di `UnifiedOutlookData` struct:
```go
// WyckoffContexts holds Wyckoff structure analysis for major symbols (Daily timeframe).
// Only includes results with Confidence != "LOW".
// Key = symbol (e.g. "EURUSD"), Value = WyckoffResult.
WyckoffContexts map[string]*wyckoffsvc.WyckoffResult
```

### 2. unified_outlook.go — tambah section di BuildUnifiedOutlookPrompt()

```go
if len(data.WyckoffContexts) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. WYCKOFF STRUCTURE ANALYSIS (Daily) ===\n", section))
    section++
    for sym, r := range data.WyckoffContexts {
        b.WriteString(fmt.Sprintf("%s: %s Phase=%s Confidence=%s\n",
            sym, r.Schematic, r.CurrentPhase, r.Confidence))
        if r.Summary != "" {
            b.WriteString(fmt.Sprintf("  %s\n", r.Summary))
        }
    }
    b.WriteString("NOTE: Wyckoff Accumulation Phase C/D (Spring/SOS) → bullish structural setup. " +
        "Distribution Phase C/D (UTAD/SOW) → bearish structural setup.\n\n")
}
```

### 3. handler.go — fetch Wyckoff di cmdOutlook

Di `cmdOutlook`, sebelum construct `unifiedData`:
```go
var wyckoffContexts map[string]*wyckoffsvc.WyckoffResult

// Fetch Wyckoff analysis using h.wyckoff if available, otherwise init engine locally.
wyckoffEngine := wyckoffsvc.NewEngine()
if h.wyckoff != nil {
    wyckoffEngine = h.wyckoff.WyckoffEngine
}

if h.dailyPriceRepo != nil {
    wyckoffContexts = make(map[string]*wyckoffsvc.WyckoffResult)
    majors := []string{"EURUSD", "XAUUSD", "BTCUSD"}
    for _, sym := range majors {
        mapping := domain.FindPriceMapping(sym)
        if mapping == nil { continue }
        recs, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.PriceCode, 200)
        if err != nil || len(recs) < 50 { continue }
        bars := pricesvc.DailyRecordsToOHLCV(recs) // existing conversion helper
        r := wyckoffEngine.Analyze(sym, "daily", bars)
        if r != nil && r.Confidence != "LOW" {
            wyckoffContexts[sym] = r
        }
    }
    if len(wyckoffContexts) == 0 {
        wyckoffContexts = nil
    }
}
```

Inject ke unifiedData:
```go
unifiedData := aisvc.UnifiedOutlookData{
    // ... existing fields ...
    WyckoffContexts: wyckoffContexts,
}
```

**Note:** `h.dailyPriceRepo` sudah tersedia di Handler struct. `DailyRecordsToOHLCV` atau konversi serupa mungkin sudah ada di `service/price` — cek sebelum menulis ulang. Lihat pola di `handler_wyckoff.go:136` untuk referensi fetch + konversi bars.

## Acceptance Criteria

- [ ] `UnifiedOutlookData` punya field `WyckoffContexts map[string]*wyckoffsvc.WyckoffResult`
- [ ] `/outlook` prompt menyertakan section "WYCKOFF STRUCTURE ANALYSIS" jika ada Confidence != LOW
- [ ] Section menampilkan: Schematic (Accumulation/Distribution), Phase, Confidence, Summary
- [ ] Fetch untuk EURUSD, XAUUSD, BTCUSD pada Daily timeframe (200 bars)
- [ ] Hanya hasil dengan Confidence != "LOW" yang dimasukkan
- [ ] Jika `h.dailyPriceRepo == nil` → skip gracefully, tidak crash
- [ ] Jika fetch gagal satu symbol → symbol di-skip, yang lain tetap jalan
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-10-feature-index-wiring-gaps-unified-outlook-putaran14.md` — Temuan #5
- `internal/service/wyckoff/engine.go:23` — Engine.Analyze(symbol, timeframe, bars) signature
- `internal/service/wyckoff/types.go` — WyckoffResult struct (Schematic, CurrentPhase, Confidence, Summary)
- `internal/adapter/telegram/handler_wyckoff.go:120` — fetchWyckoffBars() — pola bar fetching
- `internal/service/ai/unified_outlook.go:20` — UnifiedOutlookData struct
- `internal/adapter/telegram/handler.go:1004` — construction point unifiedData
- TASK-260 — Wire /wyckoff di main.go (terkait tapi tidak blocking)
