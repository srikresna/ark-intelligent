# TASK-037: /smc Telegram Command — ICT/SMC Dashboard

**Priority:** HIGH
**Cycle:** 3 (Fitur Baru)
**Estimated effort:** M (3-4 hours)
**Branch target:** agents/main
**Depends on:** TASK-035 (ict.go), TASK-036 (smc.go)

---

## Context

TASK-035 dan TASK-036 membangun ICT/SMC engine di layer TA service.
TASK-037 mengekspos hasilnya ke user melalui Telegram command `/smc`.
Ini adalah command baru yang standalone — berbeda dari `/cta` yang sudah ada.

---

## Objective

Tambah command `/smc [SYMBOL] [TF]` ke Telegram handler yang menampilkan:
- SMC market structure (BOS/CHOCH history)
- ICT Fair Value Gaps
- ICT Order Blocks (unmitigated)
- Liquidity levels (sweep targets)
- Premium/Discount zone status
- ICT Killzone untuk waktu sekarang

---

## Acceptance Criteria

### Command Format
```
/smc           → uses user's last symbol preference + 4H default
/smc EURUSD    → EURUSD 4H
/smc EURUSD 1h → EURUSD 1H
/smc XAUUSD 1d → Gold Daily
```

Supported timeframes: 15m, 1h, 4h, 1d (same as /cta)

### Output Format (HTML, Telegram)

```html
📐 <b>SMC/ICT — EURUSD 4H</b>
<code>2026-04-01 15:30 WIB | Killzone: 🇬🇧 LONDON</code>

<b>🏗 Market Structure</b>
Trend: <b>BULLISH</b> (HH + HL pattern)
Last BOS ↑: 1.08200 <i>(3 bars ago)</i>
Last CHOCH: 1.07800 BEARISH <i>(12 bars ago, overridden)</i>

<b>⚡ Fair Value Gaps</b>
• 🟢 Bullish FVG: 1.0835 – 1.0852 <i>(unfilled)</i>
• 🔴 Bearish FVG: 1.0878 – 1.0885 <i>(80% filled)</i>

<b>🔲 Order Blocks</b>
• 🟢 Bullish OB: 1.0818 – 1.0825 ✅ unmitigated [str: ★★★]
• 🔴 Bearish OB: 1.0892 – 1.0900 ⚠️ mitigated
• ⬛ Breaker: 1.0875 – 1.0882 (flipped bearish)

<b>💧 Liquidity</b>
• Buy-side: 1.09050 (equal highs × 4)
• Sell-side: 1.07980 (equal lows × 3) ← swept ✓

<b>📊 Zone: PREMIUM (68% of range)</b>
EQ: 1.08400 | Premium: &gt;1.08400 | Discount: &lt;1.08400
```

### Inline Keyboard
```
[🔄 Refresh] [📊 4H] [📊 1H] [📊 1D] [← Back to CTA]
```

### Error Handling
- Symbol not found → "Symbol tidak ditemukan. Coba: EURUSD, XAUUSD, GBPUSD"
- Insufficient data → "Data tidak cukup untuk analisis SMC (minimal 30 bar)"

---

## Implementation Plan

### File: `internal/adapter/telegram/handler_smc.go` (new)

```go
package telegram

// handler_smc.go — /smc command: ICT/SMC Smart Money dashboard

// SMCServices holds dependencies for the SMC command.
type SMCServices struct {
    TAEngine       *ta.Engine
    DailyPriceRepo pricesvc.DailyPriceStore
    IntradayRepo   pricesvc.IntradayStore
    PriceMapping   []domain.PriceSymbolMapping
}
```

### Register in handler.go
Add `/smc` to command routing alongside `/cta`.

### Formatting helpers
```go
func fmtFVG(f ta.FVG) string           // emoji + price range + fill status
func fmtOrderBlock(ob ta.OrderBlock) string  // type + zone + mitigated status
func fmtKillzone(kz string) string     // "LONDON" → "🇬🇧 LONDON"
func fmtZone(s *ta.SMCResult) string   // premium/discount with EQ level
```

### Data flow
```
/smc EURUSD 4h
  → resolve symbol via PriceMapping (same as /cta)
  → fetch intraday bars (4H) from IntradayStore
  → TAEngine.ComputeSnapshot(bars)
    → CalcICT(bars, atr)  [from ict.go]
    → CalcSMC(bars, atr)  [from smc.go]
  → format + send HTML message
  → attach inline keyboard for TF switching
```

---

## Handler Registration in main.go / handler.go

Add to handler struct:
```go
smcServices *SMCServices  // may be nil if not configured
```

Register command:
```go
h.bot.Handle("/smc", h.handleSMC)
```

---

## Definition of Done
- [ ] `/smc` command responds correctly for EURUSD, XAUUSD, GBPUSD on 4H
- [ ] TF switching via inline keyboard works
- [ ] Killzone shows correct session label based on current UTC time
- [ ] ICT/SMC data populated (requires TASK-035 + 036 done first)
- [ ] `go build ./...` passes
- [ ] No regression in existing commands
