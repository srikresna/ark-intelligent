package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// PortfolioRepo implements portfolio position storage using BadgerDB.
type PortfolioRepo struct {
	db *badger.DB
}

// NewPortfolioRepo creates a new PortfolioRepo backed by the given DB.
func NewPortfolioRepo(db *DB) *PortfolioRepo {
	return &PortfolioRepo{db: db.Badger()}
}

// --- Key builders ---

func portfolioKey(userID int64, currency string) []byte {
	return []byte(fmt.Sprintf("portfolio:%d:%s", userID, strings.ToUpper(currency)))
}

func portfolioPrefix(userID int64) []byte {
	return []byte(fmt.Sprintf("portfolio:%d:", userID))
}

// --- Portfolio storage operations ---

// SavePosition persists a single position for a user.
// If the currency already exists, it is overwritten (upsert).
func (r *PortfolioRepo) SavePosition(_ context.Context, userID int64, pos domain.Position) error {
	data, err := json.Marshal(&pos)
	if err != nil {
		return fmt.Errorf("marshal position: %w", err)
	}

	key := portfolioKey(userID, pos.Currency)
	err = r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
	if err != nil {
		return fmt.Errorf("save position %s for user %d: %w", pos.Currency, userID, err)
	}
	return nil
}

// GetPositions retrieves all positions for a user.
func (r *PortfolioRepo) GetPositions(_ context.Context, userID int64) ([]domain.Position, error) {
	var positions []domain.Position
	prefix := portfolioPrefix(userID)

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var pos domain.Position
				if err := json.Unmarshal(val, &pos); err != nil {
					return err
				}
				positions = append(positions, pos)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read position: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get positions for user %d: %w", userID, err)
	}
	return positions, nil
}

// RemovePosition removes a single position by currency for a user.
func (r *PortfolioRepo) RemovePosition(_ context.Context, userID int64, currency string) error {
	key := portfolioKey(userID, currency)
	err := r.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
	if err == badger.ErrKeyNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove position %s for user %d: %w", currency, userID, err)
	}
	return nil
}

// ClearPortfolio removes all positions for a user.
func (r *PortfolioRepo) ClearPortfolio(_ context.Context, userID int64) error {
	prefix := portfolioPrefix(userID)

	// Collect keys to delete.
	var keys [][]byte
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = false

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keys = append(keys, key)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan portfolio keys for user %d: %w", userID, err)
	}

	if len(keys) == 0 {
		return nil
	}

	wb := r.db.NewWriteBatch()
	defer wb.Cancel()

	for _, key := range keys {
		if err := wb.Delete(key); err != nil {
			return fmt.Errorf("batch delete portfolio key: %w", err)
		}
	}
	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush portfolio clear for user %d: %w", userID, err)
	}
	return nil
}
