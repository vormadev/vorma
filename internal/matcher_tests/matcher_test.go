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
/////// SHARED TEST HELPERS
/////////////////////////////////////////////////////////////////////

type pattern_option_shape struct {
	dynamic_param_prefix              rune
	splat_segment_identifier          rune
	explicit_index_segment_identifier string
}

func option_shape_for_rewrites(opts *matcher.Options) pattern_option_shape {
	if opts == nil {
		return pattern_option_shape{}
	}
	return pattern_option_shape{
		dynamic_param_prefix:              opts.DynamicParamPrefix,
		splat_segment_identifier:          opts.SplatSegmentIdentifier,
		explicit_index_segment_identifier: opts.ExplicitIndexSegmentIdentifier,
	}
}

func rewrite_patterns(
	patterns []string,
	source_index string,
	shape pattern_option_shape,
) []string {
	out := make([]string, len(patterns))
	for i, p := range patterns {
		out[i] = rewrite_single_pattern(p, source_index, shape)
	}
	return out
}

func rewrite_single_pattern(
	pattern string,
	source_index string,
	shape pattern_option_shape,
) string {
	if pattern == "" {
		return pattern
	}
	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if source_index != "" && part == source_index {
			if shape.explicit_index_segment_identifier == "" {
				parts[i] = ""
			} else {
				parts[i] = shape.explicit_index_segment_identifier
			}
			continue
		}
		if source_index == "" &&
			shape.explicit_index_segment_identifier != "" &&
			part == "" &&
			i > 0 {
			parts[i] = shape.explicit_index_segment_identifier
			continue
		}
		if len(part) > 0 && part[0] == ':' &&
			shape.dynamic_param_prefix != 0 &&
			shape.dynamic_param_prefix != ':' {
			parts[i] = string(shape.dynamic_param_prefix) + part[1:]
			continue
		}
		if part == "*" &&
			shape.splat_segment_identifier != 0 &&
			shape.splat_segment_identifier != '*' {
			parts[i] = string(shape.splat_segment_identifier)
			continue
		}
	}
	return strings.Join(parts, "/")
}

/////////////////////////////////////////////////////////////////////
/////// BEST MATCH TESTS
/////////////////////////////////////////////////////////////////////

const not_found = "NOT FOUND"

type best_match_test_case struct {
	name              string
	patterns          []string
	path              string
	want_pattern      string
	want_params       matcher.Params
	want_splat_values []string
}

func get_best_match_test_cases() []best_match_test_case {
	return []best_match_test_case{
		// empty-str cases
		{
			name:         "home route -- should match empty-str",
			patterns:     []string{""},
			path:         "/",
			want_pattern: "",
		},
		{
			name:         "home route -- idx should beat empty-str",
			patterns:     []string{"", "/"},
			path:         "/",
			want_pattern: "/",
		},
		{
			name:         "home route -- empty-str should beat root-splat",
			patterns:     []string{"", "/*"},
			path:         "/",
			want_pattern: "",
		},
		{
			name:         "home route -- idx should beat root-splat",
			patterns:     []string{"/", "/*"},
			path:         "/",
			want_pattern: "/",
		},
		{
			name:         "home route -- idx should win (empty-str, idx, root-splat registered)",
			patterns:     []string{"", "/", "/*"},
			path:         "/",
			want_pattern: "/",
		},
		{
			name:              "home route -- should match root-splat if no idx or empty-str",
			patterns:          []string{"/*"},
			path:              "/",
			want_pattern:      "/*",
			want_splat_values: []string{""},
		},
		// trailing slash should not match dynamic route
		{
			name:         "trailing slash should not match following dynamic route",
			patterns:     []string{"/users/:user"},
			path:         "/users/",
			want_pattern: not_found,
		},
		// trailing slash behavior: static matches
		{
			name:         "exact match should win over following splat",
			patterns:     []string{"/", "/users", "/users/*", "/posts"},
			path:         "/users",
			want_pattern: "/users",
		},
		{
			name:         "exact match, with trailing slash, should win over catch-all",
			patterns:     []string{"/", "/users", "/users/*", "/posts"},
			path:         "/users/",
			want_pattern: "/users",
		},
		{
			name:         "with no trailing slash, should NOT match following catch-all",
			patterns:     []string{"/", "/users/*", "/posts"},
			path:         "/users",
			want_pattern: not_found,
		},
		{
			name:              "with trailing slash, should match following catch-all",
			patterns:          []string{"/", "/users/*", "/posts"},
			path:              "/users/",
			want_pattern:      "/users/*",
			want_splat_values: []string{""},
		},
		// same as above but with trailing slash as registered pattern
		{
			name: "registered trailing slash -- exact match without trailing wins",
			patterns: []string{
				"/",
				"/users/",
				"/users",
				"/users/*",
				"/posts",
			},
			path:         "/users",
			want_pattern: "/users",
		},
		{
			name: "registered trailing slash -- trailing slash matches trailing pattern",
			patterns: []string{
				"/",
				"/users/",
				"/users",
				"/users/*",
				"/posts",
			},
			path:         "/users/",
			want_pattern: "/users/",
		},
		{
			name:         "registered trailing slash -- no trailing should NOT match trailing pattern or catch-all",
			patterns:     []string{"/", "/users/", "/users/*", "/posts"},
			path:         "/users",
			want_pattern: not_found,
		},
		{
			name:         "registered trailing slash -- trailing should match trailing pattern, not catch-all",
			patterns:     []string{"/", "/users/", "/users/*", "/posts"},
			path:         "/users/",
			want_pattern: "/users/",
		},
		// trailing slash behavior: dynamic matches
		{
			name:         "dynamic match should win over catch-all",
			patterns:     []string{"/", "/:user", "/:user/*", "/posts"},
			path:         "/bob",
			want_pattern: "/:user",
			want_params:  matcher.Params{"user": "bob"},
		},
		{
			name:         "dynamic match, with trailing slash, should win over catch-all",
			patterns:     []string{"/", "/:user", "/:user/*", "/posts"},
			path:         "/bob/",
			want_pattern: "/:user",
			want_params:  matcher.Params{"user": "bob"},
		},
		{
			name:         "dynamic - no trailing slash should NOT match following catch-all",
			patterns:     []string{"/", "/:user/*", "/posts"},
			path:         "/bob",
			want_pattern: not_found,
		},
		{
			name:              "dynamic - trailing slash should match following catch-all",
			patterns:          []string{"/", "/:user/*", "/posts"},
			path:              "/bob/",
			want_pattern:      "/:user/*",
			want_params:       matcher.Params{"user": "bob"},
			want_splat_values: []string{""},
		},
		// same with registered trailing slash
		{
			name: "registered trailing slash -- dynamic without trailing wins over catch-all",
			patterns: []string{
				"/",
				"/:user/",
				"/:user",
				"/:user/*",
				"/posts",
			},
			path:         "/bob",
			want_pattern: "/:user",
			want_params:  matcher.Params{"user": "bob"},
		},
		{
			name: "registered trailing slash -- dynamic with trailing matches trailing pattern",
			patterns: []string{
				"/",
				"/:user/",
				"/:user",
				"/:user/*",
				"/posts",
			},
			path:         "/bob/",
			want_pattern: "/:user/",
			want_params:  matcher.Params{"user": "bob"},
		},
		{
			name:         "registered trailing slash -- dynamic no trailing should NOT match",
			patterns:     []string{"/", "/:user/", "/:user/*", "/posts"},
			path:         "/bob",
			want_pattern: not_found,
		},
		{
			name:         "registered trailing slash -- dynamic trailing matches trailing pattern",
			patterns:     []string{"/", "/:user/", "/:user/*", "/posts"},
			path:         "/bob/",
			want_pattern: "/:user/",
			want_params:  matcher.Params{"user": "bob"},
		},
		// more tests
		{
			name:         "parameter match",
			patterns:     []string{"/users", "/users/:id", "/users/profile"},
			path:         "/users/123",
			want_pattern: "/users/:id",
			want_params:  matcher.Params{"id": "123"},
		},
		{
			name:         "multiple matches",
			patterns:     []string{"/", "/api", "/api/:version", "/api/v1"},
			path:         "/api/v1",
			want_pattern: "/api/v1",
		},
		{
			name:              "splat match",
			patterns:          []string{"/files", "/files/*"},
			path:              "/files/documents/report.pdf",
			want_pattern:      "/files/*",
			want_splat_values: []string{"documents", "report.pdf"},
		},
		{
			name:         "no match",
			patterns:     []string{"/users", "/posts", "/settings"},
			path:         "/profile",
			want_pattern: not_found,
		},
		{
			name: "complex nested paths",
			patterns: []string{
				"/api/v1/users",
				"/api/:version/users",
				"/api/v1/users/:id",
				"/api/:version/users/:id",
				"/api/v1/users/:id/posts",
				"/api/:version/users/:id/posts",
			},
			path:         "/api/v2/users/123/posts",
			want_pattern: "/api/:version/users/:id/posts",
			want_params:  matcher.Params{"version": "v2", "id": "123"},
		},
		{
			name:         "no patterns",
			patterns:     []string{},
			path:         "/users",
			want_pattern: not_found,
		},
		{
			name:         "many params",
			patterns:     []string{"/api/:p1/:p2/:p3/:p4/:p5"},
			path:         "/api/a/b/c/d/e",
			want_pattern: "/api/:p1/:p2/:p3/:p4/:p5",
			want_params: matcher.Params{
				"p1": "a",
				"p2": "b",
				"p3": "c",
				"p4": "d",
				"p5": "e",
			},
		},
		{
			name:         "nested no match",
			patterns:     []string{"/users/:id", "/users/:id/profile"},
			path:         "users/123/settings",
			want_pattern: not_found,
		},
	}
}

var best_match_opts_to_test = []*matcher.Options{
	{},
	{ExplicitIndexSegmentIdentifier: "_index"},
	{DynamicParamPrefix: '$'},
	{SplatSegmentIdentifier: '#'},
	{DynamicParamPrefix: '<', SplatSegmentIdentifier: '>'},
	{
		ExplicitIndexSegmentIdentifier: "_______",
		DynamicParamPrefix:             '<',
		SplatSegmentIdentifier:         '>',
	},
}

func TestFindBestMatch(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		for _, opts := range best_match_opts_to_test {
			for _, tt := range get_best_match_test_cases() {
				t.Run(tt.name, func(t *testing.T) {
					m := new_matcher_test_matcher(t, backend, opts)
					for _, p := range rewrite_patterns(tt.patterns, "", option_shape_for_rewrites(opts)) {
						must_register_test_pattern(t, m, p)
					}

					match, _ := m.FindBestMatch(tt.path)
					want_match := tt.want_pattern != not_found

					if want_match && match == nil {
						t.Errorf(
							"FindBestMatch(%s) = nil, want %s",
							tt.path,
							tt.want_pattern,
						)
						return
					}
					if !want_match {
						if match != nil {
							t.Errorf(
								"FindBestMatch(%s) = %v, want nil",
								tt.path,
								match.NormalizedPattern(),
							)
						}
						return
					}
					if match.NormalizedPattern() != tt.want_pattern {
						t.Errorf(
							"FindBestMatch() pattern = %q, want %q",
							match.NormalizedPattern(),
							tt.want_pattern,
						)
					}
					if tt.want_params == nil && len(match.Params) > 0 {
						t.Errorf(
							"FindBestMatch() params = %v, want nil",
							match.Params,
						)
					} else if tt.want_params != nil && !reflect.DeepEqual(match.Params, tt.want_params) {
						t.Errorf("FindBestMatch() params = %v, want %v", match.Params, tt.want_params)
					}
					if !reflect.DeepEqual(match.SplatValues, tt.want_splat_values) {
						t.Errorf(
							"FindBestMatch() splat = %v, want %v",
							match.SplatValues,
							tt.want_splat_values,
						)
					}
				})
			}
		}
	})
}

func TestFindBestMatchAdditionalScenarios(t *testing.T) {
	test_cases := []struct {
		name     string
		opts     *matcher.Options
		patterns []string
		path     string
	}{
		{
			name:     "default index",
			opts:     &matcher.Options{Quiet: true},
			patterns: []string{"/", "/:slug", "/app"},
			path:     "/settings/account",
		},
		{
			name: "explicit index",
			opts: &matcher.Options{
				ExplicitIndexSegmentIdentifier: "_index",
				Quiet:                          true,
			},
			patterns: []string{"/", "/:slug", "/_index", "/app"},
			path:     "/settings/account",
		},
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		for _, tc := range test_cases {
			t.Run(tc.name, func(t *testing.T) {
				m := new_matcher_test_matcher(t, backend, tc.opts)
				for _, pattern := range tc.patterns {
					must_register_test_pattern(t, m, pattern)
				}

				match, ok := m.FindBestMatch(tc.path)
				if ok {
					t.Errorf(
						"Expected no matches for path %q, but got: %v",
						tc.path,
						match,
					)
				}
			})
		}
	})
}

func TestFindBestMatchSplatSpecificityIsRegistrationOrderIndependent(t *testing.T) {
	pattern_sets := [][]string{
		{"/*", "/:x/*"},
		{"/:x/*", "/*"},
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		for i, patterns := range pattern_sets {
			m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
			for _, pattern := range patterns {
				must_register_test_pattern(t, m, pattern)
			}

			match, ok := m.FindBestMatch("/a/")
			if !ok || match == nil {
				t.Fatalf("pattern set %d: expected match", i)
			}
			if got, want := match.NormalizedPattern(), "/:x/*"; got != want {
				t.Fatalf(
					"pattern set %d: NormalizedPattern() = %q, want %q",
					i,
					got,
					want,
				)
			}
			if got, want := match.Params, (matcher.Params{"x": "a"}); !reflect.DeepEqual(got, want) {
				t.Fatalf("pattern set %d: Params = %v, want %v", i, got, want)
			}
			if got, want := match.SplatValues, []string{""}; !reflect.DeepEqual(got, want) {
				t.Fatalf("pattern set %d: SplatValues = %v, want %v", i, got, want)
			}
		}
	})
}

func TestFindBestMatchTrailingDynamicBeatsTrailingSplat(t *testing.T) {
	pattern_sets := [][]string{
		{"/:x/*", "/:y"},
		{"/:y", "/:x/*"},
		{"/*", "/:x/*", "/:y"},
		{"/*", "/:y", "/:x/*"},
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		for i, patterns := range pattern_sets {
			m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
			for _, pattern := range patterns {
				must_register_test_pattern(t, m, pattern)
			}

			match, ok := m.FindBestMatch("/a/")
			if !ok || match == nil {
				t.Fatalf("pattern set %d: expected match", i)
			}
			if got, want := match.NormalizedPattern(), "/:y"; got != want {
				t.Fatalf(
					"pattern set %d: NormalizedPattern() = %q, want %q",
					i,
					got,
					want,
				)
			}
			if got, want := match.Params, (matcher.Params{"y": "a"}); !reflect.DeepEqual(got, want) {
				t.Fatalf("pattern set %d: Params = %v, want %v", i, got, want)
			}
			if len(match.SplatValues) != 0 {
				t.Fatalf("pattern set %d: SplatValues = %v, want empty", i, match.SplatValues)
			}
		}
	})
}

/////////////////////////////////////////////////////////////////////
/////// API SURFACE TESTS
/////////////////////////////////////////////////////////////////////

func TestNewWithNilOptions(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		config := backend.matcher_config(t, nil)
		if got := config.dynamic_param_prefix; got != ":" {
			t.Fatalf("DynamicParamPrefix() = %q, want ':'", got)
		}
		if got := config.splat_segment_identifier; got != "*" {
			t.Fatalf("SplatSegmentIdentifier() = %q, want '*'", got)
		}
		if got := config.explicit_index_segment_identifier; got != "" {
			t.Fatalf("ExplicitIndexSegmentIdentifier() = %q, want empty", got)
		}
	})
}

func TestAccessorsWithCustomOptions(t *testing.T) {
	opts := &matcher.Options{
		DynamicParamPrefix:     '$',
		SplatSegmentIdentifier: '#',
		Quiet:                  true,
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		config := backend.matcher_config(t, opts)
		if got := config.dynamic_param_prefix; got != "$" {
			t.Fatalf("DynamicParamPrefix() = %q, want '$'", got)
		}
		if got := config.splat_segment_identifier; got != "#" {
			t.Fatalf("SplatSegmentIdentifier() = %q, want '#'", got)
		}

		m := new_matcher_test_matcher(t, backend, opts)
		rp, err := m.NormalizePattern("/users/$id/#")
		if err != nil {
			t.Fatalf("NormalizePattern() error: %v", err)
		}
		if rp == nil {
			t.Fatal("NormalizePattern() returned nil")
		}
		if got, want := rp.NormalizedPattern(), "/users/:id/*"; got != want {
			t.Fatalf("NormalizedPattern() = %q, want %q", got, want)
		}
	})
}

func TestAccessorsWithExplicitIndex(t *testing.T) {
	opts := &matcher.Options{
		DynamicParamPrefix:             '@',
		SplatSegmentIdentifier:         '#',
		ExplicitIndexSegmentIdentifier: "_index",
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		config := backend.matcher_config(t, opts)
		if got := config.dynamic_param_prefix; got != "@" {
			t.Fatalf("DynamicParamPrefix() = %q, want '@'", got)
		}
		if got := config.splat_segment_identifier; got != "#" {
			t.Fatalf("SplatSegmentIdentifier() = %q, want '#'", got)
		}
		if got := config.explicit_index_segment_identifier; got != "_index" {
			t.Fatalf(
				"ExplicitIndexSegmentIdentifier() = %q, want %q",
				got,
				"_index",
			)
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		users := backend.path_helpers(t, "/users")
		if !users.HasLeadingSlash {
			t.Fatal("HasLeadingSlash(\"/users\") = false, want true")
		}
		if users.HasTrailingSlash {
			t.Fatal("HasTrailingSlash(\"/users\") = true, want false")
		}
		if got, want := users.EnsureTrailingSlash, "/users/"; got != want {
			t.Fatalf("EnsureTrailingSlash = %q, want %q", got, want)
		}
		if got, want := users.StripLeadingSlash, "users"; got != want {
			t.Fatalf("StripLeadingSlash = %q, want %q", got, want)
		}

		relative_users := backend.path_helpers(t, "users")
		if relative_users.HasLeadingSlash {
			t.Fatal("HasLeadingSlash(\"users\") = true, want false")
		}
		if got, want := relative_users.EnsureLeadingSlash, "/users"; got != want {
			t.Fatalf("EnsureLeadingSlash = %q, want %q", got, want)
		}
		if got, want := relative_users.EnsureLeadingAndTrailingSlash, "/users/"; got != want {
			t.Fatalf("EnsureLeadingAndTrailingSlash = %q, want %q", got, want)
		}

		trailing_users := backend.path_helpers(t, "/users/")
		if !trailing_users.HasTrailingSlash {
			t.Fatal("HasTrailingSlash(\"/users/\") = false, want true")
		}
		if got, want := trailing_users.StripTrailingSlash, "/users"; got != want {
			t.Fatalf("StripTrailingSlash = %q, want %q", got, want)
		}

		if got, want := backend.join_patterns(
			t,
			nil,
			"/users/:id",
			"posts",
		), "/users/:id/posts"; got != want {
			t.Fatalf("JoinPatterns(..., \"posts\") = %q, want %q", got, want)
		}
		if got, want := backend.join_patterns(
			t,
			nil,
			"/users/:id",
			"/posts",
		), "/users/:id/posts"; got != want {
			t.Fatalf("JoinPatterns(..., \"/posts\") = %q, want %q", got, want)
		}
	})
}

func TestRegisterPatternAndFindBestMatch(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		m := new_matcher_test_matcher(t, backend, nil)
		must_register_test_pattern(t, m, "/users/:id")

		match, found := m.FindBestMatch("/users/42")
		if !found || match == nil {
			t.Fatal("FindBestMatch(\"/users/42\") did not find a match")
		}
		if got, want := match.NormalizedPattern(), "/users/:id"; got != want {
			t.Fatalf("NormalizedPattern() = %q, want %q", got, want)
		}
		if got, want := match.Params["id"], "42"; got != want {
			t.Fatalf("Params[\"id\"] = %q, want %q", got, want)
		}

		if match, found := m.FindBestMatch("/orders/42"); found || match != nil {
			t.Fatalf(
				"FindBestMatch(\"/orders/42\") = (%v, %v), want (nil, false)",
				match,
				found,
			)
		}
	})
}

func TestFindNestedMatchesSmokeTest(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		m := new_matcher_test_matcher(t, backend, &matcher.Options{})
		must_register_test_pattern(t, m, "")
		must_register_test_pattern(t, m, "/users")
		must_register_test_pattern(t, m, "/users/:id")

		results, found := m.FindNestedMatches("/users/123")
		if !found {
			t.Fatal("expected nested match")
		}
		if results == nil {
			t.Fatal("results should not be nil")
		}
		if got, want := len(results.Matches), 3; got != want {
			t.Fatalf("len(Matches) = %d, want %d", got, want)
		}
		if got, want := results.Params["id"], "123"; got != want {
			t.Fatalf("Params[id] = %q, want %q", got, want)
		}
	})
}

func TestRegisterPatternReturnsErrorOnNormalizedCollision(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		m := new_matcher_test_matcher(
			t,
			backend,
			&matcher.Options{DynamicParamPrefix: '$'},
		)
		must_register_test_pattern(t, m, "/users/$id")

		_, err := m.RegisterPattern("/users/:id")
		if err == nil {
			t.Fatal("expected error for normalized collision")
		}
		expected := `normalized pattern collision: "/users/:id" and "/users/$id" both normalize to "/users/:id"`
		if got := err.Error(); got != expected {
			t.Fatalf("error = %q, want %q", got, expected)
		}
	})
}

func TestRegisterPatternTreatsDuplicateOriginalAsIdempotent(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
		first := must_register_test_pattern(t, m, "/users/:id")
		second := must_register_test_pattern(t, m, "/users/:id")

		if got, want := second.OriginalPattern(), first.OriginalPattern(); got != want {
			t.Fatalf("OriginalPattern() = %q, want %q", got, want)
		}
		if got, want := second.NormalizedPattern(), first.NormalizedPattern(); got != want {
			t.Fatalf("NormalizedPattern() = %q, want %q", got, want)
		}

		match, found := m.FindBestMatch("/users/42")
		if !found {
			t.Fatal("FindBestMatch(\"/users/42\") did not find duplicate-registered pattern")
		}
		if got, want := match.NormalizedPattern(), "/users/:id"; got != want {
			t.Fatalf("NormalizedPattern() = %q, want %q", got, want)
		}
	})
}

func TestRegisterPatternReturnsErrorOnSplatAliasCollision(t *testing.T) {
	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		m := new_matcher_test_matcher(
			t,
			backend,
			&matcher.Options{SplatSegmentIdentifier: '#', Quiet: true},
		)
		must_register_test_pattern(t, m, "/files/#")

		_, err := m.RegisterPattern("/files/*")
		if err == nil {
			t.Fatal("expected error for splat alias collision")
		}
		expected := `normalized pattern collision: "/files/*" and "/files/#" both normalize to "/files/*"`
		if got := err.Error(); got != expected {
			t.Fatalf("error = %q, want %q", got, expected)
		}
	})
}

func TestRegisterPatternReturnsErrorOnRouteShapeCollision(t *testing.T) {
	test_cases := []struct {
		name     string
		first    string
		second   string
		expected string
	}{
		{
			name:     "single dynamic segment",
			first:    "/users/:id",
			second:   "/users/:slug",
			expected: `route shape collision: "/users/:slug" and "/users/:id" both match the same paths`,
		},
		{
			name:     "dynamic plus splat",
			first:    "/:section/*",
			second:   "/:page/*",
			expected: `route shape collision: "/:page/*" and "/:section/*" both match the same paths`,
		},
		{
			name:     "repeated param names are still shape equivalent",
			first:    "/:x/:x",
			second:   "/:x/:y",
			expected: `route shape collision: "/:x/:y" and "/:x/:x" both match the same paths`,
		},
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		for _, tc := range test_cases {
			t.Run(tc.name, func(t *testing.T) {
				m := new_matcher_test_matcher(t, backend, &matcher.Options{Quiet: true})
				must_register_test_pattern(t, m, tc.first)

				_, err := m.RegisterPattern(tc.second)
				if err == nil {
					t.Fatal("expected error for route shape collision")
				}
				if got := err.Error(); got != tc.expected {
					t.Fatalf("error = %q, want %q", got, tc.expected)
				}
			})
		}
	})
}

/////////////////////////////////////////////////////////////////////
/////// BEST MATCH BENCHMARKS
/////////////////////////////////////////////////////////////////////

func setup_best_match_benchmark(tb testing.TB, scale string) *matcher.Matcher {
	tb.Helper()
	m := must_new_go_matcher(tb, &matcher.Options{Quiet: true})

	switch scale {
	case "small":
		must_register_go_pattern(tb, m, "/")
		must_register_go_pattern(tb, m, "/users")
		must_register_go_pattern(tb, m, "/users/:id")
		must_register_go_pattern(tb, m, "/users/:id/profile")
		must_register_go_pattern(tb, m, "/api/v1/users")
		must_register_go_pattern(tb, m, "/api/:version/users")
		must_register_go_pattern(tb, m, "/api/v1/users/:id")
		must_register_go_pattern(tb, m, "/files/*")
	case "medium":
		for i := range 1_000 {
			must_register_go_pattern(tb, m, fmt.Sprintf("/api/v%d/users", i%5))
			must_register_go_pattern(tb, m, fmt.Sprintf("/api/v%d/users/:id", i%5))
			must_register_go_pattern(
				tb,
				m,
				fmt.Sprintf("/api/v%d/users/:id/posts/:post_id", i%5),
			)
			must_register_go_pattern(tb, m, fmt.Sprintf("/files/bucket%d/*", i%10))
		}
	case "large":
		for i := range 10_000 {
			must_register_go_pattern(tb, m, fmt.Sprintf("/api/v%d/users", i%10))
			must_register_go_pattern(tb, m, fmt.Sprintf("/api/v%d/products", i%10))
			must_register_go_pattern(tb, m, fmt.Sprintf("/docs/section%d", i%100))
			must_register_go_pattern(
				tb,
				m,
				fmt.Sprintf("/api/v%d/users/:id/posts/:post_id", i%10),
			)
			must_register_go_pattern(
				tb,
				m,
				fmt.Sprintf("/api/v%d/products/:category/:id", i%10),
			)
			must_register_go_pattern(tb, m, fmt.Sprintf("/files/bucket%d/*", i%20))
		}
	}
	return m
}

func best_match_benchmark_paths(scale string) []string {
	switch scale {
	case "small":
		return []string{
			"/",
			"/users",
			"/users/123",
			"/users/123/profile",
			"/api/v1/users",
			"/api/v2/users",
			"/files/document.pdf",
		}
	case "medium", "large":
		paths := make([]string, 0, 1000)
		for i := range 400 {
			paths = append(paths, fmt.Sprintf("/api/v%d/users", i%5))
		}
		for i := range 400 {
			paths = append(
				paths,
				fmt.Sprintf("/api/v%d/users/%d/posts/%d", i%5, i, i%100),
			)
		}
		for i := range 200 {
			paths = append(
				paths,
				fmt.Sprintf("/files/bucket%d/path/to/file%d.txt", i%10, i),
			)
		}
		return paths
	}
	return nil
}

func BenchmarkFindBestMatchSimple(b *testing.B) {
	scenarios := []struct {
		name string
		path string
	}{
		{"StaticPattern", "/api/v1/users"},
		{"DynamicPattern", "/api/v1/users/123/posts/456"},
		{"SplatPattern", "/files/bucket1/deep/path/file.txt"},
	}
	for _, s := range scenarios {
		b.Run(s.name, func(b *testing.B) {
			m := setup_best_match_benchmark(b, "medium")
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				match, _ := m.FindBestMatch(s.path)
				runtime.KeepAlive(match)
			}
		})
	}
}

func BenchmarkFindBestMatchAtScale(b *testing.B) {
	for _, scale := range []string{"small", "medium", "large"} {
		b.Run("Scale_"+scale, func(b *testing.B) {
			m := setup_best_match_benchmark(b, scale)
			paths := best_match_benchmark_paths(scale)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				match, _ := m.FindBestMatch(paths[i%len(paths)])
				runtime.KeepAlive(match)
			}
		})
	}
	b.Run("WorstCase_DeepNested", func(b *testing.B) {
		m := setup_best_match_benchmark(b, "large")
		path := "/api/v9/users/999/posts/999"
		b.ReportAllocs()
		for b.Loop() {
			match, _ := m.FindBestMatch(path)
			runtime.KeepAlive(match)
		}
	})
}
