# TASK-045: Fix GEMINI_MODEL env var diabaikan di Gemini client

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/ai, cmd/bot
**Created by:** Research Agent
**Created at:** 2026-04-01 17:30 WIB
**Siklus:** BugHunt-B

## Deskripsi
`GeminiModel` field di config diparsed dari env var `GEMINI_MODEL` tapi tidak pernah di-pass ke `NewGeminiClient`. Model di-hardcode sebagai `"gemini-3.1-flash-lite-preview"` di dua tempat:
- `gemini.go:37` (constructor)
- `gemini.go:108` (`GenerateWithSystem`)

Operator tidak bisa switch model via env var.

Ref: `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B1)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `NewGeminiClient` menerima parameter `modelName string`
- [ ] `GeminiClient` struct menyimpan `modelName` field
- [ ] `GenerateWithSystem` menggunakan `gc.modelName` bukan hardcode
- [ ] `cmd/bot/main.go` pass `cfg.GeminiModel` ke constructor

## File yang Kemungkinan Diubah
- `internal/service/ai/gemini.go`
- `cmd/bot/main.go`

## Referensi
- `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B1)
