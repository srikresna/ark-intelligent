# TASK-107: Treasury.gov TIPS Yields & Breakeven Inflation

**Priority:** high
**Type:** data
**Estimated:** S
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 21:00 WIB
**Siklus:** Data

## Deskripsi
Integrasikan Treasury.gov direct CSV untuk TIPS real yields dan hitung breakeven inflation expectations. Breakeven = Nominal yield - TIPS real yield. Ini core institutional signal: rising breakevens = hawkish = USD bullish.

Sumber ini independent dari FRED, menyediakan redundancy dan direct Treasury source authority.

## Konteks
- Treasury.gov CSV fully free, no key needed
- TIPS real yields: `https://home.treasury.gov/resource-center/data-chart-center/interest-rates/daily-treasury-rates.csv/{YEAR}/all?type=daily_treasury_real_yield_curve&field_tdr_date_value={YEAR}&page&_format=csv`
- Nominal yields: sama URL dengan `type=daily_treasury_yield_curve`
- Tenors: 5Y, 7Y, 10Y, 20Y, 30Y
- Daily update cadence
- Ref: `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat Treasury client di `internal/service/macro/treasury_client.go`
- [ ] Fetch daily TIPS real yields (5Y, 7Y, 10Y, 20Y, 30Y)
- [ ] Fetch daily nominal yields (same tenors)
- [ ] Compute breakeven inflation: nominal - real per tenor
- [ ] Parse CSV response (Date, tenors as columns)
- [ ] Cache di BadgerDB (TTL 12h, daily data)
- [ ] Expose via `/macro` command — show breakeven inflation alongside existing FRED yields
- [ ] Format: tampilkan tabel tenors dengan real yield, nominal yield, dan breakeven

## File yang Kemungkinan Diubah
- `internal/service/macro/treasury_client.go` (baru)
- `internal/adapter/telegram/formatter.go` (format breakeven table)
- `internal/service/fred/` (optional: integrate with existing yield display)

## Referensi
- `.agents/research/2026-04-01-21-data-integrasi-ecb-snb-tips-oecd-dtcc.md`
- Treasury Direct: https://home.treasury.gov/resource-center/data-chart-center/interest-rates
