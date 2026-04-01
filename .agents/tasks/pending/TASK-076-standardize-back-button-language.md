# TASK-076: Standardize Back Button Language ke Bahasa Indonesia

**Priority:** medium
**Type:** ux-improvement
**Estimated:** S (1-2 jam)
**Area:** internal/adapter/telegram/keyboard.go
**Created by:** Research Agent
**Created at:** 2026-04-01 19:00 WIB
**Siklus:** UX-1 (Siklus 1 Putaran 3)
**Ref:** UX_AUDIT.md TASK-UX-005

## Deskripsi

Back button di berbagai handler menggunakan campuran Bahasa Indonesia dan Inggris,
serta format yang tidak konsisten (`<<` vs `◀`, "Kembali ke" vs "Back to").

## Masalah Saat Ini

```
keyboard.go L190: "<< Kembali ke Ringkasan"    (macro back)
keyboard.go L206: "<< Ringkasan"               (macro back — BEDA dari L190!)
keyboard.go L443: "<< Back to Overview"        (cot — Inggris!)
keyboard.go L538: "<< Back to Categories"      (impact — Inggris!)
keyboard.go L680: "<< Grid Overview"           (seasonal — tidak ada "back")
keyboard.go L720: "<< Back to Overview"        (cot — Inggris!)
keyboard.go L783: "<< Kembali ke Ringkasan"    (alpha)
keyboard.go L848: "<< Kembali ke Ringkasan"    (cta)
keyboard.go L1006: "<< Kembali ke Dashboard"   (quant)
keyboard.go L1057: "<< Kembali ke Dashboard"   (vp)
```

## Standar yang Diterapkan

Pilih Bahasa Indonesia sebagai standar (sesuai default language "id"):
```
"◀ Ringkasan"     — untuk back to summary/overview (ganti semua varian "back to summary/overview/ringkasan")
"◀ Dashboard"     — untuk back to main dashboard
"◀ Kategori"      — untuk back to categories
"◀ Grid Overview" — untuk seasonal grid (tetap tapi tambah ◀)
```

Ganti `<<` dengan `◀` di semua back buttons untuk konsistensi visual.

## Implementasi

File: `internal/adapter/telegram/keyboard.go`

Cari dan ganti semua:
- `"<< Back to Overview"` → `"◀ Ringkasan"`
- `"<< Back to Categories"` → `"◀ Kategori"`
- `"<< Kembali ke Ringkasan"` → `"◀ Ringkasan"`
- `"<< Kembali ke Dashboard"` → `"◀ Dashboard"`
- `"<< Ringkasan"` → `"◀ Ringkasan"`
- `"<< Grid Overview"` → `"◀ Grid"`

## Acceptance Criteria
- [ ] Semua back buttons menggunakan `◀` (bukan `<<`)
- [ ] Semua teks tombol navigasi dalam Bahasa Indonesia
- [ ] Tidak ada duplikasi teks yang berbeda untuk fungsi yang sama
- [ ] `go build ./...` clean
- [ ] Callback data TIDAK berubah (hanya label yang berubah)
