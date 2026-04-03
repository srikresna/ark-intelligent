# ARK Intelligent Code Style Guide

> Dokumen ini mendokumentasikan konvensi penamaan dan gaya kode untuk proyek ARK Intelligent.
> Semua kontributor WAJIB mengikuti panduan ini untuk menjaga konsistensi codebase.

---

## 1. Naming Conventions

### 1.1 Receiver Names (Method Receivers)

**Rule:** Gunakan single letter yang merefleksikan nama type. Hindari abbreviations multi-letter.

| Type | Good | Bad |
|------|------|-----|
| `*ChatService` | `func (s *ChatService)` | `func (cs *ChatService)` ❌ |
| `*GeminiClient` | `func (g *GeminiClient)` atau `func (c *GeminiClient)` | `func (gc *GeminiClient)` ❌ |
| `*Scheduler` | `func (s *Scheduler)` | - |
| `*Breaker` | `func (b *Breaker)` | - |
| `*Fetcher` | `func (f *Fetcher)` | - |
| `*Handler` | `func (h *Handler)` | - |
| `*Bot` | `func (b *Bot)` | - |

**Rationale:** Go convention menggunakan single letter receiver names. Ini:
- Menjaga konsistensi dengan standard library Go
- Meningkatkan readability (tidak ada cognitive load untuk memahami abbreviations)
- Mengurangi visual clutter dalam method signatures

### 1.2 Interface Naming

**Rule:** Interface names mengikuti Go convention - **NO I-prefix**.

| Good | Bad |
|------|-----|
| `AIAnalyzer` | `IAIAnalyzer` ❌ |
| `ChatEngine` | `IChatEngine` ❌ |
| `Messenger` | `IMessenger` ❌ |
| `PriceFetcher` | `IPriceFetcher` ❌ |

**Rationale:** Go tidak menggunakan Hungarian notation. Interface adalah tipe first-class citizen dan namanya harus descriptive, bukan prefixed.

### 1.3 Mutex Naming

**Rule:** Gunakan `mu` untuk mutex dalam struct. Untuk package-level variables, gunakan `<variable>Mu` pattern.

#### For Struct Fields:
```go
// Good
type MyStruct struct {
    mu sync.RWMutex
    data map[string]string
}

// Avoid (unless multiple mutexes needed)
type MyStruct struct {
    dataMu sync.RWMutex  // only if you have multiple mutexes
}
```

#### For Package-Level Variables:
```go
// Good - use Mu suffix for package-level
var (
    globalCache    *cachedData
    cacheMu        sync.RWMutex
    postFetchHook  func(context.Context, *Data)
    postFetchHookMu sync.RWMutex
)
```

### 1.4 Constant Naming

**Rule:** Exported constants menggunakan PascalCase, unexported menggunakan camelCase.

```go
// Exported - PascalCase
const DefaultTTL = 1 * time.Hour
const MaxRetries = 3
var ErrAIFallback = errors.New("AI fallback")

// Unexported - camelCase
const defaultTimeout = 30 * time.Second
const maxBufferSize = 1024
```

### 1.5 Variable Naming

**Rule:** Prefer full names untuk clarity, kecuali untuk pattern yang sudah well-established.

| Context | Good | Bad |
|---------|------|-----|
| Context parameter | `ctx context.Context` | `c context.Context` ❌ |
| Error variable | `err error` | `e error` ❌ |
| Mutex | `mu sync.Mutex` | `m sync.Mutex` atau `mutex sync.Mutex` ❌ |

**Abbreviations yang diperbolehkan (well-established):**
- `ctx` untuk `context.Context` (standard Go)
- `err` untuk `error` (standard Go)
- `mu` untuk `sync.Mutex` / `sync.RWMutex` (Go convention)
- `wg` untuk `sync.WaitGroup` (Go convention)
- `id` untuk identifier (universal)
- `url`, `uri` untuk URLs (universal)
- `http`, `json`, `xml` untuk protocol names (universal)

### 1.6 Acronyms in Names

**Rule:** Acronyms (HTTP, URL, JSON, XML, API, etc.) harus konsisten casing-nya.

| Good | Bad |
|------|-----|
| `ServeHTTP` | `ServeHttp` ❌ |
| `APIKey` | `ApiKey` ❌ |
| `JSONParser` | `JsonParser` ❌ |
| `URLString` | `UrlString` ❌ |

---

## 2. Code Organization

### 2.1 Package Structure

```
internal/
  service/     # Business logic implementations
  adapter/     # External adapters (Telegram, DB, etc.)
  ports/       # Interface definitions
  domain/      # Domain models and entities
  config/      # Configuration
  scheduler/   # Background job scheduling
  
pkg/
  logger/      # Shared logging utilities
  retry/       # Shared retry utilities
  errs/        # Error handling utilities
  mathutil/    # Math utilities
  timeutil/    # Time utilities
  fmtutil/     # Formatting utilities
```

### 2.2 File Naming

- Gunakan lowercase dengan underscore untuk multi-word files
- Good: `chat_service.go`, `price_fetcher.go`, `error_handler.go`
- Bad: `chatService.go`, `PriceFetcher.go` ❌

---

## 3. Comments and Documentation

### 3.1 Package Documentation

Setiap package harus memiliki package-level comment:

```go
// Package scheduler orchestrates all background periodic jobs.
// Each job runs on its own ticker, respects context cancellation,
// and logs errors without crashing the process.
package scheduler
```

### 3.2 Exported Symbols

Semua exported symbols (types, functions, constants) harus memiliki doc comment:

```go
// ChatService orchestrates the chatbot pipeline:
// 1. Load conversation history
// 2. Build context-aware system prompt
// 3. Call Claude → Gemini → template fallback
// 4. Persist conversation
type ChatService struct {
    // ...
}
```

### 3.3 Comment Style

- Gunakan complete sentences dengan proper punctuation
- Start dengan nama symbol yang didokumentasikan
- Jelaskan "why" bukan "what" (code menjelaskan "what")

---

## 4. Refactoring Priorities

### High Priority (Fix Soon)

1. **Receiver names yang inconsistent:**
   - `internal/service/ai/chat_service.go`: Change `cs` → `s`
   - `internal/service/ai/gemini.go`: Change `gc` → `g` atau `c`

### Medium Priority (Fix When Touching Code)

2. **Standardize mutex naming:**
   - Audit semua struct fields menggunakan `Mu` suffix vs `mu`
   - Prefer `mu` untuk single mutex dalam struct

### Low Priority (New Code Only)

3. **Ensure all new code follows this guide**

---

## 5. Tools dan Validation

### 5.1 Linters yang Harus Pass

```bash
# Standard Go tools
go vet ./...
go fmt ./...

# Additional linters (via golangci-lint)
golangci-lint run
```

### 5.2 Pre-Commit Checklist

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go fmt ./...` produces no changes
- [ ] Receiver names follow single-letter convention
- [ ] Interface names have no I-prefix
- [ ] All exported symbols have doc comments

---

## 6. References

- [Effective Go - Naming](https://go.dev/doc/effective_go#names)
- [Go Code Review Comments - Naming](https://github.com/golang/go/wiki/CodeReviewComments#naming)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)

---

**Maintainer:** [TechLead-Intel](/PHI/agents/techlead-intel)  
**Last Updated:** 2026-04-03  
**Status:** Active
