package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ImpactRepo implements persistence for EventImpact records using BadgerDB.
type ImpactRepo struct {
	db *badger.DB
}

// NewImpactRepo creates a new ImpactRepo backed by the given DB.
func NewImpactRepo(db *DB) *ImpactRepo {
	return &ImpactRepo{db: db.Badger()}
}

// --- Key builders ---

// impactKey: evtimp:{currency}:{event_name_normalized}:{horizon}:{YYYYMMDD}
func impactKey(currency, eventTitle, horizon string, date time.Time) []byte {
	name := normalizeEventName(eventTitle)
	return []byte(fmt.Sprintf("evtimp:%s:%s:%s:%s", currency, name, horizon, date.Format("20060102")))
}

// impactPrefixEvent: evtimp:{currency}:{event_name_normalized}:
func impactPrefixEvent(currency, eventTitle string) []byte {
	name := normalizeEventName(eventTitle)
	return []byte(fmt.Sprintf("evtimp:%s:%s:", currency, name))
}

// impactPrefixAll: evtimp:
func impactPrefixAll() []byte {
	return []byte("evtimp:")
}

func normalizeEventName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

// --- ImpactRepo methods ---

// SaveEventImpact persists a single EventImpact record.
func (r *ImpactRepo) SaveEventImpact(_ context.Context, impact domain.EventImpact) error {
	data, err := json.Marshal(&impact)
	if err != nil {
		return fmt.Errorf("marshal event impact: %w", err)
	}

	key := impactKey(impact.Currency, impact.EventTitle, impact.TimeHorizon, impact.Timestamp)
	err = r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
	if err != nil {
		return fmt.Errorf("save event impact: %w", err)
	}
	return nil
}

// GetEventImpacts retrieves all impact records for a specific event and currency.
func (r *ImpactRepo) GetEventImpacts(_ context.Context, eventTitle, currency string) ([]domain.EventImpact, error) {
	var impacts []domain.EventImpact

	prefix := impactPrefixEvent(currency, eventTitle)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 50

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var imp domain.EventImpact
				if err := json.Unmarshal(val, &imp); err != nil {
					return err
				}
				impacts = append(impacts, imp)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read event impact: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get event impacts %s/%s: %w", currency, eventTitle, err)
	}
	return impacts, nil
}

// GetEventImpactSummary computes aggregated impact summaries by sigma bucket
// for a given event title across all currencies and time horizons.
func (r *ImpactRepo) GetEventImpactSummary(_ context.Context, eventTitle string) ([]domain.EventImpactSummary, error) {
	// Collect all impacts matching this event title across currencies
	var allImpacts []domain.EventImpact

	prefix := impactPrefixAll()

	normalizedTarget := normalizeEventName(eventTitle)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true
		opts.PrefetchSize = 100

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			// Key format: evtimp:{currency}:{event_name}:{horizon}:{date}
			keyStr := string(item.Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 5 {
				continue
			}
			// Check if the event name matches
			if parts[2] != normalizedTarget {
				continue
			}

			err := item.Value(func(val []byte) error {
				var imp domain.EventImpact
				if err := json.Unmarshal(val, &imp); err != nil {
					return err
				}
				allImpacts = append(allImpacts, imp)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read impact for summary: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get event impact summary %s: %w", eventTitle, err)
	}

	if len(allImpacts) == 0 {
		return nil, nil
	}

	// Group by: currency + sigma_bucket + time_horizon
	type groupKey struct {
		Currency    string
		SigmaBucket string
	}
	type groupData struct {
		pipsChanges []float64
		pctChanges  []float64
	}

	groups := make(map[groupKey]*groupData)

	for _, imp := range allImpacts {
		// Only use 1h horizon for summary to keep it concise
		if imp.TimeHorizon != "1h" {
			continue
		}
		gk := groupKey{Currency: imp.Currency, SigmaBucket: imp.SigmaLevel}
		if groups[gk] == nil {
			groups[gk] = &groupData{}
		}
		groups[gk].pipsChanges = append(groups[gk].pipsChanges, imp.PriceChange)
		groups[gk].pctChanges = append(groups[gk].pctChanges, imp.PctChange)
	}

	var summaries []domain.EventImpactSummary
	for gk, gd := range groups {
		n := len(gd.pipsChanges)
		if n == 0 {
			continue
		}

		// Average
		var sumPips, sumPct float64
		for i := 0; i < n; i++ {
			sumPips += gd.pipsChanges[i]
			sumPct += gd.pctChanges[i]
		}

		// Median
		sorted := make([]float64, n)
		copy(sorted, gd.pipsChanges)
		sort.Float64s(sorted)
		var median float64
		if n%2 == 0 {
			median = (sorted[n/2-1] + sorted[n/2]) / 2
		} else {
			median = sorted[n/2]
		}

		summaries = append(summaries, domain.EventImpactSummary{
			EventTitle:         eventTitle,
			Currency:           gk.Currency,
			SigmaBucket:        gk.SigmaBucket,
			AvgPriceImpactPips: math.Round(sumPips/float64(n)*10) / 10,
			AvgPctChange:       math.Round(sumPct/float64(n)*1000) / 1000,
			Occurrences:        n,
			MedianImpact:       math.Round(median*10) / 10,
		})
	}

	// Sort by sigma bucket order for consistent display
	bucketOrder := map[string]int{
		">+2\u03c3": 0, "+1\u03c3 to +2\u03c3": 1, "-1\u03c3 to +1\u03c3": 2,
		"-1\u03c3 to -2\u03c3": 3, "<-2\u03c3": 4,
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Currency != summaries[j].Currency {
			return summaries[i].Currency < summaries[j].Currency
		}
		return bucketOrder[summaries[i].SigmaBucket] < bucketOrder[summaries[j].SigmaBucket]
	})

	return summaries, nil
}

// GetTrackedEvents returns a deduplicated list of event titles that have impact records.
func (r *ImpactRepo) GetTrackedEvents(_ context.Context) ([]string, error) {
	seen := make(map[string]bool)
	prefix := impactPrefixAll()

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = false // keys only

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			// Key format: evtimp:{currency}:{event_name}:{horizon}:{date}
			parts := strings.Split(string(it.Item().Key()), ":")
			if len(parts) >= 3 {
				seen[parts[2]] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get tracked events: %w", err)
	}

	var events []string
	for name := range seen {
		// Denormalize: dashes to spaces, then title-case each word.
		display := strings.ReplaceAll(name, "-", " ")
		display = titleCase(display)
		events = append(events, display)
	}
	sort.Strings(events)
	return events, nil
}

// titleCase capitalises the first letter of each word.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
