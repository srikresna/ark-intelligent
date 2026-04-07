# TASK-229: Tambah Global OutputMode Toggle ke Settings Menu

**Priority:** low
**Type:** ux
**Estimated:** S
**Area:** internal/adapter/telegram/
**Created by:** Research Agent
**Created at:** 2026-04-02 18:00 WIB

## Deskripsi

`domain.UserPrefs.OutputMode` (prefs.go:73) menyimpan preferensi global compact/full (`"compact"` atau `"full"`). Field ini juga diset oleh `cbViewToggle` (handler.go:1800-1803) ketika user klik toggle per-command.

Namun `SettingsMenu` (keyboard.go:320) **tidak memiliki tombol untuk mengubah OutputMode secara langsung**. User tidak tahu bahwa preferensi ini ada, dan tidak bisa reset ke default tanpa klik per-command toggle (yang juga sebelumnya broken per TASK-225).

## File yang Harus Diubah

- `internal/adapter/telegram/keyboard.go` — tambah row baru di SettingsMenu()
- `internal/adapter/telegram/handler.go` — tambah `case "outputmode_toggle":` di cbSettings
- `internal/adapter/telegram/formatter.go` — tampilkan current OutputMode di FormatSettings

## Implementasi

### keyboard.go — SettingsMenu()
Tambah sebelum row "📜 View Changelog":
```go
// Row: OutputMode toggle
outputLabel := "📖 Output Mode: Compact → Full"
if prefs.OutputMode == domain.OutputFull {
    outputLabel = "📊 Output Mode: Full → Compact"
}
rows = append(rows, []ports.InlineButton{{
    Text:         outputLabel,
    CallbackData: "set:outputmode_toggle",
}})
```

### handler.go — cbSettings switch
```go
case "outputmode_toggle":
    if prefs.OutputMode == domain.OutputFull {
        prefs.OutputMode = domain.OutputCompact
    } else {
        prefs.OutputMode = domain.OutputFull
    }
```

### formatter.go — FormatSettings
Tambahkan baris status OutputMode di settings display:
```
📊 Output Mode: Compact  (atau Full)
```

## Acceptance Criteria

- [ ] /settings menampilkan tombol toggle OutputMode
- [ ] Klik toggle → OutputMode berubah antara compact dan full
- [ ] FormatSettings menampilkan current OutputMode
- [ ] Perubahan OutputMode via settings langsung berlaku saat user pakai /cot atau /macro berikutnya
- [ ] Default (empty string) diperlakukan sebagai "compact" secara konsisten

## Referensi

- `.agents/research/2026-04-02-18-ux-navigation-keyboard-gaps-putaran7.md` — Temuan 5
- `prefs.go:73` — OutputMode field di UserPrefs
- `keyboard.go:320` — SettingsMenu (perlu tambah row)
- `handler.go:1800` — cbViewToggle yang sudah set OutputMode
- TASK-225 — fix view: callback registration
