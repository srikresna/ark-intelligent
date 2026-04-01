# TASK-005: User-Friendly Error Messages Layer

**Priority:** high
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 01:00 WIB
**Siklus:** UX

## Deskripsi
Buat layer error handling yang menerjemahkan technical error menjadi pesan user-friendly dengan actionable suggestion. Pisahkan internal logging (detail teknis) dari user-facing message (ramah dan actionable).

## Konteks
Saat ini error messages expose internal error string ke user (context deadline exceeded, badger: key not found, dll). User trader tidak perlu dan tidak bisa berbuat apa-apa dengan info itu. Lihat `.agents/UX_AUDIT.md#error-messages`.

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat file `internal/adapter/telegram/errors.go`
- [ ] Fungsi `userFriendlyError(err error) string` yang map error ke pesan ramah
- [ ] Semua `fmt.Sprintf("Error: %v", err)` di handler diganti dengan helper ini
- [ ] Pesan error selalu ada actionable suggestion ("Coba lagi dengan /cot", "Hubungi admin jika terus terjadi")
- [ ] Technical error tetap di-log via zerolog (tidak hilang)
- [ ] Minimal cover: timeout error, data not found, API unavailable, quota exceeded

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/errors.go` (file baru)
- `internal/adapter/telegram/handler.go` (ganti error formatting)
- `internal/adapter/telegram/handler_cta.go`
- `internal/adapter/telegram/handler_quant.go`
- `internal/adapter/telegram/handler_alpha.go`

## Referensi
- `.agents/research/2026-04-01-01-ux-onboarding-navigation.md`
- `.agents/UX_AUDIT.md` section "Error Messages Tidak User-Friendly"
