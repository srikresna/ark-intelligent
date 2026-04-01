# TASK-021: Hapus dead code cmd pertama di runQuantEngine

**Priority:** low
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 14:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `internal/adapter/telegram/handler_quant.go`, fungsi `runQuantEngine` membuat `cmd` dua kali:
1. Pertama dengan `context.Background()` (tanpa timeout) — lalu langsung ditimpa
2. Kedua dengan `cmdCtx` yang memiliki timeout 60 detik

Block pertama adalah dead code yang menyesatkan — `cmd` diset tapi tidak pernah dipakai sebelum di-overwrite.

## Konteks
Dead code ini bisa membingungkan developer saat maintenance dan menyembunyikan intent bahwa setiap eksekusi harus punya timeout.

Ref: `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A2)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Block pertama `cmd := exec.CommandContext(context.Background(), ...)` dan `cmd.Stderr = os.Stderr` dihapus
- [ ] Hanya ada satu `cmd` yang dibuat, menggunakan `cmdCtx` dengan timeout 60 detik

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_quant.go`

## Referensi
- `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A2)
