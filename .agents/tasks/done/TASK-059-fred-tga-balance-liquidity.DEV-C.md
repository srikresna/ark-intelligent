# TASK-059: TGA Balance via FRED (WDTGAL) — Liquidity Dashboard

**Status:** ✅ DONE
**Completed by:** Dev-C
**Completed at:** 2026-04-01 20:00 WIB
**Branch:** feat/TASK-059-fred-tga-balance

## Changes:
1. **fetcher.go**: Added `TGABalance`, `TGABalanceTrend`, `LiquidityRegime` fields to `MacroData`; added `WDTGAL` to fetch jobs (8 weeks history); trend parsing with $50B threshold; sanitize; `classifyLiquidity()` function for EASY/NEUTRAL/TIGHT classification using TGA + RRP + Fed BS trinity.
2. **persistence.go**: Added `addObs("WDTGAL", data.TGABalance)` for BadgerDB persistence.
3. **regime.go**: Added `TGALabel` and `LiquidityLabel` fields to `MacroRegime`; TGA status display with direction; net liquidity regime classification with risk score adjustments.
4. **formatter.go**: Added TGA Balance and Net Liquidity display in `/macro` command output.
5. **unified_outlook.go**: Added TGA Balance and Net Liquidity to AI context prompt.
