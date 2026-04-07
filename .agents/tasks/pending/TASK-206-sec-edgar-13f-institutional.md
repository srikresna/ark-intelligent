# TASK-206: SEC EDGAR 13F Institutional Holdings Tracker

**Priority:** medium
**Type:** data
**Estimated:** L
**Area:** internal/service/sec/ (new)

## Deskripsi

Integrasi SEC EDGAR API untuk track institutional 13F filings. Monitor what Berkshire, Bridgewater, Renaissance, Citadel buying/selling. Quarterly signal.

## Endpoints

- Company search: `https://www.sec.gov/files/company_tickers.json`
- Filings: `https://data.sec.gov/submissions/CIK{padded_cik}.json`
- 13F holdings XML: via accession number path
- Auth: User-Agent header only. Rate: 10 req/sec.

## Target Institutions

| CIK | Name | AUM |
|-----|------|-----|
| 0001067983 | Berkshire Hathaway | $300B+ |
| 0001350694 | Bridgewater Associates | $150B+ |
| 0001037389 | Renaissance Technologies | $130B+ |
| 0001423053 | Citadel Advisors | $60B+ |
| 0001061768 | Soros Fund Management | $25B+ |

## File Changes

- `internal/service/sec/client.go` — NEW: SEC EDGAR HTTP client
- `internal/service/sec/parser.go` — NEW: 13F XML parser
- `internal/service/sec/models.go` — NEW: Institution, Holding, PortfolioChange types
- `internal/service/sec/analyzer.go` — NEW: Quarter-over-quarter change detection
- `internal/adapter/telegram/handler.go` — Add /13f command routing
- `internal/adapter/telegram/formatter.go` — Add institutional holdings formatting

## Acceptance Criteria

- [ ] Fetch latest 13F for 5 target institutions
- [ ] Parse holdings: issuer, value, shares, put/call
- [ ] Quarter-over-quarter change detection (new positions, exits, increases, decreases)
- [ ] /13f command shows top institutional moves
- [ ] Alert on significant moves (>$1B new position)
- [ ] Cache with 7-day TTL (quarterly data)
- [ ] User-Agent header properly set
