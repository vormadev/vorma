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

type property_cycle_a struct {
	Next *property_cycle_b `json:"next"`
}

type property_cycle_b struct {
	Next *property_cycle_c `json:"next"`
}

type property_cycle_c struct {
	Next *property_cycle_a `json:"next"`
}

type property_cycle_case struct {
	included      []int
	duplicate_set map[int]bool
	shuffle_steps []int
}

type property_cycle_case_space struct{}

var property_cycle_catalog = []struct {
	alias string
	input *GoTypeSrc
}{
	{alias: "CycleA", input: GoType[property_cycle_a]("CycleA")},
	{alias: "CycleB", input: GoType[property_cycle_b]("CycleB")},
	{alias: "CycleC", input: GoType[property_cycle_c]("CycleC")},
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesCyclicReferenceClosureMatchesModel(t *testing.T) {
	t.Run("generated_cycle_subset", hegel.Case(func(ht *hegel.T) {
		tc := (property_cycle_case_space{}).draw_case(ht)
		tc.assert_cycle_model(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_cycle_case_space) draw_case(
	ht *hegel.T,
) property_cycle_case {
	var included []int
	duplicate_set := make(map[int]bool)
	for index := range property_cycle_catalog {
		if hegel.Draw(ht, hegel.Booleans()) {
			included = append(included, index)
			duplicate_set[index] = hegel.Draw(ht, hegel.Booleans())
		}
	}
	if len(included) == 0 {
		index := hegel.Draw(ht, hegel.Integers(0, len(property_cycle_catalog)-1))
		included = append(included, index)
		duplicate_set[index] = hegel.Draw(ht, hegel.Booleans())
	}

	shuffle_steps := make([]int, len(included))
	for i := range included {
		shuffle_steps[i] = hegel.Draw(ht, hegel.Integers(0, len(included)-1))
	}

	return property_cycle_case{
		included:      included,
		duplicate_set: duplicate_set,
		shuffle_steps: shuffle_steps,
	}
}

func (tc property_cycle_case) assert_cycle_model(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	ordered := resolve_registry(tc.ordered_inputs()...)
	shuffled := resolve_registry(tc.shuffled_inputs()...)
	with_duplicates := resolve_registry(tc.duplicated_inputs()...)

	expected_ids := tc.expected_ids()
	expected_bodies := tc.expected_bodies()

	for label, defs := range map[string]ResolvedTSTypes{
		"ordered":         ordered,
		"shuffled":        shuffled,
		"with_duplicates": with_duplicates,
	} {
		actual_ids := make(map[ID]struct{}, len(defs))
		for id := range defs {
			actual_ids[id] = struct{}{}
		}
		if !reflect.DeepEqual(actual_ids, expected_ids) {
			ht.Fatalf(
				"%s ids = %#v, expected ids = %#v",
				label,
				actual_ids,
				expected_ids,
			)
		}
		for name, expected_body := range expected_bodies {
			def := find_by_name(defs, name)
			if def == nil {
				ht.Fatalf("%s missing type %q in defs %#v", label, name, defs)
			}
			if norm(def.Body) != norm(expected_body) {
				ht.Fatalf(
					"%s type %q body = %q, expected %q",
					label,
					name,
					def.Body,
					expected_body,
				)
			}
		}
	}
}

func (tc property_cycle_case) ordered_inputs() []*GoTypeSrc {
	inputs := make([]*GoTypeSrc, 0, len(tc.included))
	for _, index := range tc.included {
		inputs = append(inputs, property_cycle_catalog[index].input)
	}
	return inputs
}

func (tc property_cycle_case) shuffled_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	for i := range inputs {
		j := tc.shuffle_steps[i]
		inputs[i], inputs[j] = inputs[j], inputs[i]
	}
	return inputs
}

func (tc property_cycle_case) duplicated_inputs() []*GoTypeSrc {
	inputs := make([]*GoTypeSrc, 0, len(tc.included)*2)
	for _, index := range tc.included {
		input := property_cycle_catalog[index].input
		inputs = append(inputs, input)
		if tc.duplicate_set[index] {
			inputs = append(inputs, input)
		}
	}
	return inputs
}

func (tc property_cycle_case) expected_ids() map[ID]struct{} {
	expected := make(map[ID]struct{}, 3)
	expected[tc.id_for(0)] = struct{}{}
	expected[tc.id_for(1)] = struct{}{}
	expected[tc.id_for(2)] = struct{}{}
	return expected
}

func (tc property_cycle_case) expected_bodies() map[string]string {
	name_a := tc.name_for(0)
	name_b := tc.name_for(1)
	name_c := tc.name_for(2)
	return map[string]string{
		name_a: fmt.Sprintf("{ next?: %s; }", name_b),
		name_b: fmt.Sprintf("{ next?: %s; }", name_c),
		name_c: fmt.Sprintf("{ next?: %s; }", name_a),
	}
}

func (tc property_cycle_case) id_for(index int) ID {
	switch index {
	case 0:
		if tc.includes(0) {
			return GoType[property_cycle_a]("CycleA").ID()
		}
		return GoType[property_cycle_a]().ID()
	case 1:
		if tc.includes(1) {
			return GoType[property_cycle_b]("CycleB").ID()
		}
		return GoType[property_cycle_b]().ID()
	case 2:
		if tc.includes(2) {
			return GoType[property_cycle_c]("CycleC").ID()
		}
		return GoType[property_cycle_c]().ID()
	default:
		panic("unreachable")
	}
}

func (tc property_cycle_case) name_for(index int) string {
	if tc.includes(index) {
		return property_cycle_catalog[index].alias
	}
	switch index {
	case 0:
		return "property_cycle_a"
	case 1:
		return "property_cycle_b"
	case 2:
		return "property_cycle_c"
	default:
		panic("unreachable")
	}
}

func (tc property_cycle_case) includes(index int) bool {
	for _, included := range tc.included {
		if included == index {
			return true
		}
	}
	return false
}
