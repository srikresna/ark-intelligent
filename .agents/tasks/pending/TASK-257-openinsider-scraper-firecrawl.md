# TASK-257: OpenInsider Scraper — Insider Trading Flow via Firecrawl ke SentimentData

**Priority:** low
**Type:** data-source
**Estimated:** M
**Area:** internal/service/sentiment/sentiment.go, internal/service/sentiment/insider.go (new), internal/service/ai/unified_outlook.go
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

openinsider.com adalah SEC Form 4 aggregator yang bisa di-scrape via Firecrawl.
Diverifikasi pada 2026-04-01: Firecrawl JSON extraction berhasil mengekstrak
ticker, company, insider name, trade type (buy/sell), value, date.

Insider cluster buying (beberapa insider di sektor/saham yang sama membeli dalam
waktu dekat) adalah leading indicator bullish. Insider selling lebih ambiguous
(bisa karena diversifikasi) tapi perlu ditrack.

Berguna sebagai satu blok tambahan di unified_outlook: "INSIDER FLOW SUMMARY"
memberikan AI konteks tentang smart money equity positioning.

**Catatan:** Firecrawl key sudah ada di .env → tidak ada biaya tambahan.
Cache 24 jam sudah cukup (Form 4 update harian).

## File yang Harus Dibuat/Diubah

1. `internal/service/sentiment/insider.go` — new file: Firecrawl scrape openinsider.com
2. `internal/service/sentiment/sentiment.go` — tambah `InsiderFlow *InsiderFlowData` + fetch
3. `internal/service/ai/unified_outlook.go` — section "INSIDER FLOW" di prompt

## Implementasi

### 1. internal/service/sentiment/insider.go (new)

```go
package sentiment

// InsiderTrade holds a single insider transaction.
type InsiderTrade struct {
    Ticker   string // e.g. "AAPL"
    Company  string // e.g. "Apple Inc"
    Insider  string // e.g. "Cook Timothy D."
    TradeType string // "P - Purchase" or "S - Sale"
    Value    string // e.g. "$1,234,567"
    Date     string // e.g. "2026-04-01"
}

// InsiderFlowData holds recent insider trades fetched from openinsider.com.
type InsiderFlowData struct {
    Trades    []InsiderTrade
    BuyCount  int    // number of purchases in dataset
    SellCount int    // number of sales in dataset
    Available bool
}

// firecrawl schema + fetch logic (similar to AAII/CBOE implementation)
// URL: https://openinsider.com
// JSON extraction: ticker, company, insider, trade_type, value, date
// Cache: 24h (Form 4 updates daily)
```

JSON schema untuk Firecrawl:
```json
{
    "type": "object",
    "properties": {
        "trades": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "ticker":     {"type": "string"},
                    "company":    {"type": "string"},
                    "insider":    {"type": "string"},
                    "trade_type": {"type": "string"},
                    "value":      {"type": "string"},
                    "date":       {"type": "string"}
                }
            }
        }
    }
}
```

Prompt Firecrawl: `"Extract the 10 most recent insider transactions from the Latest
Insider Trading table: ticker, company name, insider name, transaction type
(P-Purchase or S-Sale), transaction value, and filing date."`

### 2. sentiment.go — integrate

Di `SentimentData` struct:
```go
InsiderFlow *InsiderFlowData
```

Di `Fetch()`, tambah circuit breaker call (similar ke AAII):
```go
if err := f.cbInsider.Execute(func() error {
    fetchInsiderFlow(ctx, f.httpClient, data)
    if !data.InsiderFlow.Available {
        if os.Getenv("FIRECRAWL_API_KEY") != "" {
            return fmt.Errorf("insider flow unavailable")
        }
    }
    return nil
}); err != nil {
    log.Debug().Err(err).Msg("sentiment: insider circuit breaker rejected")
}
```

Cache TTL untuk insider: 24 jam (bukan 6 jam seperti sentiment lain).
Perlu separate cache atau TTL override di cache.go.

### 3. unified_outlook.go — "INSIDER FLOW" section

```go
if data.SentimentData != nil && data.SentimentData.InsiderFlow != nil {
    flow := data.SentimentData.InsiderFlow
    if flow.Available && len(flow.Trades) > 0 {
        b.WriteString(fmt.Sprintf("=== %d. INSIDER TRADING FLOW (SEC Form 4) ===\n", section))
        section++
        b.WriteString(fmt.Sprintf("Recent: %d buys, %d sells\n", flow.BuyCount, flow.SellCount))
        for _, t := range flow.Trades {
            if t.TradeType == "P - Purchase" {
                b.WriteString(fmt.Sprintf("  BUY  %s (%s) %s by %s\n",
                    t.Ticker, t.Company, t.Value, t.Insider))
            }
        }
        b.WriteString("\n")
    }
}
```

## Acceptance Criteria

- [ ] `internal/service/sentiment/insider.go` compile clean
- [ ] `fetchInsiderFlow()` berhasil scrape openinsider.com via Firecrawl
- [ ] Jika FIRECRAWL_API_KEY tidak set → `InsiderFlow.Available = false`, tidak crash
- [ ] `SentimentData.InsiderFlow` ter-populate saat fetch
- [ ] `/outlook` prompt includes "INSIDER TRADING FLOW" section
- [ ] Cache insider data 24 jam (lebih panjang dari sentiment 6 jam)
- [ ] Circuit breaker `cbInsider` ditambah ke `SentimentFetcher`
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-09-data-sources-audit-gaps-putaran13.md` — Temuan #5
- Verified: openinsider.com scrapeable via Firecrawl, data ter-extract (tested 2026-04-01)
- `internal/service/sentiment/sentiment.go:60` — SentimentData struct
- `internal/service/sentiment/sentiment.go:70` — SentimentFetcher struct (tambah cbInsider)
- `internal/service/sentiment/sentiment.go:208` — fetchAAIISentiment() — pola referensi
- `internal/service/ai/unified_outlook.go:22` — UnifiedOutlookData
