# ADR: Dependency Injection Framework Evaluation (TECH-012)

**Status:** Accepted  
**Date:** 2026-04-02  
**Author:** Agent Dev-C  
**Ref:** TASK-094, TECH-012 in TECH_REFACTOR_PLAN.md

---

## Context

`cmd/bot/main.go` (717 LOC) is the single entry point that manually wires all
dependencies: 15 storage repositories, 15+ service constructors, and a Handler
constructor that takes 17 positional parameters. The question is whether adopting
a DI framework (google/wire or uber-go/fx) would reduce this complexity or
whether a manual restructuring is sufficient.

### Current State (post-TASK-041 refactor)

| Metric | Value |
|--------|-------|
| `cmd/bot/main.go` LOC | 717 |
| `telegram/wiring.go` LOC | 62 |
| `telegram/handler.go` LOC | 445 |
| Storage repo constructors | 15 |
| Service constructors | ~15 |
| Handler constructor params | 17 positional |
| Existing DI dependencies in go.mod | **None** |
| Total wiring code (main.go sections 3-8) | ~350 LOC |

---

## Options Evaluated

### Option A: google/wire (compile-time code generation)

**How it works:** Define `wire.Build(...)` with provider functions → run
`wire` tool → generates `wire_gen.go` with concrete constructor calls.

**Pros:**
- Type-safe: wiring errors caught at compile time (no reflection)
- Zero runtime overhead — generated code is plain Go
- Explicit dependency graph visible in `wire_gen.go`
- Forces single-responsibility constructors (each provider = 1 type)

**Cons:**
- Requires code generation step in build pipeline (`go generate`)
- Every CI run + every developer must run `wire` before `go build`
- DSL learning curve: `wire.Bind()`, `wire.Struct()`, provider sets
- Still need ~15 provider functions (same number of constructors)
- Debugging generated code is harder than reading plain `main.go`
- All 3 dev agents must learn wire patterns simultaneously

**Fit with this project:**
Wire shines in large monorepos (50+ services, 100+ providers). With ~30
total constructors and a single binary, the overhead of code generation
outweighs the benefit. The current wiring is sequential and clear — wire
would shuffle it into a generated file that's harder to reason about.

**Verdict: NOT RECOMMENDED** — overhead > benefit at current scale.

---

### Option B: uber-go/fx (runtime DI with lifecycle management)

**How it works:** Register providers via `fx.Provide()` → framework
resolves dependency graph at runtime → manages lifecycle (OnStart/OnStop).

**Pros:**
- Lifecycle management (OnStart/OnStop) could simplify scheduler startup/shutdown
- Flexible: add new services without touching existing wiring
- Module system (`fx.Module`) for logical grouping
- Popular in Uber-scale services — well-maintained

**Cons:**
- **Runtime errors**: missing dependency → panic at startup, not compile time
- Reflection-based: harder to trace "who provides X?" in IDE
- Magic: dependencies are resolved implicitly — new developers can't read
  `main.go` linearly to understand startup order
- Adds ~5MB binary bloat (reflect, dig, fx packages)
- **Breaks the explicit startup order** that main.go currently has
  (e.g., "storage must init before services, services before handler")
- Lifecycle hooks duplicate what `context.WithCancel` + `defer` already do
- Every error message refers to fx internals, not business logic

**Fit with this project:**
fx's lifecycle management would help if we had 50+ goroutines with complex
start/stop ordering. We have 3 background schedulers and a polling loop —
all managed cleanly by a single `ctx` + `cancel()` + signal handler.
fx would add indirection without solving a real problem.

**Verdict: NOT RECOMMENDED** — runtime magic + binary bloat not justified.

---

### Option C: Refactor manual wiring (structured, no framework)

**How it works:** Keep manual DI but reorganize `main.go` into focused
helper functions and a wiring registry.

**Proposed structure:**
```go
// cmd/bot/main.go — stays as orchestrator (~200 LOC)
func main() {
    cfg := config.MustLoad()
    ctx, cancel := ...
    deps := wireAll(ctx, cfg)  // returns *AppDeps
    deps.Start(ctx)
    waitForShutdown(ctx, cancel, deps)
}

// cmd/bot/wire_storage.go
func wireStorage(cfg *config.Config) *StorageDeps { ... }

// cmd/bot/wire_services.go
func wireServices(cfg *config.Config, storage *StorageDeps) *ServiceDeps { ... }

// cmd/bot/wire_telegram.go
func wireTelegram(cfg *config.Config, svc *ServiceDeps) *TelegramDeps { ... }

// cmd/bot/wire_schedulers.go
func wireSchedulers(cfg *config.Config, svc *ServiceDeps, tg *TelegramDeps) *SchedulerDeps { ... }
```

**Pros:**
- Zero new dependencies — pure Go, no framework
- Transparent: anyone can read the code top-to-bottom
- Compile-time safe (same as current)
- Each wire_*.go file is <150 LOC and self-contained
- main.go drops from 717 → ~200 LOC
- Dependency structs make Handler constructor cleaner:
  `NewHandler(deps *HandlerDeps)` instead of 17 positional params
- Incremental: can be done file-by-file without breaking anything

**Cons:**
- Still "verbose" compared to framework magic (but verbose = explicit)
- Must manually update wire files when adding new services
- No automatic lifecycle management (but we don't need it)

**Verdict: RECOMMENDED** — best fit for project size and team.

---

### Option D: Constructor injection pattern (pure Go, minimal refactor)

**How it works:** Convert the Handler's 17-param constructor to accept a
`HandlerConfig` struct. Keep everything else as-is.

**Proposed change:**
```go
type HandlerDeps struct {
    Bot            *Bot
    EventRepo      ports.EventRepository
    COTRepo        ports.COTRepository
    PrefsRepo      ports.PrefsRepository
    // ... 13 more fields
}

func NewHandler(deps HandlerDeps) *Handler { ... }
```

**Pros:**
- Minimal change — only touches Handler constructor signature
- Solves the "17 positional params" code smell immediately
- No file reorganization needed
- Can be done in 30 minutes

**Cons:**
- Doesn't address main.go's 717-LOC length
- Doesn't help with scheduler/service wiring complexity
- Band-aid on the symptom, not the root cause

**Verdict: RECOMMENDED as immediate quick-win**, but Option C should
follow as a proper restructuring.

---

## Decision

**Option C (structured manual wiring) is the recommended approach,
with Option D as a quick-win first step.**

### Rationale

1. **Scale doesn't justify a framework.** With ~30 constructors and 1 binary,
   wire/fx add complexity without proportional benefit. The industry consensus
   (Rob Pike, Dave Cheney, Go team) is that manual DI is idiomatic Go for
   projects below ~100 providers.

2. **Explicit > Implicit.** The current main.go is readable top-to-bottom.
   Three dev agents work on this codebase concurrently — implicit resolution
   (fx) or generated code (wire) would increase merge conflicts and debugging
   time.

3. **The real problem is file organization, not DI.** main.go is long because
   it does storage + services + telegram + schedulers + bootstrap in one file.
   Splitting into wire_*.go files solves this without any new dependencies.

4. **No lifecycle management needed.** The current `context.WithCancel` +
   `defer db.Close()` + signal handler pattern is clean and sufficient.

### Implementation Plan

| Step | Task | Est. |
|------|------|------|
| 1 | TASK-094-D: Convert Handler to `HandlerDeps` struct | S (1h) |
| 2 | TASK-094-C1: Extract `wire_storage.go` from main.go | S (1h) |
| 3 | TASK-094-C2: Extract `wire_services.go` from main.go | S (1h) |
| 4 | TASK-094-C3: Extract `wire_telegram.go` + `wire_schedulers.go` | M (2h) |
| 5 | Clean up main.go to <200 LOC orchestrator | S (30min) |

Each step is independently mergeable and testable (`go build` + `go vet`).

---

## Consequences

- **No new dependencies** added to go.mod
- main.go becomes a thin orchestrator (~200 LOC)
- Handler constructor becomes self-documenting via struct fields
- New services/repos added by creating entries in the appropriate wire_*.go
- Dev agents can work on different wire files without conflict
- TECH-012 in TECH_REFACTOR_PLAN.md should be updated to reflect this decision

---

## References

- [Go Wiki: Dependency Injection](https://github.com/golang/go/wiki/CodeReviewComments)
- [google/wire](https://github.com/google/wire) — used by ~5% of Go projects
- [uber-go/fx](https://github.com/uber-go/fx) — used primarily at Uber scale
- TASK-041 (bot.go split) — already reduced bot.go from 1,289 → 515 LOC
- Current codebase: `cmd/bot/main.go` (717 LOC, 15 repos, 15 services, 17-param Handler)
