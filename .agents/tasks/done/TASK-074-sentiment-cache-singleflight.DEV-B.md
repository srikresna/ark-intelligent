# TASK-074: Fix TOCTOU Race di Sentiment Cache — DONE

**Completed by:** Dev-B
**Completed at:** 2026-04-01 15:55 WIB
**Branch:** feat/TASK-074-sentiment-singleflight

## Changes
- internal/service/sentiment/cache.go: replaced Mutex unlock-before-fetch pattern
  with golang.org/x/sync/singleflight to coalesce concurrent cache-miss fetches
- Reverted to RWMutex for cache reads (better concurrency)
- go.mod: promoted golang.org/x/sync to direct dependency

## Verification
- go build ./... ✅
- go vet ./... ✅
- go test ./... ✅ (all pass)
- Concurrent cache-miss callers now coalesced: only 1 FetchSentiment() in-flight
