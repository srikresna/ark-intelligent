# TASK-105: ECB Statistical Data Warehouse API Integration

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 21:00 WIB
**Siklus:** Data

## Deskripsi
Integrasikan ECB Statistical Data Warehouse (SDW) API untuk mendapatkan data monetary policy euro area: key interest rate (MRR), M3 money supply, bank lending rates, dan EUR exchange rates resmi ECB.

Data ini institutional-grade dan gratis tanpa API key. M3 money supply growth divergence antara ECB dan Fed adalah signal kuat untuk EUR/USD.

## Konteks
- ECB SDW REST API fully free, no key needed
- Endpoint base: `https://data-api.ecb.europa.eu/service/data/{flowRef}?lastNObservations=N&format=csvdata`
- Verified flow references:
  - EUR/USD rate: `EXR/M.USD.EUR.SP00.A`
  - ECB key rate (MRR): `FM/B.U2.EUR.4F.KR.MRR_FR.LEV`
  - M3 money supply: `BSI/M.U2.Y.V.M30.X.I.U2.2300.Z01.E` (atau similar)
- Monthly update cadence
- Ref: `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat ECB client di `internal/service/macro/ecb_client.go` (atau di service/fred/ jika lebih cocok)
- [ ] Fetch ECB key rate (MRR) — current value + historical series
- [ ] Fetch M3 money supply growth rate — month-over-month dan year-over-year
- [ ] Parse CSV response dari ECB API
- [ ] Cache results di BadgerDB (monthly data, TTL 24h)
- [ ] Error handling: graceful degradation jika ECB API down
- [ ] Expose via existing `/macro` command atau command baru `/ecb`

## File yang Kemungkinan Diubah
- `internal/service/macro/ecb_client.go` (baru)
- `internal/adapter/telegram/handler.go` atau `handler_macro.go` (command routing)
- `internal/adapter/telegram/formatter.go` (format output)

## Referensi
- `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`
- ECB SDW docs: https://data.ecb.europa.eu/help/api/overview
