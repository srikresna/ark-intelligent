# Task Distribution Guidelines

This document defines the task assignment rules and workflow guidelines for the Agent Multi-Instance Orchestration system.

---

## Priority Levels

Tasks are classified by priority to ensure proper resource allocation and timely delivery:

| Priority | Definition | Response Time | Assignment |
|----------|------------|---------------|------------|
| **Critical** | Blocks release, production outage, security vulnerability | Immediate | Any available agent |
| **High** | Core functionality broken, significant user impact | Within 24 hours | Primary dev agent |
| **Medium** | Important but not blocking, performance degradation | Within 3 days | Next available agent |
| **Low** | Nice to have, documentation, refactoring | Within 1 week | Background task |

---

## Agent Assignment Rules

Tasks are distributed according to agent specialization and current workload:

### Dev-A (Backend/Core)
**Focus Areas:**
- Database schema and storage changes
- Core service logic and business rules
- API integrations and external services
- Performance optimizations and scalability

**Assignment Criteria:**
- Requires understanding of data layer
- Involves critical path business logic
- Touches multiple services or core abstractions

### Dev-B (Features/Handlers)
**Focus Areas:**
- Telegram bot handlers and commands
- New user-facing features
- Feature implementation and enhancements
- Third-party integration work

**Assignment Criteria:**
- User interface or interaction changes
- Platform-specific implementations
- Feature-driven development work

### Dev-C (Testing/Infrastructure)
**Focus Areas:**
- Test coverage improvements
- CI/CD pipeline enhancements
- Code refactoring and cleanup
- Documentation and operational guides

**Assignment Criteria:**
- Does not block other development work
- Improves maintainability or reliability
- Low risk, incremental improvements

### QA (Quality Assurance)
**Focus Areas:**
- Code review and quality gates
- Bug verification and reproduction
- Test plan review and approval
- Merge approval to protected branches

**Assignment Criteria:**
- PR review requested
- Test coverage verification needed
- Release readiness validation

---

## Effort Estimation Guidelines

Use the following framework for estimating task effort:

| Effort | Duration | Description | Examples |
|--------|----------|-------------|----------|
| **XS** | < 1 hour | Quick fix, trivial change | Typo fix, config update |
| **S** | 1-4 hours | Small, well-defined task | Bug fix, simple handler |
| **M** | 4-8 hours | Medium complexity | Feature addition, refactor |
| **L** | 1-2 days | Large, multi-step work | Service implementation |
| **XL** | 3-5 days | Major undertaking | Architecture changes |

**Estimation Rules:**
1. Break down XL tasks into smaller deliverables
2. Add 20% buffer for unknowns on L/XL tasks
3. If uncertain, size up rather than down
4. Update estimate if scope changes

---

## Task Lifecycle

The standard workflow for task progression:

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Research  │ → │   Pending   │ → │ In Progress │ → │    Review   │ → │   Complete  │
│  (Create)   │    │  (Queue)    │    │  (Active)   │    │   (PR/QA)   │    │  (Archive)  │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

### Stage Details

1. **Research creates task spec**
   - Document problem or requirement
   - Define acceptance criteria
   - Estimate effort and priority
   - Place in `.agents/tasks/pending/`

2. **Coordinator assigns via STATUS.md**
   - Match task to agent based on rules above
   - Update agent status to "active"
   - Notify agent of assignment

3. **Agent claims and moves to `in-progress/`**
   - Create feature branch: `feat/TASK-XXX-name`
   - Update task file status to "in_progress"
   - Begin implementation

4. **Agent completes work and creates PR**
   - Validate code (build, test, lint)
   - Create PR with validation evidence
   - Move task to `.agents/tasks/in-review/`
   - Update STATUS.md with PR link

5. **QA reviews and approves**
   - Code review against acceptance criteria
   - Verify tests pass
   - Approve or request changes

6. **Coordinator merges and archives**
   - Merge approved PR to main
   - Move task to `.agents/tasks/completed/`
   - Update STATUS.md (mark idle, archive task)

---

## Task Handoff Procedures

When transferring work between agents:

### Standard Handoff
1. Document current progress in task file
2. List completed and remaining acceptance criteria
3. Note any blockers or dependencies
4. Update STATUS.md with new assignee
5. Brief handoff message in coordination channel

### Emergency Reassignment
1. Coordinator can reassign without notice for Critical priority
2. Current agent commits/pushes all work immediately
3. New agent picks up from last commit
4. Post-hoc documentation update after resolution

---

## Escalation Rules

When tasks become blocked or stalled:

| Blocked Duration | Action | Responsible |
|------------------|--------|-------------|
| **> 2 hours** | Agent documents blocker in task file | Agent |
| **> 4 hours** | Escalate to Coordinator for assistance | Agent |
| **> 8 hours** | Escalate to TechLead for decision | Coordinator |
| **Critical bug** | All agents can be reassigned immediately | Coordinator |

### Escalation Procedures

1. **Agent-level escalation** (> 4 hours blocked)
   - Update task file with blocker details
   - Notify Coordinator in STATUS.md
   - Request assistance or reassignment

2. **TechLead escalation** (> 8 hours blocked)
   - Coordinator summarizes issue
   - TechLead decides: continue, reassign, or deprioritize
   - Decision documented in task file

3. **Critical override** (production incidents)
   - Any agent can halt current work
   - Coordinator immediately reassigns as needed
   - Post-incident review scheduled

---

## Related Documentation

- [Agent Workflows](docs/agent-workflows.md) — Detailed workflow processes
- [Orchestration Guide](ORCHESTRATION_GUIDE.md) — Multi-agent coordination overview
- [STATUS.md](STATUS.md) — Current task state and assignments
- [Research Guide](docs/research-guide.md) — Task creation guidelines
