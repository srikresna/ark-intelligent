# Agent Workflow Documentation

**Created:** 2026-04-03  
**Requested by:** Board User (PHI-121)  
**Created by:** TechLead-Intel

---

## Overview

ARK Intelligent uses a multi-agent system with specialized roles. Each agent operates on a continuous heartbeat cycle, picking up tasks from Paperclip and executing them.

---

## 1. TechLead-Intel (Me)

**Role:** Tech Lead - ARK Intelligent  
**Reports to:** CEO  
**Manages:** Dev-A, Dev-B, Dev-C, QA, Research

### Workflow (6-Step Loop)

```
1. SYNC
   - git checkout agents/main && git pull
   - Read .agents/STATUS.md
   - Read .agents/DIRECTION.md

2. TRIAGE
   - Check .agents/tasks/pending/
   - Check .agents/escalations/
   - Check open PRs

3. PLAN
   - Assign tasks to available dev agents
   - Write Research questions
   - Write QA assignments

4. REVIEW
   - Review open PRs
   - Merge or request changes
   - Write ADRs if needed

5. REPORT
   - Update .agents/STATUS.md
   - Escalate if blocker > 4h

6. LOOP → back to step 1
```

### Key Responsibilities
- Monitor Research→Dev→QA cycle
- Assign work to Dev-A, Dev-B, Dev-C
- Review and merge PRs
- Update STATUS.md after every loop
- Escalate blockers to CTO

---

## 2. Dev-A (Senior Developer + Reviewer)

**Role:** Developer A - ARK Intelligent  
**Reports to:** TechLead-Intel  
**Specialty:** Senior implementation + code review

### Workflow

```
1. HEARTBEAT WAKE
   - Check Paperclip inbox

2. CHECKOUT TASK
   - POST /api/issues/{id}/checkout
   - Fail if 409 (task owned by another agent)

3. IMPLEMENT
   - Read task specification
   - Create feature branch: feat/TASK-xxx
   - Implement solution
   - Run go build && go vet

4. COMMIT
   - Commit with descriptive message
   - Add Co-Authored-By: Paperclip

5. SUBMIT PR
   - Push branch to origin
   - Create PR for QA review
   - Update task status to in_review

6. RELEASE
   - POST /api/issues/{id}/release
```

### Recent Completed Work
- PHI-105: HandlerDeps struct (TASK-094-D)
- PHI-108: wire_storage.go (TASK-094-C1)
- PHI-112: wire_services.go (TASK-094-C2)
- PHI-115: wire_telegram.go + wire_schedulers.go (TASK-094-C3)
- PHI-118: Button standardization (TASK-002)

---

## 3. Dev-B (Pure Implementor)

**Role:** Developer B - ARK Intelligent  
**Reports to:** TechLead-Intel  
**Specialty:** Feature implementation

### Workflow

```
Same as Dev-A:
1. Heartbeat wake
2. Checkout task
3. Implement on feature branch
4. Test (go build, go vet)
5. Commit and push
6. Submit PR
7. Release task
```

### Recent Completed Work
- PHI-110: Split handler.go per domain (TASK-016)
- PHI-116: Interactive onboarding (TASK-001)
- PHI-117: Typing indicators (TASK-003)
  - /outlook, /quant, /cta, /backtest, /report, /accuracy
- PHI-120: Error messages layer (TASK-005)
  - errors.go (251 LOC)
  - errors_test.go (236 LOC)

---

## 4. Dev-C (Pure Implementor)

**Role:** Developer C - ARK Intelligent  
**Reports to:** TechLead-Intel  
**Specialty:** Feature implementation + migrations

### Workflow

```
Same as Dev-A and Dev-B
```

### Recent Completed Work
- PHI-113: httpclient.New() migration (TASK-306-EXT)
  - 18 services migrated
- PHI-119: Compact output mode (TASK-004)
  - /cot compact view + expand button
  - /macro compact view + expand button

---

## 5. QA Engineer

**Role:** QA Engineer - ARK Intelligent  
**Reports to:** TechLead-Intel  
**Specialty:** PR review, testing, merging

### Workflow

```
1. CHECK FOR PENDING PRs
   - Review open PRs from Dev-A, Dev-B, Dev-C

2. CODE REVIEW
   - Check code quality
   - Verify test coverage
   - Run go build && go vet

3. TEST
   - Test implementation manually if needed
   - Verify acceptance criteria

4. MERGE OR REQUEST CHANGES
   - Merge if passes all checks
   - Or request changes with comments

5. UPDATE STATUS
   - Move task to done/
   - Update Paperclip status
```

### Current Queue
- 4 PRs awaiting review:
  1. feat/TASK-002-button-standardization (Dev-A)
  2. feat/PHI-119-compact-output (Dev-C)
  3. feat/TASK-094-C3 (Dev-A)

---

## 6. Research Lead

**Role:** Research Lead - ARK Intelligent  
**Reports to:** TechLead-Intel  
**Specialty:** Audits, research, task specification

### Workflow

```
1. AUDIT CODEBASE
   - Find issues, gaps, improvements
   - Write research documents in .agents/research/

2. CREATE TASK SPECIFICATIONS
   - Write detailed task files
   - Define acceptance criteria
   - Prioritize tasks

3. HANDOFF TO TECHLEAD-INTEL
   - Research writes specs
   - TechLead assigns to devs

4. ADR WRITING
   - Architecture Decision Records
   - Document major technical decisions
```

### Recent Research
- 2026-04-01-adr-di-framework.md — DI framework evaluation
- 2026-04-01-01-ux-onboarding-navigation.md — UX audit with 5 tasks

---

## Task Lifecycle

```
┌─────────────┐
│   Research  │
│   (Audit)   │
└──────┬──────┘
       │ Creates spec
       ▼
┌─────────────┐
│  TechLead   │
│  (Assign)   │
└──────┬──────┘
       │ Assigns to Dev-A/B/C
       ▼
┌─────────────┐     ┌─────────────┐
│    Dev      │────▶│     QA      │
│(Implement)  │ PR  │ (Review)    │
└─────────────┘     └──────┬──────┘
                           │ Merge
                           ▼
                    ┌─────────────┐
                    │    Done     │
                    └─────────────┘
```

---

## Communication Flow

1. **Paperclip** — Source of truth for tasks
2. **GitHub** — Code repository, PRs
3. **STATUS.md** — Team status board
4. **Task files** — Specifications in .agents/tasks/

---

## Current Sprint Status (2026-04-03)

### Completed ✅
- TASK-001: Onboarding (PHI-116) — Dev-B
- TASK-002: Button standardization (PHI-118) — Dev-A
- TASK-003: Typing indicators (PHI-117) — Dev-B
- TASK-004: Compact output (PHI-119) — Dev-C (PR submitted)
- TASK-005: Error messages (PHI-120) — Dev-B

### In QA Review 👀
- 4 PRs awaiting merge

### Next
- All dev agents IDLE, awaiting new assignments
