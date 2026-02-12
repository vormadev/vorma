package vormabuild

import (
	"errors"
	"fmt"
)

type closableResource interface {
	Close() error
}

func runWithClosableResource[T closableResource](
	resource T,
	closeErrorContext string,
	runWithResource func(T) error,
) (operationErr error) {
	defer func() {
		operationErr = joinOperationErrorWithCloseError(
			operationErr,
			closeErrorContext,
			resource.Close(),
		)
	}()

	return runWithResource(resource)
}

func joinOperationErrorWithCloseError(
	operationErr error,
	closeErrorContext string,
	closeErr error,
) error {
	if closeErr == nil {
		return operationErr
	}

	closeErrWithContext := fmt.Errorf("%s: %w", closeErrorContext, closeErr)
	if operationErr == nil {
		return closeErrWithContext
	}
	return errors.Join(operationErr, closeErrWithContext)
}
