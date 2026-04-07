# TASK-003: Implement OutputMinimal Mode

**Priority:** MEDIUM
**Type:** Feature Completion (Dead Code Fix)
**Ref:** UX_AUDIT.md TASK-UX-010, research/2026-04-05-12-ux-audit-cycle1.md

---

## Problem

`domain.OutputMinimal` mode sudah ada di prefs, bisa disimpan via `/settings`, 
tapi **tidak ada efek** karena tidak ada handler yang mengeceknya.

Hanya 2 handler yang mengecek OutputMode sama sekali:
- `handler_cot_cmd.go`: hanya `OutputFull` vs `OutputCompact`
- `handler_macro_cmd.go`: hanya `OutputFull`

User yang set "Minimal" di settings akan bingung karena output tidak berubah.

---

## Definisi Minimal Mode

Minimal = hanya signal/bias direction + strength number. Tidak ada tabel, tidak ada breakdown. Format: satu baris per currency.

Contoh minimal COT:
```
📊 COT Minimal — 01 Apr 2026
🟢 EUR: LONG ●●● (+45.2k)
🔴 GBP: SHORT ●●○ (-23.1k)
➡ JPY: NEUTRAL ●○○ (+2.3k)
...
```

---

## Acceptance Criteria

- [ ] `/cot` menampilkan format minimal kalau `prefs.OutputMode == domain.OutputMinimal`
- [ ] `/macro` menampilkan format minimal kalau `prefs.OutputMode == domain.OutputMinimal`
- [ ] `/cta` menampilkan format minimal kalau `prefs.OutputMode == domain.OutputMinimal`
- [ ] Format minimal max 15 baris, hanya key info
- [ ] `go build ./...` bersih

---

## Implementation

**1. Tambah formatter minimal di `formatter_compact.go`:**
```go
func (f *Formatter) FormatCOTOverviewMinimal(analyses []domain.COTAnalysis, convictions []cot.ConvictionScore) string
func (f *Formatter) FormatMacroMinimal(regime fred.MacroRegime) string
```

**2. Update handler checks:**

`handler_cot_cmd.go` (sekitar baris 571):
```go
switch prefs.OutputMode {
case domain.OutputMinimal:
    html = f.FormatCOTOverviewMinimal(analyses, convictions)
case domain.OutputFull:
    html = f.FormatCOTOverviewFull(...)
default: // compact
    html = f.FormatCOTOverviewCompact(...)
}
```

**3. `handler_macro_cmd.go`:** sama, tambah case `OutputMinimal`.

---

## Notes

Tidak perlu implement di semua command sekaligus. Fokus COT + Macro dulu karena
paling sering digunakan.
