# Research: Codebase Bug Analysis — Putaran 21
**Siklus:** 5/5 (Codebase + Bug Analysis)
**Tanggal:** 2026-04-02 (Putaran 21)
**File diperiksa:** 223 Go source files

---

## Metodologi
Analisis mendalam terhadap seluruh codebase menggunakan pattern grep, pembacaan langsung file, dan tracking alur kontrol. Fokus pada: race conditions, nil pointer dereferences, goroutine leaks, division by zero, off-by-one errors, dan context misuse.

---

## BUG-1: TOCTOU Race Condition di `sentiment/cache.go`

**File:** `internal/service/sentiment/cache.go:29-58`
**Severity:** MEDIUM
**Type:** Race Condition

### Deskripsi
Fungsi `GetCachedOrFetch()` memiliki TOCTOU (Time-of-Check-Time-of-Use) race window:

```go
cacheMu.Lock()
if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
    data := cachedSentiment
    cacheMu.Unlock()  // ← UNLOCK DINI sebelum return
    return data, nil
}
cacheMu.Unlock()      // ← Unlock kedua (di slow path)

data, err := FetchSentiment(ctx)  // ← Fetch tanpa lock
if err != nil {
    return nil, err
}

cacheMu.Lock()
// Double-check: another goroutine may have fetched while we were fetching
if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
    data = cachedSentiment
} else {
    cachedSentiment = data
    cacheExpiry = time.Now().Add(cacheTTL)
}
cacheMu.Unlock()
```

Problem: Unlock di fast path sudah benar. Tapi di slow path — setelah unlock, `FetchSentiment()` dipanggil **tanpa lock**. Multiple goroutines yang cache-miss bersamaan semua memanggil `FetchSentiment()` secara paralel. Double-check setelahnya memang mencegah update ganda, tapi tetap ada **N concurrent API calls** yang berlebihan.

**Dampak:** N goroutines yang concurrently hit expired cache semua fetch simultaneously → wasted API calls ke CNN F&G, AAII, CBOE, crypto F&G.

**Fix:** Gunakan singleflight pattern untuk deduplikasi concurrent fetches.

---

## BUG-2: `context.Background()` Digunakan dalam Request Handler Goroutines

**File:** `internal/adapter/telegram/handler_quant.go:484`, `internal/adapter/telegram/handler_cta.go:581`
**Severity:** LOW-MEDIUM
**Type:** Context Propagation Issue

### Deskripsi
`fetchMultiAssetCloses()` di `handler_quant.go:484` dan `generateCTAChart()` di `handler_cta.go:581` membuat `ctx := context.Background()` lokal alih-alih menerima ctx dari parameter request.

```go
// handler_quant.go:484
func (h *Handler) fetchMultiAssetCloses(excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
    ctx := context.Background()  // ← TIDAK MENGGUNAKAN request ctx
    ...
    records, err := h.quant.DailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 300)
```

```go
// handler_cta.go:581
func (h *Handler) generateCTAChart(state *ctaState, timeframe string) ([]byte, error) {
    ctx := context.Background()  // ← TIDAK MENGGUNAKAN request ctx
```

**Dampak:** 
- Jika user membatalkan request atau koneksi putus, operasi tetap berjalan penuh
- Tidak ada cara untuk membatalkan DB read yang sedang berjalan
- Akumulasi goroutines saat load tinggi

**Fix:** Tambahkan `ctx context.Context` sebagai parameter pertama ke kedua fungsi tersebut, pass request ctx.

---

## BUG-3: HMM Baum-Welch — Log-Likelihood Konvergensi Check Salah

**File:** `internal/service/price/hmm_regime.go:79-90`
**Severity:** MEDIUM
**Type:** Logic Bug / Wrong Convergence Detection

### Deskripsi
Di loop Baum-Welch:

```go
prevLogLik := math.Inf(-1)
for iter = 0; iter < maxIter; iter++ {
    newModel, logLik := baumWelchStep(&model, obs)
    model = newModel
    if iter > 0 && math.Abs(logLik-prevLogLik) < 1e-4 {
        converged = true
        break
    }
    prevLogLik = logLik
}
```

Masalahnya: `logLik` yang dikembalikan oleh `baumWelchStep` adalah log-likelihood dari model **sebelum update** (menggunakan parameter lama untuk menghitung forward vars, baru re-estimasi). Ini bukan error fatal tapi berarti **convergence check menggunakan nilai yang off-by-one iteration** — kita membandingkan LL dari iterasi `t` vs `t-1`, padahal seharusnya `t` vs `t-1` konsisten.

Lebih serius: saat `iter == 0`, kita set `prevLogLik = logLik` tapi belum cek konvergensi. Di iterasi berikutnya, jika LL sama, kita tandai `converged = true`. Namun `math.Inf(-1)` sebagai initial value tidak akan pernah sama dengan logLik aktual, jadi iterasi 0→1 tidak bisa konvergen. Ini **oke** secara logika tapi pengecekan `iter > 0` berlebihan karena `prevLogLik` selalu berbeda dari `math.Inf(-1)` di iter 1+.

Dampak nyata: HMM kemungkinan melakukan iterasi lebih banyak dari yang dibutuhkan karena LL value dari step N dibandingkan dengan step N-1, padahal yang seharusnya dibandingkan adalah delta antara parameter sebelum dan sesudah update.

**Fix:** Simpan LL dari newModel dengan menjalankan forward pass tambahan, atau bandingkan LL dari iterasi sebelumnya secara konsisten.

---

## BUG-4: `GARCH.Converged` False Negative — Fine Grid Tidak Improve = Not Converged

**File:** `internal/service/price/garch.go:156-165`
**Severity:** LOW
**Type:** Logic Bug / Wrong Convergence Flag

### Deskripsi
```go
converged := true
if math.IsInf(fineLL, -1) || math.IsNaN(fineLL) {
    converged = false
} else if fineLL-bestLL < 0.1 {
    // Fine grid didn't meaningfully improve over coarse grid
    converged = false  // ← SALAH LOGIC
}
```

Kondisi `fineLL - bestLL < 0.1` menandai **non-converged** ketika fine grid tidak improve vs coarse grid. Tapi justru ini seharusnya berarti estimasi sudah **stabil** (coarse grid sudah cukup baik) — bukti convergence, bukan divergence. Yang ingin kita tandai non-converged adalah ketika nilai LL sangat buruk secara absolut, bukan ketika fine grid tidak meningkat.

**Dampak:** `GARCHConfidenceMultiplier()` return 1.0 (no adjustment) ketika `Converged = false`, artinya GARCH yang sebenarnya sudah konvergen ke solusi baik tidak memberikan multiplier adjustment. Signals kehilangan GARCH-informed confidence adjustment.

**Fix:** Hapus atau invert kondisi `fineLL-bestLL < 0.1`. Convergence seharusnya berdasarkan absolute LL quality, bukan delta improvement.

---

## BUG-5: `sentiment/cache.go` — `CacheAge()` Tidak Thread-Safe untuk Write

**File:** `internal/service/sentiment/cache.go:70-77`
**Severity:** LOW
**Type:** Minor Race

### Deskripsi
`CacheAge()` membaca `cachedSentiment.FetchedAt` di bawah lock, yang aman untuk read. Namun `cachedSentiment` sendiri adalah pointer ke `SentimentData`. Setelah lock dilepas, caller memegang reference ke struct yang bisa saja di-replace oleh goroutine lain saat cache TTL expire.

**Dampak:** Minimal — hanya `CacheAge()` yang berpotensi return duration yang stale, tidak ada data corruption.

---

## BUG-6: `internal/service/price/hurst.go` — `sortFloat64s` Redundant Wrapper

**File:** `internal/service/price/hurst.go` (tidak ada line number karena minor)
**Severity:** LOW
**Type:** Dead Code / Minor Issue

### Deskripsi
```go
func sortFloat64s(data []float64) {
    sort.Float64s(data)
}
```
Ini hanya wrapper satu baris dari `sort.Float64s`. Tidak ada nilai tambah, hanya menambah indirection. Tidak ada bug tapi dead abstraction.

---

## BUG-7: `handler_quant.go` & `handler_vp.go` — Python Script Timeout Orphan Process

**File:** `internal/adapter/telegram/handler_quant.go:447-452`, `internal/adapter/telegram/handler_vp.go:422-428`
**Severity:** MEDIUM
**Type:** Resource Leak

### Deskripsi
```go
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath, chartPath)
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil { ... }
```

Masalah: `exec.CommandContext` dengan timeout akan kirim **SIGKILL** ke process ketika context expired. Namun Python process yang di-kill mungkin tidak sempat membersihkan temp files (`inputPath`, `outputPath`, `chartPath`).

Lebih serius: kedua fungsi ini menggunakan `context.Background()` bukan parent request ctx. Jika request Telegram sudah timeout atau user cancel, Python script tetap berjalan penuh sampai timeout 60/90 detik tercapai.

**Dampak:** 
- Temp files tidak terhapus jika Python di-SIGKILL sebelum selesai
- Zombie Python processes jika ada multiple concurrent requests
- Memory/CPU waste

**Fix:** Pass request ctx sebagai parent, atau tambahkan `cmd.Cancel` handler untuk cleanup temp files.

---

## BUG-8: `cot/confluence_score.go` — Weight Sum Comment Mismatch

**File:** `internal/service/cot/confluence_score.go` (fungsi `ConfluenceScoreV2`)
**Severity:** LOW
**Type:** Documentation / Logic Inconsistency

### Deskripsi
Komentar di fungsi:
```
// Components (4-factor):
//   - COT positioning   (35%) — based on SentimentScore (-100..+100)
//   - Calendar surprise  (20%) — based on recent sigma surprise for this currency
//   - Macro conditions   (45%) — unified FRED score (yield, PCE, NFCI, labor, GDP)
```

Tapi kode aktual menggunakan:
```go
total := cotScore*0.35 + surpriseScore*0.20 + macroScore*0.45
```

Komentar bilang "4-factor" tapi hanya ada 3 komponen. Total weight = 0.35 + 0.20 + 0.45 = 1.0 ✓. Ini bukan bug fungsional tapi misleading documentation yang bisa menyebabkan kebingungan saat maintenance.

---

## Ringkasan Temuan

| Bug | File | Severity | Type |
|-----|------|----------|------|
| BUG-1 | sentiment/cache.go | MEDIUM | Race Condition / Stampede |
| BUG-2 | handler_quant.go, handler_cta.go | MEDIUM | Context Propagation |
| BUG-3 | price/hmm_regime.go | MEDIUM | Logic / Convergence |
| BUG-4 | price/garch.go | LOW | Logic / Convergence Flag |
| BUG-5 | sentiment/cache.go | LOW | Minor Race |
| BUG-6 | price/hurst.go | LOW | Dead Code |
| BUG-7 | handler_quant.go, handler_vp.go | MEDIUM | Resource Leak |
| BUG-8 | cot/confluence_score.go | LOW | Doc Inconsistency |

---

## Rekomendasi Task (TASK-295 s/d TASK-299)

1. **TASK-295**: Fix sentiment cache stampede dengan singleflight
2. **TASK-296**: Propagate request ctx ke fetchMultiAssetCloses & generateCTAChart
3. **TASK-297**: Fix HMM Baum-Welch convergence check (off-by-one LL comparison)
4. **TASK-298**: Fix GARCH converged flag (inverted logic for fine grid improvement)
5. **TASK-299**: Fix Python subprocess temp file cleanup on SIGKILL
