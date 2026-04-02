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
