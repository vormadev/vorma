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
