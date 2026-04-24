package schema_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type presence_holder struct {
	Name    string
	Age     int
	Address *string
}

type presence_slice_holder struct {
	Tags []string
}

type presence_map_holder struct {
	Attrs map[string]string
}

type presence_inner struct {
	X string
}

type presence_nested_struct struct {
	Inner presence_inner
}

type presence_int_default struct {
	Count int
}

type presence_string_default struct {
	Greeting string
}

type presence_named_string string

type presence_named_bool bool

type presence_named_string_default struct {
	Greeting presence_named_string
}

type presence_pointer_defaults struct {
	Name    *string
	Count   *int
	Enabled *bool
}

type presence_named_pointer_defaults struct {
	Name    *presence_named_string
	Enabled *presence_named_bool
}

type presence_pointer_values struct {
	Name    *string
	Count   *int
	Enabled *bool
	Tags    *[]string
	Attrs   *map[string]string
}

/////////////////////////////////////////////////////////////////////
/////// EXPLICIT NIL / ZERO RULES
/////////////////////////////////////////////////////////////////////

func TestPresence_ExplicitNilAndZeroRules_MultipleFields(t *testing.T) {
	_, err := schema.Enforce("s", presence_holder{}, schema.Object{
		"Name":    schema.String{MustNotBeZero: true},
		"Age":     schema.Int{MustNotBeZero: true},
		"Address": schema.Any{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"Name must not be empty",
		"Age must not be zero",
		"Address must not be nil",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in:\n%s", want, msg)
		}
	}
}

func TestPresence_ExplicitNilAndZeroRules_AllFilled(t *testing.T) {
	addr := "123 Main"
	_, err := schema.Enforce("s", presence_holder{
		Name:    "Alice",
		Age:     30,
		Address: &addr,
	}, schema.Object{
		"Name":    schema.String{MustNotBeZero: true},
		"Age":     schema.Int{MustNotBeZero: true},
		"Address": schema.Any{MustNotBeNil: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPresence_NoExplicitRules_AllZeroPasses(t *testing.T) {
	_, err := schema.Enforce("s", presence_holder{}, schema.Object{
		"Name":    schema.String{},
		"Age":     schema.Int{},
		"Address": schema.Any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPresence_MixedExplicitAndImplicitRules(t *testing.T) {
	_, err := schema.Enforce("s", presence_holder{}, schema.Object{
		"Name":    schema.String{MustNotBeZero: true},
		"Age":     schema.Int{},
		"Address": schema.Any{},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Name must not be empty") {
		t.Fatalf("expected Name error, got %q", msg)
	}
	if strings.Contains(msg, "Age must not be zero") {
		t.Fatalf("Age should not error: %q", msg)
	}
	if strings.Contains(msg, "Address must not be nil") {
		t.Fatalf("Address should not error: %q", msg)
	}
}

func TestPresence_NilPointer_MustNotBeNil_Fails(t *testing.T) {
	_, err := schema.Enforce("s", presence_holder{
		Name:    "A",
		Age:     1,
		Address: nil,
	}, schema.Object{
		"Address": schema.Any{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "Address must not be nil") {
		t.Fatalf("expected Address error, got %q", err.Error())
	}
}

func TestPresence_NilPointer_WithoutMustNotBeNil_Passes(t *testing.T) {
	_, err := schema.Enforce("s", presence_holder{Address: nil}, schema.Object{
		"Address": schema.Any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPresence_ZeroScalar_MustNotBeZero_Fails(t *testing.T) {
	_, err := schema.Enforce("s", presence_holder{
		Name: "A",
		Age:  0,
	}, schema.Object{
		"Age": schema.Int{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "Age must not be zero") {
		t.Fatalf("expected Age error, got %q", err.Error())
	}
}

func TestPresence_NilSlice_MustNotBeNil_Fails(t *testing.T) {
	_, err := schema.Enforce("s", presence_slice_holder{Tags: nil}, schema.Object{
		"Tags": schema.List{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "Tags must not be nil") {
		t.Fatalf("expected Tags error, got %q", err.Error())
	}
}

func TestPresence_NilMap_MustNotBeNil_Fails(t *testing.T) {
	_, err := schema.Enforce("s", presence_map_holder{Attrs: nil}, schema.Object{
		"Attrs": schema.Map{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "Attrs must not be nil") {
		t.Fatalf("expected Attrs error, got %q", err.Error())
	}
}

func TestPresence_AnyOnNestedStruct_DoesNotInventZeroSemantics(t *testing.T) {
	_, err := schema.Enforce("s", presence_nested_struct{
		Inner: presence_inner{},
	}, schema.Object{
		"Inner": schema.Any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

/////////////////////////////////////////////////////////////////////
/////// DEFAULTS
/////////////////////////////////////////////////////////////////////

func TestPresence_DefaultIfZero_FiresOnZero(t *testing.T) {
	out, err := schema.Enforce("s", presence_int_default{}, schema.Object{
		"Count": schema.Int{DefaultIfZero: 42},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.Count != 42 {
		t.Fatalf("expected default, got %d", out.Value.Count)
	}
}

func TestPresence_DefaultIfZero_SkippedWhenPresent(t *testing.T) {
	out, err := schema.Enforce("s", presence_int_default{Count: 5}, schema.Object{
		"Count": schema.Int{DefaultIfZero: 42},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.Count != 5 {
		t.Fatalf("expected Count=5, got %d", out.Value.Count)
	}
}

func TestPresence_DefaultIfZero_ThenSatisfiesMustNotBeZero(t *testing.T) {
	out, err := schema.Enforce("s", presence_int_default{}, schema.Object{
		"Count": schema.Int{MustNotBeZero: true, DefaultIfZero: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.Count != 1 {
		t.Fatalf("expected default, got %d", out.Value.Count)
	}
}

func TestPresence_DefaultIfZero_String(t *testing.T) {
	out, err := schema.Enforce("s", presence_string_default{}, schema.Object{
		"Greeting": schema.String{DefaultIfZero: "hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.Greeting != "hello" {
		t.Fatalf("expected default, got %q", out.Value.Greeting)
	}
}

func TestPresence_DefaultIfZero_AcceptsNamedStringDefaults(t *testing.T) {
	out, err := schema.Enforce("s", presence_named_string_default{}, schema.Object{
		"Greeting": schema.String{DefaultIfZero: presence_named_string("hello")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.Greeting != "hello" {
		t.Fatalf("expected default, got %q", out.Value.Greeting)
	}
}

func TestPresence_DefaultIfZero_OnAbsentMapKey(t *testing.T) {
	value := map[string]string{}

	out, err := schema.Enforce("m", &value, schema.Object{
		"Mode": schema.String{DefaultIfZero: "auto"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := (*out.Value)["Mode"]; got != "auto" {
		t.Fatalf("expected defaulted Mode, got %q (full map: %#v)", got, *out.Value)
	}
}

func TestPresence_DefaultIfNil_AllocatesTypedPointers(t *testing.T) {
	out, err := schema.Enforce("s", presence_pointer_defaults{}, schema.Object{
		"Name":    schema.String{DefaultIfNil: "hello"},
		"Count":   schema.Int{DefaultIfNil: 42},
		"Enabled": schema.Bool{DefaultIfNil: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.Name == nil || *out.Value.Name != "hello" {
		t.Fatalf("expected Name default, got %#v", out.Value.Name)
	}
	if out.Value.Count == nil || *out.Value.Count != 42 {
		t.Fatalf("expected Count default, got %#v", out.Value.Count)
	}
	if out.Value.Enabled == nil || !*out.Value.Enabled {
		t.Fatalf("expected Enabled default, got %#v", out.Value.Enabled)
	}
}

func TestPresence_DefaultIfNil_AcceptsNamedStringAndBoolDefaults(t *testing.T) {
	out, err := schema.Enforce("s", presence_named_pointer_defaults{}, schema.Object{
		"Name":    schema.String{DefaultIfNil: presence_named_string("hello")},
		"Enabled": schema.Bool{DefaultIfNil: presence_named_bool(true)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.Name == nil || *out.Value.Name != "hello" {
		t.Fatalf("expected Name default, got %#v", out.Value.Name)
	}
	if out.Value.Enabled == nil || !*out.Value.Enabled {
		t.Fatalf("expected Enabled default, got %#v", out.Value.Enabled)
	}
}

/////////////////////////////////////////////////////////////////////
/////// TYPED NIL POINTERS
/////////////////////////////////////////////////////////////////////

func TestPresence_TypedNilPointers_WithoutMustNotBeNilSkipLeafValidation(t *testing.T) {
	_, err := schema.Enforce("s", presence_pointer_values{}, schema.Object{
		"Name": schema.String{
			ValidateFunc: func(string) error {
				return errors.New("string validate ran")
			},
		},
		"Count": schema.Int{
			ValidateFunc: func(int) error {
				return errors.New("int validate ran")
			},
		},
		"Enabled": schema.Bool{
			ValidateFunc: func(bool) error {
				return errors.New("bool validate ran")
			},
		},
		"Tags": schema.List{
			ValidateFunc: func(int) error {
				return errors.New("list validate ran")
			},
		},
		"Attrs": schema.Map{
			ValidateFunc: func(int) error {
				return errors.New("map validate ran")
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPresence_TypedNilPointers_MustNotBeNilAreValidationErrors(t *testing.T) {
	_, err := schema.Enforce("s", presence_pointer_values{}, schema.Object{
		"Name":  schema.String{MustNotBeNil: true},
		"Count": schema.Int{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if schema.IsSchemaError(err) {
		t.Fatalf("expected validation error, got SchemaError: %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"Name must not be nil", "Count must not be nil"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in %q", want, msg)
		}
	}
}
