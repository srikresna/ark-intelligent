package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var retentionLog = logger.Component("retention")

// RetentionPolicy defines data retention windows.
type RetentionPolicy struct {
	EventMaxAge    time.Duration // Default: 6 months
	COTMaxAge      time.Duration // Default: 52 weeks
	RevisionMaxAge time.Duration // Default: 6 months
	HistoryMaxAge  time.Duration // Default: 24 months
	PriceMaxAge    time.Duration // Default: 260 weeks (5 years)
	SignalMaxAge   time.Duration // Default: 52 weeks
	IntradayMaxAge time.Duration // Default: 60 days
}

// DefaultRetentionPolicy returns the standard retention windows.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		EventMaxAge:    6 * 30 * 24 * time.Hour,  // ~6 months
		COTMaxAge:      52 * 7 * 24 * time.Hour,  // 52 weeks
		RevisionMaxAge: 6 * 30 * 24 * time.Hour,  // ~6 months
		HistoryMaxAge:  24 * 30 * 24 * time.Hour, // ~24 months
		PriceMaxAge:    260 * 7 * 24 * time.Hour, // ~5 years (matches seasonal analysis window)
		SignalMaxAge:   52 * 7 * 24 * time.Hour,  // 52 weeks
		IntradayMaxAge: 60 * 24 * time.Hour,      // 60 days
	}
}

// RunRetentionCleanup deletes data older than the configured retention windows.
// Returns total number of keys deleted. Safe to run periodically (e.g., daily).
func (d *DB) RunRetentionCleanup(ctx context.Context, policy RetentionPolicy) (int, error) {
	now := time.Now()
	totalDeleted := 0

	// 1. Events (evt:{YYYYMMDD}:{id}) — date is 1st segment after prefix
	n, err := d.deleteByDatePrefix("evt:", 0, now.Add(-policy.EventMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup evt: %w", err)
	}
	totalDeleted += n

	// 2. News events (news:{YYYYMMDD}:{id}) — date is 1st segment after prefix
	n, err = d.deleteByDatePrefix("news:", 0, now.Add(-policy.EventMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup news: %w", err)
	}
	totalDeleted += n

	// 3. COT records (cot:{contractCode}:{YYYYMMDD}) — date is 2nd segment
	n, err = d.deleteByDatePrefix("cot:", 1, now.Add(-policy.COTMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup cot: %w", err)
	}
	totalDeleted += n

	// 4. COT analyses (cotanl:{contractCode}:{YYYYMMDD}) — date is 2nd segment
	n, err = d.deleteByDatePrefix("cotanl:", 1, now.Add(-policy.COTMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup cotanl: %w", err)
	}
	totalDeleted += n

	// 5. Event revisions (evtrev:{currency}:{YYYYMMDD}:{eventID}) — date is 2nd segment
	n, err = d.deleteByDatePrefix("evtrev:", 1, now.Add(-policy.RevisionMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup evtrev: %w", err)
	}
	totalDeleted += n

	// 6. Event history (evthist:{currency}:{eventName}:{YYYYMMDD}) — date is 3rd segment
	n, err = d.deleteByDatePrefix("evthist:", 2, now.Add(-policy.HistoryMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup evthist: %w", err)
	}
	totalDeleted += n

	// 7. Price records (price:{contractCode}:{YYYYMMDD}) — date is 2nd segment
	n, err = d.deleteByDatePrefix("price:", 1, now.Add(-policy.PriceMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup price: %w", err)
	}
	totalDeleted += n

	// 8. Signal records (sig:{contractCode}:{YYYYMMDD}:{signalType}) — date is 2nd segment
	n, err = d.deleteByDatePrefix("sig:", 1, now.Add(-policy.SignalMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup sig: %w", err)
	}
	totalDeleted += n

	// 9. Intraday bars (iprice:{contract}:{interval}:{YYYYMMDDHHmm}) — special 12-char timestamp
	n, err = d.deleteByTimestampPrefix("iprice:", now.Add(-policy.IntradayMaxAge))
	if err != nil {
		return totalDeleted, fmt.Errorf("cleanup iprice: %w", err)
	}
	totalDeleted += n

	if totalDeleted > 0 {
		retentionLog.Info().Int("deleted", totalDeleted).Msg("Cleaned up expired keys")
	}
	return totalDeleted, nil
}

// deleteByDatePrefix scans all keys with the given prefix, extracts a YYYYMMDD
// date from the segment at dateSegmentIndex (0-based, segments split by ':' after
// removing the prefix), and deletes keys where that date is before cutoff.
func (d *DB) deleteByDatePrefix(prefix string, dateSegmentIndex int, cutoff time.Time) (int, error) {
	cutoffStr := cutoff.Format("20060102")
	var deleteKeys [][]byte

	err := d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = false // keys only

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			key := string(it.Item().Key())
			// Remove prefix, split remaining by ':'
			rest := strings.TrimPrefix(key, prefix)
			segments := strings.Split(rest, ":")

			if dateSegmentIndex >= len(segments) {
				continue
			}

			dateStr := segments[dateSegmentIndex]
			// Validate it looks like YYYYMMDD (8 digits)
			if len(dateStr) != 8 {
				continue
			}

			if dateStr < cutoffStr {
				deleteKeys = append(deleteKeys, it.Item().KeyCopy(nil))
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	if len(deleteKeys) == 0 {
		return 0, nil
	}

	// Batch delete
	wb := d.db.NewWriteBatch()
	defer wb.Cancel()
	for _, k := range deleteKeys {
		if err := wb.Delete(k); err != nil {
			return 0, fmt.Errorf("delete key: %w", err)
		}
	}
	if err := wb.Flush(); err != nil {
		return 0, fmt.Errorf("flush deletes: %w", err)
	}

	retentionLog.Info().Int("deleted", len(deleteKeys)).Str("prefix", prefix).Str("cutoff", cutoffStr).Msg("Deleted expired keys")
	return len(deleteKeys), nil
}

// deleteByTimestampPrefix scans all keys with the given prefix and compares
// the last 12 characters as a YYYYMMDDHHmm timestamp for deletion.
// Used for intraday keys (iprice:{contract}:{interval}:{YYYYMMDDHHmm}).
func (d *DB) deleteByTimestampPrefix(prefix string, cutoff time.Time) (int, error) {
	cutoffStr := cutoff.UTC().Format("200601021504")
	var deleteKeys [][]byte

	err := d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = false

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			key := string(it.Item().Key())
			// Key format: iprice:CODE:INTERVAL:YYYYMMDDHHmm
			if len(key) >= 12 {
				tsPart := key[len(key)-12:]
				if tsPart < cutoffStr {
					deleteKeys = append(deleteKeys, it.Item().KeyCopy(nil))
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	if len(deleteKeys) == 0 {
		return 0, nil
	}

	wb := d.db.NewWriteBatch()
	defer wb.Cancel()
	for _, k := range deleteKeys {
		if err := wb.Delete(k); err != nil {
			return 0, fmt.Errorf("delete key: %w", err)
		}
	}
	if err := wb.Flush(); err != nil {
		return 0, fmt.Errorf("flush deletes: %w", err)
	}

	retentionLog.Info().Int("deleted", len(deleteKeys)).Str("prefix", prefix).Msg("Deleted expired intraday keys")
	return len(deleteKeys), nil
}
