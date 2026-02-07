package validate

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type TestStruct struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=18"`
}

func (t *TestStruct) Validate() error {
	if t.Name == "" {
		return errors.New("name is required")
	}
	if t.Email == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(t.Email, "@") {
		return errors.New("email is invalid")
	}
	if t.Age < 18 {
		return errors.New("age must be at least 18")
	}
	return nil
}

func TestJSONBodyInto(t *testing.T) {
	// Test with valid JSON
	validJSON := `{"name": "John", "email": "john@example.com", "age": 30}`
	r := &http.Request{Body: io.NopCloser(strings.NewReader(validJSON))}
	dest := &TestStruct{}
	if err := JSONBodyInto(r, dest); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if dest.Name != "John" || dest.Email != "john@example.com" || dest.Age != 30 {
		t.Error("unexpected values in struct after decoding")
	}

	// Test with invalid JSON
	invalidJSON := `{"name": "John", "email": "john@example.com"`
	r = &http.Request{Body: io.NopCloser(strings.NewReader(invalidJSON))}
	dest = &TestStruct{}
	err := JSONBodyInto(r, dest)
	if err == nil || !strings.Contains(err.Error(), "error decoding JSON") {
		t.Errorf("expected decoding error, got %v", err)
	}

	// Test with missing required fields
	missingFieldsJSON := `{"name": "John"}`
	r = &http.Request{Body: io.NopCloser(strings.NewReader(missingFieldsJSON))}
	dest = &TestStruct{}
	err = JSONBodyInto(r, dest)
	if err == nil {
		t.Errorf("expected error, got %v", err)
	}
}

func TestJSONBytesInto(t *testing.T) {
	// Test with valid JSON
	validJSON := []byte(`{"name": "John", "email": "john@example.com", "age": 30}`)
	dest := &TestStruct{}
	if err := JSONBytesInto(validJSON, dest); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if dest.Name != "John" || dest.Email != "john@example.com" || dest.Age != 30 {
		t.Error("unexpected values in struct after decoding")
	}

	// Test with invalid JSON
	invalidJSON := []byte(`{"name": "John", "email": "john@example.com"`)
	dest = &TestStruct{}
	err := JSONBytesInto(invalidJSON, dest)
	if err == nil || !strings.Contains(err.Error(), "error decoding JSON") {
		t.Errorf("expected decoding error, got %v", err)
	}

	// Test with missing required fields
	missingFieldsJSON := []byte(`{"name": "John"}`)
	dest = &TestStruct{}
	err = JSONBytesInto(missingFieldsJSON, dest)
	if err == nil {
		t.Errorf("expected error, got %v", err)
	}
}

func TestJSONStrInto(t *testing.T) {
	// Test with valid JSON
	validJSON := `{"name": "John", "email": "john@example.com", "age": 30}`
	dest := &TestStruct{}
	if err := JSONStrInto(validJSON, dest); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if dest.Name != "John" || dest.Email != "john@example.com" || dest.Age != 30 {
		t.Error("unexpected values in struct after decoding")
	}

	// Test with invalid JSON
	invalidJSON := `{"name": "John", "email": "john@example.com"`
	dest = &TestStruct{}
	err := JSONStrInto(invalidJSON, dest)
	if err == nil || !strings.Contains(err.Error(), "error decoding JSON") {
		t.Errorf("expected decoding error, got %v", err)
	}

	// Test with missing required fields
	missingFieldsJSON := `{"name": "John"}`
	dest = &TestStruct{}
	err = JSONStrInto(missingFieldsJSON, dest)
	if err == nil {
		t.Errorf("expected error, got %v", err)
	}
}

func TestURLSearchParamsIntoHighLevel(t *testing.T) {
	// Test with valid URL parameters
	urlParams := url.Values{}
	urlParams.Add("name", "John")
	urlParams.Add("email", "john@example.com")
	urlParams.Add("age", "30")
	r := &http.Request{URL: &url.URL{RawQuery: urlParams.Encode()}}
	dest := &TestStruct{}
	if err := URLSearchParamsInto(r, dest); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if dest.Name != "John" || dest.Email != "john@example.com" || dest.Age != 30 {
		t.Error("unexpected values in struct after parsing URL parameters")
	}

	// Test with missing required fields
	urlParams = url.Values{}
	urlParams.Add("name", "John")
	r = &http.Request{URL: &url.URL{RawQuery: urlParams.Encode()}}
	dest = &TestStruct{}
	err := URLSearchParamsInto(r, dest)
	if err == nil {
		t.Errorf("expected error, got %v", err)
	}
}

func TestEdgeCases(t *testing.T) {
	// Test with empty JSON
	emptyJSON := `{}`
	dest := &TestStruct{}
	err := JSONStrInto(emptyJSON, dest)
	if err == nil {
		t.Errorf("expected error, got %v", err)
	}

	// Test with unexpected field type
	wrongTypeJSON := `{"name": "John", "email": "john@example.com", "age": "not a number"}`
	err = JSONStrInto(wrongTypeJSON, dest)
	if err == nil || !strings.Contains(err.Error(), "error decoding JSON") {
		t.Errorf("expected decoding error, got %v", err)
	}

	// Test with large payload
	largePayload := strings.Repeat(`{"name": "John", "email": "john@example.com", "age": 30}`, 10000)
	dest = &TestStruct{}
	err = JSONStrInto(largePayload, dest)
	if err == nil {
		t.Errorf("expected error due to large payload, got nil")
	}

	// Test with special characters in JSON
	specialCharJSON := `{"name": "J@hn!$#", "email": "john@example.com", "age": 30}`
	err = JSONStrInto(specialCharJSON, dest)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if dest.Name != "J@hn!$#" {
		t.Errorf("unexpected name value, got %s", dest.Name)
	}
}

func TestHighLevelNilGuards(t *testing.T) {
	dest := &TestStruct{}

	if err := JSONBodyInto(nil, dest); err == nil {
		t.Fatal("expected error for nil request in JSONBodyInto")
	}

	if err := JSONBodyInto(&http.Request{}, dest); err == nil {
		t.Fatal("expected error for nil body in JSONBodyInto")
	}

	if err := JSONBytesInto([]byte(`{}`), nil); err == nil {
		t.Fatal("expected error for nil destination in JSONBytesInto")
	}

	if err := JSONStrInto(`{}`, nil); err == nil {
		t.Fatal("expected error for nil destination in JSONStrInto")
	}

	if err := URLSearchParamsInto(nil, dest); err == nil {
		t.Fatal("expected error for nil request in URLSearchParamsInto")
	}

	if err := URLSearchParamsInto(&http.Request{}, dest); err == nil {
		t.Fatal("expected error for nil URL in URLSearchParamsInto")
	}

	if err := URLSearchParamsInto(&http.Request{URL: &url.URL{}}, nil); err == nil {
		t.Fatal("expected error for nil destination in URLSearchParamsInto")
	}
}
