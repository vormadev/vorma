package schema_test

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type empty_schema_struct struct {
	X string
}

type pointer_rule_blocks struct {
	Name  string
	Age   int
	Flag  bool
	Tags  []string
	Attrs map[string]string
	Meta  any
	Inner pointer_rule_inner
}

type pointer_rule_inner struct {
	Code string
}

type nil_pointer_rule_block struct {
	Name string
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

// Case 204: an explicit nil root schema is a no-op.
func TestSchema_EmptySchema_NoOp(t *testing.T) {
	out, err := schema.Enforce("s", empty_schema_struct{X: ""}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value.X != "" {
		t.Fatalf("expected unchanged value, got %#v", out.Value)
	}
}

// Case 205: a value-mode rule block applied to a non-matching kind is
// a SchemaError.
func TestSchema_ValueMode_OnNonMatchingKind_SchemaError(t *testing.T) {
	_, err := schema.Enforce("v", 5, schema.String{MustNotBeZero: true})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

// Case 206: an Object-mode schema applied to a scalar target is a
// SchemaError.
func TestSchema_FieldsOnNonStructNonMap_SchemaError(t *testing.T) {
	_, err := schema.Enforce("v", 5, schema.Object{
		"Name": schema.String{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

func TestSchema_PointerRuleBlocks_AreAccepted(t *testing.T) {
	_, err := schema.Enforce("s", pointer_rule_blocks{
		Name:  "Alice",
		Age:   30,
		Flag:  true,
		Tags:  []string{},
		Attrs: map[string]string{},
		Meta:  "present",
		Inner: pointer_rule_inner{Code: "x"},
	}, schema.Object{
		"Name":  &schema.String{MustNotBeZero: true},
		"Age":   &schema.Int{MustNotBeZero: true},
		"Flag":  &schema.Bool{MustBeTrue: true},
		"Tags":  &schema.List{MustNotBeNil: true},
		"Attrs": &schema.Map{MustNotBeNil: true},
		"Meta":  &schema.Any{MustNotBeNil: true},
		"Inner": &schema.Object{"Code": &schema.String{MustNotBeZero: true}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchema_NilPointerRuleBlock_IsSchemaError(t *testing.T) {
	_, err := schema.Enforce("s", nil_pointer_rule_block{Name: "Alice"}, schema.Object{
		"Name": (*schema.String)(nil),
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "nil *schema.String rule") {
		t.Fatalf("expected nil rule message, got %q", err.Error())
	}
}
