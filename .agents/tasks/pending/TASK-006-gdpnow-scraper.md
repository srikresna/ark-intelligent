# TASK-006: Add Atlanta Fed GDPNow Real-Time GDP Tracker

**Priority:** HIGH
**Type:** New Data Source
**Estimated effort:** S (half day)
**Ref:** research/2026-04-06-09-data-sources-siklus2.md

---

## Context

Atlanta Fed GDPNow provides real-time US GDP estimates updated every 2-3 days
after new economic data releases. Unlike FRED GDP (quarterly lag), GDPNow gives
the "live" market-consensus proxy for Fed policy expectations and USD direction.

Currently the /macro and /outlook commands lack any real-time GDP tracking.
This is a gap for forex traders who need to gauge USD rate expectations intraday.

---

## Implementation

### New file: `internal/service/fred/gdpnow.go`

Follow the same pattern as `service/fred/speeches.go`:
- Firecrawl JSON extraction from `https://www.atlantafed.org/cqer/research/gdpnow`
- Cache TTL: 6 hours (data updates every 2-3 days max)
- Graceful degradation: return `Available=false` if FIRECRAWL_API_KEY not set

```go
type GDPNowData struct {
    CurrentEstimate float64   // annualized Q GDP growth rate (e.g. 2.1)
    PrevEstimate    float64   // previous estimate
    Quarter         string    // e.g. "Q2 2026"
    LastUpdated     time.Time // when Atlanta Fed last updated
    Available       bool
    FetchedAt       time.Time
}

func FetchGDPNow(ctx context.Context) *GDPNowData
```

Firecrawl prompt: "Extract the current GDPNow model estimate (percent annualized),
the previous estimate, and the quarter being tracked from this page."

Schema:
```json
{
  "current_estimate": "number",
  "prev_estimate": "number",
  "quarter": "string",
  "last_updated": "string"
}
```

### Integration into /macro output

In `internal/adapter/telegram/handler_macro_cmd.go` — add GDPNow section:

```
📊 GDPNow (Atlanta Fed)
Q2 2026 estimate: +2.1% ann. (prev: +1.8%) ↑
Updated: 2026-04-04
```

### Integration into AI context builder

In `internal/service/ai/context_builder.go` — add GDPNow to the macro context
blob passed to Claude/Gemini for /outlook generation.

---

## Acceptance Criteria

- [ ] `internal/service/fred/gdpnow.go` exists with `FetchGDPNow()` and `GetGDPNowCachedOrFetch()`
- [ ] Returns `Available=false` gracefully when FIRECRAWL_API_KEY is not set
- [ ] Cache TTL = 6h, uses sync.RWMutex pattern consistent with other fetchers
- [ ] GDPNow section visible in `/macro` output when available
- [ ] `go build ./...` passes with no errors
- [ ] Unit test for graceful-degradation path (no API key)
