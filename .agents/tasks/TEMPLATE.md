# Task Specification Template

**ID:** PHI-XXX-000  
**Title:** Brief task description  
**Priority:** high / medium / low  
**Type:** feature / refactor / fix / ux / data / infrastructure  
**Estimated:** S / M / L (S=<2h, M=2-4h, L=4h+)  
**Area:** internal/service | internal/adapter | pkg | docs | config  
**Assignee:** (kosongkan untuk pickup oleh dev)

---

## Deskripsi

Jelaskan apa yang perlu dilakukan dengan jelas dan spesifik.

## Konteks

Mengapa task ini penting? Referensi ke dokumen riset atau issue terkait.

## Acceptance Criteria

- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses  
- [ ] Test coverage untuk code baru (jika applicable)
- [ ] Kriteria spesifik task...

## Files yang Kemungkinan Diubah

- `path/to/file.go`

## Referensi

- `.agents/research/YYYY-MM-DD-topik.md`
- `.agents/UX_AUDIT.md#TASK-XXX` (untuk UX tasks)
- `.agents/TECH_REFACTOR_PLAN.md#TECH-XXX` (untuk refactor tasks)
- `.agents/DATA_SOURCES_AUDIT.md` (untuk data tasks)

---

## Claim Instructions

Untuk meng-claim task ini:
1. Copy file ini ke `.agents/tasks/in-progress/`
2. Update field **Assignee** dengan nama agent (Dev-A/B/C)
3. Update `.agents/STATUS.md` — pindahkan task dari Pending ke In Progress
4. Buat branch: `git checkout -b feat/PHI-XXX-task-name`
5. Implement sesuai acceptance criteria
6. Setelah selesai, move file ke `.agents/tasks/done/` dan update STATUS.md
