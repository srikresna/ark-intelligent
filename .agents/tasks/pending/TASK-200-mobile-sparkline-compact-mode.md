# TASK-200: Mobile-Friendly COT Sparkline Compact Mode

**Priority:** high
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram/

## Deskripsi

Add mobile compact mode yang collapse COT position tables ke sparkline + one-liner summary. Fixed-width tables unreadable on 320px mobile screens.

## Current Output (Desktop)

```
Date       | Net Pos   | Chg      | L/S
───────────┼───────────┼──────────┼────
Apr 01     | +15,234   | +2,100   | 1.45
Mar 25     | +13,134   | -500     | 1.38
```

## Mobile Output

```
EUR Net Position: ▅▇█▆▅▃▂ (7w trend)
+15,234 (↑2,100 WoW) | L/S 1.45 | 📊 75th pctl
```

## File Changes

- `internal/domain/prefs.go` — Add `MobileMode bool` field
- `internal/adapter/telegram/formatter.go` — Add compact table rendering path
- `internal/adapter/telegram/formatter_compact.go` — Extend FormatCOTOverviewCompact with sparkline mode
- `internal/adapter/telegram/keyboard.go` — Add mobile mode toggle to settings

## Acceptance Criteria

- [ ] MobileMode preference toggleable in /settings
- [ ] COT tables collapse to sparkline + one-liner in mobile mode
- [ ] Sparkline uses existing sparkLine() function
- [ ] Other tables (macro, calendar) also have compact rendering
- [ ] Full table still available via "Expand" button
- [ ] Default: mobile mode OFF for existing users
