# TASK-155: BIS Statistics API Integration — Central Bank Rates & Global Liquidity

**Priority:** high
**Type:** data
**Estimated:** L
**Area:** internal/service/bis/ (new)

## Deskripsi

Integrasi BIS SDMX REST API (`https://stats.bis.org/api/v2/`) untuk fetch data macro institutional-grade yang tidak tersedia di FRED.

## Dataset Target

1. `WS_CBPOL` — Central bank policy rates (Fed, ECB, BOJ, BOE, SNB, RBA, RBNZ, BOC)
2. `WS_CREDIT_GAP` — Credit-to-GDP gaps (krisis early warning)
3. `WS_GLI` — Global liquidity indicators (USD credit to non-US entities)
4. `WS_DSR` — Debt service ratios by country
5. `WS_EER` — Real effective exchange rates

## Detail Teknis

- API gratis, no key, SDMX format
- Request: `GET /data/dataflow/BIS/{dataset}/1.0?lastNObservations=N`
- Header: `Accept: application/vnd.sdmx.data+csv` (CSV paling mudah di-parse)
- Response: CSV with header row, timestamp + value columns
- Rate limit: tidak documented, pakai 1 req/sec safety

## File Changes

- `internal/service/bis/client.go` — NEW: BIS SDMX HTTP client
- `internal/service/bis/parser.go` — NEW: CSV/SDMX response parser
- `internal/service/bis/models.go` — NEW: Domain types (PolicyRate, CreditGap, GlobalLiquidity)
- `internal/domain/bis.go` — NEW: BIS domain types
- `internal/adapter/telegram/handler.go` — Add /bis command routing
- `internal/adapter/telegram/formatter_bis.go` — NEW: BIS data formatting

## Acceptance Criteria

- [ ] Fetch central bank policy rates untuk 8 major currencies
- [ ] Fetch credit-to-GDP gap untuk US, UK, JP, EU, AU, CA, NZ, CH
- [ ] Fetch global liquidity USD credit indicators
- [ ] Cache di BadgerDB dengan TTL 24h (data update monthly/quarterly)
- [ ] /bis command menampilkan ringkasan central bank rates + credit gaps
- [ ] Error handling untuk SDMX parsing failures
- [ ] Unit tests untuk parser
