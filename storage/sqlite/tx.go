// storage/sqlite/tx.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/odysseythink/hermind/storage"
)

// WithTx runs fn inside a single SQL transaction. Committed on nil
// return, rolled back on error or panic. Panics are re-raised after
// rollback.
func (s *Store) WithTx(ctx context.Context, fn func(tx storage.Tx) error) error {
	sqlTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}

	txWrapper := &txImpl{tx: sqlTx}

	defer func() {
		if r := recover(); r != nil {
			_ = sqlTx.Rollback()
			panic(r)
		}
	}()

	if err := fn(txWrapper); err != nil {
		if rbErr := sqlTx.Rollback(); rbErr != nil {
			return fmt.Errorf("sqlite: rollback after error %v: %w", err, rbErr)
		}
		return err
	}
	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

// txImpl implements storage.Tx by wrapping a *sql.Tx.
type txImpl struct {
	tx *sql.Tx
}

var _ storage.Tx = (*txImpl)(nil)
