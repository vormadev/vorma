package tsgencore

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TEST HELPER FUNCTIONS

// processAdHoc is a helper to run ProcessTypes for a set of test types.
func processAdHoc(t *testing.T, adHocTypes ...*AdHocType) Results {
	t.Helper()
	return ProcessTypes(adHocTypes)
}

// findTypeByName searches the results for a type with a specific resolved name.
func findTypeByName(t *testing.T, r Results, name string) *TypeInfo {
	t.Helper()
	for _, typeInfo := range r.Types {
		if typeInfo.ResolvedName == name {
			return typeInfo
		}
	}
	return nil
}

// normalize removes all whitespace to allow for exact string comparison of TS types.
func normalize(s string) string {
	ws := regexp.MustCompile(`\s+`)
	s = strings.ReplaceAll(s, "\t", "")
	return ws.ReplaceAllString(s, "")
}

// assertType asserts that a type with a given name exists and has the expected TS string definition.
func assertType(t *testing.T, r Results, name, expectedTS string) {
	t.Helper()
	typeInfo := findTypeByName(t, r, name)
	if typeInfo == nil {
		t.Errorf("FAIL: Expected to find type '%s', but it was not found.", name)
		return
	}
	if normalize(typeInfo.TSStr) != normalize(expectedTS) {
		t.Errorf("FAIL: Mismatch for type '%s'.\n- Got:  %s\n- Want: %s", name, typeInfo.TSStr, expectedTS)
	}
}

// assertNotExported asserts that a type with a given name was NOT exported.
func assertNotExported(t *testing.T, r Results, name string) {
	t.Helper()
	if typeInfo := findTypeByName(t, r, name); typeInfo != nil {
		t.Errorf("FAIL: Expected type '%s' to NOT be exported, but it was.", name)
	}
}

// TEST STRUCT AND METHOD DEFINITIONS

// For Custom Type Overrides test
type WithCustom struct {
	A string `ts_type:"MyCustomString"`
	B int
}

func (w WithCustom) TSType() map[string]string {
	return map[string]string{
		"A": "OverriddenByMethod",
		"B": "MyCustomNumber",
	}
}

// For Advanced Scenarios: TSTyper and Embedding Coexistence Test
type SharedComponent struct {
	Name string `json:"name"`
}
type EmbeddingHost struct { // This struct wants to flatten SharedComponent
	ID int `json:"id"`
	SharedComponent
}
type TSTyperHost struct { // This struct wants to reference SharedComponent
	ReferenceField int // This field's type will be overridden
}

func (t TSTyperHost) TSType() map[string]string {
	idForB := getIDFromReflectType(reflect.TypeOf(SharedComponent{}), "")
	return map[string]string{"ReferenceField": idForB}
}

// For Advanced Scenarios: Deeply Nested Flattening with Overrides
type BottomLevel struct {
	FieldC string `json:"field_c" ts_type:"overridden_c"`
}
type MiddleLevel struct {
	BottomLevel
	FieldB string `json:"field_b"`
}

func (m MiddleLevel) TSType() map[string]string {
	return map[string]string{"FieldB": "overridden_b"}
}

type TopLevel struct {
	MiddleLevel
	FieldA string `json:"field_a"`
}

// TEST SUITE

func TestTsgencoreComprehensive(t *testing.T) {
	t.Run("Basic Types and Naming", func(t *testing.T) {
		type Basic struct {
			IntField    int
			StringField string
			BoolField   bool
			TimeField   time.Time
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: Basic{}, TSTypeName: "MyBasicType"})
		assertType(t, results, "MyBasicType", `{
			IntField: number;
			StringField: string;
			BoolField: boolean;
			TimeField: string;
		}`)
	})

	t.Run("JSON Tag Handling", func(t *testing.T) {
		type WithTags struct {
			Renamed     string `json:"field_one"`
			Optional    int    `json:"fieldTwo,omitempty"`
			Ignored     bool   `json:"-"`
			Pointer     *bool  `json:"pointerField"`
			IgnoredToo  any    `json:"-,"`
			OmitZeroVal int    `json:"zeroValField,omitzero"`
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: WithTags{}})
		assertType(t, results, "WithTags", `{
			field_one: string;
			fieldTwo?: number;
			pointerField?: boolean;
			zeroValField?: number;
		}`)
	})

	t.Run("Collections and Pointers", func(t *testing.T) {
		type WithTags struct {
			Renamed string `json:"field_one"`
		}
		type Collections struct {
			IntSlice    []int
			StringArray [2]string
			StructPtr   *WithTags
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: Collections{}})
		assertType(t, results, "Collections", `{
			IntSlice: Array<number>;
			StringArray: Array<string>;
			StructPtr?: WithTags;
		}`)
		assertType(t, results, "WithTags", `{ field_one: string; }`)
	})

	t.Run("Map Handling", func(t *testing.T) {
		type WithMaps struct {
			StringKey map[string]int
			IntKey    map[int]string
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: WithMaps{}})
		assertType(t, results, "WithMaps", `{
			StringKey: Record<string, number>;
			IntKey: Record<number, string>;
		}`)
	})

	t.Run("Interface and Empty Struct Handling", func(t *testing.T) {
		type WithEmpty struct {
			AnyField  any
			EmptyData struct{}
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: WithEmpty{}})
		assertType(t, results, "WithEmpty", `{
			AnyField: unknown;
			EmptyData: Record<never, never>;
		}`)
	})

	t.Run("Name Collisions", func(t *testing.T) {
		type TypeA struct{ A int }
		type TypeB struct{ B string }
		results := processAdHoc(t,
			&AdHocType{TypeInstance: TypeA{}, TSTypeName: "Collision"},
			&AdHocType{TypeInstance: TypeB{}, TSTypeName: "Collision"},
		)
		assertType(t, results, "Collision", `{ A: number; }`)
		assertType(t, results, "Collision_2", `{ B: string; }`)
	})

	t.Run("Custom Type Overrides", func(t *testing.T) {
		results := processAdHoc(t, &AdHocType{TypeInstance: WithCustom{}})
		assertType(t, results, "WithCustom", `{
			A: OverriddenByMethod;
			B: MyCustomNumber;
		}`)
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
		results := processAdHoc(t,
			&AdHocType{TypeInstance: PagedResult[Product]{}, TSTypeName: "ProductPage"},
			&AdHocType{TypeInstance: PagedResult[User]{}, TSTypeName: "UserPage"},
		)
		assertType(t, results, "ProductPage", `{ items: Array<Product>; total: number; }`)
		assertType(t, results, "UserPage", `{ items: Array<User>; total: number; }`)
		assertType(t, results, "Product", `{ name: string; price: number; }`)
		assertType(t, results, "User", `{ email: string; }`)
		assertNotExported(t, results, "PagedResult")
	})
}

func TestEmbeddingScenarios(t *testing.T) {
	type Base struct {
		Name string `json:"name"`
	}
	type PtrBase struct {
		Age int `json:"age"`
	}

	t.Run("Flattening (No Tags)", func(t *testing.T) {
		type Host struct {
			ID string `json:"id"`
			Base
			*PtrBase
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: Host{}})
		assertType(t, results, "Host", `{ id: string; name: string; age?: number; }`)
		assertNotExported(t, results, "Base")
		assertNotExported(t, results, "PtrBase")
	})

	t.Run("Nesting (With Tags)", func(t *testing.T) {
		type Host struct {
			ID string   `json:"id"`
			B  Base     `json:"b"`
			PB *PtrBase `json:"pb"`
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: Host{}})
		assertType(t, results, "Host", `{ id: string; b: Base; pb?: PtrBase; }`)
		assertType(t, results, "Base", `{ name: string; }`)
		assertType(t, results, "PtrBase", `{ age: number; }`)
	})

	t.Run("Mixed Flattening and Nesting", func(t *testing.T) {
		type Host struct {
			ID   string   `json:"id"`
			Base          // Flattened
			PB   *PtrBase `json:"pb"` // Nested
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: Host{}})
		assertType(t, results, "Host", `{ id: string; name: string; pb?: PtrBase; }`)
		assertNotExported(t, results, "Base")
		assertType(t, results, "PtrBase", `{ age: number; }`)
	})

	t.Run("Deeply Nested Flattening with Overrides", func(t *testing.T) {
		results := processAdHoc(t, &AdHocType{TypeInstance: TopLevel{}})

		// This assertion is now correct. The implementation generates fields in a deterministic
		// source-code order (depth-first for embedded fields). The test now expects this correct order.
		assertType(t, results, "TopLevel", `{
			field_c: overridden_c;
			field_b: overridden_b;
			field_a: string;
		}`)

		assertNotExported(t, results, "MiddleLevel")
		assertNotExported(t, results, "BottomLevel")
	})
}

func TestAdvancedScenarios(t *testing.T) {
	t.Run("Early Filtering of json:-", func(t *testing.T) {
		type Component struct {
			Value string `json:"value"`
		}
		type FieldWithEmbeddedComponent struct {
			Component
		}
		type Host struct {
			Ignored FieldWithEmbeddedComponent `json:"-"`
			Component
		}
		results := processAdHoc(t, &AdHocType{TypeInstance: Host{}})
		assertType(t, results, "Host", `{ value: string; }`)
		assertNotExported(t, results, "FieldWithEmbeddedComponent")
		assertNotExported(t, results, "Component")
	})

	t.Run("TSTyper Reference and Embedding Coexistence", func(t *testing.T) {
		results := processAdHoc(t,
			&AdHocType{TypeInstance: EmbeddingHost{}},
			&AdHocType{TypeInstance: TSTyperHost{}},
		)
		assertType(t, results, "EmbeddingHost", `{ id: number; name: string; }`)
		assertType(t, results, "SharedComponent", `{ name: string; }`)
		assertType(t, results, "TSTyperHost", `{ ReferenceField: SharedComponent; }`)
	})

	t.Run("Referenced and Embedded Coexistence", func(t *testing.T) {
		type Base struct {
			Name string `json:"name"`
		}
		type Host struct {
			ID string `json:"id"`
			Base
		}
		type Referencer struct {
			B Base `json:"b"`
		}
		results := processAdHoc(t,
			&AdHocType{TypeInstance: Host{}},
			&AdHocType{TypeInstance: Referencer{}},
		)
		assertType(t, results, "Host", `{ id: string; name: string; }`)
		assertType(t, results, "Referencer", `{ b: Base; }`)
		assertType(t, results, "Base", `{ name: string; }`)
	})
}
