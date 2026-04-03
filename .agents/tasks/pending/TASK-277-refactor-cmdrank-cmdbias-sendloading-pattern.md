# TASK-277: Refactor cmdRank + cmdBias ke Pola SendLoading

**Priority:** medium
**Type:** refactor
**Estimated:** S
**Area:** internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 25:00 WIB

## Deskripsi

`cmdRank` dan `cmdBias` menggunakan pola loading yang berbeda dari handler lain:

```go
// Pola lama (cmdRank ~line 1956, cmdBias ~line 1255):
loadingID, _ := h.bot.SendHTML(ctx, chatID, "📈 Menghitung... ⏳")
// ... do work ...
_ = h.bot.DeleteMessage(ctx, chatID, loadingID)  // delete, lalu kirim result baru
_, err = h.bot.SendHTML(ctx, chatID, result)
```

Masalah pola ini:
1. `SendHTML` tidak kirim typing indicator — user tidak lihat "typing..." di chat
2. Delete + SendNew → ada flash/jump visual di chat (dua operasi API)
3. Error tidak di-edit ke loading message — langsung kirim message baru

**Pola standar** (dipakai cmdMacro, cmdSentiment, cmdSMC, cmdQuant, cmdAlpha, cmdWyckoff, dll):
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, "📈 Menghitung... ⏳")
// ... do work ...
if err != nil {
    h.editUserError(ctx, chatID, loadingID, err, "rank")
    return nil
}
_ = h.bot.EditMessage(ctx, chatID, loadingID, result)
```

`SendLoading` = `SendTyping` + `SendHTML`. Hasilnya: typing indicator muncul, lalu message diedit (tidak ada flash).

## File yang Harus Diubah

### internal/adapter/telegram/handler.go

#### 1. cmdRank (~line 1956)

**Sebelum:**
```go
func (h *Handler) cmdRank(...) error {
    loadingID, _ := h.bot.SendHTML(ctx, chatID, "📈 Menghitung currency strength ranking... ⏳")
    analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
    if err != nil || len(analyses) == 0 {
        if loadingID > 0 {
            _ = h.bot.DeleteMessage(ctx, chatID, loadingID)
        }
        _, err = h.bot.SendHTML(ctx, chatID, "No COT data available...")
        return err
    }
    // ... build output ...
    if loadingID > 0 {
        _ = h.bot.DeleteMessage(ctx, chatID, loadingID)
    }
    _, err = h.bot.SendWithKeyboard(ctx, chatID, output, keyboard)
    return err
}
```

**Sesudah:**
```go
func (h *Handler) cmdRank(...) error {
    loadingID, _ := h.bot.SendLoading(ctx, chatID, "📈 Menghitung currency strength ranking... ⏳")
    analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
    if err != nil || len(analyses) == 0 {
        h.editUserError(ctx, chatID, loadingID, fmt.Errorf("no COT data"), "rank")
        return nil
    }
    // ... build output ...
    _ = h.bot.EditMessageWithKeyboard(ctx, chatID, loadingID, output, keyboard)
    return nil
}
```

#### 2. cmdBias (~line 1255) — pola yang sama

## Verifikasi

```bash
go build ./...
go test ./internal/adapter/telegram/...
```

Test manual: `/rank` → harus ada typing indicator sebelum hasil muncul, dan message diedit (bukan kirim baru).

## Acceptance Criteria

- [ ] `cmdRank` menggunakan `SendLoading` di awal
- [ ] `cmdRank` menggunakan `EditMessage` (bukan DeleteMessage + SendHTML) untuk hasil
- [ ] `cmdBias` menggunakan `SendLoading` di awal
- [ ] `cmdBias` menggunakan `EditMessage` untuk hasil
- [ ] Error handling menggunakan `editUserError` (bukan sendUserError)
- [ ] `go build ./...` clean
- [ ] Typing indicator terlihat di Telegram sebelum hasil muncul

## Referensi

- `.agents/research/2026-04-02-25-ux-audit-dead-callbacks-loading-patterns-putaran17.md` — ISSUE-UX3
- `internal/adapter/telegram/handler.go` — cmdRank (~1956), cmdBias (~1255)
- `internal/adapter/telegram/api.go` — SendLoading, EditMessage, EditMessageWithKeyboard
