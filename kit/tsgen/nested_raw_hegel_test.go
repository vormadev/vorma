package tsgen

import (
	"fmt"
	"reflect"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_nested_raw_plain struct{}

func (property_nested_raw_plain) TSType() string {
	return "{ custom: boolean }"
}

type property_nested_raw_target struct {
	Name string `json:"name"`
}

type property_nested_raw_ref struct{}

func (property_nested_raw_ref) TSType() string {
	return fmt.Sprintf(
		"{ target: %s; }",
		GoType[property_nested_raw_target]().ID(),
	)
}

type property_nested_raw_plain_host struct {
	Dep property_nested_raw_plain `json:"dep"`
}

type property_nested_raw_ref_host struct {
	Dep property_nested_raw_ref `json:"dep"`
}

type property_nested_raw_case struct {
	include_plain    bool
	include_ref      bool
	duplicate_plain  bool
	duplicate_ref    bool
	swap_input_order bool
}

type property_nested_raw_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesDiscoveredNestedRawMatchesModel(t *testing.T) {
	t.Run("generated_nested_raw_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_nested_raw_case_space{}).draw_case(ht)
		tc.assert_nested_raw_model(ht)
	}, hegel.WithTestCases(300)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_nested_raw_case_space) draw_case(
	ht *hegel.T,
) property_nested_raw_case {
	tc := property_nested_raw_case{
		include_plain:    hegel.Draw(ht, hegel.Booleans()),
		include_ref:      hegel.Draw(ht, hegel.Booleans()),
		duplicate_plain:  hegel.Draw(ht, hegel.Booleans()),
		duplicate_ref:    hegel.Draw(ht, hegel.Booleans()),
		swap_input_order: hegel.Draw(ht, hegel.Booleans()),
	}
	if !tc.include_plain && !tc.include_ref {
		tc.include_plain = true
	}
	return tc
}

func (tc property_nested_raw_case) assert_nested_raw_model(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	ordered := resolve_registry(tc.ordered_inputs()...)
	shuffled := resolve_registry(tc.shuffled_inputs()...)
	with_duplicates := resolve_registry(tc.duplicated_inputs()...)

	if !reflect.DeepEqual(ordered, shuffled) {
		ht.Fatalf("ordered = %#v, shuffled = %#v", ordered, shuffled)
	}
	if !reflect.DeepEqual(ordered, with_duplicates) {
		ht.Fatalf("ordered = %#v, duplicate = %#v", ordered, with_duplicates)
	}

	if tc.include_plain {
		assert_nested_raw_body(
			ht,
			ordered,
			"PlainHost",
			`{ dep: property_nested_raw_plain; }`,
		)
		assert_nested_raw_body(
			ht,
			ordered,
			"property_nested_raw_plain",
			`{ custom: boolean }`,
		)
	}

	if tc.include_ref {
		assert_nested_raw_body(
			ht,
			ordered,
			"RefHost",
			`{ dep: property_nested_raw_ref; }`,
		)
		assert_nested_raw_body(
			ht,
			ordered,
			"property_nested_raw_ref",
			`{ target: property_nested_raw_target; }`,
		)
		assert_nested_raw_body(
			ht,
			ordered,
			"property_nested_raw_target",
			`{ name: string; }`,
		)
	}
}

func (tc property_nested_raw_case) ordered_inputs() []*GoTypeSrc {
	var inputs []*GoTypeSrc
	if tc.include_plain {
		inputs = append(inputs, GoType[property_nested_raw_plain_host]("PlainHost"))
	}
	if tc.include_ref {
		inputs = append(
			inputs,
			GoType[property_nested_raw_ref_host]("RefHost"),
			GoType[property_nested_raw_target](),
		)
	}
	return inputs
}

func (tc property_nested_raw_case) shuffled_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	if tc.swap_input_order && len(inputs) == 2 {
		inputs[0], inputs[1] = inputs[1], inputs[0]
	}
	return inputs
}

func (tc property_nested_raw_case) duplicated_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	if tc.include_plain && tc.duplicate_plain {
		inputs = append(inputs, GoType[property_nested_raw_plain_host]("PlainHost"))
	}
	if tc.include_ref && tc.duplicate_ref {
		inputs = append(inputs, GoType[property_nested_raw_ref_host]("RefHost"))
	}
	return inputs
}

func assert_nested_raw_body(
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
