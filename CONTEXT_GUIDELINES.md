# Context Usage Guidelines

This document describes best practices for context usage in ARK Intelligent handlers and services.

## Overview

Go's `context.Context` is used throughout ARK Intelligent for:
- **Request cancellation**: Stopping operations when user disconnects or times out
- **Timeout enforcement**: Preventing handlers from running indefinitely
- **Request tracing**: Tracking requests through the system with unique IDs
- **Value propagation**: Passing request-scoped values (logging, auth)

## Quick Reference

### Standard Timeouts

Use the predefined timeout constants from `context_utils.go`:

```go
const (
    DefaultHandlerTimeout = 30 * time.Second   // Most commands
    SlowHandlerTimeout    = 60 * time.Second   // Complex analysis
    ChartHandlerTimeout   = 90 * time.Second   // Python chart scripts
    AIHandlerTimeout      = 120 * time.Second  // AI generation
    ExternalAPITimeout    = 45 * time.Second    // External APIs
)
```

### Basic Handler Pattern

```go
func (h *Handler) cmdExample(ctx context.Context, chatID string, userID int64, args string) error {
    // Setup context with timeout and request ID
    ctx, cancel := setupHandlerContext(ctx, DefaultHandlerTimeout)
    defer cancel()
    
    // Check for early cancellation
    if err := checkContext(ctx); err != nil {
        return err
    }
    
    // Pass context downstream
    result, err := h.service.GetData(ctx, params)
    if err != nil {
        return fmt.Errorf("get data: %w", err)
    }
    
    return h.bot.SendHTML(ctx, chatID, result)
}
```

### Checking Cancellation

For long-running operations or loops, check cancellation periodically:

```go
// Simple check
if isCancelled(ctx) {
    return ctx.Err()
}

// With error return
if err := checkContext(ctx); err != nil {
    return err
}

// In a loop
for _, item := range items {
    if err := checkContext(ctx); err != nil {
        return err
    }
    // Process item...
}
```

## Handler Patterns

### 1. Simple Database/API Handler (30s timeout)

Handlers that query the database or make simple API calls:

```go
func (h *Handler) cmdPins(ctx context.Context, chatID string, userID int64, _ string) error {
    ctx, cancel := setupHandlerContext(ctx, DefaultHandlerTimeout)
    defer cancel()
    
    prefs, err := h.prefsRepo.Get(ctx, userID)
    if err != nil {
        return fmt.Errorf("get prefs: %w", err)
    }
    
    // ... format and send response
    return h.bot.SendHTML(ctx, chatID, message)
}
```

### 2. Chart Generation Handler (90s timeout)

Handlers that execute Python scripts for chart generation:

```go
func (h *Handler) cmdCTA(ctx context.Context, chatID string, userID int64, args string) error {
    ctx, cancel := setupHandlerContext(ctx, ChartHandlerTimeout)
    defer cancel()
    
    // ... compute state
    
    chart, err := h.getCTAChart(ctx, state, timeframe)
    if err != nil {
        return fmt.Errorf("generate chart: %w", err)
    }
    
    return h.bot.SendPhoto(ctx, chatID, chart)
}
```

### 3. AI-Powered Handler (120s timeout)

Handlers that call AI services (Gemini/Claude):

```go
func (h *Handler) cmdOutlook(ctx context.Context, chatID string, userID int64) error {
    ctx, cancel := setupHandlerContext(ctx, AIHandlerTimeout)
    defer cancel()
    
    // Check AI quota before proceeding
    allowed, reason := h.middleware.CheckAIQuota(ctx, userID)
    if !allowed {
        return h.bot.SendHTML(ctx, chatID, reason)
    }
    
    analysis, err := h.aiService.GenerateOutlook(ctx, params)
    if err != nil {
        return fmt.Errorf("generate outlook: %w", err)
    }
    
    return h.bot.SendHTML(ctx, chatID, analysis)
}
```

### 4. External API Handler (45s timeout)

Handlers that fetch data from external financial APIs:

```go
func (h *Handler) cmdPrice(ctx context.Context, chatID string, userID int64, args string) error {
    ctx, cancel := setupHandlerContext(ctx, ExternalAPITimeout)
    defer cancel()
    
    price, err := h.priceService.FetchPrice(ctx, symbol)
    if err != nil {
        return fmt.Errorf("fetch price: %w", err)
    }
    
    return h.bot.SendHTML(ctx, chatID, formatPrice(price))
}
```

## Context Propagation

### Always Pass Context

Always pass context to downstream functions, even if they don't use it yet:

```go
// ✅ Good - context propagated
func (s *Service) FetchPrice(ctx context.Context, symbol string) (*Price, error) {
    return s.client.GetPrice(ctx, symbol)
}

// ❌ Bad - context lost
func (s *Service) FetchPrice(symbol string) (*Price, error) {
    return s.client.GetPrice(symbol) // Missing context!
}
```

### Context in Loops

For batch operations, check context between iterations:

```go
func (s *Service) FetchMultiple(ctx context.Context, symbols []string) ([]Price, error) {
    results := make([]Price, 0, len(symbols))
    
    for _, symbol := range symbols {
        // Check cancellation between items
        if err := checkContext(ctx); err != nil {
            return nil, err
        }
        
        price, err := s.FetchPrice(ctx, symbol)
        if err != nil {
            return nil, fmt.Errorf("fetch %s: %w", symbol, err)
        }
        results = append(results, price)
    }
    
    return results, nil
}
```

### Context in Goroutines

When spawning goroutines, always pass context explicitly:

```go
// ✅ Good - context passed to goroutine
func (s *Service) ParallelFetch(ctx context.Context, symbols []string) ([]Price, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    var wg sync.WaitGroup
    results := make([]Price, len(symbols))
    
    for i, symbol := range symbols {
        wg.Add(1)
        go func(idx int, sym string) {
            defer wg.Done()
            
            price, err := s.FetchPrice(ctx, sym)
            if err != nil {
                cancel() // Cancel siblings on first error
                return
            }
            results[idx] = price
        }(i, symbol)
    }
    
    wg.Wait()
    return results, nil
}

// ❌ Bad - using closure variable
func (s *Service) BadExample(ctx context.Context, symbols []string) {
    for _, symbol := range symbols {
        go func() {
            // symbol is shared - race condition!
            s.FetchPrice(ctx, symbol)
        }()
    }
}
```

## Request Tracing

### Request ID in Logs

The context utilities add a request ID automatically. Include it in logs:

```go
func (h *Handler) cmdExample(ctx context.Context, chatID string, userID int64) error {
    ctx, cancel := setupHandlerContext(ctx, DefaultHandlerTimeout)
    defer cancel()
    
    // Log with request ID
    log.Info().
        Str("request_id", requestID(ctx)).
        Str("chat_id", chatID).
        Int64("user_id", userID).
        Msg("processing command")
    
    // ... handler logic
}
```

### Request ID Propagation

For external API calls, include the request ID in headers for distributed tracing:

```go
func (c *APIClient) Call(ctx context.Context, params Params) (*Result, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    // Propagate request ID to external service
    if id := requestID(ctx); id != "" {
        req.Header.Set("X-Request-ID", id)
    }
    
    return c.http.Do(req)
}
```

## Common Mistakes

### 1. Storing Context in Structs

```go
// ❌ Never do this
type Service struct {
    ctx context.Context  // Don't store context!
    db  *sql.DB
}

// ✅ Pass context as first parameter
func (s *Service) Query(ctx context.Context, id string) (*Result, error) {
    return s.db.QueryContext(ctx, "SELECT ...", id)
}
```

### 2. Ignoring Context Cancellation

```go
// ❌ Not checking cancellation
func longOperation(ctx context.Context) error {
    for i := 0; i < 1000; i++ {
        // This keeps running even if context is cancelled
        heavyWork()
    }
    return nil
}

// ✅ Check periodically
func longOperation(ctx context.Context) error {
    for i := 0; i < 1000; i++ {
        if err := checkContext(ctx); err != nil {
            return err
        }
        heavyWork()
    }
    return nil
}
```

### 3. Not Using Timeouts

```go
// ❌ No timeout - handler can hang indefinitely
func (h *Handler) cmdSlow(ctx context.Context, chatID string) error {
    result, err := h.slowService.Call(ctx) // May hang forever
    // ...
}

// ✅ Timeout enforced
func (h *Handler) cmdSlow(ctx context.Context, chatID string) error {
    ctx, cancel := setupHandlerContext(ctx, SlowHandlerTimeout)
    defer cancel()
    
    result, err := h.slowService.Call(ctx) // Will timeout appropriately
    // ...
}
```

## Migration Guide

When updating existing handlers:

1. **Add timeout setup** at the start of handler functions:
   ```go
   ctx, cancel := setupHandlerContext(ctx, DefaultHandlerTimeout)
   defer cancel()
   ```

2. **Add cancellation checks** in loops and long operations:
   ```go
   if err := checkContext(ctx); err != nil {
       return err
   }
   ```

3. **Ensure context propagation** to all downstream calls

4. **Add request ID to logs** for better traceability

## Testing with Context

### Timeout Testing

```go
func TestHandler_Timeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
    defer cancel()
    
    err := handler.cmdSlow(ctx, "12345", 1, "")
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("expected timeout, got %v", err)
    }
}
```

### Cancellation Testing

```go
func TestHandler_Cancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    
    // Cancel immediately
    cancel()
    
    err := handler.cmdExample(ctx, "12345", 1, "")
    if !errors.Is(err, context.Canceled) {
        t.Errorf("expected canceled, got %v", err)
    }
}
```

## Summary

- ✅ Always use `setupHandlerContext()` with appropriate timeout
- ✅ Always pass context as first parameter
- ✅ Check `ctx.Done()` in loops and long operations
- ✅ Use request IDs for tracing
- ❌ Never store context in structs
- ❌ Never ignore context cancellation
- ❌ Never skip timeout setup
