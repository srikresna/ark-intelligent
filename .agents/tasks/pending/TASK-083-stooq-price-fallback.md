# TASK-083: Stooq.com Historical Forex Fallback

**Priority:** MEDIUM
**Siklus:** 2 (Data & Integrasi Gratis)
**Estimasi:** 2-3 jam
**Area:** internal/service/price/

---

## Latar Belakang

Stooq.com menyediakan historical OHLCV data untuk semua major/minor forex pairs
secara GRATIS tanpa API key melalui direct CSV download:

URL pattern: `https://stooq.com/q/d/l/?s={pair}.fx&i=w`
Contoh EURUSD weekly: `https://stooq.com/q/d/l/?s=eurusd.fx&i=w`

Data format: CSV dengan kolom Date, Open, High, Low, Close, Volume

Price fetcher saat ini punya chain: TwelveData → AlphaVantage → Yahoo Finance
Stooq bisa jadi layer ke-4 sebagai additional fallback.

---

## Tujuan

Tambah Stooq.com sebagai fallback ke-4 di price fetcher chain untuk
meningkatkan reliability data historis tanpa biaya tambahan.

---

## Implementasi

### 1. Tambah metode `fetchFromStooq` di `internal/service/price/fetcher.go`

```go
// stooqSymbol converts ark-intelligent currency code to Stooq pair notation
// "EUR" → "eurusd" (paired with USD always)
// "XAUUSD" → "xauusd"
func stooqSymbol(currency string) string

// fetchFromStooq fetches weekly OHLCV from Stooq direct CSV endpoint.
// Returns []domain.PriceRecord sorted by date ascending.
func (f *Fetcher) fetchFromStooq(ctx context.Context, currency string, weeks int) ([]domain.PriceRecord, error)
```

### 2. Circuit Breaker

```go
// Tambah ke Fetcher struct:
cbStooq *circuitbreaker.Breaker
```

```go
// Inisialisasi di NewFetcher():
cbStooq: circuitbreaker.New("stooq", 3, 5*time.Minute),
```

### 3. Extend FetchAllDetailed() chain

Setelah Yahoo Finance fallback gagal, coba Stooq:

```go
// Di loop per contract di FetchAllDetailed:
case "stooq":
    records, err = f.cbStooq.Execute(func() (interface{}, error) {
        return f.fetchFromStooq(ctx, currency, weeks)
    })
```

### 4. CSV Parsing

```go
// Stooq CSV format:
// Date,Open,High,Low,Close,Volume
// 2026-03-28,1.08234,...
func parseStooqCSV(r io.Reader, currency string) ([]domain.PriceRecord, error) {
    csvReader := csv.NewReader(r)
    // skip header
    // parse rows into domain.PriceRecord
}
```

### 5. Symbol Mapping

```go
var stooqSymbols = map[string]string{
    "EUR":    "eurusd",
    "GBP":    "gbpusd",
    "JPY":    "usdjpy",  // inverse
    "AUD":    "audusd",
    "NZD":    "nzdusd",
    "CAD":    "usdcad",  // inverse
    "CHF":    "usdchf",  // inverse
    "XAUUSD": "xauusd",
    "XAGUSD": "xagusd",
}
```

Note: Untuk inverse pairs (JPY, CAD, CHF), data perlu diinverse 
(Close = 1/Close, Open = 1/Open, dll).

---

## Testing

- Unit test: TestStooqCSVParsing (sample CSV → []PriceRecord)
- Unit test: TestStooqInversePairs (JPY/CAD/CHF inversion logic)
- Integration test: fetchFromStooq("EUR", 52) returns 52 weekly candles
- Fallback test: jika TwelveData + AlphaVantage + Yahoo gagal → Stooq berhasil

---

## File yang Dimodifikasi

- `internal/service/price/fetcher.go` (tambah stooq methods + circuit breaker)
- `internal/service/price/fetcher_test.go` (tambah stooq tests)

---

## Referensi

- Stooq.com: https://stooq.com (free, no registration needed)
- Pola fallback: `internal/service/price/fetcher.go` — existing Yahoo fallback
- Research: `.agents/research/2026-04-01-10-data-integrasi-gratis.md`
