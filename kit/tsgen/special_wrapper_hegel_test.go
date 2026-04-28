package tsgen

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_special_wrapper_created_at time.Time

type property_special_wrapper_elapsed time.Duration

type property_special_wrapper_host struct {
	Created property_special_wrapper_created_at `json:"created"`
	Took    property_special_wrapper_elapsed    `json:"took"`
}

type property_special_wrapper_case struct {
	include_host      bool
	duplicate_time    bool
	duplicate_elapsed bool
	swap_root_order   bool
}

type property_special_wrapper_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesSpecialBuiltinWrappersKeepBuiltinShape(t *testing.T) {
	t.Run("generated_special_wrapper_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_special_wrapper_case_space{}).draw_case(ht)
		tc.assert_special_wrapper_model(ht)
	}, hegel.WithTestCases(300)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_special_wrapper_case_space) draw_case(
	ht *hegel.T,
) property_special_wrapper_case {
	return property_special_wrapper_case{
		include_host:      hegel.Draw(ht, hegel.Booleans()),
		duplicate_time:    hegel.Draw(ht, hegel.Booleans()),
		duplicate_elapsed: hegel.Draw(ht, hegel.Booleans()),
		swap_root_order:   hegel.Draw(ht, hegel.Booleans()),
	}
}

func (tc property_special_wrapper_case) assert_special_wrapper_model(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	ordered := resolve_registry(tc.ordered_inputs()...)
	shuffled := resolve_registry(tc.shuffled_inputs()...)
	duplicates := resolve_registry(tc.duplicated_inputs()...)

	if !reflect.DeepEqual(ordered, shuffled) {
		ht.Fatalf("ordered = %#v, shuffled = %#v", ordered, shuffled)
	}
	if !reflect.DeepEqual(ordered, duplicates) {
		ht.Fatalf("ordered = %#v, duplicates = %#v", ordered, duplicates)
	}

	assert_special_wrapper_body(ht, ordered, "CreatedAt", `string`)
	assert_special_wrapper_body(ht, ordered, "Elapsed", `number`)

	if tc.include_host {
		assert_special_wrapper_body(
			ht,
			ordered,
			"WrapperHost",
			`{ created: CreatedAt; took: Elapsed; }`,
		)
	}
}

func (tc property_special_wrapper_case) ordered_inputs() []*GoTypeSrc {
	created := GoType[property_special_wrapper_created_at]("CreatedAt")
	elapsed := GoType[property_special_wrapper_elapsed]("Elapsed")

	var inputs []*GoTypeSrc
	if tc.swap_root_order {
		inputs = append(inputs, elapsed, created)
	} else {
		inputs = append(inputs, created, elapsed)
	}
	if tc.include_host {
		inputs = append(inputs, GoType[property_special_wrapper_host]("WrapperHost"))
	}
	return inputs
}

func (tc property_special_wrapper_case) shuffled_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	if len(inputs) >= 2 {
		inputs[0], inputs[1] = inputs[1], inputs[0]
	}
	return inputs
}

func (tc property_special_wrapper_case) duplicated_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	if tc.duplicate_time {
		inputs = append(inputs, GoType[property_special_wrapper_created_at]("CreatedAt"))
	}
	if tc.duplicate_elapsed {
		inputs = append(inputs, GoType[property_special_wrapper_elapsed]("Elapsed"))
	}
	return inputs
}

func assert_special_wrapper_body(
	ht *hegel.T,
	defs ResolvedTSTypes,
	name, expected_body string,
) {
	def := find_by_name(defs, name)
	if def == nil {
		ht.Fatalf("missing type %q in defs %#v", name, defs)
	}
	if norm(def.Body) != norm(expected_body) {
		ht.Fatalf(
			"type %q body = %q, expected %q",
			name,
			def.Body,
			expected_body,
		)
	}
}
