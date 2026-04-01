# TASK-198: AI ContentBlocks Nil Pointer Guard

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/ai/

## Deskripsi

Add nil check pada individual content blocks sebelum accessing .Type dan .Text. Prevent panic saat AI returns unexpected nil block.

## Bug Detail

```go
// chat_service.go:82-94
for _, b := range contentBlocks {
    if b.Type == "text" {  // PANIC if b is nil
        // ...
    }
}
```

## Fix

```go
for _, b := range contentBlocks {
    if b == nil { continue }  // ADD: nil guard
    if b.Type == "text" && b.Text != "" {
        effectiveText = b.Text
        break
    }
}
```

Same fix in `describeContentBlocks()` (line 320).

## File Changes

- `internal/service/ai/chat_service.go` — Add nil check in content block iteration (2 locations)

## Acceptance Criteria

- [ ] Nil content blocks skipped safely
- [ ] describeContentBlocks() also protected
- [ ] No panic on nil blocks
- [ ] Existing non-nil blocks processed normally
- [ ] Unit test: contentBlocks with nil element
