# TASK-156: Bybit Funding Rate Historical Tracking — DEV-C Claim

**Agent:** Dev-C
**Claimed:** 2026-04-02 03:33 WIB
**Branch:** feat/TASK-156-bybit-funding-rate-history
**Status:** implementing

## Changes
- Added `FundingRate` type and `GetFundingHistory()` method to Bybit client
- Added `FundingRateStats` type and `ComputeFundingStats()` for statistical analysis
- Integrated funding history into microstructure engine `Analyze()`
- Enhanced `/cryptoalpha` formatter with 7d/30d averages, min/max, regime, percentile
- Enhanced Indonesian interpretation with funding regime + percentile context
