# TASK-053 Fix: /compare + cbHistoryNav Build Fix

**Completed by:** Dev-B
**Date:** 2026-04-01
**PR:** #100

## Summary
Fixed build break on agents/main caused by handler registrations referencing
unimplemented methods (cmdCompare, cbHistoryNav). Created handler_cot_compare.go
with both implementations.

## Changes
-  (new file, 187 LOC)
  - cmdCompare: /compare EUR GBP side-by-side COT positioning
  - cbHistoryNav: hist:<currency>:<weeks> inline keyboard navigation
  - Helper functions: cotBiasLabel, cotFormatChg, historyNavBtn
