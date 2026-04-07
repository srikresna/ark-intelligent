# TASK-159: MOVE Index (Bond Volatility) via Yahoo Finance

**Priority:** medium
**Type:** data
**Estimated:** S
**Area:** internal/service/price/

## Deskripsi

Integrasi ICE BofA MOVE Index — bond market VIX equivalent. CBOE tidak publish MOVE (403), tapi Yahoo Finance ticker `^MOVE` available gratis. Codebase sudah punya Yahoo fetcher yang bisa di-reuse.

## Detail Teknis

- Ticker: `^MOVE` on Yahoo Finance
- Existing fetcher: `internal/service/price/fetcher.go` sudah punya `fetchYahooFinance()`
- Reuse existing infrastructure, tambah MOVE sebagai instrument
- Cross-asset: VIX/MOVE ratio = equity vs bond volatility divergence

## File Changes

- `internal/domain/price.go` — Add MOVE instrument mapping (Yahoo symbol `^MOVE`)
- `internal/service/price/fetcher.go` — Add MOVE to fetch list
- `internal/service/vix/analysis.go` — Add VIX/MOVE ratio computation + divergence detection
- `internal/adapter/telegram/formatter.go` — Add MOVE + VIX/MOVE ratio to volatility section

## Acceptance Criteria

- [ ] Fetch MOVE index daily close via Yahoo Finance
- [ ] Compute VIX/MOVE ratio (normal range 0.15-0.30)
- [ ] Detect divergence: high VIX + low MOVE = equity-specific fear; low VIX + high MOVE = bond stress (FX carry risk)
- [ ] Display MOVE level + VIX/MOVE ratio di /vix output
- [ ] Cache with existing price cache TTL
- [ ] Fallback gracefully jika Yahoo ticker unavailable
