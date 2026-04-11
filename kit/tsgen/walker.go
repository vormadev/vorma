package tsgen

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/reflectutil"
)

/////////////////////////////////////////////////////////////////////
/////// Public interfaces — the type-author contract
/////////////////////////////////////////////////////////////////////

// TSTyper lets a struct override specific field types or add extra fields.
// Keys are Go field names. Values are literal TS type strings, or
// sentinel IDs obtained from Type.ID() for referencing
// system-managed types.
type TSTyper interface {
	TSType() map[string]string
}

// TSTyperRaw lets a struct bypass reflection entirely.
// The returned string becomes the whole TS type body verbatim.
type TSTyperRaw interface {
	TSType() string
}

/////////////////////////////////////////////////////////////////////
/////// Node types — the layer-1 output
/////////////////////////////////////////////////////////////////////

type node_kind int

const (
	kind_null node_kind = iota
	kind_unknown
	kind_bool
	kind_number
	kind_string
	kind_object
	kind_array
	kind_map
	kind_ref // reference to a named type by id
	kind_raw // literal TS string, not reflected
)

type type_node struct {
	kind     node_kind
	fields   []field_node // kind_object
	elem     *type_node   // kind_array
	key_type *type_node   // kind_map
	val_type *type_node   // kind_map
	ref_id   string       // kind_ref (sentinel-wrapped)
	raw_ts   string       // kind_raw
}

type field_node struct {
	name     string
	node     *type_node
	optional bool
}

/////////////////////////////////////////////////////////////////////
/////// Walk entry — one discovered type
/////////////////////////////////////////////////////////////////////

type walk_entry struct {
	id               string
	requested_name   string
	node             *type_node
	is_root          bool
	is_referenced    bool
	used_as_embedded bool
}

/////////////////////////////////////////////////////////////////////
/////// Walker state
/////////////////////////////////////////////////////////////////////

type walker struct {
	types               map[reflect.Type]*type_reg
	root_type           reflect.Type
	root_requested_name string
}

type type_reg struct {
	requested_name   string
	is_referenced    bool
	used_as_embedded bool
	visited          bool
}

func new_walker(root reflect.Type, root_name string) *walker {
	return &walker{
		types:               make(map[reflect.Type]*type_reg),
		root_type:           root,
		root_requested_name: root_name,
	}
}

/////////////////////////////////////////////////////////////////////
/////// Entry point
/////////////////////////////////////////////////////////////////////

// walk_type walks a Go value's type via reflection and returns all
// discovered type entries plus the root entry's ID.
func walk_type(
	instance any,
	requested_name string,
) (map[string]*walk_entry, string) {
	if instance == nil {
		return nil, ""
	}

	t := effective_reflect_type(instance)
	if t == nil {
		return nil, ""
	}

	eff_name := effective_requested_name(t, requested_name)

	// TSTyperRaw: skip reflection entirely.
	if raw_str, ok := check_ts_typer_raw(instance); ok {
		id := make_id(t, eff_name)
		return map[string]*walk_entry{
			id: {
				id:             id,
				requested_name: eff_name,
				node:           &type_node{kind: kind_raw, raw_ts: raw_str},
				is_root:        true,
			},
		}, id
	}

	w := new_walker(t, eff_name)

	// Phase 1: discover all reachable types.
	w.collect(t, requested_name)

	// Phase 2: build a type_node for each discovered type.
	entries := w.build_entries()

	root_id := make_id(t, eff_name)
	if e, ok := entries[root_id]; ok {
		e.is_root = true
	}

	return entries, root_id
}

/////////////////////////////////////////////////////////////////////
/////// Phase 1 — collect types
/////////////////////////////////////////////////////////////////////

func (w *walker) get_or_create_reg(
	t reflect.Type,
	user_alias ...string,
) *type_reg {
	if reg, ok := w.types[t]; ok {
		if t == w.root_type && w.root_requested_name != "" &&
			reg.requested_name == "" {
			reg.requested_name = w.root_requested_name
		}
		return reg
	}

	reg := &type_reg{}

	switch {
	case t == w.root_type && w.root_requested_name != "":
		reg.requested_name = w.root_requested_name
	case len(user_alias) > 0 && user_alias[0] != "":
		reg.requested_name = user_alias[0]
	default:
		if !is_basic_type(t) {
			reg.requested_name = sanitized_name(t)
		}
	}

	w.types[t] = reg
	return reg
}

func (w *walker) collect(t reflect.Type, user_alias ...string) {
	if t == nil {
		return
	}

	is_root := t == w.root_type

	if t.Name() != "" || is_root {
		reg := w.get_or_create_reg(t, user_alias...)
		if reg.visited {
			return
		}
		reg.visited = true
		if !is_root && is_basic_type(t) {
			return
		}
	} else if !is_root && is_basic_type(t) {
		return
	}

	switch t.Kind() {
	case reflect.Struct:
		w.collect_struct_fields(t)
	case reflect.Pointer:
		if t.Name() != "" {
			w.get_or_create_reg(t, user_alias...)
		}
		w.collect(t.Elem())
	case reflect.Slice, reflect.Array:
		if t.Name() != "" {
			w.get_or_create_reg(t, user_alias...)
		}
		w.collect(t.Elem())
	case reflect.Map:
		if t.Name() != "" {
			w.get_or_create_reg(t, user_alias...)
		}
		w.collect(t.Key())
		w.collect(t.Elem())
	}
}

func (w *walker) collect_struct_fields(t reflect.Type) {
	for i := range t.NumField() {
		field := t.Field(i)
		if is_unexported(field) || should_omit_field(field) {
			continue
		}

		if field.Anonymous {
			ft := field.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				reg := w.get_or_create_reg(ft)
				reg.used_as_embedded = true
				if tag := field.Tag.Get("json"); tag != "" && tag != "-" {
					reg.is_referenced = true
				}
				w.collect(ft)
			}
			continue
		}

		w.collect_field_type(field.Type)
	}
}

func (w *walker) collect_field_type(t reflect.Type) {
	switch t.Kind() {
	case reflect.Struct:
		w.get_or_create_reg(t).is_referenced = true
		w.collect(t)
	case reflect.Pointer:
		if t.Name() != "" {
			w.get_or_create_reg(t).is_referenced = true
		}
		elem := t.Elem()
		if elem.Kind() == reflect.Struct {
			w.get_or_create_reg(elem).is_referenced = true
		}
		w.collect(elem)
	case reflect.Slice, reflect.Array:
		if t.Name() != "" {
			w.get_or_create_reg(t).is_referenced = true
		}
		elem := t.Elem()
		if elem.Kind() == reflect.Struct {
			w.get_or_create_reg(elem).is_referenced = true
		} else if elem.Kind() == reflect.Pointer && elem.Elem().Kind() == reflect.Struct {
			w.get_or_create_reg(elem.Elem()).is_referenced = true
		}
		w.collect(elem)
	case reflect.Map:
		if t.Name() != "" {
			w.get_or_create_reg(t).is_referenced = true
		}
		w.collect(t.Key())
		w.collect(t.Elem())
	}
}

/////////////////////////////////////////////////////////////////////
/////// Phase 2 — build type_nodes for each collected type
/////////////////////////////////////////////////////////////////////

func (w *walker) build_entries() map[string]*walk_entry {
	entries := make(map[string]*walk_entry, len(w.types))

	for t, reg := range w.types {
		if t == nil {
			continue
		}
		id := make_id(t, reg.requested_name)
		entries[id] = &walk_entry{
			id:               id,
			requested_name:   reg.requested_name,
			node:             w.to_node(t),
			is_referenced:    reg.is_referenced,
			used_as_embedded: reg.used_as_embedded,
		}
	}

	return entries
}

func (w *walker) to_node(t reflect.Type) *type_node {
	if t == nil {
		return &type_node{kind: kind_null}
	}

	switch t.Kind() {
	case reflect.Interface:
		return &type_node{kind: kind_unknown}
	case reflect.Bool:
		return &type_node{kind: kind_bool}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return &type_node{kind: kind_number}
	case reflect.String:
		return &type_node{kind: kind_string}
	case reflect.Pointer:
		return w.to_node(t.Elem())
	case reflect.Slice, reflect.Array:
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return &type_node{kind: kind_string} // []byte → base64 string
		}
		return &type_node{kind: kind_array, elem: w.to_node_or_ref(t.Elem())}
	case reflect.Map:
		return &type_node{
			kind:     kind_map,
			key_type: w.to_node_or_ref(t.Key()),
			val_type: w.to_node_or_ref(t.Elem()),
		}
	case reflect.Struct:
		switch {
		case t == reflect.TypeFor[time.Time]():
			return &type_node{kind: kind_string}
		case t == reflect.TypeFor[time.Duration]():
			return &type_node{kind: kind_number}
		default:
			return w.struct_to_node(t)
		}
	default:
		return &type_node{kind: kind_unknown}
	}
}

// to_node_or_ref returns a kind_ref if the type has a named entry,
// otherwise inlines via to_node.
func (w *walker) to_node_or_ref(t reflect.Type) *type_node {
	if t == nil {
		return &type_node{kind: kind_null}
	}

	// Pointers are transparent in TS.
	effective := t
	if effective.Kind() == reflect.Pointer {
		effective = effective.Elem()
	}

	// Named structs (non-basic) that were collected get a reference.
	if effective.Kind() == reflect.Struct && effective.Name() != "" &&
		!is_basic_type(effective) {
		if reg, ok := w.types[effective]; ok {
			return &type_node{
				kind:   kind_ref,
				ref_id: make_id(effective, reg.requested_name),
			}
		}
	}

	return w.to_node(t)
}

func (w *walker) struct_to_node(t reflect.Type) *type_node {
	return &type_node{kind: kind_object, fields: w.build_struct_fields(t)}
}

func (w *walker) build_struct_fields(t reflect.Type) []field_node {
	ts_type_map := get_ts_type_map(t)
	used_overrides := make(map[string]bool)
	var fields []field_node

	var process func(ct reflect.Type, is_embedded_ptr bool)
	process = func(ct reflect.Type, is_embedded_ptr bool) {
		for i := range ct.NumField() {
			field := ct.Field(i)
			if is_unexported(field) || should_omit_field(field) {
				continue
			}

			// Untagged anonymous field → recurse to match encoding/json order.
			if field.Anonymous && field.Tag.Get("json") == "" {
				et := field.Type
				is_ptr := et.Kind() == reflect.Pointer
				if is_ptr {
					et = et.Elem()
				}
				if et.Kind() == reflect.Struct {
					process(et, is_ptr || is_embedded_ptr)
				}
				continue
			}

			json_name := json_field_name(field)
			if json_name == "" {
				continue
			}

			var node *type_node

			// Precedence: TSTyper > ts_type tag > reflection.
			if custom, ok := ts_type_map[field.Name]; ok {
				node = &type_node{kind: kind_raw, raw_ts: custom}
				used_overrides[field.Name] = true
			} else if custom := field.Tag.Get("ts_type"); custom != "" {
				node = &type_node{kind: kind_raw, raw_ts: custom}
			} else {
				node = w.to_node_or_ref(field.Type)
			}

			fields = append(fields, field_node{
				name:     json_name,
				node:     node,
				optional: is_embedded_ptr || is_optional_field(field),
			})
		}
	}

	process(t, false)

	// Additive fields from TSTyper that didn't match any Go field.
	if ts_type_map != nil {
		var additive []string
		for key := range ts_type_map {
			if !used_overrides[key] {
				additive = append(additive, key)
			}
		}
		slices.Sort(additive)
		for _, key := range additive {
			fields = append(fields, field_node{
				name:     key,
				node:     &type_node{kind: kind_raw, raw_ts: ts_type_map[key]},
				optional: false,
			})
		}
	}

	return fields
}

/////////////////////////////////////////////////////////////////////
/////// Reflection helpers
/////////////////////////////////////////////////////////////////////

func effective_reflect_type(instance any) reflect.Type {
	t := reflect.TypeOf(instance)
	if t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func effective_requested_name(t reflect.Type, requested string) string {
	if requested != "" {
		return requested
	}
	return natural_name(t)
}

func natural_name(t reflect.Type) string {
	if t == nil {
		return ""
	}
	n := sanitized_name(t)
	if n != "" && is_basic_type(t) {
		return ""
	}
	return n
}

var invalid_js_ident_chars = regexp.MustCompile(`[^a-zA-Z0-9_$]`)

func sanitized_name(t reflect.Type) string {
	if t == nil {
		return ""
	}
	x := invalid_js_ident_chars.ReplaceAllString(t.Name(), "_")
	if len(x) > 0 && x[len(x)-1] == '_' {
		x = x[:len(x)-1]
	}
	return x
}

func make_id(t reflect.Type, requested_name string) string {
	natural := natural_name(t)
	eff := effective_requested_name(t, requested_name)
	var raw string
	if eff != "" && eff != natural {
		raw = fmt.Sprintf("%v+%s", t, requested_name)
	} else {
		raw = fmt.Sprintf("%v", t)
	}
	return strings.ToLower(
		bytesutil.ToBase32Raw(cryptoutil.Sha256Hash([]byte(raw))),
	)
}

func is_basic_type(t reflect.Type) bool {
	if t == nil {
		return false
	}
	if t == reflect.TypeFor[time.Time]() ||
		t == reflect.TypeFor[time.Duration]() {
		return true
	}
	switch t.Kind() {
	case reflect.Interface, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return true
	default:
		return false
	}
}

func is_unexported(field reflect.StructField) bool {
	return field.PkgPath != ""
}

func is_optional_field(field reflect.StructField) bool {
	if field.Type.Kind() == reflect.Pointer {
		return true
	}
	tag := field.Tag.Get("json")
	if tag != "" {
		parts := strings.Split(tag, ",")
		for _, part := range parts[1:] {
			if part == "omitempty" || part == "omitzero" {
				return true
			}
		}
	}
	return false
}

func should_omit_field(field reflect.StructField) bool {
	tag := field.Tag.Get("json")
	return tag == "-" || strings.HasPrefix(tag, "-,")
}

func json_field_name(field reflect.StructField) string {
	return reflectutil.JSONFieldName(field)
}

/////////////////////////////////////////////////////////////////////
/////// Interface detection
/////////////////////////////////////////////////////////////////////

func check_ts_typer_raw(instance any) (string, bool) {
	if instance == nil {
		return "", false
	}

	// Direct check (value or pointer that already satisfies).
	if r, ok := instance.(TSTyperRaw); ok {
		return r.TSType(), true
	}

	// Pointer-receiver check: create *T and test.
	t := reflect.TypeOf(instance)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if r, ok := reflect.New(t).Interface().(TSTyperRaw); ok {
		return r.TSType(), true
	}

	return "", false
}

func get_ts_type_map(t reflect.Type) map[string]string {
	if t == nil {
		return nil
	}

	iface := reflect.TypeFor[TSTyper]()

	// Value receiver.
	if t.Implements(iface) {
		v := reflect.New(t).Elem()
		init_embedded_pointers(v.Addr())
		return v.Interface().(TSTyper).TSType()
	}

	// Pointer receiver.
	if reflectutil.DoesTypeImplementInterface(t, iface) {
		v := reflect.New(t)
		init_embedded_pointers(v)
		return v.Interface().(TSTyper).TSType()
	}

	return nil
}

func init_embedded_pointers(v reflect.Value) {
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
			init_embedded_pointers(nv)
		}
	}
}
