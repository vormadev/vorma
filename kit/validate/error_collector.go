package validate

import (
	"errors"
	"fmt"
	"reflect"
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

func newAnyChecker(label string, trueValue any, reflectValue reflect.Value) *AnyChecker {
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

func (oc *ObjectChecker) Required(field string) *AnyChecker { return oc.validateField(field, true) }
func (oc *ObjectChecker) Optional(field string) *AnyChecker { return oc.validateField(field, false) }

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

func (oc *ObjectChecker) validateField(fieldName string, required bool) (c *AnyChecker) {
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
	c = newAnyChecker(fieldName, wrappedField.trueValue, wrappedField.reflectValue)
	oc.ChildCheckers = append(oc.ChildCheckers, c)
	if required {
		c.Required()
	} else {
		c.Optional()
	}
	return
}

func (oc *ObjectChecker) getFieldValue(fieldName string) (wrapped *fieldWrapper) {
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
		if !wrapped.reflectValue.IsValid() || !wrapped.reflectValue.CanInterface() {
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
		return fmt.Errorf("unknown field %s on %s", fieldName, oc.baseReflectValue.Type())
	}
	if !field.IsExported() {
		return fmt.Errorf("field %s on %s is unexported", fieldName, oc.baseReflectValue.Type())
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
		oc.failF("object must be a struct or a map with string keys (got %T)", object)
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
	validatorInterface := reflect.TypeOf((*Validator)(nil)).Elem()

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

	if !validatedByDirectCall && currentValue.Kind() != reflect.Ptr && currentValue.CanAddr() {
		ptrValue := currentValue.Addr()
		if ptrValue.Type().Implements(validatorInterface) && ptrValue.CanInterface() {
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
			if locErrs := validateRecursive(fieldLabel, fieldValue); len(locErrs) > 0 {
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

			if locErrs := validateRecursive(mapLabel+"(key)", key); len(locErrs) > 0 {
				errs = append(errs, locErrs...)
			}
			if locErrs := validateRecursive(mapLabel+"(value)", val); len(locErrs) > 0 {
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
			if locErrs := validateRecursive(elemLabel, elemValue); len(locErrs) > 0 {
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
	isMapWithStrKeysLike := isMapLike && base.Type().Key().Kind() == reflect.String
	return typeState{
		reflectValue:         reflectValue,
		isStructLike:         base.Kind() == reflect.Struct,
		isMapLike:            isMapLike,
		isMapWithStrKeysLike: isMapWithStrKeysLike,
		isSliceOrArrayLike:   base.Kind() == reflect.Slice || base.Kind() == reflect.Array,
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

	validatorInterface := reflect.TypeOf((*Validator)(nil)).Elem()

	canCallDirectly := false
	if v.Type().Implements(validatorInterface) {
		canCallDirectly = true
	} else if v.CanAddr() && reflect.PointerTo(v.Type()).Implements(validatorInterface) {
		canCallDirectly = true
	}

	if !canCallDirectly && v.Kind() != reflect.Ptr && reflect.PointerTo(v.Type()).Implements(validatorInterface) {
		copyPtr := reflect.New(v.Type())
		copyPtr.Elem().Set(v)
		effectiveValue = copyPtr
	}

	if errs := validateRecursive(label, effectiveValue); len(errs) > 0 {
		return &ValidationError{Err: errors.Join(errs...)}
	}

	return nil
}
