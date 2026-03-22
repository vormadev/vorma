package sqlutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestTransactionContext_NilDB(t *testing.T) {
	err := TransactionContext(nil, context.Background(), nil, func(tx *sql.Tx) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for nil db")
	}
	if !strings.Contains(err.Error(), "db is nil") {
		t.Fatalf("expected nil-db error, got %v", err)
	}
}

func TestTransactionContext_NilRunner(t *testing.T) {
	closedDB := &sql.DB{}
	err := TransactionContext(closedDB, context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil transaction function")
	}
	if !strings.Contains(err.Error(), "transaction function is nil") {
		t.Fatalf("expected nil-function error, got %v", err)
	}
}

var (
	errCommitFailureSentinel           = errors.New("commit failed")
	registerCommitFailureDriverOnce    sync.Once
	commitFailureDriverRegistrationKey = "sqlutil_test_commit_failure_driver"
	errRollbackFailureSentinel         = errors.New("rollback failed")
	errTransactionRunSentinel          = errors.New("transaction run failed")
	registerRollbackFailureDriverOnce  sync.Once
	rollbackFailureDriverKey           = "sqlutil_test_rollback_failure_driver"
)

type commitFailureDriver struct{}

func (commitFailureDriver) Open(string) (driver.Conn, error) {
	return commitFailureConnection{}, nil
}

type commitFailureConnection struct{}

func (commitFailureConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported in this test driver")
}

func (commitFailureConnection) Close() error {
	return nil
}

func (commitFailureConnection) Begin() (driver.Tx, error) {
	return commitFailureTx{}, nil
}

func (commitFailureConnection) BeginTx(
	context.Context,
	driver.TxOptions,
) (driver.Tx, error) {
	return commitFailureTx{}, nil
}

type commitFailureTx struct{}

func (commitFailureTx) Commit() error {
	return errCommitFailureSentinel
}

func (commitFailureTx) Rollback() error {
	return nil
}

func openCommitFailureDB(t *testing.T) *sql.DB {
	t.Helper()
	registerCommitFailureDriverOnce.Do(func() {
		sql.Register(commitFailureDriverRegistrationKey, commitFailureDriver{})
	})
	db, err := sql.Open(commitFailureDriverRegistrationKey, "")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func TestTransactionContext_ReturnsCommitError(t *testing.T) {
	db := openCommitFailureDB(t)
	defer db.Close()

	err := TransactionContext(
		db,
		context.Background(),
		nil,
		func(tx *sql.Tx) error {
			return nil
		},
	)
	if err == nil {
		t.Fatal("expected commit error, got nil")
	}
	if !errors.Is(err, errCommitFailureSentinel) {
		t.Fatalf("expected commit failure error, got %v", err)
	}
}

type rollbackFailureDriver struct{}

func (rollbackFailureDriver) Open(string) (driver.Conn, error) {
	return rollbackFailureConnection{}, nil
}

type rollbackFailureConnection struct{}

func (rollbackFailureConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported in this test driver")
}

func (rollbackFailureConnection) Close() error {
	return nil
}

func (rollbackFailureConnection) Begin() (driver.Tx, error) {
	return rollbackFailureTx{}, nil
}

func (rollbackFailureConnection) BeginTx(
	context.Context,
	driver.TxOptions,
) (driver.Tx, error) {
	return rollbackFailureTx{}, nil
}

type rollbackFailureTx struct{}

func (rollbackFailureTx) Commit() error {
	return nil
}

func (rollbackFailureTx) Rollback() error {
	return errRollbackFailureSentinel
}

func openRollbackFailureDB(t *testing.T) *sql.DB {
	t.Helper()
	registerRollbackFailureDriverOnce.Do(func() {
		sql.Register(rollbackFailureDriverKey, rollbackFailureDriver{})
	})
	db, err := sql.Open(rollbackFailureDriverKey, "")
	if err != nil {
		t.Fatalf("failed to open rollback-failure test db: %v", err)
	}
	return db
}

func TestTransactionContext_JoinsRollbackErrorWithOperationError(t *testing.T) {
	db := openRollbackFailureDB(t)
	defer db.Close()

	err := TransactionContext(
		db,
		context.Background(),
		nil,
		func(tx *sql.Tx) error {
			return errTransactionRunSentinel
		},
	)
	if err == nil {
		t.Fatal("expected transaction error, got nil")
	}
	if !errors.Is(err, errTransactionRunSentinel) {
		t.Fatalf("expected transaction run error, got %v", err)
	}
	if !errors.Is(err, errRollbackFailureSentinel) {
		t.Fatalf("expected rollback error to be included, got %v", err)
	}
}
