package schema_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"hegel.dev/go/hegel"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_schema_form struct {
	Email           string
	Username        string
	Password        string
	PasswordConfirm string
	Country         string
	StateCode       string
	Priority        int
	Bio             string
	Nickname        *string
}

type property_schema_case struct {
	form property_schema_form
}

type property_schema_map_case struct {
	entries map[string]string
}

type property_map_length_case struct {
	entries       map[string]string
	min_mode      int
	max_mode      int
	validate_mode int
}

type property_string_rule_holder struct {
	Value *string
}

type property_string_rule_case struct {
	input               *string
	trim_space          bool
	case_mode           int
	transform_mode      int
	skip_mode           int
	validate_mode       int
	must_not_be_nil     bool
	must_not_be_zero    bool
	has_default_if_nil  bool
	default_if_nil      string
	has_default_if_zero bool
	default_if_zero     string
}

type property_int_rule_holder struct {
	Value *int
}

type property_int_rule_case struct {
	input               *int
	skip_mode           int
	validate_mode       int
	must_not_be_nil     bool
	must_not_be_zero    bool
	has_default_if_nil  bool
	default_if_nil      int
	has_default_if_zero bool
	default_if_zero     int
	min_mode            int
	max_mode            int
}

type property_object_pipeline_form struct {
	Name  string
	Alias string
	Level int
}

type property_object_pipeline_case struct {
	form           property_object_pipeline_form
	transform_mode int
	validate_mode  int
}

type property_schema_snapshot struct {
	email            string
	username         string
	password         string
	password_confirm string
	country          string
	state_code       string
	priority         int
	bio              string
	nickname         string
	has_nickname     bool
}

type property_schema_outcome struct {
	snapshot      property_schema_snapshot
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

type property_map_length_outcome struct {
	length        int
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

type property_string_rule_snapshot struct {
	has_value bool
	value     string
}

type property_string_rule_outcome struct {
	snapshot      property_string_rule_snapshot
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

type property_int_rule_snapshot struct {
	has_value bool
	value     int
}

type property_int_rule_outcome struct {
	snapshot      property_int_rule_snapshot
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

type property_object_pipeline_snapshot struct {
	name  string
	alias string
	level int
}

type property_object_pipeline_outcome struct {
	snapshot      property_object_pipeline_snapshot
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

type property_schema_case_space struct{}

type property_schema_map_case_space struct{}

type property_map_length_case_space struct{}

type property_string_rule_case_space struct{}

type property_int_rule_case_space struct{}

type property_object_pipeline_case_space struct{}

var property_schema_emails = []string{
	"  JOE@Example.com  ",
	"USER@SITE.NET",
	"bad",
	"",
	"  ",
}

var property_schema_usernames = []string{
	"  joe_123  ",
	"Valid_Name",
	"A",
	"!!!!",
	"  ",
	"space name",
}

var property_schema_passwords = []string{
	"password",
	"supersecret",
	"short",
	"",
	"        ",
}

var property_schema_password_confirms = []string{
	"password",
	" password ",
	"supersecret",
	"  supersecret  ",
	"mismatch",
	"",
}

var property_schema_countries = []string{
	"us",
	"ca",
	"zz",
	"",
	" US ",
	"  ca ",
}

var property_schema_state_codes = []string{
	"ca",
	" cal ",
	"ny",
	"x",
	"",
	"  ",
}

var property_schema_priorities = []int{-1, 0, 1, 2, 3, 5, 6}

var property_schema_bios = []string{
	"",
	"  ",
	"hello",
	"  note  ",
	"this is definitely too long for the configured max len",
}

var property_schema_nicknames = []string{
	"",
	"  Bob  ",
	"A",
	"Guest",
	"  AL  ",
}

var property_schema_map_keys = []string{
	"FOO",
	"foo",
	" Foo ",
	"BAR",
	" bar",
	"baz ",
	"  BAZ  ",
	"qux",
}

var property_schema_map_values = []string{
	" one ",
	"TWO",
	" three ",
	"",
	" four",
}

var property_string_rule_inputs = []string{
	"",
	" ",
	"skip",
	"  skip  ",
	"hello",
	"  Hello  ",
	"erase",
	"  erase  ",
	"bang",
}

var property_string_rule_defaults = []string{
	"guest",
	"  Guest  ",
	"fallback",
	"  FALLBACK  ",
}

var property_int_rule_inputs = []int{-1, 0, 1, 2, 4}

var property_int_rule_defaults = []int{0, 1, 2, 3}

var property_object_pipeline_names = []string{
	"",
	"  Alice  ",
	"ADMIN",
	" guest ",
	"  ",
}

var property_object_pipeline_aliases = []string{
	"",
	"  ",
	"  Sidekick  ",
	"helper",
	"ROOT",
}

var property_object_pipeline_levels = []int{0, 1, 2, 5}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestEnforcePointerAndValueInputsAgree(t *testing.T) {
	t.Run("rich_fixture", hegel.Case(func(ht *hegel.T) {
		tc := (property_schema_case_space{}).draw_case(ht)
		tc.assert_pointer_and_value_inputs_agree(ht)
	}, hegel.WithTestCases(500)))
}

func TestEnforcePointerInputIsIdempotent(t *testing.T) {
	t.Run("rich_fixture", hegel.Case(func(ht *hegel.T) {
		tc := (property_schema_case_space{}).draw_case(ht)
		tc.assert_pointer_input_is_idempotent(ht)
	}, hegel.WithTestCases(500)))
}

func TestMapNormalizationMatchesModel(t *testing.T) {
	t.Run("key_and_value_normalization", hegel.Case(func(ht *hegel.T) {
		tc := (property_schema_map_case_space{}).draw_case(ht, 8)
		tc.assert_map_normalization_matches_model(ht)
	}, hegel.WithTestCases(500)))
}

func TestMapLengthChecksMatchNormalizedMap(t *testing.T) {
	t.Run("key_normalization_collisions", hegel.Case(func(ht *hegel.T) {
		tc := (property_map_length_case_space{}).draw_case(ht, 8)
		tc.assert_length_checks_match_normalized_map(ht)
	}, hegel.WithTestCases(500)))
}

func TestEnforceFreshValueRunsAreDeterministic(t *testing.T) {
	t.Run("rich_fixture", hegel.Case(func(ht *hegel.T) {
		tc := (property_schema_case_space{}).draw_case(ht)
		tc.assert_fresh_value_runs_are_deterministic(ht)
	}, hegel.WithTestCases(500)))
}

func TestStringRulePipelineMatchesModel(t *testing.T) {
	t.Run("generated_rule_and_input_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_string_rule_case_space{}).draw_case(ht)
		tc.assert_pipeline_matches_model(ht)
	}, hegel.WithTestCases(750)))
}

func TestIntRulePipelineMatchesModel(t *testing.T) {
	t.Run("generated_rule_and_input_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_int_rule_case_space{}).draw_case(ht)
		tc.assert_pipeline_matches_model(ht)
	}, hegel.WithTestCases(750)))
}

func TestObjectPipelineMatchesModel(t *testing.T) {
	t.Run("generated_object_pipeline", hegel.Case(func(ht *hegel.T) {
		tc := (property_object_pipeline_case_space{}).draw_case(ht)
		tc.assert_pipeline_matches_model(ht)
	}, hegel.WithTestCases(750)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_schema_case_space) draw_case(ht *hegel.T) property_schema_case {
	return property_schema_case{
		form: property_schema_form{
			Email:           hegel.Draw(ht, hegel.SampledFrom(property_schema_emails)),
			Username:        hegel.Draw(ht, hegel.SampledFrom(property_schema_usernames)),
			Password:        hegel.Draw(ht, hegel.SampledFrom(property_schema_passwords)),
			PasswordConfirm: hegel.Draw(ht, hegel.SampledFrom(property_schema_password_confirms)),
			Country:         hegel.Draw(ht, hegel.SampledFrom(property_schema_countries)),
			StateCode:       hegel.Draw(ht, hegel.SampledFrom(property_schema_state_codes)),
			Priority:        hegel.Draw(ht, hegel.SampledFrom(property_schema_priorities)),
			Bio:             hegel.Draw(ht, hegel.SampledFrom(property_schema_bios)),
			Nickname:        (property_schema_case_space{}).draw_nickname(ht),
		},
	}
}

func (property_schema_case_space) draw_nickname(ht *hegel.T) *string {
	if !hegel.Draw(ht, hegel.Booleans()) {
		return nil
	}
	nickname := hegel.Draw(ht, hegel.SampledFrom(property_schema_nicknames))
	return &nickname
}

func (property_schema_map_case_space) draw_case(
	ht *hegel.T,
	max_entries int,
) property_schema_map_case {
	entry_count := hegel.Draw(ht, hegel.Integers(0, max_entries))
	entries := make(map[string]string, entry_count)
	for range entry_count {
		key := hegel.Draw(ht, hegel.SampledFrom(property_schema_map_keys))
		value := hegel.Draw(ht, hegel.SampledFrom(property_schema_map_values))
		entries[key] = value
	}
	return property_schema_map_case{entries: entries}
}

func (property_map_length_case_space) draw_case(
	ht *hegel.T,
	max_entries int,
) property_map_length_case {
	entry_count := hegel.Draw(ht, hegel.Integers(0, max_entries))
	entries := make(map[string]string, entry_count)
	for range entry_count {
		key := hegel.Draw(ht, hegel.SampledFrom(property_schema_map_keys))
		value := hegel.Draw(ht, hegel.SampledFrom(property_schema_map_values))
		entries[key] = value
	}
	return property_map_length_case{
		entries:       entries,
		min_mode:      hegel.Draw(ht, hegel.Integers(0, 2)),
		max_mode:      hegel.Draw(ht, hegel.Integers(0, 2)),
		validate_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
	}
}

func (property_string_rule_case_space) draw_case(
	ht *hegel.T,
) property_string_rule_case {
	has_input := hegel.Draw(ht, hegel.Booleans())
	var input *string
	if has_input {
		value := hegel.Draw(ht, hegel.SampledFrom(property_string_rule_inputs))
		input = &value
	}

	tc := property_string_rule_case{
		input:           input,
		trim_space:      hegel.Draw(ht, hegel.Booleans()),
		case_mode:       hegel.Draw(ht, hegel.Integers(0, 2)),
		transform_mode:  hegel.Draw(ht, hegel.Integers(0, 2)),
		skip_mode:       hegel.Draw(ht, hegel.Integers(0, 2)),
		validate_mode:   hegel.Draw(ht, hegel.Integers(0, 2)),
		must_not_be_nil: hegel.Draw(ht, hegel.Booleans()),
		must_not_be_zero: hegel.Draw(
			ht,
			hegel.Booleans(),
		),
		has_default_if_nil:  hegel.Draw(ht, hegel.Booleans()),
		has_default_if_zero: hegel.Draw(ht, hegel.Booleans()),
	}
	if tc.has_default_if_nil {
		tc.default_if_nil = hegel.Draw(ht, hegel.SampledFrom(property_string_rule_defaults))
	}
	if tc.has_default_if_zero {
		tc.default_if_zero = hegel.Draw(ht, hegel.SampledFrom(property_string_rule_defaults))
	}
	return tc
}

func (property_int_rule_case_space) draw_case(ht *hegel.T) property_int_rule_case {
	has_input := hegel.Draw(ht, hegel.Booleans())
	var input *int
	if has_input {
		value := hegel.Draw(ht, hegel.SampledFrom(property_int_rule_inputs))
		input = &value
	}

	tc := property_int_rule_case{
		input:               input,
		skip_mode:           hegel.Draw(ht, hegel.Integers(0, 2)),
		validate_mode:       hegel.Draw(ht, hegel.Integers(0, 2)),
		must_not_be_nil:     hegel.Draw(ht, hegel.Booleans()),
		must_not_be_zero:    hegel.Draw(ht, hegel.Booleans()),
		has_default_if_nil:  hegel.Draw(ht, hegel.Booleans()),
		has_default_if_zero: hegel.Draw(ht, hegel.Booleans()),
		min_mode:            hegel.Draw(ht, hegel.Integers(0, 2)),
		max_mode:            hegel.Draw(ht, hegel.Integers(0, 2)),
	}
	if tc.has_default_if_nil {
		tc.default_if_nil = hegel.Draw(ht, hegel.SampledFrom(property_int_rule_defaults))
	}
	if tc.has_default_if_zero {
		tc.default_if_zero = hegel.Draw(ht, hegel.SampledFrom(property_int_rule_defaults))
	}
	return tc
}

func (property_object_pipeline_case_space) draw_case(
	ht *hegel.T,
) property_object_pipeline_case {
	return property_object_pipeline_case{
		form: property_object_pipeline_form{
			Name:  hegel.Draw(ht, hegel.SampledFrom(property_object_pipeline_names)),
			Alias: hegel.Draw(ht, hegel.SampledFrom(property_object_pipeline_aliases)),
			Level: hegel.Draw(ht, hegel.SampledFrom(property_object_pipeline_levels)),
		},
		transform_mode: hegel.Draw(ht, hegel.Integers(0, 3)),
		validate_mode:  hegel.Draw(ht, hegel.Integers(0, 3)),
	}
}

func (tc property_schema_case) assert_pointer_and_value_inputs_agree(ht *hegel.T) {
	tc.note(ht)

	value_input := tc.form
	value_result, value_err := schema.Enforce("form", value_input, tc.schema())

	pointer_input := tc.form
	pointer_result, pointer_err := schema.Enforce("form", &pointer_input, tc.schema())

	value_outcome := tc.outcome(value_result.Value, value_err)
	pointer_outcome := tc.outcome(*pointer_result.Value, pointer_err)

	if !value_outcome.equal(pointer_outcome) {
		ht.Fatalf(
			"value outcome = %#v, pointer outcome = %#v",
			value_outcome,
			pointer_outcome,
		)
	}
	if !pointer_input.snapshot().equal(pointer_outcome.snapshot) {
		ht.Fatalf(
			"pointer input after enforce = %#v, want %#v",
			pointer_input.snapshot(),
			pointer_outcome.snapshot,
		)
	}
}

func (tc property_schema_case) assert_pointer_input_is_idempotent(ht *hegel.T) {
	tc.note(ht)

	input := tc.form
	first_result, first_err := schema.Enforce("form", &input, tc.schema())
	first_outcome := tc.outcome(*first_result.Value, first_err)

	second_result, second_err := schema.Enforce("form", &input, tc.schema())
	second_outcome := tc.outcome(*second_result.Value, second_err)

	if !first_outcome.equal(second_outcome) {
		ht.Fatalf(
			"first outcome = %#v, second outcome = %#v",
			first_outcome,
			second_outcome,
		)
	}
	if !input.snapshot().equal(second_outcome.snapshot) {
		ht.Fatalf(
			"input after second enforce = %#v, want %#v",
			input.snapshot(),
			second_outcome.snapshot,
		)
	}
}

func (tc property_schema_map_case) assert_map_normalization_matches_model(ht *hegel.T) {
	ht.Note(fmt.Sprintf("entries = %#v", tc.entries))

	input := tc.holder()
	expected := tc.expected_entries()

	result, err := schema.Enforce("holder", &input, tc.schema())
	if err != nil {
		ht.Fatalf("unexpected error: %v", err)
	}
	if !tc.maps_equal(input.M, expected) {
		ht.Fatalf("mutated map = %#v, want %#v", input.M, expected)
	}
	if !tc.maps_equal(result.Value.M, expected) {
		ht.Fatalf("result map = %#v, want %#v", result.Value.M, expected)
	}
}

func (tc property_map_length_case) assert_length_checks_match_normalized_map(ht *hegel.T) {
	tc.note(ht)

	input := tc.holder()
	expected_entries := tc.expected_entries()
	expected_outcome := tc.expected_outcome(expected_entries)

	result, err := schema.Enforce("holder", &input, tc.schema())
	actual_outcome := tc.outcome(result.Value.M, err)

	if !expected_outcome.is_schema {
		if !tc.maps_equal(input.M, expected_entries) {
			ht.Fatalf("mutated map = %#v, want %#v", input.M, expected_entries)
		}
		if !tc.maps_equal(result.Value.M, expected_entries) {
			ht.Fatalf("result map = %#v, want %#v", result.Value.M, expected_entries)
		}
	}
	if !actual_outcome.equal(expected_outcome) {
		ht.Fatalf("actual outcome = %#v, expected outcome = %#v", actual_outcome, expected_outcome)
	}
}

func (tc property_schema_case) assert_fresh_value_runs_are_deterministic(ht *hegel.T) {
	tc.note(ht)

	first_input := tc.form.clone()
	second_input := tc.form.clone()

	first_result, first_err := schema.Enforce("form", first_input, tc.schema())
	second_result, second_err := schema.Enforce("form", second_input, tc.schema())

	first_outcome := tc.outcome(first_result.Value, first_err)
	second_outcome := tc.outcome(second_result.Value, second_err)

	if !first_outcome.equal(second_outcome) {
		ht.Fatalf(
			"first outcome = %#v, second outcome = %#v",
			first_outcome,
			second_outcome,
		)
	}
}

func (tc property_string_rule_case) assert_pipeline_matches_model(ht *hegel.T) {
	tc.note(ht)

	input := tc.holder()
	expected := tc.expected_outcome()

	result, err := schema.Enforce("holder", &input, tc.schema())
	actual := tc.outcome(result.Value, err)

	if !actual.equal(expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}
	if !input.snapshot().equal(expected.snapshot) {
		ht.Fatalf(
			"mutated input snapshot = %#v, expected = %#v",
			input.snapshot(),
			expected.snapshot,
		)
	}
}

func (tc property_int_rule_case) assert_pipeline_matches_model(ht *hegel.T) {
	tc.note(ht)

	input := tc.holder()
	expected := tc.expected_outcome()

	result, err := schema.Enforce("holder", &input, tc.schema())
	actual := tc.outcome(result.Value, err)

	if !actual.equal(expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}
	if !input.snapshot().equal(expected.snapshot) {
		ht.Fatalf(
			"mutated input snapshot = %#v, expected = %#v",
			input.snapshot(),
			expected.snapshot,
		)
	}
}

func (tc property_object_pipeline_case) assert_pipeline_matches_model(ht *hegel.T) {
	tc.note(ht)

	input := tc.form
	expected := tc.expected_outcome()

	result, err := schema.Enforce("holder", &input, tc.schema())
	actual := tc.outcome(*result.Value, err)

	if !actual.equal(expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}
	if !input.snapshot().equal(expected.snapshot) {
		ht.Fatalf(
			"mutated input snapshot = %#v, expected = %#v",
			input.snapshot(),
			expected.snapshot,
		)
	}
}

func (tc property_schema_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("input = %#v", tc.form.snapshot()))
}

func (tc property_string_rule_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("input = %#v", tc.snapshot()))
	ht.Note(
		fmt.Sprintf(
			"rule = {trim_space:%t case_mode:%d transform_mode:%d skip_mode:%d validate_mode:%d must_not_be_nil:%t must_not_be_zero:%t has_default_if_nil:%t default_if_nil:%q has_default_if_zero:%t default_if_zero:%q}",
			tc.trim_space,
			tc.case_mode,
			tc.transform_mode,
			tc.skip_mode,
			tc.validate_mode,
			tc.must_not_be_nil,
			tc.must_not_be_zero,
			tc.has_default_if_nil,
			tc.default_if_nil,
			tc.has_default_if_zero,
			tc.default_if_zero,
		),
	)
}

func (tc property_map_length_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("entries = %#v", tc.entries))
	ht.Note(
		fmt.Sprintf(
			"rule = {min_mode:%d max_mode:%d validate_mode:%d}",
			tc.min_mode,
			tc.max_mode,
			tc.validate_mode,
		),
	)
}

func (tc property_int_rule_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("input = %#v", tc.snapshot()))
	ht.Note(
		fmt.Sprintf(
			"rule = {skip_mode:%d validate_mode:%d must_not_be_nil:%t must_not_be_zero:%t has_default_if_nil:%t default_if_nil:%d has_default_if_zero:%t default_if_zero:%d min_mode:%d max_mode:%d}",
			tc.skip_mode,
			tc.validate_mode,
			tc.must_not_be_nil,
			tc.must_not_be_zero,
			tc.has_default_if_nil,
			tc.default_if_nil,
			tc.has_default_if_zero,
			tc.default_if_zero,
			tc.min_mode,
			tc.max_mode,
		),
	)
}

func (tc property_object_pipeline_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("input = %#v", tc.form.snapshot()))
	ht.Note(
		fmt.Sprintf(
			"pipeline = {transform_mode:%d validate_mode:%d}",
			tc.transform_mode,
			tc.validate_mode,
		),
	)
}

func (tc property_schema_case) schema() schema.Schema {
	return schema.Object{
		"Email": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MustBeEmail:   true,
		},
		"Username": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MinLen:        3,
			MaxLen:        20,
			AllowedChars:  "abcdefghijklmnopqrstuvwxyz0123456789_",
		},
		"Password": schema.String{
			MustNotBeZero: true,
			MinLen:        8,
			MaxLen:        128,
		},
		"PasswordConfirm": schema.String{
			MustNotBeZero: true,
		},
		"Country": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToUpper:       true,
			MustBeIn:      []string{"US", "CA"},
		},
		"StateCode": schema.String{
			TrimSpace: true,
			ToUpper:   true,
		},
		"Priority": schema.Int{
			DefaultIfZero: 3,
			Min:           1,
			Max:           5,
		},
		"Bio": schema.String{
			TrimSpace:     true,
			MaxLen:        20,
			DefaultIfZero: "no bio provided",
		},
		"Nickname": schema.String{
			DefaultIfNil: "guest",
			TrimSpace:    true,
			ToLower:      true,
			MinLen:       2,
			MaxLen:       12,
		},
		schema.TransformFunc: func(v property_schema_form) (property_schema_form, error) {
			v.PasswordConfirm = strings.TrimSpace(v.PasswordConfirm)
			return v, nil
		},
		schema.ValidateFunc: func(v property_schema_form) error {
			if v.PasswordConfirm != v.Password {
				return errors.New("passwords must match")
			}
			if v.Country == "US" && len(v.StateCode) != 2 {
				return errors.New("state code must be 2 characters for US")
			}
			return nil
		},
	}
}

func (tc property_string_rule_case) schema() schema.Schema {
	string_rule := schema.String{
		MustNotBeNil:  tc.must_not_be_nil,
		MustNotBeZero: tc.must_not_be_zero,
		TrimSpace:     tc.trim_space,
		ValidateFunc:  tc.validate_func(),
		SkipFunc:      tc.skip_func(),
		TransformFunc: tc.transform_func(),
	}
	if tc.case_mode == 1 {
		string_rule.ToLower = true
	}
	if tc.case_mode == 2 {
		string_rule.ToUpper = true
	}
	if tc.has_default_if_nil {
		string_rule.DefaultIfNil = tc.default_if_nil
	}
	if tc.has_default_if_zero {
		string_rule.DefaultIfZero = tc.default_if_zero
	}
	return schema.Object{"Value": string_rule}
}

func (tc property_int_rule_case) schema() schema.Schema {
	int_rule := schema.Int{
		MustNotBeNil:  tc.must_not_be_nil,
		MustNotBeZero: tc.must_not_be_zero,
		SkipFunc:      tc.skip_func(),
		ValidateFunc:  tc.validate_func(),
	}
	if tc.has_default_if_nil {
		int_rule.DefaultIfNil = tc.default_if_nil
	}
	if tc.has_default_if_zero {
		int_rule.DefaultIfZero = tc.default_if_zero
	}
	if min, ok := tc.min_value(); ok {
		int_rule.Min = min
	}
	if max, ok := tc.max_value(); ok {
		int_rule.Max = max
	}
	return schema.Object{"Value": int_rule}
}

func (tc property_object_pipeline_case) schema() schema.Schema {
	obj := schema.Object{
		"Name": schema.String{
			TrimSpace:     true,
			ToLower:       true,
			DefaultIfZero: "guest",
		},
		"Alias": schema.String{
			TrimSpace: true,
			ToLower:   true,
		},
		"Level": schema.Int{
			DefaultIfZero: 1,
			Min:           1,
			Max:           5,
		},
	}
	if transform_func := tc.transform_func(); transform_func != nil {
		obj[schema.TransformFunc] = transform_func
	}
	if validate_func := tc.validate_func(); validate_func != nil {
		obj[schema.ValidateFunc] = validate_func
	}
	return obj
}

func (tc property_schema_map_case) schema() schema.Schema {
	return schema.Object{
		"M": schema.Map{
			KeySchema:   schema.String{TrimSpace: true, ToLower: true},
			ValueSchema: schema.String{TrimSpace: true, ToUpper: true},
		},
	}
}

func (tc property_map_length_case) schema() schema.Schema {
	map_rule := schema.Map{
		KeySchema: schema.String{TrimSpace: true, ToLower: true},
	}
	if min_len, ok := tc.min_len(); ok {
		map_rule.MinLen = min_len
	}
	if max_len, ok := tc.max_len(); ok {
		map_rule.MaxLen = max_len
	}
	if validate_func := tc.validate_func(); validate_func != nil {
		map_rule.ValidateFunc = validate_func
	}
	return schema.Object{"M": map_rule}
}

func (tc property_schema_case) outcome(
	form property_schema_form,
	err error,
) property_schema_outcome {
	outcome := property_schema_outcome{
		snapshot: form.snapshot(),
	}
	if err == nil {
		return outcome
	}
	outcome.has_error = true
	outcome.error_message = err.Error()
	outcome.is_validation = schema.IsValidationError(err)
	outcome.is_schema = schema.IsSchemaError(err)
	return outcome
}

func (tc property_string_rule_case) outcome(
	holder *property_string_rule_holder,
	err error,
) property_string_rule_outcome {
	outcome := property_string_rule_outcome{
		snapshot: holder.snapshot(),
	}
	if err == nil {
		return outcome
	}
	outcome.has_error = true
	outcome.error_message = err.Error()
	outcome.is_validation = schema.IsValidationError(err)
	outcome.is_schema = schema.IsSchemaError(err)
	return outcome
}

func (tc property_map_length_case) outcome(
	entries map[string]string,
	err error,
) property_map_length_outcome {
	outcome := property_map_length_outcome{
		length: len(entries),
	}
	if err == nil {
		return outcome
	}
	outcome.has_error = true
	outcome.error_message = err.Error()
	outcome.is_validation = schema.IsValidationError(err)
	outcome.is_schema = schema.IsSchemaError(err)
	return outcome
}

func (tc property_int_rule_case) outcome(
	holder *property_int_rule_holder,
	err error,
) property_int_rule_outcome {
	outcome := property_int_rule_outcome{
		snapshot: holder.snapshot(),
	}
	if err == nil {
		return outcome
	}
	outcome.has_error = true
	outcome.error_message = err.Error()
	outcome.is_validation = schema.IsValidationError(err)
	outcome.is_schema = schema.IsSchemaError(err)
	return outcome
}

func (tc property_object_pipeline_case) outcome(
	form property_object_pipeline_form,
	err error,
) property_object_pipeline_outcome {
	outcome := property_object_pipeline_outcome{
		snapshot: form.snapshot(),
	}
	if err == nil {
		return outcome
	}
	outcome.has_error = true
	outcome.error_message = err.Error()
	outcome.is_validation = schema.IsValidationError(err)
	outcome.is_schema = schema.IsSchemaError(err)
	return outcome
}

func (f property_schema_form) snapshot() property_schema_snapshot {
	snapshot := property_schema_snapshot{
		email:            f.Email,
		username:         f.Username,
		password:         f.Password,
		password_confirm: f.PasswordConfirm,
		country:          f.Country,
		state_code:       f.StateCode,
		priority:         f.Priority,
		bio:              f.Bio,
	}
	if f.Nickname != nil {
		snapshot.has_nickname = true
		snapshot.nickname = *f.Nickname
	}
	return snapshot
}

func (f property_schema_form) clone() property_schema_form {
	cloned := f
	if f.Nickname != nil {
		nickname := *f.Nickname
		cloned.Nickname = &nickname
	}
	return cloned
}

func (f property_object_pipeline_form) snapshot() property_object_pipeline_snapshot {
	return property_object_pipeline_snapshot{
		name:  f.Name,
		alias: f.Alias,
		level: f.Level,
	}
}

func (tc property_int_rule_case) holder() property_int_rule_holder {
	if tc.input == nil {
		return property_int_rule_holder{}
	}
	value := *tc.input
	return property_int_rule_holder{Value: &value}
}

func (tc property_string_rule_case) holder() property_string_rule_holder {
	if tc.input == nil {
		return property_string_rule_holder{}
	}
	value := *tc.input
	return property_string_rule_holder{Value: &value}
}

func (tc property_string_rule_case) expected_outcome() property_string_rule_outcome {
	snapshot := tc.snapshot()

	if !snapshot.has_value {
		if tc.has_default_if_nil {
			snapshot.has_value = true
			snapshot.value = tc.default_if_nil
		} else {
			outcome := property_string_rule_outcome{snapshot: snapshot}
			if tc.must_not_be_nil {
				outcome.has_error = true
				outcome.error_message = "holder.Value must not be nil"
				outcome.is_validation = true
			}
			return outcome
		}
	}

	value := snapshot.value
	if tc.should_skip(value) {
		snapshot.value = value
		return property_string_rule_outcome{snapshot: snapshot}
	}

	value = tc.normalize(value)
	if value == "" && tc.has_default_if_zero {
		value = tc.default_if_zero
	}
	snapshot.value = value

	outcome := property_string_rule_outcome{snapshot: snapshot}
	if value == "" && tc.must_not_be_zero {
		outcome.has_error = true
		outcome.error_message = "holder.Value must not be empty"
		outcome.is_validation = true
		return outcome
	}
	if err := tc.validate(value); err != nil {
		outcome.has_error = true
		outcome.error_message = fmt.Sprintf("holder.Value: %s", err.Error())
		outcome.is_validation = true
	}
	return outcome
}

func (tc property_int_rule_case) expected_outcome() property_int_rule_outcome {
	snapshot := tc.snapshot()

	if min, has_min := tc.min_value(); has_min {
		if max, has_max := tc.max_value(); has_max && min > max {
			return property_int_rule_outcome{
				snapshot:  snapshot,
				has_error: true,
				error_message: fmt.Sprintf(
					"holder.Value: Min (%d) cannot be greater than Max (%d)",
					min,
					max,
				),
				is_schema: true,
			}
		}
	}

	if !snapshot.has_value {
		if tc.has_default_if_nil {
			snapshot.has_value = true
			snapshot.value = tc.default_if_nil
		} else {
			outcome := property_int_rule_outcome{snapshot: snapshot}
			if tc.must_not_be_nil {
				outcome.has_error = true
				outcome.error_message = "holder.Value must not be nil"
				outcome.is_validation = true
			}
			return outcome
		}
	}

	value := snapshot.value
	if tc.should_skip(value) {
		snapshot.value = value
		return property_int_rule_outcome{snapshot: snapshot}
	}

	if value == 0 && tc.has_default_if_zero {
		value = tc.default_if_zero
	}
	snapshot.value = value

	outcome := property_int_rule_outcome{snapshot: snapshot}
	if value == 0 && tc.must_not_be_zero {
		outcome.has_error = true
		outcome.error_message = "holder.Value must not be zero"
		outcome.is_validation = true
		return outcome
	}
	var validation_errs []error
	if min, has_min := tc.min_value(); has_min && value < min {
		validation_errs = append(
			validation_errs,
			fmt.Errorf("holder.Value: minimum is %d, got %d", min, value),
		)
	}
	if max, has_max := tc.max_value(); has_max && value > max {
		validation_errs = append(
			validation_errs,
			fmt.Errorf("holder.Value: maximum is %d, got %d", max, value),
		)
	}
	if err := tc.validate(value); err != nil {
		validation_errs = append(validation_errs, fmt.Errorf("holder.Value: %s", err.Error()))
	}
	if len(validation_errs) > 0 {
		outcome.has_error = true
		outcome.error_message = errors.Join(validation_errs...).Error()
		outcome.is_validation = true
	}
	return outcome
}

func (tc property_object_pipeline_case) expected_outcome() property_object_pipeline_outcome {
	form := tc.form

	form.Name = strings.ToLower(strings.TrimSpace(form.Name))
	if form.Name == "" {
		form.Name = "guest"
	}
	form.Alias = strings.ToLower(strings.TrimSpace(form.Alias))
	if form.Level == 0 {
		form.Level = 1
	}

	form = tc.transform(form)

	outcome := property_object_pipeline_outcome{
		snapshot: form.snapshot(),
	}
	if err := tc.validate(form); err != nil {
		outcome.has_error = true
		outcome.error_message = fmt.Sprintf("holder: %s", err.Error())
		outcome.is_validation = true
	}
	return outcome
}

func (tc property_string_rule_case) snapshot() property_string_rule_snapshot {
	snapshot := property_string_rule_snapshot{}
	if tc.input != nil {
		snapshot.has_value = true
		snapshot.value = *tc.input
	}
	return snapshot
}

func (tc property_int_rule_case) snapshot() property_int_rule_snapshot {
	snapshot := property_int_rule_snapshot{}
	if tc.input != nil {
		snapshot.has_value = true
		snapshot.value = *tc.input
	}
	return snapshot
}

func (tc property_map_length_case) holder() container_map_holder {
	entries := make(map[string]string, len(tc.entries))
	maps.Copy(entries, tc.entries)
	return container_map_holder{M: entries}
}

func (tc property_map_length_case) expected_entries() map[string]string {
	keys := make([]string, 0, len(tc.entries))
	for key := range tc.entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	expected := make(map[string]string, len(keys))
	for _, key := range keys {
		normalized_key := strings.ToLower(strings.TrimSpace(key))
		expected[normalized_key] = tc.entries[key]
	}
	return expected
}

func (tc property_map_length_case) expected_outcome(
	expected_entries map[string]string,
) property_map_length_outcome {
	length := len(expected_entries)
	outcome := property_map_length_outcome{length: length}

	if min_len, has_min := tc.min_len(); has_min {
		if max_len, has_max := tc.max_len(); has_max && min_len > max_len {
			outcome.has_error = true
			outcome.length = len(tc.entries)
			outcome.error_message = fmt.Sprintf(
				"holder.M: MinLen (%d) cannot be greater than MaxLen (%d)",
				min_len,
				max_len,
			)
			outcome.is_schema = true
			return outcome
		}
	}

	var validation_errs []error
	if min_len, ok := tc.min_len(); ok && length < min_len {
		validation_errs = append(
			validation_errs,
			fmt.Errorf("holder.M: minimum length is %d, got %d", min_len, length),
		)
	}
	if max_len, ok := tc.max_len(); ok && length > max_len {
		validation_errs = append(
			validation_errs,
			fmt.Errorf("holder.M: maximum length is %d, got %d", max_len, length),
		)
	}
	if validate_err := tc.validate(length); validate_err != nil {
		validation_errs = append(
			validation_errs,
			fmt.Errorf("holder.M: %s", validate_err.Error()),
		)
	}
	if len(validation_errs) > 0 {
		outcome.has_error = true
		outcome.error_message = errors.Join(validation_errs...).Error()
		outcome.is_validation = true
	}
	return outcome
}

func (tc property_schema_map_case) holder() container_map_holder {
	entries := make(map[string]string, len(tc.entries))
	maps.Copy(entries, tc.entries)
	return container_map_holder{M: entries}
}

func (tc property_schema_map_case) expected_entries() map[string]string {
	keys := make([]string, 0, len(tc.entries))
	for key := range tc.entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	expected := make(map[string]string, len(keys))
	for _, key := range keys {
		normalized_key := strings.ToLower(strings.TrimSpace(key))
		normalized_value := strings.ToUpper(strings.TrimSpace(tc.entries[key]))
		expected[normalized_key] = normalized_value
	}
	return expected
}

func (tc property_schema_map_case) maps_equal(
	left map[string]string,
	right map[string]string,
) bool {
	if len(left) != len(right) {
		return false
	}
	for key, left_value := range left {
		right_value, ok := right[key]
		if !ok || left_value != right_value {
			return false
		}
	}
	return true
}

func (tc property_map_length_case) maps_equal(
	left map[string]string,
	right map[string]string,
) bool {
	if len(left) != len(right) {
		return false
	}
	for key, left_value := range left {
		right_value, ok := right[key]
		if !ok || left_value != right_value {
			return false
		}
	}
	return true
}

func (tc property_string_rule_case) transform_func() func(string) (string, error) {
	switch tc.transform_mode {
	case 1:
		return func(value string) (string, error) {
			if value == "erase" {
				return "", nil
			}
			return value, nil
		}
	case 2:
		return func(value string) (string, error) {
			if value == "" {
				return "", nil
			}
			return value + "!", nil
		}
	default:
		return nil
	}
}

func (tc property_string_rule_case) skip_func() func(string) bool {
	switch tc.skip_mode {
	case 1:
		return func(value string) bool {
			return value == ""
		}
	case 2:
		return func(value string) bool {
			return strings.Contains(strings.ToLower(value), "skip")
		}
	default:
		return nil
	}
}

func (tc property_string_rule_case) validate_func() func(string) error {
	switch tc.validate_mode {
	case 1:
		return func(value string) error {
			if strings.Contains(value, "!") {
				return errors.New("bangs are not allowed")
			}
			return nil
		}
	case 2:
		return func(value string) error {
			if len(value)%2 != 0 {
				return errors.New("length must be even")
			}
			return nil
		}
	default:
		return nil
	}
}

func (tc property_string_rule_case) should_skip(value string) bool {
	skip_func := tc.skip_func()
	return skip_func != nil && skip_func(value)
}

func (tc property_string_rule_case) normalize(value string) string {
	if tc.trim_space {
		value = strings.TrimSpace(value)
	}
	switch tc.case_mode {
	case 1:
		value = strings.ToLower(value)
	case 2:
		value = strings.ToUpper(value)
	}
	transform_func := tc.transform_func()
	if transform_func == nil {
		return value
	}
	transformed, err := transform_func(value)
	if err != nil {
		return value
	}
	return transformed
}

func (tc property_string_rule_case) validate(value string) error {
	validate_func := tc.validate_func()
	if validate_func == nil {
		return nil
	}
	return validate_func(value)
}

func (tc property_int_rule_case) skip_func() func(int) bool {
	switch tc.skip_mode {
	case 1:
		return func(value int) bool {
			return value == 0
		}
	case 2:
		return func(value int) bool {
			return value < 0
		}
	default:
		return nil
	}
}

func (tc property_int_rule_case) validate_func() func(int) error {
	switch tc.validate_mode {
	case 1:
		return func(value int) error {
			if value%2 != 0 {
				return errors.New("value must be even")
			}
			return nil
		}
	case 2:
		return func(value int) error {
			if value > 2 {
				return errors.New("value must be at most two")
			}
			return nil
		}
	default:
		return nil
	}
}

func (tc property_int_rule_case) should_skip(value int) bool {
	skip_func := tc.skip_func()
	return skip_func != nil && skip_func(value)
}

func (tc property_int_rule_case) validate(value int) error {
	validate_func := tc.validate_func()
	if validate_func == nil {
		return nil
	}
	return validate_func(value)
}

func (tc property_int_rule_case) min_value() (int, bool) {
	switch tc.min_mode {
	case 1:
		return 1, true
	case 2:
		return 2, true
	default:
		return 0, false
	}
}

func (tc property_int_rule_case) max_value() (int, bool) {
	switch tc.max_mode {
	case 1:
		return 2, true
	case 2:
		return 1, true
	default:
		return 0, false
	}
}

func (tc property_map_length_case) min_len() (int, bool) {
	switch tc.min_mode {
	case 1:
		return 1, true
	case 2:
		return 2, true
	default:
		return 0, false
	}
}

func (tc property_map_length_case) max_len() (int, bool) {
	switch tc.max_mode {
	case 1:
		return 1, true
	case 2:
		return 2, true
	default:
		return 0, false
	}
}

func (tc property_map_length_case) validate_func() func(int) error {
	switch tc.validate_mode {
	case 1:
		return func(length int) error {
			if length%2 != 0 {
				return errors.New("length must be even")
			}
			return nil
		}
	case 2:
		return func(length int) error {
			if length != 1 {
				return errors.New("length must be exactly one")
			}
			return nil
		}
	default:
		return nil
	}
}

func (tc property_map_length_case) validate(length int) error {
	validate_func := tc.validate_func()
	if validate_func == nil {
		return nil
	}
	return validate_func(length)
}

func (tc property_object_pipeline_case) transform_func() func(property_object_pipeline_form) (property_object_pipeline_form, error) {
	switch tc.transform_mode {
	case 1:
		return func(form property_object_pipeline_form) (property_object_pipeline_form, error) {
			if form.Alias == "" {
				form.Alias = form.Name
			}
			return form, nil
		}
	case 2:
		return func(form property_object_pipeline_form) (property_object_pipeline_form, error) {
			if form.Alias != "" {
				form.Alias = fmt.Sprintf("%s-%d", form.Alias, form.Level)
			}
			return form, nil
		}
	case 3:
		return func(form property_object_pipeline_form) (property_object_pipeline_form, error) {
			if form.Name == "admin" {
				form.Alias = "root"
			}
			return form, nil
		}
	default:
		return nil
	}
}

func (tc property_object_pipeline_case) validate_func() func(property_object_pipeline_form) error {
	switch tc.validate_mode {
	case 1:
		return func(form property_object_pipeline_form) error {
			if form.Alias == "" {
				return errors.New("alias required")
			}
			return nil
		}
	case 2:
		return func(form property_object_pipeline_form) error {
			if form.Alias != "" && !strings.HasPrefix(form.Alias, form.Name) &&
				form.Alias != "root" {
				return errors.New("alias must derive from name")
			}
			return nil
		}
	case 3:
		return func(form property_object_pipeline_form) error {
			if form.Alias != "" && !strings.HasSuffix(form.Alias, fmt.Sprintf("-%d", form.Level)) {
				return errors.New("alias must include level suffix")
			}
			return nil
		}
	default:
		return nil
	}
}

func (tc property_object_pipeline_case) transform(
	form property_object_pipeline_form,
) property_object_pipeline_form {
	transform_func := tc.transform_func()
	if transform_func == nil {
		return form
	}
	transformed, err := transform_func(form)
	if err != nil {
		return form
	}
	return transformed
}

func (tc property_object_pipeline_case) validate(
	form property_object_pipeline_form,
) error {
	validate_func := tc.validate_func()
	if validate_func == nil {
		return nil
	}
	return validate_func(form)
}

func (o property_schema_outcome) equal(other property_schema_outcome) bool {
	return o.snapshot.equal(other.snapshot) &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}

func (o property_map_length_outcome) equal(other property_map_length_outcome) bool {
	return o.length == other.length &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}

func (o property_string_rule_outcome) equal(other property_string_rule_outcome) bool {
	return o.snapshot.equal(other.snapshot) &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}

func (o property_int_rule_outcome) equal(other property_int_rule_outcome) bool {
	return o.snapshot.equal(other.snapshot) &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}

func (o property_object_pipeline_outcome) equal(other property_object_pipeline_outcome) bool {
	return o.snapshot.equal(other.snapshot) &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}

func (s property_schema_snapshot) equal(other property_schema_snapshot) bool {
	return s.email == other.email &&
		s.username == other.username &&
		s.password == other.password &&
		s.password_confirm == other.password_confirm &&
		s.country == other.country &&
		s.state_code == other.state_code &&
		s.priority == other.priority &&
		s.bio == other.bio &&
		s.nickname == other.nickname &&
		s.has_nickname == other.has_nickname
}

func (s property_string_rule_snapshot) equal(other property_string_rule_snapshot) bool {
	return s.has_value == other.has_value && s.value == other.value
}

func (s property_int_rule_snapshot) equal(other property_int_rule_snapshot) bool {
	return s.has_value == other.has_value && s.value == other.value
}

func (s property_object_pipeline_snapshot) equal(other property_object_pipeline_snapshot) bool {
	return s.name == other.name && s.alias == other.alias && s.level == other.level
}

func (h property_string_rule_holder) snapshot() property_string_rule_snapshot {
	snapshot := property_string_rule_snapshot{}
	if h.Value != nil {
		snapshot.has_value = true
		snapshot.value = *h.Value
	}
	return snapshot
}

func (h property_int_rule_holder) snapshot() property_int_rule_snapshot {
	snapshot := property_int_rule_snapshot{}
	if h.Value != nil {
		snapshot.has_value = true
		snapshot.value = *h.Value
	}
	return snapshot
}
