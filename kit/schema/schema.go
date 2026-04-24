// Package schema provides declarative normalization, defaulting, and
// validation of Go values.
//
// Types describe their own rules by implementing Schematic. Callers
// then run those rules with Enforce.
//
// # Quick start
//
//	type Email string
//
//	func (Email) Schema() schema.Schema {
//	    return schema.String{
//	        TrimSpace:   true,
//	        ToLower:     true,
//	        MustBeEmail: true,
//	    }
//	}
//
//	type User struct {
//	    Email Email
//	    Name  string
//	}
//
//	func (User) Schema() schema.Schema {
//	    return schema.Object{
//	        "Email": schema.String{MustNotBeZero: true},
//	        "Name":  schema.String{MustNotBeZero: true, TrimSpace: true, MinLen: 1},
//	    }
//	}
//
// Callers run the schema by calling Enforce (or EnforceAny).
//
// # Mutation contract
//
// Enforce uses a mutate-when-possible strategy. When the caller
// passes a pointer, the pointee is mutated in place by normalizations
// and defaults; Result.Value holds the same pointer. When the caller
// passes a value, Go copies it on the function call, so the caller's
// original variable is unchanged and Result.Value holds the
// transformed copy. In both cases, Result.Value is the canonical way
// to read the output.
//
// Two locations cannot be mutated in place even through a pointer
// input, because Go forbids it: map keys, and values stored by-value
// inside an interface field. For map keys, the containing map is
// rebuilt so the normalized key exists; for interface-boxed
// by-value fields, the new value is boxed back into the slot. Both
// are transparent to the caller.
//
// # Presence and zero vocabulary
//
// Two separate concepts drive rule behavior: nil-ness and zero-ness.
// Nil-ness is meaningful only for nilable slots (pointers,
// interfaces, slices, maps). Zero-ness is meaningful for scalar
// values and is defined per type: the empty string, numeric zero,
// and so on. Bool does not use zero vocabulary; it uses MustBeTrue
// and MustBeFalse instead.
//
// The four control flags, applied per type where meaningful, are:
//
//   - MustNotBeNil: reject a nil slot.
//   - MustNotBeZero: reject a zero value after nil handling.
//   - DefaultIfNil: replace a nil slot with a supplied default.
//   - DefaultIfZero: replace a zero value after nil handling with a supplied default.
//
// Nil handling runs as a pre-step before leaf rules. On a nil slot:
// DefaultIfNil (if set) materializes the value and the default flows
// into the leaf rule; otherwise MustNotBeNil (if set) produces a
// validation error and leaf processing stops. Every leaf rule
// therefore operates on a concrete, non-nil value.
//
// # Execution order
//
// For each leaf value: (1) nil handling and type dispatch,
// (2) SkipFunc on the concrete value, (3) normalize (TrimSpace,
// case, TransformFunc), (4) DefaultIfZero if still zero,
// (5) write back, (6) value validators, (7) ValidateFunc.
//
// For objects: (1) field rules, (2) TransformFunc,
// (3) ValidateFunc.
package schema

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/vormadev/vorma/kit/reflectutil"
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC CONSTANTS
/////////////////////////////////////////////////////////////////////

// Reserved Object keys. These byte sequences begin with a NUL so
// they cannot collide with Go identifiers or typical map keys.
const (
	TransformFunc = "\x00transform_func"
	ValidateFunc  = "\x00validate_func"
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC INTERFACES, RESULTS, AND ERRORS
/////////////////////////////////////////////////////////////////////

// Schema is the interface that all schema return values satisfy:
// rule blocks (String, Int, ...) for newtype schemas and Object for
// struct / string-keyed-map schemas.
type Schema interface{ is_schema() }

// Schematic is implemented by types that describe their own schema.
type Schematic interface {
	Schema() Schema
}

// Result wraps the transformed output of Enforce. Callers access
// res.Value to read the output. The wrapper exists so that chaining
// and re-assignment do not accidentally shadow the input variable.
type Result[T any] struct {
	Value T
}

// ResultAny is the type-erased form of Result.
type ResultAny struct {
	Value any
}

// SchemaError is returned when the schema itself is malformed (e.g.,
// it references a field that does not exist on the target type, or
// applies a rule block to a field of the wrong kind). These are bugs
// in the schema, not failures of the data being validated.
type SchemaError struct{ Err error }

func (e *SchemaError) Error() string { return e.Err.Error() }
func (e *SchemaError) Unwrap() error { return e.Err }

// ValidationError wraps data-validation failures.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// IsValidationError reports whether err is or wraps a ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// IsSchemaError reports whether err is or wraps a SchemaError.
func IsSchemaError(err error) bool {
	var se *SchemaError
	return errors.As(err, &se)
}

/////////////////////////////////////////////////////////////////////
/////// OBJECT (struct / string-keyed-map schema)
/////////////////////////////////////////////////////////////////////

// Object is the schema for a struct or a string-keyed map. The string
// keys are either field names (when applied to a struct) or map keys,
// with reserved sentinel keys for object-level callbacks.
type Object map[string]any

func (Object) is_schema() {}

func is_reserved_key(k string) bool {
	return k == TransformFunc || k == ValidateFunc
}

/////////////////////////////////////////////////////////////////////
/////// RULE BLOCK: STRING
/////////////////////////////////////////////////////////////////////

// String describes rules for a string-kinded value or field.
type String struct {
	// Nil / zero controls.
	MustNotBeNil  bool
	MustNotBeZero bool
	DefaultIfNil  any
	DefaultIfZero any

	// Normalizations are applied in order: TrimSpace, case, TransformFunc.
	TrimSpace     bool
	ToLower       bool
	ToUpper       bool
	TransformFunc func(string) (string, error)

	// Validators. MinLen and MaxLen accept any integer-kinded value.
	MinLen        any
	MaxLen        any
	MustBeIn      []string
	MustNotBeIn   []string
	MustBeEmail   bool
	MustBeURL     bool
	MustMatch     *regexp.Regexp
	MustStartWith string
	MustEndWith   string
	AllowedChars  string

	// ValidateFunc runs after built-in validators on the final
	// post-normalization value.
	ValidateFunc func(string) error

	// SkipFunc, if non-nil and returns true, causes this rule to be
	// skipped entirely for this value.
	SkipFunc func(string) bool
}

func (String) is_schema() {}

/////////////////////////////////////////////////////////////////////
/////// RULE BLOCK: INT
/////////////////////////////////////////////////////////////////////

// Int describes rules for a signed-integer-kinded value or field.
type Int struct {
	MustNotBeNil  bool
	MustNotBeZero bool
	DefaultIfNil  any
	DefaultIfZero any

	Min          any
	Max          any
	MustBeIn     []int
	MustNotBeIn  []int
	ValidateFunc func(int) error
	SkipFunc     func(int) bool
}

func (Int) is_schema() {}

/////////////////////////////////////////////////////////////////////
/////// RULE BLOCK: UINT
/////////////////////////////////////////////////////////////////////

// Uint describes rules for an unsigned-integer-kinded value or field.
type Uint struct {
	MustNotBeNil  bool
	MustNotBeZero bool
	DefaultIfNil  any
	DefaultIfZero any

	Min          any
	Max          any
	MustBeIn     []uint
	MustNotBeIn  []uint
	ValidateFunc func(uint) error
	SkipFunc     func(uint) bool
}

func (Uint) is_schema() {}

/////////////////////////////////////////////////////////////////////
/////// RULE BLOCK: FLOAT
/////////////////////////////////////////////////////////////////////

// Float describes rules for a float-kinded value or field.
type Float struct {
	MustNotBeNil  bool
	MustNotBeZero bool
	DefaultIfNil  any
	DefaultIfZero any

	Min          any
	Max          any
	MustBeIn     []float64
	MustNotBeIn  []float64
	ValidateFunc func(float64) error
	SkipFunc     func(float64) bool
}

func (Float) is_schema() {}

/////////////////////////////////////////////////////////////////////
/////// RULE BLOCK: BOOL
/////////////////////////////////////////////////////////////////////

// Bool describes rules for a bool-kinded value or field. Bool does
// not use zero vocabulary; use MustBeTrue or MustBeFalse for value
// constraints.
type Bool struct {
	MustNotBeNil bool
	DefaultIfNil any

	MustBeTrue   bool
	MustBeFalse  bool
	ValidateFunc func(bool) error
	SkipFunc     func(bool) bool
}

func (Bool) is_schema() {}

/////////////////////////////////////////////////////////////////////
/////// RULE BLOCK: SLICE / LIST
/////////////////////////////////////////////////////////////////////

// Slice describes rules for a slice- or array-kinded value or field.
//
// If ElementSchema is non-nil, it is applied to every element. Any
// Schema() method declared by the element type is also applied, via
// recursive discovery during the outer walk.
//
// MinLen and MaxLen are value-level constraints and fire on a
// non-nil empty slice. Slice does not use zero vocabulary; emptiness
// is handled by MinLen.
type Slice struct {
	MustNotBeNil bool
	DefaultIfNil any

	ElementSchema Schema
	MinLen        any
	MaxLen        any
	ValidateFunc  func(int) error
	SkipFunc      func(int) bool
}

func (Slice) is_schema() {}

// List is an alias for Slice.
type List = Slice

/////////////////////////////////////////////////////////////////////
/////// RULE BLOCK: MAP
/////////////////////////////////////////////////////////////////////

// Map describes rules for a map-kinded value or field.
//
// If KeySchema is non-nil, it is applied to every key. If
// ValueSchema is non-nil, it is applied to every value. Both run
// before any Schema() method declared by the key or value type.
//
// MinLen and MaxLen are value-level constraints and fire on a
// non-nil empty map. Map does not use zero vocabulary; emptiness is
// handled by MinLen.
type Map struct {
	MustNotBeNil bool
	DefaultIfNil any

	KeySchema    Schema
	ValueSchema  Schema
	MinLen       any
	MaxLen       any
	ValidateFunc func(int) error
	SkipFunc     func(int) bool
}

func (Map) is_schema() {}

/////////////////////////////////////////////////////////////////////
/////// RULE BLOCK: ANY
/////////////////////////////////////////////////////////////////////

// Any is an escape-hatch rule block for values that do not fit the
// typed blocks above. It supports nil checks, nil defaults, SkipFunc,
// and ValidateFunc.
type Any struct {
	MustNotBeNil bool
	DefaultIfNil any

	ValidateFunc func(any) error
	SkipFunc     func(any) bool
}

func (Any) is_schema() {}

/////////////////////////////////////////////////////////////////////
/////// ENFORCE ENTRY POINTS
/////////////////////////////////////////////////////////////////////

// Enforce runs any schema declared by value and returns the final
// transformed value wrapped in Result.
func Enforce[T any](label string, value T, root ...Schema) (Result[T], error) {
	res, err := EnforceAny(label, value, root...)
	if err != nil {
		if out, ok := res.Value.(T); ok {
			return Result[T]{Value: out}, err
		}
		return Result[T]{}, err
	}
	out, ok := res.Value.(T)
	if !ok {
		return Result[T]{}, &SchemaError{Err: fmt.Errorf(
			"internal: enforce returned a value of type %T, expected %T",
			res.Value, *new(T),
		)}
	}
	return Result[T]{Value: out}, nil
}

// EnforceAny is the type-erased form of Enforce. If root is
// supplied, it is applied at the root in addition to any discovered
// Schema() method on the root value. The discovered schema runs
// first, then the explicit root.
func EnforceAny(label string, value any, root ...Schema) (ResultAny, error) {
	if value == nil {
		return ResultAny{Value: nil}, nil
	}
	var explicit_root Schema
	if len(root) > 0 {
		explicit_root = root[0]
	}
	errs := &classified_errors{}

	rv := reflect.ValueOf(value)
	// For pointer and map inputs, walk directly: pointees are
	// addressable through the pointer, and maps are reference types.
	//
	// For value inputs, reflect.ValueOf produces a non-addressable
	// value, so mutations would be silently dropped. Copy into an
	// addressable holder, walk the holder, and return the holder's
	// transformed contents as the result.
	if rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Map {
		walk_root(label, rv, explicit_root, errs)
		if err := build_error(errs); err != nil {
			return ResultAny{Value: value}, err
		}
		return ResultAny{Value: value}, nil
	}
	holder := reflectutil.Value{V: rv}.SettableCopy()
	walk_root(label, holder, explicit_root, errs)
	out := holder.Interface()
	if err := build_error(errs); err != nil {
		return ResultAny{Value: out}, err
	}
	return ResultAny{Value: out}, nil
}

type classified_errors struct {
	schema_errs     []error
	validation_errs []error
}

func (ce *classified_errors) add_validation(err error) {
	if err == nil {
		return
	}
	ce.validation_errs = append(ce.validation_errs, err)
}

func (ce *classified_errors) add_validation_f(format string, args ...any) {
	ce.add_validation(fmt.Errorf(format, args...))
}

func (ce *classified_errors) add_schema(err error) {
	if err == nil {
		return
	}
	ce.schema_errs = append(ce.schema_errs, err)
}

func (ce *classified_errors) add_schema_f(format string, args ...any) {
	ce.add_schema(fmt.Errorf(format, args...))
}

func (ce *classified_errors) empty() bool {
	return len(ce.schema_errs) == 0 && len(ce.validation_errs) == 0
}

func build_error(ce *classified_errors) error {
	if ce == nil || ce.empty() {
		return nil
	}
	if len(ce.schema_errs) > 0 {
		return &SchemaError{Err: errors.Join(ce.schema_errs...)}
	}
	return &ValidationError{Err: errors.Join(ce.validation_errs...)}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: CORE WALK (MUTATE-WHEN-POSSIBLE)
/////////////////////////////////////////////////////////////////////

// walk_root applies discovery plus any explicit root schema, then
// recurses into the value's structure.
func walk_root(
	label string,
	v reflect.Value,
	explicit_root Schema,
	ce *classified_errors,
) {
	if !v.IsValid() {
		return
	}
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) &&
		v.IsNil() {
		return
	}
	apply_schema_via_discovery(label, v, explicit_root, ce)
	recurse_into_structure(label, v, ce)
}

// walk_value applies any discovered schema on v, then recurses into
// its structure.
func walk_value(
	label string,
	v reflect.Value,
	ce *classified_errors,
) {
	if !v.IsValid() {
		return
	}
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) &&
		v.IsNil() {
		return
	}
	apply_schema_via_discovery(label, v, nil, ce)
	recurse_into_structure(label, v, ce)
}

// apply_schema_via_discovery walks through pointer and interface
// layers looking for a type that implements Schematic. If found,
// applies the schema to that layer's addressable target, and then
// applies explicit_root (if non-nil). If no schematic type is found,
// explicit_root is applied to the fully dereferenced concrete value.
func apply_schema_via_discovery(
	label string,
	v reflect.Value,
	explicit_root Schema,
	ce *classified_errors,
) {
	const max_depth = 8
	cur := v
	interface_slot := reflect.Value{}
	for range max_depth {
		if !cur.IsValid() {
			return
		}
		if impl, ok := schematic_impl(cur); ok {
			sch := impl.Interface().(Schematic).Schema()
			target := schematic_target(impl)
			if interface_slot.IsValid() && !target.CanSet() {
				holder := reflect.New(target.Type()).Elem()
				holder.Set(target)
				target = holder
			}
			apply_schema(label, sch, target, ce)
			if interface_slot.IsValid() && interface_slot.CanSet() &&
				target.IsValid() && target.Type().AssignableTo(interface_slot.Type()) {
				interface_slot.Set(target)
			}
			if explicit_root != nil {
				apply_schema(label, explicit_root, target, ce)
			}
			return
		}
		switch cur.Kind() {
		case reflect.Pointer:
			if cur.IsNil() {
				return
			}
			cur = cur.Elem()
		case reflect.Interface:
			if cur.IsNil() {
				return
			}
			if cur.CanSet() {
				interface_slot = cur
			}
			cur = cur.Elem()
		default:
			if explicit_root != nil {
				apply_schema(label, explicit_root, cur, ce)
			}
			return
		}
	}
}

func schematic_impl(v reflect.Value) (reflect.Value, bool) {
	return reflectutil.Value{V: v}.InterfaceImpl(schematic_type)
}

var schematic_type = reflect.TypeOf((*Schematic)(nil)).Elem()

func schematic_target(impl reflect.Value) reflect.Value {
	if impl.Kind() == reflect.Pointer {
		return impl.Elem()
	}
	return impl
}

func recurse_into_structure(
	label string,
	v reflect.Value,
	ce *classified_errors,
) {
	base := v
	for base.Kind() == reflect.Pointer || base.Kind() == reflect.Interface {
		if base.IsNil() {
			return
		}
		base = base.Elem()
	}
	switch base.Kind() {
	case reflect.Struct:
		fields, err := reflectutil.PublicStructFields(base.Type())
		if err != nil {
			ce.add_schema(err)
			return
		}
		for _, field := range fields {
			fv, ok := field.SettableValue(base)
			if !ok {
				continue
			}
			child_label := fmt.Sprintf("%s.%s", label, field.PublicName)
			walk_value(child_label, fv, ce)
		}
	case reflect.Slice, reflect.Array:
		if base.Kind() == reflect.Slice && base.IsNil() {
			return
		}
		for i := 0; i < base.Len(); i++ {
			elem_label := fmt.Sprintf("%s[%d]", label, i)
			walk_value(elem_label, base.Index(i), ce)
		}
	case reflect.Map:
		if base.IsNil() {
			return
		}
		iter := base.MapRange()
		for iter.Next() {
			key := iter.Key()
			key_str := reflectutil.Value{V: key}.FormatKey()
			map_label := fmt.Sprintf("%s[%s]", label, key_str)
			walk_value(map_label+"(key)", key, ce)
			holder, _ := reflectutil.Value{V: base}.MapValueCopy(key)
			walk_value(map_label+"(value)", holder, ce)
			reflectutil.Value{V: base}.SetMapValue(key, holder)
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: POINTER PRE-STEP
/////////////////////////////////////////////////////////////////////

// resolve_nilable handles the pointer/interface/slice/map pre-step
// that every leaf rule depends on. It returns the concrete,
// non-nilable target the leaf rule should operate on, plus a boolean
// indicating whether leaf processing should continue.
//
// On a nil slot:
//   - If default_if_nil is non-nil and the slot is addressable,
//     the slot is materialized with the default and the leaf rule
//     runs on the materialized target.
//   - Otherwise, if must_not_be_nil is true, a validation error is
//     recorded and (false) is returned.
//   - Otherwise, (false) is returned with no error.
//
// On a non-nil slot, the inner value is returned and processing
// continues.
func resolve_nilable(
	label string,
	val reflect.Value,
	must_not_be_nil bool,
	default_if_nil any,
	ce *classified_errors,
) (reflect.Value, bool) {
	value := reflectutil.Value{V: val}
	if value.PointsToNil() {
		if default_if_nil != nil {
			rv := reflect.ValueOf(default_if_nil)
			if rv.IsValid() {
				switch {
				case val.Kind() == reflect.Interface && val.CanSet() &&
					rv.Type().AssignableTo(val.Type()):
					val.Set(rv)
					return val, true
				case val.Kind() == reflect.Pointer &&
					val.CanSet() &&
					val.Type().Elem().Kind() == reflect.Interface:
					val.Set(reflect.New(val.Type().Elem()))
					elem := val.Elem()
					if rv.Type().AssignableTo(elem.Type()) {
						elem.Set(rv)
						return elem, true
					}
					val.Set(reflect.Zero(val.Type()))
				}
			}
			base, ok := value.EnsureDeref()
			if !ok {
				if must_not_be_nil {
					ce.add_validation_f("%s must not be nil", label)
				}
				return reflect.Value{}, false
			}
			if err := assign_default(base, default_if_nil); err != nil {
				ce.add_schema_f("%s: DefaultIfNil: %s", label, err.Error())
				return reflect.Value{}, false
			}
			return base, true
		}

		if must_not_be_nil {
			ce.add_validation_f("%s must not be nil", label)
		}
		return reflect.Value{}, false
	}

	base := value.Deref()
	if base.IsValid() &&
		(base.Kind() == reflect.Slice || base.Kind() == reflect.Map) &&
		base.IsNil() {
		if default_if_nil != nil {
			if err := assign_default(base, default_if_nil); err != nil {
				ce.add_schema_f("%s: DefaultIfNil: %s", label, err.Error())
				return reflect.Value{}, false
			}
			return val, true
		}

		if must_not_be_nil {
			ce.add_validation_f("%s must not be nil", label)
		}
		return reflect.Value{}, false
	}

	// Already addressable-through-pointer or a direct concrete value.
	// Leaf rules dereference further as needed.
	return val, true
}

// assign_default sets base to a value derived from raw. Primitive
// leaves use type-appropriate coercion; composite leaves require an
// assignable value. This intentionally avoids arbitrary conversions
// like int -> string.
func assign_default(base reflect.Value, raw any) error {
	if !base.CanSet() {
		return fmt.Errorf("target is not settable")
	}
	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return fmt.Errorf("default value is nil")
	}
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return fmt.Errorf("default value is nil")
		}
		rv = rv.Elem()
	}
	raw = rv.Interface()

	switch {
	case base.Kind() == reflect.String:
		s, ok, err := string_value(raw)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("default value is nil")
		}
		base.SetString(s)
		return nil
	case reflectutil.IsSignedIntKind(base.Kind()):
		n, ok, err := int_value(raw)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("default value is nil")
		}
		base.SetInt(n)
		return nil
	case reflectutil.IsUnsignedIntKind(base.Kind()):
		n, ok, err := uint_value(raw)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("default value is nil")
		}
		base.SetUint(n)
		return nil
	case reflectutil.IsFloatKind(base.Kind()):
		f, ok, err := float_value(raw)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("default value is nil")
		}
		base.SetFloat(f)
		return nil
	case base.Kind() == reflect.Bool:
		if rv.Kind() != reflect.Bool {
			return fmt.Errorf("expected bool value, got %T", raw)
		}
		base.SetBool(rv.Bool())
		return nil
	}

	if rv.Type().AssignableTo(base.Type()) {
		base.Set(rv)
		return nil
	}
	return fmt.Errorf("default value of type %T is not assignable to %s", raw, base.Type())
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: SCHEMA APPLICATION
/////////////////////////////////////////////////////////////////////

// apply_schema applies sch to target (addressable, produced upstream).
func apply_schema(
	label string,
	sch Schema,
	target reflect.Value,
	ce *classified_errors,
) {
	if sch == nil {
		return
	}
	if obj, ok := sch.(Object); ok {
		apply_object(label, obj, target, ce)
		return
	}
	apply_rule(label, sch, target, ce)
}

func apply_object(
	label string,
	obj Object,
	target reflect.Value,
	ce *classified_errors,
) {
	base := reflectutil.Value{V: target}.Deref()
	if !base.IsValid() {
		return
	}
	kind := base.Kind()
	if kind != reflect.Struct && kind != reflect.Map {
		ce.add_schema_f(
			"%s: Object can only be applied to structs or string-keyed maps, got %s",
			label, kind,
		)
		return
	}
	if kind == reflect.Map && base.Type().Key().Kind() != reflect.String {
		ce.add_schema_f(
			"%s: Object on a map requires string keys, got %s",
			label, base.Type().Key().Kind(),
		)
		return
	}

	// Partition keys into field-rule keys, sorting for deterministic
	// error order.
	field_names := make([]string, 0, len(obj))
	for k := range obj {
		if !is_reserved_key(k) {
			field_names = append(field_names, k)
		}
	}
	sort.Strings(field_names)

	// (1) Field rules.
	for _, name := range field_names {
		raw_spec := obj[name]
		if raw_spec == nil {
			continue
		}
		spec, ok := raw_spec.(Schema)
		if !ok {
			ce.add_schema_f(
				"%s.%s: expected a schema rule, got %T",
				label, name, raw_spec,
			)
			continue
		}
		field_label := fmt.Sprintf("%s.%s", label, name)
		fv, exists, write_back := reflectutil.Value{V: base}.ResolveField(name)
		if !exists && kind == reflect.Struct {
			ce.add_schema_f(
				"%s: schema references unknown field %q on %s",
				label, name, base.Type(),
			)
			continue
		}
		apply_rule(field_label, spec, fv, ce)
		if write_back != nil && (exists || !fv.IsZero()) {
			write_back(fv)
		}
	}

	// (2) TransformFunc.
	if raw_transform, ok := obj[TransformFunc]; ok {
		base = run_object_transform(label, raw_transform, base, ce)
	}

	// (3) ValidateFunc.
	if raw_validate, ok := obj[ValidateFunc]; ok {
		run_object_validate(label, raw_validate, base, ce)
	}
}

func run_object_transform(
	label string,
	raw any,
	base reflect.Value,
	ce *classified_errors,
) reflect.Value {
	fn := reflect.ValueOf(raw)
	if !fn.IsValid() {
		ce.add_schema_f("%s: object TransformFunc is nil", label)
		return base
	}
	fn_type := fn.Type()
	if fn.Kind() != reflect.Func ||
		fn_type.NumIn() != 1 ||
		fn_type.NumOut() != 2 {
		ce.add_schema_f(
			"%s: object TransformFunc must be func(T) (T, error), got %T",
			label, raw,
		)
		return base
	}
	if !fn_type.Out(1).Implements(error_type) {
		ce.add_schema_f(
			"%s: object TransformFunc second return must be error, got %s",
			label, fn_type.Out(1),
		)
		return base
	}
	arg, ok := object_callback_arg(base, fn_type.In(0))
	if !ok {
		ce.add_schema_f(
			"%s: object TransformFunc expects %s, got %s",
			label, fn_type.In(0), base.Type(),
		)
		return base
	}
	out := fn.Call([]reflect.Value{arg})
	if !out[1].IsNil() {
		err := out[1].Interface().(error)
		ce.add_validation_f("%s: %s", label, err.Error())
		return base
	}
	if !out[0].Type().AssignableTo(base.Type()) {
		ce.add_schema_f(
			"%s: object TransformFunc returned %s, expected %s",
			label, out[0].Type(), base.Type(),
		)
		return base
	}
	if base.CanSet() {
		base.Set(out[0])
		return base
	}
	return out[0]
}

func run_object_validate(
	label string,
	raw any,
	base reflect.Value,
	ce *classified_errors,
) {
	fn := reflect.ValueOf(raw)
	if !fn.IsValid() {
		ce.add_schema_f("%s: object ValidateFunc is nil", label)
		return
	}
	fn_type := fn.Type()
	if fn.Kind() != reflect.Func ||
		fn_type.NumIn() != 1 ||
		fn_type.NumOut() != 1 ||
		!fn_type.Out(0).Implements(error_type) {
		ce.add_schema_f(
			"%s: object ValidateFunc must be func(T) error, got %T",
			label, raw,
		)
		return
	}
	arg, ok := object_callback_arg(base, fn_type.In(0))
	if !ok {
		ce.add_schema_f(
			"%s: object ValidateFunc expects %s, got %s",
			label, fn_type.In(0), base.Type(),
		)
		return
	}
	out := fn.Call([]reflect.Value{arg})
	if out[0].IsNil() {
		return
	}
	err := out[0].Interface().(error)
	ce.add_validation_f("%s: %s", label, err.Error())
}

func object_callback_arg(base reflect.Value, want reflect.Type) (reflect.Value, bool) {
	if !base.IsValid() {
		return reflect.Value{}, false
	}
	if base.Type().AssignableTo(want) {
		return base, true
	}
	if base.CanAddr() {
		addr := base.Addr()
		if addr.Type().AssignableTo(want) {
			return addr, true
		}
	}
	if want.Kind() == reflect.Interface && base.Type().Implements(want) {
		return base, true
	}
	if base.CanAddr() && want.Kind() == reflect.Interface &&
		base.Addr().Type().Implements(want) {
		return base.Addr(), true
	}
	return reflect.Value{}, false
}

var error_type = reflect.TypeOf((*error)(nil)).Elem()

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: RULE DISPATCH
/////////////////////////////////////////////////////////////////////

func apply_rule(
	label string,
	spec Schema,
	val reflect.Value,
	ce *classified_errors,
) {
	switch s := spec.(type) {
	case String:
		apply_string(label, s, val, ce)
	case *String:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.String rule", label)
			return
		}
		apply_string(label, *s, val, ce)
	case Int:
		apply_int(label, s, val, ce)
	case *Int:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.Int rule", label)
			return
		}
		apply_int(label, *s, val, ce)
	case Uint:
		apply_uint(label, s, val, ce)
	case *Uint:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.Uint rule", label)
			return
		}
		apply_uint(label, *s, val, ce)
	case Float:
		apply_float(label, s, val, ce)
	case *Float:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.Float rule", label)
			return
		}
		apply_float(label, *s, val, ce)
	case Bool:
		apply_bool(label, s, val, ce)
	case *Bool:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.Bool rule", label)
			return
		}
		apply_bool(label, *s, val, ce)
	case Slice:
		apply_slice(label, s, val, ce)
	case *Slice:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.Slice rule", label)
			return
		}
		apply_slice(label, *s, val, ce)
	case Map:
		apply_map(label, s, val, ce)
	case *Map:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.Map rule", label)
			return
		}
		apply_map(label, *s, val, ce)
	case Any:
		apply_any(label, s, val, ce)
	case *Any:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.Any rule", label)
			return
		}
		apply_any(label, *s, val, ce)
	case Object:
		apply_object(label, s, val, ce)
	case *Object:
		if s == nil {
			ce.add_schema_f("%s: nil *schema.Object rule", label)
			return
		}
		apply_object(label, *s, val, ce)
	default:
		ce.add_schema_f("%s: unknown rule type %T", label, spec)
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: DEFAULT / LIMIT COERCION
/////////////////////////////////////////////////////////////////////

func int_value(raw any) (int64, bool, error) {
	if raw == nil {
		return 0, false, nil
	}
	rv := reflect.ValueOf(raw)
	if reflectutil.IsSignedIntKind(rv.Kind()) {
		return rv.Int(), true, nil
	}
	return 0, false, fmt.Errorf("expected int-like value, got %T", raw)
}

func uint_value(raw any) (uint64, bool, error) {
	if raw == nil {
		return 0, false, nil
	}
	rv := reflect.ValueOf(raw)
	switch {
	case reflectutil.IsSignedIntKind(rv.Kind()):
		n := rv.Int()
		if n < 0 {
			return 0, false, fmt.Errorf("expected non-negative uint-like value, got %d", n)
		}
		return uint64(n), true, nil
	case reflectutil.IsUnsignedIntKind(rv.Kind()):
		return rv.Uint(), true, nil
	}
	return 0, false, fmt.Errorf("expected uint-like value, got %T", raw)
}

func float_value(raw any) (float64, bool, error) {
	if raw == nil {
		return 0, false, nil
	}
	rv := reflect.ValueOf(raw)
	switch {
	case reflectutil.IsFloatKind(rv.Kind()):
		return rv.Float(), true, nil
	case reflectutil.IsSignedIntKind(rv.Kind()):
		return float64(rv.Int()), true, nil
	case reflectutil.IsUnsignedIntKind(rv.Kind()):
		return float64(rv.Uint()), true, nil
	}
	return 0, false, fmt.Errorf("expected float-like value, got %T", raw)
}

func string_value(raw any) (string, bool, error) {
	if raw == nil {
		return "", false, nil
	}
	rv := reflect.ValueOf(raw)
	if rv.Kind() != reflect.String {
		return "", false, fmt.Errorf("expected string value, got %T", raw)
	}
	return rv.String(), true, nil
}

func len_limit(raw any, field_name string) (int, bool, error) {
	if raw == nil {
		return 0, false, nil
	}
	n, ok, err := int_value(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s: %s", field_name, err.Error())
	}
	if !ok {
		return 0, false, nil
	}
	if n < 0 {
		return 0, false, fmt.Errorf("%s: must be non-negative, got %d", field_name, n)
	}
	return int(n), true, nil
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: STRING
/////////////////////////////////////////////////////////////////////

func apply_string(
	label string,
	s String,
	val reflect.Value,
	ce *classified_errors,
) {
	default_if_zero, _, err := string_value(s.DefaultIfZero)
	if err != nil {
		ce.add_schema_f("%s: DefaultIfZero: %s", label, err.Error())
		return
	}

	resolved, cont := resolve_nilable(label, val, s.MustNotBeNil, s.DefaultIfNil, ce)
	if !cont {
		return
	}
	base, ok := reflectutil.Value{V: resolved}.StringBase()
	if !ok {
		ce.add_schema_f(
			"%s: String rule applied to non-string-kinded value (%s)",
			label, resolved.Kind(),
		)
		return
	}

	str := base.String()

	if s.SkipFunc != nil && s.SkipFunc(str) {
		return
	}

	// Normalize.
	if s.TrimSpace {
		str = strings.TrimSpace(str)
	}
	switch {
	case s.ToLower:
		str = strings.ToLower(str)
	case s.ToUpper:
		str = strings.ToUpper(str)
	}
	if s.TransformFunc != nil {
		out, err := s.TransformFunc(str)
		if err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
			return
		}
		str = out
	}
	if str == "" && default_if_zero != "" {
		str = default_if_zero
	}

	reflectutil.Value{V: resolved}.WriteString(base, str)

	if str == "" && s.MustNotBeZero {
		ce.add_validation_f("%s must not be empty", label)
		return
	}

	apply_string_checks(label, str, s, ce)

	if s.ValidateFunc != nil {
		if err := s.ValidateFunc(str); err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
		}
	}
}

func apply_string_checks(
	label string,
	str string,
	s String,
	ce *classified_errors,
) {
	min_len, _, err := len_limit(s.MinLen, "MinLen")
	if err != nil {
		ce.add_schema_f("%s: %s", label, err.Error())
		return
	}
	max_len, _, err := len_limit(s.MaxLen, "MaxLen")
	if err != nil {
		ce.add_schema_f("%s: %s", label, err.Error())
		return
	}
	if min_len > 0 && max_len > 0 && min_len > max_len {
		ce.add_schema_f(
			"%s: MinLen (%d) cannot be greater than MaxLen (%d)",
			label, min_len, max_len,
		)
		return
	}
	rune_len := len([]rune(str))
	if min_len > 0 && rune_len < min_len {
		ce.add_validation_f(
			"%s: minimum length is %d, got %d", label, min_len, rune_len,
		)
	}
	if max_len > 0 && rune_len > max_len {
		ce.add_validation_f(
			"%s: maximum length is %d, got %d", label, max_len, rune_len,
		)
	}
	if len(s.MustBeIn) > 0 && !slices.Contains(s.MustBeIn, str) {
		ce.add_validation_f(
			"%s: value %q is not in the permitted set", label, str,
		)
	}
	if len(s.MustNotBeIn) > 0 && slices.Contains(s.MustNotBeIn, str) {
		ce.add_validation_f(
			"%s: value %q is in the prohibited set", label, str,
		)
	}
	if s.MustBeEmail {
		if _, err := mail.ParseAddress(str); err != nil {
			ce.add_validation_f("%s: must be a valid email address", label)
		}
	}
	if s.MustBeURL {
		if _, err := url.ParseRequestURI(str); err != nil {
			ce.add_validation_f("%s: must be a valid URL", label)
		}
	}
	if s.MustMatch != nil && !s.MustMatch.MatchString(str) {
		ce.add_validation_f("%s: does not match required pattern", label)
	}
	if s.MustStartWith != "" && !strings.HasPrefix(str, s.MustStartWith) {
		ce.add_validation_f("%s: must start with %q", label, s.MustStartWith)
	}
	if s.MustEndWith != "" && !strings.HasSuffix(str, s.MustEndWith) {
		ce.add_validation_f("%s: must end with %q", label, s.MustEndWith)
	}
	if s.AllowedChars != "" {
		allowed := make(map[rune]struct{}, len(s.AllowedChars))
		for _, r := range s.AllowedChars {
			allowed[r] = struct{}{}
		}
		for _, r := range str {
			if _, ok := allowed[r]; !ok {
				ce.add_validation_f(
					"%s: contains disallowed character %q", label, r,
				)
				break
			}
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: INT
/////////////////////////////////////////////////////////////////////

func apply_int(
	label string,
	s Int,
	val reflect.Value,
	ce *classified_errors,
) {
	default_if_zero, _, err := int_value(s.DefaultIfZero)
	if err != nil {
		ce.add_schema_f("%s: DefaultIfZero: %s", label, err.Error())
		return
	}
	min, has_min, err := int_value(s.Min)
	if err != nil {
		ce.add_schema_f("%s: Min: %s", label, err.Error())
		return
	}
	max, has_max, err := int_value(s.Max)
	if err != nil {
		ce.add_schema_f("%s: Max: %s", label, err.Error())
		return
	}
	if has_min && has_max && min > max {
		ce.add_schema_f(
			"%s: Min (%d) cannot be greater than Max (%d)",
			label, min, max,
		)
		return
	}

	resolved, cont := resolve_nilable(label, val, s.MustNotBeNil, s.DefaultIfNil, ce)
	if !cont {
		return
	}
	base, ok := reflectutil.Value{V: resolved}.SignedIntBase()
	if !ok {
		ce.add_schema_f(
			"%s: Int rule applied to non-signed-integer-kinded value (%s)",
			label, resolved.Kind(),
		)
		return
	}

	n := base.Int()

	if s.SkipFunc != nil && s.SkipFunc(int(n)) {
		return
	}

	if n == 0 && default_if_zero != 0 {
		n = default_if_zero
	}
	reflectutil.Value{V: resolved}.WriteInt(base, n)

	if n == 0 && s.MustNotBeZero {
		ce.add_validation_f("%s must not be zero", label)
		return
	}

	if has_min && n < min {
		ce.add_validation_f("%s: minimum is %d, got %d", label, min, n)
	}
	if has_max && n > max {
		ce.add_validation_f("%s: maximum is %d, got %d", label, max, n)
	}
	if len(s.MustBeIn) > 0 {
		in := make([]int64, len(s.MustBeIn))
		for i, v := range s.MustBeIn {
			in[i] = int64(v)
		}
		if !slices.Contains(in, n) {
			ce.add_validation_f(
				"%s: value %d is not in the permitted set", label, n,
			)
		}
	}
	if len(s.MustNotBeIn) > 0 {
		not_in := make([]int64, len(s.MustNotBeIn))
		for i, v := range s.MustNotBeIn {
			not_in[i] = int64(v)
		}
		if slices.Contains(not_in, n) {
			ce.add_validation_f(
				"%s: value %d is in the prohibited set", label, n,
			)
		}
	}
	if s.ValidateFunc != nil {
		if err := s.ValidateFunc(int(n)); err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: UINT
/////////////////////////////////////////////////////////////////////

func apply_uint(
	label string,
	s Uint,
	val reflect.Value,
	ce *classified_errors,
) {
	default_if_zero, _, err := uint_value(s.DefaultIfZero)
	if err != nil {
		ce.add_schema_f("%s: DefaultIfZero: %s", label, err.Error())
		return
	}
	min, has_min, err := uint_value(s.Min)
	if err != nil {
		ce.add_schema_f("%s: Min: %s", label, err.Error())
		return
	}
	max, has_max, err := uint_value(s.Max)
	if err != nil {
		ce.add_schema_f("%s: Max: %s", label, err.Error())
		return
	}
	if has_min && has_max && min > max {
		ce.add_schema_f(
			"%s: Min (%d) cannot be greater than Max (%d)",
			label, min, max,
		)
		return
	}

	resolved, cont := resolve_nilable(label, val, s.MustNotBeNil, s.DefaultIfNil, ce)
	if !cont {
		return
	}
	base, ok := reflectutil.Value{V: resolved}.UnsignedIntBase()
	if !ok {
		ce.add_schema_f(
			"%s: Uint rule applied to non-unsigned-integer-kinded value (%s)",
			label, resolved.Kind(),
		)
		return
	}

	n := base.Uint()

	if s.SkipFunc != nil && s.SkipFunc(uint(n)) {
		return
	}

	if n == 0 && default_if_zero != 0 {
		n = default_if_zero
	}
	reflectutil.Value{V: resolved}.WriteUint(base, n)

	if n == 0 && s.MustNotBeZero {
		ce.add_validation_f("%s must not be zero", label)
		return
	}

	if has_min && n < min {
		ce.add_validation_f("%s: minimum is %d, got %d", label, min, n)
	}
	if has_max && n > max {
		ce.add_validation_f("%s: maximum is %d, got %d", label, max, n)
	}
	if len(s.MustBeIn) > 0 {
		in := make([]uint64, len(s.MustBeIn))
		for i, v := range s.MustBeIn {
			in[i] = uint64(v)
		}
		if !slices.Contains(in, n) {
			ce.add_validation_f(
				"%s: value %d is not in the permitted set", label, n,
			)
		}
	}
	if len(s.MustNotBeIn) > 0 {
		not_in := make([]uint64, len(s.MustNotBeIn))
		for i, v := range s.MustNotBeIn {
			not_in[i] = uint64(v)
		}
		if slices.Contains(not_in, n) {
			ce.add_validation_f(
				"%s: value %d is in the prohibited set", label, n,
			)
		}
	}
	if s.ValidateFunc != nil {
		if err := s.ValidateFunc(uint(n)); err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: FLOAT
/////////////////////////////////////////////////////////////////////

func apply_float(
	label string,
	s Float,
	val reflect.Value,
	ce *classified_errors,
) {
	default_if_zero, _, err := float_value(s.DefaultIfZero)
	if err != nil {
		ce.add_schema_f("%s: DefaultIfZero: %s", label, err.Error())
		return
	}
	min, has_min, err := float_value(s.Min)
	if err != nil {
		ce.add_schema_f("%s: Min: %s", label, err.Error())
		return
	}
	max, has_max, err := float_value(s.Max)
	if err != nil {
		ce.add_schema_f("%s: Max: %s", label, err.Error())
		return
	}
	if has_min && has_max && min > max {
		ce.add_schema_f(
			"%s: Min (%v) cannot be greater than Max (%v)",
			label, min, max,
		)
		return
	}

	resolved, cont := resolve_nilable(label, val, s.MustNotBeNil, s.DefaultIfNil, ce)
	if !cont {
		return
	}
	base, ok := reflectutil.Value{V: resolved}.FloatBase()
	if !ok {
		ce.add_schema_f(
			"%s: Float rule applied to non-float-kinded value (%s)",
			label, resolved.Kind(),
		)
		return
	}

	f := base.Float()

	if s.SkipFunc != nil && s.SkipFunc(f) {
		return
	}

	if f == 0 && default_if_zero != 0 {
		f = default_if_zero
	}
	reflectutil.Value{V: resolved}.WriteFloat(base, f)

	if f == 0 && s.MustNotBeZero {
		ce.add_validation_f("%s must not be zero", label)
		return
	}

	if has_min && f < min {
		ce.add_validation_f("%s: minimum is %v, got %v", label, min, f)
	}
	if has_max && f > max {
		ce.add_validation_f("%s: maximum is %v, got %v", label, max, f)
	}
	if len(s.MustBeIn) > 0 && !slices.Contains(s.MustBeIn, f) {
		ce.add_validation_f(
			"%s: value %v is not in the permitted set", label, f,
		)
	}
	if len(s.MustNotBeIn) > 0 && slices.Contains(s.MustNotBeIn, f) {
		ce.add_validation_f(
			"%s: value %v is in the prohibited set", label, f,
		)
	}
	if s.ValidateFunc != nil {
		if err := s.ValidateFunc(f); err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: BOOL
/////////////////////////////////////////////////////////////////////

func apply_bool(
	label string,
	s Bool,
	val reflect.Value,
	ce *classified_errors,
) {
	resolved, cont := resolve_nilable(label, val, s.MustNotBeNil, s.DefaultIfNil, ce)
	if !cont {
		return
	}
	base, ok := reflectutil.Value{V: resolved}.BoolBase()
	if !ok {
		ce.add_schema_f(
			"%s: Bool rule applied to non-bool-kinded value (%s)",
			label, resolved.Kind(),
		)
		return
	}

	b := base.Bool()

	if s.SkipFunc != nil && s.SkipFunc(b) {
		return
	}

	reflectutil.Value{V: resolved}.WriteBool(base, b)

	if s.MustBeTrue && s.MustBeFalse {
		ce.add_schema_f("%s: both MustBeTrue and MustBeFalse are set", label)
		return
	}
	if s.MustBeTrue && !b {
		ce.add_validation_f("%s: must be true", label)
	}
	if s.MustBeFalse && b {
		ce.add_validation_f("%s: must be false", label)
	}
	if s.ValidateFunc != nil {
		if err := s.ValidateFunc(b); err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: SLICE
/////////////////////////////////////////////////////////////////////

func apply_slice(
	label string,
	s Slice,
	val reflect.Value,
	ce *classified_errors,
) {
	resolved, cont := resolve_nilable(label, val, s.MustNotBeNil, s.DefaultIfNil, ce)
	if !cont {
		return
	}
	base, ok := reflectutil.Value{V: resolved}.SliceOrArrayBase()
	if !ok {
		ce.add_schema_f(
			"%s: Slice rule applied to non-slice-kinded value (%s)",
			label, resolved.Kind(),
		)
		return
	}
	n := base.Len()
	if s.SkipFunc != nil && s.SkipFunc(n) {
		return
	}
	min_len, _, err := len_limit(s.MinLen, "MinLen")
	if err != nil {
		ce.add_schema_f("%s: %s", label, err.Error())
		return
	}
	max_len, _, err := len_limit(s.MaxLen, "MaxLen")
	if err != nil {
		ce.add_schema_f("%s: %s", label, err.Error())
		return
	}
	if min_len > 0 && max_len > 0 && min_len > max_len {
		ce.add_schema_f(
			"%s: MinLen (%d) cannot be greater than MaxLen (%d)",
			label, min_len, max_len,
		)
		return
	}
	if min_len > 0 && n < min_len {
		ce.add_validation_f(
			"%s: minimum length is %d, got %d", label, min_len, n,
		)
	}
	if max_len > 0 && n > max_len {
		ce.add_validation_f(
			"%s: maximum length is %d, got %d", label, max_len, n,
		)
	}
	if s.ElementSchema != nil {
		for i := 0; i < n; i++ {
			elem_label := fmt.Sprintf("%s[%d]", label, i)
			apply_rule(elem_label, s.ElementSchema, base.Index(i), ce)
		}
	}
	if s.ValidateFunc != nil {
		if err := s.ValidateFunc(n); err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: MAP
/////////////////////////////////////////////////////////////////////

func apply_map(
	label string,
	s Map,
	val reflect.Value,
	ce *classified_errors,
) {
	resolved, cont := resolve_nilable(label, val, s.MustNotBeNil, s.DefaultIfNil, ce)
	if !cont {
		return
	}
	base, ok := reflectutil.Value{V: resolved}.MapBase()
	if !ok {
		ce.add_schema_f(
			"%s: Map rule applied to non-map-kinded value (%s)",
			label, resolved.Kind(),
		)
		return
	}
	n := base.Len()
	if s.SkipFunc != nil && s.SkipFunc(n) {
		return
	}
	min_len, _, err := len_limit(s.MinLen, "MinLen")
	if err != nil {
		ce.add_schema_f("%s: %s", label, err.Error())
		return
	}
	max_len, _, err := len_limit(s.MaxLen, "MaxLen")
	if err != nil {
		ce.add_schema_f("%s: %s", label, err.Error())
		return
	}
	if min_len > 0 && max_len > 0 && min_len > max_len {
		ce.add_schema_f(
			"%s: MinLen (%d) cannot be greater than MaxLen (%d)",
			label, min_len, max_len,
		)
		return
	}
	if min_len > 0 && n < min_len {
		ce.add_validation_f(
			"%s: minimum length is %d, got %d", label, min_len, n,
		)
	}
	if max_len > 0 && n > max_len {
		ce.add_validation_f(
			"%s: maximum length is %d, got %d", label, max_len, n,
		)
	}
	if s.KeySchema != nil || s.ValueSchema != nil {
		keys := reflectutil.Value{V: base}.SortedMapKeys()
		for _, key := range keys {
			key_str := reflectutil.Value{V: key}.FormatKey()
			entry_label := fmt.Sprintf("%s[%s]", label, key_str)
			new_key := key
			if s.KeySchema != nil {
				key_holder := reflectutil.Value{V: key}.SettableCopy()
				apply_rule(entry_label+"(key)", s.KeySchema, key_holder, ce)
				new_key = key_holder
			}
			val_holder, exists := reflectutil.Value{V: base}.MapValueCopy(key)
			if !exists {
				continue
			}
			if s.ValueSchema != nil {
				apply_rule(entry_label+"(value)", s.ValueSchema, val_holder, ce)
			}
			if new_key.Interface() != key.Interface() {
				reflectutil.Value{V: base}.DeleteMapValue(key)
			}
			reflectutil.Value{V: base}.SetMapValue(new_key, val_holder)
		}
	}
	if s.ValidateFunc != nil {
		if err := s.ValidateFunc(n); err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL: ANY
/////////////////////////////////////////////////////////////////////

func apply_any(
	label string,
	s Any,
	val reflect.Value,
	ce *classified_errors,
) {
	resolved, cont := resolve_nilable(label, val, s.MustNotBeNil, s.DefaultIfNil, ce)
	if !cont {
		return
	}
	iface := reflectutil.Value{V: resolved}.Deref().Interface()
	if s.SkipFunc != nil && s.SkipFunc(iface) {
		return
	}
	if s.ValidateFunc != nil {
		if err := s.ValidateFunc(iface); err != nil {
			ce.add_validation_f("%s: %s", label, err.Error())
		}
	}
}
