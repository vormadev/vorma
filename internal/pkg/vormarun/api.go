package vormarun

import (
	"fmt"
	"mime"
	"net/http"

	"github.com/vormadev/vorma/kit/validate"
)

type FormData struct{}

func (m FormData) TSType() string { return "FormData" }

func parse_action_input(r *http.Request, input_ptr any) error {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return validate.URLSearchParamsInto(r, input_ptr)
	}
	content_type, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if content_type == "application/x-www-form-urlencoded" ||
		content_type == "multipart/form-data" {
		if _, ok := input_ptr.(*FormData); ok {
			return nil
		}
		return &validate.ValidationError{
			Err: fmt.Errorf("form content type required FormData input"),
		}
	}
	return validate.JSONBodyInto(r, input_ptr)
}
