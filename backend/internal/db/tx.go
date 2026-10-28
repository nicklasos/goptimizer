package db

import (
	"context"
	"database/sql"
	"fmt"
)

type TxRunner struct {
	conn *sql.DB
}

func NewTxRunner(conn *sql.DB) *TxRunner {
	return &TxRunner{conn: conn}
}

// WithTx runs fn in a transaction: it commits when fn returns nil and rolls back otherwise.
func (r *TxRunner) WithTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := fn(New(tx)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
