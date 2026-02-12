package vormabuild

import (
	"errors"
	"strings"
	"testing"
)

func TestRunWithRollbackOnFailureAndPanic(t *testing.T) {
	t.Run("returns error when run step is missing", func(t *testing.T) {
		err := runWithRollbackOnFailureAndPanic(rollbackTransactionOptions{})
		if err == nil {
			t.Fatal("expected runWithRollbackOnFailureAndPanic to return error")
		}
		if !strings.Contains(err.Error(), "run step is required") {
			t.Fatalf("error = %q, expected missing-run-step context", err)
		}
	})

	t.Run("returns nil when run succeeds", func(t *testing.T) {
		rollbackCalled := false
		err := runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					return nil
				},
				rollbackOnFailure: func() error {
					rollbackCalled = true
					return nil
				},
			},
		)
		if err != nil {
			t.Fatalf("runWithRollbackOnFailureAndPanic returned error: %v", err)
		}
		if rollbackCalled {
			t.Fatal("did not expect rollback step when run succeeds")
		}
	})

	t.Run("returns run error when rollback succeeds", func(t *testing.T) {
		expectedErr := errors.New("run failed")
		rollbackCalled := false
		err := runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					return expectedErr
				},
				rollbackOnFailure: func() error {
					rollbackCalled = true
					return nil
				},
			},
		)
		if err == nil {
			t.Fatal("expected runWithRollbackOnFailureAndPanic to return run error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped run error", err)
		}
		if !rollbackCalled {
			t.Fatal("expected rollback step after run error")
		}
	})

	t.Run("returns run error when rollback step is not configured", func(t *testing.T) {
		expectedErr := errors.New("run failed")
		err := runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					return expectedErr
				},
				rollbackOnFailure: nil,
			},
		)
		if err == nil {
			t.Fatal("expected runWithRollbackOnFailureAndPanic to return run error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped run error", err)
		}
	})

	t.Run("joins run and rollback errors with rollback context", func(t *testing.T) {
		expectedRunErr := errors.New("run failed")
		expectedRollbackErr := errors.New("rollback failed")
		err := runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					return expectedRunErr
				},
				rollbackOnFailure: func() error {
					return expectedRollbackErr
				},
				rollbackErrorContext: "restore state",
			},
		)
		if err == nil {
			t.Fatal("expected runWithRollbackOnFailureAndPanic to return joined error")
		}
		if !errors.Is(err, expectedRunErr) {
			t.Fatalf("error = %v, expected run error in joined chain", err)
		}
		if !errors.Is(err, expectedRollbackErr) {
			t.Fatalf("error = %v, expected rollback error in joined chain", err)
		}
		if !strings.Contains(err.Error(), "restore state") {
			t.Fatalf("error = %q, expected rollback context", err)
		}
	})

	t.Run("re-panics after panic and runs rollback", func(t *testing.T) {
		expectedPanic := errors.New("run panic")
		rollbackCalled := false
		defer func() {
			recoveredPanicValue := recover()
			if recoveredPanicValue == nil {
				t.Fatal("expected runWithRollbackOnFailureAndPanic to panic")
			}
			recoveredPanicErr, ok := recoveredPanicValue.(error)
			if !ok {
				t.Fatalf("recovered panic type = %T, want error", recoveredPanicValue)
			}
			if !errors.Is(recoveredPanicErr, expectedPanic) {
				t.Fatalf("recovered panic = %v, want %v", recoveredPanicErr, expectedPanic)
			}
			if !rollbackCalled {
				t.Fatal("expected rollback step after panic")
			}
		}()

		_ = runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					panic(expectedPanic)
				},
				rollbackOnFailure: func() error {
					rollbackCalled = true
					return nil
				},
			},
		)
	})

	t.Run("logs rollback failure after panic and re-panics", func(t *testing.T) {
		expectedPanic := errors.New("run panic")
		expectedRollbackErr := errors.New("rollback failed")
		var loggedRollbackErr error
		defer func() {
			recoveredPanicValue := recover()
			if recoveredPanicValue == nil {
				t.Fatal("expected runWithRollbackOnFailureAndPanic to panic")
			}
			recoveredPanicErr, ok := recoveredPanicValue.(error)
			if !ok {
				t.Fatalf("recovered panic type = %T, want error", recoveredPanicValue)
			}
			if !errors.Is(recoveredPanicErr, expectedPanic) {
				t.Fatalf("recovered panic = %v, want %v", recoveredPanicErr, expectedPanic)
			}
			if !errors.Is(loggedRollbackErr, expectedRollbackErr) {
				t.Fatalf("logged rollback error = %v, want %v", loggedRollbackErr, expectedRollbackErr)
			}
		}()

		_ = runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					panic(expectedPanic)
				},
				rollbackOnFailure: func() error {
					return expectedRollbackErr
				},
				logRollbackFailureAfterPanic: func(rollbackErr error) {
					loggedRollbackErr = rollbackErr
				},
			},
		)
	})

	t.Run("panics with rollback panic when run returns error and rollback panics", func(t *testing.T) {
		expectedRollbackPanic := errors.New("rollback panic")
		defer func() {
			recoveredPanicValue := recover()
			if recoveredPanicValue == nil {
				t.Fatal("expected runWithRollbackOnFailureAndPanic to panic")
			}
			recoveredPanicErr, ok := recoveredPanicValue.(error)
			if !ok {
				t.Fatalf("recovered panic type = %T, want error", recoveredPanicValue)
			}
			if !errors.Is(recoveredPanicErr, expectedRollbackPanic) {
				t.Fatalf("recovered panic = %v, want %v", recoveredPanicErr, expectedRollbackPanic)
			}
		}()

		_ = runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					return errors.New("run failed")
				},
				rollbackOnFailure: func() error {
					panic(expectedRollbackPanic)
				},
			},
		)
	})

	t.Run("preserves original run panic when rollback panics", func(t *testing.T) {
		expectedRunPanic := errors.New("run panic")
		expectedRollbackPanic := errors.New("rollback panic")
		var loggedRollbackErr error
		defer func() {
			recoveredPanicValue := recover()
			if recoveredPanicValue == nil {
				t.Fatal("expected runWithRollbackOnFailureAndPanic to panic")
			}
			recoveredPanicErr, ok := recoveredPanicValue.(error)
			if !ok {
				t.Fatalf("recovered panic type = %T, want error", recoveredPanicValue)
			}
			if !errors.Is(recoveredPanicErr, expectedRunPanic) {
				t.Fatalf("recovered panic = %v, want %v", recoveredPanicErr, expectedRunPanic)
			}
			if loggedRollbackErr == nil {
				t.Fatal("expected rollback panic to be logged")
			}
			if !strings.Contains(loggedRollbackErr.Error(), "rollback panic") {
				t.Fatalf("logged rollback error = %q, expected rollback panic context", loggedRollbackErr)
			}
			if !strings.Contains(loggedRollbackErr.Error(), expectedRollbackPanic.Error()) {
				t.Fatalf("logged rollback error = %q, expected original rollback panic text", loggedRollbackErr)
			}
		}()

		_ = runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					panic(expectedRunPanic)
				},
				rollbackOnFailure: func() error {
					panic(expectedRollbackPanic)
				},
				logRollbackFailureAfterPanic: func(rollbackErr error) {
					loggedRollbackErr = rollbackErr
				},
			},
		)
	})
}
