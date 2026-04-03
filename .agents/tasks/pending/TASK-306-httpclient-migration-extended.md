# TASK-306: Extended httpclient.New() Migration — Remaining Bare http.Client{} Usages

**Priority:** medium
**Type:** refactor
**Estimated:** M
**Area:** internal/service/*
**Created by:** Dev-A
**Created at:** 2026-04-03 WIB
**Siklus:** Refactor

## Deskripsi

TASK-118 migrated `fred`, `worldbank`, `sentiment`, and several other services to use
`pkg/httpclient.New()`. However, ~20 additional service packages still instantiate bare
`&http.Client{Timeout: ...}` without the shared Transport configured in TASK-118.

Under concurrent load, each bare client allocates its own TCP connection pool — no reuse
across requests to the same host. This is the resource exhaustion risk identified in
`.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`.

## Services yang Perlu Dimigrasi

- `internal/service/sec/client.go` — package-level `httpClient`
- `internal/service/imf/weo.go` — package-level `httpClient`
- `internal/service/treasury/client.go` — package-level `httpClient`
- `internal/service/bis/reer.go` — package-level `httpClient`
- `internal/service/cot/fetcher.go` — struct field `httpClient`
- `internal/service/vix/fetcher.go` — local var in `FetchTermStructure`
- `internal/service/vix/move.go` — local var in `FetchMOVE`
- `internal/service/vix/vol_suite.go` — local var in `FetchVolSuite`
- `internal/service/price/eia.go` — struct field in `NewEIAClient`
- `internal/service/news/fed_rss.go` — local var in `fetchFedRSS`
- `internal/service/fed/fedwatch.go` — local var in `FetchFedWatch`
- `internal/service/marketdata/massive/client.go` — struct field in `NewClient`
- `internal/service/macro/treasury_client.go` — struct field in `NewTreasuryClient`
- `internal/service/macro/snb_client.go` — struct field in `NewSNBClient`
- `internal/service/macro/oecd_client.go` — struct field in `NewOECDClient`
- `internal/service/macro/ecb_client.go` — struct field in `NewECBClient`
- `internal/service/macro/dtcc_client.go` — struct field in `NewDTCCClient`
- `internal/service/macro/eurostat_client.go` — struct field in `NewEurostatClient`

## Acceptance Criteria

- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Semua service di atas menggunakan `httpclient.New(httpclient.WithTimeout(d))` atau `httpclient.NewClient(d)`
- [ ] Tidak ada behavior change — semua requests tetap identik
- [ ] `net/http` import dihapus dari file yang tidak lagi menggunakannya langsung

## Referensi

- `pkg/httpclient/transport.go` — factory yang perlu digunakan
- TASK-118 — original migration (completed)
- `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`
