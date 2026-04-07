# Research: Bug Hunting & Edge Cases — Siklus 5
**Date:** 2026-04-01 14:00 WIB
**Focus:** Bug Hunting & Edge Cases (Siklus 5)

---

## Metodologi

Analisis statis codebase secara menyeluruh:
- `internal/service/ta/` — seluruh indikator TA
- `internal/service/news/` — scheduler & impact recorder
- `internal/adapter/telegram/` — handler, rate limiter, quant
- `internal/service/price/` — fetcher, hurst, aggregator
- `internal/service/ai/` — memory store
- `internal/service/microstructure/` — engine
- `pkg/` — mathutil, timeutil, fmtutil

---

## Bug yang Ditemukan

### BUG-A1: context.Sleep non-cancellable di MQL5 retry
**File:** `internal/service/news/fetcher.go:223`
**Severity:** Medium
**Deskripsi:**
```go
time.Sleep(3 * time.Second) // ← tidak cek ctx.Done()
result, fetchErr = f.doFetchMQL5(ctx, dateMode, from, to) // retry
```
Ketika context sudah cancelled/timeout, `time.Sleep` tetap berjalan selama 3 detik penuh, baru kemudian retry yang akan langsung gagal juga. Seharusnya menggunakan `select` dengan `ctx.Done()`.

**Impact:** Delay tidak perlu saat shutdown atau timeout, menghambat graceful shutdown.

**Fix:**
```go
select {
case <-ctx.Done():
    return ctx.Err()
case <-time.After(3 * time.Second):
}
```

---

### BUG-A2: Dead code / resource leak di runQuantEngine
**File:** `internal/adapter/telegram/handler_quant.go:444-456`
**Severity:** Low-Medium
**Deskripsi:**
```go
// Baris 444 — cmd dibuat dengan context.Background() lalu dibuang
cmd := exec.CommandContext(context.Background(), "python3", ...)
cmd.Stderr = os.Stderr

// Baris 450 — cmd dibuat ULANG dengan timeout context — yang pertama terbuang
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
cmd = exec.CommandContext(cmdCtx, "python3", ...) // reassign
cmd.Stderr = os.Stderr
```
`cmd` pertama dibuat tapi langsung ditimpa sebelum dipakai. Ini dead code dan confusing — bisa menyebabkan programmer salah baca logika timeout.

**Impact:** Code clarity issue, potential confusion saat maintenance.

**Fix:** Hapus block pertama, langsung buat dengan timeout context.

---

### BUG-A3: CalcStochasticSeries — window rawK tidak include bar ke-i
**File:** `internal/service/ta/indicators.go:395-406`
**Severity:** High
**Deskripsi:**
```go
for i := 0; i < n; i++ {
    ...
    hh := asc[i].High
    ll := asc[i].Low
    for j := i - kPeriod + 1; j < i; j++ { // ← TIDAK include i sendiri
        if asc[j].High > hh { hh = asc[j].High }
        if asc[j].Low < ll { ll = asc[j].Low }
    }
```
Loop inner menggunakan `j < i` (exclusive), sehingga bar ke-i tidak masuk dalam scan highest high/lowest low. Seharusnya `j <= i` atau equivalen. Ini menyebabkan stochastic %K dihitung dari range yang salah — high/low current bar tidak ikut dalam window.

Perbandingan: `CalcWilliamsR` menggunakan loop yang benar (`for i := 1; i < period; i++` dari bars[0]). `CalcCCI` tidak ada masalah ini.

**Impact:** Nilai Stochastic %K tidak akurat — bisa off secara sistematis terutama saat current bar adalah high/low baru.

**Fix:**
```go
for j := i - kPeriod + 1; j <= i; j++ { // ubah j < i → j <= i
```
*(Catatan: karena `hh` dan `ll` sudah di-seed dari `asc[i]`, loop tidak perlu include `i` — ini sebenarnya sudah benar. Perlu re-analisis mendalam.)*

**Update:** Setelah re-analisis: `hh = asc[i].High` dan `ll = asc[i].Low` sudah di-seed sebelum loop. Loop dari `j = i-kPeriod+1` sampai `j < i` iterates atas bar sebelumnya. Ini BENAR untuk kPeriod bars (bar i + kPeriod-1 sebelumnya). **False positive — bukan bug.**

---

### BUG-A4: goroutine di RecordImpact meneruskan ctx request
**File:** `internal/service/news/scheduler.go:676-683`
**Severity:** Medium
**Deskripsi:**
```go
go func() {
    defer recover()...
    s.impactRecorder.RecordImpact(ctx, ev, ...) // ← ctx dari request loop
}()
```
`ctx` yang diteruskan ke goroutine adalah context dari loop scheduler. Jika loop ini restart atau context di-cancel, goroutine akan langsung gagal pada operasi I/O berikutnya. Untuk path sinkron (past horizons), impact record tidak akan tersimpan.

Namun, untuk path async (future horizons), `delayedRecord` sudah benar menggunakan `context.Background()`.

**Impact:** Impact records untuk horizons di masa lalu bisa gagal tersimpan saat context parent di-cancel.

**Fix:** Gunakan `context.Background()` saat memanggil `RecordImpact` dari goroutine terpisah.

---

### BUG-A5: CalcBollinger Squeeze hanya cek bwSeries[:period] tapi loop bisa lebih pendek
**File:** `internal/service/ta/indicators.go:490-503`
**Severity:** Low
**Deskripsi:**
```go
bwSeries := make([]float64, 0)
for i := 0; i < len(upper) && i < period; i++ { // max period entries
    if !math.IsNaN(upper[i]) ... {
        bwSeries = append(bwSeries, ...)
    }
}
if len(bwSeries) >= period { // check >= period tapi bwSeries bisa < period karena NaN skip
    avgBW := ...
    squeeze = bw < avgBW*0.75
}
```
Loop mengambil dari newest-first slice tapi banyak nilai awal adalah NaN (dari komputasi Bollinger). Sehingga `bwSeries` hampir selalu memiliki < `period` entries karena newest-first menaruh NaN di elemen-elemen awal komputasi yang valid (newest = index 0 = most recent).

**Impact:** Squeeze detection hampir selalu return `false` karena kondisi `len(bwSeries) >= period` jarang terpenuhi.

**Fix:** Loop dari index 0 (newest) ke len(upper), ambil valid entries, dan check bila sudah cukup:
```go
for i := 0; i < len(upper); i++ {
    if !math.IsNaN(upper[i]) && ... {
        bwSeries = append(bwSeries, ...)
    }
    if len(bwSeries) >= period {
        break
    }
}
```

---

### BUG-A6: memory_store.go — create tidak check apakah file sudah ada
**File:** `internal/service/ai/memory_store.go:131-145`
**Severity:** Low
**Deskripsi:**
```go
func (ms *MemoryStore) create(ctx context.Context, userID int64, p string, content string) string {
    ...
    ms.cache[userID][p] = content // overwrite tanpa warning
    ...
    return fmt.Sprintf("File created successfully at %s", p)
}
```
`create` tidak mengecek apakah file sudah ada. Jika AI memanggil `create` pada file yang sudah ada, file akan di-overwrite tanpa peringatan. Pesan sukses yang dikembalikan menyesatkan ("File created successfully" padahal sebenarnya overwrite).

**Impact:** Memory loss yang tidak terdeteksi — AI bisa tidak sengaja menimpa memory yang ada.

**Fix:** Cek keberadaan file sebelum create, return error jika sudah ada.

---

### BUG-A7: CalcOBV SMA indexing off-by-one
**File:** `internal/service/ta/indicators.go:776-805`
**Severity:** Medium
**Deskripsi:**
```go
// CalcOBV uses CalcOBVSeries (newest-first) then:
sma := CalcSMA(series, 10) // sma[0] = newest SMA(10)
newestAvg := (series[0] + series[1]) / 2 // ← fallback heuristic
```
Comment mengatakan "we need the SMA ending at the most recent bar" tapi kemudian ada fallback yang menggunakan average dari hanya 2 bar. Ini bukan representasi yang akurat dari trend.

Lebih penting: jika `len(series) >= 10`, `sma[0]` mestinya valid (newest), tapi jika `series` terlalu pendek untuk SMA, code jatuh ke `(series[0] + series[1]) / 2` yang bisa crash jika `len(series) < 2`.

**Impact:** Potential panic jika `len(series) == 1`.

**Fix:** Guard `len(series) >= 2` sebelum fallback.

---

## Edge Cases yang Ditemukan

### EDGE-1: CalcFibonacci dengan swing high == swing low (same bar)
**File:** `internal/service/ta/fibonacci.go:101-120`
Kode sudah menangani `sh.idx == sl.idx` dengan fallback ke swing ke-2. Namun jika hanya ada 1 swing high dan 1 swing low dan keduanya di bar yang sama, fungsi return `nil`. **Sudah handled.**

### EDGE-2: CalcIchimoku boundary check untuk pastIdx
**File:** `internal/service/ta/ichimoku.go:108-122`
Ketika `n == senkouPeriod + shift` (minimal case), `pastIdx = last - shift = (n-1) - 26`. Kemudian `highestHigh(asc, pastIdx-senkouPeriod+1, pastIdx)` = indeks `0` sampai `pastIdx`. Ini valid. **Sudah handled.**

### EDGE-3: aggregator.go accesses bars[0] setelah guard len > 0
**File:** `internal/service/price/aggregator.go:101-103`
```go
result := make([]domain.IntradayBar, 0, len(buckets))
contractCode := bars[0].ContractCode // ← safe karena len(bars)==0 sudah dicek di baris 41
```
**Sudah safe.**

### EDGE-4: rateLimit cleanup bisa delete entry aktif
**File:** `internal/adapter/telegram/ratelimit.go:91-100`
Cleanup mengecek `w.timestamps[len(w.timestamps)-1].Before(cutoff)`. Jika user aktif tapi tidak mengirim command selama 5 menit, entry-nya dihapus. Ini desain yang benar (stale cleanup). **By design, bukan bug.**

---

## Rekomendasi

| Bug ID | File | Severity | Action |
|--------|------|----------|--------|
| BUG-A1 | news/fetcher.go | Medium | Fix: gunakan select+ctx.Done() |
| BUG-A2 | handler_quant.go | Low | Fix: hapus dead code cmd pertama |
| BUG-A4 | news/scheduler.go | Medium | Fix: gunakan context.Background() |
| BUG-A5 | ta/indicators.go | Low | Fix: loop BW series dari valid entries |
| BUG-A6 | ai/memory_store.go | Low | Fix: check file existence on create |
| BUG-A7 | ta/indicators.go | Medium | Fix: guard len(series) >= 2 |
