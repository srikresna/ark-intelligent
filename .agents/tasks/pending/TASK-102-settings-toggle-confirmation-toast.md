# TASK-102: Settings Toggle AnswerCallback Confirmation Toast

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 20:00 WIB
**Siklus:** UX

## Deskripsi
Saat user toggle setting (alerts on/off, language, compact mode, dll), callback handler update state dan edit message, tapi kirim empty AnswerCallback: `b.AnswerCallback(ctx, cb.ID, "")`. User tidak dapat visual/audio feedback bahwa aksi berhasil.

Tambahkan toast notification yang informatif pada setiap settings toggle.

## Konteks
- `handler.go:1144-1151` — cbSettings callback handler
- Telegram AnswerCallback dengan text non-empty menampilkan toast popup di atas chat
- User terutama di mobile sering tidak notice perubahan di message body — toast lebih eye-catching
- Ref: `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Semua AnswerCallback di settings handler mengirim toast message yang relevan:
  - Alert toggled: `"✅ Alert diaktifkan"` / `"🔕 Alert dinonaktifkan"`
  - Language changed: `"✅ Bahasa: Indonesia"` / `"✅ Language: English"`
  - Compact mode: `"✅ Mode kompak aktif"` / `"✅ Mode normal aktif"`
- [ ] Toast text max 200 chars (Telegram limit)
- [ ] Audit semua callback yang toggle state — pastikan semua ada toast

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler.go` (cbSettings dan related callbacks)
- Kemungkinan handler_*.go yang punya settings-like toggles

## Referensi
- `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`
