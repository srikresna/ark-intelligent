# Error Handling Guide

This document describes the error handling patterns used in ARK Intelligent and provides guidelines for consistent error handling across the codebase.

## Overview

ARK Intelligent follows Go's idiomatic error handling with these key principles:

1. **Wrap errors with context** using `%w` verb (Go 1.13+)
2. **Use sentinel errors** for common domain conditions
3. **Check errors with `errors.Is()`** for specific error types
4. **Preserve error chains** for debugging while showing user-friendly messages

## Quick Reference

### Wrapping Errors

Always use `%w` to wrap errors, never `%v`:

```go
// ✅ Correct - preserves error chain
return fmt.Errorf("fetching price for %s: %w", symbol, err)

// ❌ Wrong - loses error chain
return fmt.Errorf("fetching price for %s: %v", symbol, err)
```

### Sentinel Errors

Use predefined sentinel errors from `internal/domain/errors.go`:

```go
var (
    ErrNotFound         = errors.New("data not found")
    ErrInvalidInput     = errors.New("invalid input")
    ErrInsufficientData = errors.New("insufficient data")
    ErrTimeout          = errors.New("operation timed out")
    ErrRateLimited      = errors.New("rate limited")
    ErrUnavailable      = errors.New("service unavailable")
)
```

### Checking Errors

Use `errors.Is()` to check for specific error conditions:

```go
if errors.Is(err, domain.ErrNotFound) {
    // Handle not found case
}

if errors.Is(err, context.DeadlineExceeded) {
    // Handle timeout
}
```

Use `errors.As()` to extract specific error types:

```go
var netErr *net.Error
if errors.As(err, &netErr) {
    // Handle network error
}
```

## Error Handling Patterns

### 1. Repository/Storage Layer

```go
func (r *priceRepo) GetPrice(symbol string) (*domain.Price, error) {
    data, err := r.db.Get([]byte(key))
    if err != nil {
        if errors.Is(err, badger.ErrKeyNotFound) {
            return nil, fmt.Errorf("price %s: %w", symbol, domain.ErrNotFound)
        }
        return nil, fmt.Errorf("price %s: %w", symbol, domain.ErrStorage)
    }
    // ... parse and return
}
```

### 2. Service Layer

```go
func (s *priceService) FetchPrice(ctx context.Context, symbol string) (*domain.Price, error) {
    if symbol == "" {
        return nil, fmt.Errorf("fetch price: %w", domain.ErrInvalidInput)
    }
    
    price, err := s.apiClient.GetPrice(ctx, symbol)
    if err != nil {
        return nil, fmt.Errorf("fetch price %s: %w", symbol, err)
    }
    return price, nil
}
```

### 3. Handler Layer (Telegram Bot)

The Telegram adapter has sophisticated error handling in `errors.go`:

```go
// Log the technical error
log.Error().Err(err).Str("command", cmd).Msg("handler error")

// Send user-friendly message
friendly := userFriendlyError(err, command)
h.bot.SendHTML(ctx, chatID, friendly)
```

Key features:
- **Technical errors** are logged with full context for debugging
- **User-facing errors** are translated to friendly Indonesian messages
- **Retry buttons** are shown for retriable errors
- **Error classification** determines which errors warrant retry

### 4. Retriable Errors

Errors that may succeed on retry:

- Timeouts (`context.DeadlineExceeded`)
- Network issues (`connection refused`, `no such host`)
- Rate limits (`rate limit`, `429`)
- Chart rendering failures
- AI service failures

Non-retriable errors:

- Permission errors (`401`, `403`)
- Missing data (`not found`)
- Invalid input
- Insufficient data for analysis

## Domain Errors Reference

### Common Errors (`internal/domain/errors.go`)

| Error | Use When |
|-------|----------|
| `ErrNotFound` | Data/resource doesn't exist |
| `ErrInvalidInput` | Parameters are invalid |
| `ErrInsufficientData` | Not enough data for operation |
| `ErrTimeout` | Operation timed out |
| `ErrRateLimited` | Hit rate limit |
| `ErrUnavailable` | Service temporarily down |

### Storage Errors

| Error | Use When |
|-------|----------|
| `ErrStorage` | General database error |
| `ErrKeyNotFound` | Specific key not in storage |

### API Errors

| Error | Use When |
|-------|----------|
| `ErrAPIRequest` | External API call failed |
| `ErrAPITimeout` | External API timed out |
| `ErrAPIRateLimit` | External API rate limit hit |

### AI Errors

| Error | Use When |
|-------|----------|
| `ErrAIGeneration` | AI content generation failed |
| `ErrAIServiceUnavailable` | AI service not available |

## Best Practices

### 1. Always Add Context

```go
// ✅ Good - context at each layer
if err != nil {
    return fmt.Errorf("parsing COT report: %w", err)
}

// ❌ Bad - no context
if err != nil {
    return err
}
```

### 2. Don't Log and Return

```go
// ✅ Good - let caller decide to log
if err != nil {
    return fmt.Errorf("fetch data: %w", err)
}

// ❌ Bad - double logging
if err != nil {
    log.Error().Err(err).Msg("fetch failed")
    return err
}
```

Exception: Handlers at the edge (like Telegram handlers) should log since there's no caller above them.

### 3. Use Sentinels for Control Flow

```go
price, err := repo.GetPrice(symbol)
if err != nil {
    if errors.Is(err, domain.ErrNotFound) {
        // Fetch from API instead
        return s.fetchFromAPI(ctx, symbol)
    }
    return nil, err
}
```

### 4. Handle Panics at Boundaries

```go
func (h *Handler) safeOperation(ctx context.Context) (err error) {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).Msg("panic recovered")
            err = fmt.Errorf("operation panicked: %v", r)
        }
    }()
    // ... do work
}
```

Note: Use `%v` for panic values since they may not be errors.

## User-Friendly Error Messages

The Telegram adapter provides `userFriendlyError()` which:

1. Maps technical errors to Indonesian user messages
2. Provides actionable suggestions
3. Shows retry buttons for retriable errors
4. Categorizes errors by type (timeout, not found, network, etc.)

When adding new error types, update `userFriendlyError()` in `internal/adapter/telegram/errors.go`.

## Testing Errors

Use `errors.Is()` in tests:

```go
func TestFetchPrice_NotFound(t *testing.T) {
    _, err := service.FetchPrice(ctx, "UNKNOWN")
    if !errors.Is(err, domain.ErrNotFound) {
        t.Errorf("expected ErrNotFound, got %v", err)
    }
}
```

## Migration Guide

When refactoring existing code:

1. Replace `%v` with `%w` for error wrapping
2. Replace string-based error checking with `errors.Is()`
3. Add sentinel errors to `internal/domain/errors.go` as needed
4. Update error messages to follow the pattern: `context: %w`

Example migration:

```go
// Before
if err != nil {
    return fmt.Errorf("fetch failed: %v", err)
}
if err.Error() == "not found" {
    // handle not found
}

// After
if err != nil {
    return fmt.Errorf("fetch failed: %w", err)
}
if errors.Is(err, domain.ErrNotFound) {
    // handle not found
}
```
