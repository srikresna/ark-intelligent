# TASK-094-D: Convert Handler to HandlerDeps struct

**Status:** assigned to Dev-A  
**Priority:** HIGH  
**Effort:** S (Small — estimasi 1 jam)  
**Source:** ADR — Dependency Injection Framework Evaluation (TECH-012)  
**Ref:** `.agents/research/2026-04-01-adr-di-framework.md`
**Paperclip:** [PHI-105](/PHI/issues/PHI-105)

---

## Problem

Handler constructor currently takes **17 positional parameters** — difficult to maintain and error-prone.

Current signature (from handler.go):
```go
func NewHandler(
    bot *Bot,
    eventRepo ports.EventRepository,
    cotRepo ports.COTRepository,
    prefsRepo ports.PrefsRepository,
    // ... 13 more params
) *Handler
```

---

## Solution

Convert to struct-based dependency injection (Option D from ADR):

```go
type HandlerDeps struct {
    Bot            *Bot
    EventRepo      ports.EventRepository
    COTRepo        ports.COTRepository
    PrefsRepo      ports.PrefsRepository
    // ... all 17 deps as struct fields
}

func NewHandler(deps HandlerDeps) *Handler { ... }
```

---

## Acceptance Criteria

- [ ] Create `HandlerDeps` struct with all 17 dependencies as fields
- [ ] Refactor `NewHandler()` to accept `HandlerDeps` instead of 17 positional params
- [ ] Update all call sites in `cmd/bot/main.go`
- [ ] `go build ./...` clean
- [ ] `go vet ./...` zero warnings
- [ ] No behavior changes — pure refactor

---

## Files to Modify

1. `internal/adapter/telegram/handler.go` — New HandlerDeps struct + NewHandler signature
2. `cmd/bot/main.go` — Update Handler initialization call

---

## Implementation Notes

1. Start by identifying all 17 dependencies in current NewHandler
2. Create HandlerDeps struct with descriptive field names
3. Update NewHandler to accept HandlerDeps parameter
4. Update call site in main.go to build HandlerDeps struct
5. Run `go build ./...` and `go vet ./...` to verify

---

## Next Steps After Completion

This is step 1 of TECH-012 implementation. After this, continue with:
- TASK-094-C1: Extract wire_storage.go from main.go
- TASK-094-C2: Extract wire_services.go from main.go
- TASK-094-C3: Extract wire_telegram.go + wire_schedulers.go

See ADR for full roadmap.
