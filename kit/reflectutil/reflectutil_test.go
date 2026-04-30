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

func TestTypeImplements(t *testing.T) {
	iface := reflect.TypeFor[testIface]()

	if TypeImplements(nil, iface) {
		t.Fatal("expected nil concrete type to return false")
	}
	if TypeImplements(reflect.TypeFor[testValueImpl](), nil) {
		t.Fatal("expected nil interface type to return false")
	}

	if !TypeImplements(reflect.TypeFor[testValueImpl](), iface) {
		t.Fatal("expected value receiver implementation to match")
	}
	if !TypeImplements(reflect.TypeFor[*testValueImpl](), iface) {
		t.Fatal("expected pointer to value receiver type to match")
	}
	if !TypeImplements(reflect.TypeFor[testPointerImpl](), iface) {
		t.Fatal("expected pointer receiver implementation to match via PointerTo")
	}
	if !TypeImplements(reflect.TypeFor[*testPointerImpl](), iface) {
		t.Fatal("expected pointer receiver implementation to match")
	}
	if TypeImplements(reflect.TypeFor[testNoImpl](), iface) {
		t.Fatal("expected non-implementation to return false")
	}
}

func TestTypeImplements_PanicsWhenIfaceNotInterface(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for non-interface iface type")
		}
	}()

	TypeImplements(
		reflect.TypeFor[testValueImpl](),
		reflect.TypeFor[testValueImpl](),
	)
}

func TestIsNilLikeExceptNone(t *testing.T) {
	if !IsNilLikeExceptNone(nil) {
		t.Fatal("expected nil to be true")
	}

	var p *int
	if !IsNilLikeExceptNone(p) {
		t.Fatal("expected nil pointer to be true")
	}

	var wrapped any = p
	if !IsNilLikeExceptNone(wrapped) {
		t.Fatal("expected interface-wrapped nil pointer to be true")
	}

	var m map[string]int
	if !IsNilLikeExceptNone(m) {
		t.Fatal("expected nil map to be true")
	}

	var s []string
	if !IsNilLikeExceptNone(s) {
		t.Fatal("expected nil slice to be true")
	}

	if IsNilLikeExceptNone(map[string]int{}) {
		t.Fatal("expected non-nil map to be false")
	}
	if IsNilLikeExceptNone([]string{}) {
		t.Fatal("expected non-nil slice to be false")
	}
	if IsNilLikeExceptNone(testValueImpl{}) {
		t.Fatal("expected non-pointer struct to be false")
	}

	none := struct{}{}
	if IsNilLikeExceptNone(none) {
		t.Fatal("expected None sentinel struct{} to be false")
	}

	var nonePtr *struct{}
	if IsNilLikeExceptNone(nonePtr) {
		t.Fatal("expected None sentinel *struct{} nil pointer to be false")
	}
}

func TestDerefType(t *testing.T) {
	type sample struct{}

	tp, is_pointer := DerefType(reflect.TypeFor[**sample]())
	if tp != reflect.TypeFor[sample]() {
		t.Fatalf("DerefType type = %v, want %v", tp, reflect.TypeFor[sample]())
	}
	if !is_pointer {
		t.Fatal("DerefType should report pointer")
	}

	tp, is_pointer = DerefType(reflect.TypeFor[sample]())
	if tp != reflect.TypeFor[sample]() {
		t.Fatalf("DerefType type = %v, want %v", tp, reflect.TypeFor[sample]())
	}
	if is_pointer {
		t.Fatal("DerefType should not report pointer")
	}
}

func TestNewValueWithAnonymousPointerFields(t *testing.T) {
	type Embedded struct {
		Name string
	}
	type host struct {
		*Embedded
	}

	v := NewValueWithAnonymousPointerFields(reflect.TypeFor[host]())
	if v.Field(0).IsNil() {
		t.Fatal("anonymous pointer field was not initialized")
	}
}

func TestPublicStructFields(t *testing.T) {
	type Embedded struct {
		Embedded string `json:"embedded"`
		//lint:ignore U1000 .
		private string
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
		//lint:ignore SA5008 .
		DashName string `json:"'-'"`
		//lint:ignore U1000 .
		private string
	}

	fields, err := PublicStructFields(reflect.TypeFor[sample]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make(map[string]StructField, len(fields))
	for _, field := range fields {
		got[field.PublicName] = field
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
			t.Fatalf("missing public field %q in %#v", name, fields)
		}
	}
	for _, name := range []string{"Ignored", "private"} {
		if _, ok := got[name]; ok {
			t.Fatalf("unexpected omitted/private field %q in %#v", name, fields)
		}
	}

	if got["embedded"].OptionalInPublicShape {
		t.Fatalf("direct embedded value field should not be optional")
	}
	if !got["tagged"].OptionalInPublicShape {
		t.Fatalf("tagged embedded pointer should be optional")
	}
	if got["tagged"].DerefType.Kind() != reflect.Struct {
		t.Fatalf("expected tagged embedded base struct, got %s",
			got["tagged"].DerefType)
	}
	if !got["fromPtr"].OptionalInPublicShape || !got["fromPtr"].TypeWasPointer {
		t.Fatalf("pointer field should be optional and marked pointer")
	}
	if !got["fromOmitEmpty"].OptionalInPublicShape || !got["fromOmitEmpty"].OmitEmpty {
		t.Fatalf("omitempty field should be optional")
	}
	if !got["fromOmitZero"].OptionalInPublicShape || !got["fromOmitZero"].OmitZero {
		t.Fatalf("omitzero field should be optional")
	}
	if got["required"].OptionalInPublicShape {
		t.Fatalf("plain non-pointer field should be required")
	}
}

func TestPublicStructFields_OptionalEmbeddedPointer(t *testing.T) {
	type Embedded struct {
		Name string `json:"name"`
	}
	type sample struct {
		*Embedded
		ID string `json:"id"`
	}

	fields, err := PublicStructFields(reflect.TypeFor[sample]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make(map[string]StructField, len(fields))
	for _, field := range fields {
		got[field.PublicName] = field
	}
	if !got["name"].OptionalInPublicShape {
		t.Fatalf("field from optional embedded pointer should be optional")
	}
	if !got["name"].ViaOptionalEmbeddedField {
		t.Fatalf("field should be marked as coming through optional embedded pointer")
	}
	if got["id"].OptionalInPublicShape {
		t.Fatalf("sibling field should not inherit optional embedded state")
	}
}

func TestPublicStructFields_Dominance(t *testing.T) {
	type A struct {
		Name string `json:"name"`
	}
	type B struct {
		Name string `json:"name"`
	}
	string_type := reflect.TypeFor[string]()
	a_type := reflect.TypeFor[A]()
	b_type := reflect.TypeFor[B]()
	shallow_type := reflect.StructOf([]reflect.StructField{
		{Name: "A", Type: a_type, Anonymous: true},
		{Name: "Name", Type: string_type, Tag: `json:"name"`},
	})
	fields, err := PublicStructFields(shallow_type)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].FieldName != "Name" ||
		len(fields[0].Index) != 1 {
		t.Fatalf("expected shallow field to dominate: %#v", fields)
	}

	ambiguous_type := reflect.StructOf([]reflect.StructField{
		{Name: "A", Type: a_type, Anonymous: true},
		{Name: "B", Type: b_type, Anonymous: true},
	})
	fields, err = PublicStructFields(ambiguous_type)
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
	fields, err = PublicStructFields(reflect.TypeFor[tag_wins]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].Field.PkgPath != "" ||
		fields[0].Index[0] != 1 {
		t.Fatalf("expected tagged field to dominate: %#v", fields)
	}
}

func TestPublicStructFields_ExplicitInline(t *testing.T) {
	type Inline struct {
		Value string `json:"value"`
	}
	type sample struct {
		Inline Inline         `json:",inline"`
		ID     string         `json:"id"`
		Extra  map[string]any `json:",unknown"`
	}

	shape, err := PublicStructShape(reflect.TypeFor[sample]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make(map[string]StructField, len(shape.Fields))
	for _, field := range shape.Fields {
		got[field.PublicName] = field
	}
	if _, ok := got["value"]; !ok {
		t.Fatalf("expected explicit inline struct field to be flattened: %#v", shape.Fields)
	}
	if _, ok := got["id"]; !ok {
		t.Fatalf("expected normal field: %#v", shape.Fields)
	}
	if _, ok := got["Extra"]; ok {
		t.Fatalf("unknown fallback should not be a finite public field: %#v", shape.Fields)
	}
	if len(shape.Inlined) != 1 || shape.Inlined[0].FieldName != "Inline" {
		t.Fatalf("expected explicit inline field in shape: %#v", shape.Inlined)
	}
	if len(shape.Unknowns) != 1 || shape.Unknowns[0].FieldName != "Extra" {
		t.Fatalf("expected unknown fallback in shape: %#v", shape.Unknowns)
	}
}

func TestPublicStructFields_InvalidV2Tag(t *testing.T) {
	type sample struct {
		Dash string `json:"-,"`
	}
	if _, err := PublicStructFields(reflect.TypeFor[sample]()); err == nil {
		t.Fatalf("expected error for invalid v2 tag")
	}
}

func TestPublicStructFields_AnonymousNonStructRequiresName(t *testing.T) {
	type SampleString string
	type sample struct {
		SampleString
	}
	if _, err := PublicStructFields(reflect.TypeFor[sample]()); err == nil {
		t.Fatalf("expected error for anonymous non-struct without JSON name")
	}

	type named struct {
		SampleString `json:"sample"`
	}
	fields, err := PublicStructFields(reflect.TypeFor[named]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0].PublicName != "sample" {
		t.Fatalf("expected explicitly named anonymous field: %#v", fields)
	}
}

func TestPublicStructFields_NonStruct(t *testing.T) {
	if _, err := PublicStructFields(reflect.TypeFor[int]()); err == nil {
		t.Fatalf("expected error for non-struct type")
	}
}

func TestPublicStructFields_JSONV2InlinePointerDepth(t *testing.T) {
	type too_deep struct {
		X **struct{ A int } `json:",inline"`
	}
	if _, err := PublicStructFields(reflect.TypeFor[too_deep]()); err == nil {
		t.Fatalf("expected error for double pointer inline field")
	}

	type NamedPtr *struct{ A int }
	type named_ptr struct {
		X NamedPtr `json:",inline"`
	}
	if _, err := PublicStructFields(reflect.TypeFor[named_ptr]()); err == nil {
		t.Fatalf("expected error for named pointer inline field")
	}
}
