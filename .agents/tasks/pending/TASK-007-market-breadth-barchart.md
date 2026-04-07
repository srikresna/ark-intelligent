# TASK-007: Add Market Breadth Data via Barchart (Firecrawl)

**Priority:** MEDIUM
**Type:** New Data Source
**Estimated effort:** M (1 day)
**Ref:** research/2026-04-06-09-data-sources-siklus2.md

---

## Context

The /sentiment command currently shows: VIX, AAII, CNN F&G, Crypto F&G, myfxbook,
OpenInsider, CBOE P/C ratio — but has NO equity market breadth indicators.

Market breadth is critical for distinguishing:
- "Healthy bull" (broad participation, >60% stocks above 50MA) vs
- "Narrow rally" (only large caps moving, breadth diverging → topping risk)

Barchart market-pulse page provides this data publicly, and Firecrawl is already
configured and paid for.

---

## Implementation

### New file: `internal/service/sentiment/breadth.go`

Follow pattern of `service/sentiment/cboe.go` (Firecrawl JSON extraction).

```go
type MarketBreadthData struct {
    // S&P 500 breadth metrics
    PctAbove50MA    float64 // % S&P 500 stocks above 50-day MA
    PctAbove200MA   float64 // % S&P 500 stocks above 200-day MA
    NewHighs        int     // daily new 52-week highs
    NewLows         int     // daily new 52-week lows
    AdvDecRatio     float64 // Advance/Decline ratio (>1 = more advancing)
    BreadthScore    string  // "STRONG", "HEALTHY", "NARROWING", "WEAK", "BEARISH"
    Available       bool
    FetchedAt       time.Time
}

func FetchMarketBreadth(ctx context.Context) *MarketBreadthData
```

URL: `https://www.barchart.com/stocks/market-pulse`

Firecrawl JSON schema:
```json
{
  "pct_above_50ma": "number",
  "pct_above_200ma": "number",
  "new_highs": "integer",
  "new_lows": "integer",
  "advance_decline_ratio": "number"
}
```

BreadthScore logic:
- PctAbove50MA > 70% → "STRONG"
- PctAbove50MA 55-70% → "HEALTHY"
- PctAbove50MA 40-55% → "NARROWING"
- PctAbove50MA 25-40% → "WEAK"
- PctAbove50MA < 25% → "BEARISH"

Cache TTL: 1 hour (daily data, but refresh frequently)
Circuit breaker: follow pattern in SentimentFetcher

### Integration into SentimentData

In `internal/service/sentiment/sentiment.go`, add breadth to `SentimentData` struct
and call `FetchMarketBreadth` in `Fetch()`.

### Integration into /sentiment output

In `internal/adapter/telegram/handler_sentiment_cmd.go`, add section:

```
📊 Market Breadth (S&P 500)
Above 50MA: 64% | Above 200MA: 71%
New Highs/Lows: 87 / 12 → H/L Ratio: 7.3x
Breadth: HEALTHY 🟢
```

---

## Acceptance Criteria

- [ ] `internal/service/sentiment/breadth.go` with `FetchMarketBreadth()`
- [ ] Breadth section appears in `/sentiment` output when available
- [ ] `Available=false` graceful degradation when FIRECRAWL_API_KEY absent
- [ ] Cache TTL = 1h, circuit breaker after 3 failures (5 min reset)
- [ ] BreadthScore enum: STRONG / HEALTHY / NARROWING / WEAK / BEARISH
- [ ] `go build ./...` passes
