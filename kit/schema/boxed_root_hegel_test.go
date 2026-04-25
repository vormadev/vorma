package schema_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"hegel.dev/go/hegel"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_boxed_root_object_form struct {
	Name  string
	Alias string
	Level int
}

type property_boxed_root_object_direct_holder struct {
	V property_boxed_root_object_form
}

type property_boxed_root_array_direct_holder struct {
	V [3]string
}

type property_boxed_root_boxed_holder struct {
	V any
}

type property_boxed_root_object_case struct {
	form            property_boxed_root_object_form
	name_rule_mode  int
	alias_rule_mode int
	level_rule_mode int
	transform_mode  int
	validate_mode   int
}

type property_boxed_root_array_case struct {
	values            [3]string
	element_rule_mode int
}

type property_boxed_root_object_case_space struct{}

type property_boxed_root_array_case_space struct{}

type property_boxed_root_object_outcome struct {
	value_type    string
	snapshot      property_boxed_root_object_form
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

type property_boxed_root_array_outcome struct {
	value_type    string
	snapshot      [3]string
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

var property_boxed_root_names = []string{
	"",
	"  Alice  ",
	"ADMIN",
	" blocked ",
	"  ",
}

var property_boxed_root_aliases = []string{
	"",
	"  helper  ",
	"ROOT",
	" guest ",
	"  alice  ",
}

var property_boxed_root_levels = []int{0, 1, 2, 5}

var property_boxed_root_array_values = []string{
	"",
	"  A  ",
	"  B",
	"c  ",
	"  guest  ",
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestExplicitRootObjectOnBoxedValueMatchesDirectField(t *testing.T) {
	t.Run("generated_struct_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_boxed_root_object_case_space{}).draw_case(ht)
		tc.assert_boxed_matches_direct_field(ht)
	}, hegel.WithTestCases(500)))
}

func TestExplicitRootArrayOnBoxedValueMatchesDirectField(t *testing.T) {
	t.Run("generated_array_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_boxed_root_array_case_space{}).draw_case(ht)
		tc.assert_boxed_matches_direct_field(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_boxed_root_object_case_space) draw_case(
	ht *hegel.T,
) property_boxed_root_object_case {
	return property_boxed_root_object_case{
		form: property_boxed_root_object_form{
			Name:  hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_names)),
			Alias: hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_aliases)),
			Level: hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_levels)),
		},
		name_rule_mode:  hegel.Draw(ht, hegel.Integers(0, 2)),
		alias_rule_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
		level_rule_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
		transform_mode:  hegel.Draw(ht, hegel.Integers(0, 2)),
		validate_mode:   hegel.Draw(ht, hegel.Integers(0, 2)),
	}
}

func (property_boxed_root_array_case_space) draw_case(
	ht *hegel.T,
) property_boxed_root_array_case {
	return property_boxed_root_array_case{
		values: [3]string{
			hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_array_values)),
			hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_array_values)),
			hegel.Draw(ht, hegel.SampledFrom(property_boxed_root_array_values)),
		},
		element_rule_mode: hegel.Draw(ht, hegel.Integers(0, 3)),
	}
}

func (tc property_boxed_root_object_case) assert_boxed_matches_direct_field(
	ht *hegel.T,
) {
	tc.note(ht)

	direct_holder := property_boxed_root_object_direct_holder{V: tc.form}
	_, direct_err := schema.Enforce("holder", &direct_holder, tc.schema())
	direct_outcome := tc.direct_outcome(direct_holder.V, direct_err)

	boxed_holder := property_boxed_root_boxed_holder{V: tc.form}
	_, boxed_err := schema.Enforce("holder", &boxed_holder, tc.schema())
	boxed_outcome := tc.boxed_outcome(boxed_holder.V, boxed_err)

	if !boxed_outcome.equal(direct_outcome) {
		ht.Fatalf("boxed outcome = %#v, direct outcome = %#v", boxed_outcome, direct_outcome)
	}
}

func (tc property_boxed_root_array_case) assert_boxed_matches_direct_field(
	ht *hegel.T,
) {
	tc.note(ht)

	direct_holder := property_boxed_root_array_direct_holder{V: tc.values}
	_, direct_err := schema.Enforce("holder", &direct_holder, tc.schema())
	direct_outcome := tc.direct_outcome(direct_holder.V, direct_err)

	boxed_holder := property_boxed_root_boxed_holder{V: tc.values}
	_, boxed_err := schema.Enforce("holder", &boxed_holder, tc.schema())
	boxed_outcome := tc.boxed_outcome(boxed_holder.V, boxed_err)

	if !boxed_outcome.equal(direct_outcome) {
		ht.Fatalf("boxed outcome = %#v, direct outcome = %#v", boxed_outcome, direct_outcome)
	}
}

func (tc property_boxed_root_object_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {form:%#v name_rule_mode:%d alias_rule_mode:%d level_rule_mode:%d transform_mode:%d validate_mode:%d}",
			tc.form,
			tc.name_rule_mode,
			tc.alias_rule_mode,
			tc.level_rule_mode,
			tc.transform_mode,
			tc.validate_mode,
		),
	)
}

func (tc property_boxed_root_array_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {values:%#v element_rule_mode:%d}",
			tc.values,
			tc.element_rule_mode,
		),
	)
}

func (tc property_boxed_root_object_case) schema() schema.Schema {
	root := schema.Object{
		"Name":  tc.name_rule(),
		"Alias": tc.alias_rule(),
		"Level": tc.level_rule(),
	}
	if transform_func := tc.transform_func(); transform_func != nil {
		root[schema.TransformFunc] = transform_func
	}
	if validate_func := tc.validate_func(); validate_func != nil {
		root[schema.ValidateFunc] = validate_func
	}
	return schema.Object{"V": root}
}

func (tc property_boxed_root_array_case) schema() schema.Schema {
	return schema.Object{
		"V": schema.List{
			ElementSchema: tc.element_rule(),
		},
	}
}

func (tc property_boxed_root_object_case) name_rule() schema.Schema {
	switch tc.name_rule_mode {
	case 1:
		return schema.String{TrimSpace: true, ToLower: true}
	case 2:
		return schema.String{
			TrimSpace:     true,
			ToLower:       true,
			MustNotBeZero: true,
			DefaultIfZero: "guest",
			MustNotBeIn:   []string{"blocked"},
			MustStartWith: "g",
			AllowedChars:  "abcdefghijklmnopqrstuvwxyz",
			MustEndWith:   "t",
			MustBeIn:      []string{"guest", "alice", "admin"},
		}
	default:
		return schema.String{}
	}
}

func (tc property_boxed_root_object_case) alias_rule() schema.Schema {
	switch tc.alias_rule_mode {
	case 1:
		return schema.String{TrimSpace: true, ToUpper: true}
	case 2:
		return schema.String{
			TrimSpace:     true,
			ToUpper:       true,
			DefaultIfZero: "AUTO",
		}
	default:
		return schema.String{}
	}
}

func (tc property_boxed_root_object_case) level_rule() schema.Schema {
	switch tc.level_rule_mode {
	case 1:
		return schema.Int{Min: 1}
	case 2:
		return schema.Int{DefaultIfZero: 2}
	default:
		return schema.Int{}
	}
}

func (tc property_boxed_root_object_case) transform_func() any {
	switch tc.transform_mode {
	case 1:
		return func(
			v property_boxed_root_object_form,
		) (property_boxed_root_object_form, error) {
			if v.Alias == "" {
				v.Alias = "AUTO"
			}
			if v.Level == 0 {
				v.Level = len(v.Name)
			}
			return v, nil
		}
	case 2:
		return func(
			v property_boxed_root_object_form,
		) (property_boxed_root_object_form, error) {
			v.Name = strings.TrimSpace(v.Alias) + ":" + v.Name
			return v, nil
		}
	default:
		return nil
	}
}

func (tc property_boxed_root_object_case) validate_func() any {
	switch tc.validate_mode {
	case 1:
		return func(v property_boxed_root_object_form) error {
			if strings.TrimSpace(v.Name) == strings.TrimSpace(v.Alias) &&
				v.Name != "" {
				return errors.New("name and alias must differ")
			}
			return nil
		}
	case 2:
		return func(v property_boxed_root_object_form) error {
			if v.Level > 0 && strings.TrimSpace(v.Alias) == "ROOT" {
				return errors.New("root alias requires zero level")
			}
			return nil
		}
	default:
		return nil
	}
}

func (tc property_boxed_root_array_case) element_rule() schema.Schema {
	switch tc.element_rule_mode {
	case 1:
		return schema.String{TrimSpace: true, ToLower: true}
	case 2:
		return schema.String{
			TrimSpace:     true,
			ToLower:       true,
			DefaultIfZero: "guest",
			MustNotBeZero: true,
		}
	case 3:
		return schema.String{
			TrimSpace: true,
			ToLower:   true,
			MustBeIn:  []string{"a", "b", "c", "guest"},
		}
	default:
		return schema.String{}
	}
}

func (tc property_boxed_root_object_case) direct_outcome(
	value property_boxed_root_object_form,
	err error,
) property_boxed_root_object_outcome {
	outcome := property_boxed_root_object_outcome{
		value_type: reflect.TypeOf(value).String(),
		snapshot:   value,
	}
	return outcome.with_error(err)
}

func (tc property_boxed_root_object_case) boxed_outcome(
	value any,
	err error,
) property_boxed_root_object_outcome {
	outcome := property_boxed_root_object_outcome{}
	if value != nil {
		outcome.value_type = reflect.TypeOf(value).String()
	}
	if typed, ok := value.(property_boxed_root_object_form); ok {
		outcome.snapshot = typed
	}
	return outcome.with_error(err)
}

func (tc property_boxed_root_array_case) direct_outcome(
	value [3]string,
	err error,
) property_boxed_root_array_outcome {
	outcome := property_boxed_root_array_outcome{
		value_type: reflect.TypeOf(value).String(),
		snapshot:   value,
	}
	return outcome.with_error(err)
}

func (tc property_boxed_root_array_case) boxed_outcome(
	value any,
	err error,
) property_boxed_root_array_outcome {
	outcome := property_boxed_root_array_outcome{}
	if value != nil {
		outcome.value_type = reflect.TypeOf(value).String()
	}
	if typed, ok := value.([3]string); ok {
		outcome.snapshot = typed
	}
	return outcome.with_error(err)
}

func (o property_boxed_root_object_outcome) with_error(
	err error,
) property_boxed_root_object_outcome {
	if err == nil {
		return o
	}
	o.has_error = true
	o.error_message = err.Error()
	o.is_validation = schema.IsValidationError(err)
	o.is_schema = schema.IsSchemaError(err)
	return o
}

func (o property_boxed_root_array_outcome) with_error(
	err error,
) property_boxed_root_array_outcome {
	if err == nil {
		return o
	}
	o.has_error = true
	o.error_message = err.Error()
	o.is_validation = schema.IsValidationError(err)
	o.is_schema = schema.IsSchemaError(err)
	return o
}

func (o property_boxed_root_object_outcome) equal(
	other property_boxed_root_object_outcome,
) bool {
	return o.value_type == other.value_type &&
		o.snapshot == other.snapshot &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}

func (o property_boxed_root_array_outcome) equal(
	other property_boxed_root_array_outcome,
) bool {
	return o.value_type == other.value_type &&
		o.snapshot == other.snapshot &&
		o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}
