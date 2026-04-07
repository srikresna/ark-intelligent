# Research Report: Bug Hunting Siklus 5 Putaran 5
# Callback Nil Chat, Chart Zero-Byte, BadgerDB Deadlock, AI ContentBlocks, TimeWIB
**Date:** 2026-04-02 15:00 WIB
**Siklus:** 5/5 (Bug Hunting) — Putaran 5
**Author:** Research Agent

## Ringkasan

11 bugs ditemukan. 5 paling critical dijadikan task. Fokus: Telegram callback safety, Python subprocess error propagation, BadgerDB deadlock, AI nil pointer, scheduler timing.

## Temuan 1: Callback Handler — Empty chatID When Message is Nil

**File:** `internal/adapter/telegram/bot.go:345-392`

Saat `cb.Message` nil (legitimate case: inline keyboard tanpa message context), `chatID` jadi empty string "". Handlers proceed tanpa validation → Telegram API reject "chat_id not provided". User sees no response.

```go
chatID := ""
if cb.Message != nil {
    chatID = strconv.FormatInt(cb.Message.Chat.ID, 10)
}
// chatID="" passed to all callback handlers without check
```

**Impact:** Silent callback failures. User presses button, nothing happens.
**Severity:** HIGH

## Temuan 2: Chart Generation — Zero-Byte + mplfinance Error Propagation

**Files:** `handler_cta.go:716-757`, `scripts/cta_chart.py:23`

Two related issues:
1. If `mplfinance` not installed, Python exits code 1. Go handler gets generic error, real error in stderr (not captured). Note: TASK-168 covers stderr capture but not the specific mplfinance validation.
2. If Python succeeds but produces 0-byte PNG, `os.ReadFile()` returns empty bytes, sent to Telegram as broken image. No size validation.

**Impact:** Broken charts sent to users. Impossible to debug without stderr.
**Severity:** HIGH

## Temuan 3: BadgerDB DropAll() — No Timeout, Deadlock Risk

**File:** `internal/adapter/storage/badger.go:90-96`

`DropAll()` blocks writers and waits for all readers. No context timeout. If a long-running read transaction (scheduler analysis, price aggregation) holds a read lock, `DropAll()` blocks indefinitely.

**Impact:** Bot hangs if admin triggers data clear during active analysis.
**Severity:** MEDIUM

## Temuan 4: AI ContentBlocks Nil Pointer

**File:** `internal/service/ai/chat_service.go:82-94`

Iterasi `contentBlocks` tanpa nil check pada individual blocks. Jika block nil → panic accessing `b.Type`. Also `describeContentBlocks()` at line 320 has same issue.

**Impact:** Bot panic saat AI returns unexpected nil content block.
**Severity:** HIGH

## Temuan 5: News Scheduler — TimeWIB Zero-Value Not Checked

**File:** `internal/service/news/scheduler.go:435, 541`

`e.TimeWIB.Sub(now)` computed without checking `e.TimeWIB.IsZero()`. Zero-valued time.Time produces wildly incorrect duration → event either never fires or fires immediately.

**Impact:** Missed or premature event alerts.
**Severity:** MEDIUM
