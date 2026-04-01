# TASK-078: Pinned Commands — Personalized Main Keyboard

**Priority:** low
**Type:** feature
**Estimated:** M (3-4 jam)
**Area:** internal/domain/prefs.go, keyboard.go, handler.go
**Created by:** Research Agent
**Created at:** 2026-04-01 19:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 3)
**Ref:** UX_AUDIT.md TASK-UX-014

## Deskripsi

Semua user melihat keyboard yang sama di /start dan /help.
Institutional trader butuh akses cepat ke /cot, /outlook, /macro.
Retail trader butuh /bias, /calendar.
Menambah personalized "pins" di top keyboard akan meningkatkan UX secara signifikan.

## Domain Change (prefs.go)

```go
// Tambah ke UserPrefs struct:
PinnedCommands []string `json:"pinned_commands,omitempty"` // max 4, e.g. ["cot EUR", "outlook", "macro"]
```

## Fitur

### /pin command
```
/pin cot EUR    → tambah "cot EUR" ke PinnedCommands
/pin outlook    → tambah "outlook"
/pin macro      → tambah "macro"
/pins           → lihat semua pin
/unpin outlook  → hapus "outlook" dari pins
```

Max 4 pins. Jika sudah 4, tolak dengan instruksi `/unpin`.

### Personalized MainMenu
`keyboard.go` `MainMenu(pins []string)` terima parameter pins:
```go
func (kb *KeyboardBuilder) MainMenu(pins []string) ports.InlineKeyboard {
    var rows [][]ports.InlineButton
    
    // Tambah pinned row jika ada
    if len(pins) > 0 {
        var pinnedRow []ports.InlineButton
        for _, pin := range pins {
            pinnedRow = append(pinnedRow, ports.InlineButton{
                Text:         "⭐ " + strings.ToUpper(pin),
                CallbackData: "cmd:" + strings.ReplaceAll(pin, " ", ":"),
            })
        }
        rows = append(rows, pinnedRow)
    }
    
    // ... rest of standard menu rows
}
```

Handler yang memanggil `h.kb.MainMenu()` diupdate untuk pass user pins.

## Acceptance Criteria
- [ ] `UserPrefs.PinnedCommands` field ada
- [ ] `/pin <command>` menambah ke pins (max 4, validate command valid)
- [ ] `/unpin <command>` menghapus dari pins
- [ ] `/pins` atau `/pin` tanpa args menampilkan list pins saat ini
- [ ] MainMenu menampilkan row "⭐ Pinned" jika user punya pins
- [ ] `go build ./...` clean
