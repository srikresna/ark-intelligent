# TASK-054: Timestamp Format Standardization di Semua Formatter Output

**Priority:** low
**Type:** refactor/ux
**Estimated:** S (1-2 jam)
**Area:** internal/adapter/telegram/formatter.go, pkg/fmtutil/
**Created by:** Research Agent
**Created at:** 2026-04-01 18:00 WIB
**Siklus:** UX-1 (Siklus 1 Sesi 2)

## Deskripsi

Analisis `formatter.go` menemukan 3 format timestamp berbeda:
1. `"02 Jan 15:04"` — FRED macro (baris 1654), sentiment (baris 2067)
2. `"02 Jan 15:04 WIB"` — Alpha composites (baris 1839)
3. `"15:04 WIB"` — Calendar events

Ini inkonsisten dan merusak professional look.
UX_AUDIT merekomendasikan footer "Updated: HH:MM WIB" standar di setiap response.
TASK-002 (standardisasi button labels) sudah pending tapi tidak mencakup timestamp.
TASK-019 (expand fmtutil helpers) adalah refactor — belum mencakup timestamp standar.

## Solution

### 1. Tambah helper di pkg/fmtutil/format.go

```go
// UpdatedAt returns a standardized "Updated: DD MMM HH:MM WIB" timestamp string.
// Suitable for appending to analysis messages.
func UpdatedAt(t time.Time) string {
    wib := time.FixedZone("WIB", 7*60*60)
    return fmt.Sprintf("<i>Updated: %s WIB</i>", t.In(wib).Format("02 Jan 15:04"))
}

// UpdatedAtShort returns HH:MM WIB only (for inline use).
func UpdatedAtShort(t time.Time) string {
    wib := time.FixedZone("WIB", 7*60*60)
    return t.In(wib).Format("15:04 WIB")
}
```

### 2. Ganti semua timestamp inline di formatter.go

Ganti:
```go
// Sebelum (3 varian berbeda):
data.FetchedAt.Format("02 Jan 15:04")
composites.ComputedAt.Format("02 Jan 15:04 WIB")
e.TimeWIB.Format("15:04 WIB")
```

Dengan:
```go
// Sesudah (konsisten):
fmtutil.UpdatedAt(data.FetchedAt)
fmtutil.UpdatedAt(composites.ComputedAt)
fmtutil.UpdatedAtShort(e.TimeWIB)
```

### 3. Target files

- `formatter.go` — semua baris yang ada `.Format("02 Jan 15:04")` dan variasinya
- `formatter_quant.go` — cek apakah ada timestamp
- `handler_alpha.go`, `handler_seasonal.go` — jika ada timestamp inline

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Semua timestamp di formatter.go menggunakan `fmtutil.UpdatedAt()` atau `UpdatedAtShort()`
- [ ] Format output: `Updated: 01 Apr 18:30 WIB` (konsisten di semua command)
- [ ] Tidak ada string literal timestamp format di formatter kecuali calendar time display

## Notes
- Bergantung pada TASK-019 (expand fmtutil) — bisa dikerjakan bersama atau setelah TASK-019
- Scope kecil, effort rendah, dampak visual signifikan

## File yang Kemungkinan Diubah
- `pkg/fmtutil/format.go`
- `internal/adapter/telegram/formatter.go`
- `internal/adapter/telegram/formatter_quant.go`
