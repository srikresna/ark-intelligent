# TASK-169: HTTP Transport Connection Pooling Configuration

**Priority:** medium
**Type:** refactor
**Estimated:** S
**Area:** internal/service/*/

## Deskripsi

Configure `http.Transport` connection pooling parameters untuk semua service HTTP clients. Currently semua clients pakai default transport (100 idle conns global, no per-host limit).

## Detail Teknis

Shared transport configuration:
```go
var sharedTransport = &http.Transport{
    MaxIdleConns:        50,
    MaxIdleConnsPerHost: 10,
    MaxConnsPerHost:     20,
    IdleConnTimeout:     90 * time.Second,
    TLSHandshakeTimeout: 10 * time.Second,
}
```

Ini complement TASK-118 (HTTP Client Factory) — TASK-118 covers factory pattern, ini covers transport tuning specifically.

## File Changes

- `pkg/httpclient/transport.go` — NEW: Shared transport with tuned pooling
- `internal/service/vix/fetcher.go` — Use shared transport
- `internal/service/cot/fetcher.go` — Use shared transport
- `internal/service/price/fetcher.go` — Use shared transport
- `internal/service/news/fetcher.go` — Use shared transport
- `internal/service/marketdata/coingecko/client.go` — Use shared transport
- `internal/service/marketdata/bybit/client.go` — Use shared transport

## Acceptance Criteria

- [ ] Shared transport created di pkg/httpclient/
- [ ] All 6+ HTTP clients use shared transport
- [ ] Connection pooling tuned per-host (max 10 idle, 20 active)
- [ ] Idle timeout set to 90 seconds
- [ ] TLS handshake timeout set to 10 seconds
- [ ] No behavior change — only connection management improved
- [ ] Can be combined with TASK-118 implementation
