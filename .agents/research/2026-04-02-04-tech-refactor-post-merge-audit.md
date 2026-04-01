# Research Report: Tech Refactor Siklus 4 Putaran 3 — Post-Merge Quality Audit

**Tanggal:** 2026-04-02 04:00 WIB
**Fokus:** Technical Refactor & Tech Debt (Siklus 4, Putaran 3)
**Siklus:** 4/5

---

## Ringkasan

Audit kualitas kode setelah merge TASK-041 (bot wiring split), TASK-060 (COT z-score), TASK-061 (VIX term structure). Ditemukan 5 issues baru: function pointer misuse di api.go, error shadowing di VIX parser, error masking di VIX cache, silenced error returns di bot.go, dan interface{} modernization gap.

---

## Temuan

### 1. CRITICAL: fmt.Sprintf Function Pointer Misuse (api.go:254)
- Code assigns `fmt.Sprintf` (function pointer) to variable, then calls it
- Works by accident tapi semantically broken
- Could confuse future developers dan IDE linting

### 2. MEDIUM: Error Shadowing di VIX CSV Parser (vix/fetcher.go:87, 148)
- Uses `err2` instead of `err` — non-standard
- CSV read loop breaks on ANY error tanpa distinguish EOF vs parse error
- Malformed data silently ignored

### 3. MEDIUM: VIX Cache Error Masking (vix/cache.go:36-42)
- Cache returns `nil` error meskipun FetchTermStructure fails
- Caller tidak bisa distinguish network timeout vs invalid data vs no data
- Breaks error handling contract

### 4. MEDIUM: Silenced Error Returns di bot.go (300, 312, 318, 325)
- `_, _ = b.SendHTML(...)` di critical error paths
- Jika send error message gagal, user dan log keduanya buta
- Terutama masalah di rate limit scenarios

### 5. LOW: interface{} Modernization Gap (api.go:212)
- Satu occurrence `interface{}` sisa di api.go, rest already `any`
- Missed during TASK-044 modernization

---

## Task Recommendations

1. **TASK-140**: Fix fmt.Sprintf multipart builder in api.go [HIGH]
2. **TASK-141**: VIX fetcher error handling — EOF vs parse errors [MEDIUM]
3. **TASK-142**: VIX cache error propagation fix [MEDIUM]
4. **TASK-143**: Log silenced error returns in bot.go critical paths [MEDIUM]
5. **TASK-144**: Complete interface{} → any modernization cleanup [LOW]
