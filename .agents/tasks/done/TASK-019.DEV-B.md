# TASK-019: Expand pkg/fmtutil dengan Helper Methods

**Status:** pending
**Priority:** LOW
**Effort:** S (Small — estimasi 30-45 menit)
**Cycle:** Siklus 4 — Technical Refactor
**Ref:** TECH-010 in TECH_REFACTOR_PLAN.md

---

## Problem

`pkg/fmtutil/format.go` (225 LOC) sudah ada tapi kurang helpers. Pattern-pattern ini berulang inline di formatter.go:

1. `strings.Repeat("─", 28)` — divider line, muncul >20x
2. Inline header format: `emoji + " <b>" + title + "</b>\n"` — berulang di setiap section
3. Pips formatting: `fmt.Sprintf("%.1f", math.Abs(v))` — price change in pips
4. Large number formatting untuk COT net positioning: `±123,456` contracts

---

## Solution

Tambahkan ke `pkg/fmtutil/format.go` (atau file baru `pkg/fmtutil/message.go`):

```go
// Divider returns a horizontal rule line for Telegram messages.
// Example: "────────────────────────────"
func Divider() string {
    return strings.Repeat("─", 28)
}

// MessageHeader returns a bold Telegram-formatted section header.
// Example: MessageHeader("COT Analysis", "📊") → "📊 <b>COT Analysis</b>"
func MessageHeader(emoji, title string) string {
    return emoji + " <b>" + title + "</b>"
}

// FormatPips formats a float as pips value (1 decimal, always positive).
// Example: FormatPips(-12.3) → "12.3"
func FormatPips(v float64) string {
    return fmt.Sprintf("%.1f", math.Abs(v))
}

// FormatLargeInt formats an integer with thousand separators and optional sign.
// Example: FormatLargeInt(123456, true) → "+123,456"
// Example: FormatLargeInt(-45678, true) → "-45,678"
func FormatLargeInt(n int64, withSign bool) string { ... }
```

Kemudian update formatter.go untuk pakai helper-helper ini sebagai prep untuk TASK-015.

---

## Acceptance Criteria

- [ ] 4 helper functions ditambahkan ke pkg/fmtutil
- [ ] Unit tests untuk masing-masing helper (minimal 3 test cases per function)
- [ ] `go test ./pkg/fmtutil/...` pass dengan coverage >80%
- [ ] `go build ./...` clean
- [ ] Minimal 5 lokasi di formatter.go diupdate untuk pakai helper baru (proof of concept)

---

## Implementation Notes

1. Lihat `pkg/fmtutil/format_test.go` untuk pola test yang sudah ada — ikuti pattern yang sama
2. `Divider()` paling mudah — mulai dari sini
3. `MessageHeader` — cek dulu format HTML yang dipakai di formatter.go (ada yang pakai `<b>` ada yang `<b>...</b>`)
4. Task ini adalah PREP untuk TASK-015 — memastikan tools tersedia sebelum split

---

## Assigned To

(unassigned — AMAN dikerjakan parallel, low conflict risk)
