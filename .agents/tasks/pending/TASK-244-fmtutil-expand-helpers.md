# TASK-244: TECH-010 — Expand pkg/fmtutil: FormatLargeNumber, FormatPips, MessageHeader, Divider

**Priority:** low
**Type:** refactor
**Estimated:** XS
**Area:** pkg/fmtutil/format.go, pkg/fmtutil/format_test.go
**Created by:** Research Agent
**Created at:** 2026-04-02 22:00 WIB

## Deskripsi

`pkg/fmtutil/format.go` sudah punya 22 fungsi, tapi masih kekurangan 4 helper yang sering dibutuhkan di formatter code dan saat ini di-inline secara repetitif.

Ditemukan **107 lokasi** penggunaan `strings.Builder` di codebase, banyak yang mengandung boilerplate patterns yang bisa di-extract ke helpers ini.

Task ini menambah 4 fungsi ke package yang sudah ada (TECH-010 expansion).

## Fungsi yang Ditambahkan

### 1. FormatLargeNumber — market cap / volume
```go
// FormatLargeNumber formats a large float with K/M/B/T suffix for compact display.
// Example: FormatLargeNumber(1_234_567) => "1.23M"
// Example: FormatLargeNumber(999) => "999"
func FormatLargeNumber(n float64) string {
    switch {
    case math.Abs(n) >= 1e12:
        return fmt.Sprintf("%.2fT", n/1e12)
    case math.Abs(n) >= 1e9:
        return fmt.Sprintf("%.2fB", n/1e9)
    case math.Abs(n) >= 1e6:
        return fmt.Sprintf("%.2fM", n/1e6)
    case math.Abs(n) >= 1e3:
        return fmt.Sprintf("%.2fK", n/1e3)
    default:
        return fmt.Sprintf("%.0f", n)
    }
}
```

### 2. FormatPips — FX pips display
```go
// FormatPips formats a pips value to 1 decimal place with sign.
// FX pips are typically small values (e.g. 12.5 pips).
// Example: FormatPips(12.5) => "+12.5 pips"
// Example: FormatPips(-3.0) => "-3.0 pips"
func FormatPips(f float64) string {
    if f >= 0 {
        return fmt.Sprintf("+%.1f pips", f)
    }
    return fmt.Sprintf("%.1f pips", f)
}
```

### 3. MessageHeader — section header dengan emoji
```go
// MessageHeader returns a bold emoji+title line for Telegram HTML messages.
// Example: MessageHeader("COT Overview", "📊") => "<b>📊 COT Overview</b>\n"
func MessageHeader(emoji, title string) string {
    if emoji == "" {
        return fmt.Sprintf("<b>%s</b>\n", title)
    }
    return fmt.Sprintf("<b>%s %s</b>\n", emoji, title)
}
```

### 4. Divider — standard section separator
```go
// Divider returns a standard horizontal rule line for Telegram messages.
// Uses the same Unicode heavy rule used throughout the codebase.
func Divider() string {
    return "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
}

// ThinDivider returns a thin horizontal rule.
func ThinDivider() string {
    return "─────────────────────────────\n"
}
```

## File yang Harus Diubah

- `pkg/fmtutil/format.go` — tambah 4 fungsi di section yang sesuai
- `pkg/fmtutil/format_test.go` — tambah test cases untuk setiap fungsi baru

## Test Cases yang Harus Ditambahkan

```go
func TestFormatLargeNumber(t *testing.T) {
    // 1M, 1B, 1T, below 1K, negative
}

func TestFormatPips(t *testing.T) {
    // positive, negative, zero
}

func TestMessageHeader(t *testing.T) {
    // with emoji, without emoji
}

func TestDivider(t *testing.T) {
    // returns non-empty string ending with newline
}
```

## Aturan

- **Hanya tambah** ke file yang sudah ada, jangan rewrite
- Letakkan fungsi di section yang logis (Number Formatting, Visual Indicators, dll)
- `go test ./pkg/fmtutil/...` harus PASS
- `go vet ./pkg/fmtutil/...` harus bersih

## Acceptance Criteria

- [ ] 4+ fungsi baru ditambah ke `pkg/fmtutil/format.go`
- [ ] Test cases ditambah ke `pkg/fmtutil/format_test.go`
- [ ] `go test ./pkg/fmtutil/...` → PASS
- [ ] `go build ./...` sukses
- [ ] `Divider()` dan `MessageHeader()` menggunakan format yang konsisten dengan existing code

## Referensi

- `.agents/research/2026-04-02-22-tech-refactor-plan-putaran10.md` — Temuan 4
- `TECH_REFACTOR_PLAN.md#TECH-010` — Duplicate Code di Formatters
- `pkg/fmtutil/format.go` — file target (22 fungsi sudah ada)
