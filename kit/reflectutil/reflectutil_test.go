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

func TestJSONStructFields(t *testing.T) {
	type Embedded struct {
		Embedded string `json:"embedded"`
		private  string
	}
	type TaggedEmbedded struct {
		Value string `json:"value"`
	}
	type sample struct {
		Embedded
		*TaggedEmbedded `json:"tagged"`
		FromPtr         *int   `json:"fromPtr"`
		FromOmitEmpty   int    `json:"fromOmitEmpty,omitempty"`
		FromOmitZero    bool   `json:"fromOmitZero,omitzero"`
		Required        string `json:"required"`
		Ignored         string `json:"-"`
		DashName        string `json:"'-'"`
		private         string
	}

	fields, err := JSONStructFields(reflect.TypeFor[sample]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make(map[string]JSONFieldShape, len(fields))
	for _, field := range fields {
		got[field.JSONName] = field
	}

	for _, name := range []string{
		"embedded",
		"tagged",
		"fromPtr",
		"fromOmitEmpty",
		"fromOmitZero",
		"required",
		"-",
	} {
		if _, ok := got[name]; !ok {
			t.Fatalf("missing JSON field %q in %#v", name, fields)
		}
	}
	for _, name := range []string{"Ignored", "private"} {
		if _, ok := got[name]; ok {
			t.Fatalf("unexpected omitted/private field %q in %#v", name, fields)
		}
	}

	if got["embedded"].Optional {
		t.Fatalf("direct embedded value field should not be optional")
	}
	if !got["tagged"].Optional {
		t.Fatalf("tagged embedded pointer should be optional")
	}
	if got["tagged"].BaseType.Kind() != reflect.Struct {
		t.Fatalf("expected tagged embedded base struct, got %s",
			got["tagged"].BaseType)
	}
	if !got["fromPtr"].Optional || !got["fromPtr"].Pointer {
		t.Fatalf("pointer field should be optional and marked pointer")
	}
	if !got["fromOmitEmpty"].Optional || !got["fromOmitEmpty"].OmitEmpty {
		t.Fatalf("omitempty field should be optional")
	}
	if !got["fromOmitZero"].Optional || !got["fromOmitZero"].OmitZero {
		t.Fatalf("omitzero field should be optional")
	}
	if got["required"].Optional {
		t.Fatalf("plain non-pointer field should be required")
	}
}

func TestJSONStructFields_OptionalEmbeddedPointer(t *testing.T) {
	type Embedded struct {
		Name string `json:"name"`
	}
	type sample struct {
		*Embedded
		ID string `json:"id"`
	}

	fields, err := JSONStructFields(reflect.TypeFor[sample]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make(map[string]JSONFieldShape, len(fields))
	for _, field := range fields {
		got[field.JSONName] = field
	}
	if !got["name"].Optional {
		t.Fatalf("field from optional embedded pointer should be optional")
	}
	if !got["name"].ViaOptionalEmbedded {
		t.Fatalf("field should be marked as coming through optional embedded pointer")
	}
	if got["id"].Optional {
		t.Fatalf("sibling field should not inherit optional embedded state")
	}
}

func TestJSONStructFields_Dominance(t *testing.T) {
	type A struct {
		Name string `json:"name"`
	}
	type B struct {
		Name string `json:"name"`
	}
	type shallow struct {
		A
		Name string `json:"name"`
	}
	fields, err := JSONStructFields(reflect.TypeFor[shallow]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].GoName != "Name" ||
		len(fields[0].Index) != 1 {
		t.Fatalf("expected shallow field to dominate: %#v", fields)
	}

	type ambiguous struct {
		A
		B
	}
	fields, err = JSONStructFields(reflect.TypeFor[ambiguous]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Fatalf("expected ambiguous same-depth fields to be omitted: %#v", fields)
	}

	type Untagged struct {
		Value string
	}
	type Tagged struct {
		Value string `json:"Value"`
	}
	type tag_wins struct {
		Untagged
		Tagged
	}
	fields, err = JSONStructFields(reflect.TypeFor[tag_wins]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].Field.PkgPath != "" ||
		fields[0].Index[0] != 1 {
		t.Fatalf("expected tagged field to dominate: %#v", fields)
	}
}

func TestJSONStructFields_ExplicitInline(t *testing.T) {
	type Inline struct {
		Value string `json:"value"`
	}
	type sample struct {
		Inline Inline         `json:",inline"`
		ID     string         `json:"id"`
		Extra  map[string]any `json:",unknown"`
	}

	shape, err := JSONStructShape(reflect.TypeFor[sample]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make(map[string]JSONFieldShape, len(shape.Fields))
	for _, field := range shape.Fields {
		got[field.JSONName] = field
	}
	if _, ok := got["value"]; !ok {
		t.Fatalf("expected explicit inline struct field to be flattened: %#v", shape.Fields)
	}
	if _, ok := got["id"]; !ok {
		t.Fatalf("expected normal field: %#v", shape.Fields)
	}
	if _, ok := got["Extra"]; ok {
		t.Fatalf("unknown fallback should not be a finite JSON field: %#v", shape.Fields)
	}
	if len(shape.Inlined) != 1 || shape.Inlined[0].GoName != "Inline" {
		t.Fatalf("expected explicit inline field in shape: %#v", shape.Inlined)
	}
	if len(shape.Unknowns) != 1 || shape.Unknowns[0].GoName != "Extra" {
		t.Fatalf("expected unknown fallback in shape: %#v", shape.Unknowns)
	}
}

func TestJSONStructFields_InvalidInlineOptions(t *testing.T) {
	type sample struct {
		Inline struct{} `json:",inline,omitempty"`
	}
	if _, err := JSONStructFields(reflect.TypeFor[sample]()); err == nil {
		t.Fatalf("expected error for invalid inline options")
	}
}

func TestJSONStructFields_InvalidV2Tag(t *testing.T) {
	type sample struct {
		Dash string `json:"-,"`
	}
	if _, err := JSONStructFields(reflect.TypeFor[sample]()); err == nil {
		t.Fatalf("expected error for invalid v2 tag")
	}
}

func TestJSONStructFields_AnonymousNonStructRequiresName(t *testing.T) {
	type SampleString string
	type sample struct {
		SampleString
	}
	if _, err := JSONStructFields(reflect.TypeFor[sample]()); err == nil {
		t.Fatalf("expected error for anonymous non-struct without JSON name")
	}

	type named struct {
		SampleString `json:"sample"`
	}
	fields, err := JSONStructFields(reflect.TypeFor[named]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].JSONName != "sample" {
		t.Fatalf("expected explicitly named anonymous field: %#v", fields)
	}
}

func TestJSONStructFields_NonStruct(t *testing.T) {
	if _, err := JSONStructFields(reflect.TypeFor[int]()); err == nil {
		t.Fatalf("expected error for non-struct type")
	}
}

func TestJSONStructFields_JSONV2InlinePointerDepth(t *testing.T) {
	type too_deep struct {
		X **struct{ A int } `json:",inline"`
	}
	if _, err := JSONStructFields(reflect.TypeFor[too_deep]()); err == nil {
		t.Fatalf("expected error for double pointer inline field")
	}

	type NamedPtr *struct{ A int }
	type named_ptr struct {
		X NamedPtr `json:",inline"`
	}
	if _, err := JSONStructFields(reflect.TypeFor[named_ptr]()); err == nil {
		t.Fatalf("expected error for named pointer inline field")
	}
}
