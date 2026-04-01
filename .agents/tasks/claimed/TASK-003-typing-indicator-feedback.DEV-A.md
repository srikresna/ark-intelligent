# TASK-003: Typing Indicator dan Progress Feedback

**Priority:** high
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 01:00 WIB
**Siklus:** UX

## Deskripsi
Tambah immediate feedback saat user menjalankan command yang butuh waktu lama (AI generation, data fetching). Gunakan Telegram `sendChatAction` typing indicator + edit "loading" message saat proses berlangsung.

## Konteks
Command `/outlook`, `/quant`, `/cta` bisa butuh 5-15 detik. Tanpa feedback, user mengira bot hang dan kirim command ulang, menyebabkan duplicate requests dan beban server. Lihat `.agents/UX_AUDIT.md#response-time`.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Semua command yang butuh >2 detik langsung kirim "⏳ Sedang menganalisis..." sebelum proses
- [ ] Command AI (outlook, quant) kirim typing action selama processing
- [ ] Pesan loading diedit menjadi hasil akhir (bukan kirim message baru)
- [ ] Helper function `sendLoading(chatID, text)` yang bisa dipakai semua handler

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/bot.go` (helper sendLoading, sendTyping)
- `internal/adapter/telegram/handler.go` (cmdOutlook, cmdCOT, cmdMacro)
- `internal/adapter/telegram/handler_cta.go`
- `internal/adapter/telegram/handler_quant.go`

## Referensi
- `.agents/research/2026-04-01-01-ux-onboarding-navigation.md`
- `.agents/UX_AUDIT.md` section "Response Time Feedback"
