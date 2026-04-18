package validate

import (
	"fmt"
	"maps"
	"reflect"
)

const (
	url_search_params_schema_bool   = "b"
	url_search_params_schema_map    = "*"
	url_search_params_schema_number = "n"
	url_search_params_schema_string = "s"
)

type URLSearchParamsSchema any

type URLSearchParamsSchemaBuilder struct{}

func (b URLSearchParamsSchemaBuilder) FromValue(value any) (URLSearchParamsSchema, error) {
	t := reflect.TypeOf(value)
	if t == nil {
		return nil, fmt.Errorf("URL search params schema value cannot be nil")
	}
	t, _ = url_search_params.base_type(t)
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf(
			"URL search params schema root must be a struct: %s",
			t,
		)
	}
	schema, err := b.schema_from_type(t)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

func (b URLSearchParamsSchemaBuilder) schema_from_type(
	t reflect.Type,
) (URLSearchParamsSchema, error) {
	base_type, is_pointer := url_search_params.base_type(t)

	switch base_type.Kind() {
	case reflect.Bool:
		return b.scalar_schema(url_search_params_schema_bool, is_pointer), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Float32, reflect.Float64:
		return b.scalar_schema(url_search_params_schema_number, is_pointer), nil
	case reflect.String:
		return b.scalar_schema(url_search_params_schema_string, is_pointer), nil
	case reflect.Slice, reflect.Array:
		elem_type, _ := url_search_params.base_type(base_type.Elem())
		if !url_search_params.is_scalar_type(elem_type) {
			return nil, fmt.Errorf(
				"URL search params schema slices must contain scalar values: %s",
				t,
			)
		}
		elem_schema, err := b.schema_from_type(base_type.Elem())
		if err != nil {
			return nil, err
		}
		return []URLSearchParamsSchema{elem_schema}, nil
	case reflect.Map:
		if base_type.Key().Kind() != reflect.String {
			return nil, fmt.Errorf(
				"URL search params schema map keys must be strings: %s",
				t,
			)
		}
		val_type, _ := url_search_params.base_type(base_type.Elem())
		if val_type.Kind() == reflect.Slice || val_type.Kind() == reflect.Array {
			elem_type, _ := url_search_params.base_type(val_type.Elem())
			if !url_search_params.is_scalar_type(elem_type) {
				return nil, fmt.Errorf(
					"URL search params schema map slices must contain scalar values: %s",
					t,
				)
			}
		} else if !url_search_params.is_scalar_type(val_type) {
			return nil, fmt.Errorf(
				"URL search params schema maps must contain scalar or scalar slice values: %s",
				t,
			)
		}
		val_schema, err := b.schema_from_type(base_type.Elem())
		if err != nil {
			return nil, err
		}
		return []URLSearchParamsSchema{
			url_search_params_schema_map,
			val_schema,
		}, nil
	case reflect.Struct:
		return b.struct_schema(base_type)
	default:
		return nil, fmt.Errorf(
			"unsupported URL search params schema type: %s",
			t,
		)
	}
}

func (URLSearchParamsSchemaBuilder) scalar_schema(
	code string,
	is_pointer bool,
) URLSearchParamsSchema {
	if is_pointer {
		return "?" + code
	}
	return code
}

func (b URLSearchParamsSchemaBuilder) struct_schema(
	t reflect.Type,
) (URLSearchParamsSchema, error) {
	out := make(map[string]URLSearchParamsSchema)
	for field := range t.Fields() {
		field := field
		if url_search_params.should_skip_field(field) {
			continue
		}

		if field.Anonymous {
			embedded_type, _ := url_search_params.base_type(field.Type)
			if embedded_type.Kind() != reflect.Struct {
				return nil, fmt.Errorf(
					"anonymous URL search params schema field must be a struct: %s",
					field.Type,
				)
			}
			embedded_schema, err := b.struct_schema(embedded_type)
			if err != nil {
				return nil, err
			}
			maps.Copy(out, embedded_schema.(map[string]URLSearchParamsSchema))
			continue
		}

		name := url_search_params.field_name(field)
		schema, err := b.schema_from_type(field.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}
		out[name] = schema
	}
	return out, nil
}
