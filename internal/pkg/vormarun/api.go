package vormarun

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"github.com/vormadev/vorma/kit/schema"
	"github.com/vormadev/vorma/kit/searchparams"
)

type FormData struct{}

func (m FormData) TSType() string { return "FormData" }

func parse_loader_input(r *http.Request, input_ptr any) error {
	return search_params_into_struct(r, input_ptr)
}

func parse_api_input(r *http.Request, input_ptr any) error {
	if r == nil {
		return &schema.ValidationError{Err: errors.New("request is nil")}
	}
	if input_ptr == nil {
		return &schema.ValidationError{Err: errors.New("destination is nil")}
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return search_params_into_struct(r, input_ptr)
	}
	content_type, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if content_type == "application/x-www-form-urlencoded" ||
		content_type == "multipart/form-data" {
		if _, ok := input_ptr.(*FormData); ok {
			return nil
		}
		return &schema.ValidationError{
			Err: fmt.Errorf("form content type required FormData input"),
		}
	}
	if r.Body == nil {
		return &schema.ValidationError{Err: errors.New("request body is nil")}
	}
	if err := json.NewDecoder(r.Body).Decode(input_ptr); err != nil {
		return &schema.ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	_, err := schema.EnforceAny("validate.JSONBodyInto", input_ptr)
	return err
}

func search_params_into_struct(r *http.Request, dest_struct_ptr any) error {
	if r == nil {
		return &schema.ValidationError{Err: errors.New("request is nil")}
	}
	if r.URL == nil {
		return &schema.ValidationError{Err: errors.New("request URL is nil")}
	}
	if dest_struct_ptr == nil {
		return &schema.ValidationError{Err: errors.New("destination is nil")}
	}
	if err := searchparams.ParseIntoStructPtr(r, dest_struct_ptr); err != nil {
		return &schema.ValidationError{
			Err: fmt.Errorf("error parsing URL parameters: %w", err),
		}
	}
	_, err := schema.EnforceAny("validate.URLSearchParamsInto", dest_struct_ptr)
	return err
}
