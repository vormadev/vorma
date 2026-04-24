// Package searchparams decodes URL search parameters into Go structs and
// derives a type-level schema describing the accepted shape.
package searchparams

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/vormadev/vorma/kit/reflectutil"
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC API
/////////////////////////////////////////////////////////////////////

var (
	// Parse/ParseInto errors.
	ParseError              = errors.New("error parsing URL search parameters")
	ParseNilRequestError    = errors.New("parse: request is nil")
	ParseNilURLError        = errors.New("parse: request URL is nil")
	ParseNilDestError       = errors.New("parse: destination must be a non-nil pointer")
	ParseNonStructDestError = errors.New("parse: destination must point to a struct")

	// SchemaFromValue errors.
	SchemaError              = errors.New("invalid URL search params schema")
	SchemaNilValueError      = errors.New("schema: value cannot be nil")
	SchemaNonStructRootError = errors.New("schema: root must be a struct")
)

// ParseToStruct parses URL search parameters from an HTTP request into a struct of type T.
func ParseToStruct[T any](r *http.Request) (T, error) {
	var val T
	if err := ParseIntoStructPtr(r, &val); err != nil {
		return val, err
	}
	return val, nil
}

// ParseIntoStructPtr parses URL search parameters from an HTTP request into the struct
// pointed to by dest. Used when the destination type is only known at runtime
// (e.g. via reflection-based type erasure). Callers with a static type should
// prefer ParseToStruct.
func ParseIntoStructPtr(r *http.Request, destStructPtr any) error {
	if r == nil {
		return ParseNilRequestError
	}
	if r.URL == nil {
		return ParseNilURLError
	}
	dv := reflect.ValueOf(destStructPtr)
	if !dv.IsValid() || dv.Kind() != reflect.Pointer || dv.IsNil() {
		return ParseNilDestError
	}
	elem := dv.Elem()
	if elem.Kind() != reflect.Struct {
		return ParseNonStructDestError
	}
	if err := set_nested_field(elem, r.URL.Query()); err != nil {
		return errors.Join(ParseError, err)
	}
	return nil
}

// Schema describes the accepted shape of a decoded struct. Node types are:
//   - string codes ("s", "n", "b") for scalars, optionally prefixed with "?"
//     to mark pointer scalars.
//   - []Schema{elem} for slices and arrays.
//   - []Schema{"*", val} for maps with string keys.
//   - map[string]Schema for structs.
type Schema any

// SchemaFromValue derives a Schema from the type of value. The value's runtime
// contents are ignored; only its type is inspected. The root must be a struct
// (or a pointer to one).
func SchemaFromValue(value any) (Schema, error) {
	t, _ := reflectutil.DerefType(reflect.TypeOf(value))
	if t == nil {
		return nil, SchemaNilValueError
	}
	if t.Kind() != reflect.Struct {
		return nil, errors.Join(
			SchemaNonStructRootError,
			fmt.Errorf("got %s", t),
		)
	}
	schema, err := schema_from_type(t)
	if err != nil {
		return nil, errors.Join(SchemaError, err)
	}
	return schema, nil
}

/////////////////////////////////////////////////////////////////////
/////// SETTERS
/////////////////////////////////////////////////////////////////////

func set_nested_field(v reflect.Value, values map[string][]string) error {
	fields, err := reflectutil.PublicStructFields(v.Type())
	if err != nil {
		return err
	}
	for _, field_shape := range fields {
		field := field_shape.Field
		fv, ok := field_shape.SettableCompositeValue(v)
		if !ok {
			continue
		}

		public_name := field_shape.PublicName

		if fv.Kind() == reflect.Struct {
			if err := set_nested_field(
				fv, values_with_prefix(values, public_name+"."),
			); err != nil {
				return err
			}
			continue
		}

		if fv.Kind() == reflect.Map {
			if err := set_map_field(
				fv, values_with_prefix(values, public_name+"."),
			); err != nil {
				return err
			}
			continue
		}

		if fv.Kind() == reflect.Slice {
			vals, ok := values[public_name]
			if ok {
				filtered := make([]string, 0, len(vals))
				for _, val := range vals {
					if val != "" {
						filtered = append(filtered, val)
					}
				}
				vals = filtered
			}
			if !ok {
				prefix := public_name + "."
				for k, v := range values {
					if strings.HasPrefix(k, prefix) {
						for _, s := range v {
							if s != "" {
								vals = append(vals, s)
							}
						}
					}
				}
			}
			if len(vals) == 0 {
				reflectutil.Value{V: fv}.SetEmptySlice()
			} else if err := set_slice_field(fv, vals); err != nil {
				return err
			}
			continue
		}

		if val, ok := values[public_name]; ok {
			if err := set_field(fv, val); err != nil {
				return fmt.Errorf("error setting field %s: %w", field.Name, err)
			}
		}
	}
	return nil
}

func set_map_field(v reflect.Value, values map[string][]string) error {
	reflectutil.Value{V: v}.EnsureMap()
	for key, val := range values {
		kv := reflect.ValueOf(key)
		ev := reflectutil.Value{V: v}.NewElemValue()
		if ev.Kind() == reflect.Map {
			if err := set_map_field(ev, values_with_prefix(values, key+".")); err != nil {
				return err
			}
		} else {
			if err := set_field(ev, val); err != nil {
				return fmt.Errorf("error setting map value for key %s: %w", key, err)
			}
		}
		reflectutil.Value{V: v}.SetMapValue(kv, ev)
	}
	return nil
}

func set_slice_field(field reflect.Value, values []string) error {
	slice := reflectutil.Value{V: field}.MakeSlice(len(values))
	for i, val := range values {
		elem := slice.Index(i)
		if elem.Kind() == reflect.Pointer {
			var ok bool
			elem, ok = reflectutil.Value{V: elem}.EnsurePointerElem()
			if !ok {
				return fmt.Errorf("field is not settable")
			}
		}
		if err := set_single_value(elem, val); err != nil {
			return err
		}
	}
	field.Set(slice)
	return nil
}

func set_single_value(field reflect.Value, value string) error {
	return reflectutil.Value{V: field}.SetScalarFromString(value)
}

func set_field(field reflect.Value, values []string) error {
	if len(values) == 0 {
		return nil
	}
	switch field.Kind() {
	case reflect.Pointer:
		if values[0] == "" {
			reflectutil.Value{V: field}.SetZero()
			return nil
		}
		elem, ok := reflectutil.Value{V: field}.EnsurePointerElem()
		if !ok {
			return fmt.Errorf("field is not settable")
		}
		return set_single_value(elem, values[0])
	case reflect.Slice:
		return set_slice_field(field, values)
	case reflect.Map:
		return set_map_field(field, map[string][]string{"": values})
	default:
		if reflectutil.IsScalarType(field.Type()) {
			return set_single_value(field, values[0])
		}
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
}

/////////////////////////////////////////////////////////////////////
/////// SCHEMA
/////////////////////////////////////////////////////////////////////

const (
	schema_code_bool   = "b"
	schema_code_map    = "*"
	schema_code_number = "n"
	schema_code_string = "s"
)

func schema_from_type(t reflect.Type) (Schema, error) {
	base, is_pointer := reflectutil.DerefType(t)

	switch {
	case base.Kind() == reflect.Bool:
		return scalar_schema(schema_code_bool, is_pointer), nil
	case reflectutil.IsNumericKind(base.Kind()):
		return scalar_schema(schema_code_number, is_pointer), nil
	case base.Kind() == reflect.String:
		return scalar_schema(schema_code_string, is_pointer), nil
	case base.Kind() == reflect.Slice || base.Kind() == reflect.Array:
		elem_base, _ := reflectutil.DerefType(base.Elem())
		if !reflectutil.IsScalarType(elem_base) {
			return nil, fmt.Errorf("slices must contain scalar values: %s", t)
		}
		elem_schema, err := schema_from_type(base.Elem())
		if err != nil {
			return nil, err
		}
		return []Schema{elem_schema}, nil
	case base.Kind() == reflect.Map:
		if base.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map keys must be strings: %s", t)
		}
		val_base, _ := reflectutil.DerefType(base.Elem())
		switch val_base.Kind() {
		case reflect.Slice, reflect.Array:
			elem_base, _ := reflectutil.DerefType(val_base.Elem())
			if !reflectutil.IsScalarType(elem_base) {
				return nil, fmt.Errorf("map slices must contain scalar values: %s", t)
			}
		default:
			if !reflectutil.IsScalarType(val_base) {
				return nil, fmt.Errorf("maps must contain scalar or scalar slice values: %s", t)
			}
		}
		val_schema, err := schema_from_type(base.Elem())
		if err != nil {
			return nil, err
		}
		return []Schema{schema_code_map, val_schema}, nil
	case base.Kind() == reflect.Struct:
		return struct_schema(base)
	default:
		return nil, fmt.Errorf("unsupported type: %s", t)
	}
}

func struct_schema(t reflect.Type) (Schema, error) {
	out := make(map[string]Schema)
	fields, err := reflectutil.PublicStructFields(t)
	if err != nil {
		return nil, err
	}
	for _, field_shape := range fields {
		schema, err := schema_from_type(field_shape.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field_shape.FieldName, err)
		}
		out[field_shape.PublicName] = schema
	}
	return out, nil
}

func scalar_schema(code string, is_pointer bool) Schema {
	if is_pointer {
		return "?" + code
	}
	return code
}

/////////////////////////////////////////////////////////////////////
/////// UTILS
/////////////////////////////////////////////////////////////////////

func values_with_prefix(values map[string][]string, prefix string) map[string][]string {
	out := make(map[string][]string)
	for k, v := range values {
		if after, ok := strings.CutPrefix(k, prefix); ok {
			out[after] = v
		}
	}
	return out
}
