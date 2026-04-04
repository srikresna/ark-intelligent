# PHI-SETUP-001: Setup Task Ledger System

**ID:** PHI-SETUP-001  
**Title:** Setup Task Ledger System  
**Priority:** HIGH  
**Type:** infrastructure  
**Estimated:** S (<2h)  
**Area:** docs  
**Assignee:** 

---

## Deskripsi

Buat sistem task ledger sederhana agar coordinator dan dev agents bisa track task secara terstruktur. Task ini adalah fondasi untuk workflow multi-agent yang efektif.

## Konteks

Saat ini tidak ada struktur task queue yang jelas. Dev agents tidak bisa melihat task pending atau mengambil task. Task ledger akan menjadi single source of truth untuk semua task dalam workflow.

## Acceptance Criteria

- [ ] Folder structure `.agents/tasks/` sudah ada dengan subfolder:
  - `pending/` — task yang siap dikerjakan
  - `in-progress/` — task yang sedang dikerjakan
  - `done/` — task yang sudah selesai
- [ ] Template task spec di `.agents/tasks/TEMPLATE.md` sudah ada dan lengkap
- [ ] Minimal 3 contoh task di `pending/` yang mengikuti template
- [ ] File ini (PHI-SETUP-001) sudah di-move ke `in-progress/` saat di-claim
- [ ] `go build ./...` clean (tidak ada code change tapi cek build)
- [ ] `go vet ./...` clean

## Files yang Akan Dibuat/Diubah

- `.agents/tasks/TEMPLATE.md` (baru)
- `.agents/tasks/pending/PHI-DATA-001.md` (baru)
- `.agents/tasks/pending/PHI-DATA-002.md` (baru)
- `.agents/tasks/pending/PHI-UX-001.md` (baru)
- `.agents/STATUS.md` (update status task)

## Referensi

- `.agents/research/2026-04-04-initial-audit.md`
- `.agents/ORCHESTRATION_GUIDE.md`

---

## Claim Instructions

**Task ini HARUS dikerjakan pertama sebelum task lain.**

1. Copy file ini ke `.agents/tasks/in-progress/PHI-SETUP-001.md`
2. Update field **Assignee** dengan `Dev-A`
3. Update `.agents/STATUS.md` — pindahkan PHI-SETUP-001 dari Pending ke In Progress
4. Implement acceptance criteria di atas
5. Setelah selesai, move ke `.agents/tasks/done/` dan update STATUS.md
