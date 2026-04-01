# TASK-128: Wyckoff Keyboard Navigation + Related Commands

**Priority:** medium
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-02 01:00 WIB
**Siklus:** UX

## Deskripsi
Wyckoff output dikirim sebagai teks plain tanpa inline keyboard. User harus type command manual untuk analysis berikutnya. Tambahkan keyboard navigation: symbol selector, timeframe toggle, refresh, dan related commands.

## Konteks
- `handler_wyckoff.go:107-113` — SendHTML tanpa keyboard
- Contrast: GEX punya symbol switcher + refresh (`handler_gex.go:100-106`)
- Semua analysis commands seharusnya punya keyboard konsisten
- Related commands setelah Wyckoff: /cta [symbol], /ict [symbol], /levels [symbol]
- Ref: `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Tambah inline keyboard di Wyckoff output:
  - Row 1: Symbol selector (top 5 forex pairs: EUR, GBP, JPY, AUD, CHF)
  - Row 2: Timeframe toggle (1H, 4H, D)
  - Row 3: [🔄 Refresh] [📊 CTA] [🏗️ ICT] [📐 Levels]
  - Row 4: [◀ Kembali]
- [ ] Callback handlers untuk Wyckoff keyboard interactions
- [ ] State management (TTL cache) untuk Wyckoff callback data
- [ ] "Related commands" row mengirim user ke /cta, /ict, /levels dengan symbol yang sama

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_wyckoff.go` (add keyboard + callbacks)
- `internal/adapter/telegram/keyboard.go` (Wyckoff keyboard builder)

## Referensi
- `.agents/research/2026-04-02-01-ux-new-features-wyckoff-gex-discovery.md`
