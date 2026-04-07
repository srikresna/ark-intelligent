# TASK-212: Options Flow Analysis via Deribit (PCR, Large Trades, IV Term Structure)

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/optflow/ (NEW), internal/service/marketdata/deribit/

## Deskripsi

Extend Deribit integration beyond GEX. Tambahkan Options Flow analysis: Put/Call Ratio, large trade scanner (whale flow), dan IV term structure. Command baru: `/optflow BTC`

GEX sudah ada dan fokus pada gamma levels. OptionsFlow fokus pada directional intent dari option buyers/sellers.

## Components

### 1. Put/Call Ratio (PCR)
- Dari Deribit instruments + order book data
- PCR < 0.7 = bullish sentiment (more calls)
- PCR > 1.3 = bearish (more puts)
- PCR OI (open interest) dan PCR Volume terpisah

### 2. Large Trade Scanner
- Endpoint: `/public/get_last_trades_by_currency?currency=BTC&count=100`
- Filter: trades > $500k premium value
- Classify: Call buy = bullish flow, Put buy = bearish flow, Call sell = neutral/hedge
- Show last 10 large trades dengan direction

### 3. IV Term Structure
- Fetch implied volatility per expiry (weekly → monthly → quarterly)
- Normal: IV rises with time (contango)
- Inverted: near-term IV > far-term → event/fear driven
- Display IV curve text chart

### 4. Options Sentiment Score
```go
score = PCR_weight * PCR_score + LargeFlow_weight * flow_score + IVSlope_weight * iv_score
// Range -100 (extreme bearish) to +100 (extreme bullish)
```

## File Changes

- `internal/service/marketdata/deribit/client.go` — Add GetLastTrades(), GetVolatilityIndex() methods
- `internal/service/marketdata/deribit/types.go` — Add Trade, VolIndex types
- `internal/service/optflow/engine.go` — NEW: OptionsFlowEngine
- `internal/service/optflow/pcr.go` — NEW: PCR calculator
- `internal/service/optflow/scanner.go` — NEW: Large trade scanner
- `internal/service/optflow/iv_term.go` — NEW: IV term structure
- `internal/service/optflow/types.go` — NEW: OptionsFlowResult
- `internal/adapter/telegram/handler_optflow.go` — NEW: /optflow handler
- `internal/adapter/telegram/formatter_optflow.go` — NEW: formatter
- `internal/adapter/telegram/bot.go` — Wire handler

## Acceptance Criteria

- [ ] PCR (OI + Volume) fetched from Deribit for BTC and ETH
- [ ] Large trades (> $500k premium) scanned and classified
- [ ] IV term structure: at least 4 expiries shown
- [ ] Composite options sentiment score (-100 to +100)
- [ ] /optflow BTC and /optflow ETH working
- [ ] Results cached 15 min (same pattern as GEX engine)
- [ ] go build ./... clean
