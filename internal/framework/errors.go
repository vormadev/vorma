package vorma

type LoaderErrorMarker interface {
	__isLoaderError()
	ClientMessage() string
	ServerError() error
}

type LoaderError struct {
	Client string
	Server error
}

func (e *LoaderError) Error() string         { return e.Server.Error() }
func (e *LoaderError) __isLoaderError()      {}
func (e *LoaderError) ClientMessage() string { return e.Client }
func (e *LoaderError) ServerError() error    { return e.Server }
