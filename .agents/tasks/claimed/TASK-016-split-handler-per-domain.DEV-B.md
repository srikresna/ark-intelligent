# TASK-016: Split handler.go per Domain

**Status:** pending
**Priority:** HIGH
**Effort:** L (Large — estimasi 2-3 jam)
**Cycle:** Siklus 4 — Technical Refactor
**Ref:** TECH-002 in TECH_REFACTOR_PLAN.md

---

## Problem

`internal/adapter/telegram/handler.go` saat ini **2,381 LOC** — god handler yang routing semua command dan callback dalam satu file.

**Dampak:**
- Setiap PR yang tambah command baru → conflict di satu file
- Impossible to navigate — semua /command ada di satu tempat
- Dev agents tidak bisa kerja paralel pada domain berbeda

---

## Solution

Split menjadi per-domain handler files:

```
internal/adapter/telegram/handler/
├── cot.go          ← /cot, /bias, /rank, /rankx
├── calendar.go     ← /calendar
├── macro.go        ← /macro, /transition, /fred
├── price.go        ← /price, /levels, /seasonal
├── analysis.go     ← /cta, /ctabt, /quant, /vp
├── backtest.go     ← /backtest, /accuracy
├── alpha.go        ← /alpha, /xfactors, /heat, /playbook
├── ai.go           ← /outlook, chat handling
├── admin.go        ← /ban, /unban, /users, /setrole, /membership
├── settings.go     ← /settings, /prefs
└── core.go         ← /start, /help, /status, routing registry
```

---

## Acceptance Criteria

- [ ] `go build ./...` clean setelah refactor
- [ ] `go vet ./...` zero warnings
- [ ] Semua existing tests masih pass: `go test ./...`
- [ ] TIDAK ada behavior change — semua commands masih berfungsi
- [ ] Tidak ada file baru >500 LOC
- [ ] Routing registry di `core.go` tetap jelas dan mudah dipahami

---

## Implementation Notes

1. Buat direktori `internal/adapter/telegram/handler/`
2. Extract commands per domain, mulai dari yang paling self-contained (admin.go)
3. `core.go` harus berisi routing logic yang call ke sub-handlers
4. Update `bot.go` untuk import dari package handler baru
5. **Koordinasi dengan TASK-015** — jangan concurrent karena bot.go diubah keduanya

---

## Warning

Ini adalah task yang **BESAR** dan berisiko conflict dengan TASK-015.
**Kerjakan SETELAH TASK-015 selesai** atau di branch berbeda yang explicitly tidak menyentuh formatter.
Branch name: `refactor/split-handler`

---

## Assigned To

(unassigned — tersedia untuk Dev-A, Dev-B, atau Dev-C)
