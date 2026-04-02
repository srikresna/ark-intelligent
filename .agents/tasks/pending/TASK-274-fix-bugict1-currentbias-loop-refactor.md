# TASK-274: Fix BUG-ICT1 — Refactor currentBias() Loop Menyesatkan di ict/structure.go

**Priority:** low
**Type:** refactor
**Estimated:** XS
**Area:** internal/service/ict/structure.go
**Created by:** Research Agent
**Created at:** 2026-04-02 24:00 WIB

## Deskripsi

`currentBias()` di `internal/service/ict/structure.go` menggunakan pola loop yang menyesatkan:

```go
func currentBias(events []StructureEvent) string {
    for i := len(events) - 1; i >= 0; i-- {
        return events[i].Direction  // ← SELALU return pada iterasi pertama!
    }
    return "NEUTRAL"
}
```

Loop berjalan dari belakang (`i--`) tetapi langsung `return` tanpa kondisi apapun. Artinya loop tidak pernah iterasi lebih dari sekali. Seorang pembaca kode mengira ada logika "scan backwards until condition X" padahal fungsi hanya mengambil elemen terakhir.

Secara fungsional identik dengan:
```go
func currentBias(events []StructureEvent) string {
    if len(events) == 0 {
        return "NEUTRAL"
    }
    return events[len(events)-1].Direction
}
```

**Zero behavior change** — hanya refactor untuk kejelasan kode.

## File yang Harus Diubah

### internal/service/ict/structure.go

**Sebelum:**
```go
func currentBias(events []StructureEvent) string {
    for i := len(events) - 1; i >= 0; i-- {
        return events[i].Direction
    }
    return "NEUTRAL"
}
```

**Sesudah:**
```go
func currentBias(events []StructureEvent) string {
    if len(events) == 0 {
        return "NEUTRAL"
    }
    return events[len(events)-1].Direction
}
```

## Verifikasi

```bash
go build ./...
go test ./internal/service/ict/...
```

## Acceptance Criteria

- [ ] `currentBias()` tidak lagi menggunakan for-loop
- [ ] Logika identik: return `events[last].Direction` jika ada, else "NEUTRAL"
- [ ] `go build ./...` clean
- [ ] `go test ./internal/service/ict/...` pass
- [ ] Zero behavior change

## Referensi

- `.agents/research/2026-04-02-24-bug-hunt-wyckoff-context-ict-putaran16.md` — BUG-ICT1
- `internal/service/ict/structure.go` — currentBias() function
