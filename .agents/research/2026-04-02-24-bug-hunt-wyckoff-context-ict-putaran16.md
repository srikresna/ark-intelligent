# Research — Siklus 5: Analisis Codebase + Bug Hunt (Putaran 16)

**Date:** 2026-04-02 24:xx WIB
**Focus:** Wyckoff engine, ICT structure, context propagation, formatter bugs
**Files Analyzed:** service/wyckoff/, service/ict/, adapter/telegram/handler_quant.go, handler_vp.go, formatter_wyckoff.go

---

## Bugs Ditemukan

### BUG-W1: Wyckoff Formatter — Volume Multiplier Salah (formatter_wyckoff.go:45)

**File:** `internal/adapter/telegram/formatter_wyckoff.go:45`

**Problem:**
```go
e.Volume/1000, // raw ratio placeholder; real would need avgVol
```

Formatter menampilkan `e.Volume/1000` sebagai "X times avg" padahal itu bukan rasio yang benar. `WyckoffResult` tidak menyimpan `AvgVolume`, sehingga formatter tidak bisa menghitung multiplier yang akurat. User melihat angka yang tidak bermakna (misalnya "0.8x avg" padahal seharusnya "2.3x avg").

**Root Cause:** Engine (`engine.go:44`) menghitung `av := avgVolume(fwd)` tapi tidak menyimpannya ke `WyckoffResult`. Formatter tidak punya `avgVol`.

**Fix:**
1. Tambah field `AvgVolume float64` ke `WyckoffResult` di `types.go`
2. Set `result.AvgVolume = av` di `engine.go`
3. Ubah formatter: `e.Volume / r.AvgVolume` (dengan guard `r.AvgVolume > 0`)

**Severity:** Medium — display bug, tidak mempengaruhi logika analisis

---

### BUG-W2: Wyckoff Events — avgBarRange() Dipanggil di Dalam Loop (events.go:65, 277)

**File:** `internal/service/wyckoff/events.go`

**Problem:**
```go
// detectSC (L63-66):
for i := 0; i < half; i++ {
    b := bars[i]
    rangeSize := b.High - b.Low
    avgRange := avgBarRange(bars, 14)  // ← dipanggil tiap iterasi!
    ...
}

// detectBC (L275-278):
for i := 0; i < half; i++ {
    b := bars[i]
    rangeSize := b.High - b.Low
    avgRange := avgBarRange(bars, 14)  // ← same issue
    ...
}
```

`avgBarRange(bars, 14)` merekalkulasi rata-rata range 14 bar pertama pada setiap iterasi loop. `bars` tidak berubah, jadi hasilnya selalu sama. Ini O(N×14) alih-alih O(N+14).

Dengan `half` bisa sampai 60 iterasi, masing-masing memanggil loop 14, total ada ~840 operasi float64 yang seharusnya hanya ~74.

**Fix:** Hoist keluar dari loop:
```go
avgRange := avgBarRange(bars, 14)  // sebelum loop
for i := 0; i < half; i++ {
    ...
    if b.Volume > avgVol*1.5 && rangeSize > avgRange*1.2 && ... {
```

**Severity:** Low — performance issue, bars biasanya kecil sehingga impact minimal tapi tetap wrong code

---

### BUG-CTX1: handler_quant.go — context.Background() Menggantikan Parent Context (L448)

**File:** `internal/adapter/telegram/handler_quant.go:448`

**Problem:**
```go
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
```

Menggunakan `context.Background()` alih-alih parent `ctx`. Jika user disconnect atau request di-cancel, Python subprocess tetap berjalan hingga timeout 60s.

**Expected:**
```go
cmdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
```

**Handler referensi yang benar:** `handler_cta.go:714` sudah menggunakan `context.WithTimeout(ctx, 90*time.Second)`.

**Severity:** Medium — context leak, subprocess tidak bisa di-cancel dari luar

---

### BUG-CTX2: handler_vp.go — context.Background() Menggantikan Parent Context (L422)

**File:** `internal/adapter/telegram/handler_vp.go:422`

**Problem:**
```go
cmdCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
```

Same issue dengan BUG-CTX1. VP Python engine tidak bisa di-cancel via parent context.

**Fix:** Ganti ke `context.WithTimeout(ctx, 90*time.Second)`. Pastikan `ctx` sudah available di scope tersebut — perlu tambah parameter `ctx context.Context` ke `runVPEngine()` jika belum ada.

**Severity:** Medium — context leak

---

### BUG-CTX3: handler_quant.go — fetchMultiAssetCloses Tidak Menerima Context (L483)

**File:** `internal/adapter/telegram/handler_quant.go:483-484`

**Problem:**
```go
func (h *Handler) fetchMultiAssetCloses(excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    ctx := context.Background()
    ...
```

Fungsi membuat `context.Background()` sendiri alih-alih menerima parent context. Dipanggil dari `handler_quant.go:421` yang sudah punya `ctx`. Jika parent request dibatalkan, loop fetching 20+ aset tetap berjalan.

**Fix:**
```go
// Signature:
func (h *Handler) fetchMultiAssetCloses(ctx context.Context, excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    // hapus ctx := context.Background()
    ...
}
// Caller:
multiAsset, maErr := h.fetchMultiAssetCloses(ctx, state.symbol, tf)
```

**Severity:** Medium — context propagation bug (TECH-008)

---

### BUG-ICT1: ict/structure.go — Loop Pattern Menyesatkan di currentBias()

**File:** `internal/service/ict/structure.go`

**Problem:**
```go
func currentBias(events []StructureEvent) string {
    for i := len(events) - 1; i >= 0; i-- {
        return events[i].Direction  // SELALU return pada iterasi pertama!
    }
    return "NEUTRAL"
}
```

Loop `for` yang berjalan dari belakang tapi langsung `return` tanpa kondisi — artinya loop tidak pernah benar-benar iterasi lebih dari sekali. Secara fungsional sama dengan:
```go
if len(events) > 0 {
    return events[len(events)-1].Direction
}
return "NEUTRAL"
```

Tidak ada bug logika tapi kode sangat menyesatkan — pembaca mengira ada logika "scan backwards for something" padahal tidak.

**Fix:** Refactor ke idiomatic Go:
```go
func currentBias(events []StructureEvent) string {
    if len(events) == 0 {
        return "NEUTRAL"
    }
    return events[len(events)-1].Direction
}
```

**Severity:** Low — code smell, zero behavior change

---

### BUG-SMC1: SMCServices Duplikasi ICTEngine Instance

**File:** `internal/adapter/telegram/handler_smc.go:31`, `internal/adapter/telegram/handler.go:129`

**Problem:**
```go
// Handler struct punya:
ict      *ICTServices   // ← contains ICTEngine
smc      *SMCServices   // ← ALSO contains ICTEngine

// SMCServices:
type SMCServices struct {
    ICTEngine      *ictsvc.Engine  // ← terpisah dari h.ict.ICTEngine
    ...
}
```

Ada dua instance `ictsvc.Engine` — satu di `h.ict.ICTEngine` (untuk `/ict` command) dan satu di `h.smc.ICTEngine` (untuk `/smc` command). `Engine` adalah struct kosong (`type Engine struct{}`), stateless, tidak ada state yang dishare. Jadi tidak ada bug behavior tapi ini adalah unnecessary duplication.

**Fix (optional):** Saat wiring, pass `ict.ICTEngine` ke `SMCServices`. Tapi karena `Engine` stateless, impact-nya nol. Bisa dibiarkan atau diperbaiki saat wiring refactor.

**Severity:** Very Low — duplication tapi tidak menimbulkan bug

---

## Summary

| Bug ID | File | Severity | Type |
|--------|------|----------|------|
| BUG-W1 | formatter_wyckoff.go:45 | Medium | Display bug — wrong volume multiplier |
| BUG-W2 | wyckoff/events.go:65,277 | Low | Performance — avgBarRange in loop |
| BUG-CTX1 | handler_quant.go:448 | Medium | Context propagation |
| BUG-CTX2 | handler_vp.go:422 | Medium | Context propagation |
| BUG-CTX3 | handler_quant.go:483 | Medium | Context propagation |
| BUG-ICT1 | ict/structure.go | Low | Code smell — misleading loop |
| BUG-SMC1 | handler_smc.go | Very Low | Duplication |

## Tasks Yang Direkomendasikan

- TASK-270: Fix BUG-W1 — Tambah AvgVolume ke WyckoffResult, perbaiki formatter_wyckoff
- TASK-271: Fix BUG-W2 — Hoist avgBarRange keluar dari loop di detectSC dan detectBC
- TASK-272: Fix BUG-CTX1+CTX2 — Ganti context.Background() dengan parent ctx di handler_quant dan handler_vp
- TASK-273: Fix BUG-CTX3 — Tambah ctx parameter ke fetchMultiAssetCloses
- TASK-274: Fix BUG-ICT1 — Refactor currentBias() di ict/structure.go
