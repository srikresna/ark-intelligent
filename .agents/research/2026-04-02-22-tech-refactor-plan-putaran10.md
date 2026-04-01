# Research Report: Tech Refactor Siklus 4 — Putaran 10
# Contract Code Sprawl, Magic Durations, Error Package, fmtutil Gaps

**Date:** 2026-04-02 22:00 WIB
**Siklus:** 4/5 (Tech Refactor Plan) — Putaran 10
**Author:** Research Agent

---

## Ringkasan

Putaran 10 fokus pada TECH-006, TECH-007, TECH-010, dan TECH-011 dari TECH_REFACTOR_PLAN. Temuan utama: domain/contracts.go sudah dibuat (TECH-011) tapi masih ada 25 hardcoded contract code string di 3 file adapter. Magic time.Sleep tersebar di 10 lokasi. pkg/errs belum ada. fmtutil masih kekurangan 4 helper functions.

God class metrics terbaru:
- `formatter.go`: 4,539 LOC (naik dari 4,489 di plan — tambah +50 LOC)
- `handler.go`: 2,909 LOC (naik dari 2,381 di plan — tambah +528 LOC!)

---

## Temuan 1: TECH-011 Belum Selesai — Contract Codes Tersebar di 3 File Adapter

`internal/domain/contracts.go` sudah ada dan lengkap dengan:
- Konstanta `ContractEUR`, `ContractGBP`, dll.
- Map `ContractByCurrency` (currency → info)
- Map `ContractByCode` (code → info)
- Helper `CurrencyForCode()` dan `CodeForCurrency()`

**Namun, masih ada 25 hardcoded string literal contract codes di:**

### internal/adapter/telegram/handler.go (baris 1368–1398)
```go
func currencyToContractCode(currency string) string {
    m := map[string]string{
        "EUR": "099741",  // hardcoded!
        "GBP": "096742",
        "JPY": "097741",
        // ... 8 entries
    }
}
```
Padahal `domain.CodeForCurrency(currency)` sudah melakukan persis ini.

### internal/adapter/telegram/formatter.go (baris 267, 1042–1048)
```go
// Baris 267:
Codes: []string{"098662", "099741", "096742", ...}  // 8 hardcoded codes

// Baris 1042:
"099741": "EUR",
"096742": "GBP",
// ... hardcoded reverse map
```
Padahal `domain.FXContracts` dan `domain.ContractByCode` sudah tersedia.

### internal/adapter/telegram/keyboard.go
Juga mengandung hardcoded contract codes di keyboard button data.

**Risk:** Jika ada perubahan contract code di CFTC (sudah terjadi dulu untuk Bitcoin), harus update 4 files bukan 1.

---

## Temuan 2: TECH-006 — 10 Hardcoded time.Sleep Tidak Pakai Konstanta

`internal/config/constants.go` sudah punya `LongPollTimeout`, `PollRetryDelay` dll. Tapi belum ada konstanta untuk rate limiting delays yang tersebar di production code:

| File | Baris | Value | Konteks |
|------|-------|-------|---------|
| `scheduler/scheduler.go` | 353, 463, 551 | 50ms | Telegram flood limit |
| `service/news/scheduler.go` | 290, 491, 725, 767 | 50ms | Telegram flood limit |
| `adapter/telegram/api.go` | 512 | 35ms | Send rate limiting |
| `service/price/fetcher.go` | 669, 675 | 300ms | Price API rate limit |
| `service/cot/fetcher.go` | 174 | 200ms | COT API rate limit |

**10 total lokasi** menggunakan magic duration literals. Ada 6 duplikat 50ms yang seharusnya share 1 konstanta.

---

## Temuan 3: TECH-007 — pkg/errs Belum Dibuat

`pkg/` directory sudah ada dengan:
- `circuitbreaker/`
- `fmtutil/`
- `format/`
- `logger/`
- `mathutil/`
- `timeutil/`

Tapi **`pkg/errs/` belum ada**. Error handling masih inconsistent:

```go
// Pattern 1: bare return (banyak di storage repos)
return nil, err

// Pattern 2: fmt.Errorf wrap
return nil, fmt.Errorf("cot fetch: %w", err)

// Pattern 3: zerolog + return
log.Error().Err(err).Msg("failed to fetch")
return nil, err
```

Dan error silencing pattern dari putaran sebelumnya (10+ lokasi di fred/fetcher.go) belum difix.

---

## Temuan 4: TECH-010 — fmtutil Masih Kurang 4 Functions

`pkg/fmtutil/format.go` sudah punya 22 fungsi. Yang **belum ada** dari TECH_REFACTOR_PLAN:

| Function | Dibutuhkan Untuk |
|----------|-----------------|
| `FormatLargeNumber(n float64) string` | Market cap, volume formatting |
| `FormatPips(f float64) string` | FX pips display |
| `MessageHeader(title, emoji string) string` | Header formatting di formatters |
| `Divider() string` | Section separator baris |

Saat ini banyak formatter functions punya boilerplate `b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")` inline — bisa diganti dengan `fmtutil.Divider()`.

Terdapat **107 lokasi** `strings.Builder` yang bisa memanfaatkan helpers ini.

---

## Task Plan Putaran 10

| Task | TECH Ref | Scope | Estimasi |
|------|----------|-------|---------|
| TASK-240 | TECH-011 | handler.go: ganti currencyToContractCode dengan domain.CodeForCurrency | XS |
| TASK-241 | TECH-011 | formatter.go + keyboard.go: ganti hardcoded code maps dengan domain constants | S |
| TASK-242 | TECH-006 | config/constants.go: tambah TelegramFloodDelay, PriceFetchDelay, COTFetchDelay + ganti 10 magic Sleep | S |
| TASK-243 | TECH-007 | Buat pkg/errs dengan ErrNoData, ErrRateLimited, ErrNotFound + Wrap() helper | S |
| TASK-244 | TECH-010 | pkg/fmtutil: tambah FormatLargeNumber, FormatPips, MessageHeader, Divider | XS |

---

## Referensi Codebase

- `internal/domain/contracts.go` — existing contract constants (sudah lengkap, tinggal dipakai)
- `internal/config/constants.go` — existing config constants (perlu ditambah)
- `pkg/fmtutil/format.go` — existing fmtutil (perlu diperluas)
- `internal/adapter/telegram/handler.go:1368` — `currencyToContractCode()` target replace
- `internal/adapter/telegram/formatter.go:267,1042` — hardcoded code maps target replace
