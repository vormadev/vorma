package enum

import "reflect"

type AnyEnum interface {
	Struct() any
	is_vorma_kit_enum()
}

type Value interface {
	~string | ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type Native interface {
	string | int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64
}

// S must be a struct type with fields of type V.
// The field names will be used as the enum keys,
// and the field values will be used as the enum values.
type Enum[V Value, N Native, S any] struct {
	_enum        S
	_vals        []V
	_native_vals []N
}

func (e Enum[V, N, S]) is_vorma_kit_enum() {}
func (e Enum[V, N, S]) Struct() any        { return e._enum }

// S must be a struct type with fields of type V.
// The field names will be used as the enum keys,
// and the field values will be used as the enum values.
func New[V Value, N Native, S any](enum S) Enum[V, N, S] {
	enum_value := reflect.ValueOf(enum)
	if !enum_value.IsValid() {
		panic("enum.New: enum must be a struct or pointer to struct")
	}

	base := enum_value
	if base.Kind() == reflect.Pointer {
		if base.IsNil() {
			panic("enum.New: enum pointer must not be nil")
		}
		base = base.Elem()
	}

	if base.Kind() != reflect.Struct {
		panic("enum.New: enum must be a struct or pointer to struct")
	}

	value_type := reflect.TypeFor[V]()
	values := make([]V, 0, base.NumField())

	for i := 0; i < base.NumField(); i++ {
		field_type := base.Type().Field(i)
		if !field_type.IsExported() {
			continue
		}

		field_value := base.Field(i)
		for field_value.Kind() == reflect.Pointer {
			if field_value.IsNil() {
				panic("enum.New: enum field " + field_type.Name + " must not be nil")
			}
			field_value = field_value.Elem()
		}

		if !field_value.Type().AssignableTo(value_type) {
			panic(
				"enum.New: enum field " + field_type.Name +
					" does not match enum value type",
			)
		}

		values = append(values, field_value.Interface().(V))
	}

	if len(values) == 0 {
		panic("enum.New: enum must not be empty")
	}

	native_type := reflect.TypeFor[N]()
	_native_values := make([]N, len(values))
	for i, v := range values {
		_native_values[i] = reflect.ValueOf(v).Convert(native_type).Interface().(N)
	}

	return Enum[V, N, S]{
		_enum:        enum,
		_vals:        values,
		_native_vals: _native_values,
	}
}

func (e Enum[V, N, S]) Get() S            { return e._enum }
func (e Enum[V, N, S]) Values() []V       { return append([]V(nil), e._vals...) }
func (e Enum[V, N, S]) NativeValues() []N { return append([]N(nil), e._native_vals...) }

func (e Enum[V, N, S]) Parse(value N) (V, bool) {
	for i, native_value := range e._native_vals {
		if native_value == value {
			return e._vals[i], true
		}
	}

	var zero V
	return zero, false
}
