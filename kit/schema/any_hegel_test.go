package schema_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"hegel.dev/go/hegel"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_any_holder struct {
	V any
}

type property_any_case struct {
	value_mode      int
	must_not_be_nil bool
	default_mode    int
	validate_mode   int
}

type property_any_case_space struct{}

type property_any_outcome struct {
	value_type    string
	value_text    string
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestAnyRuleMatchesModel(t *testing.T) {
	t.Run("generated_dynamic_nil_and_default_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_any_case_space{}).draw_case(ht)
		tc.assert_matches_model(ht)
	}, hegel.WithTestCases(750)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_any_case_space) draw_case(ht *hegel.T) property_any_case {
	return property_any_case{
		value_mode:      hegel.Draw(ht, hegel.Integers(0, 8)),
		must_not_be_nil: hegel.Draw(ht, hegel.Booleans()),
		default_mode:    hegel.Draw(ht, hegel.Integers(0, 2)),
		validate_mode:   hegel.Draw(ht, hegel.Integers(0, 3)),
	}
}

func (tc property_any_case) assert_matches_model(ht *hegel.T) {
	tc.note(ht)

	input := tc.holder()
	result, err := schema.Enforce("holder", &input, tc.schema())

	actual := tc.outcome(result.Value.V, err)
	expected := tc.expected_outcome()

	if !actual.equal(expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}
	if tc.value_outcome(input.V).value_type != expected.value_type ||
		tc.value_outcome(input.V).value_text != expected.value_text {
		ht.Fatalf(
			"mutated input = %#v, expected value = %#v",
			tc.value_outcome(input.V),
			expected,
		)
	}
}

func (tc property_any_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {value_mode:%d must_not_be_nil:%t default_mode:%d validate_mode:%d}",
			tc.value_mode,
			tc.must_not_be_nil,
			tc.default_mode,
			tc.validate_mode,
		),
	)
}

func (tc property_any_case) schema() schema.Schema {
	any_rule := schema.Any{
		MustNotBeNil: tc.must_not_be_nil,
		ValidateFunc: tc.validate_func(),
	}
	if default_value, ok := tc.default_value(); ok {
		any_rule.DefaultIfNil = default_value
	}
	return schema.Object{"V": any_rule}
}

func (tc property_any_case) holder() property_any_holder {
	return property_any_holder{V: tc.input_value()}
}

func (tc property_any_case) input_value() any {
	switch tc.value_mode {
	case 1:
		var value *int
		return value
	case 2:
		var value *string
		return value
	case 3:
		return 0
	case 4:
		return 2
	case 5:
		value := 3
		return &value
	case 6:
		return ""
	case 7:
		return "hello"
	case 8:
		value := "hello"
		return &value
	default:
		return nil
	}
}

func (tc property_any_case) default_value() (any, bool) {
	switch tc.default_mode {
	case 1:
		return 2, true
	case 2:
		return "fallback", true
	default:
		return nil, false
	}
}

func (tc property_any_case) validate_func() func(any) error {
	switch tc.validate_mode {
	case 1:
		return func(v any) error {
			n, ok := v.(int)
			if !ok {
				return errors.New("expected int")
			}
			if n%2 != 0 {
				return errors.New("must be even")
			}
			return nil
		}
	case 2:
		return func(v any) error {
			s, ok := v.(string)
			if !ok {
				return errors.New("expected string")
			}
			if s == "" {
				return errors.New("must not be empty")
			}
			return nil
		}
	case 3:
		return func(v any) error {
			if reflect.TypeOf(v) == nil {
				return nil
			}
			if reflect.TypeOf(v).Kind() != reflect.Int && reflect.TypeOf(v).Kind() != reflect.String {
				return errors.New("expected scalar leaf")
			}
			return nil
		}
	default:
		return nil
	}
}

func (tc property_any_case) expected_outcome() property_any_outcome {
	value, present := tc.resolved_leaf()
	outcome := tc.value_outcome(tc.stored_value())

	if !present {
		if tc.must_not_be_nil {
			outcome.has_error = true
			outcome.error_message = "holder.V must not be nil"
			outcome.is_validation = true
		}
		return outcome
	}
	if validate_err := tc.validate(value); validate_err != nil {
		outcome.has_error = true
		outcome.error_message = fmt.Sprintf("holder.V: %s", validate_err.Error())
		outcome.is_validation = true
	}
	return outcome
}

func (tc property_any_case) stored_value() any {
	value := tc.input_value()
	switch typed := value.(type) {
	case nil:
		if default_value, ok := tc.default_value(); ok {
			return default_value
		}
		return nil
	case *int:
		if typed == nil {
			if default_value, ok := tc.default_value(); ok {
				return default_value
			}
		}
		return value
	case *string:
		if typed == nil {
			if default_value, ok := tc.default_value(); ok {
				return default_value
			}
		}
		return value
	default:
		return value
	}
}

func (tc property_any_case) outcome(value any, err error) property_any_outcome {
	outcome := tc.value_outcome(value)
	if err == nil {
		return outcome
	}
	outcome.has_error = true
	outcome.error_message = err.Error()
	outcome.is_validation = schema.IsValidationError(err)
	outcome.is_schema = schema.IsSchemaError(err)
	return outcome
}

func (tc property_any_case) resolved_leaf() (any, bool) {
	value := tc.input_value()
	switch typed := value.(type) {
	case nil:
		if default_value, ok := tc.default_value(); ok {
			return default_value, true
		}
		return nil, false
	case *int:
		if typed == nil {
			if default_value, ok := tc.default_value(); ok {
				return default_value, true
			}
			return nil, false
		}
		return *typed, true
	case *string:
		if typed == nil {
			if default_value, ok := tc.default_value(); ok {
				return default_value, true
			}
			return nil, false
		}
		return *typed, true
	default:
		return value, true
	}
}

func (tc property_any_case) validate(value any) error {
	validate_func := tc.validate_func()
	if validate_func == nil {
		return nil
	}
	return validate_func(value)
}

func (tc property_any_case) value_outcome(value any) property_any_outcome {
	outcome := property_any_outcome{}
	if value == nil {
		return outcome
	}
	outcome.value_type = reflect.TypeOf(value).String()
	outcome.value_text = tc.format_value(value)
	return outcome
}

func (tc property_any_case) format_value(value any) string {
	switch typed := value.(type) {
	case *int:
		if typed == nil {
			return "(*int)(nil)"
		}
		return fmt.Sprintf("&%d", *typed)
	case *string:
		if typed == nil {
			return "(*string)(nil)"
		}
		return fmt.Sprintf("&%q", *typed)
	default:
		return fmt.Sprintf("%#v", value)
	}
}

func (o property_any_outcome) equal(other property_any_outcome) bool {
	return o.value_type == other.value_type &&
		o.value_text == other.value_text &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}
