# Research Report: UX/UI Siklus 1 Putaran 6
# Mobile Sparkline, Accessibility, Quiet Hours, Aliases, Onboarding Tracking
**Date:** 2026-04-02 16:00 WIB
**Siklus:** 1/5 (UX/UI) — Putaran 6
**Author:** Research Agent

## Ringkasan

5 genuinely new UX improvements focused on mobile experience, accessibility, notification management, power user shortcuts, dan onboarding completion tracking.

## Temuan 1: Mobile — COT Table Collapse to Sparkline

handler.go:1700-1730 generates fixed-width COT position tables (4+ columns with pipe separators). On mobile (320px width), tables wrap poorly and become unreadable. Sparkline already computed (line 1706) tapi underutilized — still shows full table below.

**Fix:** When user preference "mobile mode" enabled, show sparkline + one-liner summary instead of full table. Default to mobile for new users.

## Temuan 2: Accessibility — Emoji Without Context

58 uses of bare 🟢/🔴 without text meaning. Screen readers announce "GREEN CIRCLE" instead of "Bullish Signal". All sentiment, direction, and signal indicators use bare emoji.

**Fix:** Wrap emoji in meaningful text: `🟢 Bullish (actual > forecast)` not bare `🟢`.

## Temuan 3: Quiet Hours + Granular Alert Control

UserPrefs only has binary `AlertsEnabled` + impact filter. No quiet hours (22:00-08:00), no per-alert-type toggle (COT vs FRED vs news vs signals), no daily frequency cap. Users with all alerts enabled get 20+ messages/day.

**Fix:** Add QuietHoursStart/End, AlertTypePrefs map, MaxAlertsPerDay to UserPrefs. Check before each broadcast.

## Temuan 4: Multi-Word Command Aliases Missing

Existing aliases cover single commands (/c→/cot, /q→/quant) but not command+arg combos. `/cot EUR` (most common query) has no shortcut. Power users type this 20+ times/day.

**Fix:** Add /ce, /ca, /qe shortcuts for currency-specific commands.

## Temuan 5: Onboarding Completion — No Tracking

After /start → role selection → StarterKit, there's no progression tracking. No "did user run first command?", "did user customize settings?", "did user explore 2+ features?". Users click randomly with no guided flow.

**Fix:** Add OnboardingStep (0-4) tracking, dismissible progress indicator, contextual hints at each step.
