# TASK-117: Context-Aware Retry Sleep di Gemini Client

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/ai
**Created by:** Research Agent
**Created at:** 2026-04-01 23:00 WIB
**Siklus:** Refactor

## Deskripsi
Retry logic di gemini.go menggunakan `time.Sleep(backoff)` yang BLOCKING dan mengabaikan context cancellation. Jika caller cancel context (user timeout, shutdown), function tetap block selama full sleep duration. Replace dengan context-aware sleep pattern.

## Konteks
- `gemini.go:76` — `time.Sleep(backoff)` di retry loop
- `gemini.go:120` — `time.Sleep(time.Duration(attempt*attempt) * time.Second)` di retry loop
- TASK-049 sudah fix retry awareness tapi hanya partial — sleep masih blocking
- Ref: `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Replace semua `time.Sleep()` di gemini.go retry loops dengan context-aware pattern:
  ```go
  select {
  case <-time.After(backoff):
      // continue retry
  case <-ctx.Done():
      return ctx.Err()
  }
  ```
- [ ] Audit service/ai/ lainnya (claude client) untuk pattern yang sama
- [ ] Tidak ada behavior change saat context TIDAK cancelled (retry tetap bekerja normal)
- [ ] Saat context cancelled: return immediately dengan ctx.Err()

## File yang Kemungkinan Diubah
- `internal/service/ai/gemini.go`
- `internal/service/ai/claude.go` (audit, mungkin sama)

## Referensi
- `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`
- TASK-049 (related gemini retry fix)
