# TASK-254: SettingsMenu + FormatSettings — Tampilkan dan Ubah ExperienceLevel

**Priority:** medium
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram/keyboard.go, formatter.go, handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 04:00 WIB

## Deskripsi

`ExperienceLevel` di UserPrefs dikontrol hanya saat onboarding pertama (`/start`). Setelah di-set, tidak ada cara untuk melihat atau mengubahnya tanpa reset database.

Dua masalah sekaligus:
1. `FormatSettings()` (formatter.go:932) tidak menampilkan `ExperienceLevel` — user tidak tahu level mereka
2. `SettingsMenu()` (keyboard.go:320) tidak punya tombol untuk mengubah ExperienceLevel

User yang onboarding sebagai "pemula" dan sekarang ingin switch ke "pro" harus minta admin atau workaround manual.

## File yang Harus Diubah

- `internal/adapter/telegram/formatter.go` — `FormatSettings()`: tampilkan ExperienceLevel
- `internal/adapter/telegram/keyboard.go` — `SettingsMenu()`: tambah tombol "Ubah Level"
- `internal/adapter/telegram/handler.go` — `cbSettings`: handle `set:reset_onboard` callback

## Implementasi

### 1. formatter.go — FormatSettings()

Tambah setelah baris alert currencies display (sebelum `"\n<i>Use the buttons below..."`):

```go
// Experience level display
levelDisplay := "Belum diset"
switch prefs.ExperienceLevel {
case "beginner":
    levelDisplay = "🌱 Pemula"
case "intermediate":
    levelDisplay = "📈 Intermediate"
case "pro":
    levelDisplay = "🏛 Pro / Institutional"
}
b.WriteString(fmt.Sprintf("<code>Experience Level   : %s</code>\n", levelDisplay))
```

### 2. keyboard.go — SettingsMenu()

Tambah row baru sebelum "📜 View Changelog":

```go
// Row: Reset / Change experience level
rows = append(rows, []ports.InlineButton{{
    Text:         "🔄 Ubah Level Pengalaman",
    CallbackData: "set:reset_onboard",
}})
```

### 3. handler.go — cbSettings: tambah case baru

```go
case "reset_onboard":
    // Clear experience level, then re-show onboarding
    prefs.ExperienceLevel = ""
    _ = h.prefsRepo.Set(ctx, userID, prefs)
    _ = h.bot.DeleteMessage(ctx, chatID, msgID)
    return h.cmdStart(ctx, chatID, userID, "")
```

`cmdStart` sudah memanggil onboarding flow jika `ExperienceLevel == ""` (handler.go:270).

## Acceptance Criteria

- [ ] `/settings` menampilkan `Experience Level: 🌱 Pemula` (atau level yang sesuai)
- [ ] `/settings` menampilkan tombol "🔄 Ubah Level Pengalaman"
- [ ] Klik tombol → hapus settings message, tampilkan onboarding role selector
- [ ] Setelah pilih level baru → level tersimpan, StarterKitMenu sesuai level baru
- [ ] User yang belum set level → tampil "Belum diset" di settings
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-04-ux-audit-navigation-context-settings-putaran12.md` — Temuan 4 & 5
- `formatter.go:932` — FormatSettings() (perlu tambah ExperienceLevel display)
- `keyboard.go:320` — SettingsMenu() (perlu tambah tombol)
- `handler.go:270` — cmdStart: check ExperienceLevel == "" (onboarding trigger)
- `handler.go:288` — cbOnboard: set ExperienceLevel (referensi flow)
- `UX_AUDIT.md: TASK-UX-002` — Onboarding Flow improvements
