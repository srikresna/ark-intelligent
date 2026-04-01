# 🆕 Dev-D Agent Onboarding — ark-intelligent

## Identitas
- **Role:** Developer (Implementor)
- **Git Name:** `Agent Dev-D`
- **Git Email:** `dev-d@ark-intelligent.ai`
- **Branch:** `agents/dev-d`

---

## Setup Awal (jalankan SEKALI)

```bash
cd ~
git clone https://$GH_TOKEN@github.com/arkcode369/ark-intelligent.git
cd ark-intelligent
git config user.name "Agent Dev-D"
git config user.email "dev-d@ark-intelligent.ai"
git checkout agents/main
git checkout -b agents/dev-d
git push origin agents/dev-d
```

---

## Proyek
**ark-intelligent** — Bot Telegram institusional untuk trader forex/komoditas.
- Bahasa: Go (1.22+), Clean architecture, BadgerDB, Python chart scripts
- Build check: `go build ./... && go vet ./...`

---

## Task Assignment

### Bug Fix / Defensive (20) — PRIORITAS UTAMA
Batch 3: TASK-165, 166, 167, 168, 169, 170, 171, 172, 173, 174
Batch 4: TASK-195, 196, 197, 198, 199
Overflow: TASK-140, 141, 142, 143, 144

### UX / UI (14)
TASK-150, 151, 152, 153, 154, 175, 176, 177, 178, 179, 200, 201, 202, 203

### Test Coverage (6)
TASK-084, 088, 089, 092, 097, 194

---

## Workflow

1. `git checkout agents/main && git pull origin agents/main`
2. Pick task dari pending/ sesuai daftar di atas
3. Claim: `cp pending/TASK-XXX.md claimed/TASK-XXX.DEV-D.md && rm pending/TASK-XXX.md`
4. `git checkout agents/main && git checkout -b feat/TASK-XXX-nama`
5. Implement → `go build ./... && go vet ./...`
6. `git commit && git push && gh pr create --base agents/main`
7. Move to done via `agents/dev-d` branch

## Aturan
- ❌ JANGAN push ke `main`, JANGAN review/merge PR
- ✅ `go build + go vet` WAJIB pass, satu task = satu PR
