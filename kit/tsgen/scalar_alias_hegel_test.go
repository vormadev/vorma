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

type property_scalar_alias_id string

type property_scalar_alias_count int

type property_scalar_alias_flag bool

type property_scalar_alias_direct_host struct {
	ID    property_scalar_alias_id    `json:"id"`
	Count property_scalar_alias_count `json:"count"`
	Flag  property_scalar_alias_flag  `json:"flag"`
}

type property_scalar_alias_composite_host struct {
	IDs   []property_scalar_alias_id                              `json:"ids"`
	Count *property_scalar_alias_count                            `json:"count"`
	Flags map[property_scalar_alias_id]property_scalar_alias_flag `json:"flags"`
}

type property_scalar_alias_case struct {
	include_direct    bool
	include_composite bool
	swap_root_order   bool
	duplicate_id      bool
	duplicate_count   bool
	duplicate_flag    bool
}

type property_scalar_alias_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesNamedScalarAliasesMatchModel(t *testing.T) {
	t.Run("generated_scalar_alias_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_scalar_alias_case_space{}).draw_case(ht)
		tc.assert_scalar_alias_model(ht)
	}, hegel.WithTestCases(300)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_scalar_alias_case_space) draw_case(
	ht *hegel.T,
) property_scalar_alias_case {
	tc := property_scalar_alias_case{
		include_direct:    hegel.Draw(ht, hegel.Booleans()),
		include_composite: hegel.Draw(ht, hegel.Booleans()),
		swap_root_order:   hegel.Draw(ht, hegel.Booleans()),
		duplicate_id:      hegel.Draw(ht, hegel.Booleans()),
		duplicate_count:   hegel.Draw(ht, hegel.Booleans()),
		duplicate_flag:    hegel.Draw(ht, hegel.Booleans()),
	}
	if !tc.include_direct && !tc.include_composite {
		tc.include_direct = true
	}
	return tc
}

func (tc property_scalar_alias_case) assert_scalar_alias_model(ht *hegel.T) {
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

	assert_scalar_alias_body(ht, ordered, "ScalarID", `string`)
	assert_scalar_alias_body(ht, ordered, "ScalarCount", `number`)
	assert_scalar_alias_body(ht, ordered, "ScalarFlag", `boolean`)

	if tc.include_direct {
		assert_scalar_alias_body(
			ht,
			ordered,
			"DirectHost",
			`{ id: ScalarID; count: ScalarCount; flag: ScalarFlag; }`,
		)
	}
	if tc.include_composite {
		assert_scalar_alias_body(
			ht,
			ordered,
			"CompositeHost",
			`{ ids: Array<ScalarID>; count?: ScalarCount; flags: Record<ScalarID, ScalarFlag>; }`,
		)
	}
}

func (tc property_scalar_alias_case) ordered_inputs() []*GoTypeSrc {
	id := GoType[property_scalar_alias_id]("ScalarID")
	count := GoType[property_scalar_alias_count]("ScalarCount")
	flag := GoType[property_scalar_alias_flag]("ScalarFlag")

	var roots []*GoTypeSrc
	if tc.swap_root_order {
		roots = append(roots, flag, count, id)
	} else {
		roots = append(roots, id, count, flag)
	}

	if tc.include_direct {
		roots = append(roots, GoType[property_scalar_alias_direct_host]("DirectHost"))
	}
	if tc.include_composite {
		roots = append(roots, GoType[property_scalar_alias_composite_host]("CompositeHost"))
	}
	return roots
}

func (tc property_scalar_alias_case) shuffled_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	if len(inputs) >= 4 {
		inputs[0], inputs[3] = inputs[3], inputs[0]
	}
	if len(inputs) >= 5 {
		inputs[1], inputs[4] = inputs[4], inputs[1]
	}
	return inputs
}

func (tc property_scalar_alias_case) duplicated_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	if tc.duplicate_id {
		inputs = append(inputs, GoType[property_scalar_alias_id]("ScalarID"))
	}
	if tc.duplicate_count {
		inputs = append(inputs, GoType[property_scalar_alias_count]("ScalarCount"))
	}
	if tc.duplicate_flag {
		inputs = append(inputs, GoType[property_scalar_alias_flag]("ScalarFlag"))
	}
	return inputs
}

func assert_scalar_alias_body(
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
