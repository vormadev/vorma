package matcher

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// BEST MATCH PROPERTIES
/////////////////////////////////////////////////////////////////////

const (
	property_static_segment property_segment_kind = iota
	property_dynamic_segment
	property_splat_segment
	property_index_segment
)

var property_best_match_patterns = property_pattern_catalog{
	"",
	"/",
	"/*",
	"/a",
	"/a/",
	"/b",
	"/b/",
	"/a/b",
	"/a/b/",
	"/a/c",
	"/b/a",
	"/:x",
	"/:x/",
	"/a/:x",
	"/a/:x/",
	"/:x/a",
	"/:x/:y",
	"/a/b/:x",
	"/a/:x/c",
	"/a/*",
	"/b/*",
	"/:x/*",
	"/a/b/*",
}

var property_nested_match_patterns = property_pattern_catalog{
	"",
	"/",
	"/*",
	"/a",
	"/a/",
	"/a/*",
	"/a/:x",
	"/a/:x/",
	"/a/:x/*",
	"/a/:x/b",
	"/a/:x/b/",
	"/a/b",
	"/a/b/",
	"/a/b/*",
	"/a/b/:x",
	"/a/b/:x/",
	"/b",
	"/b/",
	"/b/*",
	"/b/:x",
	"/b/:x/*",
	"/b/a",
	"/b/a/",
	"/b/a/*",
	"/:x",
	"/:x/",
	"/:x/*",
	"/:x/a",
	"/:x/a/",
	"/:x/a/*",
	"/:x/a/b",
	"/:x/:y",
	"/:x/:y/",
	"/:x/:y/*",
}

var property_nested_explicit_index_patterns = property_pattern_catalog{
	"/",
	"/_index",
	"/a",
	"/a/_index",
	"/a/*",
	"/a/:x",
	"/a/:x/_index",
	"/a/:x/*",
	"/a/b",
	"/a/b/_index",
	"/a/b/*",
	"/b",
	"/b/_index",
	"/b/*",
	"/b/:x",
	"/b/:x/_index",
	"/:x",
	"/:x/_index",
	"/:x/*",
	"/:x/a",
	"/:x/a/_index",
	"/:x/a/*",
	"/:x/:y",
	"/:x/:y/_index",
	"/:x/:y/*",
}

var property_unrelated_static_patterns = property_pattern_catalog{
	"/z",
	"/z/",
	"/z/a",
	"/z/a/",
	"/z/a/b",
	"/z/b",
	"/z/b/",
	"/z/c",
	"/z/id",
	"/zz",
	"/zz/a",
	"/zz/b",
}

var property_path_segments = []string{"a", "b", "c", "id", "0"}

var property_generated_static_segments = []string{
	"a",
	"b",
	"c",
	"id",
	"0",
	"left",
	"right",
}

var property_generated_dynamic_names = []string{
	"x",
	"y",
	"z",
	"id",
	"slug",
	"page",
}

var property_generated_dynamic_name_pairs = [][2]string{
	{"x", "y"},
	{"id", "slug"},
	{"page", "section"},
	{"left", "right"},
}

type property_segment_kind uint8

type property_route_case struct {
	catalog         property_pattern_catalog
	pattern_indexes []int
	path_segments   []string
	trailing_slash  bool
}

type property_registration_order uint8

type property_model_pattern_string string

type property_model_path_string string

type property_pattern_catalog []string

type property_generated_pattern_space struct{}

type property_collision_pattern_space struct{}

type property_normalized_collision_space struct{}

type property_invalid_pattern_space struct{}

type property_generated_explicit_index_pattern_space struct{}

type property_model_pattern struct {
	original           string
	normalized         string
	segments           []property_model_segment
	registration_order int
	is_static          bool
}

type property_model_segment struct {
	val  string
	kind property_segment_kind
}

type property_model_match struct {
	normalized_pattern string
	params             Params
	splat_values       []string
	score              int
	segment_ranks      []int
	registration_order int
	is_static          bool
	last_is_dynamic    bool
	last_is_index      bool
	last_is_splat      bool
	segment_len        int
	dynamic_params     int
}

type property_nested_observation struct {
	found        bool
	patterns     []string
	params       Params
	splat_values []string
	match_params []Params
	match_splats [][]string
}

type property_best_match_observation struct {
	found        bool
	pattern      string
	params       Params
	splat_values []string
}

type property_generated_pattern_kind uint8

type property_generated_segment_kind uint8

type property_collision_case struct {
	segments []property_collision_segment
}

type property_collision_segment struct {
	kind         property_generated_segment_kind
	static_value string
	name_pair    [2]string
}

type property_normalized_collision_case struct {
	kind              property_normalized_collision_kind
	prefix            string
	explicit_index_id string
}

type property_normalized_collision_kind uint8

const (
	property_original_order property_registration_order = iota
	property_reversed_order
	property_sorted_order
)

const (
	property_generated_empty_pattern property_generated_pattern_kind = iota
	property_generated_root_pattern
	property_generated_segmented_pattern
)

const (
	property_generated_static_kind property_generated_segment_kind = iota
	property_generated_dynamic_kind
	property_generated_splat_kind
	property_generated_index_kind
)

const (
	property_explicit_index_collision property_normalized_collision_kind = iota
	property_custom_dynamic_collision
)

func TestFindBestMatchMatchesModel(t *testing.T) {
	t.Run("default_options", hegel.Case(func(ht *hegel.T) {
		tc := property_best_match_patterns.draw_case(ht, 14, 4)
		tc.assert_best_match_matches_model(ht, &Options{Quiet: true})
	}, hegel.WithTestCases(500)))

	t.Run("custom_markers", hegel.Case(func(ht *hegel.T) {
		tc := property_best_match_patterns.draw_case(ht, 14, 4)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{DynamicParamPrefix: '$'},
			{SplatSegmentIdentifier: '#'},
			{DynamicParamPrefix: '<', SplatSegmentIdentifier: '>'},
		}))
		tc.assert_best_match_matches_model(ht, opts)
	}, hegel.WithTestCases(500)))

	t.Run("explicit_index_generated_patterns", hegel.Case(func(ht *hegel.T) {
		tc := (property_generated_explicit_index_pattern_space{}).draw_case(
			ht,
			18,
			5,
			5,
		)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{ExplicitIndexSegmentIdentifier: "_index", Quiet: true},
			{
				ExplicitIndexSegmentIdentifier: "_______",
				DynamicParamPrefix:             '<',
				SplatSegmentIdentifier:         '>',
				Quiet:                          true,
			},
		}))
		tc.assert_best_match_matches_model_with_options(ht, opts, "_index")
	}, hegel.WithTestCases(500)))
}

func TestFindBestMatchIgnoresUnrelatedStaticRoutes(t *testing.T) {
	t.Run("best_match_catalog", hegel.Case(func(ht *hegel.T) {
		tc := property_best_match_patterns.draw_case(ht, 14, 4)
		extra_patterns := property_unrelated_static_patterns.draw_patterns(ht, 8)

		base := tc.best_match_observation_with_order(
			&Options{Quiet: true},
			property_original_order,
		)
		expanded := tc.best_match_observation_with_extra_patterns(extra_patterns)

		ht.Note(fmt.Sprintf("patterns = %v", tc.patterns()))
		ht.Note(fmt.Sprintf("extra_patterns = %v", extra_patterns))
		ht.Note(fmt.Sprintf("path = %q", tc.path()))

		if !base.equal(expanded) {
			ht.Fatalf("base best match = %#v, expanded = %#v", base, expanded)
		}
	}, hegel.WithTestCases(500)))
}

func TestFindBestMatchGeneratedPatternsMatchModel(t *testing.T) {
	t.Run("generated_patterns", hegel.Case(func(ht *hegel.T) {
		tc := (property_generated_pattern_space{}).draw_case(ht, 18, 5, 5)
		tc.assert_best_match_matches_model(ht, &Options{Quiet: true})
	}, hegel.WithTestCases(500)))

	t.Run("generated_patterns_custom_markers", hegel.Case(func(ht *hegel.T) {
		tc := (property_generated_pattern_space{}).draw_case(ht, 18, 5, 5)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{DynamicParamPrefix: '$', Quiet: true},
			{SplatSegmentIdentifier: '#', Quiet: true},
			{DynamicParamPrefix: '<', SplatSegmentIdentifier: '>', Quiet: true},
		}))
		tc.assert_best_match_matches_model(ht, opts)
	}, hegel.WithTestCases(500)))
}

func TestRegisterPatternRejectsGeneratedRouteShapeCollisions(t *testing.T) {
	t.Run("generated_shapes", hegel.Case(func(ht *hegel.T) {
		tc := (property_collision_pattern_space{}).draw_case(ht, 5)
		tc.assert_route_shape_collision(ht)
	}, hegel.WithTestCases(500)))

	t.Run("generated_shapes_custom_markers", hegel.Case(func(ht *hegel.T) {
		tc := (property_collision_pattern_space{}).draw_case(ht, 5)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{DynamicParamPrefix: '$', Quiet: true},
			{DynamicParamPrefix: '<', SplatSegmentIdentifier: '>', Quiet: true},
		}))
		tc.assert_route_shape_collision_with_options(ht, opts)
	}, hegel.WithTestCases(500)))
}

func TestRegisterPatternRejectsGeneratedNormalizedCollisions(t *testing.T) {
	t.Run("generated_normalized_collisions", hegel.Case(func(ht *hegel.T) {
		tc := (property_normalized_collision_space{}).draw_case(ht)
		tc.assert_normalized_collision(ht)
	}, hegel.WithTestCases(500)))
}

func TestRegisterPatternRejectsInvalidExplicitIndexTrailingSlashPatterns(t *testing.T) {
	t.Run("generated_invalid_patterns", hegel.Case(func(ht *hegel.T) {
		pattern, opts := (property_invalid_pattern_space{}).draw_case(ht)
		panic_message := register_pattern_panic_message(opts, pattern)
		ht.Note(fmt.Sprintf("pattern = %q", pattern))
		ht.Note(fmt.Sprintf(
			"dynamic = %q, splat = %q, index = %q",
			opts.DynamicParamPrefix,
			opts.SplatSegmentIdentifier,
			opts.ExplicitIndexSegmentIdentifier,
		))
		if !strings.Contains(
			panic_message,
			"trailing slashes are not permitted when using an explicit index segment",
		) {
			ht.Fatalf("panic = %q, want explicit-index trailing-slash rejection", panic_message)
		}
	}, hegel.WithTestCases(500)))
}

func TestFindNestedMatchesRegistrationOrderIndependent(t *testing.T) {
	t.Run("best_match_catalog", hegel.Case(func(ht *hegel.T) {
		tc := property_best_match_patterns.draw_case(ht, 14, 4)
		tc.assert_nested_registration_order_independent(ht)
	}, hegel.WithTestCases(500)))

	t.Run("nested_catalog", hegel.Case(func(ht *hegel.T) {
		tc := property_nested_match_patterns.draw_case(ht, 18, 5)
		tc.assert_nested_registration_order_independent(ht)
	}, hegel.WithTestCases(500)))

	t.Run("nested_catalog_custom_markers", hegel.Case(func(ht *hegel.T) {
		tc := property_nested_match_patterns.draw_case(ht, 18, 5)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{DynamicParamPrefix: '$', Quiet: true},
			{SplatSegmentIdentifier: '#', Quiet: true},
			{DynamicParamPrefix: '<', SplatSegmentIdentifier: '>', Quiet: true},
		}))
		tc.assert_nested_registration_order_independent_with_options(ht, opts, "")
	}, hegel.WithTestCases(500)))

	t.Run("explicit_index_catalog", hegel.Case(func(ht *hegel.T) {
		tc := property_nested_explicit_index_patterns.draw_case(ht, 18, 5)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{ExplicitIndexSegmentIdentifier: "_index", Quiet: true},
			{
				ExplicitIndexSegmentIdentifier: "_______",
				DynamicParamPrefix:             '<',
				SplatSegmentIdentifier:         '>',
				Quiet:                          true,
			},
		}))
		tc.assert_nested_registration_order_independent_with_options(
			ht,
			opts,
			"_index",
		)
	}, hegel.WithTestCases(500)))

	t.Run("explicit_index_generated_patterns", hegel.Case(func(ht *hegel.T) {
		tc := (property_generated_explicit_index_pattern_space{}).draw_case(
			ht,
			18,
			5,
			5,
		)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{ExplicitIndexSegmentIdentifier: "_index", Quiet: true},
			{
				ExplicitIndexSegmentIdentifier: "_______",
				DynamicParamPrefix:             '<',
				SplatSegmentIdentifier:         '>',
				Quiet:                          true,
			},
		}))
		tc.assert_nested_registration_order_independent_with_options(
			ht,
			opts,
			"_index",
		)
	}, hegel.WithTestCases(500)))
}

func TestFindNestedMatchesIgnoresUnrelatedStaticRoutes(t *testing.T) {
	t.Run("nested_catalog", hegel.Case(func(ht *hegel.T) {
		tc := property_nested_match_patterns.draw_case(ht, 18, 5)
		extra_patterns := property_unrelated_static_patterns.draw_patterns(ht, 8)

		base := tc.nested_observation(property_original_order)
		expanded := tc.nested_observation_with_extra_patterns(extra_patterns)

		ht.Note(fmt.Sprintf("patterns = %v", tc.patterns()))
		ht.Note(fmt.Sprintf("extra_patterns = %v", extra_patterns))
		ht.Note(fmt.Sprintf("path = %q", tc.path()))

		if !base.equal(expanded) {
			ht.Fatalf("base nested result = %#v, expanded = %#v", base, expanded)
		}
	}, hegel.WithTestCases(500)))
}

func TestFindNestedMatchesMatchesSemanticModel(t *testing.T) {
	t.Run("best_match_catalog", hegel.Case(func(ht *hegel.T) {
		tc := property_best_match_patterns.draw_case(ht, 14, 4)
		tc.assert_nested_matches_model(ht, &Options{Quiet: true}, "")
	}, hegel.WithTestCases(500)))

	t.Run("default_options", hegel.Case(func(ht *hegel.T) {
		tc := property_nested_match_patterns.draw_case(ht, 18, 5)
		tc.assert_nested_matches_model(ht, &Options{Quiet: true}, "")
	}, hegel.WithTestCases(500)))

	t.Run("generated_patterns", hegel.Case(func(ht *hegel.T) {
		tc := (property_generated_pattern_space{}).draw_case(ht, 18, 5, 5)
		tc.assert_nested_matches_model(ht, &Options{Quiet: true}, "")
	}, hegel.WithTestCases(500)))

	t.Run("custom_markers", hegel.Case(func(ht *hegel.T) {
		tc := property_nested_match_patterns.draw_case(ht, 18, 5)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{DynamicParamPrefix: '$', Quiet: true},
			{SplatSegmentIdentifier: '#', Quiet: true},
			{DynamicParamPrefix: '<', SplatSegmentIdentifier: '>', Quiet: true},
		}))
		tc.assert_nested_matches_model(ht, opts, "")
	}, hegel.WithTestCases(500)))

	t.Run("generated_patterns_custom_markers", hegel.Case(func(ht *hegel.T) {
		tc := (property_generated_pattern_space{}).draw_case(ht, 18, 5, 5)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{DynamicParamPrefix: '$', Quiet: true},
			{SplatSegmentIdentifier: '#', Quiet: true},
			{DynamicParamPrefix: '<', SplatSegmentIdentifier: '>', Quiet: true},
		}))
		tc.assert_nested_matches_model(ht, opts, "")
	}, hegel.WithTestCases(500)))

	t.Run("explicit_index", hegel.Case(func(ht *hegel.T) {
		tc := property_nested_explicit_index_patterns.draw_case(ht, 18, 5)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{ExplicitIndexSegmentIdentifier: "_index", Quiet: true},
			{
				ExplicitIndexSegmentIdentifier: "_______",
				DynamicParamPrefix:             '<',
				SplatSegmentIdentifier:         '>',
				Quiet:                          true,
			},
		}))
		tc.assert_nested_matches_model(ht, opts, "_index")
	}, hegel.WithTestCases(500)))

	t.Run("explicit_index_generated_patterns", hegel.Case(func(ht *hegel.T) {
		tc := (property_generated_explicit_index_pattern_space{}).draw_case(
			ht,
			18,
			5,
			5,
		)
		opts := hegel.Draw(ht, hegel.SampledFrom([]*Options{
			{ExplicitIndexSegmentIdentifier: "_index", Quiet: true},
			{
				ExplicitIndexSegmentIdentifier: "_______",
				DynamicParamPrefix:             '<',
				SplatSegmentIdentifier:         '>',
				Quiet:                          true,
			},
		}))
		tc.assert_nested_matches_model(ht, opts, "_index")
	}, hegel.WithTestCases(500)))
}

func TestNestedSemanticModelAgreesWithExistingTableCases(t *testing.T) {
	catalog := property_pattern_catalog(nested_patterns)
	pattern_indexes := catalog.indexes(nested_patterns)
	for _, opts := range nested_opts_to_test {
		uses_explicit := opts_uses_explicit_index(opts)
		for _, tc := range nested_scenarios {
			model_tc := property_route_case{
				catalog:         catalog,
				pattern_indexes: pattern_indexes,
				path_segments: property_model_path_string(
					tc.path,
				).non_index_segments(),
				trailing_slash: property_model_path_string(
					tc.path,
				).has_non_root_trailing_slash(),
			}
			got := model_tc.nested_expectation_with_options(opts, "_index")
			want_patterns := adjust_expected_matches(
				tc.expected_matches,
				uses_explicit,
			)
			want_found := len(want_patterns) > 0
			if got.found != want_found {
				t.Fatalf(
					"%q: model found = %v, want %v",
					tc.path,
					got.found,
					want_found,
				)
			}
			if !want_found {
				continue
			}
			if !reflect.DeepEqual(got.patterns, want_patterns) {
				t.Fatalf(
					"%q: model patterns = %v, want %v",
					tc.path,
					got.patterns,
					want_patterns,
				)
			}
			if !equal_params(got.params, tc.params) {
				t.Fatalf("%q: model params = %v, want %v", tc.path, got.params, tc.params)
			}
			if !equal_splat(got.splat_values, tc.splat_values) {
				t.Fatalf(
					"%q: model splat = %v, want %v",
					tc.path,
					got.splat_values,
					tc.splat_values,
				)
			}
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// PROPERTY HELPERS
/////////////////////////////////////////////////////////////////////

func (c property_pattern_catalog) draw_case(
	ht *hegel.T,
	max_patterns int,
	max_path_segments int,
) property_route_case {
	path_segment_count := hegel.Draw(ht, hegel.Integers(0, max_path_segments))
	path_segments := make([]string, path_segment_count)
	for i := range path_segments {
		path_segments[i] = hegel.Draw(ht, hegel.SampledFrom(property_path_segments))
	}

	return property_route_case{
		catalog:         c,
		pattern_indexes: c.draw_indexes(ht, max_patterns),
		path_segments:   path_segments,
		trailing_slash:  hegel.Draw(ht, hegel.Booleans()),
	}
}

func (c property_pattern_catalog) draw_patterns(ht *hegel.T, max_patterns int) []string {
	indexes := c.draw_indexes(ht, max_patterns)
	index_seen := make(map[int]bool, len(indexes))
	patterns := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		if index_seen[idx] {
			continue
		}
		index_seen[idx] = true
		patterns = append(patterns, c[idx])
	}
	return patterns
}

func (c property_pattern_catalog) draw_indexes(ht *hegel.T, max_patterns int) []int {
	index_count := hegel.Draw(ht, hegel.Integers(0, max_patterns))
	indexes := make([]int, index_count)
	for i := range indexes {
		indexes[i] = hegel.Draw(ht, hegel.Integers(0, len(c)-1))
	}
	return indexes
}

func (property_generated_pattern_space) draw_case(
	ht *hegel.T,
	max_patterns int,
	max_pattern_segments int,
	max_path_segments int,
) property_route_case {
	pattern_count := hegel.Draw(ht, hegel.Integers(0, max_patterns))
	patterns := make([]string, 0, pattern_count)
	normalized_seen := make(map[string]bool, pattern_count)
	shape_seen := make(map[string]bool, pattern_count)
	for range pattern_count {
		pattern := (property_generated_pattern_space{}).draw_pattern(
			ht,
			max_pattern_segments,
		)
		model := property_model_pattern_string(pattern).model(len(patterns))
		if normalized_seen[model.normalized] || shape_seen[model.shape_key()] {
			continue
		}
		normalized_seen[model.normalized] = true
		shape_seen[model.shape_key()] = true
		patterns = append(patterns, pattern)
	}

	path_segment_count := hegel.Draw(ht, hegel.Integers(0, max_path_segments))
	path_segments := make([]string, path_segment_count)
	for i := range path_segments {
		path_segments[i] = hegel.Draw(ht, hegel.SampledFrom(property_path_segments))
	}

	pattern_indexes := make([]int, len(patterns))
	for i := range pattern_indexes {
		pattern_indexes[i] = i
	}

	return property_route_case{
		catalog:         property_pattern_catalog(patterns),
		pattern_indexes: pattern_indexes,
		path_segments:   path_segments,
		trailing_slash:  hegel.Draw(ht, hegel.Booleans()),
	}
}

func (property_generated_explicit_index_pattern_space) draw_case(
	ht *hegel.T,
	max_patterns int,
	max_pattern_segments int,
	max_path_segments int,
) property_route_case {
	pattern_count := hegel.Draw(ht, hegel.Integers(0, max_patterns))
	patterns := make([]string, 0, pattern_count)
	normalized_seen := make(map[string]bool, pattern_count)
	shape_seen := make(map[string]bool, pattern_count)
	for range pattern_count {
		pattern := (property_generated_explicit_index_pattern_space{}).draw_pattern(
			ht,
			max_pattern_segments,
		)
		model := property_model_pattern_string(pattern).model_with_options(
			len(patterns),
			&Options{ExplicitIndexSegmentIdentifier: "_index", Quiet: true},
			"_index",
		)
		if normalized_seen[model.normalized] || shape_seen[model.shape_key()] {
			continue
		}
		normalized_seen[model.normalized] = true
		shape_seen[model.shape_key()] = true
		patterns = append(patterns, pattern)
	}

	path_segment_count := hegel.Draw(ht, hegel.Integers(0, max_path_segments))
	path_segments := make([]string, path_segment_count)
	for i := range path_segments {
		path_segments[i] = hegel.Draw(ht, hegel.SampledFrom(property_path_segments))
	}

	pattern_indexes := make([]int, len(patterns))
	for i := range pattern_indexes {
		pattern_indexes[i] = i
	}

	return property_route_case{
		catalog:         property_pattern_catalog(patterns),
		pattern_indexes: pattern_indexes,
		path_segments:   path_segments,
		trailing_slash:  hegel.Draw(ht, hegel.Booleans()),
	}
}

func (property_generated_explicit_index_pattern_space) draw_pattern(
	ht *hegel.T,
	max_pattern_segments int,
) string {
	pattern_kind := hegel.Draw(ht, hegel.SampledFrom([]property_generated_pattern_kind{
		property_generated_empty_pattern,
		property_generated_root_pattern,
		property_generated_segmented_pattern,
	}))
	switch pattern_kind {
	case property_generated_empty_pattern:
		return ""
	case property_generated_root_pattern:
		return "/"
	}

	segment_count := hegel.Draw(ht, hegel.Integers(1, max_pattern_segments))
	segments := make([]string, segment_count)
	last_kind := (property_generated_explicit_index_pattern_space{}).draw_last_kind(ht)
	for i := range segments {
		is_last := i == len(segments)-1
		switch {
		case !is_last:
			segments[i] = (property_generated_pattern_space{}).draw_segment(ht, false)
		default:
			switch last_kind {
			case property_generated_splat_kind:
				segments[i] = "*"
			case property_generated_dynamic_kind:
				segments[i] = ":" + hegel.Draw(
					ht,
					hegel.SampledFrom(property_generated_dynamic_names),
				)
			case property_generated_static_kind:
				segments[i] = hegel.Draw(ht, hegel.SampledFrom(property_generated_static_segments))
			default:
				segments[i] = "_index"
			}
		}
	}
	return "/" + strings.Join(segments, "/")
}

func (property_generated_explicit_index_pattern_space) draw_last_kind(
	ht *hegel.T,
) property_generated_segment_kind {
	return hegel.Draw(ht, hegel.SampledFrom([]property_generated_segment_kind{
		property_generated_static_kind,
		property_generated_dynamic_kind,
		property_generated_splat_kind,
		property_generated_index_kind,
	}))
}

func (property_generated_pattern_space) draw_pattern(
	ht *hegel.T,
	max_pattern_segments int,
) string {
	pattern_kind := hegel.Draw(ht, hegel.SampledFrom([]property_generated_pattern_kind{
		property_generated_empty_pattern,
		property_generated_root_pattern,
		property_generated_segmented_pattern,
	}))
	switch pattern_kind {
	case property_generated_empty_pattern:
		return ""
	case property_generated_root_pattern:
		return "/"
	}

	segment_count := hegel.Draw(ht, hegel.Integers(1, max_pattern_segments))
	segments := make([]string, segment_count)
	for i := range segments {
		segments[i] = (property_generated_pattern_space{}).draw_segment(
			ht,
			i == len(segments)-1,
		)
	}

	pattern := "/" + strings.Join(segments, "/")
	if segments[len(segments)-1] == "*" {
		return pattern
	}
	if hegel.Draw(ht, hegel.Booleans()) {
		pattern += "/"
	}
	return pattern
}

func (property_generated_pattern_space) draw_segment(
	ht *hegel.T,
	is_last bool,
) string {
	segment_kinds := []property_generated_segment_kind{
		property_generated_static_kind,
		property_generated_dynamic_kind,
	}
	if is_last {
		segment_kinds = append(segment_kinds, property_generated_splat_kind)
	}
	segment_kind := hegel.Draw(ht, hegel.SampledFrom(segment_kinds))
	switch segment_kind {
	case property_generated_static_kind:
		return hegel.Draw(ht, hegel.SampledFrom(property_generated_static_segments))
	case property_generated_dynamic_kind:
		return ":" + hegel.Draw(ht, hegel.SampledFrom(property_generated_dynamic_names))
	default:
		return "*"
	}
}

func (property_collision_pattern_space) draw_case(
	ht *hegel.T,
	max_segments int,
) property_collision_case {
	segment_count := hegel.Draw(ht, hegel.Integers(1, max_segments))
	segments := make([]property_collision_segment, segment_count)
	has_dynamic := false
	for i := range segments {
		segment_kind := (property_generated_pattern_space{}).draw_segment_kind(
			ht,
			i == len(segments)-1,
		)
		segment := property_collision_segment{kind: segment_kind}
		switch segment_kind {
		case property_generated_static_kind:
			segment.static_value = hegel.Draw(
				ht,
				hegel.SampledFrom(property_generated_static_segments),
			)
		case property_generated_dynamic_kind:
			segment.name_pair = hegel.Draw(
				ht,
				hegel.SampledFrom(property_generated_dynamic_name_pairs),
			)
			has_dynamic = true
		}
		segments[i] = segment
	}
	if !has_dynamic {
		segments[0].kind = property_generated_dynamic_kind
		segments[0].static_value = ""
		segments[0].name_pair = hegel.Draw(
			ht,
			hegel.SampledFrom(property_generated_dynamic_name_pairs),
		)
	}
	return property_collision_case{segments: segments}
}

func (property_generated_pattern_space) draw_segment_kind(
	ht *hegel.T,
	is_last bool,
) property_generated_segment_kind {
	segment_kinds := []property_generated_segment_kind{
		property_generated_static_kind,
		property_generated_dynamic_kind,
	}
	if is_last {
		segment_kinds = append(segment_kinds, property_generated_splat_kind)
	}
	return hegel.Draw(ht, hegel.SampledFrom(segment_kinds))
}

func (property_normalized_collision_space) draw_case(
	ht *hegel.T,
) property_normalized_collision_case {
	kind := hegel.Draw(ht, hegel.SampledFrom([]property_normalized_collision_kind{
		property_explicit_index_collision,
		property_custom_dynamic_collision,
	}))
	return property_normalized_collision_case{
		kind: kind,
		prefix: hegel.Draw(ht, hegel.SampledFrom([]string{
			"/users",
			"/users/profile",
			"/a",
			"/a/b",
		})),
		explicit_index_id: hegel.Draw(ht, hegel.SampledFrom([]string{
			"_index",
			"_______",
			"home",
		})),
	}
}

func (property_invalid_pattern_space) draw_case(
	ht *hegel.T,
) (string, *Options) {
	prefix := hegel.Draw(ht, hegel.SampledFrom([]string{
		"/users",
		"/users/profile",
		"/a",
		"/a/b",
		"/dashboard/customers",
	}))
	explicit_index_id := hegel.Draw(ht, hegel.SampledFrom([]string{
		"_index",
		"_______",
		"home",
	}))
	return prefix + "/",
		&Options{ExplicitIndexSegmentIdentifier: explicit_index_id, Quiet: true}
}

func (tc property_route_case) assert_best_match_matches_model(
	ht *hegel.T,
	opts *Options,
) {
	tc.assert_best_match_matches_model_with_options(ht, opts, "")
}

func (tc property_route_case) assert_best_match_matches_model_with_options(
	ht *hegel.T,
	opts *Options,
	source_index string,
) {
	expected, expected_found := tc.expectation_with_options(opts, source_index)
	actual, actual_found := tc.actual_match_with_order_and_source_index(
		opts,
		source_index,
		property_original_order,
	)
	actual_reversed, actual_reversed_found := tc.actual_match_with_order_and_source_index(
		opts,
		source_index,
		property_reversed_order,
	)
	actual_sorted, actual_sorted_found := tc.actual_match_with_order_and_source_index(
		opts,
		source_index,
		property_sorted_order,
	)

	tc.note(ht, opts)

	if actual_found != expected_found {
		ht.Fatalf("FindBestMatch found = %v, want %v", actual_found, expected_found)
	}
	if actual_reversed_found != expected_found {
		ht.Fatalf(
			"FindBestMatch with reversed registration found = %v, want %v",
			actual_reversed_found,
			expected_found,
		)
	}
	if actual_sorted_found != expected_found {
		ht.Fatalf(
			"FindBestMatch with sorted registration found = %v, want %v",
			actual_sorted_found,
			expected_found,
		)
	}
	if !expected_found {
		return
	}

	if actual == nil {
		ht.Fatal("FindBestMatch returned nil with found=true")
	}
	if actual_reversed == nil {
		ht.Fatal("FindBestMatch with reversed registration returned nil with found=true")
	}
	if actual_sorted == nil {
		ht.Fatal("FindBestMatch with sorted registration returned nil with found=true")
	}
	expected.assert_equal_match(ht, "original", actual)
	expected.assert_equal_match(ht, "reversed", actual_reversed)
	expected.assert_equal_match(ht, "sorted", actual_sorted)
}

func (m property_model_match) assert_equal_match(
	ht *hegel.T,
	label string,
	actual *BestMatch,
) {
	if got := actual.NormalizedPattern(); got != m.normalized_pattern {
		ht.Fatalf(
			"%s NormalizedPattern() = %q, want %q",
			label,
			got,
			m.normalized_pattern,
		)
	}
	if !equal_params(actual.Params, m.params) {
		ht.Fatalf("%s Params = %v, want %v", label, actual.Params, m.params)
	}
	if !equal_splat(actual.SplatValues, m.splat_values) {
		ht.Fatalf(
			"%s SplatValues = %v, want %v",
			label,
			actual.SplatValues,
			m.splat_values,
		)
	}
}

func (o property_best_match_observation) equal(
	other property_best_match_observation,
) bool {
	return o.found == other.found &&
		o.pattern == other.pattern &&
		equal_params(o.params, other.params) &&
		equal_splat(o.splat_values, other.splat_values)
}

func (tc property_collision_case) assert_route_shape_collision(ht *hegel.T) {
	left, right := tc.pattern_pair()
	ht.Note(fmt.Sprintf("left = %q", left))
	ht.Note(fmt.Sprintf("right = %q", right))

	for _, patterns := range [][2]string{{left, right}, {right, left}} {
		panic_message := tc.register_collision_message(patterns[0], patterns[1])
		if !strings.Contains(panic_message, "route shape collision:") {
			ht.Fatalf("panic = %q, want route shape collision", panic_message)
		}
	}
}

func (tc property_collision_case) assert_route_shape_collision_with_options(
	ht *hegel.T,
	opts *Options,
) {
	left, right := tc.pattern_pair_with_options(opts)
	ht.Note(fmt.Sprintf("left = %q", left))
	ht.Note(fmt.Sprintf("right = %q", right))
	ht.Note(fmt.Sprintf(
		"dynamic = %q, splat = %q, index = %q",
		opts.DynamicParamPrefix,
		opts.SplatSegmentIdentifier,
		opts.ExplicitIndexSegmentIdentifier,
	))

	for _, patterns := range [][2]string{{left, right}, {right, left}} {
		panic_message := tc.register_collision_message_with_options(
			opts,
			patterns[0],
			patterns[1],
		)
		if !strings.Contains(panic_message, "route shape collision:") {
			ht.Fatalf("panic = %q, want route shape collision", panic_message)
		}
	}
}

func (tc property_normalized_collision_case) assert_normalized_collision(ht *hegel.T) {
	first, second, opts := tc.collision_inputs()
	ht.Note(fmt.Sprintf("first = %q", first))
	ht.Note(fmt.Sprintf("second = %q", second))
	ht.Note(fmt.Sprintf(
		"dynamic = %q, splat = %q, index = %q",
		opts.DynamicParamPrefix,
		opts.SplatSegmentIdentifier,
		opts.ExplicitIndexSegmentIdentifier,
	))

	panic_message := tc.register_collision_message_with_options(opts, first, second)
	if !strings.Contains(panic_message, "normalized pattern collision:") {
		ht.Fatalf("panic = %q, want normalized pattern collision", panic_message)
	}
}

func (tc property_route_case) note(ht *hegel.T, opts *Options) {
	ht.Note(fmt.Sprintf("patterns = %v", tc.patterns()))
	ht.Note(fmt.Sprintf("path = %q", tc.path()))
	ht.Note(fmt.Sprintf(
		"dynamic = %q, splat = %q, index = %q",
		opts.DynamicParamPrefix,
		opts.SplatSegmentIdentifier,
		opts.ExplicitIndexSegmentIdentifier,
	))
}

func (tc property_route_case) patterns() []string {
	index_seen := make(map[int]bool, len(tc.pattern_indexes))
	patterns := make([]string, 0, len(tc.pattern_indexes))
	for _, idx := range tc.pattern_indexes {
		if index_seen[idx] {
			continue
		}
		index_seen[idx] = true
		patterns = append(patterns, tc.catalog[idx])
	}
	return patterns
}

func (tc property_route_case) path() string {
	if len(tc.path_segments) == 0 {
		return "/"
	}
	path := "/" + strings.Join(tc.path_segments, "/")
	if tc.trailing_slash {
		path += "/"
	}
	return path
}

func (tc property_route_case) actual_match_with_order_and_source_index(
	opts *Options,
	source_index string,
	order property_registration_order,
) (*BestMatch, bool) {
	m := New(opts)
	patterns := tc.ordered_patterns(order)
	return tc.actual_match_with_patterns(m, opts, source_index, patterns)
}

func (tc property_route_case) actual_match_with_patterns(
	m *Matcher,
	opts *Options,
	source_index string,
	patterns []string,
) (*BestMatch, bool) {
	patterns = rewrite_patterns(
		patterns,
		source_index,
		option_shape_for_rewrites(opts),
	)
	for _, pattern := range patterns {
		m.RegisterPattern(pattern)
	}
	return m.FindBestMatch(tc.path())
}

func (tc property_route_case) best_match_observation_with_order(
	opts *Options,
	order property_registration_order,
) property_best_match_observation {
	m := New(opts)
	patterns := tc.ordered_patterns(order)
	return tc.best_match_observation_with_patterns(m, opts, "", patterns)
}

func (tc property_route_case) best_match_observation_with_extra_patterns(
	extra_patterns []string,
) property_best_match_observation {
	m := New(&Options{Quiet: true})
	patterns := tc.ordered_patterns(property_original_order)
	patterns = append(patterns, extra_patterns...)
	return tc.best_match_observation_with_patterns(
		m,
		&Options{Quiet: true},
		"",
		patterns,
	)
}

func (tc property_route_case) best_match_observation_with_patterns(
	m *Matcher,
	opts *Options,
	source_index string,
	patterns []string,
) property_best_match_observation {
	match, found := tc.actual_match_with_patterns(m, opts, source_index, patterns)
	observation := property_best_match_observation{found: found}
	if !found || match == nil {
		return observation
	}
	observation.pattern = match.NormalizedPattern()
	observation.params = match.Params
	observation.splat_values = match.SplatValues
	return observation
}

func (tc property_collision_case) pattern_pair() (string, string) {
	return tc.pattern_pair_with_options(&Options{})
}

func (tc property_collision_case) pattern_pair_with_options(
	opts *Options,
) (string, string) {
	left_segments := make([]string, len(tc.segments))
	right_segments := make([]string, len(tc.segments))
	dynamic_prefix := byte(or_default(opts.DynamicParamPrefix, ':'))
	splat_segment := string(or_default(opts.SplatSegmentIdentifier, '*'))
	for i, segment := range tc.segments {
		switch segment.kind {
		case property_generated_static_kind:
			left_segments[i] = segment.static_value
			right_segments[i] = segment.static_value
		case property_generated_dynamic_kind:
			left_segments[i] = string(dynamic_prefix) + segment.name_pair[0]
			right_segments[i] = string(dynamic_prefix) + segment.name_pair[1]
		default:
			left_segments[i] = splat_segment
			right_segments[i] = splat_segment
		}
	}
	return "/" + strings.Join(left_segments, "/"),
		"/" + strings.Join(right_segments, "/")
}

func (tc property_collision_case) register_collision_message(
	first string,
	second string,
) (panic_message string) {
	return tc.register_collision_message_with_options(
		&Options{Quiet: true},
		first,
		second,
	)
}

func (tc property_collision_case) register_collision_message_with_options(
	opts *Options,
	first string,
	second string,
) (panic_message string) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			panic_message = ""
			return
		}
		panic_message = fmt.Sprint(recovered)
	}()

	m := New(opts)
	m.RegisterPattern(first)
	m.RegisterPattern(second)
	return ""
}

func (tc property_normalized_collision_case) collision_inputs() (
	string,
	string,
	*Options,
) {
	switch tc.kind {
	case property_explicit_index_collision:
		return "",
			"/",
			&Options{
				ExplicitIndexSegmentIdentifier: tc.explicit_index_id,
				Quiet:                          true,
			}
	default:
		return tc.prefix + "/$id",
			tc.prefix + "/:id",
			&Options{DynamicParamPrefix: '$', Quiet: true}
	}
}

func (tc property_normalized_collision_case) register_collision_message_with_options(
	opts *Options,
	first string,
	second string,
) (panic_message string) {
	return register_patterns_panic_message(opts, first, second)
}

func register_pattern_panic_message(
	opts *Options,
	pattern string,
) string {
	return register_patterns_panic_message(opts, pattern)
}

func register_patterns_panic_message(
	opts *Options,
	patterns ...string,
) (panic_message string) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			panic_message = ""
			return
		}
		panic_message = fmt.Sprint(recovered)
	}()

	m := New(opts)
	for _, pattern := range patterns {
		m.RegisterPattern(pattern)
	}
	return ""
}

func (tc property_route_case) ordered_patterns(
	order property_registration_order,
) []string {
	patterns := tc.patterns()
	switch order {
	case property_reversed_order:
		slices.Reverse(patterns)
	case property_sorted_order:
		slices.Sort(patterns)
	}
	return patterns
}

func (tc property_route_case) nested_observation(
	order property_registration_order,
) property_nested_observation {
	return tc.nested_observation_with_options(
		&Options{Quiet: true},
		"",
		order,
	)
}

func (tc property_route_case) nested_observation_with_options(
	opts *Options,
	source_index string,
	order property_registration_order,
) property_nested_observation {
	patterns := tc.ordered_patterns(order)
	return tc.nested_observation_with_patterns(opts, source_index, patterns)
}

func (tc property_route_case) nested_observation_with_extra_patterns(
	extra_patterns []string,
) property_nested_observation {
	patterns := tc.ordered_patterns(property_original_order)
	patterns = append(patterns, extra_patterns...)
	return tc.nested_observation_with_patterns(
		&Options{Quiet: true},
		"",
		patterns,
	)
}

func (tc property_route_case) nested_observation_with_patterns(
	opts *Options,
	source_index string,
	patterns []string,
) property_nested_observation {
	m := New(opts)
	patterns = rewrite_patterns(
		patterns,
		source_index,
		option_shape_for_rewrites(opts),
	)
	for _, pattern := range patterns {
		m.RegisterPattern(pattern)
	}

	results, found := m.FindNestedMatches(tc.path())
	observation := property_nested_observation{found: found}
	if !found || results == nil {
		return observation
	}
	observation.params = results.Params
	observation.splat_values = results.SplatValues
	observation.patterns = make([]string, len(results.Matches))
	observation.match_params = make([]Params, len(results.Matches))
	observation.match_splats = make([][]string, len(results.Matches))
	for i, match := range results.Matches {
		observation.patterns[i] = match.NormalizedPattern()
		observation.match_params[i] = match.Params()
		observation.match_splats[i] = match.SplatValues()
	}
	return observation
}

func (tc property_route_case) assert_nested_registration_order_independent(ht *hegel.T) {
	tc.assert_nested_registration_order_independent_with_options(
		ht,
		&Options{Quiet: true},
		"",
	)
}

func (tc property_route_case) assert_nested_registration_order_independent_with_options(
	ht *hegel.T,
	opts *Options,
	source_index string,
) {
	forward := tc.nested_observation_with_options(
		opts,
		source_index,
		property_original_order,
	)
	reversed := tc.nested_observation_with_options(
		opts,
		source_index,
		property_reversed_order,
	)
	sorted := tc.nested_observation_with_options(
		opts,
		source_index,
		property_sorted_order,
	)

	tc.note(ht, opts)

	if !forward.equal(reversed) {
		ht.Fatalf("forward nested result = %#v, reversed = %#v", forward, reversed)
	}
	if !forward.equal(sorted) {
		ht.Fatalf("forward nested result = %#v, sorted = %#v", forward, sorted)
	}
}

func (tc property_route_case) assert_nested_matches_model(
	ht *hegel.T,
	opts *Options,
	source_index string,
) {
	expected := tc.nested_expectation_with_options(opts, source_index)
	forward := tc.nested_observation_with_options(
		opts,
		source_index,
		property_original_order,
	)
	reversed := tc.nested_observation_with_options(
		opts,
		source_index,
		property_reversed_order,
	)
	sorted := tc.nested_observation_with_options(
		opts,
		source_index,
		property_sorted_order,
	)

	tc.note(ht, opts)

	if !expected.equal(forward) {
		ht.Fatalf("model nested result = %#v, actual = %#v", expected, forward)
	}
	if !expected.equal(reversed) {
		ht.Fatalf(
			"model nested result = %#v, reversed actual = %#v",
			expected,
			reversed,
		)
	}
	if !expected.equal(sorted) {
		ht.Fatalf(
			"model nested result = %#v, sorted actual = %#v",
			expected,
			sorted,
		)
	}
}

func (o property_nested_observation) equal(other property_nested_observation) bool {
	return o.found == other.found &&
		reflect.DeepEqual(o.patterns, other.patterns) &&
		equal_params(o.params, other.params) &&
		equal_splat(o.splat_values, other.splat_values) &&
		reflect.DeepEqual(o.match_params, other.match_params) &&
		reflect.DeepEqual(o.match_splats, other.match_splats)
}

func (tc property_route_case) expectation() (property_model_match, bool) {
	return tc.expectation_with_options(&Options{}, "")
}

func (tc property_route_case) expectation_with_options(
	opts *Options,
	source_index string,
) (property_model_match, bool) {
	path := property_model_path_string(tc.path())
	patterns := tc.patterns()
	var best property_model_match
	found := false

	for i, pattern := range patterns {
		model_pattern := property_model_pattern_string(pattern).model_with_options(
			i,
			opts,
			source_index,
		)
		match, ok := model_pattern.match(path)
		if !ok {
			continue
		}
		if !found || match.better_than(best) {
			best = match
			found = true
		}
	}

	return best, found
}

func (tc property_route_case) nested_expectation_with_options(
	opts *Options,
	source_index string,
) property_nested_observation {
	real_path := property_model_path_string(tc.path()).strip_trailing_slash()
	path := property_model_path_string(real_path)
	path_segments := path.segments()
	matches := make(map[string]property_model_match)
	patterns := tc.patterns()

	if real_path == "" {
		for _, pattern := range patterns {
			model_pattern := property_model_pattern_string(pattern).model_with_options(
				0,
				opts,
				source_index,
			)
			if model_pattern.normalized == "" {
				matches[""] = model_pattern.base_match(nil, nil)
				continue
			}
			if model_pattern.normalized == "/" {
				matches["/"] = model_pattern.base_match(nil, nil)
			}
		}
		if _, has_slash := matches["/"]; !has_slash {
			for _, pattern := range patterns {
				model_pattern := property_model_pattern_string(pattern).model_with_options(
					0,
					opts,
					source_index,
				)
				if model_pattern.normalized == "/*" {
					matches["/*"] = model_pattern.base_match(nil, []string{})
					break
				}
			}
		}
		return path.flatten_nested_matches(matches, false)
	}

	found_full_static := false
	for _, pattern := range patterns {
		model_pattern := property_model_pattern_string(pattern).model_with_options(
			0,
			opts,
			source_index,
		)
		match, ok := model_pattern.nested_static_match(path_segments)
		if !ok {
			continue
		}
		matches[match.normalized_pattern] = match
		if match.segment_len == len(path_segments) && model_pattern.is_static &&
			!model_pattern.last_segment_is_index() {
			found_full_static = true
		}
	}

	if !found_full_static {
		model_patterns := make(map[string]property_model_pattern, len(patterns))
		for _, pattern := range patterns {
			model_pattern := property_model_pattern_string(pattern).model_with_options(
				0,
				opts,
				source_index,
			)
			model_patterns[model_pattern.normalized] = model_pattern
			if model_pattern.normalized == "/*" {
				matches["/*"] = model_pattern.base_match(
					nil,
					slices.Clone(path_segments),
				)
				continue
			}
			if model_pattern.is_static {
				continue
			}
			match, ok := model_pattern.nested_dynamic_match(path_segments)
			if !ok {
				continue
			}
			matches[match.normalized_pattern] = match
		}
		for normalized, model_pattern := range model_patterns {
			if model_pattern.is_static || !model_pattern.last_segment_is_index() {
				continue
			}
			parent_pattern := strings.TrimSuffix(normalized, "/")
			parent_match, ok := matches[parent_pattern]
			if !ok || parent_match.segment_len != len(path_segments) {
				continue
			}
			matches[normalized] = model_pattern.base_match(parent_match.params, nil)
		}
	}

	return path.flatten_nested_matches(matches, true)
}

func (p property_model_pattern_string) model(registration_order int) property_model_pattern {
	return p.model_with_options(registration_order, &Options{}, "")
}

func (p property_model_pattern_string) model_with_options(
	registration_order int,
	opts *Options,
	source_index string,
) property_model_pattern {
	input := rewrite_single_pattern(
		string(p),
		source_index,
		option_shape_for_rewrites(opts),
	)
	normalized := property_model_pattern_string(input).normalized_with_options(opts)
	segments := property_model_path_string(normalized).segments()
	model_segments := make([]property_model_segment, len(segments))
	is_static := true
	for i, segment := range segments {
		model_segment := property_model_segment{val: segment, kind: property_static_segment}
		switch {
		case segment == "":
			model_segment.kind = property_index_segment
		case segment == "*":
			model_segment.kind = property_splat_segment
			is_static = false
		case strings.HasPrefix(segment, ":"):
			model_segment.kind = property_dynamic_segment
			is_static = false
		}
		model_segments[i] = model_segment
	}
	return property_model_pattern{
		original:           input,
		normalized:         normalized,
		segments:           model_segments,
		registration_order: registration_order,
		is_static:          is_static,
	}
}

func (p property_model_pattern_string) normalized_with_options(opts *Options) string {
	if p == "" {
		return ""
	}
	dynamic_param_prefix := ':'
	splat_segment_identifier := '*'
	explicit_index_segment_identifier := ""
	if opts != nil {
		dynamic_param_prefix = or_default(opts.DynamicParamPrefix, ':')
		splat_segment_identifier = or_default(opts.SplatSegmentIdentifier, '*')
		explicit_index_segment_identifier = opts.ExplicitIndexSegmentIdentifier
	}

	normalized := string(p)
	if explicit_index_segment_identifier != "" {
		if strings.HasSuffix(normalized, "/") {
			if normalized != "/" {
				panic("test model got invalid trailing slash with explicit index")
			}
			normalized = strings.TrimRight(normalized, "/")
		}
		if strings.HasSuffix(normalized, "/"+explicit_index_segment_identifier) {
			normalized = strings.TrimSuffix(
				normalized,
				explicit_index_segment_identifier,
			)
		}
	}

	raw_segments := property_model_path_string(normalized).segments()
	segs := make([]property_model_segment, 0, len(raw_segments))
	for _, raw_segment := range raw_segments {
		segment := property_model_segment{
			val:  raw_segment,
			kind: property_static_segment,
		}
		switch {
		case raw_segment == "":
			segment.kind = property_index_segment
		case len(raw_segment) == 1 &&
			raw_segment == string(splat_segment_identifier):
			segment.val = "*"
			segment.kind = property_splat_segment
		case len(raw_segment) > 0 &&
			raw_segment[0] == byte(dynamic_param_prefix):
			segment.val = ":" + raw_segment[1:]
			segment.kind = property_dynamic_segment
		}
		segs = append(segs, segment)
	}

	last_type := property_static_segment
	if len(segs) > 0 {
		last_type = segs[len(segs)-1].kind
	}

	var sb strings.Builder
	sb.WriteString("/")
	for i, segment := range segs {
		sb.WriteString(segment.val)
		if i < len(segs)-1 {
			sb.WriteString("/")
		}
	}
	final := sb.String()
	if strings.HasSuffix(final, "/") && last_type != property_index_segment {
		final = strings.TrimRight(final, "/")
	}
	return final
}

func (p property_model_pattern) match(
	path property_model_path_string,
) (property_model_match, bool) {
	if p.is_static {
		return p.match_static(path)
	}
	return p.match_dynamic(path)
}

func (p property_model_pattern) match_static(
	path property_model_path_string,
) (property_model_match, bool) {
	path_string := string(path)
	if p.normalized != path_string {
		if !path.has_trailing_slash() {
			return property_model_match{}, false
		}
		if p.normalized != path.strip_trailing_slash() {
			return property_model_match{}, false
		}
	}
	return p.base_match(nil, nil), true
}

func (p property_model_pattern) match_dynamic(
	path property_model_path_string,
) (property_model_match, bool) {
	path_segments := path.segments()
	has_splat := len(p.segments) > 0 &&
		p.segments[len(p.segments)-1].kind == property_splat_segment
	if has_splat {
		splat_idx := len(p.segments) - 1
		if len(path_segments) < len(p.segments) {
			return property_model_match{}, false
		}
		if !p.match_prefix(path_segments[:splat_idx]) {
			return property_model_match{}, false
		}
		params := p.params(path_segments)
		splat_values := slices.Clone(path_segments[splat_idx:])
		return p.base_match(params, splat_values), true
	}

	if len(path_segments) == len(p.segments) && p.match_segments(path_segments) {
		return p.base_match(p.params(path_segments), nil), true
	}

	if path.has_trailing_slash() && len(path_segments) > 0 &&
		!p.last_segment_is_index() {
		trimmed_segments := path_segments[:len(path_segments)-1]
		if len(trimmed_segments) == len(p.segments) && p.match_segments(trimmed_segments) {
			return p.base_match(p.params(trimmed_segments), nil), true
		}
	}

	return property_model_match{}, false
}

func (p property_model_pattern) nested_static_match(
	path_segments []string,
) (property_model_match, bool) {
	if !p.is_static {
		return property_model_match{}, false
	}
	if p.last_segment_is_index() {
		base_segments := p.segments[:len(p.segments)-1]
		if len(base_segments) != len(path_segments) {
			return property_model_match{}, false
		}
		if !p.match_prefix(path_segments) {
			return property_model_match{}, false
		}
		return p.base_match(nil, nil), true
	}
	if len(p.segments) > len(path_segments) {
		return property_model_match{}, false
	}
	if !p.match_prefix(path_segments[:len(p.segments)]) {
		return property_model_match{}, false
	}
	return p.base_match(nil, nil), true
}

func (p property_model_pattern) nested_dynamic_match(
	path_segments []string,
) (property_model_match, bool) {
	if p.is_static || p.normalized == "/*" {
		return property_model_match{}, false
	}
	if p.last_segment_is_index() {
		return property_model_match{}, false
	}
	if p.last_segment_is_splat() {
		base_segments := p.segments[:len(p.segments)-1]
		if len(path_segments) <= len(base_segments) {
			return property_model_match{}, false
		}
		if !p.match_prefix(path_segments[:len(base_segments)]) {
			return property_model_match{}, false
		}
		return p.base_match(
			p.params(path_segments),
			slices.Clone(path_segments[len(base_segments):]),
		), true
	}
	if len(p.segments) > len(path_segments) {
		return property_model_match{}, false
	}
	if !p.match_prefix(path_segments[:len(p.segments)]) {
		return property_model_match{}, false
	}
	return p.base_match(p.params(path_segments), nil), true
}

func (p property_model_pattern) match_prefix(path_segments []string) bool {
	if len(path_segments) > len(p.segments) {
		return false
	}
	for i, path_segment := range path_segments {
		if !p.segments[i].matches(path_segment) {
			return false
		}
	}
	return true
}

func (p property_model_pattern) match_segments(path_segments []string) bool {
	if len(path_segments) != len(p.segments) {
		return false
	}
	for i, pattern_segment := range p.segments {
		if !pattern_segment.matches(path_segments[i]) {
			return false
		}
	}
	return true
}

func (s property_model_segment) matches(path_segment string) bool {
	switch s.kind {
	case property_static_segment:
		return s.val == path_segment
	case property_index_segment:
		return path_segment == ""
	case property_dynamic_segment:
		return path_segment != ""
	default:
		return false
	}
}

func (p property_model_pattern) params(path_segments []string) Params {
	var params Params
	for i, segment := range p.segments {
		if segment.kind != property_dynamic_segment {
			continue
		}
		if params == nil {
			params = make(Params)
		}
		params[strings.TrimPrefix(segment.val, ":")] = path_segments[i]
	}
	return params
}

func (p property_model_pattern) last_segment_is_index() bool {
	return len(p.segments) > 0 &&
		p.segments[len(p.segments)-1].kind == property_index_segment
}

func (p property_model_pattern) last_segment_is_dynamic() bool {
	return len(p.segments) > 0 &&
		p.segments[len(p.segments)-1].kind == property_dynamic_segment
}

func (p property_model_pattern) last_segment_is_splat() bool {
	return len(p.segments) > 0 &&
		p.segments[len(p.segments)-1].kind == property_splat_segment
}

func (p property_model_pattern) shape_key() string {
	var sb strings.Builder
	for i, segment := range p.segments {
		if i > 0 {
			sb.WriteString("/")
		}
		switch segment.kind {
		case property_static_segment:
			sb.WriteString("s:")
			sb.WriteString(segment.val)
		case property_dynamic_segment:
			sb.WriteString(":")
		case property_splat_segment:
			sb.WriteString("*")
		default:
			sb.WriteString("i")
		}
	}
	return sb.String()
}

func (p property_model_pattern) base_match(
	params Params,
	splat_values []string,
) property_model_match {
	score := 0
	ranks := make([]int, len(p.segments))
	for i, segment := range p.segments {
		rank := segment.rank()
		score += rank
		ranks[i] = rank
	}
	return property_model_match{
		normalized_pattern: p.normalized,
		params:             params,
		splat_values:       splat_values,
		score:              score,
		segment_ranks:      ranks,
		registration_order: p.registration_order,
		is_static:          p.is_static,
		last_is_dynamic:    p.last_segment_is_dynamic(),
		last_is_index:      p.last_segment_is_index(),
		last_is_splat:      p.last_segment_is_splat(),
		segment_len:        len(p.segments),
		dynamic_params:     int(p.num_dynamic_params()),
	}
}

func (p property_model_pattern) num_dynamic_params() uint8 {
	var count uint8
	for _, segment := range p.segments {
		if segment.kind == property_dynamic_segment {
			count++
		}
	}
	return count
}

func (s property_model_segment) rank() int {
	switch s.kind {
	case property_static_segment, property_index_segment:
		return score_static
	case property_dynamic_segment:
		return score_dynamic
	default:
		return 0
	}
}

func (m property_model_match) better_than(other property_model_match) bool {
	if m.is_static != other.is_static {
		return m.is_static
	}
	if m.score != other.score {
		return m.score > other.score
	}
	for i := range min(len(m.segment_ranks), len(other.segment_ranks)) {
		if m.segment_ranks[i] != other.segment_ranks[i] {
			return m.segment_ranks[i] > other.segment_ranks[i]
		}
	}
	if m.last_is_splat != other.last_is_splat {
		return !m.last_is_splat
	}
	if len(m.segment_ranks) != len(other.segment_ranks) {
		return len(m.segment_ranks) > len(other.segment_ranks)
	}
	return m.registration_order < other.registration_order
}

func (p property_model_path_string) has_trailing_slash() bool {
	return len(p) > 0 && p[len(p)-1] == '/'
}

func (p property_model_path_string) strip_trailing_slash() string {
	if !p.has_trailing_slash() {
		return string(p)
	}
	return string(p[:len(p)-1])
}

func (p property_model_path_string) segments() []string {
	path := string(p)
	if path == "" {
		return nil
	}
	if path == "/" {
		return []string{""}
	}
	path = strings.TrimPrefix(path, "/")
	if before, ok := strings.CutSuffix(path, "/"); ok {
		path = before
		return append(strings.Split(path, "/"), "")
	}
	return strings.Split(path, "/")
}

func (p property_model_path_string) flatten_nested_matches(
	matches map[string]property_model_match,
	prune bool,
) property_nested_observation {
	if prune {
		if _, ok := matches["/*"]; ok {
			_, has_empty := matches[""]
			switch {
			case has_empty && len(matches) > 2:
				delete(matches, "/*")
			case !has_empty && len(matches) > 1:
				delete(matches, "/*")
			}
		}

		if len(matches) >= 2 {
			p.prune_nested_matches(matches)
		}
	}

	results := make([]property_model_match, 0, len(matches))
	for _, match := range matches {
		results = append(results, match)
	}
	if len(results) == 0 {
		return property_nested_observation{}
	}
	if len(results) > 1 {
		slices.SortFunc(results, func(a, b property_model_match) int {
			if a.last_is_index != b.last_is_index {
				if a.last_is_index {
					return 1
				}
				return -1
			}
			if d := a.segment_len - b.segment_len; d != 0 {
				return d
			}
			return strings.Compare(a.normalized_pattern, b.normalized_pattern)
		})
	}

	real_path := string(p)
	real_segment_len := len(p.segments())
	if real_path != "" && real_path != "/" && len(results) == 1 &&
		results[0].normalized_pattern == "" {
		return property_nested_observation{}
	}

	last := results[len(results)-1]
	if !last.last_is_non_root_splat() && last.normalized_pattern != "/*" {
		if last.segment_len < real_segment_len {
			return property_nested_observation{}
		}
		if last.segment_len == real_segment_len && last.dynamic_params > 0 &&
			len(last.params) == 0 {
			return property_nested_observation{}
		}
	}

	observation := property_nested_observation{
		found:        true,
		params:       last.params,
		splat_values: last.splat_values,
		patterns:     make([]string, len(results)),
		match_params: make([]Params, len(results)),
		match_splats: make([][]string, len(results)),
	}
	for i, match := range results {
		observation.patterns[i] = match.normalized_pattern
		observation.match_params[i] = match.params
		if len(match.splat_values) > 0 {
			observation.match_splats[i] = match.splat_values
		}
	}
	return observation
}

func (p property_model_path_string) prune_nested_matches(
	matches map[string]property_model_match,
) {
	longest_len := 0
	has_longest_index := false
	has_longest_dynamic := false
	has_longest_splat := false
	for _, match := range matches {
		if match.segment_len > longest_len {
			longest_len = match.segment_len
			has_longest_index = false
			has_longest_dynamic = false
			has_longest_splat = false
		}
		if match.segment_len != longest_len {
			continue
		}
		switch {
		case match.last_is_index:
			has_longest_index = true
		case match.last_is_splat:
			has_longest_splat = true
		case match.last_is_dynamic:
			has_longest_dynamic = true
		}
	}

	for pattern, match := range matches {
		if match.segment_len < longest_len &&
			(match.last_is_non_root_splat() || match.last_is_index) {
			delete(matches, pattern)
		}
	}

	type_count := 0
	if has_longest_index {
		type_count++
	}
	if has_longest_dynamic {
		type_count++
	}
	if has_longest_splat {
		type_count++
	}
	if type_count <= 1 {
		return
	}

	real_segment_len := len(p.segments())
	for pattern, match := range matches {
		if match.segment_len != longest_len {
			continue
		}
		if match.last_is_index {
			delete(matches, pattern)
			continue
		}
		if real_segment_len == longest_len && has_longest_dynamic &&
			has_longest_splat && match.last_is_splat {
			delete(matches, pattern)
			continue
		}
		if real_segment_len > longest_len && has_longest_dynamic &&
			has_longest_splat && match.last_is_dynamic {
			delete(matches, pattern)
		}
	}
}

func (m property_model_match) last_is_non_root_splat() bool {
	return m.last_is_splat && m.segment_len > 1
}

func TestBestMatchModelPatternNormalizationIsUnique(t *testing.T) {
	seen := make(map[string]string, len(property_best_match_patterns))
	for _, pattern := range property_best_match_patterns {
		model := property_model_pattern_string(pattern).model(0)
		if existing, ok := seen[model.normalized]; ok {
			t.Fatalf(
				"patterns %q and %q both normalize to %q",
				existing,
				pattern,
				model.normalized,
			)
		}
		seen[model.normalized] = pattern
	}
}

func TestBestMatchModelAgreesWithExistingTableCases(t *testing.T) {
	for _, tc := range get_best_match_test_cases() {
		if !slices.Contains(property_best_match_patterns, tc.want_pattern) &&
			tc.want_pattern != not_found {
			continue
		}
		if !property_best_match_patterns.contains_all(tc.patterns) {
			continue
		}

		model_tc := property_route_case{
			catalog:         property_best_match_patterns,
			pattern_indexes: property_best_match_patterns.indexes(tc.patterns),
			path_segments:   property_model_path_string(tc.path).non_index_segments(),
			trailing_slash:  property_model_path_string(tc.path).has_non_root_trailing_slash(),
		}
		got, ok := model_tc.expectation()
		want_ok := tc.want_pattern != not_found
		if ok != want_ok {
			t.Fatalf("%s: model found = %v, want %v", tc.name, ok, want_ok)
		}
		if !want_ok {
			continue
		}
		if got.normalized_pattern != tc.want_pattern {
			t.Fatalf(
				"%s: model pattern = %q, want %q",
				tc.name,
				got.normalized_pattern,
				tc.want_pattern,
			)
		}
		if !reflect.DeepEqual(got.params, tc.want_params) {
			t.Fatalf("%s: model params = %v, want %v", tc.name, got.params, tc.want_params)
		}
		if !reflect.DeepEqual(got.splat_values, tc.want_splat_values) {
			t.Fatalf(
				"%s: model splat = %v, want %v",
				tc.name,
				got.splat_values,
				tc.want_splat_values,
			)
		}
	}
}

func (c property_pattern_catalog) contains_all(needles []string) bool {
	for _, needle := range needles {
		if !slices.Contains(c, needle) {
			return false
		}
	}
	return true
}

func (c property_pattern_catalog) indexes(patterns []string) []int {
	indexes := make([]int, 0, len(patterns))
	for _, pattern := range patterns {
		indexes = append(indexes, slices.Index(c, pattern))
	}
	return indexes
}

func (p property_model_path_string) non_index_segments() []string {
	segments := p.segments()
	if len(segments) == 1 && segments[0] == "" {
		return nil
	}
	if len(segments) > 0 && segments[len(segments)-1] == "" {
		return segments[:len(segments)-1]
	}
	return segments
}

func (p property_model_path_string) has_non_root_trailing_slash() bool {
	return p != "/" && p.has_trailing_slash()
}
