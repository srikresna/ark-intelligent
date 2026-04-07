# TASK-192: Comprehensive Config Startup Validation

**Priority:** medium
**Type:** refactor
**Estimated:** M
**Area:** internal/config/

## Deskripsi

Expand config.go validate() dari 3 checks ke 15+. Log feature disablement. Prevent invalid configurations from starting bot.

## Validations to Add

```go
func (c *Config) validate() error {
    // Existing (keep):
    // - COTHistoryWeeks >= 4
    // - COTFetchInterval >= 1m
    // - ConfluenceCalcInterval >= 1m

    // NEW:
    if c.PriceFetchInterval <= 0 { return ErrInvalidConfig("PriceFetchInterval must be > 0") }
    if c.IntradayRetentionDays < 1 { return ErrInvalidConfig("IntradayRetentionDays must be >= 1") }
    if c.ChatHistoryLimit <= 0 { c.ChatHistoryLimit = 50 } // default
    if c.AIMaxDaily < c.AIMaxRPM { log.Warn("AIMaxDaily < AIMaxRPM — may exhaust quota quickly") }

    // Feature availability logging
    if c.TwelveDataAPIKeys == "" { log.Warn("TwelveData keys missing — price fetcher degraded") }
    if c.FREDAPIKey == "" { log.Warn("FRED key missing — macro features disabled") }
    if c.FirecrawlKey == "" { log.Warn("Firecrawl key missing — scraping disabled") }

    // Storage check
    if _, err := os.Stat(c.DataDir); os.IsNotExist(err) {
        return ErrInvalidConfig("DataDir does not exist: " + c.DataDir)
    }
}
```

## File Changes

- `internal/config/config.go` — Expand validate() with 12+ new checks
- `internal/config/config.go` — Add feature availability logging at startup

## Acceptance Criteria

- [ ] 15+ validation checks (up from 3)
- [ ] Startup log shows enabled/disabled features based on config
- [ ] Invalid PriceFetchInterval prevents startup
- [ ] Missing API keys logged as warnings (not errors — graceful)
- [ ] Storage directory checked for existence
- [ ] Unit test for validate() with various invalid configs
