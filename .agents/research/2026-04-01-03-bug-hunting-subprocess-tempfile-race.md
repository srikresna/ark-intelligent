# Research Report — Siklus 5: Bug Hunting & Edge Cases
**Tanggal:** 2026-04-01 03:00 WIB
**Fokus:** Subprocess hanging, temp file leaks, silenced parse errors, cache race

---

## Ringkasan

Analisis statis codebase menemukan 5 bug kategori **medium-high** yang berpotensi menyebabkan handler hang tanpa batas, disk space leak di /tmp, dan data orderbook yang diam-diam corrupt dengan zero values.

---

## Bug #1 — Subprocess Tanpa Timeout (HIGH)

**File:** `internal/adapter/telegram/handler_cta.go:745`

Fungsi `runChartScript()` (line 728) memanggil Python chart renderer dengan `context.Background()` tanpa timeout. Jika script Python hang (matplotlib deadlock, tidak ada display), seluruh goroutine handler akan hang selamanya — memblokir Telegram update polling.

**Severity:** HIGH — Bisa membuat bot tidak responsif

**Fix:** Tambahkan `context.WithTimeout(context.Background(), 60*time.Second)` seperti di handler_vp.go.

---

## Bug #2 — Subprocess Tanpa Timeout di CTA Backtest (HIGH)

**File:** `internal/adapter/telegram/handler_ctabt.go:472`

Fungsi `runBacktestChartScript()` mengalami masalah yang sama dengan Bug #1. Script backtest chart bisa lebih lambat dari script biasa.

**Severity:** HIGH

---

## Bug #3 — Dead Code + chartPath Tidak Di-Cleanup di handler_quant.go (MEDIUM)

**File:** `internal/adapter/telegram/handler_quant.go:445-451`

Line 445 membuat `cmd` yang langsung di-overwrite di line 451. Selain dead code, `chartPath` tidak ada di `defer os.Remove()` sehingga file /tmp bocor pada error path.

**Severity:** MEDIUM

---

## Bug #4 — Temp File Leak chartPath di handler_vp.go (MEDIUM)

**File:** `internal/adapter/telegram/handler_vp.go:407`

`chartPath` dibuat tapi tidak ada `defer os.Remove(chartPath)`. Jika `cmd.Run()` gagal atau `os.ReadFile(outputPath)` error, chartPath yang sudah dibuat Python tidak pernah dihapus.

**Severity:** MEDIUM

---

## Bug #5 — TOCTOU Race di Sentiment Cache (LOW)

**File:** `internal/service/sentiment/cache.go:21-35`

Antara `RUnlock()` dan `Lock()`, beberapa goroutine bisa bersamaan melewati cache check dan masing-masing memanggil `FetchSentiment()`. Hasilnya: N concurrent requests ke CNN/AAII API.

**Severity:** LOW

---

## Bug #6 — Silenced Parse Errors di Bybit Client (MEDIUM)

**File:** `internal/service/marketdata/bybit/client.go:164-481`

Banyak `ParseFloat(..., _)` dan `ParseInt(..., _)` yang error-nya discarded. Jika Bybit mengirim format aneh, price/quantity jadi 0.0 dan ikut masuk kalkulasi microstructure tanpa log warning apapun.

**Severity:** MEDIUM

---

## Statistik Bug

| ID | Severity | File | Baris |
|----|----------|------|-------|
| BUG-A | HIGH | handler_cta.go | 745 |
| BUG-B | HIGH | handler_ctabt.go | 472 |
| BUG-C | MEDIUM | handler_quant.go | 445-451 |
| BUG-D | MEDIUM | handler_vp.go | 407 |
| BUG-E | LOW | sentiment/cache.go | 21-35 |
| BUG-F | MEDIUM | bybit/client.go | 164-481 |
