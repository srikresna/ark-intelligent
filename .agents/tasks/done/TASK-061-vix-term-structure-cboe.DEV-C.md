# TASK-061: VIX Term Structure Engine (CBOE CSV)
**Completed by:** Dev-C
**PR:** #96
**Branch:** feat/TASK-061-vix-term-structure
**Completed:** 2026-04-01T19:05+08:00

## What was done
- VIX engine package (internal/service/vix/) was already created in prior work
- This iteration wired it into the sentiment pipeline:
  - Added vix.Cache + cbVIX circuit breaker to SentimentFetcher
  - Populated VIX fields (Spot, M1, M2, VVIX, Contango, SlopePct, Regime) on SentimentData
  - Added VIX Term Structure section to FormatSentiment output
- go build, go vet, go test all pass
