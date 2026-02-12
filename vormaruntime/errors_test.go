package vormaruntime

import (
	"errors"
	"testing"
)

func TestLoaderErrorMarkerContract(t *testing.T) {
	serverErr := errors.New("server exploded")
	err := &LoaderError{
		Client: "Something went wrong",
		Server: serverErr,
	}

	// Explicit marker invocation validates the marker contract method exists.
	err.__isLoaderError()

	if got, want := err.ClientMessage(), "Something went wrong"; got != want {
		t.Fatalf("ClientMessage() = %q, want %q", got, want)
	}
	if got := err.ServerError(); !errors.Is(got, serverErr) {
		t.Fatalf("ServerError() = %v, want wrapped %v", got, serverErr)
	}
	if got, want := err.Error(), "server exploded"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestLoaderErrorError_NilServerFallsBackToClientMessage(t *testing.T) {
	err := &LoaderError{
		Client: "Something went wrong",
		Server: nil,
	}

	if got, want := err.Error(), "Something went wrong"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestLoaderErrorError_NoServerOrClientUsesSafeFallback(t *testing.T) {
	err := &LoaderError{}

	if got, want := err.Error(), "loader error"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestLoaderErrorError_NilReceiverUsesSafeFallback(t *testing.T) {
	var err *LoaderError

	if got, want := err.Error(), "loader error"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
