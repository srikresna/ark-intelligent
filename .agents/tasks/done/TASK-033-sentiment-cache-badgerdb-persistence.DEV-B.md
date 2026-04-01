# TASK-033: Sentiment Cache BadgerDB Persistence — DONE

**Completed by:** Dev-B
**Completed at:** 2026-04-01 16:55 WIB
**PR:** #81
**Branch:** feat/TASK-033-sentiment-badgerdb-cache

## Summary
Upgraded sentiment cache from pure in-memory to BadgerDB-backed persistence.
Bot restarts no longer force 3 expensive Firecrawl API calls when cached data
is still within its 6-hour TTL.

## Changes
- sentiment/cache.go: Added InitSentimentCache(db), BadgerDB load/persist helpers,
  disk-aware GetCachedOrFetch(), and disk-clearing InvalidateCache().
  Fully backward-compatible (nil DB = in-memory fallback).
- cmd/bot/main.go: Wired sentimentsvc.InitSentimentCache after storage layer init.

## Verification
- go build ./... PASS
- go vet ./... PASS
