# TASK-103: Message Chunk ID Tracking for Multi-Part Responses

**Priority:** medium
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 20:00 WIB
**Siklus:** UX

## Deskripsi
Saat pesan >4096 chars di-split ke beberapa chunks:
- `SendHTML()` hanya return ID message terakhir — chunk sebelumnya orphaned
- `EditMessage()` hanya edit chunk pertama, overflow jadi message baru tanpa tracking

Ini menyebabkan chat clutter saat user berulang kali klik button yang memicu edit pada response panjang.

## Konteks
- `bot.go:460-503` — SendHTML split logic, hanya return last msgID
- `bot.go:514-530` — EditMessage chunking, overflow IDs lost
- Commands yang sering overflow: /calendar week, /report, /outlook, /cot (multi-currency)
- Ref: `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] `SendHTML()` return slice of message IDs (atau struct yang berisi semua IDs)
- [ ] `EditMessage()` track overflow chunk IDs — pada subsequent edit, delete old overflow chunks sebelum kirim baru
- [ ] Backward compatible — callers yang hanya butuh single ID tetap bisa bekerja (helper method `.LastID()` atau similar)
- [ ] Test: kirim message >8192 chars, edit via callback → tidak ada orphan chunks
- [ ] Pertimbangkan memory: jangan simpan chunk IDs selamanya, TTL-based cleanup

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/bot.go` (SendHTML, EditMessage)
- Callers yang menggunakan return value SendHTML (audit usage)

## Referensi
- `.agents/research/2026-04-01-20-ux-callback-error-session-polish.md`
