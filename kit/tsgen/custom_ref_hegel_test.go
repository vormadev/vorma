package tsgen

import (
	"fmt"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_custom_ref_target struct {
	Name string `json:"name"`
}

type property_custom_ref_wrapper struct {
	Target property_custom_ref_target `json:"target"`
}

type property_custom_ref_tstyper_host struct{}

func (property_custom_ref_tstyper_host) TSType() map[string]string {
	return map[string]string{
		"target": string(GoType[property_custom_ref_target]().ID()),
	}
}

type property_custom_ref_raw_host struct{}

func (property_custom_ref_raw_host) TSType() string {
	return fmt.Sprintf(
		"{ target: %s; }",
		GoType[property_custom_ref_target]().ID(),
	)
}

type property_custom_ref_case struct {
	include_tstyper bool
	include_raw     bool
}

type property_custom_ref_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesCustomRefsFollowCanonicalAlias(t *testing.T) {
	t.Run("generated_custom_ref_subset", hegel.Case(func(ht *hegel.T) {
		tc := (property_custom_ref_case_space{}).draw_case(ht)
		tc.assert_custom_refs(ht)
	}, hegel.WithTestCases(300)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_custom_ref_case_space) draw_case(
	ht *hegel.T,
) property_custom_ref_case {
	tc := property_custom_ref_case{
		include_tstyper: hegel.Draw(ht, hegel.Booleans()),
		include_raw:     hegel.Draw(ht, hegel.Booleans()),
	}
	if !tc.include_tstyper && !tc.include_raw {
		tc.include_tstyper = true
	}
	return tc
}

func (tc property_custom_ref_case) assert_custom_refs(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	inputs := []*GoTypeSrc{
		GoType[property_custom_ref_target]("AliasTarget"),
		GoType[property_custom_ref_wrapper]("Wrapper"),
	}
	expected_names := []string{"AliasTarget", "Wrapper"}

	if tc.include_tstyper {
		inputs = append(inputs, GoType[property_custom_ref_tstyper_host]("TSTyperHost"))
		expected_names = append(expected_names, "TSTyperHost")
	}
	if tc.include_raw {
		inputs = append(inputs, GoType[property_custom_ref_raw_host]("RawHost"))
		expected_names = append(expected_names, "RawHost")
	}

	defs := resolve_registry(inputs...)

	for _, name := range expected_names {
		if find_by_name(defs, name) == nil {
			ht.Fatalf("missing type %q in defs %#v", name, defs)
		}
	}

	assert_custom_ref_body(ht, defs, "Wrapper", `{ target: AliasTarget; }`)
	if tc.include_tstyper {
		assert_custom_ref_body(ht, defs, "TSTyperHost", `{ target: AliasTarget; }`)
	}
	if tc.include_raw {
		assert_custom_ref_body(ht, defs, "RawHost", `{ target: AliasTarget; }`)
	}
}

func assert_custom_ref_body(
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
