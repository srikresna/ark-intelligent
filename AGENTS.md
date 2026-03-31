# AGENTS.md — Konstitusi Multi-Agent ark-intelligent

> Semua agent WAJIB membaca dan mengikuti dokumen ini sebelum melakukan apapun.

---

## Struktur Tim

| Agent | Branch | Role |
|---|---|---|
| **Research** | `agents/research` | Riset rotating focus, buat task spec, kirim laporan ke Telegram |
| **Dev-A** | `agents/dev-a` | Implementasi + Senior Reviewer (merge PR Dev-B & Dev-C) |
| **Dev-B** | `agents/dev-b` | Implementasi task dari queue |
| **Dev-C** | `agents/dev-c` | Implementasi task dari queue |

## Research: Rotating Focus

Research Agent TIDAK mengerjakan semua sekaligus. Setiap siklus punya **satu fokus**:

| Siklus | Fokus | Output |
|---|---|---|
| 1 | UX/UI improvement | 3-5 task UX |
| 2 | Data & integrasi baru (gratis) | 3-5 task data |
| 3 | Fitur baru (ICT, SMC, Quant, dll) | 3-5 task fitur |
| 4 | Bug hunting & tech debt | 3-5 task fix/refactor |
| 5 | Review & optimasi yang sudah ada | 3-5 task improvement |
| → rotate kembali ke siklus 1 | | |

Referensi riset: baca `.agents/FEATURE_INDEX.md`, `.agents/UX_AUDIT.md`, `.agents/DATA_SOURCES_AUDIT.md`

## Dev-A: Senior Reviewer

Dev-A mengerjakan task seperti Dev-B dan Dev-C, **ditambah**:
- Review PR dari Dev-B dan Dev-C setelah selesai implement task
- Merge PR ke `agents/main` kalau build clean dan logic benar
- Kalau ada issue → comment di PR + buat task fix di `pending/` dengan tag `[BLOCKING]`
- Dev-A TIDAK review PR-nya sendiri

## Hierarki Branch

```
main                  ← HANYA owner yang merge ke sini
└── agents/main       ← branch integrasi semua agent
    ├── agents/research
    ├── agents/dev-a
    ├── agents/dev-b
    └── agents/dev-c
```

**ATURAN KERAS:**
- ❌ Tidak ada yang push langsung ke `main`
- ❌ Tidak ada yang merge ke `main` — itu hak owner
- ✅ Semua PR diarahkan ke `agents/main`
- ✅ Sebelum kerja, selalu `git pull origin agents/main`

---

## Workflow Task Queue

### Research Agent
1. `git pull origin agents/main`
2. Analisis codebase + merged PR terbaru
3. Tulis task spec ke `.agents/tasks/pending/TASK-XXX-nama.md`
4. Tulis hasil riset ke `.agents/research/YYYY-MM-DD-topik.md`
5. Commit + push ke `agents/research`
6. Kirim ringkasan ke Telegram owner (chat_id: **1476273971**)
7. Update `.agents/STATUS.md`
8. Tunggu interval berikutnya → ulangi

### Dev Agent (A / B / C)
1. `git pull origin agents/main`
2. Cek `.agents/tasks/pending/` — ambil 1 task
3. "Claim" task: pindah file ke `.agents/tasks/claimed/TASK-XXX-nama.DEV-X.md`
4. Commit claim ke branch agent sendiri
5. Buat feature branch: `git checkout -b feat/TASK-XXX-nama` dari `agents/main`
6. Implement
7. Commit + push feature branch
8. Buat PR ke `agents/main`
9. Pindah task ke `.agents/tasks/done/`
10. Ambil task berikutnya → ulangi

---

## Format Task Spec

File: `.agents/tasks/pending/TASK-XXX-nama-singkat.md`

```markdown
# TASK-XXX: Nama Task

**Priority:** high / medium / low
**Estimated:** S / M / L (Small <2h, Medium 2-4h, Large 4h+)
**Area:** internal/service | internal/adapter | pkg | docs
**Created by:** Research Agent
**Created at:** YYYY-MM-DD HH:MM

## Deskripsi
[Apa yang perlu dilakukan]

## Konteks
[Mengapa ini penting, hasil riset terkait]

## Acceptance Criteria
- [ ] ...
- [ ] ...

## File yang Kemungkinan Diubah
- `path/to/file.go`

## Referensi
- [link atau nama file riset terkait]
```

---

## Format Laporan Research ke Telegram

Setiap selesai siklus riset, Research agent kirim ke Telegram:

```
🔬 [RESEARCH REPORT]

📌 Topik: <nama topik>
🕐 Waktu: <timestamp WIB>

📊 Temuan:
<ringkasan 3-5 poin temuan utama>

📋 Task Dibuat:
- TASK-XXX: <nama task> [priority]
- TASK-YYY: <nama task> [priority]

🔗 Detail: .agents/research/YYYY-MM-DD-topik.md
```

---

## Git Identity per Agent

Setiap agent HARUS set git identity sebelum commit:

| Agent | name | email |
|---|---|---|
| Research | `Agent Research` | `research@ark-intelligent.ai` |
| Dev-A | `Agent Dev-A` | `dev-a@ark-intelligent.ai` |
| Dev-B | `Agent Dev-B` | `dev-b@ark-intelligent.ai` |
| Dev-C | `Agent Dev-C` | `dev-c@ark-intelligent.ai` |

```bash
git config user.name "Agent Research"
git config user.email "research@ark-intelligent.ai"
```

---

## Aturan Commit

```
feat(TASK-XXX): deskripsi singkat
fix(TASK-XXX): deskripsi singkat
research: topik yang diriset
chore: hal-hal maintenance
```

---

## Konflik & Collision Prevention

- Satu task hanya boleh diklaim oleh satu agent
- Claim dilakukan dengan atomic file rename (pindah ke `claimed/` + tambahkan `.DEV-X`)
- Kalau dua agent claim task yang sama → yang duluan commit claim menang, yang lain batalkan
- Jangan edit file yang sama di branch berbeda tanpa koordinasi di `.agents/STATUS.md`

---

## STATUS.md

Setiap agent update `.agents/STATUS.md` setelah setiap aksi:

```markdown
# Agent Status

## Research
- **Last run:** YYYY-MM-DD HH:MM WIB
- **Current:** idle / researching <topik>
- **Tasks created today:** N

## Dev-A
- **Last run:** YYYY-MM-DD HH:MM WIB  
- **Current:** idle / working on TASK-XXX
- **PRs today:** N

## Dev-B
- **Last run:** YYYY-MM-DD HH:MM WIB
- **Current:** idle / working on TASK-XXX
- **PRs today:** N

## Dev-C
- **Last run:** YYYY-MM-DD HH:MM WIB
- **Current:** idle / working on TASK-XXX
- **PRs today:** N
```
