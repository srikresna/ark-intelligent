# TASK-024: Fix potential panic di CalcOBV fallback

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/ta
**Created by:** Research Agent
**Created at:** 2026-04-01 14:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `CalcOBV` (`internal/service/ta/indicators.go:776-805`), ketika SMA tidak bisa dihitung (len(series) < 10), ada fallback:

```go
newestAvg := (series[0] + series[1]) / 2
```

Jika `len(series) == 1`, akses `series[1]` akan menyebabkan **index out of range panic**.

`CalcOBVSeries` bisa return slice dengan 1 element jika hanya ada 1 bar input. `CalcOBV` mendapatkan series dari `CalcOBVSeries(bars)` — jika `len(bars) >= 1` (cek minimal ada di engine.go baris 40), maka series bisa berisi 1 element.

## Konteks
Panic yang tidak di-recover dalam goroutine akan crash seluruh proses. Meski ada recover di handler level, better to fix at source.

Ref: `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A7)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Guard `len(series) >= 2` ditambahkan sebelum akses `series[1]` di fallback path
- [ ] Jika hanya 1 element, gunakan `series[0]` saja atau return trend "FLAT"

## File yang Kemungkinan Diubah
- `internal/service/ta/indicators.go`

## Referensi
- `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A7)
