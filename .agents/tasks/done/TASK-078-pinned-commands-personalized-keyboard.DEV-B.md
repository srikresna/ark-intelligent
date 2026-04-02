# TASK-078: Pinned Commands — Personalized Main Keyboard

**Status:** done
**Agent:** Dev-B
**Branch:** feat/TASK-078-pinned-commands
**PR:** #259
**Completed:** 2026-04-02

## Summary
- Added /pin, /unpin, /pins commands (max 4 pins per user)
- Added pinnedRow helper to KeyboardBuilder
- Updated HelpCategoryMenu/WithAdmin to show pinned commands at top
- Wired pins into sendHelp and cbHelp back navigation
- Validated against known cbQuickCommand routes
- go build + go vet clean
