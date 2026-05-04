package main

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/matcher"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

var nested_patterns = []string{
	"/_index",
	"/articles/_index",
	"/articles/test/articles/_index",
	"/bear/_index",
	"/dashboard/_index",
	"/dashboard/customers/_index",
	"/dashboard/customers/:customer_id/_index",
	"/dashboard/customers/:customer_id/orders/_index",
	"/dynamic-index/:pagename/_index",
	"/lion/_index",
	"/tiger/_index",
	"/tiger/:tiger_id/_index",

	// When ExplicitIndexSegmentIdentifier is set, "/" normalizes to "".
	// When it is NOT set, "/" normalizes to "/" and "" is never registered.
	"/",

	"/*",
	"/bear",
	"/bear/:bear_id",
	"/bear/:bear_id/*",
	"/dashboard",
	"/dashboard/*",
	"/dashboard/customers",
	"/dashboard/customers/:customer_id",
	"/dashboard/customers/:customer_id/orders",
	"/dashboard/customers/:customer_id/orders/:order_id",
	"/dynamic-index/index",
	"/lion",
	"/lion/*",
	"/tiger",
	"/tiger/:tiger_id",
	"/tiger/:tiger_id/:tiger_cub_id",
	"/tiger/:tiger_id/*",

	// Unnamed dynamic params.
	"/a/b/:",
	"/c/d/e/:_",
	"/f/g/h/i/:/:",
	"/j/k/l/m/n/:_/:_",
}

type nested_test_scenario struct {
	path             string
	expected_matches []string
	splat_values     []string
	params           matcher.Params
}

var nested_scenarios = []nested_test_scenario{
	{
		path: "/does-not-exist", splat_values: []string{"does-not-exist"},
		expected_matches: []string{"", "/*"},
	},
	{
		path: "/this-should-be-ignored", splat_values: []string{"this-should-be-ignored"},
		expected_matches: []string{"", "/*"},
	},
	{path: "/", expected_matches: []string{"", "/"}},
	{path: "/lion", expected_matches: []string{"", "/lion", "/lion/"}},
	{
		path: "/lion/123", splat_values: []string{"123"},
		expected_matches: []string{"", "/lion", "/lion/*"},
	},
	{
		path: "/lion/123/456", splat_values: []string{"123", "456"},
		expected_matches: []string{"", "/lion", "/lion/*"},
	},
	{
		path: "/lion/123/456/789", splat_values: []string{"123", "456", "789"},
		expected_matches: []string{"", "/lion", "/lion/*"},
	},
	{path: "/tiger", expected_matches: []string{"", "/tiger", "/tiger/"}},
	{
		path: "/tiger/123", params: matcher.Params{"tiger_id": "123"},
		expected_matches: []string{
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/",
		},
	},
	{
		path: "/tiger/123/456", params: matcher.Params{"tiger_id": "123", "tiger_cub_id": "456"},
		expected_matches: []string{
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/:tiger_cub_id",
		},
	},
	{
		path: "/tiger/123/456/789", params: matcher.Params{"tiger_id": "123"}, splat_values: []string{"456", "789"},
		expected_matches: []string{
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/*",
		},
	},
	{path: "/bear", expected_matches: []string{"", "/bear", "/bear/"}},
	{
		path: "/bear/123", params: matcher.Params{"bear_id": "123"},
		expected_matches: []string{"", "/bear", "/bear/:bear_id"},
	},
	{
		path: "/bear/123/456", params: matcher.Params{"bear_id": "123"}, splat_values: []string{"456"},
		expected_matches: []string{
			"",
			"/bear",
			"/bear/:bear_id",
			"/bear/:bear_id/*",
		},
	},
	{
		path: "/bear/123/456/789", params: matcher.Params{"bear_id": "123"}, splat_values: []string{"456", "789"},
		expected_matches: []string{
			"",
			"/bear",
			"/bear/:bear_id",
			"/bear/:bear_id/*",
		},
	},
	{
		path:             "/dashboard",
		expected_matches: []string{"", "/dashboard", "/dashboard/"},
	},
	{
		path: "/dashboard/asdf", splat_values: []string{"asdf"},
		expected_matches: []string{"", "/dashboard", "/dashboard/*"},
	},
	{
		path: "/dashboard/customers",
		expected_matches: []string{
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/",
		},
	},
	{
		path: "/dashboard/customers/123", params: matcher.Params{"customer_id": "123"},
		expected_matches: []string{
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/",
		},
	},
	{
		path: "/dashboard/customers/123/orders", params: matcher.Params{"customer_id": "123"},
		expected_matches: []string{
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/orders",
			"/dashboard/customers/:customer_id/orders/",
		},
	},
	{
		path: "/dashboard/customers/123/orders/456", params: matcher.Params{"customer_id": "123", "order_id": "456"},
		expected_matches: []string{
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/orders",
			"/dashboard/customers/:customer_id/orders/:order_id",
		},
	},
	{path: "/articles", expected_matches: []string{"", "/articles/"}},
	{
		path: "/articles/bob", splat_values: []string{"articles", "bob"},
		expected_matches: []string{"", "/*"},
	},
	{
		path: "/articles/test", splat_values: []string{"articles", "test"},
		expected_matches: []string{"", "/*"},
	},
	{
		path:             "/articles/test/articles",
		expected_matches: []string{"", "/articles/test/articles/"},
	},
	{
		path:             "/dynamic-index/index",
		expected_matches: []string{"", "/dynamic-index/index"},
	},
	{
		path:             "/a/b/hi",
		params:           matcher.Params{"": "hi"},
		expected_matches: []string{"", "/a/b/:"},
	},
	{
		path:             "/c/d/e/hi",
		params:           matcher.Params{"_": "hi"},
		expected_matches: []string{"", "/c/d/e/:_"},
	},
	{
		path:             "/f/g/h/i/hi/hi2",
		params:           matcher.Params{"": "hi2"},
		expected_matches: []string{"", "/f/g/h/i/:/:"},
	},
	{
		path:             "/j/k/l/m/n/hi/hi2",
		params:           matcher.Params{"_": "hi2"},
		expected_matches: []string{"", "/j/k/l/m/n/:_/:_"},
	},
}

var nested_opts_to_test = []*matcher.Options{
	{},
	{ExplicitIndexSegmentIdentifier: "_index"},
	{DynamicParamPrefix: '$'},
	{SplatSegmentIdentifier: '#'},
	{
		ExplicitIndexSegmentIdentifier: "_______",
		DynamicParamPrefix:             '<',
		SplatSegmentIdentifier:         '>',
	},
	{
		ExplicitIndexSegmentIdentifier: "",
		DynamicParamPrefix:             '<',
		SplatSegmentIdentifier:         '>',
	},
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func equal_params(a, b matcher.Params) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func equal_splat(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// opts_uses_explicit_index reports whether the option config produces a
// registered "" (empty-string) pattern from "/". This only happens when
// ExplicitIndexSegmentIdentifier is set to a non-empty value.
func opts_uses_explicit_index(opts *matcher.Options) bool {
	return opts != nil && opts.ExplicitIndexSegmentIdentifier != ""
}

// adjust_expected_matches strips "" from expected matches when the option
// config does not use explicit index, because "" is only a valid registered
// pattern in explicit-index configurations.
func adjust_expected_matches(
	expected []string,
	uses_explicit_index bool,
) []string {
	if uses_explicit_index {
		return expected
	}
	filtered := make([]string, 0, len(expected))
	for _, e := range expected {
		if e != "" {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

/////////////////////////////////////////////////////////////////////
/////// CORE NESTED MATCH TESTS
/////////////////////////////////////////////////////////////////////

func TestFindNestedMatches(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		for _, opts := range nested_opts_to_test {
			uses_explicit := opts_uses_explicit_index(opts)

			m := new_matcher_test_matcher(t, backend, opts)
			for _, p := range rewrite_patterns(nested_patterns, "_index", option_shape_for_rewrites(opts)) {
				must_register_test_pattern(t, m, p)
			}

			for _, tc := range nested_scenarios {
				t.Run(tc.path, func(t *testing.T) {
					results, ok := m.FindNestedMatches(tc.path)

					expected := adjust_expected_matches(
						tc.expected_matches,
						uses_explicit,
					)

					if !ok {
						if len(expected) == 0 {
							return
						}
						t.Fatalf("Expected matches but got none.")
					}

					if !equal_params(tc.params, results.Params) {
						t.Errorf(
							"Expected params %v, got %v",
							tc.params,
							results.Params,
						)
					}
					if !equal_splat(tc.splat_values, results.SplatValues) {
						t.Errorf(
							"Expected splat values %v, got %v",
							tc.splat_values,
							results.SplatValues,
						)
					}

					actual := results.Matches
					expected_count := len(expected)
					actual_count := len(actual)

					fail := expected_count != actual_count
					for i := range max(expected_count, actual_count) {
						if i < expected_count && i < actual_count {
							if expected[i] != actual[i].NormalizedPattern() {
								fail = true
								break
							}
						} else {
							fail = true
							break
						}
					}

					if fail {
						var errs []string
						errs = append(
							errs,
							fmt.Sprintf(
								"\n===== Path: %q (explicit_index=%v) =====",
								tc.path,
								uses_explicit,
							),
						)
						if expected_count != actual_count {
							errs = append(
								errs,
								fmt.Sprintf(
									"Expected %d matches, got %d",
									expected_count,
									actual_count,
								),
							)
						}
						errs = append(errs, "Expected Matches:")
						for i, e := range expected {
							errs = append(errs, fmt.Sprintf("  [%d] %q", i, e))
						}
						errs = append(errs, "Actual Matches:")
						for i, a := range actual {
							errs = append(
								errs,
								fmt.Sprintf("  [%d] %q", i, a.NormalizedPattern()),
							)
						}
						t.Error(strings.Join(errs, "\n"))
					}
				})
			}
		}
	})
}

func TestFindNestedMatchesAdditionalScenarios(t *testing.T) {
	test_cases := []struct {
		name             string
		patterns         []string
		path             string
		expect_match     bool
		expected_matches []string
	}{
		{
			name:         "Invalid match with unhandled segment",
			patterns:     []string{"/", "/:slug", "/_index", "/app"},
			path:         "/settings/account",
			expect_match: false,
		},
		{
			name:         "Deeper Invalid 'Almost' Match",
			patterns:     []string{"/dashboard/customers"},
			path:         "/dashboard/customers/reports",
			expect_match: false,
		},
		{
			name:             "Splat as the Only Full Match",
			patterns:         []string{"/files/*", "/files/images"},
			path:             "/files/documents/report.pdf",
			expect_match:     true,
			expected_matches: []string{"/files/*"},
		},
		{
			name:         "Index Segment Edge Case with Extra Segment",
			patterns:     []string{"/articles/_index"},
			path:         "/articles/some-topic",
			expect_match: false,
		},
		{
			name:         "No Root Fallback for Multi-Segment Path",
			patterns:     []string{"/"},
			path:         "/some/random/path",
			expect_match: false,
		},
		{
			name:             "A",
			patterns:         []string{"/"},
			path:             "/",
			expect_match:     true,
			expected_matches: []string{"/"},
		},
		{
			name:             "B",
			patterns:         []string{"/*"},
			path:             "/",
			expect_match:     true,
			expected_matches: []string{"/*"},
		},
		{
			name:             "C",
			patterns:         []string{"/_index"},
			path:             "/",
			expect_match:     true,
			expected_matches: []string{"/_index"},
		},
		{
			name:             "AB",
			patterns:         []string{"/", "/*"},
			path:             "/",
			expect_match:     true,
			expected_matches: []string{"/", "/*"},
		},
		{
			name:             "AC",
			patterns:         []string{"/", "/_index"},
			path:             "/",
			expect_match:     true,
			expected_matches: []string{"/", "/_index"},
		},
		{
			name:             "BC",
			patterns:         []string{"/*", "/_index"},
			path:             "/",
			expect_match:     true,
			expected_matches: []string{"/_index"},
		},
		{
			name:             "ABC",
			patterns:         []string{"/", "/*", "/_index"},
			path:             "/",
			expect_match:     true,
			expected_matches: []string{"/", "/_index"},
		},
		{
			name:         "A-docs",
			patterns:     []string{"/"},
			path:         "/docs",
			expect_match: false,
		},
		{
			name:             "B-docs",
			patterns:         []string{"/*"},
			path:             "/docs",
			expect_match:     true,
			expected_matches: []string{"/*"},
		},
		{
			name:         "C-docs",
			patterns:     []string{"/_index"},
			path:         "/docs",
			expect_match: false,
		},
		{
			name:             "AB-docs",
			patterns:         []string{"/", "/*"},
			path:             "/docs",
			expect_match:     true,
			expected_matches: []string{"/", "/*"},
		},
		{
			name:         "AC-docs",
			patterns:     []string{"/", "/_index"},
			path:         "/docs",
			expect_match: false,
		},
		{
			name:             "BC-docs",
			patterns:         []string{"/*", "/_index"},
			path:             "/docs",
			expect_match:     true,
			expected_matches: []string{"/*"},
		},
		{
			name:             "ABC-docs",
			patterns:         []string{"/", "/*", "/_index"},
			path:             "/docs",
			expect_match:     true,
			expected_matches: []string{"/", "/*"},
		},
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		for _, tc := range test_cases {
			t.Run(tc.name, func(t *testing.T) {
				m := new_matcher_test_matcher(
					t,
					backend,
					&matcher.Options{ExplicitIndexSegmentIdentifier: "_index", Quiet: true},
				)
				for _, p := range tc.patterns {
					must_register_test_pattern(t, m, p)
				}

				results, ok := m.FindNestedMatches(tc.path)

				if ok != tc.expect_match {
					t.Errorf(
						"Expected match=%v for path %q, got %v",
						tc.expect_match,
						tc.path,
						ok,
					)
				}
				if !tc.expect_match {
					if results != nil && len(results.Matches) != 0 {
						t.Errorf(
							"Expected no matches for path %q, got %d",
							tc.path,
							len(results.Matches),
						)
					}
					return
				}
				if results == nil || len(results.Matches) == 0 {
					t.Errorf("Expected matches for path %q, got none", tc.path)
					return
				}
				if tc.expected_matches == nil {
					return
				}
				actual := make([]string, len(results.Matches))
				for i, m := range results.Matches {
					actual[i] = m.OriginalPattern()
				}
				if len(actual) != len(tc.expected_matches) {
					t.Errorf(
						"Path %q: expected %d matches %v, got %d matches %v",
						tc.path,
						len(tc.expected_matches),
						tc.expected_matches,
						len(actual),
						actual,
					)
					return
				}
				for i, expected := range tc.expected_matches {
					if actual[i] != expected {
						t.Errorf(
							"Path %q: at [%d] expected %q, got %q",
							tc.path,
							i,
							expected,
							actual[i],
						)
					}
				}
			})
		}
	})
}

/////////////////////////////////////////////////////////////////////
/////// TRAILING SLASH BEHAVIOR
/////////////////////////////////////////////////////////////////////

func TestTrailingSlashBehavior(t *testing.T) {
	patterns := []string{
		"/",
		"/_index",
		"/about",
		"/about/location",
		"/about/hobbies",
		"/about/:id",
		"/about/*",
	}

	type trailing_slash_case struct {
		name               string
		path               string
		expected_matches   []string
		unexpected_matches []string
	}

	cases := []trailing_slash_case{
		{
			name: "about with trailing slash", path: "/about/",
			expected_matches:   []string{"/", "/about"},
			unexpected_matches: []string{"/about/:id", "/about/*"},
		},
		{
			name: "about without trailing slash", path: "/about",
			expected_matches:   []string{"/", "/about"},
			unexpected_matches: []string{"/about/:id", "/about/*"},
		},
		{
			name: "about with actual id", path: "/about/123",
			expected_matches:   []string{"/", "/about/:id"},
			unexpected_matches: []string{"/about/*"},
		},
		{
			name: "about location exact match", path: "/about/location",
			expected_matches:   []string{"/", "/about", "/about/location"},
			unexpected_matches: []string{"/_index", "/about/:id", "/about/*"},
		},
		{
			name: "about with multiple segments", path: "/about/something/else",
			expected_matches:   []string{"/", "/about/*"},
			unexpected_matches: []string{"/about/:id", "/about/location"},
		},
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		m := new_matcher_test_matcher(
			t,
			backend,
			&matcher.Options{ExplicitIndexSegmentIdentifier: "_index", Quiet: true},
		)
		for _, p := range patterns {
			must_register_test_pattern(t, m, p)
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				results, ok := m.FindNestedMatches(tc.path)
				if !ok {
					t.Fatalf("Path %q: expected matches, got none", tc.path)
				}

				for _, expected := range tc.expected_matches {
					found := false
					for _, actual := range results.Matches {
						if actual.OriginalPattern() == expected {
							found = true
							break
						}
					}
					if !found && expected != "" {
						t.Errorf("Path %q: expected %q to match", tc.path, expected)
					}
				}
				for _, unexpected := range tc.unexpected_matches {
					for _, actual := range results.Matches {
						if actual.OriginalPattern() == unexpected {
							t.Errorf(
								"Path %q: %q should NOT match",
								tc.path,
								unexpected,
							)
						}
					}
				}
				if t.Failed() {
					t.Logf("Actual matches for %q:", tc.path)
					for i, m := range results.Matches {
						t.Logf("  [%d] %q", i, m.OriginalPattern())
					}
				}
			})
		}
	})
}

/////////////////////////////////////////////////////////////////////
/////// PARTIAL MATCHING WITH GAPS
/////////////////////////////////////////////////////////////////////

func TestPartialMatchingWithGaps(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		t.Run(
			"match parent and deeply nested route without intermediates",
			func(t *testing.T) {
				m := new_matcher_test_matcher(
					t,
					backend,
					&matcher.Options{ExplicitIndexSegmentIdentifier: "_index"},
				)
				must_register_test_pattern(t, m, "/bob")
				must_register_test_pattern(t, m, "/bob/larry/susan/jeff")

				results, ok := m.FindNestedMatches("/bob/larry/susan/jeff")
				if !ok {
					t.Fatal("Expected matches for /bob/larry/susan/jeff")
				}
				if len(results.Matches) != 2 {
					t.Errorf("Expected 2 matches, got %d", len(results.Matches))
					for i, m := range results.Matches {
						t.Logf("  [%d] %q", i, m.OriginalPattern())
					}
				}

				found_bob, found_jeff := false, false
				for _, m := range results.Matches {
					switch m.OriginalPattern() {
					case "/bob":
						found_bob = true
					case "/bob/larry/susan/jeff":
						found_jeff = true
					}
				}
				if !found_bob {
					t.Error("Expected /bob to match")
				}
				if !found_jeff {
					t.Error("Expected /bob/larry/susan/jeff to match")
				}
			},
		)

		t.Run(
			"should not match unregistered intermediate paths",
			func(t *testing.T) {
				m := new_matcher_test_matcher(
					t,
					backend,
					&matcher.Options{ExplicitIndexSegmentIdentifier: "_index"},
				)
				must_register_test_pattern(t, m, "/bob")
				must_register_test_pattern(t, m, "/bob/larry/susan/jeff")

				results, ok := m.FindNestedMatches("/bob/larry")
				if ok {
					t.Error("Should not find matches for /bob/larry")
					for i, m := range results.Matches {
						t.Logf("  [%d] %q", i, m.OriginalPattern())
					}
				}
			},
		)
	})
}

/////////////////////////////////////////////////////////////////////
/////// DETERMINISM
/////////////////////////////////////////////////////////////////////

func TestMatchOrderingDeterminism(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		t.Run("static vs dynamic same depth", func(t *testing.T) {
			var first_params matcher.Params
			var first_order []string

			for i := range 1000 {
				m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
				must_register_test_pattern(t, m, "/api/v1")
				must_register_test_pattern(t, m, "/api/:version")

				results, ok := m.FindNestedMatches("/api/v1")
				if !ok {
					t.Fatal("Expected matches")
				}

				order := make([]string, len(results.Matches))
				for j, m := range results.Matches {
					order[j] = m.NormalizedPattern()
				}

				if i == 0 {
					first_params = results.Params
					first_order = order
					continue
				}
				if !reflect.DeepEqual(results.Params, first_params) {
					t.Fatalf(
						"Iteration %d: params inconsistent. First: %v, Now: %v",
						i,
						first_params,
						results.Params,
					)
				}
				if !reflect.DeepEqual(order, first_order) {
					t.Fatalf(
						"Iteration %d: order inconsistent. First: %v, Now: %v",
						i,
						first_order,
						order,
					)
				}
			}
		})

		t.Run("dynamic and splat same depth", func(t *testing.T) {
			var first_params matcher.Params
			var first_splat []string

			for i := range 1000 {
				m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
				must_register_test_pattern(t, m, "/users/:id")
				must_register_test_pattern(t, m, "/users/*")

				results, ok := m.FindNestedMatches("/users/123")
				if !ok {
					t.Fatal("Expected matches")
				}

				if i == 0 {
					first_params = results.Params
					first_splat = results.SplatValues
					if len(first_params) == 0 {
						t.Fatal("Expected params from dynamic match")
					}
					continue
				}
				if !reflect.DeepEqual(results.Params, first_params) {
					t.Fatalf(
						"Iteration %d: params inconsistent. First: %v, Now: %v",
						i,
						first_params,
						results.Params,
					)
				}
				if !reflect.DeepEqual(results.SplatValues, first_splat) {
					t.Fatalf(
						"Iteration %d: splat inconsistent. First: %v, Now: %v",
						i,
						first_splat,
						results.SplatValues,
					)
				}
			}
		})

		t.Run("three patterns same depth", func(t *testing.T) {
			var first_params matcher.Params
			var first_order []string

			for i := range 1000 {
				m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
				must_register_test_pattern(t, m, "/a/b")
				must_register_test_pattern(t, m, "/a/:p")
				must_register_test_pattern(t, m, "/:x/b")

				results, ok := m.FindNestedMatches("/a/b")
				if !ok {
					t.Fatal("Expected matches")
				}

				order := make([]string, len(results.Matches))
				for j, m := range results.Matches {
					order[j] = m.NormalizedPattern()
				}

				if i == 0 {
					first_params = results.Params
					first_order = order
					continue
				}
				if !reflect.DeepEqual(results.Params, first_params) {
					t.Fatalf(
						"Iteration %d: params inconsistent. First: %v, Now: %v",
						i,
						first_params,
						results.Params,
					)
				}
				if !reflect.DeepEqual(order, first_order) {
					t.Fatalf(
						"Iteration %d: order inconsistent. First: %v, Now: %v",
						i,
						first_order,
						order,
					)
				}
			}
		})

		t.Run("deep splat prunes all competing longest dynamics", func(t *testing.T) {
			pattern_sets := [][]string{
				{"", "/a/:x", "/:x/:y", "/a/*"},
				{"", "/a/*", "/:x/:y", "/a/:x"},
				{"/:x/:y", "", "/a/:x", "/a/*"},
				{"/a/:x", "/a/*", "", "/:x/:y"},
			}

			var first_patterns []string
			var first_params matcher.Params
			var first_splat []string

			for i, patterns := range pattern_sets {
				m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
				for _, pattern := range patterns {
					must_register_test_pattern(t, m, pattern)
				}

				results, ok := m.FindNestedMatches("/a/a/a")
				if !ok {
					t.Fatalf("pattern set %d: expected matches", i)
				}

				actual_patterns := make([]string, len(results.Matches))
				for j, match := range results.Matches {
					actual_patterns[j] = match.NormalizedPattern()
				}

				if i == 0 {
					first_patterns = actual_patterns
					first_params = results.Params
					first_splat = results.SplatValues
					continue
				}
				if !reflect.DeepEqual(actual_patterns, first_patterns) {
					t.Fatalf(
						"pattern set %d: patterns = %v, want %v",
						i,
						actual_patterns,
						first_patterns,
					)
				}
				if !reflect.DeepEqual(results.Params, first_params) {
					t.Fatalf(
						"pattern set %d: params = %v, want %v",
						i,
						results.Params,
						first_params,
					)
				}
				if !reflect.DeepEqual(results.SplatValues, first_splat) {
					t.Fatalf(
						"pattern set %d: splat = %v, want %v",
						i,
						results.SplatValues,
						first_splat,
					)
				}
			}

			expected_patterns := []string{"", "/a/*"}
			if !reflect.DeepEqual(first_patterns, expected_patterns) {
				t.Fatalf("patterns = %v, want %v", first_patterns, expected_patterns)
			}
			if len(first_params) != 0 {
				t.Fatalf("params = %v, want empty", first_params)
			}
			expected_splat := []string{"a", "a"}
			if !reflect.DeepEqual(first_splat, expected_splat) {
				t.Fatalf("splat = %v, want %v", first_splat, expected_splat)
			}
		})

		t.Run("exact dynamic prunes all competing longest splats", func(t *testing.T) {
			pattern_sets := [][]string{
				{"/b/*", "/:x/:y", "/:x/*"},
				{"/:x/*", "/:x/:y", "/b/*"},
				{"/:x/:y", "/b/*", "/:x/*"},
				{"/:x/*", "/b/*", "/:x/:y"},
			}

			var first_patterns []string
			var first_params matcher.Params
			var first_splat []string

			for i, patterns := range pattern_sets {
				m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
				for _, pattern := range patterns {
					must_register_test_pattern(t, m, pattern)
				}

				results, ok := m.FindNestedMatches("/b/a")
				if !ok {
					t.Fatalf("pattern set %d: expected matches", i)
				}

				actual_patterns := make([]string, len(results.Matches))
				for j, match := range results.Matches {
					actual_patterns[j] = match.NormalizedPattern()
				}

				if i == 0 {
					first_patterns = actual_patterns
					first_params = results.Params
					first_splat = results.SplatValues
					continue
				}
				if !reflect.DeepEqual(actual_patterns, first_patterns) {
					t.Fatalf(
						"pattern set %d: patterns = %v, want %v",
						i,
						actual_patterns,
						first_patterns,
					)
				}
				if !reflect.DeepEqual(results.Params, first_params) {
					t.Fatalf(
						"pattern set %d: params = %v, want %v",
						i,
						results.Params,
						first_params,
					)
				}
				if !reflect.DeepEqual(results.SplatValues, first_splat) {
					t.Fatalf(
						"pattern set %d: splat = %v, want %v",
						i,
						results.SplatValues,
						first_splat,
					)
				}
			}

			expected_patterns := []string{"/:x/:y"}
			if !reflect.DeepEqual(first_patterns, expected_patterns) {
				t.Fatalf("patterns = %v, want %v", first_patterns, expected_patterns)
			}
			expected_params := matcher.Params{"x": "b", "y": "a"}
			if !reflect.DeepEqual(first_params, expected_params) {
				t.Fatalf("params = %v, want %v", first_params, expected_params)
			}
			if len(first_splat) != 0 {
				t.Fatalf("splat = %v, want empty", first_splat)
			}
		})
	})
}

/////////////////////////////////////////////////////////////////////
/////// BENCHMARKS
/////////////////////////////////////////////////////////////////////

func setup_nested_benchmark(tb testing.TB) *matcher.Matcher {
	tb.Helper()
	m := must_new_go_matcher(tb, &matcher.Options{Quiet: true})
	for _, p := range nested_patterns {
		must_register_go_pattern(tb, m, p)
	}
	return m
}

func nested_benchmark_paths() []string {
	return []string{
		"/",
		"/dashboard",
		"/dashboard/customers",
		"/dashboard/customers/123",
		"/dashboard/customers/123/orders",
		"/dashboard/customers/123/orders/456",
		"/tiger",
		"/tiger/123",
		"/tiger/123/456",
		"/tiger/123/456/789",
		"/bear/123/456/789",
		"/articles/test/articles",
		"/does-not-exist",
		"/dashboard/unknown/path",
	}
}

func BenchmarkFindNestedMatches(b *testing.B) {
	cases := []struct {
		name  string
		paths []string
	}{
		{
			name: "StaticPatterns",
			paths: []string{
				"/",
				"/dashboard",
				"/dashboard/customers",
				"/tiger",
				"/lion",
			},
		},
		{
			name: "DynamicPatterns",
			paths: []string{
				"/dashboard/customers/123",
				"/dashboard/customers/456/orders",
				"/tiger/123",
				"/bear/123",
			},
		},
		{
			name: "DeepNestedPatterns",
			paths: []string{
				"/dashboard/customers/123/orders/456",
				"/tiger/123/456/789",
				"/bear/123/456/789",
				"/articles/test/articles",
			},
		},
		{
			name: "SplatPatterns",
			paths: []string{
				"/does-not-exist",
				"/dashboard/unknown/path",
				"/tiger/123/456/789/extra",
				"/bear/123/456/789/extra",
			},
		},
		{name: "MixedPatterns", paths: nested_benchmark_paths()},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			m := setup_nested_benchmark(b)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				matches, _ := m.FindNestedMatches(tc.paths[i%len(tc.paths)])
				runtime.KeepAlive(matches)
			}
		})
	}
}
