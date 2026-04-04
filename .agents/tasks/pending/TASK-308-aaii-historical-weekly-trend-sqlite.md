# TASK-308: AAII Historical Weekly Trend — SQLite Storage + 4-Week Average

**Priority:** medium
**Type:** data-enhancement
**Estimated:** M
**Area:** internal/service/sentiment/sentiment.go, internal/db/ (SQLite), internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-04 01:00 WIB
**Siklus:** Data-2 (Siklus 2 Putaran 23)
**Ref:** research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md

## Deskripsi

AAII Investor Sentiment Survey hanya mengambil data **minggu terbaru** dan tidak menyimpan historical. Ini menyebabkan kehilangan informasi penting:

1. **Trend tidak terdeteksi:** Bull-Bear = -15% tapi 4 minggu lalu -30% → sentiment sedang *membaik* (bullish signal). Tanpa history, kita tidak tahu ini.
2. **Extreme readings butuh context:** Bull-Bear = +25% — apakah ini level tertinggi 3 bulan atau normal?
3. **4-week MA divergence:** Saat AAII saat ini vs 4wk average menjauh drastis → potential reversal signal.

**Contoh output yang diinginkan:**
```
AAII Investor Sentiment (minggu 2026-03-25)
Bull      : 45.6%   (+8.2pp vs 4wk avg)
Bear      : 28.1%   (-5.1pp vs 4wk avg)
Bull-Bear : +17.5   [4wk avg: +3.2] ↑ Sentimen membaik
```

## Implementasi

### Step 1: Schema SQLite table baru

```sql
CREATE TABLE IF NOT EXISTS aaii_sentiment_history (
    week_date TEXT PRIMARY KEY,  -- "2026-03-25" (normalized)
    bullish   REAL NOT NULL,     -- % bullish
    bearish   REAL NOT NULL,     -- % bearish
    neutral   REAL NOT NULL,     -- % neutral
    bull_bear REAL NOT NULL,     -- bull - bear spread
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Step 2: Tambah ke `internal/db/` atau `internal/service/sentiment/aaii_history.go`

Fungsi yang dibutuhkan:
```go
// SaveAAIIWeeklyReading upserts a weekly AAII reading into the history table.
func SaveAAIIWeeklyReading(db *sql.DB, weekDate string, bullish, bearish, neutral float64) error

// GetAAIIHistory returns the last N weekly readings, most recent first.
func GetAAIIHistory(db *sql.DB, n int) ([]AAIIWeeklyReading, error)

// ComputeAAII4WeekAverage returns the simple average of Bull-Bear spread over last 4 weeks.
func ComputeAAII4WeekAverage(db *sql.DB) (float64, error)
```

### Step 3: Tambah fields ke SentimentData

```go
// AAII Historical Trend (populated from SQLite if available)
AAII4WeekAvgBullBear  float64 // 4-week moving average of Bull-Bear spread
AAIIBullBearVs4WkAvg  float64 // current - 4wk avg (positive = getting more bullish)
AAIITrend             string  // "IMPROVING", "DETERIORATING", "STABLE"
AAIIHistoryAvailable  bool
```

### Step 4: Wire ke sentiment fetcher

Setelah `fetchAAIISentiment()` berhasil, panggil:
```go
if data.AAIIAvailable {
    // Store reading
    if err := SaveAAIIWeeklyReading(db, data.AAIIWeekDate, data.AAIIBullish, data.AAIIBearish, data.AAIINeutral); err != nil {
        log.Warn().Err(err).Msg("AAII: failed to store history")
    }
    // Compute 4-week average
    avg4w, err := ComputeAAII4WeekAverage(db)
    if err == nil {
        data.AAII4WeekAvgBullBear  = avg4w
        data.AAIIBullBearVs4WkAvg  = data.AAIIBullBear - avg4w
        data.AAIIHistoryAvailable  = true
        switch {
        case data.AAIIBullBearVs4WkAvg > 5:
            data.AAIITrend = "IMPROVING"
        case data.AAIIBullBearVs4WkAvg < -5:
            data.AAIITrend = "DETERIORATING"
        default:
            data.AAIITrend = "STABLE"
        }
    }
}
```

### Step 5: Perlu akses ke `*sql.DB` di sentiment package

**Challenge:** `sentiment` package saat ini tidak punya akses ke database.

**Options:**
A. Pass `*sql.DB` ke `SentimentFetcher` (via constructor injection)
B. Buat `aaii_history.go` di `internal/service/sentiment/` dengan own DB connection
C. Move history storage ke caller (`handler.go`) setelah fetch

**Recommended: Option A** — tambah `DB *sql.DB` ke SentimentFetcher:
```go
type SentimentFetcher struct {
    httpClient *http.Client
    db         *sql.DB  // nil if not available — history skipped gracefully
    // ... circuit breakers
}

func (f *SentimentFetcher) WithDB(db *sql.DB) *SentimentFetcher {
    f.db = db
    return f
}
```

### Step 6: Formatter update

```go
if data.AAIIHistoryAvailable {
    trendEmoji := "→"
    if data.AAIITrend == "IMPROVING"    { trendEmoji = "↑" }
    if data.AAIITrend == "DETERIORATING" { trendEmoji = "↓" }

    b.WriteString(fmt.Sprintf("<code>Bull-Bear : %+.1f   [4wk avg: %+.1f] %s</code>\n",
        data.AAIIBullBear, data.AAII4WeekAvgBullBear, trendEmoji))
} else {
    b.WriteString(fmt.Sprintf("<code>Bull-Bear : %+.1f</code>\n", data.AAIIBullBear))
}
```

## Acceptance Criteria

- [ ] `aaii_sentiment_history` tabel dibuat otomatis saat app start (IF NOT EXISTS)
- [ ] Setiap berhasil fetch AAII → data disimpan ke SQLite (upsert by week_date)
- [ ] `AAII4WeekAvgBullBear` diisi dari history jika ≥4 readings tersedia
- [ ] `AAIITrend` = "IMPROVING"/"DETERIORATING"/"STABLE" berdasarkan threshold ±5pp
- [ ] Formatter menampilkan 4-week average dan trend arrow
- [ ] Jika history < 4 readings → `AAIIHistoryAvailable = false`, tampilkan normal (no crash)
- [ ] Jika `db = nil` → skip history gracefully
- [ ] `go build ./...` clean

## Ketergantungan

- Perlu akses `*sql.DB` — cek bagaimana `internal/db/` diinisialisasi di `cmd/bot/main.go`
- DB sudah ada (SQLite) — lihat `internal/db/*.go` untuk pattern existing tables

## Referensi

- `internal/service/sentiment/sentiment.go` — SentimentData struct, fetchAAIISentiment()
- `internal/db/` — pattern create table, upsert
- `cmd/bot/main.go` — DB initialization, injection ke services
- research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md
