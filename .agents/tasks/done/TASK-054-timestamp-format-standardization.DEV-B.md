# TASK-054: Timestamp Format Standardization

**Status:** done
**Agent:** Dev-B
**Completed:** 2026-04-01
**PR:** #76

## Changes
- Replaced 17 inline time.Format() calls in formatter.go with centralized fmtutil helpers
- Added WIB() and FormatDateShortWIB() exports to pkg/fmtutil
- All macro/sentiment/composites timestamps use FormatDateTimeWIB()
- Ranking headers use FormatDateWIB()
- Walk-forward windows and weekly report use FormatDateShortWIB()
- Calendar event times use UpdatedAtShort()

## Files Changed
- internal/adapter/telegram/formatter.go (17 replacements)
- pkg/fmtutil/format.go (2 new exports)

## Verification
- go build ./... clean
- go vet ./... clean
- go test ./pkg/fmtutil/... all pass
- Zero behavior change
