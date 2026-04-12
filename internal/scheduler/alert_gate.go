package scheduler

// alert_gate.go — Quiet hours, per-alert-type, and daily frequency cap (TASK-202).

import (
	"context"
	"fmt"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// AlertGate enforces quiet hours, per-alert-type toggles, and daily caps.
type AlertGate struct {
	db *badger.DB
	mu sync.Mutex // protects counter reads/writes
}

// NewAlertGate creates a gate backed by the given BadgerDB.
func NewAlertGate(db *badger.DB) *AlertGate {
	return &AlertGate{db: db}
}

// ShouldDeliver returns true if the alert should be delivered right now.
// It checks (in order):
//  1. Quiet hours — is it within the user's DND window?
//  2. Alert type — has the user disabled this alert type?
//  3. Daily cap — has the user exceeded MaxAlertsPerDay?
//
// reason is a short human-readable explanation when blocked.
func (g *AlertGate) ShouldDeliver(prefs domain.UserPrefs, alertType string) (ok bool, reason string) {
	// 1. Quiet hours check.
	now := timeutil.NowWIB()
	if prefs.IsInQuietHours(now.Hour()) {
		return false, "quiet_hours"
	}

	// 2. Per-type check.
	if !prefs.IsAlertTypeEnabled(alertType) {
		return false, "type_disabled"
	}

	// 3. Daily cap (checked but not incremented here — call RecordDelivery after send).
	if prefs.MaxAlertsPerDay > 0 && g.db != nil {
		count := g.todayCount(prefs.ChatID)
		if count >= prefs.MaxAlertsPerDay {
			return false, "daily_cap"
		}
	}

	return true, ""
}

// RecordDelivery increments the daily alert counter for the user.
// Call after a successful send.
func (g *AlertGate) RecordDelivery(_ context.Context, chatID string) {
	if g.db == nil || chatID == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	key := g.dailyKey(chatID)
	_ = g.db.Update(func(txn *badger.Txn) error {
		var count int
		item, err := txn.Get(key)
		if err == nil {
			_ = item.Value(func(val []byte) error {
				fmt.Sscanf(string(val), "%d", &count)
				return nil
			})
		}
		count++
		e := badger.NewEntry(key, []byte(fmt.Sprintf("%d", count)))
		// Expire at next midnight WIB (auto-reset).
		e = e.WithTTL(g.ttlUntilMidnightWIB())
		return txn.SetEntry(e)
	})
}

// todayCount returns the number of alerts delivered today for the given chatID.
func (g *AlertGate) todayCount(chatID string) int {
	if g.db == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	var count int
	key := g.dailyKey(chatID)
	_ = g.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			fmt.Sscanf(string(val), "%d", &count)
			return nil
		})
	})
	return count
}

func (g *AlertGate) dailyKey(chatID string) []byte {
	day := timeutil.NowWIB().Format("2006-01-02")
	return []byte(fmt.Sprintf("alert_count:%s:%s", chatID, day))
}

func (g *AlertGate) ttlUntilMidnightWIB() time.Duration {
	now := timeutil.NowWIB()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	d := midnight.Sub(now)
	if d <= 0 {
		d = 24 * time.Hour
	}
	return d
}
