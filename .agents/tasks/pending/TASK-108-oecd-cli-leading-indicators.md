# TASK-108: OECD Composite Leading Indicators (CLI)

**Priority:** medium
**Type:** data
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 21:00 WIB
**Siklus:** Data

## Deskripsi
Integrasikan OECD SDMX API untuk Composite Leading Indicators (CLI). CLI adalah 6-9 bulan forward-looking macro indicator. Cross-country CLI divergence memprediksi FX trends: contoh US CLI naik vs EU CLI turun = bullish USD/bearish EUR.

## Konteks
- OECD SDMX REST API fully free, no key needed
- Endpoint: `https://sdmx.oecd.org/public/rest/data/OECD.SDD.STES,DSD_STES@DF_CLI/.M.LI...AA...H?startPeriod=2024-01&format=csvfilewithlabels`
- Data: CLI amplitude-adjusted, semua OECD countries, monthly
- Also available: Consumer Confidence (CSCICP02), Business Confidence, Industrial Production
- Cloudflare di old endpoint tapi `sdmx.oecd.org` bersih
- Ref: `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat OECD client di `internal/service/macro/oecd_client.go`
- [ ] Fetch CLI data untuk G7 countries minimal (US, EU, UK, JP, CA, AU, NZ — forex-relevant)
- [ ] Parse CSV/SDMX response
- [ ] Hitung CLI divergence antar negara (e.g., US vs EU spread)
- [ ] Cache di BadgerDB (TTL 24h, monthly data)
- [ ] Expose: bisa lewat `/macro` command atau command baru `/leading`
- [ ] Format: ranking countries by CLI momentum, highlight divergences

## File yang Kemungkinan Diubah
- `internal/service/macro/oecd_client.go` (baru)
- `internal/adapter/telegram/handler.go` (routing)
- `internal/adapter/telegram/formatter.go` (format CLI table)

## Referensi
- `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`
- OECD SDMX docs: https://sdmx.oecd.org
