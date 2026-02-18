package sqlutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Transaction runs a function within a transaction.
func Transaction(db *sql.DB, f func(tx *sql.Tx) error) error {
	return TransactionContext(db, context.Background(), nil, f)
}

// TransactionContext runs a function within a transaction using the provided context and options.
func TransactionContext(
	db *sql.DB,
	ctx context.Context,
	opts *sql.TxOptions,
	f func(tx *sql.Tx) error,
) (transactionError error) {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if f == nil {
		return fmt.Errorf("transaction function is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	transaction, beginTransactionError := db.BeginTx(ctx, opts)
	if beginTransactionError != nil {
		return fmt.Errorf(
			"failed to begin transaction: %w",
			beginTransactionError,
		)
	}
	defer func() {
		if panicValue := recover(); panicValue != nil {
			_ = transaction.Rollback()
			panic(panicValue) // re-throw panic after Rollback
		}
		if transactionError != nil {
			rollbackError := transaction.Rollback()
			if rollbackError != nil && !errors.Is(rollbackError, sql.ErrTxDone) {
				transactionError = errors.Join(
					transactionError,
					fmt.Errorf("rollback transaction: %w", rollbackError),
				)
			}
			return
		}
		transactionError = transaction.Commit()
	}()
	transactionError = f(transaction)
	return transactionError
}
