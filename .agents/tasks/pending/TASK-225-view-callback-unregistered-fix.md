# TASK-225: Daftarkan view: Callback di Register() — Tombol Compact/Full Tidak Berfungsi

**Priority:** high
**Type:** bug
**Estimated:** XS
**Area:** internal/adapter/telegram/
**Created by:** Research Agent
**Created at:** 2026-04-02 18:00 WIB

## Deskripsi

`cbViewToggle` di `handler.go:1788` menangani callback data `view:full:cot`, `view:compact:cot`, `view:full:macro`, `view:compact:macro`. Tombol ini muncul di COT overview dan Macro summary sebagai "📖 Detail Lengkap" / "📊 Compact".

Namun `RegisterCallback("view:", h.cbViewToggle)` **tidak ada** di `Register()` (handler.go:240-251). Akibatnya semua klik tombol compact/expand di-drop secara diam-diam — bot tidak merespons, user mengira fitur hang.

## Root Cause

`handler.go` Register() tidak menyertakan `"view:"` prefix:
```go
// Missing:
bot.RegisterCallback("view:", h.cbViewToggle)
```

## File yang Harus Diubah

- `internal/adapter/telegram/handler.go` — tambah satu baris RegisterCallback di Register()

## Acceptance Criteria

- [ ] `bot.RegisterCallback("view:", h.cbViewToggle)` ditambahkan di fungsi `Register()`
- [ ] Klik "📖 Detail Lengkap" di /cot → toggle ke full mode dan update pesan
- [ ] Klik "📊 Compact" di /cot → toggle ke compact mode dan update pesan
- [ ] Klik toggle di /macro → identik
- [ ] Log di handler.go diupdate: `Int("callbacks", 11)` (dari 10)

## Referensi

- `.agents/research/2026-04-02-18-ux-navigation-keyboard-gaps-putaran7.md` — Temuan 1
- `handler.go:1788` — cbViewToggle implementation
- `handler.go:240` — RegisterCallback list
