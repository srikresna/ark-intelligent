# TASK-307: Tampilkan Data Age di /sentiment Output

**Priority:** medium
**Type:** ux-improvement
**Estimated:** S
**Area:** internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-04 01:00 WIB
**Siklus:** Data-2 (Siklus 2 Putaran 23)
**Ref:** research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md

## Deskripsi

`SentimentData.FetchedAt` diisi dengan `time.Now()` saat fetch, tapi **tidak pernah ditampilkan** ke user di formatter.

**Problem nyata:** AAII survey update **mingguan** (setiap Rabu). Kalau user request /sentiment hari Selasa, AAII data sudah **6 hari lama**. User tidak tahu ini dan mungkin salah interpretasi sentiment.

**Current output:**
```
AAII Investor Sentiment Survey
Bull      : 45.6%
Bear      : 28.1%
Neutral   : 26.3%
Bull-Bear : +17.5
```

**Expected output:**
```
AAII Investor Sentiment Survey (minggu 2026-03-25, 10 hari lalu)
Bull      : 45.6%
Bear      : 28.1%
Neutral   : 26.3%
Bull-Bear : +17.5
```

## Cakupan Perubahan

Semua section di `FormatSentiment()`:

| Section | Timing info yang ditampilkan |
|---|---|
| CNN Fear & Greed | "3h lalu" atau "hari ini" |
| AAII Sentiment | "minggu YYYY-MM-DD (X hari lalu)" |
| CBOE Put/Call | "kemarin (market close)" atau "hari ini" |
| Crypto F&G | "X menit lalu" |
| VIX Term Structure | "X menit lalu" |
| NAAIM (jika TASK-305 done) | "minggu YYYY-MM-DD" |

## Implementasi

### Helper function

Tambah helper di `formatter.go` atau `fmtutil`:

```go
// fmtDataAge returns a human-readable string for how old data is.
// Examples: "baru saja", "3 jam lalu", "1 hari lalu", "6 hari lalu"
func fmtDataAge(fetchedAt time.Time) string {
    if fetchedAt.IsZero() {
        return ""
    }
    d := time.Since(fetchedAt)
    switch {
    case d < 5*time.Minute:
        return "baru saja"
    case d < 60*time.Minute:
        return fmt.Sprintf("%d mnt lalu", int(d.Minutes()))
    case d < 24*time.Hour:
        return fmt.Sprintf("%.0f jam lalu", d.Hours())
    default:
        days := int(d.Hours() / 24)
        return fmt.Sprintf("%d hari lalu", days)
    }
}
```

### Integrasi ke FormatSentiment

SentimentData punya `FetchedAt` (waktu fetch keseluruhan), tapi tidak per-source.

Dua pendekatan:
1. **Simple (recommended):** Gunakan `SentimentData.FetchedAt` untuk semua — "data /sentiment diambil X menit lalu"
2. **Detailed:** Tambah per-source FetchedAt fields (terlalu banyak perubahan struct)

**Recommended: tambah 1 baris footer:**

```go
// Di akhir FormatSentiment(), sebelum return:
if !data.FetchedAt.IsZero() {
    age := fmtDataAge(data.FetchedAt)
    b.WriteString(fmt.Sprintf("\n<i>Data diambil %s</i>\n", age))
}
```

**Untuk AAII (data mingguan), tambahkan inline:**

```go
// Di section AAII, gunakan NAAIMWeekDate atau field tanggal survey AAII
if data.AAIIAvailable && data.AAIIWeekDate != "" {
    // AAIIWeekDate sudah ada di SentimentData ("3/25/2026")
    b.WriteString(fmt.Sprintf("<b>AAII Investor Sentiment</b> <i>(minggu %s)</i>\n", data.AAIIWeekDate))
} else {
    b.WriteString("<b>AAII Investor Sentiment</b>\n")
}
```

### Footer diagnostics (lihat juga TASK-309)

Jika beberapa source tidak tersedia, tampilkan di footer:

```go
// Count available sources
available := 0
total := 0
sourceStatus := make([]string, 0)

total++; if data.CNNAvailable    { available++; sourceStatus = append(sourceStatus, "CNN✅")    } else { sourceStatus = append(sourceStatus, "CNN❌") }
total++; if data.AAIIAvailable   { available++; sourceStatus = append(sourceStatus, "AAII✅")   } else { sourceStatus = append(sourceStatus, "AAII❌") }
total++; if data.PutCallAvailable { available++; sourceStatus = append(sourceStatus, "CBOE✅")  } else { sourceStatus = append(sourceStatus, "CBOE❌") }
total++; if data.CryptoFearGreedAvailable { available++; sourceStatus = append(sourceStatus, "CryptoFG✅") } else { sourceStatus = append(sourceStatus, "CryptoFG❌") }
total++; if data.VIXAvailable    { available++; sourceStatus = append(sourceStatus, "VIX✅")    } else { sourceStatus = append(sourceStatus, "VIX❌") }

if available < total {
    b.WriteString(fmt.Sprintf("\n<i>📡 %d/%d sumber: %s</i>\n",
        available, total, strings.Join(sourceStatus, " ")))
}
```

## Acceptance Criteria

- [ ] Header AAII section menampilkan `AAIIWeekDate` (sudah ada di struct)
- [ ] Footer `/sentiment` menampilkan waktu fetch (`FetchedAt`)
- [ ] Helper `fmtDataAge()` menghasilkan string yang human-readable
- [ ] Ketika data berumur 0-5 menit → "baru saja"
- [ ] Ketika AAII berumur 6 hari → "6 hari lalu"
- [ ] Footer diagnostics muncul jika ada source yang unavailable
- [ ] `go build ./...` clean
- [ ] Test: request /sentiment → periksa footer menampilkan usia data yang benar

## Referensi

- `internal/adapter/telegram/formatter.go` — FormatSentiment() function
- `internal/service/sentiment/sentiment.go:115` — SentimentData struct (FetchedAt, AAIIWeekDate)
- `pkg/fmtutil/` — cek apakah ada helper format yang bisa digunakan
- research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md
