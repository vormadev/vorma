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

type any_must_not_be_nil struct {
	V *string
}

func (any_must_not_be_nil) Schema() schema.Schema {
	return schema.Object{"V": schema.Any{MustNotBeNil: true}}
}

type any_leaf_validate struct {
	V *int
}

func (any_leaf_validate) Schema() schema.Schema {
	return schema.Object{"V": schema.Any{
		ValidateFunc: func(v any) error {
			n, ok := v.(int)
			if !ok {
				return errors.New("expected int")
			}
			if n%2 != 0 {
				return errors.New("must be even")
			}
			return nil
		},
	}}
}

type any_default_nil_string struct {
	V *string
}

func (any_default_nil_string) Schema() schema.Schema {
	return schema.Object{"V": schema.Any{DefaultIfNil: "fallback"}}
}

type any_default_nil_interface struct {
	V any
}

func (any_default_nil_interface) Schema() schema.Schema {
	return schema.Object{"V": schema.Any{DefaultIfNil: "fallback"}}
}

type any_non_nil_empty_collections struct {
	Tags  []string
	Attrs map[string]string
}

func (any_non_nil_empty_collections) Schema() schema.Schema {
	return schema.Object{
		"Tags":  schema.Any{MustNotBeNil: true},
		"Attrs": schema.Any{MustNotBeNil: true},
	}
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestAny_MustNotBeNil_NilFails(t *testing.T) {
	_, err := schema.Enforce("s", any_must_not_be_nil{V: nil})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "must not be nil") {
		t.Fatalf("got %q", err.Error())
	}
}

func TestAny_MustNotBeNil_NonNilPasses(t *testing.T) {
	s := "value"
	_, err := schema.Enforce("s", any_must_not_be_nil{V: &s})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAny_ValidateFunc_SeesDereferencedLeafValue(t *testing.T) {
	odd := 3
	_, err := schema.Enforce("s", any_leaf_validate{V: &odd})
	if err == nil {
		t.Fatalf("expected error for odd value")
	}

	even := 4
	_, err = schema.Enforce("s", any_leaf_validate{V: &even})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAny_DefaultIfNil_MaterializesPointer(t *testing.T) {
	out, err := schema.Enforce("s", any_default_nil_string{V: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.V == nil {
		t.Fatalf("expected pointer to be materialized")
	}
	if *out.Value.V != "fallback" {
		t.Fatalf("expected fallback value, got %#v", *out.Value.V)
	}
}

func TestAny_DefaultIfNil_FillsNilInterface(t *testing.T) {
	out, err := schema.Enforce("s", any_default_nil_interface{V: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.V != "fallback" {
		t.Fatalf("expected fallback value, got %#v", out.Value.V)
	}
}

func TestAny_MustNotBeNil_EmptyCollectionsAreStillPresent(t *testing.T) {
	_, err := schema.Enforce("s", any_non_nil_empty_collections{
		Tags:  []string{},
		Attrs: map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
