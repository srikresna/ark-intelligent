# TASK-109: DTCC FX Swap Data Repository Integration

**Priority:** medium
**Type:** data
**Estimated:** L
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 21:00 WIB
**Siklus:** Data

## Deskripsi
Integrasikan DTCC Public Price Dissemination data untuk FX derivatives. Data ini berisi actual post-trade swap records — notional amounts, currencies, maturities. Volume patterns di FX forwards/swaps signal hedging flows dan institutional positioning shifts.

Ini institutional-grade data yang sangat sedikit retail trader akses.

## Konteks
- DTCC PPD free, web-based + REST API
- Dashboard: `https://pddata.dtcc.com/ppd/`
- API pattern: `https://pddata.dtcc.com/ppd/api/report/cumulative/CFTC/FOREX?asof=YYYY-MM-DD`
- Data: FX asset class under CFTC reporting — individual trade records
- Mungkin perlu pagination untuk dataset besar
- Ref: `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat DTCC client di `internal/service/macro/dtcc_client.go`
- [ ] Fetch cumulative FX swap data (CFTC/FOREX asset class)
- [ ] Parse JSON response — extract per-currency notional volumes
- [ ] Aggregate: total notional per currency pair, daily changes
- [ ] Detect anomalies: unusual volume spikes (>2 std dev dari historical)
- [ ] Cache di BadgerDB (TTL 12h)
- [ ] Expose via command baru `/swaps` atau integrate ke existing `/cot` untuk institutional context
- [ ] Handle pagination jika API returns paged results

## File yang Kemungkinan Diubah
- `internal/service/macro/dtcc_client.go` (baru)
- `internal/adapter/telegram/handler.go` (routing)
- `internal/adapter/telegram/formatter.go` (format output)

## Referensi
- `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`
- DTCC PPD: https://pddata.dtcc.com/ppd/
