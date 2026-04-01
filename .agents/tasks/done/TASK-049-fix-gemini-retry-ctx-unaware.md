# TASK-049: Fix Gemini retry sleep tidak menghormati context

**Priority:** low
**Type:** fix
**Estimated:** S
**Area:** internal/service/ai
**Created by:** Research Agent
**Created at:** 2026-04-01 17:30 WIB
**Siklus:** BugHunt-B

## Deskripsi
Di `internal/service/ai/gemini.go`, dua fungsi (`Generate` baris 74 dan `GenerateWithSystem` baris 118) menggunakan `time.Sleep` untuk retry backoff tanpa mengecek `ctx.Done()`:

```go
time.Sleep(backoff)          // Generate
time.Sleep(time.Duration(attempt*attempt) * time.Second)  // GenerateWithSystem
```

Bandingkan dengan `claude.go` yang sudah benar:
```go
select {
case <-time.After(backoff):
case <-ctx.Done():
    return "", ctx.Err()
}
```

Jika context di-cancel saat sleeping (user request timeout, bot shutdown), sleep tetap berjalan penuh.

Ref: `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B5)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `time.Sleep(backoff)` di `Generate` diganti dengan select + ctx.Done()
- [ ] `time.Sleep(...)` di `GenerateWithSystem` diganti dengan select + ctx.Done()
- [ ] Return `ctx.Err()` jika context di-cancel saat sleep

## File yang Kemungkinan Diubah
- `internal/service/ai/gemini.go`

## Referensi
- `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B5)
- `internal/service/ai/claude.go:272-278` — contoh implementasi yang benar
