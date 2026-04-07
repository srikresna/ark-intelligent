# TASK-238: ICT Multi-Timeframe (MTF) HTF Bias Overlay untuk /ict

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/ict/, internal/adapter/telegram/handler_ict.go, internal/adapter/telegram/formatter_ict.go
**Created by:** Research Agent
**Created at:** 2026-04-02 21:00 WIB

## Deskripsi

Saat ini `/ict EURUSD H4` hanya menganalisis timeframe H4 saja. Dalam ICT methodology, **HTF (Higher TimeFrame) bias adalah filter utama** — tidak boleh long di H4 jika Daily structure bearish.

Menambahkan HTF bias overlay akan:
1. Secara otomatis fetch dan analisis Daily (untuk H1/H4 requests) atau Weekly (untuk D1 requests)
2. Tampilkan HTF bias sebagai context di atas analisis LTF
3. Beri warning jika LTF bias bertentangan dengan HTF

Infrastructure sudah ada: `DailyPriceStore` dan `IntradayStore` sudah tersedia di `ICTServices`.

## File yang Harus Diubah

- `internal/service/ict/types.go` — tambah `HTFBias`, `HTFSummary`, `HTFTimeframe` ke `ICTResult`
- `internal/service/ict/engine.go` — tambah method `AnalyzeHTF()` atau parameter opsional
- `internal/adapter/telegram/handler_ict.go` — fetch HTF bars, run secondary Analyze(), pass ke result
- `internal/adapter/telegram/formatter_ict.go` — tampilkan MTF section di header

## Aturan HTF Selection

```
Requested TF → HTF yang dipakai
M15, M30    → H4 daily bars (4-hour)
H1, H4      → Daily bars  
D1          → Weekly bars (jika tersedia, else skip)
W1, MN      → tidak ada HTF (sudah top-level)
```

## Implementasi

### types.go — field baru di ICTResult
```go
// HTF context — set when a higher timeframe analysis was performed
HTFBias      string    // "BULLISH" | "BEARISH" | "NEUTRAL" | "" (empty = not available)
HTFTimeframe string    // e.g. "D1" (the HTF analyzed)
HTFSummary   string    // short narrative: "Daily: CHoCH bullish, OB @ 1.0820 below price"
HTFAligned   bool      // true if LTF bias matches HTF bias
```

### handler_ict.go — fetch HTF dan run secondary analysis
```go
// Setelah mendapat result LTF, fetch HTF bars dan analisis:
htfTF := higherTimeframe(requestedTF) // returns "D1" for "H4"
if htfTF != "" {
    htfBars, err := fetchBarsForTF(ctx, h.ict, symbol, htfTF)
    if err == nil && len(htfBars) >= 30 {
        htfResult := h.ict.Engine.Analyze(htfBars, symbol, htfTF)
        result.HTFBias = htfResult.Bias
        result.HTFTimeframe = htfTF
        result.HTFSummary = htfResult.Summary[:min(100, len(htfResult.Summary))]
        result.HTFAligned = (result.Bias == htfResult.Bias)
    }
}
```

### Formatter display (header section)
```
📐 ICT/SMC Analysis — EURUSD H4
┌─ Multi-TF Context ────────────────────────────┐
│ Daily  : 🔴 BEARISH (CHoCH bearish confirmed) │
│ H4     : 🟡 NEUTRAL (awaiting BOS)             │
│ Signal : ⚠️ HTF bearish — favor sell setups   │
└───────────────────────────────────────────────┘
```

Jika aligned:
```
│ Daily  : 🟢 BULLISH │ H4 : 🟢 BULLISH │ ✅ MTF Aligned │
```

## Acceptance Criteria

- [ ] Untuk `/ict EURUSD H4`, Daily bars di-fetch dan dianalisis secara otomatis
- [ ] `result.HTFBias` terisi dengan "BULLISH"/"BEARISH"/"NEUTRAL"
- [ ] `result.HTFAligned` benar (true jika LTF bias sama dengan HTF bias)
- [ ] Formatter menampilkan MTF section di header output `/ict`
- [ ] Jika HTF fetch gagal → HTF section di-skip gracefully (tidak error)
- [ ] Untuk D1 request: HTF = Weekly; jika weekly tidak tersedia → skip silently
- [ ] Cache waktu HTF fetch independent dari LTF (LTF lebih fresh)
- [ ] Unit test: `TestHigherTimeframe` untuk mapping H4→D1, M15→H4, dll.

## Referensi

- `.agents/research/2026-04-02-21-feature-gaps-skew-credit-ict-pdarray-cot-seasonal-putaran9.md` — Temuan 4
- `internal/adapter/telegram/handler_ict.go` — handler yang perlu ditambah HTF fetch step
- `internal/service/ict/engine.go:Analyze()` — reuse untuk HTF bars
- `internal/service/price/` — DailyPriceStore untuk daily bars HTF
