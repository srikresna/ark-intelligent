# Escalation: GitHub CLI Authentication Blocker

**Date:** 2026-04-03  
**Escalated by:** TechLead-Intel  
**Severity:** HIGH — Blocks PR submission  
**Status:** 🔴 ACTIVE

---

## Problem

GitHub CLI (`gh`) is not authenticated in the agent environment. This blocks PR creation, which is essential for the sprint workflow.

**Error encountered:**
```
To get started with GitHub CLI, please run: gh auth login
Alternatively, populate the GH_TOKEN environment variable with a GitHub API authentication token.
```

---

## Impact

**5 PRs ready for submission are blocked:**
1. TASK-002 (Dev-A) — Button standardization
2. TASK-094-C3 (Dev-A) — DI wiring
3. TASK-094-D (Dev-A) — HandlerDeps struct
4. TASK-001-EXT (Dev-B) — Onboarding role selector
5. PHI-119 (Dev-C) — Compact output

**All branches are pushed to origin** — only PR creation is blocked.

---

## Resolution Options

### Option 1: Environment Variable (Preferred for automation)
Set `GH_TOKEN` environment variable with a GitHub Personal Access Token.

### Option 2: Interactive Login
Run `gh auth login` and complete the OAuth flow.

### Option 3: Use Git directly with curl
Fall back to Git commands + curl API calls for PR creation.

---

## Action Required

**CTO/DevOps:** Please configure GitHub authentication for the agent environment to enable PR creation.

---

## References

- [GitHub CLI Authentication Docs](https://cli.github.com/manual/gh_auth_login)
- STATUS.md loop #66 for full PR queue status
