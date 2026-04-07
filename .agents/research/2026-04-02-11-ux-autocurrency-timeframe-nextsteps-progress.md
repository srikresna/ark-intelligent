# Research Report: UX/UI Siklus 1 Putaran 5
# Auto-Currency, Default Timeframe, Next Steps, Progress, Page Indicators
**Date:** 2026-04-02 11:00 WIB
**Siklus:** 1/5 (UX/UI) — Putaran 5
**Author:** Research Agent

## Ringkasan

Analisis 17 UX opportunities baru. 5 terbaik dijadikan task — fokus quick wins dan high-impact improvements. Tema utama: reduce friction, improve discoverability, better feedback loops.

## Temuan 1: Auto-Reload Last Currency

`LastCurrency` sudah ada di `UserPrefs` (prefs.go:72) tapi TIDAK digunakan. Saat user ketik `/cot` tanpa argument, bot minta pilih currency. Padahal bisa langsung load currency terakhir.

**Impact:** Menghilangkan 1 klik untuk setiap command. Untuk user aktif yang analisis 5-10 pair/hari, ini saving signifikan.
**Effort:** 3 handler changes (cmdCOT, cmdCTA, cmdQuant) + 1 prefs update after each command.

## Temuan 2: Default Timeframe Preference

Saat user ketik `/cta EUR` tanpa timeframe, default ke "daily" (handler_cta.go:138). Pro traders prefer 4H; beginners prefer daily. Tidak ada cara set default.

**Impact:** Pro traders save 1 argument per command. Beginners get appropriate default.
**Effort:** Add `DefaultTimeframe` ke UserPrefs, use in CTA/quant handlers.

## Temuan 3: Related "Next Steps" Command Suggestions

Setelah command selesai, tidak ada guidance ke command terkait. User harus tahu sendiri bahwa setelah `/cot EUR`, `/bias EUR` memberikan directional context, atau `/cta EUR` memberikan technical levels.

**Impact:** Feature discoverability naik. Users explore more commands naturally.
**Effort:** Add `RelatedCommandsRow()` ke keyboard.go, call after command completion.

## Temuan 4: Multi-Step Progress for Long Commands

Python subprocess commands (quant, cta, vp) bisa 3-10 detik. Loading message statis "Computing..." tanpa progress update. User thinks bot frozen.

**Impact:** Perceived performance improvement. Users don't re-send commands.
**Effort:** Add goroutine ticker yang edit loading message every 3 seconds.

## Temuan 5: Page X/Y Indicator + Error Retry Buttons

Chunked messages (>4096 chars) tanpa page indicator. User tidak tahu ada continuation. Error messages tanpa retry button — user harus retype command manually.

**Impact:** Better orientation in multi-page outputs. Lower friction on errors.
**Effort:** Add page footer to chunks, add keyboard to error messages.
