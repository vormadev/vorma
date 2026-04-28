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

type property_collision_ref_shared struct {
	Name string `json:"name"`
}

type property_collision_ref_other struct {
	Value int `json:"value"`
}

type property_collision_ref_host struct {
	Ref property_collision_ref_shared `json:"ref"`
}

type property_collision_ref_tstyper_host struct{}

func (property_collision_ref_tstyper_host) TSType() map[string]string {
	return map[string]string{
		"ref": string(GoType[property_collision_ref_shared]().ID()),
	}
}

type property_collision_ref_raw_host struct{}

func (property_collision_ref_raw_host) TSType() string {
	return fmt.Sprintf(
		"{ ref: %s; }",
		GoType[property_collision_ref_shared]().ID(),
	)
}

type property_collision_ref_case struct {
	first_alias      string
	second_alias     string
	collision_mode   int
	include_reflect  bool
	include_tstyper  bool
	include_raw      bool
	swap_root_order  bool
	duplicate_first  bool
	duplicate_second bool
}

type property_collision_ref_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestResolveTypesCollisionRefsTrackCanonicalSharedAlias(t *testing.T) {
	t.Run("generated_collision_refs", hegel.Case(func(ht *hegel.T) {
		tc := (property_collision_ref_case_space{}).draw_case(ht)
		tc.assert_collision_refs(ht)
	}, hegel.WithTestCases(400)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_collision_ref_case_space) draw_case(
	ht *hegel.T,
) property_collision_ref_case {
	pool := []string{
		"AliasA",
		"AliasB",
		"AliasC",
		"Alpha",
		"Beta",
		"Gamma",
	}

	first_index := hegel.Draw(ht, hegel.Integers(0, len(pool)-1))
	second_index := hegel.Draw(ht, hegel.Integers(0, len(pool)-2))
	if second_index >= first_index {
		second_index++
	}

	tc := property_collision_ref_case{
		first_alias:      pool[first_index],
		second_alias:     pool[second_index],
		collision_mode:   hegel.Draw(ht, hegel.Integers(0, 2)),
		include_reflect:  hegel.Draw(ht, hegel.Booleans()),
		include_tstyper:  hegel.Draw(ht, hegel.Booleans()),
		include_raw:      hegel.Draw(ht, hegel.Booleans()),
		swap_root_order:  hegel.Draw(ht, hegel.Booleans()),
		duplicate_first:  hegel.Draw(ht, hegel.Booleans()),
		duplicate_second: hegel.Draw(ht, hegel.Booleans()),
	}
	if !tc.include_reflect && !tc.include_tstyper && !tc.include_raw {
		tc.include_reflect = true
	}
	return tc
}

func (tc property_collision_ref_case) assert_collision_refs(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))

	ordered_defs := resolve_registry(tc.ordered_inputs()...)
	shuffled_defs := resolve_registry(tc.shuffled_inputs()...)
	duplicate_defs := resolve_registry(tc.duplicated_inputs()...)

	if !reflect.DeepEqual(ordered_defs, shuffled_defs) {
		ht.Fatalf("ordered = %#v, shuffled = %#v", ordered_defs, shuffled_defs)
	}
	if !reflect.DeepEqual(ordered_defs, duplicate_defs) {
		ht.Fatalf("ordered = %#v, duplicate = %#v", ordered_defs, duplicate_defs)
	}

	expected_ref_name := tc.expected_ref_name(ordered_defs, ht)

	if tc.include_reflect {
		assert_collision_ref_body(
			ht,
			ordered_defs,
			"ReflectHost",
			fmt.Sprintf("{ ref: %s; }", expected_ref_name),
		)
	}
	if tc.include_tstyper {
		assert_collision_ref_body(
			ht,
			ordered_defs,
			"TSTyperHost",
			fmt.Sprintf("{ ref: %s; }", expected_ref_name),
		)
	}
	if tc.include_raw {
		assert_collision_ref_body(
			ht,
			ordered_defs,
			"RawHost",
			fmt.Sprintf("{ ref: %s; }", expected_ref_name),
		)
	}
}

func (tc property_collision_ref_case) ordered_inputs() []*GoTypeSrc {
	first := GoType[property_collision_ref_shared](tc.first_alias)
	second := GoType[property_collision_ref_shared](tc.second_alias)
	collision := GoType[property_collision_ref_other](tc.collision_name())

	var inputs []*GoTypeSrc
	if tc.swap_root_order {
		inputs = append(inputs, second, first)
	} else {
		inputs = append(inputs, first, second)
	}
	inputs = append(inputs, collision)
	if tc.include_reflect {
		inputs = append(inputs, GoType[property_collision_ref_host]("ReflectHost"))
	}
	if tc.include_tstyper {
		inputs = append(inputs, GoType[property_collision_ref_tstyper_host]("TSTyperHost"))
	}
	if tc.include_raw {
		inputs = append(inputs, GoType[property_collision_ref_raw_host]("RawHost"))
	}
	return inputs
}

func (tc property_collision_ref_case) shuffled_inputs() []*GoTypeSrc {
	inputs := tc.ordered_inputs()
	if len(inputs) >= 3 {
		inputs[0], inputs[2] = inputs[2], inputs[0]
	}
	if len(inputs) >= 4 {
		inputs[1], inputs[3] = inputs[3], inputs[1]
	}
	return inputs
}

func (tc property_collision_ref_case) duplicated_inputs() []*GoTypeSrc {
	inputs := make([]*GoTypeSrc, 0, len(tc.ordered_inputs())+2)
	ordered := tc.ordered_inputs()
	inputs = append(inputs, ordered...)
	first := GoType[property_collision_ref_shared](tc.first_alias)
	second := GoType[property_collision_ref_shared](tc.second_alias)
	if tc.duplicate_first {
		inputs = append(inputs, first)
	}
	if tc.duplicate_second {
		inputs = append(inputs, second)
	}
	return inputs
}

func (tc property_collision_ref_case) collision_name() string {
	switch tc.collision_mode {
	case 0:
		return tc.first_alias
	case 1:
		return tc.second_alias
	default:
		return "OtherRoot"
	}
}

func (tc property_collision_ref_case) expected_ref_name(
	defs ResolvedTSTypes,
	ht *hegel.T,
) string {
	canonical_id := GoType[property_collision_ref_shared](tc.canonical_alias()).ID()
	def, ok := defs[canonical_id]
	if !ok {
		ht.Fatalf("missing canonical shared id %q in defs %#v", canonical_id, defs)
	}
	return def.Name
}

func (tc property_collision_ref_case) canonical_alias() string {
	if tc.first_alias < tc.second_alias {
		return tc.first_alias
	}
	return tc.second_alias
}

func assert_collision_ref_body(
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
