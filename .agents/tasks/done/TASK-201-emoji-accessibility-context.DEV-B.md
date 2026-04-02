# TASK-201: Accessibility — Emoji Paired with Meaningful Text

**Priority:** medium
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram/

## Deskripsi

Replace 58+ bare emoji sentiment indicators (🟢/🔴/⚪) with emoji + text pairs. Screen readers announce "GREEN CIRCLE" which conveys no trading information.

## Current

```go
return "🟢"  // screen reader: "GREEN CIRCLE"
```

## Fix

```go
return "🟢 Bullish"  // screen reader: "GREEN CIRCLE Bullish"
```

## Pattern

Create helper:
```go
// pkg/fmtutil/accessibility.go
func SentimentLabel(positive bool, context string) string {
    if positive {
        return fmt.Sprintf("🟢 %s", context)
    }
    return fmt.Sprintf("🔴 %s", context)
}
```

## File Changes

- `pkg/fmtutil/accessibility.go` — NEW: SentimentLabel, DirectionLabel, SignalLabel helpers
- `internal/adapter/telegram/formatter.go` — Replace bare emoji with labeled versions (58+ locations)
- `internal/adapter/telegram/formatter_ict.go` — Same
- `internal/adapter/telegram/formatter_gex.go` — Same

## Acceptance Criteria

- [ ] All 🟢/🔴/⚪ paired with text meaning
- [ ] Helper functions in fmtutil for consistent labeling
- [ ] Screen reader users understand signal direction
- [ ] Visual output still readable (emoji + short text)
- [ ] No output length increase >15% (keep concise labels)
