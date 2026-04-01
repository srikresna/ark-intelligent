# TASK-114: Trading Session Analysis Engine

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/ta
**Created by:** Research Agent
**Created at:** 2026-04-01 22:00 WIB
**Siklus:** Fitur

## Deskripsi
Buat session analysis engine yang classify London/NY/Tokyo trading session behavior per pair. Output: "London session trends 65% of the time for EUR/USD" → suggest breakout strategy. "NY session ranges 70%" → suggest mean reversion. Include current session context dan countdown ke session berikutnya.

## Konteks
- Volume Profile sudah mention session splits di docs tapi belum implement
- Intraday data tersedia via TwelveData (15m → 12h)
- ADX, range, volatility calculations semua sudah ada di TA engine
- Gap: tidak ada session classification atau session-specific strategy
- Institutional traders selalu trade sesuai session character
- Ref: `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `internal/service/ta/session_analyzer.go`
- [ ] Define sessions: Tokyo 00:00-09:00 UTC, London 08:00-17:00 UTC, NY 13:00-22:00 UTC (overlap periods noted)
- [ ] Tag intraday bars (15m/1h) dengan session
- [ ] Per session: compute ADX average, range (pips), volatility, % time trending vs ranging
- [ ] Classify: TRENDING (ADX>25 >50% time), RANGING (ADX<20 >50% time), VOLATILE, CALM
- [ ] Store last 20 sessions per pair untuk rolling statistics
- [ ] Telegram command: `/session [CURRENCY]`
- [ ] Output: current session + countdown, session behavior stats (4-week rolling), recommended strategy per session
- [ ] Cache results (TTL 1h)

## File yang Kemungkinan Diubah
- `internal/service/ta/session_analyzer.go` (baru)
- `internal/service/ta/types.go` (new session types)
- `internal/adapter/telegram/handler.go` (routing /session)
- `internal/adapter/telegram/formatter.go` (format session output)
- `internal/adapter/telegram/keyboard.go` (currency + session selector)

## Referensi
- `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`
- `internal/service/ta/indicators.go` (ADX)
- `internal/service/price/` (intraday data fetching)
