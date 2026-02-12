package vormaruntime

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

func (e *LoaderError) Error() string {
	if e == nil {
		return "loader error"
	}
	if e.Server != nil {
		return e.Server.Error()
	}
	if e.Client != "" {
		return e.Client
	}
	return "loader error"
}

func (e *LoaderError) __isLoaderError()      {}
func (e *LoaderError) ClientMessage() string { return e.Client }
func (e *LoaderError) ServerError() error    { return e.Server }
