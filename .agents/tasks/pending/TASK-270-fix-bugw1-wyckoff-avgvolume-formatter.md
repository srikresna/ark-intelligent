# TASK-270: Fix BUG-W1 — Tambah AvgVolume ke WyckoffResult dan Perbaiki Volume Display

**Priority:** medium
**Type:** bugfix
**Estimated:** S
**Area:** internal/service/wyckoff/types.go, internal/service/wyckoff/engine.go, internal/adapter/telegram/formatter_wyckoff.go
**Created by:** Research Agent
**Created at:** 2026-04-02 24:00 WIB

## Deskripsi

`formatter_wyckoff.go:45` menggunakan `e.Volume/1000` sebagai placeholder untuk menampilkan volume multiplier:

```go
b.WriteString(fmt.Sprintf("  [%s] %s <b>%s</b>: %.5f (vol %.1fx avg)%s\n",
    phase, icon, string(e.Name),
    e.Price,
    e.Volume/1000, // raw ratio placeholder; real would need avgVol
    sigIcon,
))
```

Ini menampilkan angka yang salah (e.g. "0.8x avg") karena `WyckoffResult` tidak menyimpan rata-rata volume. Engine sudah menghitung `av := avgVolume(fwd)` di `engine.go:44` tapi tidak di-store ke result.

## File yang Harus Diubah

### 1. internal/service/wyckoff/types.go — Tambah field AvgVolume

```go
// WyckoffResult is the full output of an analysis run.
type WyckoffResult struct {
    // ... existing fields ...
    AvgVolume     float64         // mean volume over analyzed bars (for formatter)
    AnalyzedAt    time.Time
}
```

### 2. internal/service/wyckoff/engine.go — Populate AvgVolume

Setelah `av := avgVolume(fwd)` di L44, tambahkan:
```go
result.AvgVolume = av
```

### 3. internal/adapter/telegram/formatter_wyckoff.go:45 — Gunakan AvgVolume

**Sebelum:**
```go
e.Volume/1000, // raw ratio placeholder; real would need avgVol
```

**Sesudah:**
```go
volumeMult(e.Volume, r.AvgVolume),
```

Tambah helper di file yang sama:
```go
func volumeMult(vol, avg float64) float64 {
    if avg <= 0 {
        return 0
    }
    return vol / avg
}
```

## Verifikasi

```bash
go build ./...
go vet ./...
```

Output `/wyckoff EURUSD` harus menampilkan angka seperti "2.3x avg" atau "1.8x avg" yang bermakna, bukan "0.0x avg" atau "1234.5x avg".

## Acceptance Criteria

- [ ] `WyckoffResult.AvgVolume` ditambahkan ke `types.go`
- [ ] `engine.go` meng-assign `result.AvgVolume = av`
- [ ] `formatter_wyckoff.go` menggunakan `AvgVolume` untuk menghitung multiplier
- [ ] Guard `r.AvgVolume > 0` mencegah division by zero
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

## Referensi

- `.agents/research/2026-04-02-24-bug-hunt-wyckoff-context-ict-putaran16.md` — BUG-W1
- `internal/service/wyckoff/engine.go:44` — `av := avgVolume(fwd)`
- `internal/service/wyckoff/types.go` — WyckoffResult struct
- `internal/adapter/telegram/formatter_wyckoff.go:45` — placeholder
