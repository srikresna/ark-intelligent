# TASK-203: Multi-Word Command Aliases for Power Users

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram/

## Deskripsi

Add shortcuts for top 5 command+arg combinations. Current aliases only cover single commands (/c→/cot). Power users type `/cot EUR` 20+ times/day.

## New Aliases

| Alias | Expands To | Use Case |
|-------|-----------|----------|
| `/ce <cur>` | `/cot <cur>` | COT by currency (most common) |
| `/ca <cur>` | `/cta <cur>` | CTA by currency |
| `/qe <cur>` | `/quant <cur>` | Quant by currency |
| `/bta` | `/backtest all` | Backtest all signals |
| `/of` | `/outlook fred` | FRED macro outlook |

## File Changes

- `internal/adapter/telegram/handler.go` — Register 5 new aliases in command routing

## Acceptance Criteria

- [ ] 5 aliases registered and working
- [ ] /help shortcuts shows all aliases including new ones
- [ ] Aliases work with additional args (e.g., `/ca EUR 4h` → `/cta EUR 4h`)
- [ ] No conflict with existing commands
