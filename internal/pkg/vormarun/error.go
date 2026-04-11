package vormarun

type LoaderError struct {
	ClientMsg string
	Err       error
}

func (e *LoaderError) Error() string {
	if e == nil {
		return "unknown loader error"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.ClientMsg != "" {
		return e.ClientMsg
	}
	return "unknown loader error"
}
