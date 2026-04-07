# TASK-233: SEC EDGAR Form 4 — Daily Insider Trading Signal

**Priority:** medium
**Type:** data
**Estimated:** M
**Area:** internal/service/sec/ (extend TASK-206 base), internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 19:00 WIB

## Deskripsi

**Berbeda dari TASK-206 (13F quarterly institutional):** Form 4 adalah laporan harian dari direktur, eksekutif, dan pemegang saham >10% ke SEC — wajib submit dalam 2 hari kerja setelah transaksi.

**Konfirmasi:** `data.sec.gov/submissions/CIK{padded_cik}.json` sudah ditest live dan berfungsi tanpa API key (hanya User-Agent header). Response mengembalikan semua filings terbaru termasuk Form 4.

**Signal value:** Insider cluster buying (CEO + CFO + 3 direktur beli dalam 2 minggu) adalah salah satu signal paling reliable untuk reversal bullish di equities. Digunakan sebagai konteks macro/sentiment untuk trader forex+crypto karena equity insider action mencerminkan confidence eksekutif terhadap ekonomi.

## File yang Harus Dibuat/Diubah

- `internal/service/sec/form4.go` — NEW: Form 4 fetcher & parser
- `internal/service/sec/models.go` — Extend/create: Form4Filing struct
- `internal/adapter/telegram/handler.go` — Expose via /outlook atau /sentiment
- `internal/service/ai/unified_outlook.go` — Tambah insider signal ke AI context

## Implementasi

### Target Companies untuk Monitoring

Fokus pada S&P 500 bellwether companies sebagai macro proxy:
```go
var form4TargetCIKs = map[string]string{
    "0000320193": "Apple (AAPL)",
    "0001018724": "Amazon (AMZN)",
    "0001045810": "NVIDIA (NVDA)",
    "0001326801": "Meta (META)",
    "0001652044": "Alphabet (GOOGL)",
    "0000789019": "Microsoft (MSFT)",
    "0001067983": "Berkshire Hathaway (BRK)",
    "0001467858": "JPMorgan Chase (JPM)",
}
```

### form4.go — Fetch recent Form 4 filings

```go
// FetchRecentForm4 fetches Form 4 filings from the past N days for a CIK.
// Returns parsed transactions: acquision (P = buy) vs disposition (S = sell).
func FetchRecentForm4(ctx context.Context, cik string, lookbackDays int) ([]Form4Filing, error) {
    // GET https://data.sec.gov/submissions/CIK{padded}.json
    // Filter: form == "4" AND filed >= lookbackDays ago
    // For each Form 4: parse XML from accession number path (complex step)
    // OR: use EDGAR full text search API for simpler approach
}
```

### Simpler approach — EDGAR full text search

```go
// EDGAR EFTS search (confirmed working):
// GET https://efts.sec.gov/LATEST/search-index?forms=4&entity={company}&dateRange=custom&startdt=...
// Returns filing metadata; parse issuerName, reportingName, transactionDate, shares, pricePerShare
```

### AI context output

```
## Insider Activity (Last 30 Days)
• NVDA CEO Jensen Huang: SOLD 500K shares @ $892 (2026-03-28)
• JPM CEO Jamie Dimon: PURCHASED 10K shares @ $207 (2026-03-25) — unusual for Dimon
→ Signal: Mixed. Tech insider selling, bank insider buying.
```

## Acceptance Criteria

- [ ] Fetch Form 4 filings dari 8 target companies via EDGAR API
- [ ] Parse: insider name, role, transaction type (Buy/Sell), date, shares, value
- [ ] Cluster detection: 3+ insiders dari perusahaan sama beli dalam 7 hari → alert
- [ ] Output tersedia di /outlook atau /sentiment sebagai section "Insider Activity"
- [ ] Cache: 6 jam (Form 4 data tidak update per menit)
- [ ] Jika EDGAR unreachable → skip gracefully dengan log warning
- [ ] Prioritaskan CIK yang punya large purchases (director buying own-company stock signifikan)

## Referensi

- `.agents/research/2026-04-02-19-data-fed-speeches-cg-trending-edgar-form4-putaran8.md` — Temuan 3
- TASK-206 — 13F quarterly (beda dari Form 4 harian ini)
- `data.sec.gov` — confirmed working, User-Agent header required
- `internal/service/sec/` — Jika TASK-206 sudah membuat folder, extend dari sana
