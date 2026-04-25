package schema_test

import (
	"fmt"
	"reflect"
	"testing"

	"hegel.dev/go/hegel"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_discovery_case struct {
	input     string
	depth     int
	root_mode int
}

type property_discovery_case_space struct{}

type property_discovery_outcome struct {
	value_type    string
	value_text    string
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

var property_discovery_inputs = []string{
	"  FOO@BAR.COM  ",
	" hello@example.com ",
	"",
	"  ",
	" BAR@example.com",
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestDiscoveryWrappedPointerMatchesShallowDiscovery(t *testing.T) {
	t.Run("within_depth_budget", hegel.Case(func(ht *hegel.T) {
		tc := (property_discovery_case_space{}).draw_case(ht, 48)
		tc.assert_wrapped_pointer_matches_shallow_discovery(ht)
	}, hegel.WithTestCases(500)))
}

func TestDiscoveryWrappedBoxedValueMatchesShallowDiscovery(t *testing.T) {
	t.Run("within_depth_budget", hegel.Case(func(ht *hegel.T) {
		tc := (property_discovery_case_space{}).draw_case(ht, 32)
		tc.assert_wrapped_boxed_value_matches_shallow_discovery(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_discovery_case_space) draw_case(
	ht *hegel.T,
	max_depth int,
) property_discovery_case {
	return property_discovery_case{
		input:     hegel.Draw(ht, hegel.SampledFrom(property_discovery_inputs)),
		depth:     hegel.Draw(ht, hegel.Integers(0, max_depth)),
		root_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
	}
}

func (tc property_discovery_case) assert_wrapped_pointer_matches_shallow_discovery(
	ht *hegel.T,
) {
	tc.note(ht)

	direct_leaf := wrapped_email(tc.input)
	direct_root := tc.wrap(&direct_leaf)
	_, direct_err := schema.EnforceAny("value", direct_root, tc.root())
	direct_outcome := tc.error_outcome(direct_err)

	wrapped_leaf := wrapped_email(tc.input)
	wrapped_root := tc.wrap_with_extra_layers(&wrapped_leaf)
	_, wrapped_err := schema.EnforceAny("value", wrapped_root, tc.root())
	wrapped_outcome := tc.error_outcome(wrapped_err)

	if !wrapped_outcome.equal(direct_outcome) {
		ht.Fatalf(
			"wrapped outcome = %#v, direct outcome = %#v",
			wrapped_outcome,
			direct_outcome,
		)
	}
	if wrapped_leaf != direct_leaf {
		ht.Fatalf("wrapped leaf = %q, direct leaf = %q", wrapped_leaf, direct_leaf)
	}
}

func (tc property_discovery_case) assert_wrapped_boxed_value_matches_shallow_discovery(
	ht *hegel.T,
) {
	tc.note(ht)

	var direct_box any = wrapped_email(tc.input)
	direct_root := tc.wrap_box(&direct_box)
	_, direct_err := schema.EnforceAny("value", direct_root, tc.root())
	direct_outcome := tc.error_outcome(direct_err)
	direct_box_outcome := tc.value_outcome(direct_box)

	var wrapped_box any = wrapped_email(tc.input)
	wrapped_root := tc.wrap_box_with_extra_layers(&wrapped_box)
	_, wrapped_err := schema.EnforceAny("value", wrapped_root, tc.root())
	wrapped_outcome := tc.error_outcome(wrapped_err)
	wrapped_box_outcome := tc.value_outcome(wrapped_box)

	if !wrapped_outcome.equal(direct_outcome) {
		ht.Fatalf(
			"wrapped outcome = %#v, direct outcome = %#v",
			wrapped_outcome,
			direct_outcome,
		)
	}
	if !wrapped_box_outcome.equal(direct_box_outcome) {
		ht.Fatalf(
			"wrapped boxed outcome = %#v, direct boxed outcome = %#v",
			wrapped_box_outcome,
			direct_box_outcome,
		)
	}
}

func (tc property_discovery_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {input:%q depth:%d root_mode:%d}",
			tc.input,
			tc.depth,
			tc.root_mode,
		),
	)
}

func (tc property_discovery_case) root() schema.Schema {
	switch tc.root_mode {
	case 1:
		return schema.String{MustNotBeZero: true}
	case 2:
		return schema.String{MustBeIn: []string{"foo@bar.com", "bar@example.com"}}
	default:
		return nil
	}
}

func (tc property_discovery_case) wrap(leaf *wrapped_email) any {
	return leaf
}

func (tc property_discovery_case) wrap_with_extra_layers(leaf *wrapped_email) any {
	var value any = leaf
	for range tc.depth {
		wrap := value
		value = &wrap
	}
	return value
}

func (tc property_discovery_case) wrap_box(box *any) any {
	return box
}

func (tc property_discovery_case) wrap_box_with_extra_layers(box *any) any {
	var value any = box
	for range tc.depth {
		wrap := value
		value = &wrap
	}
	return value
}

func (tc property_discovery_case) error_outcome(
	err error,
) property_discovery_outcome {
	outcome := property_discovery_outcome{}
	if err == nil {
		return outcome
	}
	outcome.has_error = true
	outcome.error_message = err.Error()
	outcome.is_validation = schema.IsValidationError(err)
	outcome.is_schema = schema.IsSchemaError(err)
	return outcome
}

func (tc property_discovery_case) value_outcome(value any) property_discovery_outcome {
	return property_discovery_outcome{
		value_type: reflect.TypeOf(value).String(),
		value_text: tc.format_value(value),
	}
}

func (tc property_discovery_case) format_value(value any) string {
	switch typed := value.(type) {
	case wrapped_email:
		return string(typed)
	case *wrapped_email:
		if typed == nil {
			return "<nil>"
		}
		return string(*typed)
	default:
		return fmt.Sprintf("%#v", value)
	}
}

func (o property_discovery_outcome) equal(other property_discovery_outcome) bool {
	return o.value_type == other.value_type &&
		o.value_text == other.value_text &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}
