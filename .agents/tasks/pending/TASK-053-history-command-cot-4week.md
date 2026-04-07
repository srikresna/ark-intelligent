# TASK-053: /history Command — COT Positioning 4-Week History View

**Priority:** medium
**Type:** feature
**Estimated:** M (3-4 jam)
**Area:** internal/adapter/telegram/handler.go, internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-01 18:00 WIB
**Siklus:** UX-1 (Siklus 1 Sesi 2)

## Deskripsi

Tidak ada command untuk melihat historical COT data.
Data COT historical sudah tersedia (cotRepo methods untuk GetRecordsForContract ada di codebase).
UX_AUDIT TASK-UX-013 merekomendasikan:
- `/history EUR` → COT positioning EUR 4 minggu terakhir dalam satu view
- `/compare EUR GBP` → side-by-side comparison

Ini sangat berguna untuk melihat tren positioning, apakah smart money sedang akumulasi atau distribusi.

## Solution

### 1. Register command `/history` dan `/compare` di handler.go

```go
bot.RegisterCommand("/history", h.cmdHistory)
bot.RegisterCommand("/compare", h.cmdCompare)
```

### 2. Implementasi `cmdHistory()`

```go
func (h *Handler) cmdHistory(ctx context.Context, chatID string, userID int64, args string) error {
    // Parse: /history EUR [4|8|12] (default 4 weeks)
    // Fetch records via cotRepo
    // Format as ASCII mini-chart atau tabel
    // Show trend direction
}
```

Format output (ASCII trend):
```
📊 COT History — EUR (4 Minggu)

Wk  Net Position    Chg      Bias
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
-4  +112,345       base     🟢 LONG
-3  +119,234    +6,889 ↑   🟢 LONG
-2  +128,901    +9,667 ↑   🟢 LONG
-1  +121,456    -7,445 ↓   🟢 LONG
 0  +134,231   +12,775 ↑   🟢 LONG

Trend: ↗ Akumulasi (3/4 minggu positif)
Momentum: Kuat (+19,3% dari baseline)
```

### 3. Implementasi `cmdCompare()`

```go
// /compare EUR GBP
// Side-by-side Net Position + Bias untuk 2 currency
```

Format:
```
⚖️ COT Comparison — EUR vs GBP

         EUR              GBP
Net:  +134,231 🟢    -45,678 🔴
Pct:    78%               23%
Conv:   8.2/10            4.1/10
Trend:  ↗ Akumulasi    ↘ Distribusi
```

### 4. Keyboard untuk history view

```
[◀ Prev 4W] [4W ✓] [8W] [12W] [▶]
[🔄 Refresh] [📤 Share] [🏠 Home]
```

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `/history EUR` → tampilkan 4 minggu terakhir COT positioning EUR
- [ ] `/history EUR 8` → tampilkan 8 minggu
- [ ] `/compare EUR GBP` → side-by-side comparison dua currency
- [ ] Tabel terbaca dengan baik di Telegram mobile
- [ ] Error graceful jika data tidak tersedia

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler.go`
- `internal/adapter/telegram/formatter.go`
- `internal/adapter/telegram/keyboard.go`
