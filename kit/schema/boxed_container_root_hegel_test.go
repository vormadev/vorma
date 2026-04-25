package schema_test

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"hegel.dev/go/hegel"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_boxed_root_slice_direct_holder struct {
	V []string
}

type property_boxed_root_map_direct_holder struct {
	V map[string]string
}

type property_boxed_root_slice_case struct {
	nil_input         bool
	values            []string
	default_mode      int
	element_rule_mode int
	min_mode          int
	validate_mode     int
}

type property_boxed_root_map_case struct {
	nil_input       bool
	entries         map[string]string
	default_mode    int
	key_rule_mode   int
	value_rule_mode int
	min_mode        int
	validate_mode   int
}

type property_boxed_root_slice_case_space struct{}

type property_boxed_root_map_case_space struct{}

type property_boxed_container_outcome struct {
	value_type    string
	value_text    string
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

var property_boxed_root_slice_values = []string{
	"",
	"  A  ",
	"  B",
	"c  ",
	"  guest  ",
}

var property_boxed_root_map_keys = []string{
	" A ",
	"a",
	" B",
	"b ",
	" C ",
}

var property_boxed_root_map_values = []string{
	"",
	" one ",
	"TWO",
	" three ",
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestExplicitRootSliceOnBoxedValueMatchesDirectField(t *testing.T) {
	t.Run("generated_slice_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_boxed_root_slice_case_space{}).draw_case(ht)
		tc.assert_boxed_matches_direct_field(ht)
	}, hegel.WithTestCases(500)))
}

func TestExplicitRootMapOnBoxedValueMatchesDirectField(t *testing.T) {
	t.Run("generated_map_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_boxed_root_map_case_space{}).draw_case(ht, 5)
		tc.assert_boxed_matches_direct_field(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_boxed_root_slice_case_space) draw_case(
	ht *hegel.T,
) property_boxed_root_slice_case {
	value_count := hegel.Draw(ht, hegel.Integers(0, 4))
	values := make([]string, 0, value_count)
	for range value_count {
		values = append(values, hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_slice_values)))
	}
	return property_boxed_root_slice_case{
		nil_input:         hegel.Draw(ht, hegel.Booleans()),
		values:            values,
		default_mode:      hegel.Draw(ht, hegel.Integers(0, 1)),
		element_rule_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
		min_mode:          hegel.Draw(ht, hegel.Integers(0, 1)),
		validate_mode:     hegel.Draw(ht, hegel.Integers(0, 1)),
	}
}

func (property_boxed_root_map_case_space) draw_case(
	ht *hegel.T,
	max_entries int,
) property_boxed_root_map_case {
	entry_count := hegel.Draw(ht, hegel.Integers(0, max_entries))
	entries := make(map[string]string, entry_count)
	for range entry_count {
		key := hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_map_keys))
		value := hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_map_values))
		entries[key] = value
	}
	return property_boxed_root_map_case{
		nil_input:       hegel.Draw(ht, hegel.Booleans()),
		entries:         entries,
		default_mode:    hegel.Draw(ht, hegel.Integers(0, 1)),
		key_rule_mode:   hegel.Draw(ht, hegel.Integers(0, 1)),
		value_rule_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
		min_mode:        hegel.Draw(ht, hegel.Integers(0, 1)),
		validate_mode:   hegel.Draw(ht, hegel.Integers(0, 1)),
	}
}

func (tc property_boxed_root_slice_case) assert_boxed_matches_direct_field(
	ht *hegel.T,
) {
	tc.note(ht)

	direct_holder := property_boxed_root_slice_direct_holder{V: tc.direct_value()}
	_, direct_err := schema.Enforce("holder", &direct_holder, tc.schema())
	direct_outcome := tc.direct_outcome(direct_holder.V, direct_err)

	boxed_holder := property_boxed_root_boxed_holder{V: tc.boxed_value()}
	_, boxed_err := schema.Enforce("holder", &boxed_holder, tc.schema())
	boxed_outcome := tc.boxed_outcome(boxed_holder.V, boxed_err)

	if !boxed_outcome.equal(direct_outcome) {
		ht.Fatalf("boxed outcome = %#v, direct outcome = %#v", boxed_outcome, direct_outcome)
	}
}

func (tc property_boxed_root_map_case) assert_boxed_matches_direct_field(
	ht *hegel.T,
) {
	tc.note(ht)

	direct_holder := property_boxed_root_map_direct_holder{V: tc.direct_value()}
	_, direct_err := schema.Enforce("holder", &direct_holder, tc.schema())
	direct_outcome := tc.direct_outcome(direct_holder.V, direct_err)

	boxed_holder := property_boxed_root_boxed_holder{V: tc.boxed_value()}
	_, boxed_err := schema.Enforce("holder", &boxed_holder, tc.schema())
	boxed_outcome := tc.boxed_outcome(boxed_holder.V, boxed_err)

	if !boxed_outcome.equal(direct_outcome) {
		ht.Fatalf("boxed outcome = %#v, direct outcome = %#v", boxed_outcome, direct_outcome)
	}
}

func (tc property_boxed_root_slice_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {nil_input:%t values:%#v default_mode:%d element_rule_mode:%d min_mode:%d validate_mode:%d}",
			tc.nil_input,
			tc.values,
			tc.default_mode,
			tc.element_rule_mode,
			tc.min_mode,
			tc.validate_mode,
		),
	)
}

func (tc property_boxed_root_map_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {nil_input:%t entries:%#v default_mode:%d key_rule_mode:%d value_rule_mode:%d min_mode:%d validate_mode:%d}",
			tc.nil_input,
			tc.entries,
			tc.default_mode,
			tc.key_rule_mode,
			tc.value_rule_mode,
			tc.min_mode,
			tc.validate_mode,
		),
	)
}

func (tc property_boxed_root_slice_case) schema() schema.Schema {
	list_rule := schema.List{
		ElementSchema: tc.element_rule(),
	}
	if default_value, ok := tc.default_value(); ok {
		list_rule.DefaultIfNil = default_value
	}
	if tc.min_mode == 1 {
		list_rule.MinLen = 1
	}
	if tc.validate_mode == 1 {
		list_rule.ValidateFunc = func(n int) error {
			if n%2 != 0 {
				return errors.New("length must be even")
			}
			return nil
		}
	}
	return schema.Object{"V": list_rule}
}

func (tc property_boxed_root_map_case) schema() schema.Schema {
	map_rule := schema.Map{
		KeySchema:   tc.key_rule(),
		ValueSchema: tc.value_rule(),
	}
	if default_value, ok := tc.default_value(); ok {
		map_rule.DefaultIfNil = default_value
	}
	if tc.min_mode == 1 {
		map_rule.MinLen = 1
	}
	if tc.validate_mode == 1 {
		map_rule.ValidateFunc = func(n int) error {
			if n%2 != 0 {
				return errors.New("length must be even")
			}
			return nil
		}
	}
	return schema.Object{"V": map_rule}
}

func (tc property_boxed_root_slice_case) direct_value() []string {
	if tc.nil_input {
		return nil
	}
	return append([]string(nil), tc.values...)
}

func (tc property_boxed_root_slice_case) boxed_value() any {
	value := tc.direct_value()
	return value
}

func (tc property_boxed_root_map_case) direct_value() map[string]string {
	if tc.nil_input {
		return nil
	}
	value := make(map[string]string, len(tc.entries))
	for key, entry_value := range tc.entries {
		value[key] = entry_value
	}
	return value
}

func (tc property_boxed_root_map_case) boxed_value() any {
	value := tc.direct_value()
	return value
}

func (tc property_boxed_root_slice_case) default_value() (any, bool) {
	if tc.default_mode == 0 {
		return nil, false
	}
	return []string{"  guest  ", ""}, true
}

func (tc property_boxed_root_map_case) default_value() (any, bool) {
	if tc.default_mode == 0 {
		return nil, false
	}
	return map[string]string{" A ": " one "}, true
}

func (tc property_boxed_root_slice_case) element_rule() schema.Schema {
	switch tc.element_rule_mode {
	case 1:
		return schema.String{TrimSpace: true, ToLower: true}
	case 2:
		return schema.String{
			TrimSpace:     true,
			ToLower:       true,
			DefaultIfZero: "guest",
		}
	default:
		return schema.String{}
	}
}

func (tc property_boxed_root_map_case) key_rule() schema.Schema {
	if tc.key_rule_mode == 1 {
		return schema.String{TrimSpace: true, ToLower: true}
	}
	return nil
}

func (tc property_boxed_root_map_case) value_rule() schema.Schema {
	switch tc.value_rule_mode {
	case 1:
		return schema.String{TrimSpace: true, ToLower: true}
	case 2:
		return schema.String{
			TrimSpace:     true,
			ToLower:       true,
			DefaultIfZero: "guest",
		}
	default:
		return nil
	}
}

func (tc property_boxed_root_slice_case) direct_outcome(
	value []string,
	err error,
) property_boxed_container_outcome {
	outcome := property_boxed_container_outcome{
		value_type: reflect.TypeOf(value).String(),
		value_text: tc.format_slice(value),
	}
	return outcome.with_error(err)
}

func (tc property_boxed_root_slice_case) boxed_outcome(
	value any,
	err error,
) property_boxed_container_outcome {
	outcome := property_boxed_container_outcome{}
	if value != nil {
		outcome.value_type = reflect.TypeOf(value).String()
	}
	if typed, ok := value.([]string); ok {
		outcome.value_text = tc.format_slice(typed)
	}
	return outcome.with_error(err)
}

func (tc property_boxed_root_map_case) direct_outcome(
	value map[string]string,
	err error,
) property_boxed_container_outcome {
	outcome := property_boxed_container_outcome{
		value_type: reflect.TypeOf(value).String(),
		value_text: tc.format_map(value),
	}
	return outcome.with_error(err)
}

func (tc property_boxed_root_map_case) boxed_outcome(
	value any,
	err error,
) property_boxed_container_outcome {
	outcome := property_boxed_container_outcome{}
	if value != nil {
		outcome.value_type = reflect.TypeOf(value).String()
	}
	if typed, ok := value.(map[string]string); ok {
		outcome.value_text = tc.format_map(typed)
	}
	return outcome.with_error(err)
}

func (tc property_boxed_root_slice_case) format_slice(value []string) string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%#v", value)
}

func (tc property_boxed_root_map_case) format_map(value map[string]string) string {
	if value == nil {
		return "<nil>"
	}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%q=%q", key, value[key]))
	}
	return strings.Join(parts, ",")
}

func (o property_boxed_container_outcome) with_error(
	err error,
) property_boxed_container_outcome {
	if err == nil {
		return o
	}
	o.has_error = true
	o.error_message = err.Error()
	o.is_validation = schema.IsValidationError(err)
	o.is_schema = schema.IsSchemaError(err)
	return o
}

func (o property_boxed_container_outcome) equal(
	other property_boxed_container_outcome,
) bool {
	return o.value_type == other.value_type &&
		o.value_text == other.value_text &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}
