package sqlutil

import (
	"context"
	"database/sql"
	"strings"
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
