# TASK-130: Deribit IV Surface + Skew + Term Structure

**Priority:** high
**Type:** feature
**Estimated:** L
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-02 02:00 WIB
**Siklus:** Data

## Deskripsi
Expand Deribit integration dari basic GEX (TASK-012) ke full implied volatility analytics: IV surface construction, volatility skew per expiry, dan term structure. Semua data available dari existing Deribit public API — hanya perlu tambah endpoints.

## Konteks
- GEX engine sudah pakai Deribit public API di `internal/service/gex/`
- Endpoint baru: `public/get_book_summary_by_currency` → return mark_iv, OI, volume untuk SEMUA options (880+ BTC) dalam satu call
- IV Surface: group by expiry (Y-axis) × strike (X-axis) → heatmap
- Skew: per expiry, plot IV vs moneyness → detect put skew (fear) vs call skew (greed)
- Term Structure: DVOL atau ATM IV per expiry → forward-looking vol expectations
- Ref: `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Fetch bulk options data via `get_book_summary_by_currency` (BTC, ETH)
- [ ] Build IV surface matrix: Strike × Expiry → mark_iv
- [ ] Compute skew metrics per expiry: 25-delta put IV - 25-delta call IV (risk reversal)
- [ ] Compute term structure: ATM IV per expiry date
- [ ] Cache results (TTL 30 min — options data changes frequently)
- [ ] Telegram output: `/ivol BTC` atau extend `/gex BTC` dengan section IV Surface
- [ ] Formatter: ASCII heatmap untuk IV surface, chart/table untuk skew & term structure

## File yang Kemungkinan Diubah
- `internal/service/gex/deribit_client.go` (tambah bulk endpoint)
- `internal/service/gex/iv_surface.go` (baru)
- `internal/adapter/telegram/handler_gex.go` (extend command)
- `internal/adapter/telegram/formatter_gex.go` (IV surface formatter)

## Referensi
- `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`
- Deribit API: https://docs.deribit.com/#public-get_book_summary_by_currency
