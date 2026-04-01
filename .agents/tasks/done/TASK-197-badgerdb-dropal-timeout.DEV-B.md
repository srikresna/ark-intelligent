# TASK-197: BadgerDB DropAll() — Add Context Timeout

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/storage/

## Deskripsi

Wrap BadgerDB DropAll() dengan context timeout untuk prevent deadlock saat concurrent readers hold transactions.

## Current

```go
func (d *DB) DropAll() error {
    return d.db.DropAll()  // blocks indefinitely
}
```

## Fix

```go
func (d *DB) DropAll(ctx context.Context) error {
    done := make(chan error, 1)
    go func() { done <- d.db.DropAll() }()
    select {
    case err := <-done:
        if err != nil {
            return fmt.Errorf("badger drop all: %w", err)
        }
        log.Info().Msg("all data dropped")
        return nil
    case <-ctx.Done():
        return fmt.Errorf("badger drop all timed out: %w", ctx.Err())
    }
}
```

## File Changes

- `internal/adapter/storage/badger.go` — Add context parameter to DropAll, wrap with timeout
- Callers of DropAll — Pass context with 30s timeout

## Acceptance Criteria

- [ ] DropAll accepts context.Context
- [ ] 30-second timeout default
- [ ] Timeout produces clear error message
- [ ] Existing callers updated
- [ ] No deadlock possible
