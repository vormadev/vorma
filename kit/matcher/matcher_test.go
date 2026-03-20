package matcher

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

/////////////////////////////////////////////////////////////////////
/////// SHARED TEST HELPERS
/////////////////////////////////////////////////////////////////////

type pattern_option_shape struct {
	dynamic_param_prefix              rune
	splat_segment_identifier          rune
	explicit_index_segment_identifier string
}

func option_shape_for_rewrites(opts *Options) pattern_option_shape {
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
	want_params       Params
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
			want_params:  Params{"user": "bob"},
		},
		{
			name:         "dynamic match, with trailing slash, should win over catch-all",
			patterns:     []string{"/", "/:user", "/:user/*", "/posts"},
			path:         "/bob/",
			want_pattern: "/:user",
			want_params:  Params{"user": "bob"},
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
			want_params:       Params{"user": "bob"},
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
			want_params:  Params{"user": "bob"},
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
			want_params:  Params{"user": "bob"},
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
			want_params:  Params{"user": "bob"},
		},
		// more tests
		{
			name:         "parameter match",
			patterns:     []string{"/users", "/users/:id", "/users/profile"},
			path:         "/users/123",
			want_pattern: "/users/:id",
			want_params:  Params{"id": "123"},
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
			want_params:  Params{"version": "v2", "id": "123"},
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
			want_params: Params{
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

var best_match_opts_to_test = []*Options{
	{},
	{DynamicParamPrefix: '$'},
	{SplatSegmentIdentifier: '#'},
	{DynamicParamPrefix: '<', SplatSegmentIdentifier: '>'},
}

func TestFindBestMatch(t *testing.T) {
	for _, opts := range best_match_opts_to_test {
		for _, tt := range get_best_match_test_cases() {
			t.Run(tt.name, func(t *testing.T) {
				m := New(opts)
				for _, p := range rewrite_patterns(tt.patterns, "", option_shape_for_rewrites(opts)) {
					m.RegisterPattern(p)
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
}

func TestFindBestMatchAdditionalScenarios(t *testing.T) {
	m := New(&Options{Quiet: true})
	m.RegisterPattern("/")
	m.RegisterPattern("/:slug")
	m.RegisterPattern("/app")

	path := "/settings/account"
	match, ok := m.FindBestMatch(path)
	if ok {
		t.Errorf("Expected no matches for path %q, but got: %v", path, match)
	}
}

/////////////////////////////////////////////////////////////////////
/////// API SURFACE TESTS
/////////////////////////////////////////////////////////////////////

func TestNewWithNilOptions(t *testing.T) {
	m := New(nil)
	if got := m.DynamicParamPrefix(); got != ':' {
		t.Fatalf("DynamicParamPrefix() = %q, want ':'", got)
	}
	if got := m.SplatSegmentIdentifier(); got != '*' {
		t.Fatalf("SplatSegmentIdentifier() = %q, want '*'", got)
	}
	if got := m.ExplicitIndexSegmentIdentifier(); got != "" {
		t.Fatalf("ExplicitIndexSegmentIdentifier() = %q, want empty", got)
	}
}

func TestAccessorsWithCustomOptions(t *testing.T) {
	m := New(&Options{
		DynamicParamPrefix:     '$',
		SplatSegmentIdentifier: '#',
		Quiet:                  true,
	})
	if got := m.DynamicParamPrefix(); got != '$' {
		t.Fatalf("DynamicParamPrefix() = %q, want '$'", got)
	}
	if got := m.SplatSegmentIdentifier(); got != '#' {
		t.Fatalf("SplatSegmentIdentifier() = %q, want '#'", got)
	}

	rp := m.NormalizePattern("/users/$id/#")
	if rp == nil {
		t.Fatal("NormalizePattern() returned nil")
	}
	if got, want := rp.NormalizedPattern(), "/users/:id/*"; got != want {
		t.Fatalf("NormalizedPattern() = %q, want %q", got, want)
	}
}

func TestAccessorsWithExplicitIndex(t *testing.T) {
	m := New(&Options{
		DynamicParamPrefix:             '@',
		SplatSegmentIdentifier:         '#',
		ExplicitIndexSegmentIdentifier: "_index",
	})
	if got := m.DynamicParamPrefix(); got != '@' {
		t.Fatalf("DynamicParamPrefix() = %q, want '@'", got)
	}
	if got := m.SplatSegmentIdentifier(); got != '#' {
		t.Fatalf("SplatSegmentIdentifier() = %q, want '#'", got)
	}
	if got := m.ExplicitIndexSegmentIdentifier(); got != "_index" {
		t.Fatalf(
			"ExplicitIndexSegmentIdentifier() = %q, want %q",
			got,
			"_index",
		)
	}
}

func TestHelperFunctions(t *testing.T) {
	if !HasLeadingSlash("/users") {
		t.Fatal("HasLeadingSlash(\"/users\") = false, want true")
	}
	if HasLeadingSlash("users") {
		t.Fatal("HasLeadingSlash(\"users\") = true, want false")
	}
	if !HasTrailingSlash("/users/") {
		t.Fatal("HasTrailingSlash(\"/users/\") = false, want true")
	}
	if HasTrailingSlash("/users") {
		t.Fatal("HasTrailingSlash(\"/users\") = true, want false")
	}
	if got, want := EnsureLeadingSlash("users"), "/users"; got != want {
		t.Fatalf("EnsureLeadingSlash = %q, want %q", got, want)
	}
	if got, want := EnsureTrailingSlash("/users"), "/users/"; got != want {
		t.Fatalf("EnsureTrailingSlash = %q, want %q", got, want)
	}
	if got, want := EnsureLeadingAndTrailingSlash("users"), "/users/"; got != want {
		t.Fatalf("EnsureLeadingAndTrailingSlash = %q, want %q", got, want)
	}
	if got, want := StripLeadingSlash("/users"), "users"; got != want {
		t.Fatalf("StripLeadingSlash = %q, want %q", got, want)
	}
	if got, want := StripTrailingSlash("/users/"), "/users"; got != want {
		t.Fatalf("StripTrailingSlash = %q, want %q", got, want)
	}

	m := New(nil)
	rp := m.NormalizePattern("/users/:id")
	if rp == nil {
		t.Fatal("NormalizePattern returned nil")
	}
	if got, want := JoinPatterns(rp, "posts"), "/users/:id/posts"; got != want {
		t.Fatalf("JoinPatterns(..., \"posts\") = %q, want %q", got, want)
	}
	if got, want := JoinPatterns(rp, "/posts"), "/users/:id/posts"; got != want {
		t.Fatalf("JoinPatterns(..., \"/posts\") = %q, want %q", got, want)
	}
}

func TestRegisterPatternAndFindBestMatch(t *testing.T) {
	m := New(nil)
	m.RegisterPattern("/users/:id")

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
}

func TestFindNestedMatchesSmokeTest(t *testing.T) {
	m := New(&Options{})
	m.RegisterPattern("")
	m.RegisterPattern("/users")
	m.RegisterPattern("/users/:id")

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
}

func TestRegisterPatternPanicsOnNormalizedCollision(t *testing.T) {
	m := New(&Options{DynamicParamPrefix: '$'})
	m.RegisterPattern("/users/$id")

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic for normalized collision")
		}
		expected := `normalized pattern collision: "/users/:id" and "/users/$id" both normalize to "/users/:id"`
		if got := recovered.(string); got != expected {
			t.Fatalf("panic = %q, want %q", got, expected)
		}
	}()

	m.RegisterPattern("/users/:id")
}

/////////////////////////////////////////////////////////////////////
/////// BEST MATCH BENCHMARKS
/////////////////////////////////////////////////////////////////////

func setup_best_match_benchmark(scale string) *Matcher {
	m := New(&Options{Quiet: true})

	switch scale {
	case "small":
		m.RegisterPattern("/")
		m.RegisterPattern("/users")
		m.RegisterPattern("/users/:id")
		m.RegisterPattern("/users/:id/profile")
		m.RegisterPattern("/api/v1/users")
		m.RegisterPattern("/api/:version/users")
		m.RegisterPattern("/api/v1/users/:id")
		m.RegisterPattern("/files/*")
	case "medium":
		for i := range 1_000 {
			m.RegisterPattern(fmt.Sprintf("/api/v%d/users", i%5))
			m.RegisterPattern(fmt.Sprintf("/api/v%d/users/:id", i%5))
			m.RegisterPattern(
				fmt.Sprintf("/api/v%d/users/:id/posts/:post_id", i%5),
			)
			m.RegisterPattern(fmt.Sprintf("/files/bucket%d/*", i%10))
		}
	case "large":
		for i := range 10_000 {
			m.RegisterPattern(fmt.Sprintf("/api/v%d/users", i%10))
			m.RegisterPattern(fmt.Sprintf("/api/v%d/products", i%10))
			m.RegisterPattern(fmt.Sprintf("/docs/section%d", i%100))
			m.RegisterPattern(
				fmt.Sprintf("/api/v%d/users/:id/posts/:post_id", i%10),
			)
			m.RegisterPattern(
				fmt.Sprintf("/api/v%d/products/:category/:id", i%10),
			)
			m.RegisterPattern(fmt.Sprintf("/files/bucket%d/*", i%20))
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
			m := setup_best_match_benchmark("medium")
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
			m := setup_best_match_benchmark(scale)
			paths := best_match_benchmark_paths(scale)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				match, _ := m.FindBestMatch(paths[i%len(paths)])
				runtime.KeepAlive(match)
			}
		})
	}
	b.Run("WorstCase_DeepNested", func(b *testing.B) {
		m := setup_best_match_benchmark("large")
		path := "/api/v9/users/999/posts/999"
		b.ReportAllocs()
		for b.Loop() {
			match, _ := m.FindBestMatch(path)
			runtime.KeepAlive(match)
		}
	})
}
