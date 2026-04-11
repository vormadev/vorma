package reflectutil

import (
	"reflect"
	"testing"
)

type testIface interface {
	Foo()
}

type testValueImpl struct{}

func (testValueImpl) Foo() {}

type testPointerImpl struct{}

func (*testPointerImpl) Foo() {}

type testNoImpl struct{}

func TestDoesTypeImplementInterface(t *testing.T) {
	iface := reflect.TypeFor[testIface]()

	if DoesTypeImplementInterface(nil, iface) {
		t.Fatal("expected nil concrete type to return false")
	}
	if DoesTypeImplementInterface(reflect.TypeOf(testValueImpl{}), nil) {
		t.Fatal("expected nil interface type to return false")
	}

	if !DoesTypeImplementInterface(reflect.TypeOf(testValueImpl{}), iface) {
		t.Fatal("expected value receiver implementation to match")
	}
	if !DoesTypeImplementInterface(reflect.TypeOf(&testValueImpl{}), iface) {
		t.Fatal("expected pointer to value receiver type to match")
	}
	if !DoesTypeImplementInterface(reflect.TypeOf(testPointerImpl{}), iface) {
		t.Fatal("expected pointer receiver implementation to match via PointerTo")
	}
	if !DoesTypeImplementInterface(reflect.TypeOf(&testPointerImpl{}), iface) {
		t.Fatal("expected pointer receiver implementation to match")
	}
	if DoesTypeImplementInterface(reflect.TypeOf(testNoImpl{}), iface) {
		t.Fatal("expected non-implementation to return false")
	}
}

func TestDoesTypeImplementInterface_PanicsWhenIfaceNotInterface(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for non-interface iface type")
		}
	}()

	DoesTypeImplementInterface(
		reflect.TypeOf(testValueImpl{}),
		reflect.TypeOf(testValueImpl{}),
	)
}

func TestExcludingNoneGetIsNilOrUltimatelyPointsToNil(t *testing.T) {
	if !ExcludingNoneGetIsNilOrUltimatelyPointsToNil(nil) {
		t.Fatal("expected nil to be true")
	}

	var p *int
	if !ExcludingNoneGetIsNilOrUltimatelyPointsToNil(p) {
		t.Fatal("expected nil pointer to be true")
	}

	var wrapped any = p
	if !ExcludingNoneGetIsNilOrUltimatelyPointsToNil(wrapped) {
		t.Fatal("expected interface-wrapped nil pointer to be true")
	}

	var m map[string]int
	if !ExcludingNoneGetIsNilOrUltimatelyPointsToNil(m) {
		t.Fatal("expected nil map to be true")
	}

	var s []string
	if !ExcludingNoneGetIsNilOrUltimatelyPointsToNil(s) {
		t.Fatal("expected nil slice to be true")
	}

	if ExcludingNoneGetIsNilOrUltimatelyPointsToNil(map[string]int{}) {
		t.Fatal("expected non-nil map to be false")
	}
	if ExcludingNoneGetIsNilOrUltimatelyPointsToNil([]string{}) {
		t.Fatal("expected non-nil slice to be false")
	}
	if ExcludingNoneGetIsNilOrUltimatelyPointsToNil(testValueImpl{}) {
		t.Fatal("expected non-pointer struct to be false")
	}

	none := struct{}{}
	if ExcludingNoneGetIsNilOrUltimatelyPointsToNil(none) {
		t.Fatal("expected None sentinel struct{} to be false")
	}

	var nonePtr *struct{}
	if ExcludingNoneGetIsNilOrUltimatelyPointsToNil(nonePtr) {
		t.Fatal("expected None sentinel *struct{} nil pointer to be false")
	}
}

func TestGetJSONFieldName(t *testing.T) {
	type sample struct {
		ID        string `json:"id,omitempty"`
		Raw       string
		Ignored   string `json:"-"`
		Fallback  string `json:",omitempty"`
		Explicit  string `json:"explicit"`
		ExtraOpts string `json:"name,string,omitempty"`
	}

	fields := map[string]string{
		"ID":        "id",
		"Raw":       "Raw",
		"Ignored":   "",
		"Fallback":  "Fallback",
		"Explicit":  "explicit",
		"ExtraOpts": "name",
	}

	typ := reflect.TypeFor[sample]()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		got := JSONFieldName(f)
		want := fields[f.Name]
		if got != want {
			t.Fatalf("field %s: expected %q, got %q", f.Name, want, got)
		}
	}
}
