package errutil

import (
	"errors"
	"fmt"
	"strings"
)

type Err struct {
	Wrapper error
	Cause   error
	Prefix  string
}

func (e Err) Error() string {
	var b strings.Builder
	if e.Wrapper != nil {
		b.WriteString(e.Wrapper.Error())
	}
	if e.Prefix != "" {
		if b.Len() > 0 {
			b.WriteString(": ")
		}
		b.WriteString(e.Prefix)
	}
	if e.Cause != nil && e.Cause != e.Wrapper {
		if b.Len() > 0 {
			b.WriteString(": ")
		}
		b.WriteString(e.Cause.Error())
	}
	return b.String()
}

func (e Err) Unwrap() []error {
	if e.Cause == e.Wrapper {
		return []error{e.Wrapper}
	}
	return []error{e.Wrapper, e.Cause}
}

func Maybe(msg string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func ToIsErrFunc(targetErr error) func(error) bool {
	return func(err error) bool { return errors.Is(err, targetErr) }
}
