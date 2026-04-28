package tsgen

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

/////////////////////////////////////////////////////////////////////
/////// TEST HELPERS
/////////////////////////////////////////////////////////////////////

func resolve_types(t *testing.T, input_types ...*GoTypeSrc) ResolvedTSTypes {
	t.Helper()
	r := &GoTypeRegistry{}
	r.Add(input_types...)
	defs, err := r.ResolveTypes()
	if err != nil {
		t.Fatalf("ResolveTypes() error: %v", err)
	}
	return defs
}

func find_by_name(defs ResolvedTSTypes, name string) *ResolvedTSType {
	for _, td := range defs {
		if td.Name == name {
			cp := td
			return &cp
		}
	}
	return nil
}

var ws_re = regexp.MustCompile(`\s+`)

func norm(s string) string {
	return ws_re.ReplaceAllString(strings.TrimSpace(s), " ")
}

func assert_type(
	t *testing.T,
	defs ResolvedTSTypes,
	name, expected string,
) {
	t.Helper()
	td := find_by_name(defs, name)
	if td == nil {
		t.Errorf("type '%s' not found in results", name)
		return
	}
	if norm(td.Body) != norm(expected) {
		t.Errorf(
			"type '%s' mismatch:\n  got:  %s\n  want: %s",
			name,
			td.Body,
			expected,
		)
	}
}

func assert_absent(t *testing.T, defs ResolvedTSTypes, name string) {
	t.Helper()
	if find_by_name(defs, name) != nil {
		t.Errorf("type '%s' should be absent but was found", name)
	}
}

func assert_named_count(t *testing.T, defs ResolvedTSTypes, expected int) {
	t.Helper()
	n := 0
	for _, d := range defs {
		if d.Name != "" {
			n++
		}
	}
	if n != expected {
		t.Errorf("expected %d named types, got %d", expected, n)
	}
}

func contains_any(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

/////////////////////////////////////////////////////////////////////
/////// FILE-SCOPE TEST STRUCTS
/////////////////////////////////////////////////////////////////////

// Custom type overrides (TSTyper method)

type WithCustomOverrides struct {
	A string `ts_type:"MyCustomString"`
	B int
}

func (w WithCustomOverrides) TSType() map[string]string {
	return map[string]string{"A": "OverriddenByMethod", "B": "MyCustomNumber"}
}

// TSTyper reference + embedding coexistence

type SharedComponent struct {
	Name string `json:"name"`
}

type EmbeddingHost struct {
	ID int `json:"id"`
	SharedComponent
}

var shared_component_type = &GoTypeSrc{Instance: SharedComponent{}}

type TSTyperRefHost struct {
	ReferenceField int
}

func (t TSTyperRefHost) TSType() map[string]string {
	return map[string]string{
		"ReferenceField": string(shared_component_type.ID()),
	}
}

// Deeply nested flattening with overrides

type DeepBottom struct {
	FieldC string `json:"field_c" ts_type:"overridden_c"`
}

type DeepMiddle struct {
	DeepBottom
	FieldB string `json:"field_b"`
}

func (m DeepMiddle) TSType() map[string]string {
	return map[string]string{"FieldB": "overridden_b"}
}

type DeepTop struct {
	DeepMiddle
	FieldA string `json:"field_a"`
}

// TSType method overrides ts_type struct tag

type TagVsMethodStruct struct {
	X string `json:"x" ts_type:"from_tag"`
}

func (s TagVsMethodStruct) TSType() map[string]string {
	return map[string]string{"X": "from_method"}
}

// TSType on value receiver (no json tags)

type TSTypeBase struct {
	A int    `ts_type:"qwer"`
	B string `ts_type:"tyui"`
	X string
}

func (b TSTypeBase) TSType() map[string]string {
	return map[string]string{"X": "asdf"}
}

// TSType on pointer receiver (no json tags)

type TSTypePtrMethod struct {
	A int    `ts_type:"qwer"`
	B string `ts_type:"tyui"`
	X string
}

func (b *TSTypePtrMethod) TSType() map[string]string {
	return map[string]string{"X": "asdf"}
}

// Wrapped variants (no json tags)

type TSTypeBaseWrapped struct{ TSTypeBase }
type TSTypeBasePtrWrapped struct{ *TSTypeBase }
type TSTypePtrMethodWrapped struct{ TSTypePtrMethod }
type TSTypePtrMethodPtrWrapped struct{ *TSTypePtrMethod }

// TSType on value receiver (with json tags)

type TSTypeBaseJson struct {
	A int    `json:"a" ts_type:"qwer"`
	B string `json:"b" ts_type:"tyui"`
	X string `json:"x"`
}

func (b TSTypeBaseJson) TSType() map[string]string {
	return map[string]string{"X": "asdf"}
}

// TSType on pointer receiver (with json tags)

type TSTypePtrMethodJson struct {
	A int    `json:"a" ts_type:"qwer"`
	B string `json:"b" ts_type:"tyui"`
	X string `json:"x"`
}

func (b *TSTypePtrMethodJson) TSType() map[string]string {
	return map[string]string{"X": "asdf"}
}

// Wrapped variants (with json tags)

type TSTypeBaseWrappedJson struct{ TSTypeBaseJson }
type TSTypeBasePtrWrappedJson struct{ *TSTypeBaseJson }
type TSTypePtrMethodWrappedJson struct{ TSTypePtrMethodJson }
type TSTypePtrMethodPtrWrappedJson struct{ *TSTypePtrMethodJson }

type ResolveAliasAlpha struct {
	Next *ResolveAliasBeta `json:"next"`
}

type ResolveAliasBeta struct {
	Next *ResolveAliasAlpha `json:"next"`
}

type ResolveMultiAliasShared struct {
	Name string `json:"name"`
}

type ResolveMultiAliasHost struct {
	Value ResolveMultiAliasShared `json:"value"`
}

type ResolveNestedRawDep struct{}

func (ResolveNestedRawDep) TSType() string {
	return "{ custom: boolean }"
}

type ResolveNestedRawRefTarget struct {
	Name string `json:"name"`
}

type ResolveNestedRawRefDep struct{}

func (ResolveNestedRawRefDep) TSType() string {
	return fmt.Sprintf(
		"{ target: %s; }",
		GoType[ResolveNestedRawRefTarget]().ID(),
	)
}

/////////////////////////////////////////////////////////////////////
/////// TESTS: Type.ID() AND MAP LOOKUP
/////////////////////////////////////////////////////////////////////

func TestTypeID(t *testing.T) {
	type A struct{ X int }
	type B struct{ Y string }

	t.Run("SameTypeAndName", func(t *testing.T) {
		t1 := &GoTypeSrc{Instance: A{}, RequestedName: "Foo"}
		t2 := &GoTypeSrc{Instance: A{}, RequestedName: "Foo"}
		if t1.ID() != t2.ID() {
			t.Errorf(
				"same type+name should produce same ID: %q vs %q",
				t1.ID(),
				t2.ID(),
			)
		}
	})

	t.Run("DifferentTypeSameName", func(t *testing.T) {
		t1 := &GoTypeSrc{Instance: A{}, RequestedName: "Foo"}
		t2 := &GoTypeSrc{Instance: B{}, RequestedName: "Foo"}
		if t1.ID() == t2.ID() {
			t.Error(
				"different types with same name should produce different IDs",
			)
		}
	})

	t.Run("SameTypeDifferentName", func(t *testing.T) {
		t1 := &GoTypeSrc{Instance: A{}, RequestedName: "Foo"}
		t2 := &GoTypeSrc{Instance: A{}, RequestedName: "Bar"}
		if t1.ID() == t2.ID() {
			t.Error(
				"same type with different names should produce different IDs",
			)
		}
	})

	t.Run("PointerAndValueProduceSameID", func(t *testing.T) {
		t1 := &GoTypeSrc{Instance: A{}, RequestedName: "Foo"}
		t2 := &GoTypeSrc{Instance: &A{}, RequestedName: "Foo"}
		if t1.ID() != t2.ID() {
			t.Errorf(
				"pointer and value should produce same ID: %q vs %q",
				t1.ID(),
				t2.ID(),
			)
		}
	})

	t.Run("NewTypeMatchesStructLiteral", func(t *testing.T) {
		from_literal := &GoTypeSrc{Instance: A{}, RequestedName: "Foo"}
		from_generic := GoType[A]("Foo")
		if from_literal.ID() != from_generic.ID() {
			t.Error("NewType[T] and Type literal should produce the same ID")
		}
	})

	t.Run("EmptyNameDeterministic", func(t *testing.T) {
		t1 := &GoTypeSrc{Instance: A{}}
		t2 := &GoTypeSrc{Instance: A{}}
		if t1.ID() != t2.ID() {
			t.Errorf(
				"empty name should be deterministic: %q vs %q",
				t1.ID(),
				t2.ID(),
			)
		}
	})
}

func TestResolveMapLookup(t *testing.T) {
	type Inner struct {
		V string `json:"v"`
	}
	type Outer struct {
		I Inner `json:"i"`
	}

	t.Run("LookupByID", func(t *testing.T) {
		input := &GoTypeSrc{Instance: Outer{}, RequestedName: "MyOuter"}
		defs := resolve_types(t, input)

		td, ok := defs[input.ID()]
		if !ok {
			t.Fatal("Type.ID() not found in ResolveTypes() map")
		}
		if td.Name != "MyOuter" {
			t.Errorf("Name=%q, want 'MyOuter'", td.Name)
		}
		if !strings.Contains(td.Body, "I:") &&
			!strings.Contains(td.Body, "i:") {
			t.Error("Body should contain the field definition")
		}
	})

	t.Run("LookupDiscoveredDependency", func(t *testing.T) {
		input := &GoTypeSrc{Instance: Outer{}, RequestedName: "MyOuter"}
		defs := resolve_types(t, input)

		// Inner was not explicitly Add'd, but it was discovered as a
		// dependency. Its ID is based on its natural name.
		inner_id := GoType[Inner]().ID()
		td, ok := defs[inner_id]
		if !ok {
			t.Fatal("discovered dependency not found via NewType[T]().ID()")
		}
		if td.Name != "Inner" {
			t.Errorf("Name=%q, want 'Inner'", td.Name)
		}
	})

	t.Run("LookupAfterNameCollision", func(t *testing.T) {
		type X struct{ A int }
		type Y struct{ B string }

		tx := &GoTypeSrc{Instance: X{}, RequestedName: "Same"}
		ty := &GoTypeSrc{Instance: Y{}, RequestedName: "Same"}
		defs := resolve_types(t, tx, ty)

		td_x, ok := defs[tx.ID()]
		if !ok {
			t.Fatal("first type not found")
		}
		td_y, ok := defs[ty.ID()]
		if !ok {
			t.Fatal("second type not found")
		}

		if td_x.Name == td_y.Name {
			t.Error("collision resolution should produce different names")
		}
	})
}

/////////////////////////////////////////////////////////////////////
/////// TESTS: SENTINEL RESOLUTION VIA TSTyper
/////////////////////////////////////////////////////////////////////

func TestResolveSentinelInTSTyper(t *testing.T) {
	// TSTyperRefHost uses shared_component_type.ID() as a TSTyper
	// value. The sentinel-wrapped ID should cause SharedComponent to
	// be kept in the output and resolved to its name.
	defs := resolve_types(t,
		&GoTypeSrc{Instance: EmbeddingHost{}},
		&GoTypeSrc{Instance: TSTyperRefHost{}},
	)
	assert_type(t, defs, "EmbeddingHost", `{ id: number; name: string; }`)
	assert_type(t, defs, "SharedComponent", `{ name: string; }`)
	assert_type(
		t,
		defs,
		"TSTyperRefHost",
		`{ ReferenceField: SharedComponent; }`,
	)
}

/////////////////////////////////////////////////////////////////////
/////// TESTS: CORE TYPE RESOLUTION
/////////////////////////////////////////////////////////////////////

func TestResolve(t *testing.T) {
	t.Run("BasicTypesAndNaming", func(t *testing.T) {
		type Basic struct {
			IntField    int
			StringField string
			BoolField   bool
			TimeField   time.Time
		}
		defs := resolve_types(t,
			&GoTypeSrc{Instance: Basic{}, RequestedName: "MyBasicType"},
		)
		assert_type(t, defs, "MyBasicType", `{
			IntField: number;
			StringField: string;
			BoolField: boolean;
			TimeField: string;
		}`)
	})

	t.Run("AllPrimitiveTypes", func(t *testing.T) {
		type AllPrimitives struct {
			BoolField    bool    `json:"boolField"`
			IntField     int     `json:"intField"`
			Int8Field    int8    `json:"int8Field"`
			Int16Field   int16   `json:"int16Field"`
			Int32Field   int32   `json:"int32Field"`
			Int64Field   int64   `json:"int64Field"`
			UintField    uint    `json:"uintField"`
			Uint8Field   uint8   `json:"uint8Field"`
			Uint16Field  uint16  `json:"uint16Field"`
			Uint32Field  uint32  `json:"uint32Field"`
			Uint64Field  uint64  `json:"uint64Field"`
			Float32Field float32 `json:"float32Field"`
			Float64Field float64 `json:"float64Field"`
			StringField  string  `json:"stringField"`
			ByteField    byte    `json:"byteField"`
			RuneField    rune    `json:"runeField"`
		}
		defs := resolve_types(t,
			&GoTypeSrc{Instance: AllPrimitives{}, RequestedName: "Primitives"},
		)
		td := find_by_name(defs, "Primitives")
		if td == nil {
			t.Fatal("type not found")
		}
		if !strings.Contains(td.Body, "boolField: boolean") {
			t.Error("bool not typed correctly")
		}
		if !strings.Contains(td.Body, "stringField: string") {
			t.Error("string not typed correctly")
		}
		numerics := []string{
			"intField", "int8Field", "int16Field", "int32Field", "int64Field",
			"uintField", "uint8Field", "uint16Field", "uint32Field", "uint64Field",
			"float32Field", "float64Field", "byteField", "runeField",
		}
		for _, f := range numerics {
			if !strings.Contains(td.Body, f+": number") {
				t.Errorf("field %s not typed as number", f)
			}
		}
	})

	t.Run("JSONTagHandling", func(t *testing.T) {
		type WithTags struct {
			Renamed     string `json:"field_one"`
			Optional    int    `json:"fieldTwo,omitempty"`
			Ignored     bool   `json:"-"`
			Pointer     *bool  `json:"pointerField"`
			DashName    any    `json:"'-'"`
			OmitZeroVal int    `json:"zeroValField,omitzero"`
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: WithTags{}})
		assert_type(t, defs, "WithTags", `{
			field_one: string;
			fieldTwo?: number;
			pointerField?: boolean;
			"-": unknown;
			zeroValField?: number;
		}`)
	})

	t.Run("CollectionsAndPointers", func(t *testing.T) {
		type Inner struct {
			Renamed string `json:"field_one"`
		}
		type Outer struct {
			IntSlice    []int
			StringArray [2]string
			StructPtr   *Inner
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: Outer{}})
		assert_type(t, defs, "Outer", `{
			IntSlice: Array<number>;
			StringArray: Array<string>;
			StructPtr?: Inner;
		}`)
		assert_type(t, defs, "Inner", `{ field_one: string; }`)
	})

	t.Run("PointerFieldsAreOptional", func(t *testing.T) {
		type WithPointers struct {
			StringPtr *string         `json:"stringPtr"`
			IntPtr    *int            `json:"intPtr"`
			BoolPtr   *bool           `json:"boolPtr"`
			SlicePtr  *[]int          `json:"slicePtr"`
			MapPtr    *map[string]int `json:"mapPtr"`
		}
		defs := resolve_types(t,
			&GoTypeSrc{Instance: WithPointers{}, RequestedName: "Ptrs"},
		)
		td := find_by_name(defs, "Ptrs")
		if td == nil {
			t.Fatal("type not found")
		}
		if !strings.Contains(td.Body, "stringPtr?: string") {
			t.Error("string pointer not optional")
		}
		if !strings.Contains(td.Body, "intPtr?: number") {
			t.Error("int pointer not optional")
		}
		if !strings.Contains(td.Body, "boolPtr?: boolean") {
			t.Error("bool pointer not optional")
		}
		if !strings.Contains(td.Body, "slicePtr?: Array<number>") {
			t.Error("slice pointer not handled correctly")
		}
		if !strings.Contains(td.Body, "mapPtr?: Record<string, number>") {
			t.Error("map pointer not handled correctly")
		}
	})

	t.Run("MapHandling", func(t *testing.T) {
		type WithMaps struct {
			StringKey map[string]int
			IntKey    map[int]string
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: WithMaps{}})
		assert_type(t, defs, "WithMaps", `{
			StringKey: Record<string, number>;
			IntKey: Record<number, string>;
		}`)
	})

	t.Run("InterfaceAndEmptyStruct", func(t *testing.T) {
		type WithEmpty struct {
			AnyField  any
			EmptyData struct{}
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: WithEmpty{}})
		assert_type(t, defs, "WithEmpty", `{
			AnyField: unknown;
			EmptyData: Record<never, never>;
		}`)
	})

	t.Run("TimeFields", func(t *testing.T) {
		type WithTime struct {
			Created time.Time  `json:"created"`
			Updated *time.Time `json:"updated"`
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: WithTime{}})
		td := find_by_name(defs, "WithTime")
		if td == nil {
			t.Fatal("type not found")
		}
		if !strings.Contains(td.Body, "created: string") {
			t.Error("time.Time not mapped to string")
		}
		if !strings.Contains(td.Body, "updated?: string") {
			t.Error("*time.Time not mapped to optional string")
		}
	})

	t.Run("DurationField", func(t *testing.T) {
		type WithDuration struct {
			Dur time.Duration `json:"dur"`
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: WithDuration{}})
		td := find_by_name(defs, "WithDuration")
		if td == nil {
			t.Fatal("type not found")
		}
		if !strings.Contains(td.Body, "dur: number") {
			t.Error("time.Duration not mapped to number")
		}
	})

	t.Run("ByteSliceIsString", func(t *testing.T) {
		type WithBytes struct {
			Data []byte `json:"data"`
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: WithBytes{}})
		assert_type(t, defs, "WithBytes", `{ data: string; }`)
	})

	t.Run("NameCollisions", func(t *testing.T) {
		type TypeA struct{ A int }
		type TypeB struct{ B string }
		defs := resolve_types(t,
			&GoTypeSrc{Instance: TypeA{}, RequestedName: "Collision"},
			&GoTypeSrc{Instance: TypeB{}, RequestedName: "Collision"},
		)
		ta := defs[GoType[TypeA]("Collision").ID()]
		tb := defs[GoType[TypeB]("Collision").ID()]
		if ta.Name == tb.Name {
			t.Error("collision resolution should produce different names")
		}
		if norm(ta.Body) != norm("{ A: number; }") {
			t.Errorf("TypeA body mismatch: %s", ta.Body)
		}
		if norm(tb.Body) != norm("{ B: string; }") {
			t.Errorf("TypeB body mismatch: %s", tb.Body)
		}
	})

	t.Run("ThreeWayNameCollision", func(t *testing.T) {
		type T1 struct{ Field1 string }
		type T2 struct{ Field2 int }
		type T3 struct{ Field3 int }
		defs := resolve_types(t,
			&GoTypeSrc{Instance: T1{}, RequestedName: "Same"},
			&GoTypeSrc{Instance: T2{}, RequestedName: "Same"},
			&GoTypeSrc{Instance: T3{}, RequestedName: "Same"},
		)
		td1 := defs[GoType[T1]("Same").ID()]
		td2 := defs[GoType[T2]("Same").ID()]
		td3 := defs[GoType[T3]("Same").ID()]
		if td1.Name == td2.Name || td1.Name == td3.Name ||
			td2.Name == td3.Name {
			t.Error("all three collision names should be distinct")
		}
		if norm(td1.Body) != norm("{ Field1: string; }") {
			t.Errorf("T1 body mismatch: %s", td1.Body)
		}
		if norm(td2.Body) != norm("{ Field2: number; }") {
			t.Errorf("T2 body mismatch: %s", td2.Body)
		}
		if norm(td3.Body) != norm("{ Field3: number; }") {
			t.Errorf("T3 body mismatch: %s", td3.Body)
		}
	})

	t.Run("CustomTypeOverrides", func(t *testing.T) {
		defs := resolve_types(t, &GoTypeSrc{Instance: WithCustomOverrides{}})
		assert_type(t, defs, "WithCustomOverrides", `{
			A: OverriddenByMethod;
			B: MyCustomNumber;
		}`)
	})

	t.Run("TSTypeMethodOverridesStructTag", func(t *testing.T) {
		defs := resolve_types(t, &GoTypeSrc{Instance: TagVsMethodStruct{}})
		td := find_by_name(defs, "TagVsMethodStruct")
		if td == nil {
			t.Fatal("type not found")
		}
		if strings.Contains(td.Body, "from_tag") {
			t.Error(
				"ts_type struct tag should be overridden by TSType() method",
			)
		}
		if !strings.Contains(td.Body, "from_method") {
			t.Error("TSType() method value should appear")
		}
	})

	t.Run("Generics", func(t *testing.T) {
		type Product struct {
			Name  string `json:"name"`
			Price int    `json:"price"`
		}
		type User struct {
			Email string `json:"email"`
		}
		type PagedResult[T any] struct {
			Items []T `json:"items"`
			Total int `json:"total"`
		}
		defs := resolve_types(t,
			&GoTypeSrc{
				Instance:      PagedResult[Product]{},
				RequestedName: "ProductPage",
			},
			&GoTypeSrc{
				Instance:      PagedResult[User]{},
				RequestedName: "UserPage",
			},
		)
		assert_type(
			t,
			defs,
			"ProductPage",
			`{ items: Array<Product>; total: number; }`,
		)
		assert_type(
			t,
			defs,
			"UserPage",
			`{ items: Array<User>; total: number; }`,
		)
		assert_type(t, defs, "Product", `{ name: string; price: number; }`)
		assert_type(t, defs, "User", `{ email: string; }`)
		assert_absent(t, defs, "PagedResult")
	})

	t.Run("NilInstance", func(t *testing.T) {
		defs := resolve_types(t,
			&GoTypeSrc{Instance: nil, RequestedName: "ShouldBeAbsent"},
		)
		assert_absent(t, defs, "ShouldBeAbsent")
	})
}

/////////////////////////////////////////////////////////////////////
/////// TESTS: EMBEDDING SCENARIOS
/////////////////////////////////////////////////////////////////////

func TestResolveEmbedding(t *testing.T) {
	type Base struct {
		Name string `json:"name"`
	}
	type PtrBase struct {
		Age int `json:"age"`
	}

	t.Run("FlatteningNoTags", func(t *testing.T) {
		type Host struct {
			ID string `json:"id"`
			Base
			*PtrBase
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: Host{}})
		assert_type(
			t,
			defs,
			"Host",
			`{ id: string; name: string; age?: number; }`,
		)
		assert_absent(t, defs, "Base")
		assert_absent(t, defs, "PtrBase")
		assert_named_count(t, defs, 1)
	})

	t.Run("NestingWithTags", func(t *testing.T) {
		type Host struct {
			ID string   `json:"id"`
			B  Base     `json:"b"`
			PB *PtrBase `json:"pb"`
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: Host{}})
		assert_type(t, defs, "Host", `{ id: string; b: Base; pb?: PtrBase; }`)
		assert_type(t, defs, "Base", `{ name: string; }`)
		assert_type(t, defs, "PtrBase", `{ age: number; }`)
		assert_named_count(t, defs, 3)
	})

	t.Run("MixedFlattenAndNest", func(t *testing.T) {
		type Host struct {
			ID   string   `json:"id"`
			Base          // flattened
			PB   *PtrBase `json:"pb"` // nested
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: Host{}})
		assert_type(
			t,
			defs,
			"Host",
			`{ id: string; name: string; pb?: PtrBase; }`,
		)
		assert_absent(t, defs, "Base")
		assert_type(t, defs, "PtrBase", `{ age: number; }`)
		assert_named_count(t, defs, 2)
	})

	t.Run("DeeplyNestedWithOverrides", func(t *testing.T) {
		defs := resolve_types(t, &GoTypeSrc{Instance: DeepTop{}})
		assert_type(t, defs, "DeepTop", `{
			field_c: overridden_c;
			field_b: overridden_b;
			field_a: string;
		}`)
		assert_absent(t, defs, "DeepMiddle")
		assert_absent(t, defs, "DeepBottom")
	})

	t.Run("ReferencedAndEmbeddedCoexistence", func(t *testing.T) {
		type Base struct {
			Name string `json:"name"`
		}
		type Flattener struct {
			ID string `json:"id"`
			Base
		}
		type Referencer struct {
			B Base `json:"b"`
		}
		defs := resolve_types(t,
			&GoTypeSrc{Instance: Flattener{}},
			&GoTypeSrc{Instance: Referencer{}},
		)
		assert_type(t, defs, "Flattener", `{ id: string; name: string; }`)
		assert_type(t, defs, "Referencer", `{ b: Base; }`)
		assert_type(t, defs, "Base", `{ name: string; }`)
	})
}

/////////////////////////////////////////////////////////////////////
/////// TESTS: ADVANCED SCENARIOS
/////////////////////////////////////////////////////////////////////

func TestResolveAdvanced(t *testing.T) {
	t.Run("EarlyFilteringOfJsonDash", func(t *testing.T) {
		type Component struct {
			Value string `json:"value"`
		}
		type FieldWithEmbedded struct {
			Component
		}
		type Host struct {
			Ignored FieldWithEmbedded `json:"-"`
			Component
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: Host{}})
		assert_type(t, defs, "Host", `{ value: string; }`)
		assert_absent(t, defs, "FieldWithEmbedded")
		assert_absent(t, defs, "Component")
	})

	t.Run("NestedStructReference", func(t *testing.T) {
		type Address struct {
			Street  string `json:"street"`
			City    string `json:"city"`
			Country string `json:"country"`
		}
		type Customer struct {
			ID      int     `json:"id"`
			Name    string  `json:"name"`
			Address Address `json:"address"`
		}
		defs := resolve_types(t,
			&GoTypeSrc{Instance: Customer{}, RequestedName: "Customer"},
		)
		td := find_by_name(defs, "Customer")
		if td == nil {
			t.Fatal("type not found")
		}
		if !strings.Contains(td.Body, "address: Address") {
			t.Error("nested struct should be referenced by name")
		}
		assert_type(
			t,
			defs,
			"Address",
			`{ street: string; city: string; country: string; }`,
		)
	})

	t.Run("StructMapValues", func(t *testing.T) {
		type Person struct {
			Name string `json:"name"`
		}
		type WithMaps struct {
			StringToPerson map[string]Person `json:"stringToPerson"`
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: WithMaps{}})
		td := find_by_name(defs, "WithMaps")
		if td == nil {
			t.Fatal("type not found")
		}
		if !strings.Contains(td.Body, "stringToPerson: Record<string,") {
			t.Error("map with struct value not handled")
		}
	})

	t.Run("SliceOfStructs", func(t *testing.T) {
		type Person struct {
			Name string `json:"name"`
		}
		type WithCollections struct {
			StructSlice []Person  `json:"structSlice"`
			StructArray [2]Person `json:"structArray"`
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: WithCollections{}})
		td := find_by_name(defs, "WithCollections")
		if td == nil {
			t.Fatal("type not found")
		}
		if !strings.Contains(td.Body, "structSlice: Array<") {
			t.Error("struct slice not handled")
		}
		if !strings.Contains(td.Body, "structArray: Array<") {
			t.Error("struct array not handled")
		}
	})

	t.Run("SliceOfPointerToStruct", func(t *testing.T) {
		type Inner struct {
			V string `json:"v"`
		}
		type Outer struct {
			Items []*Inner `json:"items"`
		}
		defs := resolve_types(t, &GoTypeSrc{Instance: Outer{}})
		assert_type(t, defs, "Outer", `{ items: Array<Inner>; }`)
		assert_type(t, defs, "Inner", `{ v: string; }`)
	})

	t.Run("NamedCollectionAliasesAreReferenced", func(t *testing.T) {
		type Person struct {
			Name string `json:"name"`
		}
		type PersonList []Person
		type PersonIndex map[string]Person
		type PersonArray [2]Person
		type Host struct {
			Items PersonList   `json:"items"`
			Index PersonIndex  `json:"index"`
			Fixed *PersonArray `json:"fixed"`
		}

		defs := resolve_types(t, &GoTypeSrc{Instance: Host{}})
		assert_type(t, defs, "Host", `{
			items: PersonList;
			index: PersonIndex;
			fixed?: PersonArray;
		}`)
		assert_type(t, defs, "PersonList", `Array<Person>`)
		assert_type(t, defs, "PersonIndex", `Record<string, Person>`)
		assert_type(t, defs, "PersonArray", `Array<Person>`)
		assert_type(t, defs, "Person", `{ name: string; }`)
	})

	t.Run("ExplicitRootAliasWinsOverDiscoveredNaturalDuplicate", func(t *testing.T) {
		defs := resolve_types(t,
			&GoTypeSrc{Instance: ResolveAliasAlpha{}, RequestedName: "AliasAlpha"},
			&GoTypeSrc{Instance: ResolveAliasBeta{}, RequestedName: "AliasBeta"},
		)

		assert_named_count(t, defs, 2)
		assert_absent(t, defs, "ResolveAliasAlpha")
		assert_absent(t, defs, "ResolveAliasBeta")
		assert_type(t, defs, "AliasAlpha", `{ next?: AliasBeta; }`)
		assert_type(t, defs, "AliasBeta", `{ next?: AliasAlpha; }`)
	})

	t.Run("MultiAliasReferencesUseLexicallySmallestAlias", func(t *testing.T) {
		defs := resolve_types(t,
			&GoTypeSrc{Instance: ResolveMultiAliasShared{}, RequestedName: "AliasZ"},
			&GoTypeSrc{Instance: ResolveMultiAliasShared{}, RequestedName: "AliasA"},
			&GoTypeSrc{Instance: ResolveMultiAliasHost{}, RequestedName: "Host"},
		)

		assert_type(t, defs, "Host", `{ value: AliasA; }`)
		assert_type(t, defs, "AliasA", `{ name: string; }`)
		assert_type(t, defs, "AliasZ", `{ name: string; }`)
	})

	t.Run("DiscoveredNestedTSTyperRawUsesRawBody", func(t *testing.T) {
		type Host struct {
			Dep ResolveNestedRawDep `json:"dep"`
		}

		defs := resolve_types(t, &GoTypeSrc{Instance: Host{}, RequestedName: "Host"})
		assert_type(t, defs, "Host", `{ dep: ResolveNestedRawDep; }`)
		assert_type(t, defs, "ResolveNestedRawDep", `{ custom: boolean }`)
	})

	t.Run("DiscoveredNestedTSTyperRawSentinelKeepsReferencedType", func(t *testing.T) {
		type Host struct {
			Dep ResolveNestedRawRefDep `json:"dep"`
		}

		defs := resolve_types(t,
			&GoTypeSrc{Instance: Host{}, RequestedName: "Host"},
			&GoTypeSrc{Instance: ResolveNestedRawRefTarget{}},
		)
		assert_type(t, defs, "Host", `{ dep: ResolveNestedRawRefDep; }`)
		assert_type(t, defs, "ResolveNestedRawRefDep", `{ target: ResolveNestedRawRefTarget; }`)
		assert_type(t, defs, "ResolveNestedRawRefTarget", `{ name: string; }`)
	})

	t.Run("NamedScalarAliasesAreReferenced", func(t *testing.T) {
		type UserID string
		type Count int
		type Host struct {
			ID    UserID `json:"id"`
			Count Count  `json:"count"`
		}

		defs := resolve_types(t,
			&GoTypeSrc{Instance: UserID(""), RequestedName: "UserID"},
			&GoTypeSrc{Instance: Count(0), RequestedName: "Count"},
			&GoTypeSrc{Instance: Host{}, RequestedName: "Host"},
		)
		assert_type(t, defs, "Host", `{ id: UserID; count: Count; }`)
		assert_type(t, defs, "UserID", `string`)
		assert_type(t, defs, "Count", `number`)
	})

	t.Run("NamedTimeLikeWrappersKeepBuiltinShape", func(t *testing.T) {
		type CreatedAt time.Time
		type Elapsed time.Duration
		type Host struct {
			Created CreatedAt `json:"created"`
			Took    Elapsed   `json:"took"`
		}

		defs := resolve_types(t,
			&GoTypeSrc{Instance: CreatedAt(time.Time{}), RequestedName: "CreatedAt"},
			&GoTypeSrc{Instance: Elapsed(0), RequestedName: "Elapsed"},
			&GoTypeSrc{Instance: Host{}, RequestedName: "Host"},
		)
		assert_type(t, defs, "Host", `{ created: CreatedAt; took: Elapsed; }`)
		assert_type(t, defs, "CreatedAt", `string`)
		assert_type(t, defs, "Elapsed", `number`)
	})
}

/////////////////////////////////////////////////////////////////////
/////// TESTS: TS TYPE / ts_type TAG MATRIX
/////////////////////////////////////////////////////////////////////

func TestResolveTSTypeVariants(t *testing.T) {
	variants := []struct {
		label    string
		instance any
	}{
		// Without json tags
		{"TSTypeBase", TSTypeBase{}},
		{"*TSTypeBase", &TSTypeBase{}},
		{"TSTypePtrMethod", TSTypePtrMethod{}},
		{"*TSTypePtrMethod", &TSTypePtrMethod{}},
		{"TSTypeBaseWrapped", TSTypeBaseWrapped{}},
		{"*TSTypeBaseWrapped", &TSTypeBaseWrapped{}},
		{"TSTypeBasePtrWrapped", TSTypeBasePtrWrapped{}},
		{"*TSTypeBasePtrWrapped", &TSTypeBasePtrWrapped{}},
		{"TSTypePtrMethodWrapped", TSTypePtrMethodWrapped{}},
		{"*TSTypePtrMethodWrapped", &TSTypePtrMethodWrapped{}},
		{"TSTypePtrMethodPtrWrapped", TSTypePtrMethodPtrWrapped{}},
		{"*TSTypePtrMethodPtrWrapped", &TSTypePtrMethodPtrWrapped{}},

		// With json tags
		{"TSTypeBaseJson", TSTypeBaseJson{}},
		{"*TSTypeBaseJson", &TSTypeBaseJson{}},
		{"TSTypePtrMethodJson", TSTypePtrMethodJson{}},
		{"*TSTypePtrMethodJson", &TSTypePtrMethodJson{}},
		{"TSTypeBaseWrappedJson", TSTypeBaseWrappedJson{}},
		{"*TSTypeBaseWrappedJson", &TSTypeBaseWrappedJson{}},
		{"TSTypeBasePtrWrappedJson", TSTypeBasePtrWrappedJson{}},
		{"*TSTypeBasePtrWrappedJson", &TSTypeBasePtrWrappedJson{}},
		{"TSTypePtrMethodWrappedJson", TSTypePtrMethodWrappedJson{}},
		{"*TSTypePtrMethodWrappedJson", &TSTypePtrMethodWrappedJson{}},
		{"TSTypePtrMethodPtrWrappedJson", TSTypePtrMethodPtrWrappedJson{}},
		{"*TSTypePtrMethodPtrWrappedJson", &TSTypePtrMethodPtrWrappedJson{}},
	}

	for _, v := range variants {
		t.Run(v.label, func(t *testing.T) {
			defs := resolve_types(t, &GoTypeSrc{Instance: v.instance})
			if len(defs) == 0 {
				t.Fatal("no types resolved")
			}

			var all strings.Builder
			for _, d := range defs {
				all.WriteString(d.Body)
				all.WriteString("\n")
			}
			combined := all.String()

			if !contains_any(
				combined,
				"a: qwer",
				"a?: qwer",
				"A: qwer",
				"A?: qwer",
			) {
				t.Errorf("ts_type override for A not found:\n%s", combined)
			}
			if !contains_any(
				combined,
				"b: tyui",
				"b?: tyui",
				"B: tyui",
				"B?: tyui",
			) {
				t.Errorf("ts_type override for B not found:\n%s", combined)
			}
			if !contains_any(
				combined,
				"x: asdf",
				"x?: asdf",
				"X: asdf",
				"X?: asdf",
			) {
				t.Errorf("TSType() override for X not found:\n%s", combined)
			}
		})
	}
}
