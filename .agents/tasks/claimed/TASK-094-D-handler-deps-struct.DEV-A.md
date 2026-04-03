# TASK-094-D: Convert Handler to HandlerDeps struct — 🔄 IN PROGRESS

**Status:** 🔄 ASSIGNED → Dev-A (PREP WORK)  
**Priority:** HIGH  
**Effort:** S (Small — estimasi 1 jam)  
**Source:** ADR — Dependency Injection Framework Evaluation (TECH-012)  
**Ref:** `.agents/research/2026-04-01-adr-di-framework.md`  
**Paperclip:** [PHI-115](/PHI/issues/PHI-115)  
**Blocked by:** C3 PR merge to agents/main  
**Parent:** TASK-094 (DI Restructuring)

---

## Summary

Convert Handler constructor from 17 positional parameters to struct-based dependency injection per ADR TECH-012. **WAITING for C3 PR merge to agents/main before implementation.**

---

## Changes To Make

### Files To Modify
- `internal/adapter/telegram/handler.go`
  - Add `HandlerDeps` struct with all 17 dependencies as fields
  - Refactor `NewHandler(deps HandlerDeps) *Handler` signature
  - Update all field assignments to use `deps.FieldName`

- `cmd/bot/main.go`
  - Update NewHandler call to use `HandlerDeps{...}` struct literal
  - All 17 dependencies now passed as named fields

---

## Acceptance Criteria

- [ ] Create `HandlerDeps` struct with all 17 dependencies as fields
- [ ] Refactor `NewHandler()` to accept `HandlerDeps` instead of 17 positional params
- [ ] Update all call sites in `cmd/bot/main.go`
- [ ] `go build ./...` clean
- [ ] `go vet ./...` zero warnings
- [ ] No behavior changes — pure refactor

---

## HandlerDeps Struct

```go
type HandlerDeps struct {
    Bot            *Bot
    EventRepo      ports.EventRepository
    COTRepo        ports.COTRepository
    PrefsRepo      ports.PrefsRepository
    NewsRepo       ports.NewsRepository
    NewsFetcher    ports.NewsFetcher
    AIAnalyzer     ports.AIAnalyzer
    Changelog      string
    NewsScheduler  SurpriseProvider
    Middleware     *Middleware
    PriceRepo      ports.PriceRepository
    SignalRepo     ports.SignalRepository
    ChatService    *aisvc.ChatService
    ClaudeAnalyzer *aisvc.ClaudeAnalyzer
    ImpactProvider ImpactProvider
    DailyPriceRepo pricesvc.DailyPriceStore
    IntradayRepo   pricesvc.IntradayStore
}
```

---

## Next Steps (per TECH-012)

- TASK-094-C1: Extract wire_storage.go from main.go
- TASK-094-C2: Extract wire_services.go from main.go
- TASK-094-C3: Extract wire_telegram.go + wire_schedulers.go

See ADR for full roadmap.
