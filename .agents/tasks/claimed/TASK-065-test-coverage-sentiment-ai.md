# TASK-065: Test Coverage — Sentiment & AI Service (TECH-009)

**Priority:** HIGH  
**Type:** Tech Refactor / Test  
**Ref:** TECH-009 in TECH_REFACTOR_PLAN.md  
**Branch target:** dev-b atau dev-c  
**Estimated size:** Medium (300-500 LOC test)

---

## Problem

`internal/service/sentiment/` dan `internal/service/ai/` memiliki ZERO test files.
TECH plan menyebut target minimal 60% coverage untuk service layer.

Sentiment service memanggil 3 external APIs (CNN, AAII, CBOE) tanpa test coverage apapun.
AI service (`cached_interpreter.go`) memiliki caching logic kompleks yang perlu divalidasi.

---

## Scope

### 1. `internal/service/sentiment/sentiment_test.go` (baru)

Test target (gunakan interface atau inject `*http.Client` mock):

```go
// Test 1: FetchSentiment dengan mock HTTP — verifikasi field terisi
func TestFetchSentiment_MockHTTP(t *testing.T) { ... }

// Test 2: normalizeFearGreedLabel mapping
func TestNormalizeFearGreedLabel(t *testing.T) {
    cases := []struct{ input, want string }{
        {"Extreme Fear", "Extreme Fear"},
        {"extreme fear", "Extreme Fear"},
        {"GREED", "Greed"},
        {"", "Neutral"},
    }
}

// Test 3: SentimentData zero value — tidak ada panic jika field kosong
func TestSentimentData_ZeroValue(t *testing.T) { ... }
```

### 2. `internal/service/ai/cache_test.go` (baru)

Test target untuk `CachedInterpreter`:

```go
// Test 1: Cache hit — inner tidak dipanggil jika cache valid
func TestCachedInterpreter_CacheHit(t *testing.T) { ... }

// Test 2: Cache miss — inner dipanggil, result disimpan ke cache
func TestCachedInterpreter_CacheMiss(t *testing.T) { ... }

// Test 3: InvalidateOnCOTUpdate — entry yang tepat di-invalidate
func TestCachedInterpreter_Invalidate(t *testing.T) { ... }

// Test 4: IsAvailable — return false jika inner tidak available
func TestCachedInterpreter_IsAvailable(t *testing.T) { ... }
```

---

## Approach

- Gunakan in-memory mock untuk `ports.AICacheRepository` (interface) — tidak perlu Badger
- Gunakan `httptest.NewServer` untuk mock HTTP calls di sentiment
- Jangan test `GeminiClient` atau `ClaudeClient` langsung (butuh real API)
- Test hanya **pure logic** dan **caching behavior**

---

## Acceptance Criteria

- [ ] `sentiment_test.go` dibuat dengan minimal 3 test functions
- [ ] `cache_test.go` dibuat dengan minimal 4 test functions
- [ ] `go test ./internal/service/sentiment/...` dan `./internal/service/ai/...` pass
- [ ] No build errors: `go build ./...`

---

## Notes

- Baca TECH_REFACTOR_PLAN.md TECH-009 sebelum mulai
- Target coverage: minimal 40% untuk kedua package ini (dari 0%)
- Jika sulit mock karena dependency injection tidak tersedia, buat minimal test untuk pure functions dulu
