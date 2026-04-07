# TASK-004: Unify Navigation Button Labels

**Priority:** LOW
**Type:** UX Polish
**Ref:** UX_AUDIT.md TASK-UX-001, research/2026-04-05-12-ux-audit-cycle1.md

---

## Problem

Terdapat inkonsistensi label tombol navigasi di berbagai keyboard:
- `"🏠 Home"` (di keyboard_misc.go dan beberapa lainnya)
- `"🏠 Menu Utama"` (di keyboard_help.go dan beberapa lainnya)
- `"◀ Kembali"` (di beberapa keyboard)

UX_AUDIT merekomendasikan konsistensi bahasa Indonesia sebagai default.

---

## Acceptance Criteria

- [ ] Semua tombol "home" menggunakan label yang sama di seluruh codebase
- [ ] Pilih satu: `"🏠 Menu Utama"` (Indonesia) atau `"🏠 Home"` (Inggris)
  - Rekomendasi: `"🏠 Menu Utama"` sesuai TASK-UX-005
- [ ] Audit semua `keyboard_*.go` untuk konsistensi
- [ ] `go build ./...` bersih

---

## Implementation

1. Grep semua label: `grep -rn '"🏠' internal/adapter/telegram/ --include="*.go"`
2. Tentukan label standar
3. Replace semua inkonsistensi

Pertimbangkan membuat konstanta:
```go
// keyboard.go atau keyboard_misc.go
const BtnHome = "🏠 Menu Utama"
const BtnBack = "◀ Kembali"
```

---

## Notes

Perubahan ini murni kosmetik — tidak ada behavior change.
