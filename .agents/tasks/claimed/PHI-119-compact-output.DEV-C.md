# PHI-119: TASK-004 Compact Output Mode Default + Expand Button

**Status:** in_progress  
**Assigned to:** Dev-C  
**Priority:** medium  
**Type:** feature  
**Estimated:** M  
**Area:** internal/adapter/telegram/handler/  
**Created at:** 2026-04-03 WIB  
**Siklus:** UX/UI  

## Deskripsi

Implement compact output mode as default with "📖 Detail Lengkap" expand button for long outputs.

## Background

Per .agents/research/2026-04-01-01-ux-onboarding-navigation.md, some outputs (/cot, /macro) are very long (>4000 chars). Telegram cuts messages or users scroll extensively on mobile.

## Scope

### 1. Default to "compact" view for commands:
- `/cot` — show summary + key numbers only
- `/macro` — show key indicators only

### 2. Add expand button `📖 Detail Lengkap` to show full output

### 3. Store user preference (compact/full) in BadgerDB:
- Key: `user:{chatID}:output_mode`
- Values: `compact`, `full`

### 4. Add `/settings output_mode` command to change preference

## Acceptance Criteria

- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] /cot shows compact view by default (<1000 chars)
- [ ] /macro shows compact view by default
- [ ] Expand button shows full details
- [ ] User preference persisted in BadgerDB

## Referensi

- Research: .agents/research/2026-04-01-01-ux-onboarding-navigation.md
- Files: handler_cot.go, handler_macro.go, formatter.go
- Paperclip: [PHI-119](/PHI/issues/PHI-119)

## Progress

- [ ] Add compact formatter functions
- [ ] Add expand button callback handler
- [ ] Add BadgerDB persistence for preference
- [ ] Update /cot handler
- [ ] Update /macro handler
- [ ] Add /settings output_mode command
- [ ] Test both modes
- [ ] Create PR
