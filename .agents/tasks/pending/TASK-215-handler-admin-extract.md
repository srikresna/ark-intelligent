# TASK-215: Extract handler_admin.go from handler.go

**Priority:** HIGH
**Type:** Tech Refactor
**Estimated:** S
**Area:** internal/adapter/telegram/handler.go → handler_admin.go
**Ref:** TECH-002 in TECH_REFACTOR_PLAN.md (first step)
**Created by:** Research Agent
**Created at:** 2026-04-02 08:00 WIB
**Siklus:** 4 — Technical Refactor

## Problem

`handler.go` tumbuh dari 2,381 → 2,909 LOC karena setiap command baru langsung masuk ke sini. Admin commands (ban, unban, users, setRole, membership) tidak ada hubungannya dengan COT/macro/calendar logic tapi berada di file yang sama, menyebabkan merge conflict setiap kali ada dev yang touch handler.go.

Admin block yang perlu dipindahkan (~280 LOC):
- `cmdMembership` (line ~2329, ~75 LOC)
- `requireAdmin` (line ~2404, ~13 LOC)
- `cmdUsers` (line ~2417, ~23 LOC)
- `cmdSetRole` (line ~2440, ~65 LOC)
- `cmdBan` (line ~2505, ~47 LOC)
- `cmdUnban` (line ~2552, ~33 LOC)
- `notifyOwnerDebug` (line ~2585, ~16 LOC)

## Approach

1. Buat `internal/adapter/telegram/handler_admin.go` dengan package `telegram`
2. Pindahkan 7 fungsi di atas ke file baru (cut & paste — jangan ubah logic)
3. Hapus dari handler.go
4. `go build ./...` harus clean (tidak ada behavior change)

## File Changes

- `internal/adapter/telegram/handler.go` — REMOVE admin functions (~280 LOC berkurang)
- `internal/adapter/telegram/handler_admin.go` — NEW file, same package

## Acceptance Criteria

- [ ] handler_admin.go baru berisi 7 fungsi admin
- [ ] handler.go kehilangan ~280 LOC
- [ ] `go build ./... && go vet ./...` clean
- [ ] Tidak ada behavior change — pure move, no logic change
- [ ] 1 PR, branch: `refactor/handler-admin-extract`
