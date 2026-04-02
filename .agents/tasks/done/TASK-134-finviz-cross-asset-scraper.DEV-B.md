# TASK-134: Finviz Cross-Asset Sentiment Scraper via Firecrawl

**Priority:** medium
**Type:** data
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-02 02:00 WIB
**Siklus:** Data

## Deskripsi
Scrape Finviz.com via Firecrawl untuk cross-asset performance data: futures heatmap, sector rotation, FX performance. Useful sebagai risk-on/risk-off signal dan market context untuk forex analysis.

## Konteks
- Firecrawl key sudah ada
- Finviz clean scrapeable — markdown tables, easily parseable
- Pages: `/futures.ashx` (futures), `/forex.ashx` (FX), `/groups.ashx?g=sector&v=140` (sectors), `/forex_performance.ashx` (FX performance)
- Data delayed ~15min (free tier) — acceptable untuk macro context
- Ref: `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `internal/service/marketdata/finviz_client.go`
- [ ] Scrape futures page → extract major indices, energy, metals, currency futures
- [ ] Scrape sector performance → extract 11 sectors with multi-timeframe returns
- [ ] Parse Firecrawl markdown output ke structured data
- [ ] Cache di BadgerDB (TTL 1h)
- [ ] Expose via command baru `/market` atau extend `/sentiment`:
  - Section "Cross-Asset": futures green/red count, sector leaders/laggards
  - Risk-on/risk-off classification berdasarkan: equities up + gold down + yields up = risk-on
- [ ] Rate limit: max 2 scrapes per page per hour

## File yang Kemungkinan Diubah
- `internal/service/marketdata/finviz_client.go` (baru)
- `internal/adapter/telegram/handler.go` (new /market command atau extend /sentiment)
- `internal/adapter/telegram/formatter.go` (cross-asset formatter)

## Referensi
- `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`
# TASK-134: Finviz Cross-Asset Sentiment Scraper via Firecrawl — DONE

**Completed by:** Dev-B
**Completed at:** 2026-04-02 07:30 WIB
**Branch:** feat/TASK-134-finviz-cross-asset-scraper
**PR:** pending

## Summary

Implemented Finviz cross-asset market overview scraper using Firecrawl:
- New `internal/service/marketdata/finviz/client.go` — Firecrawl-powered scraper for futures + sector pages
- New `internal/adapter/telegram/handler_market.go` — /market command handler
- New `internal/adapter/telegram/formatter_market.go` — Telegram HTML formatter
- Registered /market command in handler.go

## Features
- Scrapes Finviz futures page → indices, energy, metals, currencies, bonds, agriculture
- Scrapes Finviz sector performance → 11 sectors with 1D/1W/1M returns
- Risk-on/risk-off classification (equities + gold + yields logic)
- In-memory cache with 1-hour TTL
- Graceful degradation if FIRECRAWL_API_KEY not set
- Leader/laggard sector identification

## Build Status
- go build ./... ✅
- go vet ./... ✅
