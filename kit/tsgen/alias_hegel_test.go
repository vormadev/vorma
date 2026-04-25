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

type property_alias_user struct {
	Name string `json:"name"`
}

type property_alias_list []property_alias_user

type property_alias_map map[string]property_alias_user

type property_alias_array [2]property_alias_user

type property_alias_list_host struct {
	Items property_alias_list `json:"items"`
}

type property_alias_list_ptr_host struct {
	Items *property_alias_list `json:"items"`
}

type property_alias_map_host struct {
	Index property_alias_map `json:"index"`
}

type property_alias_array_host struct {
	Fixed property_alias_array `json:"fixed"`
}

type property_alias_case struct {
	included []int
}

type property_alias_case_space struct{}

var property_alias_catalog = []struct {
	input         *GoTypeSrc
	expected_body string
	expected_ids  []ID
}{
	{
		input:         GoType[property_alias_list_host]("AliasListHost"),
		expected_body: `{ items: property_alias_list; }`,
		expected_ids: []ID{
			GoType[property_alias_list_host]("AliasListHost").ID(),
			GoType[property_alias_list]().ID(),
			GoType[property_alias_user]().ID(),
		},
	},
	{
		input:         GoType[property_alias_list_ptr_host]("AliasListPtrHost"),
		expected_body: `{ items?: property_alias_list; }`,
		expected_ids: []ID{
			GoType[property_alias_list_ptr_host]("AliasListPtrHost").ID(),
			GoType[property_alias_list]().ID(),
			GoType[property_alias_user]().ID(),
		},
	},
	{
		input:         GoType[property_alias_map_host]("AliasMapHost"),
		expected_body: `{ index: property_alias_map; }`,
		expected_ids: []ID{
			GoType[property_alias_map_host]("AliasMapHost").ID(),
			GoType[property_alias_map]().ID(),
			GoType[property_alias_user]().ID(),
		},
	},
	{
		input:         GoType[property_alias_array_host]("AliasArrayHost"),
		expected_body: `{ fixed: property_alias_array; }`,
		expected_ids: []ID{
			GoType[property_alias_array_host]("AliasArrayHost").ID(),
			GoType[property_alias_array]().ID(),
			GoType[property_alias_user]().ID(),
		},
	},
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesKeepNamedCollectionAliasesReachable(t *testing.T) {
	t.Run("generated_alias_subset", hegel.Case(func(ht *hegel.T) {
		tc := (property_alias_case_space{}).draw_case(ht)
		tc.assert_alias_closure(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_alias_case_space) draw_case(
	ht *hegel.T,
) property_alias_case {
	var included []int
	for index := range property_alias_catalog {
		if hegel.Draw(ht, hegel.Booleans()) {
			included = append(included, index)
		}
	}
	if len(included) == 0 {
		included = append(
			included,
			hegel.Draw(ht, hegel.Integers(0, len(property_alias_catalog)-1)),
		)
	}
	return property_alias_case{included: included}
}

func (tc property_alias_case) assert_alias_closure(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	inputs := make([]*GoTypeSrc, 0, len(tc.included))
	expected_ids := make(map[ID]struct{})
	expected_bodies := make(map[string]string)

	for _, index := range tc.included {
		entry := property_alias_catalog[index]
		inputs = append(inputs, entry.input)
		expected_bodies[entry.input.RequestedName] = entry.expected_body
		for _, id := range entry.expected_ids {
			expected_ids[id] = struct{}{}
		}
	}

	defs := resolve_registry(inputs...)

	actual_ids := make(map[ID]struct{}, len(defs))
	for id := range defs {
		actual_ids[id] = struct{}{}
	}

	if !reflect.DeepEqual(actual_ids, expected_ids) {
		ht.Fatalf("actual ids = %#v, expected ids = %#v", actual_ids, expected_ids)
	}

	for name, expected_body := range expected_bodies {
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
}
