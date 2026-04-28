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

type property_nested_collision_target struct {
	Name string `json:"name"`
}

type property_nested_collision_other struct {
	Value int `json:"value"`
}

type property_nested_collision_dep struct{}

func (property_nested_collision_dep) TSType() string {
	return fmt.Sprintf(
		"{ target: %s; }",
		GoType[property_nested_collision_target]().ID(),
	)
}

type property_nested_collision_host struct {
	Dep property_nested_collision_dep `json:"dep"`
}

type property_nested_collision_case struct {
	first_alias      string
	second_alias     string
	collision_mode   int
	swap_root_order  bool
	duplicate_first  bool
	duplicate_second bool
}

type property_nested_collision_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesNestedRawCollisionRefsTrackCanonicalAlias(t *testing.T) {
	t.Run("generated_nested_collision_refs", hegel.Case(func(ht *hegel.T) {
		tc := (property_nested_collision_case_space{}).draw_case(ht)
		tc.assert_nested_collision_model(ht)
	}, hegel.WithTestCases(300)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_nested_collision_case_space) draw_case(
	ht *hegel.T,
) property_nested_collision_case {
	pool := []string{
		"AliasA",
		"AliasB",
		"AliasC",
		"Alpha",
		"Beta",
	}

	first_index := hegel.Draw(ht, hegel.Integers(0, len(pool)-1))
	second_index := hegel.Draw(ht, hegel.Integers(0, len(pool)-2))
	if second_index >= first_index {
		second_index++
	}

	return property_nested_collision_case{
		first_alias:      pool[first_index],
		second_alias:     pool[second_index],
		collision_mode:   hegel.Draw(ht, hegel.Integers(0, 2)),
		swap_root_order:  hegel.Draw(ht, hegel.Booleans()),
		duplicate_first:  hegel.Draw(ht, hegel.Booleans()),
		duplicate_second: hegel.Draw(ht, hegel.Booleans()),
	}
}

func (tc property_nested_collision_case) assert_nested_collision_model(
	ht *hegel.T,
) {
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

	expected_ref_name := tc.expected_ref_name(ordered, ht)
	assert_nested_collision_body(ht, ordered, "Host", `{ dep: property_nested_collision_dep; }`)
	assert_nested_collision_body(
		ht,
		ordered,
		"property_nested_collision_dep",
		fmt.Sprintf("{ target: %s; }", expected_ref_name),
	)
}

func (tc property_nested_collision_case) ordered_inputs() []*GoTypeSrc {
	first := GoType[property_nested_collision_target](tc.first_alias)
	second := GoType[property_nested_collision_target](tc.second_alias)
	collision := GoType[property_nested_collision_other](tc.collision_name())
	host := GoType[property_nested_collision_host]("Host")

	var inputs []*GoTypeSrc
	if tc.swap_root_order {
		inputs = append(inputs, second, first)
	} else {
		inputs = append(inputs, first, second)
	}
	inputs = append(inputs, collision, host)
	return inputs
}

func (tc property_nested_collision_case) shuffled_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	if len(inputs) >= 4 {
		inputs[0], inputs[3] = inputs[3], inputs[0]
		inputs[1], inputs[2] = inputs[2], inputs[1]
	}
	return inputs
}

func (tc property_nested_collision_case) duplicated_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	if tc.duplicate_first {
		inputs = append(inputs, GoType[property_nested_collision_target](tc.first_alias))
	}
	if tc.duplicate_second {
		inputs = append(inputs, GoType[property_nested_collision_target](tc.second_alias))
	}
	return inputs
}

func (tc property_nested_collision_case) collision_name() string {
	switch tc.collision_mode {
	case 0:
		return tc.first_alias
	case 1:
		return tc.second_alias
	default:
		return "OtherRoot"
	}
}

func (tc property_nested_collision_case) canonical_alias() string {
	if tc.first_alias < tc.second_alias {
		return tc.first_alias
	}
	return tc.second_alias
}

func (tc property_nested_collision_case) expected_ref_name(
	defs ResolvedTSTypes,
	ht *hegel.T,
) string {
	canonical_id := GoType[property_nested_collision_target](tc.canonical_alias()).ID()
	def, ok := defs[canonical_id]
	if !ok {
		ht.Fatalf("missing canonical target id %q in defs %#v", canonical_id, defs)
	}
	return def.Name
}

func assert_nested_collision_body(
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
