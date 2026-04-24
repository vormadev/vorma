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

type bool_ptr_holder struct {
	Enabled *bool
}

type bool_expectation struct {
	Accepted bool
}

type bool_field_holder struct {
	Flag bool
}

type bool_on_int struct {
	N int
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestBool_DefaultIfNil_True(t *testing.T) {
	res, err := schema.Enforce("s", bool_ptr_holder{Enabled: nil}, schema.Object{
		"Enabled": schema.Bool{DefaultIfNil: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.Enabled == nil {
		t.Fatalf("expected pointer to be materialized")
	}
	if !*res.Value.Enabled {
		t.Fatalf("expected DefaultIfNil to materialize true")
	}
}

func TestBool_DefaultIfNil_False(t *testing.T) {
	res, err := schema.Enforce("s", bool_ptr_holder{Enabled: nil}, schema.Object{
		"Enabled": schema.Bool{DefaultIfNil: false},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.Enabled == nil {
		t.Fatalf("expected pointer to be materialized")
	}
	if *res.Value.Enabled {
		t.Fatalf("expected false default, got true")
	}
}

func TestBool_MustBeTrue(t *testing.T) {
	_, err := schema.Enforce("s", bool_expectation{Accepted: false}, schema.Object{
		"Accepted": schema.Bool{MustBeTrue: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "must be true") {
		t.Fatalf("expected must-be-true error, got %q", err.Error())
	}

	_, err = schema.Enforce("s", bool_expectation{Accepted: true}, schema.Object{
		"Accepted": schema.Bool{MustBeTrue: true},
	})
	if err != nil {
		t.Fatalf("unexpected error for true input: %v", err)
	}
}

func TestBool_MustBeFalse(t *testing.T) {
	_, err := schema.Enforce("s", bool_expectation{Accepted: true}, schema.Object{
		"Accepted": schema.Bool{MustBeFalse: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "must be false") {
		t.Fatalf("expected must-be-false error, got %q", err.Error())
	}

	_, err = schema.Enforce("s", bool_expectation{Accepted: false}, schema.Object{
		"Accepted": schema.Bool{MustBeFalse: true},
	})
	if err != nil {
		t.Fatalf("unexpected error for false input: %v", err)
	}
}

func TestBool_ValidateFunc_Runs(t *testing.T) {
	_, err := schema.Enforce("s", bool_field_holder{Flag: false}, schema.Object{
		"Flag": schema.Bool{
			ValidateFunc: func(v bool) error {
				if !v {
					return errors.New("flag must be set")
				}
				return nil
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "flag must be set") {
		t.Fatalf("got %q", err.Error())
	}

	_, err = schema.Enforce("s", bool_field_holder{Flag: true}, schema.Object{
		"Flag": schema.Bool{
			ValidateFunc: func(v bool) error {
				if !v {
					return errors.New("flag must be set")
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBool_AppliedToInt_SchemaError(t *testing.T) {
	_, err := schema.Enforce("s", bool_on_int{N: 5}, schema.Object{
		"N": schema.Bool{MustBeTrue: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}
