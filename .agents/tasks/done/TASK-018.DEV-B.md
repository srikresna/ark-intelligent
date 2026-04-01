# TASK-018: Magic Numbers → Named Constants di config/constants.go

**Status:** pending
**Priority:** MEDIUM
**Effort:** S (Small — estimasi 30-45 menit)
**Cycle:** Siklus 4 — Technical Refactor
**Ref:** TECH-006 in TECH_REFACTOR_PLAN.md

---

## Problem

Magic numbers dan magic durations tersebar di 15+ lokasi:

**Signal Strength Threshold:**
```go
// internal/scheduler/scheduler.go:392
if sig.Strength >= 4 { ... }

// internal/service/backtest/stats.go:152
if s.Strength >= 4 { ... }
```

**Rate Limiter Durations:**
```go
// Muncul 6x di news/scheduler.go dan scheduler.go
time.Sleep(50 * time.Millisecond)

// internal/service/cot/fetcher.go:174
time.Sleep(200 * time.Millisecond)

// internal/service/price/fetcher.go:669,675
time.Sleep(300 * time.Millisecond)

// internal/adapter/telegram/bot.go:870
time.Sleep(35*time.Millisecond - sinceLastSend)
```

---

## Solution

Buat file `internal/config/constants.go`:

```go
package config

import "time"

// Signal thresholds
const (
    // MinAlertStrength adalah minimum COT signal strength untuk trigger alert ke user.
    // Scale 1-5, di mana 5 adalah sinyal terkuat.
    MinAlertStrength = 4
)

// Rate limiter delays — untuk menghindari flood/rate limiting dari provider
const (
    // TelegramFloodDelay adalah delay antar pesan Telegram untuk avoid flood control.
    TelegramFloodDelay = 50 * time.Millisecond

    // TelegramSendDelay adalah delay minimum antar send operasi di bot loop.
    TelegramSendDelay = 35 * time.Millisecond

    // COTFetcherDelay adalah delay antar request ke CFTC saat fetch batch data.
    COTFetcherDelay = 200 * time.Millisecond

    // PriceFetcherDelay adalah delay antar request ke price data provider (AlphaVantage/TwelveData).
    PriceFetcherDelay = 300 * time.Millisecond

    // NewsFetcherDelay adalah delay antar fetch cycle di news scheduler.
    NewsFetcherDelay = 3 * time.Second
)
```

Kemudian replace semua literal values dengan constants ini.

---

## Acceptance Criteria

- [ ] `internal/config/constants.go` dibuat
- [ ] `MinAlertStrength` digunakan di scheduler.go dan backtest/stats.go
- [ ] Delay constants digunakan di semua sleep locations
- [ ] `go build ./...` clean
- [ ] `go test ./...` pass
- [ ] TIDAK ada behavior change (nilai sama persis, hanya pindah ke named constant)

---

## Implementation Notes

1. Verifikasi dulu bahwa semua 50ms sleep memang Telegram flood — ada yang mungkin untuk purpose lain
2. Cek `bot.go:870` — `35*time.Millisecond - sinceLastSend` pakai duration berbeda, jangan salah replace
3. Pakai `grep -rn "time.Sleep" internal/` untuk temukan semua lokasi

---

## Assigned To

(unassigned — AMAN dikerjakan parallel, tidak conflict dengan task lain)
