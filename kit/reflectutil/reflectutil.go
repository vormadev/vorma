package reflectutil

import (
	"reflect"
	"strings"

	"github.com/vormadev/vorma/kit/genericsutil"
)

func ImplementsInterface(t reflect.Type, iface reflect.Type) bool {
	if t == nil {
		return false
	}
	if iface == nil {
		return false
	}
	if iface.Kind() != reflect.Interface {
		panic("reflectutil error: expected interface type")
	}
	if t.Implements(iface) {
		return true
	}
	if t.Kind() != reflect.Ptr {
		if reflect.PointerTo(t).Implements(iface) {
			return true
		}
	}
	return false
}

func ToInterfaceReflectType[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func ExcludingNoneGetIsNilOrUltimatelyPointsToNil(v any) bool {
	return excludingNoneGetIsNilOrUltimatelyPointsToNil_inner(v, false)
}

func excludingNoneGetIsNilOrUltimatelyPointsToNil_inner(v any, skipIsNoneCheck bool) bool {
	if !skipIsNoneCheck && genericsutil.IsNone(v) {
		return false
	}

	if v == nil {
		return true
	}

	reflectVal := reflect.ValueOf(v)

	switch reflectVal.Kind() {
	case reflect.Ptr, reflect.Interface:
		if reflectVal.IsNil() {
			return true
		}
		return excludingNoneGetIsNilOrUltimatelyPointsToNil_inner(reflectVal.Elem().Interface(), true)

	case reflect.Map, reflect.Slice:
		return reflectVal.IsNil()

	default:
		return false
	}
}

func GetJSONFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return ""
	}
	if parts[0] != "" {
		return parts[0]
	}
	return field.Name
}
