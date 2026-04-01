# TASK-004: Compact Mode Output + Expand Button

**Priority:** medium
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 01:00 WIB
**Siklus:** UX

## Deskripsi
Tambah "compact mode" sebagai default output. User bisa expand ke full detail via button. Simpan preferensi per user di BadgerDB. Trader mobile sangat terbantu dengan output ringkas.

## Konteks
Output `/cot` dan `/macro` bisa >4000 karakter dengan ASCII table yang tidak bagus di semua font Telegram. User trader biasanya butuh signal dan key number saja, bukan full report. Lihat `.agents/UX_AUDIT.md#output-density`.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Default output: compact (summary + top 3 key numbers + signal)
- [ ] Button "📖 Detail Lengkap" untuk toggle ke full output
- [ ] Button "📊 Compact" untuk kembali ke compact
- [ ] Preference disimpan per user (persistent via BadgerDB)
- [ ] Setting bisa diubah via `/settings` → "Format Output"
- [ ] Compact output max 1500 karakter

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/formatter.go` (formatCOTCompact, formatMacroCompact, dll)
- `internal/adapter/telegram/keyboard.go` (compact/full toggle button)
- `internal/adapter/telegram/handler.go` (callback expand/compact)
- `internal/domain/preferences.go` (tambah OutputMode field)
- `internal/adapter/storage/preferences_repo.go`

## Referensi
- `.agents/research/2026-04-01-01-ux-onboarding-navigation.md`
- `.agents/UX_AUDIT.md` section "Output Terlalu Dense" + "TASK-UX-010"
