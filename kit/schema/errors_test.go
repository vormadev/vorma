package schema_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type simple_validation_failure struct {
	Username string
	Password string
}

func (simple_validation_failure) Schema() schema.Schema {
	return schema.Object{
		"Username": schema.String{MustNotBeZero: true, MinLen: 3},
		"Password": schema.String{MustNotBeZero: true, MinLen: 8},
	}
}

type simple_schema_failure struct {
	Name string
}

func (simple_schema_failure) Schema() schema.Schema {
	return schema.Object{
		"Nmae": schema.String{MustNotBeZero: true},
	}
}

type both_errors_type struct {
	Name string
}

func (both_errors_type) Schema() schema.Schema {
	return schema.Object{
		"Name":        schema.String{MustNotBeZero: true},
		"Nonexistent": schema.String{MustNotBeZero: true},
	}
}

type alphabetical_order_struct struct {
	Banana string
	Apple  string
	Cherry string
}

func (alphabetical_order_struct) Schema() schema.Schema {
	return schema.Object{
		"Banana": schema.String{MustNotBeZero: true},
		"Apple":  schema.String{MustNotBeZero: true},
		"Cherry": schema.String{MustNotBeZero: true},
	}
}

type label_struct struct {
	Nested error_label_nested
}

type error_label_nested struct {
	Field string
}

func (error_label_nested) Schema() schema.Schema {
	return schema.Object{
		"Field": schema.String{MustNotBeZero: true},
	}
}

func (label_struct) Schema() schema.Schema {
	return schema.Object{}
}

type error_rec_slice_item struct {
	ID int
}

func (error_rec_slice_item) Schema() schema.Schema {
	return schema.Object{
		"ID": schema.Int{
			ValidateFunc: func(v int) error {
				if v <= 0 {
					return errors.New("ID must be positive")
				}
				return nil
			},
		},
	}
}

type label_slice_struct struct {
	Items []error_rec_slice_item
}

func (label_slice_struct) Schema() schema.Schema {
	return schema.Object{"Items": schema.Slice{}}
}

type label_array_holder struct {
	A [2]error_rec_slice_item
}

func (label_array_holder) Schema() schema.Schema {
	return schema.Object{"A": schema.Slice{}}
}

type label_map_struct struct {
	M map[string]error_rec_slice_item
}

func (label_map_struct) Schema() schema.Schema {
	return schema.Object{"M": schema.Map{}}
}

type multi_accum struct {
	Name       string
	Properties map[string]error_rec_slice_item
	Items      []error_rec_slice_item
}

func (multi_accum) Schema() schema.Schema {
	return schema.Object{
		"Name": schema.String{MustNotBeZero: true},
	}
}

type optional_field_recursion struct {
	Required string
	Optional *simple_validation_failure
}

func (optional_field_recursion) Schema() schema.Schema {
	return schema.Object{
		"Required": schema.String{MustNotBeZero: true},
		"Optional": schema.Any{},
	}
}

type unknown_field_fixture struct {
	Name string
}

func (unknown_field_fixture) Schema() schema.Schema {
	return schema.Object{
		"Nmae": schema.String{MustNotBeZero: true},
	}
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestErrors_IsValidationError_Direct(t *testing.T) {
	_, err := schema.Enforce("u", simple_validation_failure{
		Username: "ab",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsValidationError(err) {
		t.Fatalf("expected IsValidationError true, got false (%T: %v)", err, err)
	}
}

func TestErrors_IsValidationError_Wrapped(t *testing.T) {
	_, err := schema.Enforce("u", simple_validation_failure{
		Username: "ab",
	})
	wrapped := fmt.Errorf("context: %w", err)
	if !schema.IsValidationError(wrapped) {
		t.Fatalf("expected wrapped ValidationError to be detected")
	}
}

func TestErrors_IsValidationError_RegularError_False(t *testing.T) {
	if schema.IsValidationError(errors.New("plain")) {
		t.Fatalf("plain error incorrectly identified as ValidationError")
	}
}

func TestErrors_IsSchemaError_Direct(t *testing.T) {
	_, err := schema.Enforce("u", simple_schema_failure{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected IsSchemaError true, got false (%T: %v)", err, err)
	}
}

func TestErrors_IsSchemaError_Wrapped(t *testing.T) {
	_, err := schema.Enforce("u", simple_schema_failure{})
	wrapped := fmt.Errorf("context: %w", err)
	if !schema.IsSchemaError(wrapped) {
		t.Fatalf("expected wrapped SchemaError to be detected")
	}
}

func TestErrors_OriginalMessagePreserved(t *testing.T) {
	_, err := schema.Enforce("u", simple_validation_failure{
		Username: "ab",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "minimum length") {
		t.Fatalf("expected underlying error message in output, got %q", err.Error())
	}
}

func TestErrors_SchemaError_WinsOverValidation(t *testing.T) {
	_, err := schema.Enforce("u", both_errors_type{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError to win; got %T: %v", err, err)
	}
}

func TestErrors_DeterministicFieldOrder(t *testing.T) {
	_, err := schema.Enforce("s", alphabetical_order_struct{})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	ia := strings.Index(msg, ".Apple ")
	ib := strings.Index(msg, ".Banana ")
	ic := strings.Index(msg, ".Cherry ")
	if ia < 0 || ib < 0 || ic < 0 {
		t.Fatalf("expected all three errors; got %q", msg)
	}
	if !(ia < ib && ib < ic) {
		t.Fatalf("errors not alphabetical: Apple=%d Banana=%d Cherry=%d\n%s", ia, ib, ic, msg)
	}
}

func TestErrors_ErrorPathLabels_Struct(t *testing.T) {
	_, err := schema.Enforce("root", label_struct{
		Nested: error_label_nested{Field: ""},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "root.Nested.Field") {
		t.Fatalf("expected nested path label, got %q", err.Error())
	}
}

func TestErrors_ErrorPathLabels_Slice(t *testing.T) {
	h := label_slice_struct{
		Items: []error_rec_slice_item{{ID: 1}, {ID: 0}},
	}
	_, err := schema.Enforce("root", h)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "Items[1]") {
		t.Fatalf("expected Items[1] label, got %q", err.Error())
	}
}

func TestErrors_ErrorPathLabels_Array(t *testing.T) {
	h := label_array_holder{A: [2]error_rec_slice_item{{ID: 1}, {ID: 0}}}
	_, err := schema.Enforce("root", h)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "A[1]") {
		t.Fatalf("expected A[1] label, got %q", err.Error())
	}
}

func TestErrors_ErrorPathLabels_Map(t *testing.T) {
	h := label_map_struct{
		M: map[string]error_rec_slice_item{"entry": {ID: 0}},
	}
	_, err := schema.Enforce("root", h)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "M[entry]") {
		t.Fatalf("expected M[entry] label, got %q", err.Error())
	}
}

func TestErrors_MultiLevelAccumulation_Full(t *testing.T) {
	h := multi_accum{
		Name: "",
		Properties: map[string]error_rec_slice_item{
			"a": {ID: 0},
		},
		Items: []error_rec_slice_item{{ID: 1}, {ID: -1}},
	}
	_, err := schema.Enforce("m", h)
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	expected := []string{
		"must not be empty",
		"Properties[a]",
		"Items[1]",
		"ID must be positive",
	}
	for _, want := range expected {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in:\n%s", want, msg)
		}
	}
}

func TestErrors_IdempotentAcrossMultipleEnforces(t *testing.T) {
	h := simple_validation_failure{Username: "ab"}
	_, err1 := schema.Enforce("u", h)
	_, err2 := schema.Enforce("u", h)
	if err1 == nil || err2 == nil {
		t.Fatalf("expected errors both times")
	}
	if err1.Error() != err2.Error() {
		t.Fatalf("non-idempotent:\n1: %s\n2: %s", err1.Error(), err2.Error())
	}
}

func TestErrors_UnknownFieldInSchema_SchemaError(t *testing.T) {
	_, err := schema.Enforce("u", unknown_field_fixture{})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected 'unknown field' message, got %q", err.Error())
	}
}

func TestErrors_OptionalFieldRecursionStillRuns(t *testing.T) {
	h := optional_field_recursion{
		Required: "present",
		Optional: &simple_validation_failure{Username: "ab"},
	}
	_, err := schema.Enforce("o", h)
	if err == nil {
		t.Fatalf("expected error from nested Schematic")
	}
	if !strings.Contains(err.Error(), "minimum length") {
		t.Fatalf("expected nested min-length error, got %q", err.Error())
	}
}
