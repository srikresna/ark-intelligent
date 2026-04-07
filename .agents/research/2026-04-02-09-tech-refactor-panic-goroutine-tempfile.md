# Research Report: Tech Refactor Siklus 4 Putaran 4
# Panic Recovery, Goroutine Pooling, Temp File Race, Subprocess Stderr
**Date:** 2026-04-02 09:00 WIB
**Siklus:** 4/5 (Tech Refactor) — Putaran 4
**Author:** Research Agent

## Ringkasan

Deep analysis menemukan 12 tech debt items baru. 5 yang paling impactful dijadikan task. Fokus: panic recovery gaps, goroutine lifecycle, dan subprocess management.

## Temuan 1: Missing Panic Recovery in News Scheduler Goroutines

**File:** `internal/service/news/scheduler.go:114-127`

`Start()` meluncurkan 5 goroutine (`runInitialSync`, `runWeeklySyncLoop`, `runDailyReminderLoop`, `runMicroScrapeLoop`, `runPreEventReminderLoop`) tanpa panic recovery di level outer. Meskipun inner loop functions punya `defer recover()`, jika panic terjadi sebelum loop masuk defer, goroutine mati silent.

**Impact:** News scheduler berhenti tanpa notifikasi. Users tidak terima alert lagi.
**Severity:** HIGH

## Temuan 2: Unbounded Goroutine Dispatch in Bot Polling

**File:** `internal/adapter/telegram/bot.go:205`

```go
for _, update := range updates {
    go b.handleUpdate(ctx, update)  // NO GOROUTINE LIMIT
}
```

Setiap update spawn goroutine baru tanpa pooling. Jika Telegram kirim 100 updates sekaligus (e.g., setelah outage), 100 goroutines spawn simultaneously. Ditambah masing-masing handler bisa spawn sub-goroutine (Python subprocess, API calls).

**Impact:** Resource exhaustion under burst load.
**Severity:** MEDIUM

## Temuan 3: Temp File Race Condition in Python Subprocess

**Files:** `handler_vp.go:410-447`, `handler_cta.go:707+`, `handler_quant.go:442+`

Pattern berbahaya:
```go
defer os.Remove(inputPath)   // cleanup on exit
defer os.Remove(outputPath)  // cleanup on exit
cmd.Run()                    // subprocess writes to outputPath
result.ChartPath = chartPath // returned to caller
```

Problem: `defer os.Remove()` di awal function. Jika function exit sebelum caller baca chart, file sudah dihapus. Atau jika Python subprocess timeout, cleanup berjalan saat Python masih menulis.

**Impact:** Intermittent chart failures, corrupted output.
**Severity:** MEDIUM

## Temuan 4: Double exec.CommandContext + Stderr Not Captured

**File:** `handler_quant.go:446-452`

Dua `exec.CommandContext()` calls — yang pertama langsung didiscard oleh yang kedua. Wasted allocation. Ditambah semua Python subprocess pakai `cmd.Stderr = os.Stderr` langsung, bukan buffer. Saat Python crash, error message hilang (tidak masuk structured log).

**Severity:** MEDIUM

## Temuan 5: HTTP Client Connection Pooling Absent

**Files:** Semua fetcher (`vix/fetcher.go`, `cot/fetcher.go`, `price/fetcher.go`, `coingecko/client.go`)

Setiap service buat `http.Client{}` sendiri tanpa configure `MaxIdleConns`, `MaxConnsPerHost`, `MaxIdleConnsPerHost`. Masing-masing punya default 100 idle conns global. Total 6+ services × default pooling = suboptimal connection reuse.

**Note:** TASK-118 (HTTP Client Factory) covers factory pattern tapi TIDAK covers connection pooling tuning. Task ini fokus pada Transport configuration yang specific.

**Severity:** LOW-MEDIUM

## Findings Not Turned Into Tasks (Lower Priority)

- Health check goroutine without WaitGroup (LOW)
- Formatter concurrent map safety audit (LOW)
- Hardcoded 2-min delay in scheduler bootstrap (LOW)
- News broadcaster error cascade (LOW-MEDIUM)
