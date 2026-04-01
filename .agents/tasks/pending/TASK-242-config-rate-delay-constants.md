# TASK-242: TECH-006 — Tambah Rate Delay Constants ke config/constants.go

**Priority:** medium
**Type:** refactor
**Estimated:** S
**Area:** internal/config/constants.go, internal/scheduler/scheduler.go, internal/service/news/scheduler.go, internal/adapter/telegram/api.go, internal/service/price/fetcher.go, internal/service/cot/fetcher.go
**Created by:** Research Agent
**Created at:** 2026-04-02 22:00 WIB

## Deskripsi

Terdapat **10 lokasi** hardcoded `time.Sleep` dengan magic duration literals di production code yang seharusnya menggunakan named constants. Ini bagian dari TECH-006 (Magic Numbers & Strings).

`internal/config/constants.go` sudah punya timing constants (`LongPollTimeout`, `PollRetryDelay`, dll) tapi belum ada constants untuk rate limiting delays.

## Lokasi Magic Durations yang Perlu Difix

| File | Baris | Value | Konteks |
|------|-------|-------|---------|
| `scheduler/scheduler.go` | 353 | `50 * time.Millisecond` | Telegram flood delay |
| `scheduler/scheduler.go` | 463 | `50 * time.Millisecond` | Telegram flood delay |
| `scheduler/scheduler.go` | 551 | `50 * time.Millisecond` | Telegram flood delay |
| `service/news/scheduler.go` | 290 | `50 * time.Millisecond` | Telegram flood delay |
| `service/news/scheduler.go` | 491 | `50 * time.Millisecond` | Telegram flood delay |
| `service/news/scheduler.go` | 725 | `50 * time.Millisecond` | Telegram flood delay |
| `service/news/scheduler.go` | 767 | `50 * time.Millisecond` | Telegram flood delay |
| `adapter/telegram/api.go` | 512 | `35 * time.Millisecond` | Telegram send rate |
| `service/price/fetcher.go` | 669 | `300 * time.Millisecond` | Price API rate limit |
| `service/price/fetcher.go` | 675 | `300 * time.Millisecond` | Price API rate limit |
| `service/cot/fetcher.go` | 174 | `200 * time.Millisecond` | COT fetch delay |

## Implementasi

### Step 1: Tambah ke internal/config/constants.go

```go
// ---------------------------------------------------------------------------
// Rate Limiting Delays
// ---------------------------------------------------------------------------

const (
    // TelegramFloodDelay is the sleep duration between bulk Telegram sends
    // to avoid hitting Telegram's flood-control limits (30 msgs/sec).
    TelegramFloodDelay = 50 * time.Millisecond

    // TelegramSendRateDelay is the enforced delay between individual API sends
    // in the Telegram client to stay within rate limits.
    TelegramSendRateDelay = 35 * time.Millisecond

    // PriceFetchDelay is the sleep duration between price API requests
    // to respect rate limits of external price data providers.
    PriceFetchDelay = 300 * time.Millisecond

    // COTFetchDelay is the sleep duration between COT data requests
    // to avoid overloading the CFTC data endpoint.
    COTFetchDelay = 200 * time.Millisecond
)
```

### Step 2: Ganti di semua file

```go
// Sebelum:
time.Sleep(50 * time.Millisecond) // Avoid Telegram flood

// Sesudah:
time.Sleep(config.TelegramFloodDelay)
```

Pastikan setiap file yang diubah mengimport `"github.com/arkcode369/ark-intelligent/internal/config"`.

## Aturan Refactor

- **NO behavior change** — nilai delay harus identik
- Jangan ubah nilai delay (50ms tetap 50ms, bukan tuning)
- `go build ./...` harus bersih
- `go vet ./...` harus bersih

## Acceptance Criteria

- [ ] 4 konstanta baru ditambah ke `internal/config/constants.go` dengan komentar
- [ ] Semua 10+ lokasi `time.Sleep` magic literals di-replace dengan constants
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Tidak ada perubahan timing behavior

## Referensi

- `.agents/research/2026-04-02-22-tech-refactor-plan-putaran10.md` — Temuan 2
- `internal/config/constants.go` — file target untuk constants baru
- `TECH_REFACTOR_PLAN.md#TECH-006` — Magic Numbers & Strings
