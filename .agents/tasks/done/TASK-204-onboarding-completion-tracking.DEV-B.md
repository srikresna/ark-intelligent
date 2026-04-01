# TASK-204: Onboarding Completion Tracking + Guided Progression

**Priority:** medium
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram/, internal/domain/

## Deskripsi

Track onboarding completion (4 steps) dan show dismissible progress indicator. New users currently click randomly after /start with no guidance.

## Onboarding Steps

```
Step 0: /start sent → Role selector shown
Step 1: Role chosen → "Now run your first command!"
Step 2: First command executed → "Great! Now customize /settings"
Step 3: /settings opened → "Try exploring /calendar or /macro"
Step 4: Second feature explored → "Onboarding complete! 🎉"
```

## Display

After any command, if onboarding incomplete:
```
💡 Onboarding (2/4): ✅Role ✅First cmd ⬜Settings ⬜Explore
   → Try /settings to customize alerts
```

## File Changes

- `internal/domain/prefs.go` — Add OnboardingStep int, OnboardingComplete bool
- `internal/adapter/telegram/handler.go` — After each command, check+advance onboarding step
- `internal/adapter/telegram/formatter.go` — Add onboarding progress footer
- `internal/adapter/telegram/handler.go` — Dismiss button to skip onboarding

## Acceptance Criteria

- [ ] 4-step onboarding tracked per user
- [ ] Contextual hint after each command (until complete)
- [ ] "Skip" button to dismiss onboarding
- [ ] Progress indicator shown until step 4 or dismissed
- [ ] Existing users (created before feature) default to complete
- [ ] No output clutter — hint is single line, dismissible
