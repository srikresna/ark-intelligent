# TASK-176: Default Timeframe User Preference

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/domain/, internal/adapter/telegram/

## Deskripsi

Tambah `DefaultTimeframe` ke UserPrefs. Saat user ketik `/cta EUR` tanpa timeframe, gunakan preference bukan hardcoded "daily". Configurable via /settings.

## File Changes

- `internal/domain/prefs.go` — Add `DefaultTimeframe string` field, default "daily"
- `internal/adapter/telegram/handler_cta.go` — Use prefs.DefaultTimeframe when no timeframe arg
- `internal/adapter/telegram/handler_quant.go` — Same pattern
- `internal/adapter/telegram/keyboard.go` — Add timeframe selector to settings menu: [Daily] [4H] [1H] [Weekly]
- `internal/adapter/telegram/handler.go` — Handle settings callback for timeframe change

## Acceptance Criteria

- [ ] DefaultTimeframe field in UserPrefs with default "daily"
- [ ] /cta EUR (no timeframe) → use prefs.DefaultTimeframe
- [ ] /quant EUR (no timeframe) → use prefs.DefaultTimeframe
- [ ] /settings shows timeframe selector
- [ ] Timeframe change persisted di BadgerDB
- [ ] Hint: "Using daily (default). Change in /settings"
