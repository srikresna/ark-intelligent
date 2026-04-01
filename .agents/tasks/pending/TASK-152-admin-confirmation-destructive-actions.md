# TASK-152: Admin Confirmation Flow for Destructive Actions

**Priority:** high
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 06:00 WIB
**Siklus:** UX

## Deskripsi
Admin commands `/ban`, `/unban`, `/setrole` execute instantly tanpa konfirmasi. Satu typo di user ID bisa ban wrong user. Tambahkan confirmation step dengan inline keyboard.

## Konteks
- `handler.go:2388-2430` — cmdBan: langsung execute tanpa "Are you sure?"
- `handler.go:2435-2464` — cmdUnban: set ke RoleFree tanpa option restore previous tier
- `handler.go:2323-2384` — cmdSetRole: no confirmation, no audit
- Ref: `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] `/ban <userID>`: show preview "Ban user <userID> (username: X, current role: Y)?" dengan keyboard [✅ Ya, Ban] [❌ Batal]
- [ ] `/unban <userID>`: show "Unban user <userID>? Restore as:" dengan keyboard [Free] [Member] [Batal]
- [ ] `/setrole <userID> <role>`: show "Set role <userID> dari <current> ke <new>?" dengan [✅ Ya] [❌ Batal]
- [ ] Callback handler untuk confirmation buttons
- [ ] Timeout: jika admin tidak confirm dalam 60 detik, cancel otomatis
- [ ] Log semua admin actions ke structured log (admin_id, action, target_id, timestamp)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler.go` (ban, unban, setrole commands)
- `internal/adapter/telegram/keyboard.go` (confirmation keyboard)

## Referensi
- `.agents/research/2026-04-02-06-ux-ai-chat-charts-admin-alerts-deeplink.md`
