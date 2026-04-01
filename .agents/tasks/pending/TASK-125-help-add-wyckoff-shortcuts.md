# TASK-125: Update /help — Tambah /wyckoff + Shortcuts Section

**Priority:** high
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 01:00 WIB
**Siklus:** UX

## Deskripsi
/help command tidak mencantumkan /wyckoff di kategori "Research" dan tidak mendokumentasikan 11 command shortcuts yang sudah diimplementasi (TASK-028). User tidak bisa discover fitur ini.

## Konteks
- `handler.go:413-432` — /help Research category lists /cta, /ctabt, /quant, /vp, /ict, /gex, /backtest, /accuracy, /report — tapi TIDAK /wyckoff
- `handler.go:220-228` — 11 shortcuts: /c→/cot, /cal→/calendar, /out→/outlook, /m→/macro, /b→/bias, /q→/quant, /bt→/backtest, /r→/rank, /s→/sentiment, /p→/price, /l→/levels
- Ini quick win — hanya perlu edit teks di /help handler
- Ref: `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] /wyckoff ditambahkan ke kategori "Research" di /help output
- [ ] Tambah section baru "⚡ Quick Shortcuts" di /help yang list semua aliases:
  - Format: `/c` → `/cot` | `/cal` → `/calendar` | dll
- [ ] Pastikan semua command yang terdaftar di handler juga ada di /help
- [ ] Cek /gex juga ada di /help (sudah ada? verify)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler.go` (cmdHelp function)

## Referensi
- `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`
