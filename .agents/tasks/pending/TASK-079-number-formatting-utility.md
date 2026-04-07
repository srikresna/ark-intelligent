# TASK-079: Number Formatting Utility — FormatThousands, FormatPct, FormatForex

**Priority:** medium
**Type:** refactor
**Estimated:** S (2 jam)
**Area:** pkg/format/ (baru), internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-01 19:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 3)
**Ref:** UX_AUDIT.md Format & Visual Improvements section

## Deskripsi

Formatting angka di codebase tidak konsisten:
- COT net position: "123456" (tanpa separator)
- Beberapa tempat pakai "+123456", ada yang "+123,456"
- Tidak ada centralized formatter untuk number formatting
- UX_AUDIT merekomendasikan: 123,456 (ribuan), 67.3% (1 decimal pct), 5 decimal forex

## File Baru: pkg/format/numbers.go

```go
package format

import (
    "fmt"
    "math"
    "strings"
)

// FormatInt formats an integer with thousands separator.
// Example: 123456 → "123,456", -1234567 → "-1,234,567"
func FormatInt(n int64) string { ... }

// FormatFloat formats a float with thousands separator and given decimal places.
// Example: FormatFloat(12345.678, 2) → "12,345.68"
func FormatFloat(f float64, decimals int) string { ... }

// FormatPct formats a percentage to 1 decimal place.
// Example: 0.673 → "67.3%", 67.3 → "67.3%"
func FormatPct(f float64) string { ... }

// FormatForex formats a forex price.
// JPY pairs (isJPY=true): 2 decimal places
// Other pairs: 5 decimal places
func FormatForex(price float64, isJPY bool) string { ... }

// FormatNetPosition formats a COT net position with sign and thousands.
// Example: 123456 → "+123,456", -50000 → "-50,000"
func FormatNetPosition(n int64) string { ... }
```

## File Test: pkg/format/numbers_test.go

Test cases untuk semua fungsi.

## Refactor formatter.go

Setelah utility ada, refactor `internal/adapter/telegram/formatter.go`:
- Ganti `fmt.Sprintf("%.0f", netPos)` → `format.FormatInt(netPos)`
- Ganti manual percentage calculation → `format.FormatPct(pct)`

Lakukan refactor HANYA untuk kasus yang jelas — jangan ubah semua sekaligus.
Fokus pada formatter yang paling sering muncul di output COT.

## Acceptance Criteria
- [ ] `pkg/format/numbers.go` ada dengan 5 fungsi
- [ ] `pkg/format/numbers_test.go` ada dengan test coverage >80%
- [ ] COT formatter menggunakan FormatInt dan FormatNetPosition
- [ ] `go test ./pkg/format/...` pass
- [ ] `go build ./...` clean
