package reflectutil

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vormadev/vorma/kit/genericsutil"
)

func DoesTypeImplementInterface(t reflect.Type, iface reflect.Type) bool {
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
	if t.Kind() != reflect.Pointer {
		if reflect.PointerTo(t).Implements(iface) {
			return true
		}
	}
	return false
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
	case reflect.Pointer, reflect.Interface:
		if reflectVal.IsNil() {
			return true
		}
		return excludingNoneGetIsNilOrUltimatelyPointsToNil_inner(
			reflectVal.Elem().Interface(),
			true,
		)

	case reflect.Map, reflect.Slice:
		return reflectVal.IsNil()

	default:
		return false
	}
}

type JSONShape struct {
	Fields   []JSONFieldShape
	Inlined  []JSONFieldShape
	Unknowns []JSONFieldShape
}

type JSONFieldShape struct {
	Field               reflect.StructField
	GoName              string
	JSONName            string
	Type                reflect.Type
	BaseType            reflect.Type
	Index               []int
	Pointer             bool
	Optional            bool
	OmitEmpty           bool
	OmitZero            bool
	Inline              bool
	Unknown             bool
	ViaOptionalEmbedded bool
}

type Value struct {
	V reflect.Value
}

type Type struct {
	T reflect.Type
}

func (typ Type) NewValue() reflect.Value {
	return reflect.New(typ.T).Elem()
}

func JSONStructShape(t reflect.Type) (JSONShape, error) {
	base, _ := JSONBaseType(t)
	if base == nil {
		return JSONShape{}, fmt.Errorf("JSONStructShape: type is nil")
	}
	if base.Kind() != reflect.Struct {
		return JSONShape{}, fmt.Errorf("JSONStructShape: expected struct, got %s", base)
	}

	return collect_json_shape(base)
}

func JSONStructFields(t reflect.Type) ([]JSONFieldShape, error) {
	shape, err := JSONStructShape(t)
	if err != nil {
		return nil, err
	}
	return shape.Fields, nil
}

func JSONBaseType(t reflect.Type) (reflect.Type, bool) {
	is_pointer := false
	for t != nil && t.Kind() == reflect.Pointer {
		is_pointer = true
		t = t.Elem()
	}
	return t, is_pointer
}

func json_indirect_type(t reflect.Type) reflect.Type {
	if t != nil && t.Kind() == reflect.Pointer && t.Name() == "" {
		return t.Elem()
	}
	return t
}

func IsScalarType(t reflect.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case reflect.Bool, reflect.String:
		return true
	default:
		return IsNumericKind(t.Kind())
	}
}

func IsNumericKind(kind reflect.Kind) bool {
	return IsSignedIntKind(kind) || IsUnsignedIntKind(kind) || IsFloatKind(kind)
}

func IsSignedIntKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

func IsUnsignedIntKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func IsFloatKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func (value Value) Deref() reflect.Value {
	v := value.V
	for v.IsValid() &&
		(v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func (value Value) PointsToNil() bool {
	v := value.V
	for v.IsValid() &&
		(v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	return false
}

func (value Value) StringBase() (reflect.Value, bool) {
	base := value.Deref()
	if !base.IsValid() || base.Kind() != reflect.String {
		return reflect.Value{}, false
	}
	return base, true
}

func (value Value) SignedIntBase() (reflect.Value, bool) {
	base := value.Deref()
	if !base.IsValid() || !IsSignedIntKind(base.Kind()) {
		return reflect.Value{}, false
	}
	return base, true
}

func (value Value) UnsignedIntBase() (reflect.Value, bool) {
	base := value.Deref()
	if !base.IsValid() || !IsUnsignedIntKind(base.Kind()) {
		return reflect.Value{}, false
	}
	return base, true
}

func (value Value) FloatBase() (reflect.Value, bool) {
	base := value.Deref()
	if !base.IsValid() || !IsFloatKind(base.Kind()) {
		return reflect.Value{}, false
	}
	return base, true
}

func (value Value) BoolBase() (reflect.Value, bool) {
	base := value.Deref()
	if !base.IsValid() || base.Kind() != reflect.Bool {
		return reflect.Value{}, false
	}
	return base, true
}

func (value Value) SliceOrArrayBase() (reflect.Value, bool) {
	base := value.Deref()
	if !base.IsValid() ||
		(base.Kind() != reflect.Slice && base.Kind() != reflect.Array) {
		return reflect.Value{}, false
	}
	return base, true
}

func (value Value) MapBase() (reflect.Value, bool) {
	base := value.Deref()
	if !base.IsValid() || base.Kind() != reflect.Map {
		return reflect.Value{}, false
	}
	return base, true
}

func (value Value) WriteString(base reflect.Value, s string) {
	if base.CanSet() {
		base.SetString(s)
		return
	}
	if value.V.CanSet() {
		value.V.Set(reflect.ValueOf(s).Convert(value.V.Type()))
	}
}

func (value Value) WriteInt(base reflect.Value, n int64) {
	if base.CanSet() {
		base.SetInt(n)
		return
	}
	if value.V.CanSet() {
		value.V.Set(reflect.ValueOf(n).Convert(value.V.Type()))
	}
}

func (value Value) WriteUint(base reflect.Value, n uint64) {
	if base.CanSet() {
		base.SetUint(n)
		return
	}
	if value.V.CanSet() {
		value.V.Set(reflect.ValueOf(n).Convert(value.V.Type()))
	}
}

func (value Value) WriteFloat(base reflect.Value, f float64) {
	if base.CanSet() {
		base.SetFloat(f)
		return
	}
	if value.V.CanSet() {
		value.V.Set(reflect.ValueOf(f).Convert(value.V.Type()))
	}
}

func (value Value) WriteBool(base reflect.Value, b bool) {
	if base.CanSet() {
		base.SetBool(b)
		return
	}
	if value.V.CanSet() {
		value.V.Set(reflect.ValueOf(b).Convert(value.V.Type()))
	}
}

func (value Value) SetZero() {
	if value.V.CanSet() {
		value.V.Set(reflect.Zero(value.V.Type()))
	}
}

func (value Value) EnsurePointerElem() (reflect.Value, bool) {
	if value.V.Kind() != reflect.Pointer {
		return value.V, value.V.CanSet()
	}
	if value.V.IsNil() {
		if !value.V.CanSet() {
			return reflect.Value{}, false
		}
		value.V.Set(reflect.New(value.V.Type().Elem()))
	}
	return value.V.Elem(), value.V.Elem().CanSet()
}

func (value Value) EnsureDeref() (reflect.Value, bool) {
	v := value.V
	for v.IsValid() {
		switch v.Kind() {
		case reflect.Interface:
			if v.IsNil() {
				return reflect.Value{}, false
			}
			elem := v.Elem()
			if elem.Kind() == reflect.Pointer && elem.IsNil() {
				if !v.CanSet() {
					return reflect.Value{}, false
				}
				elem = reflect.New(elem.Type().Elem())
				v.Set(elem)
			}
			v = elem
		case reflect.Pointer:
			if v.IsNil() {
				if !v.CanSet() {
					return reflect.Value{}, false
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		default:
			return v, v.CanSet()
		}
	}
	return reflect.Value{}, false
}

func (value Value) EnsureMap() bool {
	if value.V.Kind() != reflect.Map {
		return false
	}
	if value.V.IsNil() {
		if !value.V.CanSet() {
			return false
		}
		value.V.Set(reflect.MakeMap(value.V.Type()))
	}
	return true
}

func (value Value) SetEmptySlice() bool {
	if value.V.Kind() != reflect.Slice || !value.V.CanSet() {
		return false
	}
	value.V.Set(reflect.MakeSlice(value.V.Type(), 0, 0))
	return true
}

func (value Value) MakeSlice(length int) reflect.Value {
	return reflect.MakeSlice(value.V.Type(), length, length)
}

func (value Value) NewElemValue() reflect.Value {
	return reflect.New(value.V.Type().Elem()).Elem()
}

func (value Value) SettableCopy() reflect.Value {
	copy_val := reflect.New(value.V.Type()).Elem()
	copy_val.Set(value.V)
	return copy_val
}

func (value Value) MapValueCopy(key reflect.Value) (reflect.Value, bool) {
	existing := value.V.MapIndex(key)
	copy_val := value.NewElemValue()
	if !existing.IsValid() {
		return copy_val, false
	}
	copy_val.Set(existing)
	return copy_val, true
}

func (value Value) SetMapValue(key, val reflect.Value) {
	value.V.SetMapIndex(key, val)
}

func (value Value) DeleteMapValue(key reflect.Value) {
	value.V.SetMapIndex(key, reflect.Value{})
}

func (value Value) SortedMapKeys() []reflect.Value {
	keys := value.V.MapKeys()
	if value.V.Type().Key().Kind() == reflect.String {
		slices.SortFunc(keys, func(a, b reflect.Value) int {
			return cmp.Compare(a.String(), b.String())
		})
	}
	return keys
}

func (value Value) SetScalarFromString(raw string) error {
	if !value.V.CanSet() {
		return fmt.Errorf("field is not settable")
	}
	if raw == "" {
		return nil
	}
	switch value.V.Kind() {
	case reflect.String:
		value.V.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		value.V.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		value.V.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		value.V.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		value.V.SetBool(b)
	default:
		return fmt.Errorf("unsupported field type %s", value.V.Type())
	}
	return nil
}

func (value Value) EffectivelyZero() bool {
	v := value.V
	if !v.IsValid() {
		return true
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		return false
	case reflect.Map, reflect.Slice:
		return v.IsNil()
	default:
		return v.IsZero()
	}
}

func (value Value) FormatKey() string {
	v := value.V
	if !v.IsValid() {
		return "<invalid>"
	}
	if v.CanInterface() {
		return fmt.Sprintf("%v", v.Interface())
	}
	if v.Kind() == reflect.String {
		return v.String()
	}
	return "<unstringable>"
}

func (value Value) ResolveField(
	name string,
) (val reflect.Value, exists bool, write_back func(reflect.Value)) {
	base := value.V
	if base.Kind() == reflect.Struct {
		fields, err := JSONStructFields(base.Type())
		if err != nil {
			return reflect.Value{}, false, nil
		}
		for _, field := range fields {
			if field.JSONName != name && field.GoName != name {
				continue
			}
			fv, ok := field.SettableValue(base)
			return fv, ok, nil
		}
		return reflect.Value{}, false, nil
	}
	key := reflect.ValueOf(name)
	copy_val, exists := (Value{V: base}).MapValueCopy(key)
	write_back = func(v reflect.Value) { (Value{V: base}).SetMapValue(key, v) }
	return copy_val, exists, write_back
}

func (value Value) CountNonZeroFields(names []string) int {
	count := 0
	for _, name := range names {
		val, _, _ := value.ResolveField(name)
		if !(Value{V: val}).EffectivelyZero() {
			count++
		}
	}
	return count
}

func (value Value) InterfaceOrNil() any {
	if value.V.IsValid() && value.V.CanInterface() {
		return value.V.Interface()
	}
	return nil
}

func (value Value) InterfaceImpl(iface reflect.Type) (reflect.Value, bool) {
	if iface == nil {
		return reflect.Value{}, false
	}
	if iface.Kind() != reflect.Interface {
		panic("reflectutil error: expected interface type")
	}
	if !value.V.IsValid() {
		return reflect.Value{}, false
	}
	if value.V.Type().Implements(iface) && value.V.CanInterface() {
		return value.V, true
	}
	if value.V.Kind() == reflect.Pointer {
		return reflect.Value{}, false
	}
	if reflect.PointerTo(value.V.Type()).Implements(iface) {
		if value.V.CanAddr() {
			addr := value.V.Addr()
			if addr.CanInterface() {
				return addr, true
			}
		}
		holder := reflect.New(value.V.Type())
		holder.Elem().Set(value.V)
		return holder, true
	}
	return reflect.Value{}, false
}

func (value Value) InitAnonymousPointerFields() {
	v := value.V
	if v.IsValid() && v.Kind() != reflect.Pointer && v.CanAddr() {
		v = v.Addr()
	}
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < elem.NumField(); i++ {
		f := elem.Field(i)
		ft := elem.Type().Field(i)
		if ft.Anonymous && f.Kind() == reflect.Pointer && f.IsNil() {
			nv := reflect.New(f.Type().Elem())
			f.Set(nv)
			Value{V: nv}.InitAnonymousPointerFields()
		}
	}
}

func (value Value) StringEnumValues() ([]string, error) {
	base := value.Deref()
	if !base.IsValid() || base.Kind() != reflect.Struct {
		return nil, fmt.Errorf("enum must be a struct or pointer to struct")
	}
	out := make([]string, 0, base.NumField())
	for i := 0; i < base.NumField(); i++ {
		field_type := base.Type().Field(i)
		if !field_type.IsExported() {
			continue
		}
		fv := Value{V: base.Field(i)}.Deref()
		if !fv.IsValid() {
			continue
		}
		if fv.Kind() != reflect.String {
			return nil, fmt.Errorf(
				"enum field %q is not string-kinded", field_type.Name,
			)
		}
		out = append(out, fv.String())
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("enum is empty")
	}
	return out, nil
}

func (value Value) IntEnumValues() ([]int64, error) {
	base := value.Deref()
	if !base.IsValid() || base.Kind() != reflect.Struct {
		return nil, fmt.Errorf("enum must be a struct or pointer to struct")
	}
	out := make([]int64, 0, base.NumField())
	for i := 0; i < base.NumField(); i++ {
		field_type := base.Type().Field(i)
		if !field_type.IsExported() {
			continue
		}
		fv := Value{V: base.Field(i)}.Deref()
		if !fv.IsValid() {
			continue
		}
		if !IsSignedIntKind(fv.Kind()) {
			return nil, fmt.Errorf(
				"enum field %q is not integer-kinded", field_type.Name,
			)
		}
		out = append(out, fv.Int())
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("enum is empty")
	}
	return out, nil
}

func (field JSONFieldShape) StructBaseType() (reflect.Type, bool) {
	if field.BaseType == nil || field.BaseType.Kind() != reflect.Struct {
		return nil, false
	}
	return field.BaseType, true
}

func (field JSONFieldShape) SettableValue(v reflect.Value) (reflect.Value, bool) {
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.Kind() == reflect.Pointer && v.IsNil() {
			if !v.CanSet() {
				return reflect.Value{}, false
			}
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	for i, field_index := range field.Index {
		if v.Kind() != reflect.Struct || field_index >= v.NumField() {
			return reflect.Value{}, false
		}
		v = v.Field(field_index)

		if i == len(field.Index)-1 {
			return v, v.CanSet()
		}
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !v.CanSet() {
					return reflect.Value{}, false
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
	}

	return v, v.CanSet()
}

func (field JSONFieldShape) SettableCompositeValue(
	v reflect.Value,
) (reflect.Value, bool) {
	v, ok := field.SettableValue(v)
	if !ok {
		return reflect.Value{}, false
	}
	if v.Kind() != reflect.Pointer {
		return v, v.CanSet()
	}

	elem_kind := v.Type().Elem().Kind()
	if elem_kind != reflect.Struct && elem_kind != reflect.Map &&
		elem_kind != reflect.Slice {
		return v, v.CanSet()
	}
	if v.IsNil() {
		if !v.CanSet() {
			return reflect.Value{}, false
		}
		v.Set(reflect.New(v.Type().Elem()))
	}
	v = v.Elem()
	return v, v.CanSet()
}

type json_field_options struct {
	name       string
	has_name   bool
	inline     bool
	unknown    bool
	omit_empty bool
	omit_zero  bool
}

type json_field_candidate struct {
	shape  JSONFieldShape
	tagged bool
	id     int
}

type json_field_queue_entry struct {
	typ                   reflect.Type
	index                 []int
	via_optional_embedded bool
	visit_children        bool
}

func collect_json_shape(root reflect.Type) (JSONShape, error) {
	queue := []json_field_queue_entry{{
		typ:            root,
		visit_children: true,
	}}
	seen := map[reflect.Type]bool{root: true}
	var candidates []json_field_candidate
	var inlined []JSONFieldShape
	var unknown []JSONFieldShape

	for queue_index := 0; queue_index < len(queue); queue_index++ {
		entry := queue[queue_index]
		t := entry.typ
		for field := range t.Fields() {
			options, ignored, err := parse_json_field_options(field)
			if err != nil {
				return JSONShape{}, err
			}
			if ignored {
				continue
			}

			index := append_index(entry.index, field.Index)
			base, pointer := JSONBaseType(field.Type)
			inline_base := json_indirect_type(field.Type)
			if field.Anonymous && !options.has_name &&
				(inline_base == nil || inline_base.Kind() != reflect.Struct) {
				return JSONShape{}, fmt.Errorf(
					"embedded Go struct field %s of non-struct type must be explicitly given a JSON name",
					field.Name,
				)
			}
			inline := options.inline ||
				field.Anonymous && !options.has_name &&
					inline_base != nil && inline_base.Kind() == reflect.Struct
			if inline || options.unknown {
				shape := json_field_shape(
					field,
					options,
					index,
					base,
					pointer,
					entry.via_optional_embedded,
				)
				shape.Inline = inline
				shape.Unknown = options.unknown

				if options.unknown {
					if inline_base == nil || inline_base.Kind() != reflect.Map ||
						inline_base.Key().Kind() != reflect.String {
						return JSONShape{}, fmt.Errorf(
							"Go struct field %s with `unknown` tag must be a map with string keys",
							field.Name,
						)
					}
					unknown = append(unknown, shape)
					continue
				}

				if inline_base == nil || inline_base.Kind() != reflect.Struct {
					if inline_base != nil && inline_base.Kind() == reflect.Map &&
						inline_base.Key().Kind() == reflect.String {
						inlined = append(inlined, shape)
						continue
					}
					return JSONShape{}, fmt.Errorf(
						"inlined Go struct field %s of type %s must be a Go struct or map with string keys",
						field.Name,
						field.Type,
					)
				}

				inlined = append(inlined, shape)
				if entry.visit_children {
					queue = append(queue, json_field_queue_entry{
						typ:                   base,
						index:                 index,
						via_optional_embedded: entry.via_optional_embedded || pointer,
						visit_children:        !seen[base],
					})
				}
				seen[base] = true
				continue
			}

			candidates = append(candidates, json_field_candidate{
				shape: json_field_shape(
					field,
					options,
					index,
					base,
					pointer,
					entry.via_optional_embedded,
				),
				tagged: options.has_name,
				id:     len(candidates),
			})
		}
	}

	slices.SortStableFunc(candidates, func(a, b json_field_candidate) int {
		return cmp.Or(
			strings.Compare(a.shape.JSONName, b.shape.JSONName),
			cmp.Compare(len(a.shape.Index), len(b.shape.Index)),
			compare_bools(!a.tagged, !b.tagged),
		)
	})

	dominant := candidates[:0]
	for len(candidates) > 0 {
		n := 1
		for n < len(candidates) &&
			candidates[n-1].shape.JSONName == candidates[n].shape.JSONName {
			n++
		}
		if n == 1 ||
			len(candidates[0].shape.Index) != len(candidates[1].shape.Index) ||
			candidates[0].tagged != candidates[1].tagged {
			dominant = append(dominant, candidates[0])
		}
		candidates = candidates[n:]
	}

	slices.SortFunc(dominant, func(a, b json_field_candidate) int {
		return cmp.Compare(a.id, b.id)
	})
	slices.SortFunc(dominant, func(a, b json_field_candidate) int {
		return slices.Compare(a.shape.Index, b.shape.Index)
	})

	out := make([]JSONFieldShape, 0, len(dominant))
	for _, field := range dominant {
		out = append(out, field.shape)
	}
	return JSONShape{
		Fields:   out,
		Inlined:  inlined,
		Unknowns: unknown,
	}, nil
}

func json_field_shape(
	field reflect.StructField,
	options json_field_options,
	index []int,
	base reflect.Type,
	pointer bool,
	via_optional_embedded bool,
) JSONFieldShape {
	return JSONFieldShape{
		Field:    field,
		GoName:   field.Name,
		JSONName: options.name,
		Type:     field.Type,
		BaseType: base,
		Index:    index,
		Pointer:  pointer,
		Optional: via_optional_embedded || pointer || options.omit_empty ||
			options.omit_zero,
		OmitEmpty:           options.omit_empty,
		OmitZero:            options.omit_zero,
		Inline:              options.inline,
		Unknown:             options.unknown,
		ViaOptionalEmbedded: via_optional_embedded,
	}
}

func parse_json_field_options(
	field reflect.StructField,
) (json_field_options, bool, error) {
	tag, has_tag := field.Tag.Lookup("json")
	if tag == "-" {
		return json_field_options{}, true, nil
	}
	if !field.IsExported() && !field.Anonymous {
		return json_field_options{}, true, nil
	}

	out := json_field_options{name: field.Name}
	if tag != "" && !strings.HasPrefix(tag, ",") {
		name, n, err := consume_json_tag_name(tag)
		if err != nil {
			return json_field_options{}, false, fmt.Errorf(
				"Go struct field %s has malformed `json` tag: %w",
				field.Name,
				err,
			)
		}
		if !utf8.ValidString(name) {
			name = string([]rune(name))
		}
		out.has_name = true
		out.name = name
		tag = tag[n:]
	}
	if !has_tag {
		return out, false, nil
	}

	seen := make(map[string]bool)
	for tag != "" {
		if tag[0] != ',' {
			return json_field_options{}, false, fmt.Errorf(
				"Go struct field %s has malformed `json` tag",
				field.Name,
			)
		}
		tag = tag[1:]
		if tag == "" {
			return json_field_options{}, false, fmt.Errorf(
				"Go struct field %s has malformed `json` tag",
				field.Name,
			)
		}

		option, n, err := consume_json_tag_option(tag)
		if err != nil {
			return json_field_options{}, false, fmt.Errorf(
				"Go struct field %s has malformed `json` tag: %w",
				field.Name,
				err,
			)
		}
		tag = tag[n:]
		if seen[option] {
			return json_field_options{}, false, fmt.Errorf(
				"Go struct field %s has duplicate `%s` tag option",
				field.Name,
				option,
			)
		}
		seen[option] = true

		switch option {
		case "inline":
			out.inline = true
		case "unknown":
			out.unknown = true
		case "omitempty":
			out.omit_empty = true
		case "omitzero":
			out.omit_zero = true
		case "string":
		case "case", "format":
			if !strings.HasPrefix(tag, ":") {
				return json_field_options{}, false, fmt.Errorf(
					"Go struct field %s is missing value for `%s` tag option",
					field.Name,
					option,
				)
			}
			tag = tag[1:]
			_, n, err := consume_json_tag_option(tag)
			if err != nil {
				return json_field_options{}, false, err
			}
			tag = tag[n:]
		default:
		}
	}

	if out.inline && out.unknown {
		return json_field_options{}, false, fmt.Errorf(
			"Go struct field %s cannot have both `inline` and `unknown` specified",
			field.Name,
		)
	}
	if (out.inline || out.unknown) && (out.has_name || len(seen) > 1) {
		return json_field_options{}, false, fmt.Errorf(
			"Go struct field %s cannot combine `inline` or `unknown` with other JSON tag options",
			field.Name,
		)
	}

	return out, false, nil
}

func consume_json_tag_name(tag string) (string, int, error) {
	if strings.HasPrefix(tag, "'") {
		return consume_json_tag_option(tag)
	}
	n := len(tag) - len(strings.TrimLeftFunc(tag, func(r rune) bool {
		return !strings.ContainsRune(",\\'\"`", r)
	}))
	return tag[:n], n, nil
}

func consume_json_tag_option(tag string) (string, int, error) {
	i := strings.IndexByte(tag, ',')
	if i < 0 {
		i = len(tag)
	}

	r, _ := utf8.DecodeRuneInString(tag)
	switch {
	case r == '_' || unicode.IsLetter(r):
		n := len(tag) - len(strings.TrimLeftFunc(tag, is_json_tag_letter_or_digit))
		return tag[:n], n, nil
	case r == '\'':
		var in_escape bool
		b := []byte{'"'}
		n := len("'")
		for len(tag) > n {
			r, rn := utf8.DecodeRuneInString(tag[n:])
			switch {
			case in_escape:
				if r == '\'' {
					b = b[:len(b)-1]
				}
				in_escape = false
			case r == '\\':
				in_escape = true
			case r == '"':
				b = append(b, '\\')
			case r == '\'':
				b = append(b, '"')
				n += len("'")
				out, err := strconv.Unquote(string(b))
				if err != nil {
					return tag[:i], i, err
				}
				return out, n, nil
			}
			b = append(b, tag[n:][:rn]...)
			n += rn
		}
		return tag[:i], i, fmt.Errorf("single-quoted string not terminated")
	case tag == "":
		return "", 0, fmt.Errorf("unexpected end of tag")
	default:
		return tag[:i], i, fmt.Errorf("invalid character %q at start of option", r)
	}
}

func is_json_tag_letter_or_digit(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

func compare_bools(a, b bool) int {
	switch {
	case !a && b:
		return -1
	case a && !b:
		return 1
	default:
		return 0
	}
}

func append_index(prefix, suffix []int) []int {
	out := make([]int, 0, len(prefix)+len(suffix))
	out = append(out, prefix...)
	out = append(out, suffix...)
	return out
}
