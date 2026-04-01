# TASK-177: Related "Next Steps" Command Suggestions After Execution

**Priority:** medium
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram/

## Deskripsi

Setelah command selesai, tampilkan inline keyboard row dengan 2-3 related commands. Membantu user discover features dan build analysis workflow.

## Command Relationships

```
/cot EUR    → [📈 /bias EUR] [📊 /rank] [🔬 /alpha EUR]
/bias EUR   → [📉 /cot EUR] [🎯 /cta EUR] [🌐 /macro]
/cta EUR    → [📊 /quant EUR] [🔑 /levels EUR] [🎯 /playbook EUR]
/macro      → [📅 /calendar] [🔄 /transition] [📊 /sentiment]
/quant EUR  → [📈 /cta EUR] [📊 /backtest EUR] [🎲 /scenario EUR]
/calendar   → [💥 /impact] [📊 /macro] [📈 /price]
/gex        → [📊 /vix] [🔬 /alpha] [📈 /cryptoalpha]
```

## File Changes

- `internal/adapter/telegram/keyboard.go` — Add `RelatedCommandsRow(command, currency string) [][]InlineKeyboardButton`
- `internal/adapter/telegram/handler.go` — After each command, append related commands keyboard
- `internal/adapter/telegram/handler_cta.go` — Same pattern

## Acceptance Criteria

- [ ] Related commands shown as inline buttons after command output
- [ ] Buttons include currency context (e.g., "/bias EUR" not just "/bias")
- [ ] At least 10 commands have related suggestions defined
- [ ] Buttons execute command on click (callback → cmd:bias:EUR)
- [ ] Does not show if output already has navigation keyboard (no double keyboard)
