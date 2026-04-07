# TASK-094: DI Framework Evaluation — Wire vs Fx vs Manual Wiring (TECH-012)

**Priority:** LOW
**Type:** Research / Architecture Decision
**Ref:** TECH-012 in TECH_REFACTOR_PLAN.md
**Branch target:** N/A (research task, output: ADR document)
**Estimated size:** Research (R) — no code, output dokumen
**Created by:** Research Agent
**Created at:** 2026-04-01 15:30 WIB
**Siklus:** 4 — Technical Refactor

---

## Problem

`internal/adapter/telegram/bot.go` (1,289 LOC) berisi manual dependency wiring yang verbose
dan mudah error. TECH-012 menyebut `google/wire` atau `uber-go/fx` sebagai opsi.

Sebelum commitment ke salah satu framework, perlu evaluasi trade-off yang jelas
karena ini akan mempengaruhi seluruh codebase dan semua dev agents.

---

## Output yang Diharapkan

Buat Architecture Decision Record (ADR) di `.agents/research/2026-04-01-adr-di-framework.md`
dengan evaluasi:

### Option A: google/wire (code generation)
- Pros: type-safe, zero runtime overhead, error di compile time
- Cons: butuh code generation step, build complexity bertambah, belajar DSL baru
- Fit dengan project ini: ???

### Option B: uber-go/fx (runtime DI)
- Pros: flexible, lifecycle management (OnStart/OnStop), reflection-based
- Cons: runtime errors (bukan compile time), overhead sedikit, magic black box
- Fit dengan project ini: ???

### Option C: Refactor manual wiring tanpa framework
- Pisah bot.go → `wiring.go` + `registry.go` (sesuai TECH-004 TASK-041)
- Tetap manual tapi lebih terstruktur
- Pros: zero dependencies, transparent, mudah dipahami semua dev
- Cons: masih verbose

### Option D: Constructor injection pattern (pure Go)
- Pakai constructor functions yang sudah ada, buat factory functions
- Pros: idiomatic Go, tidak ada framework
- Cons: masih perlu wiring code

---

## Cara Evaluasi

1. Cek `go.mod` — apakah wire/fx sudah jadi dependency?
2. Lihat `bot.go` secara detail — berapa banyak dependencies yang di-wire?
3. Hitung jumlah services yang perlu di-inject (agar tahu kompleksitas)
4. Pertimbangkan: apakah ukuran project ini justify framework overhead?

---

## Acceptance Criteria

- [ ] Tulis ADR di `.agents/research/2026-04-01-adr-di-framework.md`
- [ ] ADR berisi: Context, Options (A-D), Analysis, Decision/Recommendation, Consequences
- [ ] Recommendation jelas: pilih option mana dan mengapa
- [ ] Jika rekomendasi pakai framework: buat follow-up task untuk implementasi
- [ ] Jika rekomendasi tidak pakai framework: update TECH-012 di TECH_REFACTOR_PLAN.md

---

## Catatan

- Ini RESEARCH task, bukan implementation. Jangan tulis kode.
- ADR harus bisa dibaca oleh Dev-A dan dijadikan dasar keputusan
- Kalau conclusion adalah "tidak perlu framework" — itu valid, update plan accordingly
- Tag `[NEEDS DISCUSSION]` di ADR jika ada trade-off yang tidak jelas
