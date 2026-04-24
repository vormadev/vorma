package schema_test

import (
	"reflect"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type normalizing_email string

func (normalizing_email) Schema() schema.Schema {
	return schema.String{
		TrimSpace: true,
		ToLower:   true,
	}
}

type empty_schema_type struct {
	Name string
}

func (empty_schema_type) Schema() schema.Schema { return nil }

type pointer_receiver_only struct {
	Name string
}

func (p *pointer_receiver_only) Schema() schema.Schema {
	return schema.Object{
		"Name": schema.String{MustNotBeZero: true, TrimSpace: true},
	}
}

type map_holder struct {
	Emails map[string]normalizing_email
}

type interface_boxed_holder struct {
	V any
}

func (interface_boxed_holder) Schema() schema.Schema {
	return schema.Object{
		"V": schema.Any{},
	}
}

type additive_root_user struct {
	Name string
}

func (additive_root_user) Schema() schema.Schema {
	return schema.Object{
		"Name": schema.String{
			TrimSpace: true,
			ToLower:   true,
		},
	}
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestEnforceAny_NilInput(t *testing.T) {
	res, err := schema.EnforceAny("x", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value != nil {
		t.Fatalf("expected nil value, got %#v", res.Value)
	}
}

func TestEnforce_PreservesConcreteType(t *testing.T) {
	in := normalizing_email("foo@bar.com")
	res, err := schema.Enforce("e", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out normalizing_email = res.Value
	if out != "foo@bar.com" {
		t.Fatalf("expected unchanged value, got %q", out)
	}
}

func TestEnforceAny_PreservesDynamicType(t *testing.T) {
	in := normalizing_email("foo@bar.com")
	res, err := schema.EnforceAny("e", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := reflect.TypeOf(res.Value)
	want := reflect.TypeOf(in)
	if got != want {
		t.Fatalf("dynamic type changed: got %v, want %v", got, want)
	}
	if _, ok := res.Value.(normalizing_email); !ok {
		t.Fatalf("type assertion failed")
	}
}

func TestEnforce_ValueInput_OriginalUnchanged(t *testing.T) {
	const original = normalizing_email("  FOO@BAR.COM  ")
	caller_var := original
	_, err := schema.Enforce("e", caller_var)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caller_var != original {
		t.Fatalf("caller's value changed: before=%q after=%q", original, caller_var)
	}
}

func TestEnforce_PointerInput_PointeeMutated(t *testing.T) {
	e := normalizing_email("  FOO@BAR.COM  ")
	_, err := schema.Enforce("e", &e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e != "foo@bar.com" {
		t.Fatalf("expected pointee mutated, got %q", e)
	}
}

func TestEnforce_PointerInput_ResultAliasesInput(t *testing.T) {
	e := normalizing_email("foo@bar.com")
	in := &e
	res, err := schema.Enforce("e", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value != in {
		t.Fatalf("expected result to alias input pointer")
	}
}

func TestEnforce_OutputReflectsTransformations(t *testing.T) {
	res_val, err := schema.Enforce("e", normalizing_email("  HELLO@Example.com  "))
	if err != nil {
		t.Fatalf("value case: unexpected error: %v", err)
	}
	if res_val.Value != "hello@example.com" {
		t.Fatalf("value case: got %q", res_val.Value)
	}

	e := normalizing_email("  HELLO@Example.com  ")
	res_ptr, err := schema.Enforce("e", &e)
	if err != nil {
		t.Fatalf("pointer case: unexpected error: %v", err)
	}
	if *res_ptr.Value != "hello@example.com" {
		t.Fatalf("pointer case: got %q", *res_ptr.Value)
	}
}

func TestEnforce_ValueInValueOut(t *testing.T) {
	in := normalizing_email("foo@bar.com")
	res, err := schema.Enforce("e", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kind := reflect.ValueOf(res.Value).Kind()
	if kind != reflect.String {
		t.Fatalf("expected String kind, got %v", kind)
	}
}

func TestEnforce_PointerInPointerOut(t *testing.T) {
	e := normalizing_email("foo@bar.com")
	res, err := schema.Enforce("e", &e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rv := reflect.ValueOf(res.Value)
	if rv.Kind() != reflect.Pointer {
		t.Fatalf("expected Pointer kind, got %v", rv.Kind())
	}
	if rv.IsNil() {
		t.Fatalf("expected non-nil pointer")
	}
	if rv.Elem().Kind() != reflect.String {
		t.Fatalf("expected pointer to String kind, got pointer to %v", rv.Elem().Kind())
	}
}

func TestEnforce_EmptySchemaNoOp(t *testing.T) {
	in := empty_schema_type{Name: "Alice"}
	res, err := schema.Enforce("x", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.Name != "Alice" {
		t.Fatalf("expected unchanged value, got %q", res.Value.Name)
	}
	if in.Name != "Alice" {
		t.Fatalf("input changed")
	}
}

func TestEnforce_MapValueMutation_ReflectedInMap(t *testing.T) {
	h := map_holder{Emails: map[string]normalizing_email{
		"primary":   "  ALICE@Example.com  ",
		"secondary": "  BOB@Example.COM  ",
	}}
	_, err := schema.Enforce("h", &h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := h.Emails["primary"]; got != "alice@example.com" {
		t.Fatalf("primary not normalized: got %q", got)
	}
	if got := h.Emails["secondary"]; got != "bob@example.com" {
		t.Fatalf("secondary not normalized: got %q", got)
	}
}

func TestEnforce_InterfaceBoxedSchematicValue_WritesBack(t *testing.T) {
	h := interface_boxed_holder{V: normalizing_email("  ALICE@Example.COM  ")}
	_, err := schema.Enforce("h", &h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := h.V.(normalizing_email)
	if !ok {
		t.Fatalf("expected normalizing_email, got %T", h.V)
	}
	if got != "alice@example.com" {
		t.Fatalf("expected normalized boxed value, got %q", got)
	}
}

func TestEnforce_NonAddressableValueReceiverOnly_ValidatesButDoesNotMutate(t *testing.T) {
	original := pointer_receiver_only{Name: "  Alice  "}
	caller_var := original
	_, err := schema.Enforce("p", caller_var)
	if err != nil {
		t.Fatalf("unexpected error (value case): %v", err)
	}
	if caller_var.Name != "  Alice  " {
		t.Fatalf("unexpected mutation of caller's value: %q", caller_var.Name)
	}

	ptr_var := original
	_, err = schema.Enforce("p", &ptr_var)
	if err != nil {
		t.Fatalf("unexpected error (pointer case): %v", err)
	}
	if ptr_var.Name != "Alice" {
		t.Fatalf("pointer case: expected mutation, got %q", ptr_var.Name)
	}
}

func TestEnforce_ExplicitRoot_AddsToDiscoveredSchema(t *testing.T) {
	res, err := schema.Enforce("user", additive_root_user{Name: "  HELLO  "}, schema.Object{
		"Name": schema.String{
			MustBeIn: []string{"hello"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.Name != "hello" {
		t.Fatalf("expected normalized name, got %q", res.Value.Name)
	}
}
