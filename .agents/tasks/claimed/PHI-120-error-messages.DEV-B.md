# PHI-120: TASK-005 User-Friendly Error Messages Layer

**Status:** in_progress  
**Assigned to:** Dev-B  
**Priority:** high  
**Type:** feature  
**Estimated:** S  
**Area:** internal/adapter/telegram/  
**Created at:** 2026-04-03 WIB  
**Siklus:** UX/UI  

## Deskripsi

Implement user-friendly error message layer to replace technical error exposure.

## Background

Per .agents/research/2026-04-01-01-ux-onboarding-navigation.md, error messages currently expose internal technical details to users:
- "context deadline exceeded" → users don't need to know about contexts
- "badger: key not found" → users don't need to know about BadgerDB
- Technical errors cause confusion

## Scope

### 1. Create `internal/adapter/telegram/errors.go` with:
- User-friendly error message mapping
- Error categorization (network, data, auth, system)
- Actionable suggestions for users

### 2. Map error types to user-friendly messages:
| Error Type | User Message |
|------------|--------------|
| `context.DeadlineExceeded` | "⏱️ Request took too long. Please try again." |
| `badger.ErrKeyNotFound` | "📊 Data not found. Try a different symbol or timeframe." |
| Network errors | "🌐 Connection issue. Check your internet and retry." |
| AI service errors | "🤖 AI service temporarily unavailable. Please try again shortly." |
| Generic errors | "⚠️ Something went wrong. Please try again or contact support." |

### 3. Update handler error handlers to use new error layer

## Acceptance Criteria

- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Technical errors mapped to user-friendly messages
- [ ] All user-facing errors are actionable
- [ ] Internal errors logged separately for debugging

## Referensi

- Research: .agents/research/2026-04-01-01-ux-onboarding-navigation.md
- Current error handling: handler.go error handling sections
- Paperclip: [PHI-120](/PHI/issues/PHI-120)

## Progress

- [ ] Create errors.go with error mapping
- [ ] Define error categories and messages
- [ ] Create user-facing error formatter
- [ ] Update handler error handling
- [ ] Add internal error logging
- [ ] Test error scenarios
- [ ] Create PR
