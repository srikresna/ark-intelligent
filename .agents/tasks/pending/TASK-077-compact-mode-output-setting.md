# TASK-077: Compact Mode — OutputMode di UserPrefs + Toggle di /settings

**Priority:** medium
**Type:** feature
**Estimated:** M (3-4 jam)
**Area:** internal/domain/prefs.go, handler.go (cmdSettings), internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-01 19:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 3)
**Ref:** UX_AUDIT.md TASK-UX-010

## Deskripsi

Semua output saat ini adalah full-detail, tidak ada opsi compact/minimal.
Untuk trader mobile yang hanya butuh signal direction, full output terlalu panjang.
UX_AUDIT merekomendasikan 3 mode: Normal, Compact, Minimal.

## Domain Change (prefs.go)

```go
// Tambah ke UserPrefs struct:
OutputMode string `json:"output_mode"` // "full" (default), "compact", "minimal"
```

```go
// Tambah ke DefaultPrefs():
OutputMode: "full",
```

## Settings Keyboard

Tambah row di settings keyboard:
```
[📋 Output: Full ✓] → toggle ke compact
[📋 Output: Compact] → toggle ke minimal  
[📋 Output: Minimal] → toggle ke full
```

Atau 3 button terpisah dalam satu row:
```
[Full] [Compact ✓] [Minimal]
```

## Output Mode Definitions

### Full (default)
Output seperti sekarang — semua detail statistik, breakdown, penjelasan.

### Compact
- COT: hanya top 3 signal + net position per currency (tanpa histogram/grafik ASCII)
- Quant: hanya summary line + 3 top indicator result
- Bias: hanya signal direction + strength (tanpa full breakdown)
- Alpha: hanya factor ranking top 5 + playbook highlights

### Minimal
- COT: satu baris per currency: "EUR: 🟢 BULLISH +85.2K"
- Bias: "EUR ↑ LONG | GBP ↓ SHORT | JPY ↑ LONG"
- Quant: "EUR EURUSD: BUY confidence 72%"

## Implementasi

1. Tambah `OutputMode` ke `UserPrefs` + `DefaultPrefs()`
2. Tambah toggle button di settings keyboard + callback handler
3. Buat `formatter.go` helper: `(h *Handler) outputMode(ctx, userID) string`
4. Implementasi compact formatter untuk COT detail (prioritas utama, karena paling panjang)
5. Pass OutputMode ke formatter calls yang relevan

## Acceptance Criteria
- [ ] `UserPrefs.OutputMode` field ada dengan default "full"
- [ ] /settings menampilkan toggle Output Mode
- [ ] COT detail ada versi compact (kurang dari 10 baris per currency)
- [ ] `go build ./...` clean
- [ ] Prefs tersimpan dan digunakan antar session
