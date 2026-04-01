# TASK-097: Unit Tests untuk splitMessage & detectUnclosedTags

**Priority:** MEDIUM
**Siklus:** 5 (Bug Hunting)
**Estimasi:** 2 jam

## Problem

`splitMessage()` dan `detectUnclosedTags()` di `internal/adapter/telegram/bot.go` adalah fungsi kritis (semua pesan panjang melewatinya) tetapi tidak punya unit test sama sekali.

Edge cases yang belum dicovered:
1. Tag dengan newline/tab sebelum `>` (e.g., `<pre\n>`)
2. Mismatched closing tags (e.g., `</b>` tanpa opening)
3. Split di tengah multibyte UTF-8 character
4. Pesan yang tepat 4096 karakter (boundary)
5. Nested tags yang dalam (e.g., `<b><i><code>text`)
6. Empty string input
7. Single tag yang tidak pernah ditutup

## Solution

Buat file `internal/adapter/telegram/bot_split_test.go`:

```go
package telegram

import (
    "strings"
    "testing"
)

func TestSplitMessage_ShortMessage(t *testing.T) { ... }
func TestSplitMessage_ExactLimit(t *testing.T) { ... }
func TestSplitMessage_LongMessage(t *testing.T) { ... }
func TestSplitMessage_UnclosedTags(t *testing.T) { ... }
func TestSplitMessage_MultibyteUTF8(t *testing.T) { ... }
func TestDetectUnclosedTags_Empty(t *testing.T) { ... }
func TestDetectUnclosedTags_Balanced(t *testing.T) { ... }
func TestDetectUnclosedTags_Mismatched(t *testing.T) { ... }
```

Jika ditemukan bug saat testing (e.g., UTF-8 split memotong di tengah rune), fix sekalian.

## Acceptance Criteria
- [ ] Coverage ≥ 90% untuk `splitMessage` dan `detectUnclosedTags`
- [ ] Semua 7 edge cases di atas dicovered
- [ ] Jika ada bug ditemukan, fix dalam PR yang sama
- [ ] `go test ./internal/adapter/telegram/... -run TestSplit` pass
- [ ] `go build ./...` clean

## Files to Create/Modify
- `internal/adapter/telegram/bot_split_test.go` (baru)
- `internal/adapter/telegram/bot.go` (jika ada bug yang perlu fix)
