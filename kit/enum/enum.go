package enum

import "reflect"

type AnyEnum interface {
	Struct() any
	is_vorma_kit_enum()
}

type Value interface {
	~string | ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// S must be a struct type with fields of type V.
// The field names will be used as the enum keys,
// and the field values will be used as the enum values.
type Enum[V Value, S any] struct {
	_enum S
	_vals []V
}

func (e Enum[V, S]) is_vorma_kit_enum() {}
func (e Enum[V, S]) Struct() any        { return e._enum }

// S must be a struct type with fields of type V.
// The field names will be used as the enum keys,
// and the field values will be used as the enum values.
func New[V Value, S any](enum S) Enum[V, S] {
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

	return Enum[V, S]{
		_enum: enum,
		_vals: values,
	}
}

func (e Enum[V, S]) Get() S      { return e._enum }
func (e Enum[V, S]) Values() []V { return append([]V(nil), e._vals...) }
