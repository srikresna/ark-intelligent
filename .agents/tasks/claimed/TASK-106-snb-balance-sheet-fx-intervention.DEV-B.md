# TASK-106: SNB Balance Sheet / FX Intervention Proxy

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 21:00 WIB
**Siklus:** Data

## Deskripsi
Integrasikan SNB (Swiss National Bank) Data API untuk memantau balance sheet, khususnya "Foreign currency investments" sebagai proxy untuk FX intervention. Perubahan signifikan di FX reserves (~$850B+) langsung menggerakkan CHF.

## Konteks
- SNB Data API fully free, no key needed
- Data endpoint: `https://data.snb.ch/api/cube/{cubeId}/data/csv/en`
- Dimensions: `https://data.snb.ch/api/cube/snbbipo/dimensions/en`
- Cube `snbbipo`: Full balance sheet — gold, foreign currency investments, repo, sight deposits, banknotes
- Foreign currency investments (dimension D) = direct FX intervention indicator
- Monthly update cadence
- Ref: `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat SNB client di `internal/service/macro/snb_client.go`
- [ ] Fetch balance sheet data dari cube `snbbipo`
- [ ] Extract key metrics: foreign currency investments, sight deposits, gold holdings
- [ ] Hitung month-over-month change di FX reserves (besar change = likely intervention)
- [ ] Parse CSV response
- [ ] Cache di BadgerDB (TTL 24h, monthly data)
- [ ] Alert jika FX reserve change > threshold (e.g., >5B CHF/month)
- [ ] Expose via `/macro` command untuk CHF context

## File yang Kemungkinan Diubah
- `internal/service/macro/snb_client.go` (baru)
- `internal/adapter/telegram/handler.go` (routing)
- `internal/adapter/telegram/formatter.go` (format output)

## Referensi
- `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`
- SNB data portal: https://data.snb.ch
