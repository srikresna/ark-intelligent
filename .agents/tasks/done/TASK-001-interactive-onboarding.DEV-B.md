# TASK-001: Interactive Onboarding dengan Role Selector

**Priority:** high
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 01:00 WIB
**Siklus:** UX

## Deskripsi
Ubah `/start` dari welcome message statis menjadi guided onboarding interaktif dengan role selector. User memilih peran mereka (Pemula / Intermediate / Pro) dan mendapat "starter kit" keyboard yang relevan + brief tutorial 3 langkah.

## Konteks
User baru melihat 28+ command sekaligus, sangat overwhelming. Onboarding yang baik meningkatkan retention dan time-to-first-value. Lihat `.agents/UX_AUDIT.md#onboarding`.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] `/start` menampilkan role selector keyboard: Pemula / Intermediate / Pro
- [ ] Setiap role menampilkan starter kit keyboard (max 4 command)
- [ ] Tutorial 3 langkah singkat setelah pilih role
- [ ] Role disimpan di user preferences BadgerDB
- [ ] Existing user (sudah punya role) skip langsung ke menu utama

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler.go` (cmdStart)
- `internal/adapter/telegram/keyboard.go` (tambah role keyboard)
- `internal/adapter/telegram/bot.go` (callback handler untuk role select)
- `internal/domain/user.go` (tambah Role field jika belum ada)
- `internal/adapter/storage/user_repo.go` (persist role)

## Referensi
- `.agents/research/2026-04-01-01-ux-onboarding-navigation.md`
- `.agents/UX_AUDIT.md` section "Onboarding Buruk"
