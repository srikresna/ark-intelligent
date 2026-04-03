# TASK-306: Extended httpclient.New() Migration

**Status:** ✅ COMPLETED — Merged to main  
**Assigned:** Dev-A  
**Commit:** 1144f17  
**Type:** refactor
**Estimated:** M
**Area:** internal/service/*, pkg/httpclient
**Siklus:** Refactor

## Description

Migrated remaining bare `http.Client{}` instantiations across 18 service packages to use the shared `httpclient.New()` factory with connection pooling. This completes the httpclient standardization started in TASK-118.

## Services Migrated

- `internal/service/sec/client.go`
- `internal/service/imf/weo.go`
- `internal/service/treasury/client.go`
- `internal/service/bis/reer.go`
- `internal/service/cot/fetcher.go`
- `internal/service/vix/fetcher.go`
- `internal/service/vix/move.go`
- `internal/service/vix/vol_suite.go`
- `internal/service/price/eia.go`
- `internal/service/news/fed_rss.go`
- `internal/service/fed/fedwatch.go`
- `internal/service/marketdata/massive/client.go`
- `internal/service/macro/treasury_client.go`
- `internal/service/macro/snb_client.go`
- `internal/service/macro/oecd_client.go`
- `internal/service/macro/ecb_client.go`
- `internal/service/macro/dtcc_client.go`
- `internal/service/macro/eurostat_client.go`

## Acceptance Criteria

- [x] All 18 services use `httpclient.New()` factory
- [x] Shared transport with connection pooling (MaxIdleConns=100)
- [x] `go build ./...` succeeds
- [x] `go vet ./...` succeeds
- [x] No behavior change — all requests identical
- [x] Merged to main (1144f17)

## Related

- TASK-118: Original httpclient migration (completed)
- PR that merged this work to main
