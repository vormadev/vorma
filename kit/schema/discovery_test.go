package schema_test

import (
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type value_receiver_type struct {
	Name string
}

func (value_receiver_type) Schema() schema.Schema {
	return schema.Object{
		"Name": schema.String{MustNotBeZero: true, TrimSpace: true},
	}
}

type pointer_receiver_type struct {
	Name string
}

func (p *pointer_receiver_type) Schema() schema.Schema {
	return schema.Object{
		"Name": schema.String{MustNotBeZero: true, TrimSpace: true},
	}
}

type wrapped_email string

func (wrapped_email) Schema() schema.Schema {
	return schema.String{
		TrimSpace: true,
		ToLower:   true,
	}
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestDiscovery_AnyWrappedValue(t *testing.T) {
	var in any = value_receiver_type{Name: ""}
	_, err := schema.EnforceAny("v", in)
	if err == nil {
		t.Fatalf("expected validation error for empty Name")
	}
	if !schema.IsValidationError(err) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestDiscovery_AnyWrappedPointer(t *testing.T) {
	v := &pointer_receiver_type{Name: "  Alice  "}
	var in any = v
	_, err := schema.EnforceAny("v", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Name != "Alice" {
		t.Fatalf("expected pointee mutated, got %q", v.Name)
	}
}

func TestDiscovery_DoubleWrappedPointer(t *testing.T) {
	p := &value_receiver_type{Name: "  Bob  "}
	var inner any = p
	var outer any = &inner
	_, err := schema.EnforceAny("v", outer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "Bob" {
		t.Fatalf("expected mutation through wrapping, got %q", p.Name)
	}
}

func TestDiscovery_PointerToPointer(t *testing.T) {
	inner := &value_receiver_type{Name: "  Carol  "}
	outer := &inner
	_, err := schema.EnforceAny("v", outer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inner.Name != "Carol" {
		t.Fatalf("expected mutation, got %q", inner.Name)
	}
}

func TestDiscovery_ValueReceiver_OnValue(t *testing.T) {
	in := value_receiver_type{Name: ""}
	_, err := schema.Enforce("v", in)
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestDiscovery_ValueReceiver_OnPointer(t *testing.T) {
	v := &value_receiver_type{Name: "  Dave  "}
	_, err := schema.Enforce("v", v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Name != "Dave" {
		t.Fatalf("expected mutation, got %q", v.Name)
	}
}

func TestDiscovery_PointerReceiver_OnPointer(t *testing.T) {
	v := &pointer_receiver_type{Name: "  Eve  "}
	_, err := schema.Enforce("v", v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Name != "Eve" {
		t.Fatalf("expected mutation, got %q", v.Name)
	}
}

func TestDiscovery_PointerReceiver_OnValue_CopyFallback(t *testing.T) {
	in := pointer_receiver_type{Name: "  Frank  "}
	res, err := schema.Enforce("v", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.Name != "Frank" {
		t.Fatalf("expected Result.Value to carry mutation, got %q", res.Value.Name)
	}
	if in.Name != "  Frank  " {
		t.Fatalf("input mutated unexpectedly: %q", in.Name)
	}
}

func TestDiscovery_NilPointerSkipped(t *testing.T) {
	var v *pointer_receiver_type
	_, err := schema.EnforceAny("v", v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscovery_NilInterfaceSkipped(t *testing.T) {
	type holder struct {
		X any
	}
	h := holder{X: nil}
	_, err := schema.Enforce("h", &h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscovery_DeeplyWrappedPointerStillDiscovers(t *testing.T) {
	e := wrapped_email("  foo@BAR.com  ")
	var v any = &e
	for i := 0; i < 64; i++ {
		wrap := v
		v = &wrap
	}
	_, err := schema.EnforceAny("v", v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(e) != "foo@bar.com" {
		t.Fatalf("expected mutation through deep wrapping, got %q", e)
	}
}

func TestDiscovery_DeeplyWrappedBoxedValueStillDiscovers(t *testing.T) {
	var boxed any = wrapped_email("  foo@BAR.com  ")
	var v any = &boxed
	for i := 0; i < 64; i++ {
		wrap := v
		v = &wrap
	}
	_, err := schema.EnforceAny("v", v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := boxed.(wrapped_email)
	if !ok {
		t.Fatalf("expected wrapped_email in box, got %T", boxed)
	}
	if string(got) != "foo@bar.com" {
		t.Fatalf("expected boxed value mutation through deep wrapping, got %q", got)
	}
}

func TestDiscovery_WrapperCycleDoesNotLoop(t *testing.T) {
	var v any
	v = &v

	_, err := schema.EnforceAny("v", v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
