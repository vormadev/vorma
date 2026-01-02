package validate

import (
	"fmt"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/vormadev/vorma/kit/set"
)

func (c *AnyChecker) If(condition bool, f func(*AnyChecker) *AnyChecker) *AnyChecker {
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
	if reflect.DeepEqual(a.Interface(), b.Interface()) {
		return true
	}

	aKind := a.Kind()
	bKind := b.Kind()

	if aKind == bKind {
		switch aKind {
		case reflect.String:
			return a.String() == b.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return a.Int() == b.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
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
	base := safeDereference(reflect.ValueOf(valuesSlice))
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
	trueBaseReflect := safeDereference(reflect.ValueOf(c.trueValue))
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

func (oc *ObjectChecker) MutuallyExclusive(label string, fields ...string) *ObjectChecker {
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

func (oc *ObjectChecker) MutuallyRequired(label string, fields ...string) *ObjectChecker {
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

func (oc *ObjectChecker) validateFieldGroupConstraint(label string, fields []string, constraintFn constraintFn) *ObjectChecker {
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

func (oc *ObjectChecker) validateFieldGroup(fieldNames []string) (set.Set[string], int) {
	truthySet := set.New[string]()
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
	allowedCharsSet := set.New[rune]()
	for _, char := range allowedChars {
		allowedCharsSet.Add(char)
	}
	for _, char := range str {
		if !allowedCharsSet.Contains(char) {
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
		return fmt.Sprintf("minimum permitted %s for %s is %v, got %v", typeName, c.label, min, val)
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
		return fmt.Sprintf("maximum permitted %s for %s is %v, got %v", typeName, c.label, max, val)
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
		return fmt.Sprintf("permitted %s range for %s is [%v, %v], got %v", typeName, c.label, min, max, val)
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
		return fmt.Sprintf("permitted %s range for %s is (%v, %v), got %v", typeName, c.label, min, max, val)
	}
	return c.validateNumeric(f1, f2)
}

type checkFn func(float64) bool
type getErrorMsg func(typeName string, val float64) string

func (c *AnyChecker) validateNumeric(checkFn checkFn, getErrorMsg getErrorMsg) *AnyChecker {
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

func extractNumericFromReflectValue(value reflect.Value) (trueValue float64, nature string, ok bool) {
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(value.Int()), "value", true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(value.Uint()), "value", true
	case reflect.Float32, reflect.Float64:
		return value.Float(), "value", true
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return float64(value.Len()), "length", true
	}
	return 0, "", false
}
