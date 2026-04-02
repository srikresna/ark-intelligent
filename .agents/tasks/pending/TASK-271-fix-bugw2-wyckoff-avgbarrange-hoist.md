# TASK-271: Fix BUG-W2 — Hoist avgBarRange() Keluar dari Loop di detectSC dan detectBC

**Priority:** low
**Type:** bugfix
**Estimated:** XS
**Area:** internal/service/wyckoff/events.go
**Created by:** Research Agent
**Created at:** 2026-04-02 24:00 WIB

## Deskripsi

`avgBarRange(bars, 14)` dipanggil **di dalam loop** di dua fungsi: `detectSC` (L65) dan `detectBC` (L277). Hasilnya selalu identik karena `bars` tidak berubah — ini adalah kalkulasi redundan pada setiap iterasi.

```go
// detectSC (dan detectBC — sama persis):
for i := 0; i < half; i++ {
    b := bars[i]
    rangeSize := b.High - b.Low
    avgRange := avgBarRange(bars, 14)  // ← dipanggil tiap iterasi, selalu sama!
    if b.Volume > avgVol*1.5 && rangeSize > avgRange*1.2 && b.Close < b.Open {
        ...
    }
}
```

Dengan `half` maksimal 60 iterasi, masing-masing memanggil `avgBarRange` yang loop 14 elemen → ~840 float64 ops yang seharusnya ~74.

## File yang Harus Diubah

### internal/service/wyckoff/events.go

**detectSC (sekitar L60-72):**
```go
// SEBELUM:
best := -1
bestVol := 0.0
for i := 0; i < half; i++ {
    b := bars[i]
    rangeSize := b.High - b.Low
    avgRange := avgBarRange(bars, 14)
    ...
}

// SESUDAH:
best := -1
bestVol := 0.0
avgRange := avgBarRange(bars, 14)  // ← hoist ke sini
for i := 0; i < half; i++ {
    b := bars[i]
    rangeSize := b.High - b.Low
    // avgRange sudah ada dari atas
    ...
}
```

**detectBC (sekitar L272-284) — perubahan identik:**
Hoist `avgRange := avgBarRange(bars, 14)` ke sebelum loop `for i := 0; i < half; i++`.

## Verifikasi

```bash
go build ./...
go test ./internal/service/wyckoff/...
```

## Acceptance Criteria

- [ ] `avgRange := avgBarRange(bars, 14)` di `detectSC` dipindah ke sebelum loop
- [ ] `avgRange := avgBarRange(bars, 14)` di `detectBC` dipindah ke sebelum loop
- [ ] Tidak ada perubahan pada hasil kalkulasi (zero behavior change)
- [ ] `go build ./...` clean
- [ ] `go test ./internal/service/wyckoff/...` pass

## Referensi

- `.agents/research/2026-04-02-24-bug-hunt-wyckoff-context-ict-putaran16.md` — BUG-W2
- `internal/service/wyckoff/events.go:65` — detectSC loop
- `internal/service/wyckoff/events.go:277` — detectBC loop
