# TASK-133: TradingEconomics Macro Scraper via Firecrawl

**Priority:** medium
**Type:** data
**Estimated:** L
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-02 02:00 WIB
**Siklus:** Data

## Deskripsi
Scrape TradingEconomics.com via Firecrawl untuk key macro indicators per G10 country. Data: GDP growth, CPI, employment rate, manufacturing PMI, consumer sentiment. Enhance COT analysis dengan macro context per currency.

## Konteks
- Firecrawl key sudah ada di .env
- TradingEconomics scrapeable — calendar page returns 100+ events, indicator pages return structured data
- URL pattern: `tradingeconomics.com/{country}/{indicator}`
- Countries: united-states, euro-area, united-kingdom, japan, canada, australia, switzerland, new-zealand
- Indicators: gdp-growth-rate, inflation-cpi, unemployment-rate, manufacturing-pmi, consumer-confidence
- Supplement FRED data (US only) dengan global macro data
- Ref: `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `internal/service/macro/tradingeconomics_client.go`
- [ ] Scrape via Firecrawl: GDP, CPI, unemployment, PMI, consumer confidence untuk 8 countries
- [ ] Parse markdown output ke structured data (current value, previous, YoY change)
- [ ] Cache di BadgerDB (TTL 6h — macro data perubahan lambat)
- [ ] Expose via `/macro` command — tambah "Global Macro Dashboard" section
- [ ] Rate limit: max 1 scrape per country per 6h (jangan abuse Firecrawl quota)

## File yang Kemungkinan Diubah
- `internal/service/macro/tradingeconomics_client.go` (baru)
- `internal/adapter/telegram/handler.go` (extend /macro)
- `internal/adapter/telegram/formatter.go` (format global macro table)

## Referensi
- `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`
