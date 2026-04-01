# TASK-008: Market Breadth via Firecrawl (barchart.com)

**Priority:** medium
**Type:** data
**Estimated:** M
**Area:** internal/service | internal/domain
**Created by:** Research Agent
**Created at:** 2026-04-01 00:00 WIB
**Siklus:** Data

## Deskripsi
Tambahkan market breadth data dari `barchart.com/stocks/market-pulse` via Firecrawl. Data ini mencakup % NYSE/S&P stocks above 50MA, % above 200MA, advance-decline line, dan new 52-week highs/lows. Integrasikan ke `SentimentData` atau sebagai struct baru di `UnifiedOutlookData`.

## Konteks
Market breadth adalah indikator health dari keseluruhan pasar saham yang sangat relevan sebagai risk sentiment proxy untuk forex:
- **% above 200MA < 30%**: pasar dalam kondisi lemah secara breadth — risk-off bias
- **% above 200MA > 70%**: pasar sehat secara breadth — risk-on
- **Breadth divergence**: harga indeks naik tapi breadth melemah → distribusi, bearish warning
- **Complement**: berpasangan dengan CNN F&G, AAII, dan CBOE P/C untuk picture sentiment lengkap

barchart.com/stocks/market-pulse adalah public page, tanpa auth. Firecrawl sudah ada.

DATA_SOURCES_AUDIT.md mencantumkan ini: "Market breadth data — barchart.com/stocks/market-pulse (via Firecrawl)".

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat `internal/domain/market_breadth.go` dengan struct `MarketBreadthData`:
  - `PctAbove50MA float64` — % S&P stocks above 50-day MA
  - `PctAbove200MA float64` — % S&P stocks above 200-day MA
  - `AdvanceDeclineRatio float64` — advance / decline ratio
  - `New52WkHighs int` — new 52-week highs
  - `New52WkLows int` — new 52-week lows
  - `Available bool`
  - `FetchedAt time.Time`
- [ ] Buat `internal/service/sentiment/breadth.go` dengan fungsi `FetchMarketBreadth(ctx context.Context) (*domain.MarketBreadthData, error)` via Firecrawl JSON extraction
- [ ] Jika `FIRECRAWL_API_KEY` tidak di-set → return Available=false, no error
- [ ] `SentimentData` di `sentiment.go` ditambah field `MarketBreadth *domain.MarketBreadthData`
- [ ] `FetchSentiment()` memanggil `FetchMarketBreadth()` dan mengisi field tersebut
- [ ] `FormatSentiment()` di `formatter.go` menampilkan market breadth jika available
- [ ] `BuildUnifiedOutlookPrompt()` di `unified_outlook.go` menyertakan breadth data di section MARKET SENTIMENT

## File yang Kemungkinan Diubah
- `internal/domain/market_breadth.go` (baru)
- `internal/service/sentiment/breadth.go` (baru)
- `internal/service/sentiment/sentiment.go` (struct SentimentData + FetchSentiment)
- `internal/service/ai/unified_outlook.go` (section MARKET SENTIMENT)
- `internal/adapter/telegram/formatter.go` (FormatSentiment)

## Referensi
- `.agents/research/2026-04-01-00-data-integrasi-baru.md`
- `.agents/DATA_SOURCES_AUDIT.md` (section "Sumber yang bisa di-scrape via Firecrawl")
- `internal/service/sentiment/cboe.go` (pattern Firecrawl integration)
- `internal/service/sentiment/sentiment.go` (SentimentData struct + FetchSentiment pattern)
