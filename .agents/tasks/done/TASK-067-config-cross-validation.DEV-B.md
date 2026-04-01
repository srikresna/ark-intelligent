# TASK-067: Config Cross-Validation Expansion (TECH-014)

**Priority:** MEDIUM  
**Type:** Tech Refactor / Reliability  
**Ref:** TECH-014 in TECH_REFACTOR_PLAN.md  
**Branch target:** dev-c  
**Estimated size:** Small (40-60 LOC)

---

## Problem

`config.validate()` di `internal/config/config.go:235` hanya cek 3 field numerik.
Beberapa konfigurasi yang tidak valid lolos tanpa warning jelas:

1. `CLAUDE_ENDPOINT` set tapi `ClaudeModel` kosong → runtime error saat Claude dipanggil
2. `MASSIVE_S3_ACCESS_KEY` set tapi `MASSIVE_S3_SECRET_KEY` kosong → S3 ops gagal diam-diam
3. `MASSIVE_S3_SECRET_KEY` set tapi `MASSIVE_S3_ACCESS_KEY` kosong → sama
4. `GEMINI_MODEL` kosong setelah `GEMINI_API_KEY` diset → akan pakai default hardcoded, tapi bisa membingungkan
5. `DATA_DIR` tidak ada atau tidak writable → app crash saat first write, bukan saat startup

---

## Solution

Expand `config.validate()` di `internal/config/config.go`:

```go
func (c *Config) validate() {
    // Existing checks
    if c.COTHistoryWeeks < 4 {
        log.Fatal().Msg("COT_HISTORY_WEEKS must be >= 4")
    }
    if c.COTFetchInterval < 1*time.Minute {
        log.Fatal().Msg("COT_FETCH_INTERVAL must be >= 1m")
    }
    if c.ConfluenceCalcInterval < 1*time.Minute {
        log.Fatal().Msg("CONFLUENCE_CALC_INTERVAL must be >= 1m")
    }

    // NEW: Cross-field validation
    if c.ClaudeEndpoint != "" && c.ClaudeModel == "" {
        log.Fatal().Msg("CLAUDE_MODEL must be set when CLAUDE_ENDPOINT is configured")
    }

    // NEW: Massive S3 must be paired
    hasS3Key := c.MassiveS3AccessKey != ""
    hasS3Secret := c.MassiveS3SecretKey != ""
    if hasS3Key != hasS3Secret {
        log.Fatal().Msg("MASSIVE_S3_ACCESS_KEY and MASSIVE_S3_SECRET_KEY must both be set or both empty")
    }

    // NEW: DATA_DIR writable check
    testFile := filepath.Join(c.DataDir, ".write_test")
    if err := os.WriteFile(testFile, []byte("ok"), 0600); err != nil {
        log.Fatal().Str("dir", c.DataDir).Err(err).Msg("DATA_DIR is not writable")
    }
    _ = os.Remove(testFile)
}
```

---

## Implementation Steps

1. Edit `internal/config/config.go` — expand `validate()` function
2. Add `"path/filepath"` import jika belum ada
3. Tambah log.Warn (bukan Fatal) untuk case yang hanya "strange but ok":
   - `GEMINI_API_KEY` set tapi `GEMINI_MODEL` masih default → log.Warn saja

---

## Acceptance Criteria

- [ ] 2 cross-field validations (Claude + Massive S3) ditambahkan dengan log.Fatal
- [ ] DATA_DIR writable check ditambahkan
- [ ] `go build ./...` clean
- [ ] Tidak ada behavior change pada config yang valid

---

## Notes

- JANGAN ubah logic di luar `validate()` function
- Ini adalah Phase 1 change — tidak ada behavior change untuk config yang valid
- Import tambahan: `"os"` sudah ada, cek apakah `"path/filepath"` perlu ditambah
