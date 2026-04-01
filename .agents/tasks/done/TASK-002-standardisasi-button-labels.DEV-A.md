# TASK-002: Standardisasi Button Labels + Universal Home Button

**Priority:** medium
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 01:00 WIB
**Siklus:** UX

## Deskripsi
Standarisasi semua button label di keyboard.go dan formatter.go. Hilangkan mixed language (Indonesia/Inggris di command yang sama). Tambah "🏠 Menu Utama" button di semua multi-step keyboard agar user tidak perlu `/start` ulang.

## Konteks
Saat ini ada 4+ versi back button yang berbeda. User bingung dan experience terasa tidak polished. Lihat `.agents/UX_AUDIT.md#navigation`.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Semua back button standar: `◀ Kembali` (satu level) atau `🏠 Menu Utama` (home)
- [ ] Buat konstanta di `keyboard.go` untuk semua label (tidak ada hardcode string di tempat lain)
- [ ] Tidak ada mixed language di button yang sama
- [ ] Home button ada di semua keyboard yang memiliki lebih dari 1 level navigasi

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/keyboard.go` (konstanta + home button builder)
- `internal/adapter/telegram/formatter.go` (ganti hardcoded strings)
- `internal/adapter/telegram/handler.go` (callback home handler)

## Referensi
- `.agents/research/2026-04-01-01-ux-onboarding-navigation.md`
- `.agents/UX_AUDIT.md` section "Navigation Tidak Konsisten"
