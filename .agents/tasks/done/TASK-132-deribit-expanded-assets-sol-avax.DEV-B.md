# TASK-132: Deribit Expanded Assets — SOL, AVAX, XRP Options (DONE)

**Completed by:** Dev-B
**Date:** 2026-04-02
**Branch:** feat/TASK-132-deribit-expanded-assets
**PR:** pending

## Changes

1. **internal/service/gex/engine.go** — Added `assetConfig` struct and `supportedAssets` map to route BTC/ETH via their own currency and SOL/AVAX/XRP via `currency=USDC` with instrument prefix filtering. Contract size now read from instrument metadata (10 for USDC altcoins vs 1 for BTC/ETH). Index price fallback uses config-driven index names (e.g. `sol_usdc`).

2. **internal/service/gex/types.go** — Added `LowLiquidity bool` field to `GEXResult` for thin markets (<20 strikes with data).

3. **internal/service/marketdata/deribit/client.go** — Added `GetIndexPriceByName()` method for explicit index name lookup (needed for USDC-settled altcoins where index name pattern differs from `{currency}_usd`).

4. **internal/adapter/telegram/handler_gex.go** — Expanded `validGEXSymbols` to include SOL, AVAX, XRP. Updated keyboard with all 5 symbols.

5. **internal/adapter/telegram/formatter_gex.go** — Added low-liquidity warning banner when `LowLiquidity` flag is set.

## Verification

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./internal/service/gex/...` ✅
- `go test ./internal/service/marketdata/deribit/...` ✅
