# TASK-001-EXT: Interactive Onboarding with Role Selector

**Status:** 📋 ASSIGNED → Dev-B  
**Priority:** HIGH  
**Effort:** M (Medium — estimasi 4-6 jam)  
**Source:** UX Research Siklus 1 Extension  
**Ref:** `.agents/research/2026-04-01-01-ux-onboarding-navigation.md`  
**Paperclip:** [PHI-122](/PHI/issues/PHI-122) (to be created)  
**Depends on:** PHI-116 (basic onboarding flow) — ✅ COMPLETED  

---

## Summary

Extend the existing onboarding flow (PHI-116) with interactive role selector and personalized starter kits. This builds on the foundation laid in TASK-001 to create a guided, role-based onboarding experience that reduces first-session churn.

---

## Background

Current `/start` shows 28+ commands at once — overwhelming for new users. Research shows users churn when they don't know which commands are relevant to their needs.

TASK-001 (PHI-116) created the foundation:
- Settings/preferences persistence in BadgerDB
- Welcome message with basic structure
- User preference tracking

TASK-001-EXT adds the interactive layer:
- Role selector (Trader Pemula / Intermediate / Pro)
- Personalized "starter kit" keyboard per role
- Brief 3-step interactive tutorial

---

## Acceptance Criteria

- [ ] `/start` detects first-time users and triggers onboarding flow
- [ ] Step 1: Display role selector keyboard:
  - 🌱 **Trader Pemula** — "Saya baru memulai trading"
  - 📊 **Trader Intermediate** — "Saya sudah punya pengalaman"
  - 🎯 **Trader Pro** — "Saya butuh data institusional"
- [ ] Step 2: After role selection, show personalized starter kit (3-4 relevant commands):
  - **Pemula:** /help, /start, /settings, /changelog
  - **Intermediate:** /cot, /macro, /signal, /help
  - **Pro:** /cot, /macro, /quant, /impact, /accuracy, /backtest
- [ ] Step 3: Brief 3-step interactive tutorial with navigation buttons (Next ← → Done)
- [ ] Tutorial content per role explains the starter kit commands
- [ ] Save role preference to user settings in BadgerDB
- [ ] Subsequent `/start` shows "Welcome back" with quick-access keyboard based on role
- [ ] Add `/onboarding` command to restart tutorial voluntarily
- [ ] All strings in Indonesian with consistent tone

---

## Files to Modify

### Primary
- `internal/adapter/telegram/handler_onboarding.go` — Extend with role selector flow (exists from PHI-116)
- `internal/adapter/telegram/onboarding.go` — Add role detection and tutorial steps (exists from PHI-116)
- `internal/adapter/telegram/onboarding_test.go` — Add role selector tests (exists from PHI-116)

### Supporting
- `internal/adapter/telegram/handler.go` — Add `/onboarding` command handler
- `internal/adapter/telegram/keyboard.go` — Add starter kit keyboard builders per role
- `internal/domain/user_prefs.go` — Add `Role` field to UserPreferences (if not exists)
- `internal/adapter/storage/prefs_repo.go` — Ensure role persistence

---

## Data Model

```go
// internal/domain/user_prefs.go
type UserRole string

const (
    RoleBeginner      UserRole = "beginner"
    RoleIntermediate  UserRole = "intermediate"
    RolePro           UserRole = "pro"
)

type UserPreferences struct {
    UserID        int64     `json:"user_id"`
    PreferredModel string   `json:"preferred_model"`
    OutputMode    string    `json:"output_mode"` // compact | full
    Role          UserRole  `json:"role"`        // NEW FIELD
    // ... existing fields
}
```

---

## Flow Diagram

```
User types /start
    ↓
Is first time? (check UserPreferences exists)
    ↓ YES                          ↓ NO
Show role selector           Show "Welcome back" + quick actions
    ↓
User selects role
    ↓
Save role to UserPreferences
    ↓
Show starter kit keyboard for selected role
    ↓
Offer 3-step tutorial? (Yes/Skip)
    ↓ YES                          ↓ NO
Tutorial Step 1 of 3           End onboarding
    ↓
User clicks Next → Step 2
    ↓
User clicks Next → Step 3
    ↓
User clicks Done → End onboarding
```

---

## Role Configuration

```go
// internal/adapter/telegram/onboarding.go

type RoleConfig struct {
    Name         string
    Description  string
    StarterKit   []string // command names
    TutorialSteps []TutorialStep
}

var RoleConfigs = map[domain.UserRole]RoleConfig{
    domain.RoleBeginner: {
        Name:        "🌱 Trader Pemula",
        Description: "Saya baru memulai trading dan ingin belajar dasar-dasarnya",
        StarterKit:  []string{"/help", "/start", "/settings", "/changelog"},
        TutorialSteps: []TutorialStep{
            {Title: "Langkah 1: Perintah Dasar", Content: "/help menampilkan semua perintah yang tersedia..."},
            {Title: "Langkah 2: Pengaturan", Content: "/settings mengatur preferensi model AI..."},
            {Title: "Langkah 3: Update", Content: "/changelog melihat update terbaru..."},
        },
    },
    domain.RoleIntermediate: {
        Name:        "📊 Trader Intermediate",
        Description: "Saya sudah punya pengalaman dan ingin analisis teknikal",
        StarterKit:  []string{"/cot", "/macro", "/signal", "/help"},
        TutorialSteps: []TutorialStep{
            {Title: "Langkah 1: COT Analysis", Content: "/cot menampilkan Commitment of Traders..."},
            {Title: "Langkah 2: Macro Intel", Content: "/macro menampilkan kalender ekonomi..."},
            {Title: "Langkah 3: Trading Signals", Content: "/signal melihat sinyal trading..."},
        },
    },
    domain.RolePro: {
        Name:        "🎯 Trader Pro",
        Description: "Saya butuh data institusional dan analisis kuantitatif",
        StarterKit:  []string{"/cot", "/macro", "/quant", "/impact", "/accuracy", "/backtest"},
        TutorialSteps: []TutorialStep{
            {Title: "Langkah 1: Quant Analysis", Content: "/quant menghasilkan laporan kuantitatif..."},
            {Title: "Langkah 2: Event Impact", Content: "/impact melihat dampak event ekonomi..."},
            {Title: "Langkah 3: Backtest", Content: "/backtest menguji strategi trading..."},
        },
    },
}
```

---

## Keyboard Layouts

### Role Selector Keyboard
```
┌─────────────────────────────────────┐
│ 🌱 Trader Pemula                    │
│ Saya baru memulai trading            │
├─────────────────────────────────────┤
│ 📊 Trader Intermediate              │
│ Saya sudah punya pengalaman          │
├─────────────────────────────────────┤
│ 🎯 Trader Pro                       │
│ Saya butuh data institusional        │
└─────────────────────────────────────┘
```

### Starter Kit: Pemula
```
┌─────────────┬─────────────┐
│   ❓ Help   │   ⚙️ Settings│
├─────────────┼─────────────┤
│  🔄 Restart │   📋 Changelog│
└─────────────┴─────────────┘
```

### Starter Kit: Pro
```
┌─────────────┬─────────────┬─────────────┐
│    📊 COT   │    🌍 Macro │    🔢 Quant │
├─────────────┼─────────────┼─────────────┤
│   ⚡ Impact  │   🎯 Signal │   📈 Backtest│
└─────────────┴─────────────┴─────────────┘
```

---

## Test Requirements

- [ ] Unit tests for role detection (first-time vs returning user)
- [ ] Unit tests for role persistence (BadgerDB round-trip)
- [ ] Unit tests for starter kit keyboard generation per role
- [ ] Unit tests for tutorial step navigation (Next/Back/Done)
- [ ] Integration test: Full onboarding flow simulation
- [ ] All tests use Indonesian language strings

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] Unit tests written with >80% coverage for new code
- [ ] `go test ./...` passes
- [ ] `go build ./...` clean
- [ ] Manual test: Verify onboarding flow in Telegram bot
- [ ] PR submitted with clear description
- [ ] Code review approved

---

## Notes

- **Builds on:** PHI-116 (TASK-001) — reuse existing settings infrastructure
- **Related to:** TASK-002 (button standardization) — use consistent button patterns
- **Next iteration:** TASK-006 (help command search/filter) could use role preference

---

*Assigned to: Dev-B*  
*Assigned by: TechLead-Intel (loop #28)*  
*Date: 2026-04-03*
