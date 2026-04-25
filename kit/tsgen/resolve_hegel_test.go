package tsgen

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_tsgen_user struct {
	Name string `json:"name"`
}

type property_tsgen_customer struct {
	User property_tsgen_user `json:"user"`
}

type property_tsgen_event struct {
	At time.Time `json:"at"`
}

type property_tsgen_collision_a struct {
	A int `json:"a"`
}

type property_tsgen_collision_b struct {
	B string `json:"b"`
}

type property_tsgen_embed_base struct {
	Name string `json:"name"`
}

type property_tsgen_embed_host struct {
	ID int `json:"id"`
	property_tsgen_embed_base
}

type property_tsgen_raw struct{}

func (property_tsgen_raw) TSType() string {
	return "{ custom: boolean }"
}

type property_tsgen_registry_case struct {
	included        []int
	duplicate_modes []bool
	shuffle_steps   []int
}

type property_tsgen_registry_case_space struct{}

var property_tsgen_registry_catalog = []*GoTypeSrc{
	{Instance: property_tsgen_customer{}, RequestedName: "CustomerResponse"},
	{Instance: property_tsgen_event{}, RequestedName: "EventPayload"},
	{Instance: property_tsgen_collision_a{}, RequestedName: "Collision"},
	{Instance: property_tsgen_collision_b{}, RequestedName: "Collision"},
	{Instance: property_tsgen_raw{}, RequestedName: "RawPayload"},
	{Instance: property_tsgen_embed_host{}, RequestedName: "EmbedHost"},
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesOrderAndDuplicationInvariant(t *testing.T) {
	t.Run("generated_registry_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_tsgen_registry_case_space{}).draw_case(ht)
		tc.assert_invariant(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_tsgen_registry_case_space) draw_case(
	ht *hegel.T,
) property_tsgen_registry_case {
	var included []int
	var duplicate_modes []bool
	for index := range property_tsgen_registry_catalog {
		if hegel.Draw(ht, hegel.Booleans()) {
			included = append(included, index)
			duplicate_modes = append(duplicate_modes, hegel.Draw(ht, hegel.Booleans()))
		}
	}
	if len(included) == 0 {
		included = append(
			included,
			hegel.Draw(ht, hegel.Integers(0, len(property_tsgen_registry_catalog)-1)),
		)
		duplicate_modes = append(duplicate_modes, hegel.Draw(ht, hegel.Booleans()))
	}

	shuffle_steps := make([]int, len(included))
	for i := range included {
		shuffle_steps[i] = hegel.Draw(ht, hegel.Integers(0, len(included)-1))
	}

	return property_tsgen_registry_case{
		included:        included,
		duplicate_modes: duplicate_modes,
		shuffle_steps:   shuffle_steps,
	}
}

func (tc property_tsgen_registry_case) assert_invariant(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	ordered := tc.ordered_inputs()
	shuffled := tc.shuffled_inputs()
	with_duplicates := tc.duplicated_inputs()

	ordered_defs := resolve_registry(ordered...)
	shuffled_defs := resolve_registry(shuffled...)
	duplicate_defs := resolve_registry(with_duplicates...)

	if !reflect.DeepEqual(ordered_defs, shuffled_defs) {
		ht.Fatalf("ordered = %#v, shuffled = %#v", ordered_defs, shuffled_defs)
	}
	if !reflect.DeepEqual(ordered_defs, duplicate_defs) {
		ht.Fatalf("ordered = %#v, duplicate = %#v", ordered_defs, duplicate_defs)
	}
	if !resolved_defs_are_well_formed(ordered_defs) {
		ht.Fatalf("resolved defs are not well formed: %#v", ordered_defs)
	}
}

func (tc property_tsgen_registry_case) ordered_inputs() []*GoTypeSrc {
	inputs := make([]*GoTypeSrc, 0, len(tc.included))
	for _, index := range tc.included {
		inputs = append(inputs, property_tsgen_registry_catalog[index])
	}
	return inputs
}

func (tc property_tsgen_registry_case) shuffled_inputs() []*GoTypeSrc {
	inputs := append([]*GoTypeSrc(nil), tc.ordered_inputs()...)
	for i := range inputs {
		j := tc.shuffle_steps[i]
		inputs[i], inputs[j] = inputs[j], inputs[i]
	}
	return inputs
}

func (tc property_tsgen_registry_case) duplicated_inputs() []*GoTypeSrc {
	inputs := make([]*GoTypeSrc, 0, len(tc.included)*2)
	for i, index := range tc.included {
		input := property_tsgen_registry_catalog[index]
		inputs = append(inputs, input)
		if tc.duplicate_modes[i] {
			inputs = append(inputs, input)
		}
	}
	return inputs
}

func resolve_registry(input_types ...*GoTypeSrc) ResolvedTSTypes {
	registry := &GoTypeRegistry{}
	registry.Add(input_types...)
	defs, err := registry.ResolveTypes()
	if err != nil {
		panic(err)
	}
	return defs
}

func resolved_defs_are_well_formed(defs ResolvedTSTypes) bool {
	seen_names := make(map[string]struct{})
	ids := make([]string, 0, len(defs))
	for id := range defs {
		ids = append(ids, string(id))
	}

	for _, def := range defs {
		if def.Name != "" {
			if _, exists := seen_names[def.Name]; exists {
				return false
			}
			seen_names[def.Name] = struct{}{}
		}
		for _, id := range ids {
			if strings.Contains(def.Body, id) {
				return false
			}
		}
	}

	return true
}
