// Package validate provides validation and parsing for HTTP request data.
package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/set"
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC TYPES
/////////////////////////////////////////////////////////////////////

// Validator is implemented by types that can self-validate.
type Validator interface{ Validate() error }

// ValidationError wraps validation failures.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// IsValidationError reports whether err is or wraps a ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

/////////////////////////////////////////////////////////////////////
/////// ENTRY POINTS: PARSING + VALIDATION
/////////////////////////////////////////////////////////////////////

// JSONBodyInto decodes an HTTP request body into a struct and validates it.
func JSONBodyInto(r *http.Request, dest any) error {
	if r == nil {
		return &ValidationError{Err: errors.New("request is nil")}
	}
	if r.Body == nil {
		return &ValidationError{Err: errors.New("request body is nil")}
	}
	if dest == nil {
		return &ValidationError{Err: errors.New("destination is nil")}
	}
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	return attempt_validation("validate.JSONBodyInto", dest)
}

// JSONBytesInto decodes JSON bytes into a struct and validates it.
func JSONBytesInto(data []byte, dest any) error {
	if dest == nil {
		return &ValidationError{Err: errors.New("destination is nil")}
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	return attempt_validation("validate.JSONBytesInto", dest)
}

// JSONStrInto decodes a JSON string into a struct and validates it.
func JSONStrInto(data string, dest any) error {
	if dest == nil {
		return &ValidationError{Err: errors.New("destination is nil")}
	}
	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	return attempt_validation("validate.JSONStrInto", dest)
}

// URLSearchParamsInto parses URL query parameters into a struct and validates it.
func URLSearchParamsInto(r *http.Request, dest any) error {
	if r == nil {
		return &ValidationError{Err: errors.New("request is nil")}
	}
	if r.URL == nil {
		return &ValidationError{Err: errors.New("request URL is nil")}
	}
	if dest == nil {
		return &ValidationError{Err: errors.New("destination is nil")}
	}
	if err := parse_url_values(r.URL.Query(), dest); err != nil {
		return &ValidationError{
			Err: fmt.Errorf("error parsing URL parameters: %w", err),
		}
	}
	return attempt_validation("validate.URLSearchParamsInto", dest)
}

/////////////////////////////////////////////////////////////////////
/////// ENTRY POINTS: CHECKER API
/////////////////////////////////////////////////////////////////////

// An "object" as defined by this library is either (1) a struct,
// (2) a map with string keys, or (3) a pointer to (1) or (2). If
// you want to add field-level validation rules to an object, use
// this entry point. If you are not validating an object, or you
// just want any embedded fields that implement Validator to be
// validated, you can use the Any function. If the target is an
// object, both Object() and Any() will auto-validate any of the
// object's fields that implement Validator.

// Any creates a checker for any value.
func Any(label string, anything any) *AnyChecker {
	return new_any_checker(label, anything, reflect.ValueOf(anything))
}

// Object creates a checker for a struct or string-keyed map.
func Object(object any) *ObjectChecker {
	oc := &ObjectChecker{}
	if object == nil {
		oc.fail("object cannot be nil")
		return oc
	}
	rv := reflect.ValueOf(object)
	ts := get_type_state(rv)
	if !ts.is_struct_like && !ts.is_map_str_keys {
		oc.fail_f(
			"object must be a struct or a map with string keys (got %T)",
			object,
		)
		return oc
	}
	oc.label = rv.Type().String()
	oc.true_value = object
	oc.reflect_value = rv
	oc.base_reflect_value = safe_deref(rv)
	oc.type_state = ts
	return oc
}

/////////////////////////////////////////////////////////////////////
/////// ANY CHECKER
/////////////////////////////////////////////////////////////////////

// AnyChecker validates a single value.
type AnyChecker struct {
	label              string
	true_value         any
	base_reflect_value reflect.Value
	type_state

	done   bool
	errors []error
}

func new_any_checker(
	label string,
	true_value any,
	rv reflect.Value,
) *AnyChecker {
	return &AnyChecker{
		label:              label,
		true_value:         true_value,
		base_reflect_value: safe_deref(rv),
		type_state:         get_type_state(rv),
	}
}

// Required marks the value as required and triggers validation.
func (c *AnyChecker) Required() *AnyChecker { return c.init_check(true) }

// Optional marks the value as optional and triggers validation only if present.
func (c *AnyChecker) Optional() *AnyChecker { return c.init_check(false) }

// Error returns a ValidationError if any checks failed.
func (c *AnyChecker) Error() error {
	if len(c.errors) > 0 {
		return &ValidationError{Err: errors.Join(c.errors...)}
	}
	return nil
}

func (c *AnyChecker) ok() { c.done = true }

func (c *AnyChecker) fail(msg string) {
	c.done = true
	c.errors = append(c.errors, errors.New(msg))
}

func (c *AnyChecker) fail_f(format string, args ...any) {
	c.fail(fmt.Sprintf(format, args...))
}

func (c *AnyChecker) init_check(required bool) *AnyChecker {
	if c.done {
		return c
	}
	if is_effectively_zero(c.reflect_value) {
		if required {
			c.fail(fmt.Sprintf("%s is required", c.label))
		} else {
			c.ok()
		}
		return c
	}
	if errs := validate_recursive(c.label, c.reflect_value); len(errs) > 0 {
		c.errors = append(c.errors, errs...)
		c.done = true
	}
	return c
}

/////////////////////////////////////////////////////////////////////
/////// OBJECT CHECKER
/////////////////////////////////////////////////////////////////////

// ObjectChecker validates structs and string-keyed maps with field-level checks.
type ObjectChecker struct {
	AnyChecker
	ChildCheckers []*AnyChecker
}

// Required validates that a field is present and valid.
func (oc *ObjectChecker) Required(field string) *AnyChecker {
	return oc.validate_field(field, true)
}

// Optional validates a field only if it is present.
func (oc *ObjectChecker) Optional(field string) *AnyChecker {
	return oc.validate_field(field, false)
}

// Error returns a ValidationError combining object-level and field-level errors.
func (oc *ObjectChecker) Error() error {
	errs := make([]error, 0, len(oc.errors)+len(oc.ChildCheckers))
	errs = append(errs, oc.errors...)
	for _, child := range oc.ChildCheckers {
		if err := child.Error(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &ValidationError{Err: errors.Join(errs...)}
	}
	return nil
}

func (oc *ObjectChecker) validate_field(
	field_name string,
	required bool,
) *AnyChecker {
	if oc.done {
		c := new_any_checker(field_name, nil, reflect.Value{})
		c.done = true
		return c
	}
	if oc.is_struct_like {
		if err := oc.ensure_struct_field(field_name); err != nil {
			c := new_any_checker(field_name, nil, reflect.Value{})
			c.fail(err.Error())
			oc.ChildCheckers = append(oc.ChildCheckers, c)
			return c
		}
	}
	wrapped := oc.get_field_value(field_name)
	c := new_any_checker(field_name, wrapped.true_value, wrapped.reflect_value)
	oc.ChildCheckers = append(oc.ChildCheckers, c)
	if required {
		c.Required()
	} else {
		c.Optional()
	}
	return c
}

func (oc *ObjectChecker) get_field_value(field_name string) *field_wrapper {
	wrapped := &field_wrapper{}
	if oc.is_map_str_keys {
		key := reflect.ValueOf(field_name)
		wrapped.reflect_value = oc.base_reflect_value.MapIndex(key)
		if !wrapped.reflect_value.IsValid() {
			return wrapped
		}
		wrapped.true_value = wrapped.reflect_value.Interface()
		return wrapped
	}
	if oc.is_struct_like {
		wrapped.reflect_value = oc.base_reflect_value.FieldByName(field_name)
		if !wrapped.reflect_value.IsValid() ||
			!wrapped.reflect_value.CanInterface() {
			return wrapped
		}
		wrapped.true_value = wrapped.reflect_value.Interface()
		return wrapped
	}
	panic("this should never happen")
}

func (oc *ObjectChecker) ensure_struct_field(field_name string) error {
	field, ok := oc.base_reflect_value.Type().FieldByName(field_name)
	if !ok {
		return fmt.Errorf(
			"unknown field %s on %s",
			field_name,
			oc.base_reflect_value.Type(),
		)
	}
	if !field.IsExported() {
		return fmt.Errorf(
			"field %s on %s is unexported",
			field_name,
			oc.base_reflect_value.Type(),
		)
	}
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// FIELD GROUP CONSTRAINTS
/////////////////////////////////////////////////////////////////////

// MutuallyExclusive ensures at most one of the named fields is set.
func (oc *ObjectChecker) MutuallyExclusive(
	label string,
	fields ...string,
) *ObjectChecker {
	if oc.done {
		return oc
	}
	return oc.validate_field_group_constraint(
		label,
		fields,
		func(truthy, _ int) string {
			if truthy > 1 {
				return "fields in group %s are mutually exclusive"
			}
			return ""
		},
	)
}

// MutuallyRequired ensures all named fields are set if any is set.
func (oc *ObjectChecker) MutuallyRequired(
	label string,
	fields ...string,
) *ObjectChecker {
	if oc.done {
		return oc
	}
	return oc.validate_field_group_constraint(
		label,
		fields,
		func(truthy, total int) string {
			if truthy > 0 && truthy < total {
				return "all fields in group %s are required when any is provided"
			}
			return ""
		},
	)
}

func (oc *ObjectChecker) validate_field_group_constraint(
	label string,
	fields []string,
	check func(truthy_count, total int) string,
) *ObjectChecker {
	if oc.done {
		return oc
	}
	_, truthy := oc.count_truthy_fields(fields)
	if msg := check(truthy, len(fields)); msg != "" {
		oc.errors = append(oc.errors, fmt.Errorf(msg, label))
	}
	return oc
}

func (oc *ObjectChecker) count_truthy_fields(
	field_names []string,
) (set.Set[string], int) {
	truthy_set := set.Set[string]{}
	var count int
	for _, name := range field_names {
		if oc.get_field_value(name).is_truthy() {
			truthy_set.Add(name)
			count++
		}
	}
	return truthy_set, count
}

/////////////////////////////////////////////////////////////////////
/////// RULES: CONDITIONAL
/////////////////////////////////////////////////////////////////////

// If applies f only when condition is true.
func (c *AnyChecker) If(
	condition bool,
	f func(*AnyChecker) *AnyChecker,
) *AnyChecker {
	if c.done {
		return c
	}
	if condition {
		return f(c)
	}
	return c
}

/////////////////////////////////////////////////////////////////////
/////// RULES: MEMBERSHIP
/////////////////////////////////////////////////////////////////////

// In validates the value is in the permitted slice.
func (c *AnyChecker) In(permitted any) *AnyChecker {
	if c.done {
		return c
	}
	if c.validate_against_slice(permitted) {
		return c
	}
	c.fail_f("%s has an invalid value (%v)", c.label, c.true_value)
	return c
}

// NotIn validates the value is not in the prohibited slice.
func (c *AnyChecker) NotIn(prohibited any) *AnyChecker {
	if c.done {
		return c
	}
	if c.validate_against_slice(prohibited) {
		c.fail_f("%s has a prohibited value (%v)", c.label, c.true_value)
		return c
	}
	return c
}

func (c *AnyChecker) validate_against_slice(values_slice any) bool {
	if c.done {
		return false
	}
	if values_slice == nil {
		c.fail_f("%s is nil", c.label)
		c.done = true
		return false
	}
	rv := reflect.ValueOf(values_slice)
	if !rv.IsValid() {
		c.fail_f("%s is nil", c.label)
		c.done = true
		return false
	}
	base := safe_deref(rv)
	if base.Kind() != reflect.Slice && base.Kind() != reflect.Array {
		c.fail_f("%s is not a slice or array", c.label)
		c.done = true
		return false
	}
	if base.Len() == 0 {
		c.fail_f("%s is empty", c.label)
		c.done = true
		return false
	}
	tv := reflect.ValueOf(c.true_value)
	if !tv.IsValid() {
		return false
	}
	true_base := safe_deref(tv)
	for i := range base.Len() {
		if compare_values(true_base, safe_deref(base.Index(i))) {
			return true
		}
	}
	return false
}

/////////////////////////////////////////////////////////////////////
/////// RULES: STRINGS
/////////////////////////////////////////////////////////////////////

func (c *AnyChecker) validate_str() (string, bool) {
	if c.done {
		return "", false
	}
	base := safe_deref(c.reflect_value)
	if base.Kind() != reflect.String {
		c.fail_f("%s is not string-like", c.label)
		return "", false
	}
	return base.String(), true
}

// PermittedChars ensures the string contains only characters in allowed.
func (c *AnyChecker) PermittedChars(allowed string) *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validate_str()
	if !ok {
		return c
	}
	allowed_set := set.Set[rune]{}
	for _, ch := range allowed {
		allowed_set.Add(ch)
	}
	for _, ch := range str {
		if !allowed_set.Has(ch) {
			c.fail_f("%s contains invalid character: %q", c.label, ch)
			return c
		}
	}
	return c
}

// Email validates the string is a valid email address.
func (c *AnyChecker) Email() *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validate_str()
	if !ok {
		return c
	}
	if str == "" {
		c.fail_f("%s is required", c.label)
		return c
	}
	if _, err := mail.ParseAddress(str); err != nil {
		c.fail_f("%s must be a valid email address", c.label)
	}
	return c
}

// Regex validates the string matches the pattern.
func (c *AnyChecker) Regex(pattern *regexp.Regexp) *AnyChecker {
	if c.done {
		return c
	}
	if pattern == nil {
		c.fail_f("regexp pattern for %s validation is nil", c.label)
		return c
	}
	str, ok := c.validate_str()
	if !ok {
		return c
	}
	if !pattern.MatchString(str) {
		c.fail_f("%s does not match required pattern", c.label)
	}
	return c
}

// StartsWith validates the string starts with prefix.
func (c *AnyChecker) StartsWith(prefix string) *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validate_str()
	if !ok {
		return c
	}
	if !strings.HasPrefix(str, prefix) {
		c.fail_f("%s must start with %s", c.label, prefix)
	}
	return c
}

// EndsWith validates the string ends with suffix.
func (c *AnyChecker) EndsWith(suffix string) *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validate_str()
	if !ok {
		return c
	}
	if !strings.HasSuffix(str, suffix) {
		c.fail_f("%s must end with %s", c.label, suffix)
	}
	return c
}

// URL validates the string is a valid URL.
func (c *AnyChecker) URL() *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validate_str()
	if !ok {
		return c
	}
	if _, err := url.ParseRequestURI(str); err != nil {
		c.fail_f("%s must be a valid URL", c.label)
	}
	return c
}

/////////////////////////////////////////////////////////////////////
/////// RULES: NUMERIC
/////////////////////////////////////////////////////////////////////

// Min validates the numeric value or length is >= min.
func (c *AnyChecker) Min(min float64) *AnyChecker {
	if c.done {
		return c
	}
	return c.validate_numeric(
		func(v float64) bool { return v >= min },
		func(nature string, v float64) string {
			return fmt.Sprintf(
				"minimum permitted %s for %s is %v, got %v",
				nature,
				c.label,
				min,
				v,
			)
		},
	)
}

// Max validates the numeric value or length is <= max.
func (c *AnyChecker) Max(max float64) *AnyChecker {
	if c.done {
		return c
	}
	return c.validate_numeric(
		func(v float64) bool { return v <= max },
		func(nature string, v float64) string {
			return fmt.Sprintf(
				"maximum permitted %s for %s is %v, got %v",
				nature,
				c.label,
				max,
				v,
			)
		},
	)
}

// RangeInclusive validates the numeric value or length is in [min, max].
func (c *AnyChecker) RangeInclusive(min, max float64) *AnyChecker {
	if c.done {
		return c
	}
	return c.validate_numeric(
		func(v float64) bool { return v >= min && v <= max },
		func(nature string, v float64) string {
			return fmt.Sprintf(
				"permitted %s range for %s is [%v, %v], got %v",
				nature,
				c.label,
				min,
				max,
				v,
			)
		},
	)
}

// RangeExclusive validates the numeric value or length is in (min, max).
func (c *AnyChecker) RangeExclusive(min, max float64) *AnyChecker {
	if c.done {
		return c
	}
	return c.validate_numeric(
		func(v float64) bool { return v > min && v < max },
		func(nature string, v float64) string {
			return fmt.Sprintf(
				"permitted %s range for %s is (%v, %v), got %v",
				nature,
				c.label,
				min,
				max,
				v,
			)
		},
	)
}

func (c *AnyChecker) validate_numeric(
	check func(float64) bool,
	err_msg func(nature string, val float64) string,
) *AnyChecker {
	if c.done {
		return c
	}
	val, nature, ok := extract_numeric(c.base_reflect_value)
	if !ok {
		c.fail_f(
			"cannot apply numeric check to type %s for %s",
			c.base_reflect_value.Kind(),
			c.label,
		)
		return c
	}
	if !check(val) {
		c.fail(err_msg(nature, val))
	}
	return c
}

func extract_numeric(v reflect.Value) (float64, string, bool) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), "value", true
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return float64(v.Uint()), "value", true
	case reflect.Float32, reflect.Float64:
		return v.Float(), "value", true
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return float64(v.Len()), "length", true
	}
	return 0, "", false
}

/////////////////////////////////////////////////////////////////////
/////// REFLECT UTILITIES
/////////////////////////////////////////////////////////////////////

type type_state struct {
	reflect_value     reflect.Value
	is_struct_like    bool
	is_map_like       bool
	is_map_str_keys   bool
	is_slice_or_array bool
}

func get_type_state(rv reflect.Value) type_state {
	base := safe_deref(rv)
	is_map := base.Kind() == reflect.Map
	return type_state{
		reflect_value:   rv,
		is_struct_like:  base.Kind() == reflect.Struct,
		is_map_like:     is_map,
		is_map_str_keys: is_map && base.Type().Key().Kind() == reflect.String,
		is_slice_or_array: base.Kind() == reflect.Slice ||
			base.Kind() == reflect.Array,
	}
}

type field_wrapper struct {
	true_value    any
	reflect_value reflect.Value
}

func (fw *field_wrapper) is_truthy() bool {
	return !is_effectively_zero(fw.reflect_value)
}

func safe_deref(rv reflect.Value) reflect.Value {
	if rv.Kind() == reflect.Pointer {
		return rv.Elem()
	}
	return rv
}

func safe_is_nil(v reflect.Value) bool {
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		return v.IsNil()
	}
	return false
}

func is_effectively_zero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
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

func compare_values(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return !a.IsValid() && !b.IsValid()
	}
	if reflect.DeepEqual(a.Interface(), b.Interface()) {
		return true
	}
	if a.Kind() == b.Kind() {
		switch a.Kind() {
		case reflect.String:
			return a.String() == b.String()
		case reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64:
			return a.Int() == b.Int()
		case reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64:
			return a.Uint() == b.Uint()
		case reflect.Float32, reflect.Float64:
			return a.Float() == b.Float()
		case reflect.Bool:
			return a.Bool() == b.Bool()
		}
	}
	return false
}

/////////////////////////////////////////////////////////////////////
/////// RECURSIVE VALIDATION
/////////////////////////////////////////////////////////////////////

var validator_type = reflect.TypeFor[Validator]()

func validate_recursive(label string, current reflect.Value) []error {
	var errs []error

	if !current.IsValid() || safe_is_nil(current) {
		return errs
	}

	validated_directly := false

	if current.CanInterface() {
		if impl, ok := current.Interface().(Validator); ok {
			if err := impl.Validate(); err != nil {
				if !IsValidationError(err) {
					errs = append(errs, fmt.Errorf("%s: %w", label, err))
				} else {
					errs = append(errs, err)
				}
			}
			validated_directly = true
		}
	}

	if !validated_directly && current.Kind() != reflect.Pointer &&
		current.CanAddr() {
		ptr := current.Addr()
		if reflectutil.DoesTypeImplementInterface(ptr.Type(), validator_type) &&
			ptr.CanInterface() {
			if impl, ok := ptr.Interface().(Validator); ok {
				if err := impl.Validate(); err != nil {
					if !IsValidationError(err) {
						errs = append(errs, fmt.Errorf("%s: %w", label, err))
					} else {
						errs = append(errs, err)
					}
				}
			}
		}
	}

	base := current
	if base.Kind() == reflect.Pointer {
		if base.IsNil() {
			return errs
		}
		base = base.Elem()
	}

	switch base.Kind() {
	case reflect.Struct:
		for i := range base.NumField() {
			field := base.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			field_label := fmt.Sprintf("%s.%s", label, field.Name)
			if sub := validate_recursive(field_label, base.Field(i)); len(
				sub,
			) > 0 {
				errs = append(errs, sub...)
			}
		}
	case reflect.Map:
		if base.IsNil() {
			break
		}
		iter := base.MapRange()
		for iter.Next() {
			key := iter.Key()
			key_str := "<unstringable_key>"
			if key.IsValid() {
				if key.CanInterface() {
					key_str = fmt.Sprintf("%v", key.Interface())
				} else if key.Kind() == reflect.String {
					key_str = key.String()
				}
			}
			map_label := fmt.Sprintf("%s[%s]", label, key_str)
			if sub := validate_recursive(map_label+"(key)", key); len(sub) > 0 {
				errs = append(errs, sub...)
			}
			if sub := validate_recursive(map_label+"(value)", iter.Value()); len(
				sub,
			) > 0 {
				errs = append(errs, sub...)
			}
		}
	case reflect.Slice, reflect.Array:
		if base.Kind() == reflect.Slice && base.IsNil() {
			break
		}
		for i := range base.Len() {
			elem_label := fmt.Sprintf("%s[%d]", label, i)
			if sub := validate_recursive(elem_label, base.Index(i)); len(
				sub,
			) > 0 {
				errs = append(errs, sub...)
			}
		}
	}

	return errs
}

func attempt_validation(label string, x any) error {
	if x == nil {
		return nil
	}

	v := reflect.ValueOf(x)
	effective := v

	implements := reflectutil.DoesTypeImplementInterface(
		v.Type(),
		validator_type,
	)
	can_call := implements &&
		(v.Type().Implements(validator_type) || v.CanAddr())

	if !can_call && v.Kind() != reflect.Pointer && implements {
		cp := reflect.New(v.Type())
		cp.Elem().Set(v)
		effective = cp
	}

	if errs := validate_recursive(label, effective); len(errs) > 0 {
		return &ValidationError{Err: errors.Join(errs...)}
	}
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// URL SEARCH PARAMS PARSING
/////////////////////////////////////////////////////////////////////

func parse_url_values(values map[string][]string, dest any) error {
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Pointer || dv.IsNil() {
		return fmt.Errorf(
			"validate.parseURLValues: destination must be non-nil",
		)
	}
	elem := dv.Elem()
	if elem.Kind() == reflect.Interface {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf(
			"validate.parseURLValues: destination must point to a struct",
		)
	}
	return set_nested_field(elem, values)
}

func set_nested_field(v reflect.Value, values map[string][]string) error {
	t := v.Type()
	for i := range v.NumField() {
		field := t.Field(i)
		fv := v.Field(i)

		if fv.Kind() == reflect.Pointer {
			kind := fv.Type().Elem().Kind()
			if kind == reflect.Struct || kind == reflect.Map ||
				kind == reflect.Slice {
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				fv = fv.Elem()
			}
		}

		if !fv.CanSet() {
			continue
		}

		tag := reflectutil.JSONFieldName(field)

		if field.Anonymous {
			if err := set_nested_field(fv, values); err != nil {
				return err
			}
			continue
		}

		if fv.Kind() == reflect.Struct {
			nested := make(map[string][]string)
			pfx := tag + "."
			for k, v := range values {
				if strings.HasPrefix(k, pfx) {
					nested[strings.TrimPrefix(k, pfx)] = v
				}
			}
			if err := set_nested_field(fv, nested); err != nil {
				return err
			}
			continue
		}

		if fv.Kind() == reflect.Map {
			nested := make(map[string][]string)
			pfx := tag + "."
			for k, v := range values {
				if strings.HasPrefix(k, pfx) {
					nested[strings.TrimPrefix(k, pfx)] = v
				}
			}
			if err := set_map_field(fv, nested); err != nil {
				return err
			}
			continue
		}

		if fv.Kind() == reflect.Slice {
			var nested []string
			for k, v := range values {
				if strings.HasPrefix(k, tag) {
					for _, s := range v {
						if s != "" {
							nested = append(nested, s)
						}
					}
				}
			}
			if len(nested) == 0 {
				fv.Set(reflect.MakeSlice(fv.Type(), 0, 0))
			} else if err := set_slice_field(fv, nested); err != nil {
				return err
			}
			continue
		}

		if val, ok := values[tag]; ok {
			if err := set_field(fv, val); err != nil {
				return fmt.Errorf("error setting field %s: %w", field.Name, err)
			}
		}
	}
	return nil
}

func set_map_field(v reflect.Value, values map[string][]string) error {
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}
	for key, val := range values {
		kv := reflect.ValueOf(key)
		ev := reflect.New(v.Type().Elem()).Elem()
		if ev.Kind() == reflect.Map {
			nested := make(map[string][]string)
			pfx := key + "."
			for nk, nv := range values {
				if strings.HasPrefix(nk, pfx) {
					nested[strings.TrimPrefix(nk, pfx)] = nv
				}
			}
			if err := set_map_field(ev, nested); err != nil {
				return err
			}
		} else {
			if err := set_field(ev, val); err != nil {
				return fmt.Errorf("error setting map value for key %s: %w", key, err)
			}
		}
		v.SetMapIndex(kv, ev)
	}
	return nil
}

func set_field(field reflect.Value, values []string) error {
	if len(values) == 0 {
		return nil
	}
	switch field.Kind() {
	case reflect.Pointer:
		if values[0] == "" {
			field.Set(reflect.Zero(field.Type()))
			return nil
		}
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return set_single_value(field.Elem(), values[0])
	case reflect.Slice:
		return set_slice_field(field, values)
	case reflect.Map:
		return set_map_field(field, map[string][]string{"": values})
	case reflect.String,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Float32,
		reflect.Float64,
		reflect.Bool:
		return set_single_value(field, values[0])
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
}

func set_slice_field(field reflect.Value, values []string) error {
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))
	for i, val := range values {
		elem := slice.Index(i)
		if elem.Kind() == reflect.Pointer {
			elem.Set(reflect.New(elem.Type().Elem()))
			elem = elem.Elem()
		}
		if err := set_single_value(elem, val); err != nil {
			return err
		}
	}
	field.Set(slice)
	return nil
}

func set_single_value(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("field is not settable")
	}
	if value == "" {
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}
