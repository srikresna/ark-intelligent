# TASK-132: Deribit Expanded Assets — SOL, AVAX, XRP Options

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/gex
**Created by:** Research Agent
**Created at:** 2026-04-02 02:00 WIB
**Siklus:** Data

## Deskripsi
Expand GEX engine dari BTC/ETH only ke SOL, AVAX, XRP, TRX via Deribit USDC-settled options. Deribit punya 2,296 USDC options yang cover altcoins ini — same public API, different currency parameter.

## Konteks
- Current GEX: `handler_gex.go:44-47` — hanya support BTC dan ETH
- Deribit USDC-settled: `get_instruments?currency=USDC` → SOL, AVAX, XRP, TRX
- Same API, same parsing — hanya tambah currency options
- Ref: `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Tambah SOL, AVAX, XRP ke supported symbols di GEX handler
- [ ] Handle USDC-settled option naming convention (mungkin berbeda dari BTC/ETH)
- [ ] Update keyboard di GEX untuk include new symbols
- [ ] Test: `/gex SOL` harus return valid GEX analysis
- [ ] Graceful: jika USDC options tidak cukup liquid, show warning "Low liquidity, data may be unreliable"

## File yang Kemungkinan Diubah
- `internal/service/gex/engine.go` (tambah currency support)
- `internal/service/gex/deribit_client.go` (USDC currency handling)
- `internal/adapter/telegram/handler_gex.go` (expand symbol list)
- `internal/adapter/telegram/keyboard.go` (update GEX keyboard)

## Referensi
- `.agents/research/2026-04-02-02-data-deribit-expanded-tradingeconomics-finviz.md`
