# TASK-012: ICT Intraday Macro Windows Detection

**Priority:** MEDIUM
**Type:** Feature Enhancement
**Estimated effort:** S (half day)
**Ref:** research/2026-04-06-11-feature-deep-dive-siklus3.md

---

## Context

Dalam ICT methodology, "Macros" adalah intraday time windows BERBEDA dari Killzones.
Macros adalah period spesifik di mana algoritma bank aktif mengdelivery harga dan
mengisi FVG.

**6 ICT Macro Windows (Eastern Time / UTC offset -4/-5):**
| Macro | ET | UTC (EDT) | UTC (EST) |
|---|---|---|---|
| Pre-London | 02:33–03:00 | 06:33–07:00 | 07:33–08:00 |
| London Open | 08:50–09:10 | 12:50–13:10 | 13:50–14:10 |
| Mid-Morning | 09:50–10:10 | 13:50–14:10 | 14:50–15:10 |
| Late-Morning | 10:50–11:10 | 14:50–15:10 | 15:50–16:10 |
| PM Open (Silver Bullet) | 13:10–13:40 | 17:10–17:40 | 18:10–18:40 |
| PM Close | 15:15–15:45 | 19:15–19:45 | 20:15–20:45 |

Saat ini `detectKillzone()` hanya mendeteksi broad session windows (2-3 jam).
ICT Macros jauh lebih presisi (20-30 menit) dan lebih relevan untuk intraday entry.

Catatan: ICT "Silver Bullet" = PM Open Macro window (13:10-13:40 ET).

---

## Implementation

### Step 1: Add macro detection to `internal/service/ict/engine.go`

```go
// MacroWindow represents a named ICT intraday timing window.
type MacroWindow struct {
    Name    string // e.g. "London Open Macro", "Silver Bullet"
    Active  bool   // true if current time is within the window
    EndsIn  int    // minutes until window closes (if active)
    NextIn  int    // minutes until next window opens (if not active)
}

// detectICTMacro maps a UTC time to the currently active ICT Macro window.
// Returns nil if no macro window is currently active.
// ET = UTC-4 (EDT, Mar-Nov) or UTC-5 (EST, Nov-Mar).
func detectICTMacro(t time.Time) *MacroWindow {
    // Convert UTC to ET
    // Check each of the 6 windows
    // Return matching MacroWindow or nil
}
```

Add `ActiveMacro *MacroWindow` field to `ICTResult` (alongside existing `Killzone string`).

### Step 2: Update `internal/service/ict/engine.go` Analyze()

Replace or augment Step 7 (killzone detection):

```go
// Step 7a: Killzone (broad session window)
result.Killzone = detectKillzone(bars[0].Date)
// Step 7b: ICT Macro (precise 20-30min algorithm window)
result.ActiveMacro = detectICTMacro(bars[0].Date)
```

### Step 3: Update `internal/adapter/telegram/formatter_ict.go`

Show macro info near the Killzone section:

```
⏰ <b>Timing</b>
Killzone: 🇬🇧 London Open (07:00–10:00 UTC)
ICT Macro: <b>🎯 London Open Macro AKTIF</b> (12:50–13:10 UTC)
```

or when outside macro:
```
ICT Macro: Berikutnya: Mid-Morning Macro dalam 45 menit
```

### Step 4: Add DST-aware ET conversion

Handle US DST transitions (second Sunday March → first Sunday November).
Use `time.LoadLocation("America/New_York")` — no hardcoded UTC offsets.

---

## Acceptance Criteria

- [ ] `detectICTMacro()` diimplementasikan dengan semua 6 windows
- [ ] DST-aware: menggunakan `time.LoadLocation("America/New_York")`
- [ ] `ICTResult.ActiveMacro` field ditambahkan
- [ ] Formatter menampilkan macro status di `/ict` output
- [ ] Unit test: TestICTMacro_AllSixWindows verifikasi setiap window aktif pada waktunya
- [ ] Unit test: TestICTMacro_DST verifikasi behavior saat transisi EDT↔EST
- [ ] `go build ./...` dan `go test ./...` bersih
