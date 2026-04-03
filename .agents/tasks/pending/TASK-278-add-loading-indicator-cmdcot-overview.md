# TASK-278: Tambah Loading Indicator di cmdCOT Overview

**Priority:** low
**Type:** enhancement
**Estimated:** XS
**Area:** internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 25:00 WIB

## Deskripsi

`cmdCOT` (tanpa args — overview mode) tidak memiliki loading feedback sama sekali. Langsung query BadgerDB dan kirim hasil. Semua command utama lainnya (cmdMacro, cmdSentiment, cmdRank, cmdBias, cmdAlpha, dll) setidaknya mengirim typing indicator atau loading message.

Ketika BadgerDB cold start atau scan memakan waktu, user tidak tahu apakah bot sedang bekerja atau hang.

Perlu tambah minimal `SendTyping` di awal overview path, atau pola `SendLoading` + `EditMessage` jika ingin konsisten dengan handler berat.

## File yang Harus Diubah

### internal/adapter/telegram/handler.go — cmdCOT (~line 550)

Temukan path "overview mode" (ketika `len(parts) == 0` atau setelah semua currency-specific args diproses):

**Minimal fix:**
```go
func (h *Handler) cmdCOT(ctx context.Context, chatID string, userID int64, args string) error {
    parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))

    if len(parts) > 0 {
        // currency-specific path — sudah cepat, skip loading
        // ... existing code ...
    }

    // Overview path — tambah typing indicator
    h.bot.SendTyping(ctx, chatID)

    // ... existing overview code ...
}
```

**Atau pola lengkap (preferred, konsisten dengan handler lain):**
```go
// Overview path
loadingID, _ := h.bot.SendLoading(ctx, chatID, "📊 Memuat COT overview... ⏳")
analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
if err != nil || len(analyses) == 0 {
    h.editUserError(ctx, chatID, loadingID, err, "cot")
    return nil
}
// ... build output ...
_ = h.bot.EditMessageWithKeyboard(ctx, chatID, loadingID, output, keyboard)
```

Pilih antara minimal fix (SendTyping saja) atau pola lengkap. Minimal fix lebih aman karena tidak mengubah struktur render.

## Verifikasi

```bash
go build ./...
```

Test manual: `/cot` → harus ada "typing..." indicator di chat.

## Acceptance Criteria

- [ ] `/cot` (overview mode) menampilkan typing indicator sebelum hasil
- [ ] Tidak ada perubahan pada output/format
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-25-ux-audit-dead-callbacks-loading-patterns-putaran17.md` — ISSUE-UX4
- `internal/adapter/telegram/handler.go` — cmdCOT (~line 550)
- `internal/adapter/telegram/api.go` — SendTyping, SendLoading
