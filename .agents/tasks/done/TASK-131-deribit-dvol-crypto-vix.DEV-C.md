# TASK-131: Deribit DVOL Index (Crypto VIX Equivalent) - DONE

**Completed by:** Agent Dev-C
**Completed at:** 2026-04-02 03:55 WIB
**Branch:** feat/TASK-131-deribit-dvol-crypto-vix
**PR:** pending

## Changes Made

### New Files
- `internal/service/marketdata/deribit/dvol.go` — GetDVOL() and GetHistoricalVolatility() endpoints
- `internal/service/dvol/dvol.go` — DVOL analysis engine with caching (1h TTL)
- `internal/service/sentiment/dvol_integration.go` — IntegrateDVOLIntoSentiment helper

### Modified Files
- `internal/service/sentiment/sentiment.go` — Added DVOL fields to SentimentData, DVOL circuit breaker, fetch integration
- `internal/adapter/telegram/format_macro.go` — Added "Crypto Volatility (Deribit DVOL)" section to sentiment dashboard

## Features
- Fetches BTC and ETH DVOL (30-day implied vol) candles from Deribit public API
- Fetches historical (realized) volatility for IV/HV spread analysis
- 24h range, change%, spike detection (>20% move)
- IV-HV spread signal: EXTREME FEAR PREMIUM → EXTREME COMPLACENCY
- Cross-asset vol comparison: BTC DVOL/VIX ratio
- Circuit breaker protection (3 failures → 10min cooldown)
- 1h cache TTL for DVOL data
- Integrated into /sentiment command output
