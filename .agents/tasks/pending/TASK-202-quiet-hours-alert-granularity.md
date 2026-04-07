# TASK-202: Quiet Hours + Per-Alert-Type Granularity + Frequency Cap

**Priority:** medium
**Type:** ux
**Estimated:** L
**Area:** internal/domain/, internal/scheduler/, internal/adapter/telegram/

## Deskripsi

Add quiet hours (DND window), per-alert-type toggles, dan daily frequency cap. Users with all alerts get 20+ messages/day with no control beyond binary on/off.

## New Preferences

```go
type UserPrefs struct {
    // Existing...
    
    // NEW:
    QuietHoursEnabled bool   `json:"quiet_hours_enabled"`
    QuietHoursStart   int    `json:"quiet_hours_start"`  // 0-23 WIB
    QuietHoursEnd     int    `json:"quiet_hours_end"`     // 0-23 WIB
    
    AlertTypes map[string]bool `json:"alert_types"` // per-type toggle
    // Keys: "cot_release", "fred_regime", "signal_strong", "news_high", "concentration"
    
    MaxAlertsPerDay int `json:"max_alerts_per_day"` // 0 = unlimited
}
```

## File Changes

- `internal/domain/prefs.go` — Add new preference fields
- `internal/scheduler/scheduler.go` — Check quiet hours + alert type before broadcast
- `internal/service/news/scheduler.go` — Same
- `internal/adapter/telegram/keyboard.go` — Add alert management sub-menu
- `internal/adapter/telegram/handler.go` — Handle alert settings callbacks
- `internal/adapter/storage/prefs_repo.go` — Track daily alert count per user

## Acceptance Criteria

- [ ] Quiet hours: no alerts between start and end hour (WIB)
- [ ] Per-alert-type toggle in /settings
- [ ] Daily frequency cap with counter reset at midnight WIB
- [ ] Default: quiet hours OFF, all alert types ON, no cap
- [ ] Queued alerts sent when quiet hours end (option: discard or delay)
- [ ] /settings shows clear alert management sub-menu
