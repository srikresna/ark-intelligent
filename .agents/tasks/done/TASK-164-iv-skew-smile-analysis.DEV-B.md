# TASK-164: IV Skew / Smile Analysis + Skew Flip Detection

**Status:** done
**Agent:** DEV-B
**PR:** #251
**Branch:** feat/TASK-164-iv-skew-smile-analysis
**Completed:** 2026-04-02T08:15+08:00

## Summary

Built IV skew analysis layer on top of existing Deribit IV surface data:

- **skew.go**: 5-point moneyness smile curve, put/call IV ratio with historical percentile, skew slope via linear regression, skew flip detection, ATM IV term structure slope
- **skew_test.go**: 15 unit tests (all passing)
- **/skew command**: New Telegram command with full inline keyboard and cross-navigation to /gex and /ivol
- **FormatSkewResult**: Telegram HTML formatter with smile table, alerts section
- **Keyboard updates**: Added skew button to GEX and IV Surface keyboards, added skew to keyboard.go navigation map

## Files Changed

- internal/service/gex/skew.go (NEW)
- internal/service/gex/skew_test.go (NEW)
- internal/adapter/telegram/formatter_gex.go (FormatSkewResult added)
- internal/adapter/telegram/handler_gex.go (/skew command, skewKeyboard, handleSkewCallback, WithGEX registration)
- internal/adapter/telegram/keyboard.go (skew entry in related-commands map)
