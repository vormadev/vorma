package schema_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type slice_holder struct {
	T []string
}

type int_slice_holder struct {
	T []int
}

type nested_slice_item struct {
	ID int
}

func (nested_slice_item) Schema() schema.Schema {
	return schema.Object{
		"ID": schema.Int{Min: 1},
	}
}

type slice_of_schematic_holder struct {
	Items []*nested_slice_item
}

type slice_of_values_holder struct {
	Items []nested_slice_item
}

type not_slice_holder struct {
	V string
}

type array_holder struct {
	A [3]nested_slice_item
}

type int_array_holder struct {
	A [3]int
}

type Tags []string

func (Tags) Schema() schema.Schema {
	return schema.List{
		MaxLen: 10,
		ElementSchema: schema.String{
			TrimSpace: true,
			ToLower:   true,
			MaxLen:    50,
		},
	}
}

type container_map_holder struct {
	M map[string]string
}

type string_keyed_map map[string]string

type non_string_key_map map[int]string

type not_map_holder struct {
	V string
}

/////////////////////////////////////////////////////////////////////
/////// SLICE CASES
/////////////////////////////////////////////////////////////////////

func TestSlice_Required_Nil_Fails(t *testing.T) {
	_, err := schema.Enforce("s", slice_holder{T: nil}, schema.Object{
		"T": schema.List{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSlice_Required_Empty_Passes(t *testing.T) {
	_, err := schema.Enforce("s", slice_holder{T: []string{}}, schema.Object{
		"T": schema.List{MustNotBeNil: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSlice_ElementSchema_AppliedToEveryElement(t *testing.T) {
	_, err := schema.Enforce("s", int_slice_holder{T: []int{5, 20, 8, 99}}, schema.Object{
		"T": schema.List{
			ElementSchema: schema.Int{Min: 1, Max: 10},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "[1]") || !strings.Contains(msg, "[3]") {
		t.Fatalf("expected failing element labels, got %q", msg)
	}
}

func TestSlice_ElementSchema_NormalizationReflectedInOutput(t *testing.T) {
	out, err := schema.Enforce("s", slice_holder{T: []string{"  A  ", "BB"}}, schema.Object{
		"T": schema.List{
			ElementSchema: schema.String{TrimSpace: true, ToLower: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(out.Value.T, []string{"a", "bb"}) {
		t.Fatalf("got %#v", out.Value.T)
	}
}

func TestSlice_MinLen(t *testing.T) {
	_, err := schema.Enforce("s", slice_holder{T: []string{"one"}}, schema.Object{
		"T": schema.List{MinLen: 2, MaxLen: 4},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSlice_MaxLen(t *testing.T) {
	_, err := schema.Enforce("s", slice_holder{
		T: []string{"a", "b", "c", "d", "e"},
	}, schema.Object{
		"T": schema.List{MinLen: 2, MaxLen: 4},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSlice_NegativeLengthLimit_IsSchemaError(t *testing.T) {
	_, err := schema.Enforce("s", slice_holder{T: []string{"a"}}, schema.Object{
		"T": schema.List{MinLen: -1},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

func TestSlice_ValidateFunc_ReceivesLength(t *testing.T) {
	_, err := schema.Enforce("s", slice_holder{T: []string{"a", "b", "c"}}, schema.Object{
		"T": schema.List{
			ValidateFunc: func(n int) error {
				if n%2 != 0 {
					return errors.New("length must be even")
				}
				return nil
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	_, err = schema.Enforce("s", slice_holder{T: []string{"a", "b"}}, schema.Object{
		"T": schema.List{
			ValidateFunc: func(n int) error {
				if n%2 != 0 {
					return errors.New("length must be even")
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSlice_Array_ElementSchemaWorksOnFixedArray(t *testing.T) {
	_, err := schema.Enforce("s", int_array_holder{A: [3]int{5, 20, 8}}, schema.Object{
		"A": schema.List{
			ElementSchema: schema.Int{Min: 1, Max: 10},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSlice_NilElementSkipped_Schematic(t *testing.T) {
	_, err := schema.Enforce("s", slice_of_schematic_holder{
		Items: []*nested_slice_item{
			nil,
			{ID: 5},
		},
	}, schema.Object{
		"Items": schema.List{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSlice_InvalidElementFails_Schematic(t *testing.T) {
	_, err := schema.Enforce("s", slice_of_schematic_holder{
		Items: []*nested_slice_item{
			{ID: -1},
		},
	}, schema.Object{
		"Items": schema.List{},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSlice_NestedInStruct(t *testing.T) {
	_, err := schema.Enforce("s", slice_of_values_holder{
		Items: []nested_slice_item{{ID: 1}, {ID: -1}, {ID: -2}},
	}, schema.Object{
		"Items": schema.List{},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "[1]") || !strings.Contains(msg, "[2]") {
		t.Fatalf("expected failing element labels, got %q", msg)
	}
}

func TestSlice_AppliedToNonSlice_SchemaError(t *testing.T) {
	_, err := schema.Enforce("s", not_slice_holder{V: "nope"}, schema.Object{
		"V": schema.List{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

func TestSlice_Newtype_ValueMode(t *testing.T) {
	out, err := schema.Enforce("t", Tags{"  A  ", "BB", "  cC  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual([]string(out.Value), []string{"a", "bb", "cc"}) {
		t.Fatalf("got %#v", out.Value)
	}
}

func TestSlice_ArrayOfSchematics_Recurses(t *testing.T) {
	_, err := schema.Enforce("s", array_holder{
		A: [3]nested_slice_item{{ID: 1}, {ID: -1}, {ID: 2}},
	}, schema.Object{
		"A": schema.List{},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "[1]") {
		t.Fatalf("expected failing array index in %q", err.Error())
	}
}

/////////////////////////////////////////////////////////////////////
/////// MAP CASES
/////////////////////////////////////////////////////////////////////

func TestMap_Required_Nil_Fails(t *testing.T) {
	_, err := schema.Enforce("s", container_map_holder{M: nil}, schema.Object{
		"M": schema.Map{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMap_NotRequired_Nil_Passes(t *testing.T) {
	_, err := schema.Enforce("s", container_map_holder{M: nil}, schema.Object{
		"M": schema.Map{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMap_Empty_Passes(t *testing.T) {
	_, err := schema.Enforce("s", container_map_holder{M: map[string]string{}}, schema.Object{
		"M": schema.Map{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMap_KeySchema_Validation(t *testing.T) {
	_, err := schema.Enforce("s", container_map_holder{M: map[string]string{
		"good":    "x",
		"bad key": "y",
	}}, schema.Object{
		"M": schema.Map{
			KeySchema: schema.String{
				ValidateFunc: func(v string) error {
					if strings.Contains(v, " ") {
						return errors.New("key cannot contain spaces")
					}
					return nil
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "key cannot contain spaces") {
		t.Fatalf("got %q", err.Error())
	}
}

func TestMap_ValueSchema_Validation(t *testing.T) {
	_, err := schema.Enforce("s", container_map_holder{M: map[string]string{
		"k1": "x",
	}}, schema.Object{
		"M": schema.Map{
			ValueSchema: schema.String{MustNotBeZero: true, MinLen: 2},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMap_KeySchemaAndValueSchema_BothFail(t *testing.T) {
	_, err := schema.Enforce("s", container_map_holder{M: map[string]string{
		"bad key": "x",
	}}, schema.Object{
		"M": schema.Map{
			KeySchema: schema.String{
				ValidateFunc: func(v string) error {
					if strings.Contains(v, " ") {
						return errors.New("key cannot contain spaces")
					}
					return nil
				},
			},
			ValueSchema: schema.String{MustNotBeZero: true, MinLen: 2},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "key cannot contain spaces") {
		t.Fatalf("missing key error: %q", msg)
	}
	if !strings.Contains(msg, "minimum length") {
		t.Fatalf("missing value error: %q", msg)
	}
}

func TestMap_MinLen(t *testing.T) {
	_, err := schema.Enforce(
		"s",
		container_map_holder{M: map[string]string{"a": "1"}},
		schema.Object{
			"M": schema.Map{MinLen: 2, MaxLen: 3},
		},
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMap_MaxLen(t *testing.T) {
	_, err := schema.Enforce("s", container_map_holder{M: map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
	}}, schema.Object{
		"M": schema.Map{MinLen: 2, MaxLen: 3},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMap_ValueNormalization_WrittenBack(t *testing.T) {
	value := &container_map_holder{M: map[string]string{
		"primary":   "  HELLO  ",
		"secondary": "  WORLD  ",
	}}

	_, err := schema.Enforce("s", value, schema.Object{
		"M": schema.Map{
			ValueSchema: schema.String{TrimSpace: true, ToLower: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value.M["primary"] != "hello" || value.M["secondary"] != "world" {
		t.Fatalf("values not normalized: %#v", value.M)
	}
}

func TestMap_KeyNormalization_WrittenBack(t *testing.T) {
	value := &container_map_holder{M: map[string]string{
		"  PRIMARY  ": "x",
	}}

	_, err := schema.Enforce("s", value, schema.Object{
		"M": schema.Map{
			KeySchema: schema.String{TrimSpace: true, ToLower: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := value.M["primary"]; !ok {
		t.Fatalf("key not normalized: %#v", value.M)
	}
}

func TestMap_KeyNormalization_Collision_LastWins(t *testing.T) {
	value := &container_map_holder{M: map[string]string{
		"FOO": "first",
		"foo": "second",
	}}

	_, err := schema.Enforce("s", value, schema.Object{
		"M": schema.Map{
			KeySchema: schema.String{TrimSpace: true, ToLower: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(value.M) != 1 {
		t.Fatalf("expected 1 entry after collision, got %d: %#v", len(value.M), value.M)
	}
	if _, ok := value.M["foo"]; !ok {
		t.Fatalf("expected normalized key to survive: %#v", value.M)
	}
}

func TestMap_FieldsMode_StringKeyedMapAsTarget(t *testing.T) {
	_, err := schema.Enforce("s", string_keyed_map{
		"required_key": "x",
	}, schema.Object{
		"required_key": schema.String{MustNotBeZero: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMap_FieldsMode_MissingKey_Required_Fails(t *testing.T) {
	_, err := schema.Enforce("s", string_keyed_map{}, schema.Object{
		"required_key": schema.String{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMap_FieldsMode_MissingKey_NoOp_DoesNotMaterializeZeroValue(t *testing.T) {
	value := string_keyed_map{}

	out, err := schema.Enforce("s", value, schema.Object{
		"optional_key": schema.String{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out.Value["optional_key"]; ok {
		t.Fatalf("unexpected key materialization: %#v", out.Value)
	}
}

func TestMap_FieldsMode_NonStringKey_SchemaError(t *testing.T) {
	_, err := schema.Enforce("s", non_string_key_map{1: "x"}, schema.Object{
		"anything": schema.String{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

func TestMap_AppliedToNonMap_SchemaError(t *testing.T) {
	_, err := schema.Enforce("s", not_map_holder{V: "nope"}, schema.Object{
		"V": schema.Map{MustNotBeNil: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}
