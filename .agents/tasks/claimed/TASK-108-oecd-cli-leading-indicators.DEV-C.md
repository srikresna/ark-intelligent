# TASK-108: OECD CLI Leading Indicators — DEV-C

**Agent:** Dev-C
**Started:** 2026-04-02 06:46 WIB
**Status:** PR ready

## Implementation

- Created `internal/service/macro/oecd_client.go`:
  - OECDClient with cache (24h TTL) and mutex
  - Fetches CLI data from OECD SDMX REST API (free, no key)
  - Parses CSV response for G7+ forex-relevant countries
  - Computes month-over-month momentum
  - FX divergence detection (US vs DE, JP, GB, CA, AU)
  - Telegram HTML formatter with color-coded zones

- Added `/leading` command in `handler_macro_cmd.go`
- Registered command in `handler.go`
- Added to help menu in `handler_onboarding.go`

## Build Status
- `go build ./...` ✅
- `go vet ./...` ✅
