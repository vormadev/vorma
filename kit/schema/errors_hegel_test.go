package schema_test

import (
	"errors"
	"fmt"
	"testing"

	"hegel.dev/go/hegel"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_error_form struct {
	Name  string
	Count int
}

type property_error_case struct {
	form            property_error_form
	name_rule_mode  int
	count_rule_mode int
}

type property_error_case_space struct{}

type property_error_outcome struct {
	has_error     bool
	error_message string
	is_validation bool
	is_schema     bool
}

var property_error_names = []string{"", "  ", "ok"}

var property_error_counts = []int{0, 1, 3}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestSchemaErrorPrecedenceMatchesModel(t *testing.T) {
	t.Run("generated_schema_and_validation_cross_product", hegel.Case(func(ht *hegel.T) {
		tc := (property_error_case_space{}).draw_case(ht)
		tc.assert_schema_error_precedence_matches_model(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_error_case_space) draw_case(ht *hegel.T) property_error_case {
	return property_error_case{
		form: property_error_form{
			Name:  hegel.Draw(ht, hegel.SampledFrom(property_error_names)),
			Count: hegel.Draw(ht, hegel.SampledFrom(property_error_counts)),
		},
		name_rule_mode:  hegel.Draw(ht, hegel.Integers(0, 4)),
		count_rule_mode: hegel.Draw(ht, hegel.Integers(0, 4)),
	}
}

func (tc property_error_case) assert_schema_error_precedence_matches_model(ht *hegel.T) {
	tc.note(ht)

	_, err := schema.Enforce("holder", tc.form, tc.schema())
	actual := tc.outcome(err)
	expected := tc.expected_outcome()

	if !actual.equal(expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}
}

func (tc property_error_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {form:%#v name_rule_mode:%d count_rule_mode:%d}",
			tc.form,
			tc.name_rule_mode,
			tc.count_rule_mode,
		),
	)
}

func (tc property_error_case) schema() schema.Schema {
	return schema.Object{
		"Name":  tc.name_rule(),
		"Count": tc.count_rule(),
	}
}

func (tc property_error_case) name_rule() schema.Schema {
	switch tc.name_rule_mode {
	case 1:
		return schema.String{MustNotBeZero: true}
	case 2:
		return schema.String{MinLen: -1}
	case 3:
		return schema.String{DefaultIfZero: 1}
	case 4:
		return schema.Int{Min: 1}
	default:
		return schema.String{}
	}
}

func (tc property_error_case) count_rule() schema.Schema {
	switch tc.count_rule_mode {
	case 1:
		return schema.Int{MustNotBeZero: true, Min: 1}
	case 2:
		return schema.Int{Min: 2, Max: 1}
	case 3:
		return schema.Int{DefaultIfZero: "fallback"}
	case 4:
		return schema.String{MustNotBeZero: true}
	default:
		return schema.Int{}
	}
}

func (tc property_error_case) expected_outcome() property_error_outcome {
	var schema_errs []error
	if schema_err := tc.count_schema_error(); schema_err != nil {
		schema_errs = append(schema_errs, schema_err)
	}
	if schema_err := tc.name_schema_error(); schema_err != nil {
		schema_errs = append(schema_errs, schema_err)
	}
	if len(schema_errs) > 0 {
		return property_error_outcome{
			has_error:     true,
			error_message: errors.Join(schema_errs...).Error(),
			is_schema:     true,
		}
	}

	var validation_errs []error
	if validation_err := tc.count_validation_error(); validation_err != nil {
		validation_errs = append(validation_errs, validation_err)
	}
	if validation_err := tc.name_validation_error(); validation_err != nil {
		validation_errs = append(validation_errs, validation_err)
	}
	if len(validation_errs) > 0 {
		return property_error_outcome{
			has_error:     true,
			error_message: errors.Join(validation_errs...).Error(),
			is_validation: true,
		}
	}
	return property_error_outcome{}
}

func (tc property_error_case) name_schema_error() error {
	switch tc.name_rule_mode {
	case 2:
		return errors.New("holder.Name: MinLen: must be non-negative, got -1")
	case 3:
		return errors.New("holder.Name: DefaultIfZero: expected string value, got int")
	case 4:
		return errors.New("holder.Name: Int rule applied to non-signed-integer-kinded value (string)")
	default:
		return nil
	}
}

func (tc property_error_case) count_schema_error() error {
	switch tc.count_rule_mode {
	case 2:
		return errors.New("holder.Count: Min (2) cannot be greater than Max (1)")
	case 3:
		return errors.New("holder.Count: DefaultIfZero: expected int-like value, got string")
	case 4:
		return errors.New("holder.Count: String rule applied to non-string-kinded value (int)")
	default:
		return nil
	}
}

func (tc property_error_case) name_validation_error() error {
	if tc.name_rule_mode != 1 {
		return nil
	}
	if tc.form.Name == "" {
		return errors.New("holder.Name must not be empty")
	}
	return nil
}

func (tc property_error_case) count_validation_error() error {
	if tc.count_rule_mode != 1 {
		return nil
	}
	var validation_errs []error
	if tc.form.Count == 0 {
		validation_errs = append(validation_errs, errors.New("holder.Count must not be zero"))
		return errors.Join(validation_errs...)
	}
	if tc.form.Count < 1 {
		validation_errs = append(
			validation_errs,
			fmt.Errorf("holder.Count: minimum is 1, got %d", tc.form.Count),
		)
	}
	if len(validation_errs) == 0 {
		return nil
	}
	return errors.Join(validation_errs...)
}

func (tc property_error_case) outcome(err error) property_error_outcome {
	if err == nil {
		return property_error_outcome{}
	}
	return property_error_outcome{
		has_error:     true,
		error_message: err.Error(),
		is_validation: schema.IsValidationError(err),
		is_schema:     schema.IsSchemaError(err),
	}
}

func (o property_error_outcome) equal(other property_error_outcome) bool {
	return o.has_error == other.has_error &&
		o.error_message == other.error_message &&
		o.is_validation == other.is_validation &&
		o.is_schema == other.is_schema
}
