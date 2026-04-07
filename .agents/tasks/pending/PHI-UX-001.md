# PHI-UX-001: Standardize Navigation Buttons

**ID:** PHI-UX-001  
**Title:** Standardize Navigation Buttons  
**Priority:** MEDIUM  
**Type:** ux  
**Estimated:** S (<2h)  
**Area:** internal/adapter  
**Assignee:** 

---

## Deskripsi

Standardisasi semua navigation button ke Bahasa Indonesia dengan format yang konsisten. Saat ini ada mix bahasa dan format yang membuat UX fragmented.

## Konteks

Dari UX audit, ditemukan inconsistency:
- Beberapa pakai `<< Kembali ke Ringkasan`
- Beberapa pakai `<< Back to Overview`  
- Beberapa pakai `<< Kembali ke Dashboard`
- Mix bahasa Indonesia dan Inggris

Target: Semua navigation button konsisten dalam Bahasa Indonesia dengan format `[icon] Label`.

## Acceptance Criteria

- [ ] Audit semua button text di `internal/adapter/telegram/`
- [ ] Standardisasi format: `[icon] Label` (konsisten di semua button)
- [ ] Semua "Back" button → `<< Kembali` atau `<< Kembali ke [Context]`
- [ ] Semua "Home" button → `🏠 Beranda`
- [ ] Semua "Overview" button → `📊 Ringkasan`
- [ ] Semua "Settings" button → `⚙️ Pengaturan`
- [ ] Update `KeyboardBuilder` methods untuk consistency
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

## Standard Button Mapping

| Fungsi | Icon + Label |
|--------|-------------|
| Home/Beranda | `🏠 Beranda` |
| Back/Kembali | `<< Kembali` |
| Overview | `📊 Ringkasan` |
| Settings | `⚙️ Pengaturan` |
| Refresh | `🔄 Refresh` |
| Close | `❌ Tutup` |
| Expand | `📖 Selengkapnya` |
| Compact | `📋 Ringkas` |

## Files yang Akan Diubah

- `internal/adapter/telegram/keyboard.go` — update KeyboardBuilder methods
- `internal/adapter/telegram/handler_*.go` — update button text (cari pattern `kb.Add`)

## Referensi

- `.agents/UX_AUDIT.md` — bagian "Navigation Tidak Konsisten"
- `.agents/UX_AUDIT.md` — bagian "Standardize Language"

---

## Claim Instructions

1. Pastikan PHI-SETUP-001 sudah selesai
2. Copy file ini ke `.agents/tasks/in-progress/PHI-UX-001.md`
3. Update field **Assignee** dengan `Dev-A`
4. Update `.agents/STATUS.md`
5. Buat branch: `git checkout -b ux/PHI-001-nav-standardization`
6. Implement dan test
7. Setelah selesai, move ke `done/` dan update STATUS.md
