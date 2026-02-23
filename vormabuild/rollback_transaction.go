package vormabuild

import (
	"errors"
	"fmt"
)

type rollbackTransactionOptions struct {
	run                          func() error
	rollbackOnFailure            func() error
	rollbackErrorContext         string
	logRollbackFailureAfterPanic func(error)
}

func runWithRollbackOnFailureAndPanic(
	options rollbackTransactionOptions,
) (operationErr error) {
	if options.run == nil {
		return errors.New("rollback transaction run step is required")
	}

	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue == nil {
			return
		}

		rollbackPanicValue, rollbackErr := runRollbackIfConfigured(
			options.rollbackOnFailure,
		)
		if rollbackErr != nil && options.logRollbackFailureAfterPanic != nil {
			options.logRollbackFailureAfterPanic(rollbackErr)
		}
		if rollbackPanicValue != nil &&
			options.logRollbackFailureAfterPanic != nil {
			options.logRollbackFailureAfterPanic(
				fmt.Errorf("rollback panic: %v", rollbackPanicValue),
			)
		}

		panic(recoveredPanicValue)
	}()

	operationErr = options.run()
	if operationErr == nil {
		return nil
	}

	rollbackPanicValue, rollbackErr := runRollbackIfConfigured(
		options.rollbackOnFailure,
	)
	if rollbackPanicValue != nil {
		panic(rollbackPanicValue)
	}
	if rollbackErr == nil {
		return operationErr
	}

	if options.rollbackErrorContext != "" {
		rollbackErr = fmt.Errorf(
			"%s: %w",
			options.rollbackErrorContext,
			rollbackErr,
		)
	}

	return errors.Join(operationErr, rollbackErr)
}

func runRollbackIfConfigured(
	rollbackStep func() error,
) (rollbackPanicValue any, rollbackErr error) {
	if rollbackStep == nil {
		return nil, nil
	}

	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue != nil {
			rollbackPanicValue = recoveredPanicValue
		}
	}()

	rollbackErr = rollbackStep()
	return nil, rollbackErr
}
