# TASK-173: FRED Composites Nil Pointer Guard in Regime + Confluence

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/service/fred/, internal/service/cot/

## Deskripsi

Add nil checks untuk ComputeComposites() return values di semua call sites. Currently beberapa paths access composite fields tanpa nil check → panic saat country data missing.

## Bug Locations

1. `internal/service/cot/confluence_score.go:255-260` — checks `composites != nil` tapi tidak check individual composite entries
2. `internal/service/fred/regime.go:254+` — accesses `comp.LaborHealth` etc tanpa full nil guard

## Fix Pattern

```go
composites := fred.ComputeComposites(macroData)
if composites == nil || len(composites) == 0 {
    // use fallback scoring without composites
    macroScore = 0
} else {
    for _, comp := range composites {
        if comp == nil { continue }
        // safe to access comp fields
    }
}
```

## File Changes

- `internal/service/cot/confluence_score.go` — Add defensive nil checks for composites array + individual entries
- `internal/service/fred/regime.go` — Add nil checks before accessing composite fields
- `internal/service/fred/composites.go` — Ensure ComputeComposites never returns slice with nil entries

## Acceptance Criteria

- [ ] All ComputeComposites() call sites have nil guard
- [ ] Individual composite entries checked before field access
- [ ] Fallback scoring (0) when composites unavailable
- [ ] No panic on fresh install with empty FRED data
- [ ] Unit test: ComputeComposites with empty/partial macro data
