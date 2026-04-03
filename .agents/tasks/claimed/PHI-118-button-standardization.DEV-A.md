# PHI-118: TASK-002 Standardize Button Labels and Add Home Button

**Status:** ✅ COMPLETED — PR submitted, awaiting QA review  
**Assigned to:** Dev-A  
**Priority:** medium  
**Type:** refactor  
**Estimated:** S  
**Area:** internal/adapter/telegram/  
**Created at:** 2026-04-03 WIB  
**Siklus:** UX/UI  
**Branch:** `feat/TASK-002-button-standardization`  
**Commit:** 9b010c3  

## Deskripsi

Standardize all button labels and add universal home button for consistent navigation.

## Background

Per .agents/research/2026-04-01-01-ux-onboarding-navigation.md, button labels are inconsistent:
- `<< Kembali ke Ringkasan`
- `<< Back to Overview`
- `<< Kembali ke Dashboard`
- `↩ Back`

No universal "home" button exists. Users in deep drill-down must `/start` again.

## Scope

### 1. Standardize all button labels:
- `🏠 Menu Utama` — universal home button
- `◀ Kembali` — consistent back button

### 2. Add home button to all multi-step keyboards:
- COT drill-down keyboards
- Macro analysis keyboards
- Settings menus
- Alert configuration

### 3. Create `keyboard.go` constants for all button labels (DRY principle)

## Acceptance Criteria

- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] All back buttons use `◀ Kembali`
- [ ] All keyboards have `🏠 Menu Utama` home button
- [ ] Button labels defined as constants (not hardcoded strings)

## Referensi

- Research: .agents/research/2026-04-01-01-ux-onboarding-navigation.md
- Files: internal/adapter/telegram/handler/*.go, keyboard.go
- Paperclip: [PHI-118](/PHI/issues/PHI-118)

## Progress

- [x] Define button label constants in keyboard.go (already existed: btnBack, btnHome, btnBackGrid)
- [x] Update keyboard.go — use btnBack + " Settings"
- [x] Update keyboard_settings.go — use btnBack + " Settings"
- [x] Update keyboard_feedback.go — use btnHome constant
- [x] Update keyboard_misc.go — use btnBackGrid constant
- [x] Verify all multi-step keyboards have home button (already implemented)
- [x] Create PR — branch: `feat/TASK-002-button-standardization`
