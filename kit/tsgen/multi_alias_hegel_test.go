package tsgen

import (
	"fmt"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_multi_alias_shared struct {
	Name string `json:"name"`
}

type property_multi_alias_host struct {
	Value property_multi_alias_shared `json:"value"`
}

type property_multi_alias_case struct {
	first_alias      string
	second_alias     string
	swap_root_order  bool
	duplicate_first  bool
	duplicate_second bool
}

type property_multi_alias_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesMultiAliasReferencesUseStableCanonicalName(t *testing.T) {
	t.Run("generated_alias_pairs", hegel.Case(func(ht *hegel.T) {
		tc := (property_multi_alias_case_space{}).draw_case(ht)
		tc.assert_canonical_alias(ht)
	}, hegel.WithTestCases(400)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_multi_alias_case_space) draw_case(
	ht *hegel.T,
) property_multi_alias_case {
	pool := []string{
		"AliasA",
		"AliasB",
		"AliasC",
		"Alpha",
		"Beta",
		"Gamma",
		"Left",
		"Right",
	}

	first_index := hegel.Draw(ht, hegel.Integers(0, len(pool)-1))
	second_index := hegel.Draw(ht, hegel.Integers(0, len(pool)-2))
	if second_index >= first_index {
		second_index++
	}

	return property_multi_alias_case{
		first_alias:      pool[first_index],
		second_alias:     pool[second_index],
		swap_root_order:  hegel.Draw(ht, hegel.Booleans()),
		duplicate_first:  hegel.Draw(ht, hegel.Booleans()),
		duplicate_second: hegel.Draw(ht, hegel.Booleans()),
	}
}

func (tc property_multi_alias_case) assert_canonical_alias(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	inputs := tc.registry_inputs()
	defs := resolve_registry(inputs...)

	expected_alias := tc.expected_alias()
	assert_multi_alias_body(ht, defs, "Host", fmt.Sprintf("{ value: %s; }", expected_alias))
	assert_multi_alias_body(ht, defs, tc.first_alias, `{ name: string; }`)
	assert_multi_alias_body(ht, defs, tc.second_alias, `{ name: string; }`)
}

func (tc property_multi_alias_case) registry_inputs() []*GoTypeSrc {
	first := GoType[property_multi_alias_shared](tc.first_alias)
	second := GoType[property_multi_alias_shared](tc.second_alias)
	host := GoType[property_multi_alias_host]("Host")

	var inputs []*GoTypeSrc
	if tc.swap_root_order {
		inputs = append(inputs, second, first)
	} else {
		inputs = append(inputs, first, second)
	}
	if tc.duplicate_first {
		inputs = append(inputs, first)
	}
	if tc.duplicate_second {
		inputs = append(inputs, second)
	}
	inputs = append(inputs, host)
	return inputs
}

func (tc property_multi_alias_case) expected_alias() string {
	if tc.first_alias < tc.second_alias {
		return tc.first_alias
	}
	return tc.second_alias
}

func assert_multi_alias_body(
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
