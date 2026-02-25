package artifactio

import (
	"errors"
	"strings"
	"testing"
)

type fakeClosableResource struct {
	closeErr    error
	closeCalled bool
}

func (resource *fakeClosableResource) Close() error {
	resource.closeCalled = true
	return resource.closeErr
}

func TestRunWithClosableResource(t *testing.T) {
	t.Run("returns operation error when close succeeds", func(t *testing.T) {
		expectedErr := errors.New("operation failed")
		resource := &fakeClosableResource{}

		err := RunWithClosableResource(
			resource,
			"close resource",
			func(gotResource *fakeClosableResource) error {
				if gotResource != resource {
					t.Fatalf(
						"runWithResource received resource %p, want %p",
						gotResource,
						resource,
					)
				}
				return expectedErr
			},
		)
		if err == nil {
			t.Fatal(
				"expected RunWithClosableResource to return operation error",
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped operation error", err)
		}
		if !resource.closeCalled {
			t.Fatal("expected resource.Close to be called")
		}
	})

	t.Run("returns close error when operation succeeds", func(t *testing.T) {
		expectedErr := errors.New("close failed")
		resource := &fakeClosableResource{
			closeErr: expectedErr,
		}

		err := RunWithClosableResource(
			resource,
			"close resource",
			func(*fakeClosableResource) error {
				return nil
			},
		)
		if err == nil {
			t.Fatal("expected RunWithClosableResource to return close error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped close error", err)
		}
		if !strings.Contains(err.Error(), "close resource") {
			t.Fatalf("error = %q, expected close context", err)
		}
	})

	t.Run("joins operation and close errors", func(t *testing.T) {
		expectedOperationErr := errors.New("operation failed")
		expectedCloseErr := errors.New("close failed")
		resource := &fakeClosableResource{
			closeErr: expectedCloseErr,
		}

		err := RunWithClosableResource(
			resource,
			"close resource",
			func(*fakeClosableResource) error {
				return expectedOperationErr
			},
		)
		if err == nil {
			t.Fatal("expected RunWithClosableResource to return joined error")
		}
		if !errors.Is(err, expectedOperationErr) {
			t.Fatalf(
				"error = %v, expected operation error in joined chain",
				err,
			)
		}
		if !errors.Is(err, expectedCloseErr) {
			t.Fatalf("error = %v, expected close error in joined chain", err)
		}
		if !strings.Contains(err.Error(), "close resource") {
			t.Fatalf("error = %q, expected close context", err)
		}
	})

	t.Run("returns nil when operation and close succeed", func(t *testing.T) {
		resource := &fakeClosableResource{}
		err := RunWithClosableResource(
			resource,
			"close resource",
			func(*fakeClosableResource) error {
				return nil
			},
		)
		if err != nil {
			t.Fatalf("RunWithClosableResource returned error: %v", err)
		}
		if !resource.closeCalled {
			t.Fatal("expected resource.Close to be called")
		}
	})
}
