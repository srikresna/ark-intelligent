# ARK Intelligence — Changelog

## v3.0.0 — Production Hardening
- Structured JSON logging (zerolog) across entire codebase
- Circuit breaker for external APIs (CFTC, MQL5)
- Health check endpoint (/health, /ready)
- Telegram retry with exponential backoff + retry_after
- Per-user rate limiting (10 cmd/60s)
- AI cost protection: RPM limit + daily cap (AI_MAX_RPM, AI_MAX_DAILY)
- Per-user AI cooldown (30s between /outlook)
- Bot owner exempt from rate limits
- GitHub Actions CI (build + test on push)
- Removed legacy monolithic main.go

## v2.0.0 — Economic Calendar & Notifications
- MQL5 Economic Calendar integration (no API key needed)
- Day/week/month view with inline navigation
- Real-time actual release values with direction indicators
- Pre-event reminders + actual release alerts
- AI flash analysis on releases (if Opus active)
- Daily morning summary (06:00 WIB)
- Weekly auto-sync (Minggu 23:00 WIB)
- Per-user filter & alert preferences
- Auto-chunking for long messages

## v1.0.0 — Initial Release
- COT positioning analysis (CFTC Socrata + CSV)
- Williams COT Index, sentiment scoring, divergence detection
- AI narrative generation (Opus)
- Currency strength ranking (/rank)
- FRED macro regime dashboard (/macro)
- COT signal detection (/signals)
- Cross-market correlation analysis
- BadgerDB persistence
