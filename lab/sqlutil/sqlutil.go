package sqlutil

import (
	"context"
	"database/sql"
	"fmt"
)

// Transaction runs a function within a transaction.
func Transaction(db *sql.DB, f func(tx *sql.Tx) error) error {
	return TransactionContext(db, context.Background(), nil, f)
}

// TransactionContext runs a function within a transaction using the provided context and options.
func TransactionContext(db *sql.DB, ctx context.Context, opts *sql.TxOptions, f func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			_ = tx.Rollback() // err is non-nil; don't change it
		} else {
			err = tx.Commit() // err is nil; if Commit returns error update err
		}
	}()
	err = f(tx)
	return err
}
