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

type property_retention_user struct {
	Name string `json:"name"`
}

type property_retention_customer struct {
	User property_retention_user `json:"user"`
}

type property_retention_collection struct {
	Items []property_retention_user          `json:"items"`
	Index map[string]property_retention_user `json:"index"`
}

type property_retention_base struct {
	Name string `json:"name"`
}

type property_retention_flat_host struct {
	ID int `json:"id"`
	property_retention_base
}

type property_retention_tagged_host struct {
	Base property_retention_base `json:"base"`
}

type property_retention_node struct {
	Next *property_retention_node `json:"next"`
}

type property_retention_case struct {
	included []int
}

type property_retention_case_space struct{}

var property_retention_catalog = []struct {
	input        *GoTypeSrc
	expected_ids []ID
}{
	{
		input: GoType[property_retention_customer]("RetentionCustomer"),
		expected_ids: []ID{
			GoType[property_retention_customer]("RetentionCustomer").ID(),
			GoType[property_retention_user]().ID(),
		},
	},
	{
		input: GoType[property_retention_collection]("RetentionCollection"),
		expected_ids: []ID{
			GoType[property_retention_collection]("RetentionCollection").ID(),
			GoType[property_retention_user]().ID(),
		},
	},
	{
		input: GoType[property_retention_flat_host]("RetentionFlatHost"),
		expected_ids: []ID{
			GoType[property_retention_flat_host]("RetentionFlatHost").ID(),
		},
	},
	{
		input: GoType[property_retention_tagged_host]("RetentionTaggedHost"),
		expected_ids: []ID{
			GoType[property_retention_tagged_host]("RetentionTaggedHost").ID(),
			GoType[property_retention_base]().ID(),
		},
	},
	{
		input: GoType[property_retention_node]("RetentionNode"),
		expected_ids: []ID{
			GoType[property_retention_node]("RetentionNode").ID(),
		},
	},
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesRetainsExactlyReferencedNamedTypes(t *testing.T) {
	t.Run("generated_root_subset", hegel.Case(func(ht *hegel.T) {
		tc := (property_retention_case_space{}).draw_case(ht)
		tc.assert_retention_closure(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_retention_case_space) draw_case(
	ht *hegel.T,
) property_retention_case {
	var included []int
	for index := range property_retention_catalog {
		if hegel.Draw(ht, hegel.Booleans()) {
			included = append(included, index)
		}
	}
	if len(included) == 0 {
		included = append(
			included,
			hegel.Draw(ht, hegel.Integers(0, len(property_retention_catalog)-1)),
		)
	}
	return property_retention_case{included: included}
}

func (tc property_retention_case) assert_retention_closure(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	inputs := make([]*GoTypeSrc, 0, len(tc.included))
	expected := make(map[ID]struct{})
	for _, index := range tc.included {
		entry := property_retention_catalog[index]
		inputs = append(inputs, entry.input)
		for _, id := range entry.expected_ids {
			expected[id] = struct{}{}
		}
	}

	actual := resolve_registry(inputs...)

	actual_ids := make(map[ID]struct{}, len(actual))
	for id := range actual {
		actual_ids[id] = struct{}{}
	}

	if !reflect.DeepEqual(actual_ids, expected) {
		ht.Fatalf("actual ids = %#v, expected ids = %#v", actual_ids, expected)
	}
}
