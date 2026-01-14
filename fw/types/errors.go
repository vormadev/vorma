package types

// LoaderErrorMarker is an interface for loader errors that can provide
// separate client and server error messages.
type LoaderErrorMarker interface {
	__isLoaderError()
	ClientMessage() string
	ServerError() error
}

// LoaderError represents an error from a loader with separate client
// and server error messages.
type LoaderError struct {
	Client string
	Server error
}

func (e *LoaderError) Error() string         { return e.Server.Error() }
func (e *LoaderError) __isLoaderError()      {}
func (e *LoaderError) ClientMessage() string { return e.Client }
func (e *LoaderError) ServerError() error    { return e.Server }
