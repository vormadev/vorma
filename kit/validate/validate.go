// Package validate provides a simple way to validate and parse data from HTTP requests.
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

type Validator interface{ Validate() error }

type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

/////////////////////////////////////////////////////////////////////
/////// ANY CHECKER
/////////////////////////////////////////////////////////////////////

type AnyChecker struct {
	label            string
	trueValue        any
	baseReflectValue reflect.Value
	typeState

	done   bool
	errors []error
}

func newAnyChecker(
	label string,
	trueValue any,
	reflectValue reflect.Value,
) *AnyChecker {
	return &AnyChecker{
		label:            label,
		trueValue:        trueValue,
		baseReflectValue: safeDereference(reflectValue),
		typeState:        getTypeState(reflectValue),
	}
}

func (c *AnyChecker) Required() *AnyChecker { return c.init(true) }
func (c *AnyChecker) Optional() *AnyChecker { return c.init(false) }

func (c *AnyChecker) Error() error {
	if len(c.errors) > 0 {
		return &ValidationError{Err: errors.Join(c.errors...)}
	}
	return nil
}

func (c *AnyChecker) ok() { c.done = true }

func (c *AnyChecker) fail(errMsg string) {
	c.done = true
	c.errors = append(c.errors, errors.New(errMsg))
}

func (c *AnyChecker) failF(format string, args ...any) {
	c.fail(fmt.Sprintf(format, args...))
}

func (c *AnyChecker) init(required bool) *AnyChecker {
	if c.done {
		return c
	}
	if isEffectivelyZero(c.reflectValue) {
		if required {
			c.fail(fmt.Sprintf("%s is required", c.label))
		} else {
			c.ok()
		}
		return c
	}
	if errs := validateRecursive(c.label, c.reflectValue); len(errs) > 0 {
		c.errors = append(c.errors, errs...)
		c.done = true
	}
	return c
}

/////////////////////////////////////////////////////////////////////
/////// OBJECT CHECKER
/////////////////////////////////////////////////////////////////////

type ObjectChecker struct {
	AnyChecker
	ChildCheckers []*AnyChecker
}

func (oc *ObjectChecker) Required(
	field string,
) *AnyChecker {
	return oc.validateField(field, true)
}

func (oc *ObjectChecker) Optional(
	field string,
) *AnyChecker {
	return oc.validateField(field, false)
}

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

func (oc *ObjectChecker) validateField(
	fieldName string,
	required bool,
) (c *AnyChecker) {
	if oc.done {
		c = newAnyChecker(fieldName, nil, reflect.Value{})
		c.done = true
		return c
	}
	if oc.isStructLike {
		if err := oc.ensureStructField(fieldName); err != nil {
			c = newAnyChecker(fieldName, nil, reflect.Value{})
			c.fail(err.Error())
			oc.ChildCheckers = append(oc.ChildCheckers, c)
			return c
		}
	}
	wrappedField := oc.getFieldValue(fieldName)
	c = newAnyChecker(
		fieldName,
		wrappedField.trueValue,
		wrappedField.reflectValue,
	)
	oc.ChildCheckers = append(oc.ChildCheckers, c)
	if required {
		c.Required()
	} else {
		c.Optional()
	}
	return
}

func (oc *ObjectChecker) getFieldValue(
	fieldName string,
) (wrapped *fieldWrapper) {
	wrapped = &fieldWrapper{}
	if oc.isMapWithStrKeysLike {
		key := reflect.ValueOf(fieldName)
		wrapped.reflectValue = oc.baseReflectValue.MapIndex(key)
		if !wrapped.reflectValue.IsValid() {
			return
		}
		wrapped.trueValue = wrapped.reflectValue.Interface()
		return
	}
	if oc.isStructLike {
		wrapped.reflectValue = oc.baseReflectValue.FieldByName(fieldName)
		if !wrapped.reflectValue.IsValid() ||
			!wrapped.reflectValue.CanInterface() {
			return
		}
		wrapped.trueValue = wrapped.reflectValue.Interface()
		return
	}
	panic("this should never happen")
}

func (oc *ObjectChecker) ensureStructField(fieldName string) error {
	field, ok := oc.baseReflectValue.Type().FieldByName(fieldName)
	if !ok {
		return fmt.Errorf(
			"unknown field %s on %s",
			fieldName,
			oc.baseReflectValue.Type(),
		)
	}
	if !field.IsExported() {
		return fmt.Errorf(
			"field %s on %s is unexported",
			fieldName,
			oc.baseReflectValue.Type(),
		)
	}
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// CORE ENTRY POINTS
/////////////////////////////////////////////////////////////////////

// An "object" as defined by this library is either (1) a struct,
// (2) a map with string keys, or (3) a pointer to (1) or (2). If
// you want to add field-level validation rules to an object, use
// this entry point. If you are not validating an object, or you
// just want any embedded fields that implement Validator to be
// validated, you can use the Any function. If the target is an
// object, both Object() and Any() will auto-validate any of the
// object's fields that implement Validator.

func Any(label string, anything any) *AnyChecker {
	return newAnyChecker(label, anything, reflect.ValueOf(anything))
}

func Object(object any) *ObjectChecker {
	oc := &ObjectChecker{}
	if object == nil {
		oc.fail("object cannot be nil")
		return oc
	}
	reflectValue := reflect.ValueOf(object)
	typeState := getTypeState(reflectValue)
	if !typeState.isStructLike && !typeState.isMapWithStrKeysLike {
		oc.failF(
			"object must be a struct or a map with string keys (got %T)",
			object,
		)
		return oc
	}
	oc.label = reflectValue.Type().String()
	oc.trueValue = object
	oc.reflectValue = reflectValue
	oc.baseReflectValue = safeDereference(reflectValue)
	oc.typeState = typeState
	return oc
}

/////////////////////////////////////////////////////////////////////
/////// UTILS
/////////////////////////////////////////////////////////////////////

func validateRecursive(label string, currentValue reflect.Value) []error {
	var errs []error

	if !currentValue.IsValid() || safeIsNil(currentValue) {
		return errs
	}

	validatedByDirectCall := false
	validatorInterface := reflect.TypeFor[Validator]()

	if currentValue.CanInterface() {
		if impl, ok := currentValue.Interface().(Validator); ok {
			if err := impl.Validate(); err != nil {
				if !IsValidationError(err) {
					errs = append(errs, fmt.Errorf("%s: %w", label, err))
				} else {
					errs = append(errs, err)
				}
			}
			validatedByDirectCall = true
		}
	}

	if !validatedByDirectCall && currentValue.Kind() != reflect.Ptr &&
		currentValue.CanAddr() {
		ptrValue := currentValue.Addr()
		if reflectutil.DoesTypeImplementInterface(
			ptrValue.Type(),
			validatorInterface,
		) &&
			ptrValue.CanInterface() {
			if impl, ok := ptrValue.Interface().(Validator); ok {
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

	baseValue := currentValue
	if baseValue.Kind() == reflect.Ptr {
		if baseValue.IsNil() {
			return errs
		}
		baseValue = baseValue.Elem()
	}

	switch baseValue.Kind() {
	case reflect.Struct:
		for i := range baseValue.NumField() {
			field := baseValue.Type().Field(i)
			fieldValue := baseValue.Field(i)
			if !field.IsExported() {
				continue
			}
			fieldLabel := fmt.Sprintf("%s.%s", label, field.Name)
			if locErrs := validateRecursive(fieldLabel, fieldValue); len(
				locErrs,
			) > 0 {
				errs = append(errs, locErrs...)
			}
		}
	case reflect.Map:
		if baseValue.IsNil() {
			break
		}
		iter := baseValue.MapRange()
		for iter.Next() {
			key := iter.Key()
			val := iter.Value()
			keyLabelPart := "<unstringable_key>"
			if key.IsValid() {
				if key.CanInterface() {
					keyLabelPart = fmt.Sprintf("%v", key.Interface())
				} else if key.Kind() == reflect.String {
					keyLabelPart = key.String()
				}
			}
			mapLabel := fmt.Sprintf("%s[%s]", label, keyLabelPart)

			if locErrs := validateRecursive(mapLabel+"(key)", key); len(
				locErrs,
			) > 0 {
				errs = append(errs, locErrs...)
			}
			if locErrs := validateRecursive(mapLabel+"(value)", val); len(
				locErrs,
			) > 0 {
				errs = append(errs, locErrs...)
			}
		}
	case reflect.Slice, reflect.Array:
		if baseValue.Kind() == reflect.Slice && baseValue.IsNil() {
			break
		}
		for i := range baseValue.Len() {
			elemValue := baseValue.Index(i)
			elemLabel := fmt.Sprintf("%s[%d]", label, i)
			if locErrs := validateRecursive(elemLabel, elemValue); len(
				locErrs,
			) > 0 {
				errs = append(errs, locErrs...)
			}
		}
	}

	return errs
}

func safeDereference(reflectValue reflect.Value) reflect.Value {
	if reflectValue.Kind() == reflect.Ptr {
		return reflectValue.Elem()
	}
	return reflectValue
}

type typeState struct {
	reflectValue         reflect.Value
	isStructLike         bool
	isMapLike            bool
	isMapWithStrKeysLike bool
	isSliceOrArrayLike   bool
}

func getTypeState(reflectValue reflect.Value) typeState {
	base := safeDereference(reflectValue)
	isMapLike := base.Kind() == reflect.Map
	isMapWithStrKeysLike := isMapLike &&
		base.Type().Key().Kind() == reflect.String
	return typeState{
		reflectValue:         reflectValue,
		isStructLike:         base.Kind() == reflect.Struct,
		isMapLike:            isMapLike,
		isMapWithStrKeysLike: isMapWithStrKeysLike,
		isSliceOrArrayLike: base.Kind() == reflect.Slice ||
			base.Kind() == reflect.Array,
	}
}

type fieldWrapper struct {
	trueValue    any
	reflectValue reflect.Value
}

func (fw *fieldWrapper) isTruthy() bool {
	return !isEffectivelyZero(fw.reflectValue)
}

func isEffectivelyZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}
		if v.Kind() == reflect.Ptr {
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

func safeIsNil(v reflect.Value) bool {
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		return v.IsNil()
	}
	return false
}

func attemptValidation(label string, x any) error {
	if x == nil {
		return nil
	}

	v := reflect.ValueOf(x)
	var effectiveValue reflect.Value = v

	validatorInterface := reflect.TypeFor[Validator]()
	implementsValidator := reflectutil.DoesTypeImplementInterface(
		v.Type(),
		validatorInterface,
	)
	canCallDirectly := implementsValidator &&
		(v.Type().Implements(validatorInterface) || v.CanAddr())

	if !canCallDirectly && v.Kind() != reflect.Ptr && implementsValidator {
		copyPtr := reflect.New(v.Type())
		copyPtr.Elem().Set(v)
		effectiveValue = copyPtr
	}

	if errs := validateRecursive(label, effectiveValue); len(errs) > 0 {
		return &ValidationError{Err: errors.Join(errs...)}
	}

	return nil
}

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

// Helper function to compare values across types
func compareValues(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return !a.IsValid() && !b.IsValid()
	}
	if reflect.DeepEqual(a.Interface(), b.Interface()) {
		return true
	}

	aKind := a.Kind()
	bKind := b.Kind()

	if aKind == bKind {
		switch aKind {
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

// validateAgainstSlice checks if the value matches any element in the slice
// Returns true if a match is found, false otherwise
func (c *AnyChecker) validateAgainstSlice(valuesSlice any) bool {
	if c.done {
		return false
	}
	if valuesSlice == nil {
		c.failF("%s is nil", c.label)
		c.done = true
		return false
	}
	baseValue := reflect.ValueOf(valuesSlice)
	if !baseValue.IsValid() {
		c.failF("%s is nil", c.label)
		c.done = true
		return false
	}
	base := safeDereference(baseValue)
	if base.Kind() != reflect.Slice && base.Kind() != reflect.Array {
		c.failF("%s is not a slice or array", c.label)
		c.done = true
		return false
	}
	if base.Len() == 0 {
		c.failF("%s is empty", c.label)
		c.done = true
		return false
	}
	trueValue := reflect.ValueOf(c.trueValue)
	if !trueValue.IsValid() {
		return false
	}
	trueBaseReflect := safeDereference(trueValue)
	for i := range base.Len() {
		itemBase := safeDereference(base.Index(i))
		if compareValues(trueBaseReflect, itemBase) {
			return true
		}
	}
	return false
}

// In validates that the value is in the permitted values slice
func (c *AnyChecker) In(permittedValuesSlice any) *AnyChecker {
	if c.done {
		return c
	}
	if c.validateAgainstSlice(permittedValuesSlice) {
		return c
	}
	c.failF("%s has an invalid value (%v)", c.label, c.trueValue)
	return c
}

// NotIn validates that the value is not in the prohibited values slice
func (c *AnyChecker) NotIn(prohibitedValuesSlice any) *AnyChecker {
	if c.done {
		return c
	}
	if c.validateAgainstSlice(prohibitedValuesSlice) {
		c.failF("%s has a prohibited value (%v)", c.label, c.trueValue)
		return c
	}
	return c
}

/////////////////////////////////////////////////////////////////////
/////// RELATIONSHIPS BETWEEN OBJECT FIELDS
/////////////////////////////////////////////////////////////////////

func (oc *ObjectChecker) MutuallyExclusive(
	label string,
	fields ...string,
) *ObjectChecker {
	if oc.done {
		return oc
	}
	f := func(truthyCount, totalFields int) string {
		if truthyCount > 1 {
			return "fields in group %s are mutually exclusive"
		}
		return ""
	}
	return oc.validateFieldGroupConstraint(label, fields, f)
}

func (oc *ObjectChecker) MutuallyRequired(
	label string,
	fields ...string,
) *ObjectChecker {
	if oc.done {
		return oc
	}
	f := func(truthyCount, totalFields int) string {
		if truthyCount > 0 && truthyCount < totalFields {
			return "all fields in group %s are required when any is provided"
		}
		return ""
	}
	return oc.validateFieldGroupConstraint(label, fields, f)
}

type constraintFn func(truthyCount, totalFields int) string

func (oc *ObjectChecker) validateFieldGroupConstraint(
	label string,
	fields []string,
	constraintFn constraintFn,
) *ObjectChecker {
	if oc.done {
		return oc
	}
	_, truthyCount := oc.validateFieldGroup(fields)
	totalFields := len(fields)
	if errMsgFExpectingLabel := constraintFn(truthyCount, totalFields); errMsgFExpectingLabel != "" {
		oc.errors = append(oc.errors, fmt.Errorf(errMsgFExpectingLabel, label))
	}
	return oc
}

func (oc *ObjectChecker) validateFieldGroup(
	fieldNames []string,
) (set.Set[string], int) {
	truthySet := set.Set[string]{}
	var truthyCount int
	for _, fieldName := range fieldNames {
		if oc.getFieldValue(fieldName).isTruthy() {
			truthySet.Add(fieldName)
			truthyCount++
		}
	}
	return truthySet, truthyCount
}

/////////////////////////////////////////////////////////////////////
/////// STRINGS
/////////////////////////////////////////////////////////////////////

func (c *AnyChecker) validateStr() (trueStr string, ok bool) {
	if c.done {
		return "", false
	}
	base := safeDereference(c.reflectValue)
	if base.Kind() != reflect.String {
		c.failF("%s is not string-like", c.label)
		return "", false
	}
	return base.String(), true
}

func (c *AnyChecker) PermittedChars(allowedChars string) *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validateStr()
	if !ok {
		return c
	}
	allowedCharsSet := set.Set[rune]{}
	for _, char := range allowedChars {
		allowedCharsSet.Add(char)
	}
	for _, char := range str {
		if !allowedCharsSet.Has(char) {
			c.failF("%s contains invalid character: %q", c.label, char)
			return c
		}
	}
	return c
}

func (c *AnyChecker) Email() *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validateStr()
	if !ok {
		return c
	}
	if str == "" {
		c.failF("%s is required", c.label)
		return c
	}
	if _, err := mail.ParseAddress(str); err != nil {
		c.failF("%s must be a valid email address", c.label)
	}
	return c
}

func (c *AnyChecker) Regex(regex *regexp.Regexp) *AnyChecker {
	if c.done {
		return c
	}
	if regex == nil {
		c.failF("regexp pattern for %s validation is nil", c.label)
		return c
	}
	str, ok := c.validateStr()
	if !ok {
		return c
	}
	if !regex.MatchString(str) {
		c.failF("%s does not match required pattern", c.label)
	}
	return c
}

func (c *AnyChecker) StartsWith(prefix string) *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validateStr()
	if !ok {
		return c
	}
	if !strings.HasPrefix(str, prefix) {
		c.failF("%s must start with %s", c.label, prefix)
	}
	return c
}

func (c *AnyChecker) EndsWith(suffix string) *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validateStr()
	if !ok {
		return c
	}
	if !strings.HasSuffix(str, suffix) {
		c.failF("%s must end with %s", c.label, suffix)
	}
	return c
}

func (c *AnyChecker) URL() *AnyChecker {
	if c.done {
		return c
	}
	str, ok := c.validateStr()
	if !ok {
		return c
	}
	if _, err := url.ParseRequestURI(str); err != nil {
		c.failF("%s must be a valid URL", c.label)
	}
	return c
}

/////////////////////////////////////////////////////////////////////
/////// NUMERIC
/////////////////////////////////////////////////////////////////////

func (c *AnyChecker) Min(min float64) *AnyChecker {
	if c.done {
		return c
	}
	f1 := func(val float64) bool {
		return val >= min
	}
	f2 := func(typeName string, val float64) string {
		return fmt.Sprintf(
			"minimum permitted %s for %s is %v, got %v",
			typeName,
			c.label,
			min,
			val,
		)
	}
	return c.validateNumeric(f1, f2)
}

func (c *AnyChecker) Max(max float64) *AnyChecker {
	if c.done {
		return c
	}
	f1 := func(val float64) bool {
		return val <= max
	}
	f2 := func(typeName string, val float64) string {
		return fmt.Sprintf(
			"maximum permitted %s for %s is %v, got %v",
			typeName,
			c.label,
			max,
			val,
		)
	}
	return c.validateNumeric(f1, f2)
}

func (c *AnyChecker) RangeInclusive(min, max float64) *AnyChecker {
	if c.done {
		return c
	}
	f1 := func(val float64) bool {
		return val >= min && val <= max
	}
	f2 := func(typeName string, val float64) string {
		return fmt.Sprintf(
			"permitted %s range for %s is [%v, %v], got %v",
			typeName,
			c.label,
			min,
			max,
			val,
		)
	}
	return c.validateNumeric(f1, f2)
}

func (c *AnyChecker) RangeExclusive(min, max float64) *AnyChecker {
	if c.done {
		return c
	}
	f1 := func(val float64) bool {
		return val > min && val < max
	}
	f2 := func(typeName string, val float64) string {
		return fmt.Sprintf(
			"permitted %s range for %s is (%v, %v), got %v",
			typeName,
			c.label,
			min,
			max,
			val,
		)
	}
	return c.validateNumeric(f1, f2)
}

type checkFn func(float64) bool
type getErrorMsg func(typeName string, val float64) string

func (c *AnyChecker) validateNumeric(
	checkFn checkFn,
	getErrorMsg getErrorMsg,
) *AnyChecker {
	if c.done {
		return c
	}
	trueValue, nature, ok := extractNumericFromReflectValue(c.baseReflectValue)
	if !ok {
		c.failF(
			"cannot apply numeric check to type %s for %s",
			c.baseReflectValue.Kind(), c.label,
		)
		return c
	}
	if ok = checkFn(trueValue); !ok {
		c.fail(getErrorMsg(nature, trueValue))
	}
	return c
}

func extractNumericFromReflectValue(
	value reflect.Value,
) (trueValue float64, nature string, ok bool) {
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(value.Int()), "value", true
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return float64(value.Uint()), "value", true
	case reflect.Float32, reflect.Float64:
		return value.Float(), "value", true
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return float64(value.Len()), "length", true
	}
	return 0, "", false
}

// parseURLValues parses URL values into a struct.
func parseURLValues(values map[string][]string, destStructPtr any) error {
	dstValue := reflect.ValueOf(destStructPtr)
	if dstValue.Kind() != reflect.Ptr || dstValue.IsNil() {
		return fmt.Errorf(
			"validate.parseURLValues: destination must be non-nil",
		)
	}

	dstElem := dstValue.Elem()

	if dstElem.Kind() == reflect.Interface {
		dstElem = dstElem.Elem()
	}

	if dstElem.Kind() != reflect.Struct {
		return fmt.Errorf(
			"validate.parseURLValues: destination must point to a struct",
		)
	}

	return setNestedField(dstElem, values)
}

func setNestedField(v reflect.Value, values map[string][]string) error {
	t := v.Type()
	for i := range v.NumField() {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if fieldValue.Kind() == reflect.Ptr {
			kind := fieldValue.Type().Elem().Kind()
			if kind == reflect.Struct || kind == reflect.Map ||
				kind == reflect.Slice {
				if fieldValue.IsNil() {
					fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
				}
				fieldValue = fieldValue.Elem()
			}
		}

		if !fieldValue.CanSet() {
			continue
		}

		tag := reflectutil.JSONFieldName(field)

		// Handle embedded structs
		if field.Anonymous {
			if err := setNestedField(fieldValue, values); err != nil {
				return err
			}
			continue
		}

		if fieldValue.Kind() == reflect.Struct {
			nestedValues := make(map[string][]string)
			prefix := tag + "."

			for key, value := range values {
				if strings.HasPrefix(key, prefix) {
					nestedValues[strings.TrimPrefix(key, prefix)] = value
				}
			}

			if err := setNestedField(fieldValue, nestedValues); err != nil {
				return err
			}

			continue
		}

		if fieldValue.Kind() == reflect.Map {
			nestedValues := make(map[string][]string)
			prefix := tag + "."

			for key, value := range values {
				if strings.HasPrefix(key, prefix) {
					nestedValues[strings.TrimPrefix(key, prefix)] = value
				}
			}

			if err := setMapField(fieldValue, nestedValues); err != nil {
				return err
			}

			continue
		}

		if fieldValue.Kind() == reflect.Slice {
			var nestedValues []string
			for key, value := range values {
				if strings.HasPrefix(key, tag) {
					// Filter out empty strings to avoid creating unintended elements
					for _, v := range value {
						if v != "" {
							nestedValues = append(nestedValues, v)
						}
					}
				}
			}
			if len(nestedValues) == 0 {
				// Set to an empty slice if no valid values are found
				fieldValue.Set(reflect.MakeSlice(fieldValue.Type(), 0, 0))
			} else if err := setSliceField(fieldValue, nestedValues); err != nil {
				return err
			}
			continue
		}

		if value, ok := values[tag]; ok {
			if err := setField(fieldValue, value); err != nil {
				return fmt.Errorf("error setting field %s: %w", field.Name, err)
			}

			continue
		}
	}

	return nil
}

func setMapField(v reflect.Value, values map[string][]string) error {
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}

	for key, value := range values {
		keyValue := reflect.ValueOf(key)
		elemValue := reflect.New(v.Type().Elem()).Elem()

		if elemValue.Kind() == reflect.Map {
			nestedValues := make(map[string][]string)
			prefix := key + "."
			for nKey, nValue := range values {
				if strings.HasPrefix(nKey, prefix) {
					nestedValues[strings.TrimPrefix(nKey, prefix)] = nValue
				}
			}
			if err := setMapField(elemValue, nestedValues); err != nil {
				return err
			}
		} else {
			if err := setField(elemValue, value); err != nil {
				return fmt.Errorf("error setting map value for key %s: %w", key, err)
			}
		}

		v.SetMapIndex(keyValue, elemValue)
	}

	return nil
}

func setField(field reflect.Value, values []string) error {
	if len(values) == 0 {
		return nil
	}

	switch field.Kind() {
	case reflect.Ptr:
		if values[0] == "" {
			// Set to nil for empty values
			field.Set(reflect.Zero(field.Type()))
			return nil
		}
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setSingleValueField(field.Elem(), values[0])
	case reflect.Slice:
		return setSliceField(field, values)
	case reflect.Map:
		return setMapField(field, map[string][]string{"": values})
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
		return setSingleValueField(field, values[0])
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
}

func setSliceField(field reflect.Value, values []string) error {
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))
	for i, value := range values {
		elem := slice.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem.Set(reflect.New(elem.Type().Elem()))
			elem = elem.Elem()
		}
		err := setSingleValueField(elem, value)
		if err != nil {
			return err
		}
	}
	field.Set(slice)
	return nil
}

func setSingleValueField(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("field is not settable")
	}

	if value == "" {
		return nil // Do nothing for empty values on non-pointer types
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intValue)
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		uintValue, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(uintValue)
	case reflect.Float32, reflect.Float64:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatValue)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolValue)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}

func validateDestination(destStructPtr any) error {
	if destStructPtr == nil {
		return errors.New("destination is nil")
	}
	return nil
}

// JSONBodyInto decodes an HTTP request body into a struct and validates it.
func JSONBodyInto(r *http.Request, destStructPtr any) error {
	if r == nil {
		return &ValidationError{Err: errors.New("request is nil")}
	}
	if r.Body == nil {
		return &ValidationError{Err: errors.New("request body is nil")}
	}
	if err := validateDestination(destStructPtr); err != nil {
		return &ValidationError{Err: err}
	}
	if err := json.NewDecoder(r.Body).Decode(destStructPtr); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	if err := attemptValidation("validate.JSONBodyInto", destStructPtr); err != nil {
		return err
	}
	return nil
}

// JSONBytesInto decodes a byte slice containing JSON data into a struct and validates it.
func JSONBytesInto(data []byte, destStructPtr any) error {
	if err := validateDestination(destStructPtr); err != nil {
		return &ValidationError{Err: err}
	}
	if err := json.Unmarshal(data, destStructPtr); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	if err := attemptValidation("validate.JSONBytesInto", destStructPtr); err != nil {
		return err
	}
	return nil
}

// JSONStrInto decodes a string containing JSON data into a struct and validates it.
func JSONStrInto(data string, destStructPtr any) error {
	if err := validateDestination(destStructPtr); err != nil {
		return &ValidationError{Err: err}
	}
	if err := json.Unmarshal([]byte(data), destStructPtr); err != nil {
		return &ValidationError{Err: fmt.Errorf("error decoding JSON: %w", err)}
	}
	if err := attemptValidation("validate.JSONStrInto", destStructPtr); err != nil {
		return err
	}
	return nil
}

// URLSearchParamsInto parses the URL parameters of an HTTP request into a struct and validates it.
func URLSearchParamsInto(r *http.Request, destStructPtr any) error {
	if r == nil {
		return &ValidationError{Err: errors.New("request is nil")}
	}
	if r.URL == nil {
		return &ValidationError{Err: errors.New("request URL is nil")}
	}
	if err := validateDestination(destStructPtr); err != nil {
		return &ValidationError{Err: err}
	}
	if err := parseURLValues(r.URL.Query(), destStructPtr); err != nil {
		return &ValidationError{
			Err: fmt.Errorf("error parsing URL parameters: %w", err),
		}
	}
	if err := attemptValidation("validate.URLSearchParamsInto", destStructPtr); err != nil {
		return err
	}
	return nil
}
