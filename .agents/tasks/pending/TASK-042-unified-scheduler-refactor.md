# TASK-042: Unify Dual Scheduler — DRY Panic Recovery & Job Interface

**Type:** refactor  
**Priority:** MEDIUM  
**Effort:** L (4-6h)  
**Phase:** Tech Refactor Phase 3 (TECH-005)  
**Assignee:** unassigned

---

## Problem

Ada dua scheduler yang bertanggung jawab berbeda tapi pattern-nya redundan:
- `internal/scheduler/scheduler.go` — 1,112 LOC (general jobs: COT, price, FRED, sentiment)
- `internal/service/news/scheduler.go` — 1,099 LOC (news jobs: sync, reminder, impact, alert)

Masalah spesifik:
1. Pattern `recover()` diulang 5x di news/scheduler.go dengan kode identik
2. `time.Sleep(50ms)` untuk Telegram flood control ada di 8 lokasi berbeda
3. Job goroutines tidak respek `ctx.Done()` — tidak bisa di-cancel bersih
4. Tidak ada interface `Job` yang unified — dua scheduler punya struct berbeda

## Solution

### Phase A: DRY Panic Recovery (quick win, bisa dikerjakan duluan)
```go
// pkg/saferun/saferun.go — wrapper goroutine dengan panic recovery
func Go(ctx context.Context, name string, logger zerolog.Logger, fn func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                logger.Error().Interface("panic", r).Str("goroutine", name).Msg("PANIC recovered")
            }
        }()
        fn()
    }()
}
```

### Phase B: Flood Control Constant
```go
// internal/config/constants.go (lihat TASK-018)
const TelegramFloodDelay = 50 * time.Millisecond
```

### Phase C: Context-aware loops (jangka panjang)
```go
// Ganti time.Sleep(50ms) dengan:
select {
case <-time.After(TelegramFloodDelay):
case <-ctx.Done():
    return
}
```

## Acceptance Criteria
- [ ] `pkg/saferun/` package dibuat dengan Go() helper
- [ ] news/scheduler.go 5x recover() blocks diganti `saferun.Go()`
- [ ] `TelegramFloodDelay` constant dipakai konsisten
- [ ] `go build ./...` sukses
- [ ] Job behavior identik

## Notes
- JANGAN merge dua scheduler jadi satu file — itu terlalu besar refactor, bisa jadi future work
- Fokus pada DRY-ing pattern yang redundan
- Phase C (ctx-aware) bisa jadi subtask terpisah jika waktunya tidak cukup
- Cek apakah TASK-018 (magic-numbers-constants) sudah selesai — kalau sudah, pakai constants dari sana
