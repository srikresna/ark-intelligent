# Research Report — Initial Audit & Task Creation
**Date:** 2026-04-04  
**Researcher:** Agent Research  
**Focus:** Initial codebase audit, workflow setup, task identification

---

## Ringkasan Temuan

### 1. Struktur Proyek
- **Proyek:** ARK Intelligent — Telegram bot analisis pasar finansial
- **Bahasa:** Go 1.22
- **Arsitektur:** Clean architecture dengan domain-driven design
- **Storage:** BadgerDB (embedded KV)
- **Services:** 30+ service modules (COT, FRED, TA, AI, etc.)

### 2. Status Workflow
- **Branch integrasi:** `agents/main` (sudah ada)
- **Queue task:** Kosong — belum ada task pending
- **Semua agent:** Idle (perlu inisialisasi task)
- **Blocker:** Tidak ada

### 3. Temuan Gap & Peluang

#### A. Workflow Infrastructure (HIGH PRIORITY)
**Gap:** Tidak ada task queue/ledger yang terstruktur  
**Impact:** Dev agents tidak bisa mengambil task  
**Solusi:** Buat sistem task ledger sederhana

#### B. Data Sources Integration (MEDIUM PRIORITY)  
**Gap:** AAII Sentiment dan Fear & Greed Index belum diimplement  
**Impact:** Missing sentiment indicators (Firecrawl sudah ada, tinggal pakai)  
**Referensi:** `.agents/DATA_SOURCES_AUDIT.md`

#### C. UX Consistency (MEDIUM PRIORITY)
**Gap:** Navigation button tidak konsisten (campur ID/EN)  
**Impact:** User experience fragmented  
**Referensi:** `.agents/UX_AUDIT.md`

#### D. Command Aliases (LOW PRIORITY)
**Gap:** Tidak ada command shortcuts (`/c` untuk `/cot`, dll)  
**Impact:** UX kurang optimal untuk power users

### 4. Risiko Teknis Teridentifikasi

| Risiko | Level | Area |
|--------|-------|------|
| Float precision di perhitungan moneter | Medium | `pkg/fmtutil/` |
| Handler files sangat besar (>500 LOC) | Medium | `internal/adapter/telegram/` |
| Missing test coverage di beberapa service | Low | `internal/service/*` |

---

## Task yang Dibuat

### Task 1: Setup Task Ledger System
**ID:** PHI-SETUP-001  
**Priority:** HIGH  
**Type:** infrastructure  
**Assignee:** Dev-A  

**Deskripsi:**  
Buat sistem task ledger sederhana agar coordinator dan dev agents bisa track task.

**Acceptance Criteria:**
- [ ] Buat folder `.agents/tasks/` dengan subfolder: `pending/`, `in-progress/`, `done/`
- [ ] Buat template task spec di `.agents/tasks/TEMPLATE.md`
- [ ] Buat 3 task sample di `pending/` sebagai contoh
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

**Files:**
- `.agents/tasks/TEMPLATE.md` (baru)
- `.agents/tasks/pending/*.md` (baru)
- `.agents/tasks/in-progress/.gitkeep` (baru)
- `.agents/tasks/done/.gitkeep` (baru)

---

### Task 2: Implement AAII Sentiment via Firecrawl
**ID:** PHI-DATA-001  
**Priority:** MEDIUM  
**Type:** feature  
**Assignee:** Dev-B  

**Deskripsi:**  
Tambah AAII Investor Sentiment sebagai data source. Firecrawl sudah tersedia di `.env`.

**Acceptance Criteria:**
- [ ] Buat `internal/service/sentiment/aaii.go`
- [ ] Implement fetcher via Firecrawl ke `aaii.com/sentimentsurvey/sent_results`
- [ ] Parsing: Bullish, Neutral, Bearish percentages
- [ ] Caching: 24 jam TTL di BadgerDB
- [ ] Tambah test: `aaii_test.go`
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

**Files:**
- `internal/service/sentiment/aaii.go` (baru)
- `internal/service/sentiment/aaii_test.go` (baru)

**Referensi:** `.agents/DATA_SOURCES_AUDIT.md`

---

### Task 3: Implement Fear & Greed Index
**ID:** PHI-DATA-002  
**Priority:** MEDIUM  
**Type:** feature  
**Assignee:** Dev-C  

**Deskripsi:**  
Tambah CNN Fear & Greed Index scraper via Firecrawl.

**Acceptance Criteria:**
- [ ] Buat `internal/service/sentiment/fear_greed.go`
- [ ] Implement fetcher via Firecrawl ke `money.cnn.com/data/fear-and-greed`
- [ ] Parsing: Index value (0-100) + classification
- [ ] Caching: 4 jam TTL (update harian)
- [ ] Tambah test: `fear_greed_test.go`
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

**Files:**
- `internal/service/sentiment/fear_greed.go` (baru)
- `internal/service/sentiment/fear_greed_test.go` (baru)

**Referensi:** `.agents/DATA_SOURCES_AUDIT.md`

---

### Task 4: Standardize Navigation Buttons
**ID:** PHI-UX-001  
**Priority:** MEDIUM  
**Type:** ux  
**Assignee:** Dev-A  

**Deskripsi:**  
Standardisasi semua navigation button ke Bahasa Indonesia dengan fallback English.

**Acceptance Criteria:**
- [ ] Audit semua button text di `internal/adapter/telegram/`
- [ ] Standardisasi format: `[icon] Label` (konsisten)
- [ ] Semua "Back" button → `<< Kembali`
- [ ] Semua "Home" button → `🏠 Beranda`
- [ ] Update `KeyboardBuilder` methods
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

**Files:**
- `internal/adapter/telegram/keyboard.go` (modifikasi)
- Berbagai `handler_*.go` (modifikasi minor)

**Referensi:** `.agents/UX_AUDIT.md`

---

### Task 5: Add Command Aliases
**ID:** PHI-UX-002  
**Priority:** LOW  
**Type:** ux  
**Assignee:** Dev-B  

**Deskripsi:**  
Tambah command aliases untuk commands yang sering dipakai.

**Acceptance Criteria:**
- [ ] `/c` → `/cot`
- [ ] `/m` → `/macro`
- [ ] `/cal` → `/calendar`
- [ ] `/out` → `/outlook`
- [ ] `/q` → `/quant`
- [ ] Update command router di `handler.go`
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

**Files:**
- `internal/adapter/telegram/handler.go` (modifikasi)
- `internal/adapter/telegram/command_router.go` (jika belum ada, buat baru)

**Referensi:** `.agents/UX_AUDIT.md`

---

## Risk & Dependency Notes

1. **Task Ledger (PHI-SETUP-001)** adalah blocker untuk task lain — harus dikerjakan pertama
2. **Firecrawl API** sudah tersedia di `.env` — tidak perlu setup baru
3. **Navigation standardization** mungkin conflict dengan PR yang sedang pending
4. **Command aliases** butuh testing manual karena tidak bisa unit test mudah

---

## Rekomendasi Prioritas

```
1. PHI-SETUP-001 (HIGH)   → Setup infrastructure task ledger
2. PHI-DATA-001 (MEDIUM) → AAII Sentiment
3. PHI-DATA-002 (MEDIUM) → Fear & Greed Index  
4. PHI-UX-001 (MEDIUM)   → Navigation standardization
5. PHI-UX-002 (LOW)      → Command aliases
```

---

## Next Research Cycle

Setelah task di atas selesai, research cycle berikutnya akan fokus pada:
- **Siklus UX:** Deep dive UX audit untuk improvement navigasi
- **Siklus Data:** Integrasi World Bank & IMF data (macro global)
- **Siklus Refactor:** Handler decomposition (file terlalu besar)
- **Siklus BugHunt:** Edge case testing

---

*Report generated by Agent Research — 2026-04-04*
