package searchparams

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_schema_root_spec struct {
	fields []property_schema_field_spec
}

type property_schema_field_spec struct {
	kind         int
	name_mode    int
	nested_kinds []int
}

type property_schema_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestSchemaFromValueGeneratedShapesMatchModel(t *testing.T) {
	t.Run("generated_type_shape_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_schema_case_space{}).draw_case(ht)
		tc.assert_matches_model(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_schema_case_space) draw_case(ht *hegel.T) property_schema_root_spec {
	field_count := hegel.Draw(ht, hegel.Integers(0, 4))
	fields := make([]property_schema_field_spec, 0, field_count)
	for range field_count {
		spec := property_schema_field_spec{
			kind:      hegel.Draw(ht, hegel.Integers(0, 13)),
			name_mode: hegel.Draw(ht, hegel.Integers(0, 4)),
		}
		if spec.kind == 9 || spec.kind == 10 || spec.kind == 11 {
			nested_count := hegel.Draw(ht, hegel.Integers(1, 3))
			spec.nested_kinds = make([]int, 0, nested_count)
			for range nested_count {
				spec.nested_kinds = append(spec.nested_kinds, hegel.Draw(ht, hegel.Integers(0, 7)))
			}
		}
		fields = append(fields, spec)
	}
	return property_schema_root_spec{fields: fields}
}

func (tc property_schema_root_spec) assert_matches_model(ht *hegel.T) {
	tc.note(ht)

	root_type, expected, supported := tc.root_type_and_expected_schema()
	value := reflect.New(root_type).Elem().Interface()

	actual, err := SchemaFromValue(value)
	if supported {
		if err != nil {
			ht.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(actual, expected) {
			ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
		}
		return
	}

	if err == nil {
		ht.Fatalf("expected error, got schema %#v", actual)
	}
	if !errors.Is(err, ErrInvalidSchema) {
		ht.Fatalf("expected SchemaError, got %v", err)
	}
}

func (tc property_schema_root_spec) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))
}

func (tc property_schema_root_spec) root_type_and_expected_schema() (
	reflect.Type,
	Schema,
	bool,
) {
	struct_fields := make([]reflect.StructField, 0, len(tc.fields))
	expected := make(map[string]Schema)
	supported := true

	for index, field_spec := range tc.fields {
		field, field_schema, field_supported, include := field_spec.struct_field(index)
		struct_fields = append(struct_fields, field)
		if include && !field_supported {
			supported = false
		}
		if include {
			expected[field_name(index, field_spec.name_mode)] = field_schema
		}
	}

	return reflect.StructOf(struct_fields), expected, supported
}

func (spec property_schema_field_spec) struct_field(
	index int,
) (reflect.StructField, Schema, bool, bool) {
	field_type, field_schema, supported := spec.field_type_and_schema(index)
	field := reflect.StructField{
		Name: fmt.Sprintf("Field%d", index),
		Type: field_type,
		Tag:  reflect.StructTag(spec.json_tag(index)),
	}
	include := spec.name_mode != 4
	return field, field_schema, supported, include
}

func (spec property_schema_field_spec) field_type_and_schema(
	index int,
) (reflect.Type, Schema, bool) {
	switch spec.kind {
	case 0:
		return reflect.TypeFor[string](), schema_code_string, true
	case 1:
		return reflect.TypeFor[*string](), "?" + schema_code_string, true
	case 2:
		return reflect.TypeFor[int](), schema_code_number, true
	case 3:
		return reflect.TypeFor[**int](), "?" + schema_code_number, true
	case 4:
		return reflect.TypeFor[bool](), schema_code_bool, true
	case 5:
		return reflect.TypeFor[[]string](), []Schema{schema_code_string}, true
	case 6:
		return reflect.TypeFor[[]*int](), []Schema{"?" + schema_code_number}, true
	case 7:
		return reflect.TypeFor[map[string]string](), []Schema{
			schema_code_map,
			schema_code_string,
		}, true
	case 8:
		return reflect.TypeFor[map[string][]bool](), []Schema{
			schema_code_map,
			[]Schema{schema_code_bool},
		}, true
	case 9:
		nested_type, nested_schema, nested_supported := spec.nested_struct_type_and_schema(index)
		return nested_type, nested_schema, nested_supported
	case 10:
		nested_type, nested_schema, nested_supported := spec.nested_struct_type_and_schema(index)
		return reflect.PointerTo(nested_type), nested_schema, nested_supported
	case 11:
		nested_type, _, _ := spec.nested_struct_type_and_schema(index)
		return reflect.SliceOf(nested_type), nil, false
	case 12:
		return reflect.TypeFor[map[string]map[string]string](), nil, false
	case 13:
		return reflect.TypeFor[chan int](), nil, false
	default:
		return reflect.TypeFor[any](), nil, false
	}
}

func (spec property_schema_field_spec) nested_struct_type_and_schema(
	index int,
) (reflect.Type, Schema, bool) {
	nested_fields := make([]reflect.StructField, 0, len(spec.nested_kinds))
	expected := make(map[string]Schema)
	supported := true

	for nested_index, nested_kind := range spec.nested_kinds {
		nested_spec := property_schema_field_spec{
			kind:      nested_kind % 9,
			name_mode: nested_index % 4,
		}
		field, field_schema, field_supported, include := nested_spec.struct_field(
			index*10 + nested_index + 1,
		)
		nested_fields = append(nested_fields, field)
		if !field_supported {
			supported = false
		}
		if include {
			expected[field_name(index*10+nested_index+1, nested_spec.name_mode)] = field_schema
		}
	}

	return reflect.StructOf(nested_fields), expected, supported
}

func (spec property_schema_field_spec) json_tag(index int) string {
	switch spec.name_mode {
	case 1:
		return fmt.Sprintf(`json:"named_%d"`, index)
	case 2:
		return `json:",omitempty"`
	case 3:
		return fmt.Sprintf(`json:"named_%d,omitempty"`, index)
	case 4:
		return `json:"-"`
	default:
		return ""
	}
}

func field_name(index int, name_mode int) string {
	switch name_mode {
	case 1, 3:
		return fmt.Sprintf("named_%d", index)
	default:
		return fmt.Sprintf("Field%d", index)
	}
}
