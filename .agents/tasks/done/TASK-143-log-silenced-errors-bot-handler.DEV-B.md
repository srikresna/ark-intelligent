# TASK-143: Log Silenced Error Returns in bot.go Critical Paths

**Priority:** medium
**Type:** refactor
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB
**Siklus:** Refactor

## Deskripsi
Di `bot.go:300, 312, 318, 325` — error returns dari `SendHTML()` di-silence dengan `_, _ =`. Jika send error message gagal (rate limit, network), user dan log buta. Tambah error logging.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Replace `_, _ = b.SendHTML(...)` dengan proper error logging:
  ```go
  if _, err := b.SendHTML(ctx, chatID, msg); err != nil {
      log.Error().Err(err).Str("chat_id", chatID).Msg("failed to send error message")
  }
  ```
- [ ] Audit semua `_, _ =` patterns di bot.go — prioritize yang di error handling paths
- [ ] Jangan log untuk non-critical paths (e.g., delete message failures bisa tetap silent)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/bot.go`

## Referensi
- `.agents/research/2026-04-02-04-tech-refactor-post-merge-audit.md`
