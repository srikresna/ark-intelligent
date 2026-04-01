# TASK-193: Formatter Code Consolidation into fmtutil

**Priority:** medium
**Type:** refactor
**Estimated:** M
**Area:** pkg/fmtutil/, internal/adapter/telegram/

## Deskripsi

Extract duplicated formatting patterns dari 5 formatter files into pkg/fmtutil/. Reduce 300+ LOC of duplication.

## Patterns to Consolidate

### 1. Header Builder
```go
// pkg/fmtutil/header.go
func AnalysisHeader(emoji, title, symbol, timeframe string) string {
    return fmt.Sprintf("%s <b>%s — %s %s</b>\n", emoji, title, symbol, timeframe)
}
```

### 2. Emoji/Icon Mapper
```go
// pkg/fmtutil/icons.go
func BiasIcon(bias string) string {
    switch bias {
    case "BULLISH": return "🟢"
    case "BEARISH": return "🔴"
    default: return "⚪"
    }
}
```

### 3. Remove gexCommaSep duplication
```go
// formatter_gex.go uses gexCommaSep() — replace with fmtutil.FmtNum()
```

### 4. Bar Chart Helper
```go
// pkg/fmtutil/charts.go
func ProgressBar(value, max float64, width int, fillChar, emptyChar string) string
```

## File Changes

- `pkg/fmtutil/header.go` — NEW: AnalysisHeader, SectionDivider
- `pkg/fmtutil/icons.go` — NEW: BiasIcon, DirectionArrow, RegimeEmoji
- `pkg/fmtutil/charts.go` — NEW: ProgressBar, BarChart
- `internal/adapter/telegram/formatter_ict.go` — Use fmtutil headers/icons
- `internal/adapter/telegram/formatter_wyckoff.go` — Use fmtutil headers
- `internal/adapter/telegram/formatter_gex.go` — Replace gexCommaSep with FmtNum

## Acceptance Criteria

- [ ] Header builder used by 5+ formatter files
- [ ] Icon mapper used by 3+ formatter files
- [ ] gexCommaSep() removed, FmtNum() used instead
- [ ] ProgressBar used for bar chart rendering
- [ ] No visual change in output (formatting identical)
- [ ] Formatter LOC reduced by 100+ lines
