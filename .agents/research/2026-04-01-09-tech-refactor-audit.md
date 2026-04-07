# Research Report: Tech Refactor Audit (Siklus 4)
**Date:** 2026-04-01 09:00 WIB  
**Focus:** TECH_REFACTOR_PLAN.md — gap analysis dan temuan baru

---

## Summary

Audit dilakukan terhadap 15 item di TECH_REFACTOR_PLAN.md. Dari 15 item:
- **11 item** sudah punya task di pending/ (TASK-015 s/d TASK-043)
- **4 item** BELUM punya task: TECH-008, TECH-009, TECH-013, TECH-015
- **1 temuan baru** (tidak ada di TECH plan): Sentiment service tanpa circuit breaker

---

## Temuan Detail

### 1. Missing Test Coverage (TECH-009) — HIGH
File test ada 25 untuk 176 file production (14% ratio).
Coverage gaps kritikal:
- `internal/service/sentiment/` — ZERO test files. Padahal FetchSentiment() memanggil 3 external API (CNN, AAII, CBOE).
- `internal/service/ai/` — ZERO test files. cached_interpreter.go (17 fungsi) tak ada test.
- `internal/adapter/telegram/formatter.go` — 84 fungsi, tak ada test. Padahal TECH plan menyebut target 80% coverage untuk format layer.
- `internal/service/cot/signals_test.go` ADA (981 LOC) — bagus. Tapi `analyzer.go`, `index.go`, `fetcher.go` tidak di-cover.

### 2. Sentiment Service Tanpa Circuit Breaker — HIGH (NEW FINDING)
`internal/service/sentiment/sentiment.go` memanggil 3 external endpoints:
- CNN Fear & Greed (`cnnFearGreedURL`)
- AAII Sentiment Survey (`firecrawlScrapeURL`) — via Firecrawl
- CBOE Put/Call (`cboe.go`)

Tidak ada `circuitbreaker.Breaker` di sentiment service. Berbeda dengan:
- COT: `cbSocrata` + `cbCSV` breakers ✅
- Price: 4 breakers (twelve-data, alpha-vantage, yahoo, coingecko) ✅
- News: `cb: circuitbreaker.New("mql5",...)` ✅

Jika CNN atau AAII down, sentiment fetch akan timeout setiap request tanpa backoff.

### 3. Config Validation Incomplete (TECH-014) — MEDIUM
`config.validate()` di config.go:235 hanya check 3 field numerik:
- COT_HISTORY_WEEKS >= 4 ✅
- COT_FETCH_INTERVAL >= 1m ✅
- CONFLUENCE_CALC_INTERVAL >= 1m ✅

Tidak divalidasi:
- Jika `CLAUDE_ENDPOINT` set tapi `ClaudeModel` kosong → runtime error
- `GEMINI_MODEL` bisa diisi string apapun tanpa validasi nama model
- `DATA_DIR` tidak dicek apakah writable/mountable
- `MASSIVE_S3_ACCESS_KEY` + `MASSIVE_S3_SECRET_KEY` wajib pair — jika salah satu kosong, S3 akan gagal diam-diam

### 4. Structured Logging Tidak Konsisten (TECH-013) — LOW
Dari audit 120 log.Error()/log.Warn() calls di internal/:
- Beberapa tidak menyertakan identifiers penting (currency, userID, contract code)
- Contoh inconsistency: `log.Error().Err(err).Msg("failed")` tanpa context vs `log.Error().Str("series", seriesID).Err(err).Msg("failed")`
- `pkg/logger` component logging (`logger.Component("config")`) digunakan, tapi banyak service tidak pakai component logger

### 5. Command Latency Tidak Terukur (TECH-015) — LOW
Tidak ada log-based latency tracking per command. Jika `/cot` butuh 8 detik vs 2 detik, tidak ada cara detect regresi tanpa debug manual.
Middleware sudah ada (`middleware.go` — 20 fungsi) namun tidak include timing.

---

## Gap vs TECH_REFACTOR_PLAN

| Item | Task Exists? | Status |
|------|-------------|--------|
| TECH-001 formatter split | TASK-015 | Pending |
| TECH-002 handler split | TASK-016 | Pending |
| TECH-003 handler_cta | TASK-040 | Pending |
| TECH-004 bot.go split | TASK-041 | Pending |
| TECH-005 dual scheduler | TASK-042 | Pending |
| TECH-006 magic numbers | TASK-018 | Pending |
| TECH-007 error handling | TASK-043 | Pending |
| **TECH-008 ctx propagation** | **NONE** | ⚠️ Gap |
| **TECH-009 test coverage** | **NONE** | ⚠️ Gap |
| TECH-010 fmtutil | TASK-019 | Pending |
| TECH-011 contract codes | TASK-017 | Pending |
| TECH-012 DI framework | NONE | Low priority, OK skip |
| **TECH-013 structured log** | **NONE** | ⚠️ Gap |
| TECH-014 config validation | NONE (partial) | MEDIUM |
| **TECH-015 metrics** | **NONE** | ⚠️ Gap |

---

## New Tasks Created

- TASK-065: Test Coverage — Sentiment + AI Service (TECH-009)
- TASK-066: Circuit Breaker untuk Sentiment Service (new finding)
- TASK-067: Config Cross-Validation Expansion (TECH-014)
- TASK-068: Structured Log Component Fields (TECH-013)
- TASK-069: Command Latency Tracking via Middleware (TECH-015)
