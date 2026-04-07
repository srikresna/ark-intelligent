# TASK-235: CBOE SKEW Index → VIX Term Structure + /sentiment Display

**Priority:** high
**Type:** data
**Estimated:** S
**Area:** internal/service/vix/, internal/service/sentiment/, internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-02 21:00 WIB

## Deskripsi

CBOE SKEW Index mengukur tail risk / biaya hedging terhadap crash (OTM puts vs ATM). SKEW >145 = institutions secara aktif melindungi portfolio. SKEW <115 = complacency.

Saat ini `service/vix/` mengambil VIX spot, VX futures M1/M2/M3, VVIX — tetapi **tidak mengambil SKEW**. Data tersedia di CBOE CDN dengan format identik yang sudah dipakai.

`/sentiment` harus menampilkan SKEW sebagai bagian dari volatility surface section, dengan cross-signal VIX × SKEW.

## Verified Data Source (GRATIS):
```
URL: https://cdn.cboe.com/api/global/us_indices/daily_prices/SKEW_EOD.csv
Format: Date,Open,High,Low,Close  (identik dengan VIX_EOD.csv dan VVIX_EOD.csv)
Fungsi yang sudah ada: fetchSingleIndexCSV() — cukup 1 constant URL baru
```

## File yang Harus Diubah

- `internal/service/vix/fetcher.go` — tambah `skewEODURL` constant + fetch SKEW di `FetchTermStructure()`
- `internal/service/vix/types.go` — tambah `SKEW float64` field ke `VIXTermStructure`
- `internal/service/sentiment/sentiment.go` — tambah `SKEWValue float64`, `SKEWAvailable bool` ke `SentimentData`
- `internal/service/sentiment/sentiment.go` — pass SKEW dari VIX data ke SentimentData
- `internal/adapter/telegram/formatter.go` — tampilkan SKEW di `FormatSentiment()` dengan classification + cross-signal

## Implementasi

### vix/fetcher.go
```go
const skewEODURL = "https://cdn.cboe.com/api/global/us_indices/daily_prices/SKEW_EOD.csv"

// Di FetchTermStructure(), setelah VVIX:
skew, err := fetchSingleIndexCSV(ctx, client, skewEODURL)
if err == nil {
    ts.SKEW = skew
}
```

### SKEW Signal Classification
```go
func ClassifySKEW(skew float64) (level, description string) {
    switch {
    case skew >= 150:
        return "EXTREME TAIL RISK", "Heavy crash hedging active — institutions buying OTM puts aggressively"
    case skew >= 140:
        return "ELEVATED", "Active tail hedging — market pricing significant downside risk"
    case skew >= 120:
        return "NORMAL", "Normal tail risk premium — typical market conditions"
    case skew >= 110:
        return "LOW", "Low tail risk concern — complacency in crash protection"
    default:
        return "VERY LOW", "Extreme complacency — very low demand for crash protection"
    }
}
```

### Cross-signal VIX × SKEW
```
VIX rendah (<20) + SKEW tinggi (>140) = 🟡 STEALTH HEDGE — Institutions buy protection quietly
VIX tinggi (>30) + SKEW rendah (<120)  = 🟢 CAPITULATION — Panic selling, not hedging; potential bottom
VIX tinggi (>30) + SKEW tinggi (>140)  = 🔴 FULL PANIC — Crash hedging + fear elevated; high-stress environment
VIX rendah (<20) + SKEW rendah (<120)  = 🟢 RISK-ON — Genuine low-risk environment
```

### formatter.go — tampilan di /sentiment
```
📊 VIX Volatility Surface
VIX Spot : 18.4  → RISK_ON_NORMAL
SKEW     : 138.2 → ELEVATED
Signal   : 🟡 VIX rendah tapi SKEW elevated — institutional crash hedging silent
```

## Acceptance Criteria

- [ ] `ts.SKEW` diisi dari CBOE CDN di setiap call `FetchTermStructure()`
- [ ] Jika CBOE CDN SKEW gagal → SKEW = 0 (non-fatal, sama dengan VVIX)
- [ ] `SentimentData.SKEWValue` dan `SKEWAvailable` terisi dari VIX data
- [ ] `/sentiment` menampilkan SKEW value + level classification
- [ ] Cross-signal VIX × SKEW tampil jika keduanya available
- [ ] Unit test: `TestClassifySKEW` memverifikasi 5 level classification

## Referensi

- `.agents/research/2026-04-02-21-feature-gaps-skew-credit-ict-pdarray-cot-seasonal-putaran9.md` — Temuan 1
- `internal/service/vix/fetcher.go:fetchSingleIndexCSV()` — fungsi fetch yang sudah ada (reuse)
- `internal/service/vix/types.go:VIXTermStructure` — struct yang perlu ditambah field SKEW
- `internal/service/sentiment/cboe.go` — pola integrasi sentiment dari external data
