// Package validate provides a simple way to validate and parse data from HTTP requests.
package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func validateDestination(destStructPtr any) error {
	if destStructPtr == nil {
		return errors.New("destination is nil")
	}
	return nil
}

// JSONBodyInto decodes an HTTP request body into a struct and validates it.
func JSONBodyInto(r *http.Request, destStructPtr any) error {
	if r == nil {
		return &ValidationError{Err: errors.New("request is nil")}
	}
	if r.Body == nil {
		return &ValidationError{Err: errors.New("request body is nil")}
	}
	if err := validateDestination(destStructPtr); err != nil {
		return &ValidationError{Err: err}
	}
	if err := json.NewDecoder(r.Body).Decode(destStructPtr); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	if err := attemptValidation("validate.JSONBodyInto", destStructPtr); err != nil {
		return err
	}
	return nil
}

// JSONBytesInto decodes a byte slice containing JSON data into a struct and validates it.
func JSONBytesInto(data []byte, destStructPtr any) error {
	if err := validateDestination(destStructPtr); err != nil {
		return &ValidationError{Err: err}
	}
	if err := json.Unmarshal(data, destStructPtr); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	if err := attemptValidation("validate.JSONBytesInto", destStructPtr); err != nil {
		return err
	}
	return nil
}

// JSONStrInto decodes a string containing JSON data into a struct and validates it.
func JSONStrInto(data string, destStructPtr any) error {
	if err := validateDestination(destStructPtr); err != nil {
		return &ValidationError{Err: err}
	}
	if err := json.Unmarshal([]byte(data), destStructPtr); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	if err := attemptValidation("validate.JSONStrInto", destStructPtr); err != nil {
		return err
	}
	return nil
}

// URLSearchParamsInto parses the URL parameters of an HTTP request into a struct and validates it.
func URLSearchParamsInto(r *http.Request, destStructPtr any) error {
	if r == nil {
		return &ValidationError{Err: errors.New("request is nil")}
	}
	if r.URL == nil {
		return &ValidationError{Err: errors.New("request URL is nil")}
	}
	if err := validateDestination(destStructPtr); err != nil {
		return &ValidationError{Err: err}
	}
	if err := parseURLValues(r.URL.Query(), destStructPtr); err != nil {
		return &ValidationError{Err: fmt.Errorf("error parsing URL parameters: %w", err)}
	}
	if err := attemptValidation("validate.URLSearchParamsInto", destStructPtr); err != nil {
		return err
	}
	return nil
}
