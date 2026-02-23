package matcher

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"github.com/vormadev/vorma/kit/internal/matchercore/testutil"
)

const NOT_FOUND = "NOT FOUND"

type testCase struct {
	name              string
	patterns          []string
	path              string
	wantPattern       string
	wantParams        Params
	wantSplatSegments []string
}

func getTestCases() []testCase {
	return []testCase{
		// empty-str cases
		{
			name:        "home route -- should match empty-str",
			patterns:    []string{""},
			path:        "/",
			wantPattern: "",
			wantParams:  nil,
		},
		{
			name:        "home route -- idx should beat empty-str",
			patterns:    []string{"", "/"},
			path:        "/",
			wantPattern: "/",
			wantParams:  nil,
		},
		{
			name:        "home route -- empty-str should beat root-splat",
			patterns:    []string{"", "/*"},
			path:        "/",
			wantPattern: "",
			wantParams:  nil,
		},
		{
			name:        "home route -- idx should beat root-splat",
			patterns:    []string{"/", "/*"},
			path:        "/",
			wantPattern: "/",
			wantParams:  nil,
		},
		{
			name:        "home route -- idx should win (empty-str, idx, root-splat registered)",
			patterns:    []string{"", "/", "/*"},
			path:        "/",
			wantPattern: "/",
			wantParams:  nil,
		},
		{
			name:              "home route -- should match root-splat if no idx or empty-str",
			patterns:          []string{"/*"},
			path:              "/",
			wantPattern:       "/*",
			wantParams:        nil,
			wantSplatSegments: []string{""},
		},

		// TRAILING SLASH SHOULD NOT MATCH DYNAMIC ROUTE
		{
			name:              "trailing slash should not match following dynamic route",
			patterns:          []string{"/users/:user"},
			path:              "/users/",
			wantPattern:       NOT_FOUND,
			wantParams:        nil,
			wantSplatSegments: nil,
		},

		// TEST TRAILING SLASH BEHAVIOR RELATED TO STATIC MATCHES
		{
			name:              "exact match should win over following splat",
			patterns:          []string{"/", "/users", "/users/*", "/posts"},
			path:              "/users",
			wantPattern:       "/users",
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:              "exact match, with trailing slash, should win over catch-all",
			patterns:          []string{"/", "/users", "/users/*", "/posts"},
			path:              "/users/",
			wantPattern:       "/users",
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:              "with no trailing slash, should NOT match following catch-all",
			patterns:          []string{"/", "/users/*", "/posts"},
			path:              "/users",
			wantPattern:       NOT_FOUND,
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:              "with trailing slash, should match following catch-all",
			patterns:          []string{"/", "/users/*", "/posts"},
			path:              "/users/",
			wantPattern:       "/users/*",
			wantParams:        nil,
			wantSplatSegments: []string{""},
		},

		// SAME AS ABOVE, BUT WITH A TRAILING SLASH AS AN ACTUAL REGISTERED PATTERN
		{
			name: "with registered trailing slash -- exact match without trailing should win over catch-all and should win over registered pattern with trailing slash",
			patterns: []string{
				"/",
				"/users/",
				"/users",
				"/users/*",
				"/posts",
			},
			path:              "/users",
			wantPattern:       "/users",
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name: "with registered trailing slash -- exact match, with trailing slash, should win over catch-all and should match pattern with trailing slash, not pattern without trailing slash",
			patterns: []string{
				"/",
				"/users/",
				"/users",
				"/users/*",
				"/posts",
			},
			path:              "/users/",
			wantPattern:       "/users/",
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:              "with registered trailing slash -- with no trailing slash, should NOT match following catch-all, nor should it match a registered pattern with a trailing slash",
			patterns:          []string{"/", "/users/", "/users/*", "/posts"},
			path:              "/users",
			wantPattern:       NOT_FOUND,
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:              "with registered trailing slash -- with trailing slash, should match the registered pattern with a trailing slash, not the following catch-all",
			patterns:          []string{"/", "/users/", "/users/*", "/posts"},
			path:              "/users/",
			wantPattern:       "/users/",
			wantParams:        nil,
			wantSplatSegments: nil,
		},

		// TEST TRAILING SLASH BEHAVIOR RELATED TO DYNAMIC MATCHES
		{
			name:              "dynamic match should win over catch-all",
			patterns:          []string{"/", "/:user", "/:user/*", "/posts"},
			path:              "/bob",
			wantPattern:       "/:user",
			wantParams:        Params{"user": "bob"},
			wantSplatSegments: nil,
		},
		{
			name:              "dynamic match, with trailing slash, should win over catch-all",
			patterns:          []string{"/", "/:user", "/:user/*", "/posts"},
			path:              "/bob/",
			wantPattern:       "/:user",
			wantParams:        Params{"user": "bob"},
			wantSplatSegments: nil,
		},
		{
			name:              "dynamic - with no trailing slash, should NOT match following catch-all",
			patterns:          []string{"/", "/:user/*", "/posts"},
			path:              "/bob",
			wantPattern:       NOT_FOUND,
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:              "dynamic - with trailing slash, should match following catch-all",
			patterns:          []string{"/", "/:user/*", "/posts"},
			path:              "/bob/",
			wantPattern:       "/:user/*",
			wantParams:        Params{"user": "bob"},
			wantSplatSegments: []string{""},
		},

		// SAME AS ABOVE, BUT WITH A TRAILING SLASH AS AN ACTUAL REGISTERED PATTERN
		{
			name: "with registered trailing slash -- dynamic match without trailing should win over catch-all and should win over registered pattern with trailing slash",
			patterns: []string{
				"/",
				"/:user/",
				"/:user",
				"/:user/*",
				"/posts",
			},
			path:              "/bob",
			wantPattern:       "/:user",
			wantParams:        Params{"user": "bob"},
			wantSplatSegments: nil,
		},
		{
			name: "with registered trailing slash -- dynamic match, with trailing slash, should win over catch-all and should match pattern with trailing slash, not pattern without trailing slash",
			patterns: []string{
				"/",
				"/:user/",
				"/:user",
				"/:user/*",
				"/posts",
			},
			path:              "/bob/",
			wantPattern:       "/:user/",
			wantParams:        Params{"user": "bob"},
			wantSplatSegments: nil,
		},
		{
			name:              "with registered trailing slash -- dynamic with no trailing slash, should NOT match following catch-all, nor should it match a registered pattern with a trailing slash",
			patterns:          []string{"/", "/:user/", "/:user/*", "/posts"},
			path:              "/bob",
			wantPattern:       NOT_FOUND,
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:              "with registered trailing slash -- dynamic with trailing slash, should match the registered pattern with a trailing slash, not the following catch-all",
			patterns:          []string{"/", "/:user/", "/:user/*", "/posts"},
			path:              "/bob/",
			wantPattern:       "/:user/",
			wantParams:        Params{"user": "bob"},
			wantSplatSegments: nil,
		},

		// MORE TESTS
		{
			name: "parameter match",
			patterns: []string{
				"/users",
				"/users/:id",
				"/users/profile",
			},
			path:              "/users/123",
			wantPattern:       "/users/:id",
			wantParams:        Params{"id": "123"},
			wantSplatSegments: nil,
		},
		{
			name: "multiple matches",
			patterns: []string{
				"/",
				"/api",
				"/api/:version",
				"/api/v1",
			},
			path:              "/api/v1",
			wantPattern:       "/api/v1",
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:              "splat match",
			patterns:          []string{"/files", "/files/*"},
			path:              "/files/documents/report.pdf",
			wantPattern:       "/files/*",
			wantParams:        nil,
			wantSplatSegments: []string{"documents", "report.pdf"},
		},
		{
			name:              "no match",
			patterns:          []string{"/users", "/posts", "/settings"},
			path:              "/profile",
			wantPattern:       NOT_FOUND,
			wantParams:        nil,
			wantSplatSegments: nil,
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
			path:              "/api/v2/users/123/posts",
			wantPattern:       "/api/:version/users/:id/posts",
			wantParams:        Params{"version": "v2", "id": "123"},
			wantSplatSegments: nil,
		},
		{
			name:              "no patterns",
			patterns:          []string{},
			path:              "/users",
			wantPattern:       NOT_FOUND,
			wantParams:        nil,
			wantSplatSegments: nil,
		},
		{
			name:        "many params",
			patterns:    []string{"/api/:p1/:p2/:p3/:p4/:p5"},
			path:        "/api/a/b/c/d/e",
			wantPattern: "/api/:p1/:p2/:p3/:p4/:p5",
			wantParams: Params{
				"p1": "a",
				"p2": "b",
				"p3": "c",
				"p4": "d",
				"p5": "e",
			},
			wantSplatSegments: nil,
		},
		{
			name:              "nested no match",
			patterns:          []string{"/users/:id", "/users/:id/profile"},
			path:              "users/123/settings",
			wantPattern:       NOT_FOUND,
			wantParams:        nil,
			wantSplatSegments: nil,
		},
	}
}

func optionShapeForPatternRewrites(opts *Options) testutil.PatternOptionShape {
	if opts == nil {
		return testutil.PatternOptionShape{}
	}
	return testutil.PatternOptionShape{
		DynamicParamPrefix:             opts.DynamicParamPrefix,
		SplatSegmentIdentifier:         opts.SplatSegmentIdentifier,
		ExplicitIndexSegmentIdentifier: "",
	}
}

var differentOptsToTest = []*Options{
	{},
	{DynamicParamPrefix: '$'},
	{SplatSegmentIdentifier: '#'},
	{
		DynamicParamPrefix:     '<',
		SplatSegmentIdentifier: '>',
	},
}

func TestFindBestMatch(t *testing.T) {
	for _, opts := range differentOptsToTest {
		for _, tt := range getTestCases() {
			t.Run(tt.name, func(t *testing.T) {
				m := New(opts)
				for _, pattern := range testutil.RewritePatternsForOptionShape(
					tt.patterns,
					"",
					optionShapeForPatternRewrites(opts),
				) {
					m.RegisterPattern(pattern)
				}

				match, _ := m.FindBestMatch(tt.path)

				wantMatch := tt.wantPattern != NOT_FOUND

				if wantMatch && match == nil {
					t.Errorf(
						"FindBestMatch() match for %s = nil -- want %s",
						tt.path,
						tt.wantPattern,
					)
					return
				}

				if !wantMatch {
					if match != nil {
						t.Errorf(
							"FindBestMatch() match for %s = %v -- want nil",
							tt.path,
							match.RegisteredPattern.NormalizedPattern(),
						)
					}
					return
				}

				if match.NormalizedPattern() != tt.wantPattern {
					t.Errorf(
						"FindBestMatch() pattern = %q, want %q",
						match.NormalizedPattern(),
						tt.wantPattern,
					)
				}

				// Compare params, allowing nil == empty map
				if tt.wantParams == nil && len(match.Params) > 0 {
					t.Errorf(
						"FindBestMatch() params = %v, want nil",
						match.Params,
					)
				} else if tt.wantParams != nil && !reflect.DeepEqual(match.Params, tt.wantParams) {
					t.Errorf("FindBestMatch() params = %v, want %v", match.Params, tt.wantParams)
				}

				// Compare splat segments
				if !reflect.DeepEqual(match.SplatValues, tt.wantSplatSegments) {
					t.Errorf(
						"FindBestMatch() splat segments = %v (%d), want %v (%d)",
						match.SplatValues,
						len(match.SplatValues),
						tt.wantSplatSegments,
						len(tt.wantSplatSegments),
					)
				}
			})
		}
	}
}

func TestFindBestMatchAdditionalScenarios(t *testing.T) {
	// register /, /:slug, and /app, and make sure that /settings/account does
	// not match
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
/////// BENCHMARKS
/////////////////////////////////////////////////////////////////////

func setupNonNestedMatcherForBenchmark(scale string) *Matcher {
	m := New(&Options{Quiet: true})

	switch scale {
	case "small":
		// Basic patterns for simple tests
		m.RegisterPattern("/")
		m.RegisterPattern("/users")
		m.RegisterPattern("/users/:id")
		m.RegisterPattern("/users/:id/profile")
		m.RegisterPattern("/api/v1/users")
		m.RegisterPattern("/api/:version/users")
		m.RegisterPattern("/api/v1/users/:id")
		m.RegisterPattern("/files/*")

	case "medium":
		// RESTful API-style patterns
		for i := range 1_000 {
			m.RegisterPattern(fmt.Sprintf("/api/v%d/users", i%5))
			m.RegisterPattern(fmt.Sprintf("/api/v%d/users/:id", i%5))
			m.RegisterPattern(
				fmt.Sprintf("/api/v%d/users/:id/posts/:post_id", i%5),
			)
			m.RegisterPattern(fmt.Sprintf("/files/bucket%d/*", i%10))
		}

	case "large":
		// Complex application patterns
		for i := range 10_000 {
			// Static patterns
			m.RegisterPattern(fmt.Sprintf("/api/v%d/users", i%10))
			m.RegisterPattern(fmt.Sprintf("/api/v%d/products", i%10))
			m.RegisterPattern(fmt.Sprintf("/docs/section%d", i%100))

			// Dynamic patterns
			m.RegisterPattern(
				fmt.Sprintf("/api/v%d/users/:id/posts/:post_id", i%10),
			)
			m.RegisterPattern(
				fmt.Sprintf("/api/v%d/products/:category/:id", i%10),
			)

			// Splat patterns
			m.RegisterPattern(fmt.Sprintf("/files/bucket%d/*", i%20))
		}
	}

	return m
}

// generateNonNestedPathsForBenchmark creates test paths for different scenarios
func generateNonNestedPathsForBenchmark(scale string) []string {
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

		// Static paths (40%)
		for i := range 400 {
			paths = append(paths, fmt.Sprintf("/api/v%d/users", i%5))
		}

		// Dynamic paths (40%)
		for i := range 400 {
			paths = append(
				paths,
				fmt.Sprintf("/api/v%d/users/%d/posts/%d", i%5, i, i%100),
			)
		}

		// Splat paths (20%)
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
			var memStatsBefore runtime.MemStats
			runtime.ReadMemStats(&memStatsBefore)

			m := setupNonNestedMatcherForBenchmark("medium")

			runtime.GC()
			var memStatsAfter runtime.MemStats
			runtime.ReadMemStats(&memStatsAfter)

			b.ResetTimer()
			b.ReportAllocs()

			matches := 0
			for b.Loop() {
				match, ok := m.FindBestMatch(s.path)
				if ok {
					matches++
				}
				runtime.KeepAlive(match)
			}
		})
	}
}

func BenchmarkFindBestMatchAtScale(b *testing.B) {
	scales := []string{"small", "medium", "large"}

	for _, scale := range scales {
		b.Run(fmt.Sprintf("Scale_%s", scale), func(b *testing.B) {
			m := setupNonNestedMatcherForBenchmark(scale)
			paths := generateNonNestedPathsForBenchmark(scale)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				path := paths[i%len(paths)]
				match, _ := m.FindBestMatch(path)
				runtime.KeepAlive(match)
			}
		})
	}

	b.Run("WorstCase_DeepNested", func(b *testing.B) {
		m := setupNonNestedMatcherForBenchmark("large")
		path := "/api/v9/users/999/posts/999"
		b.ReportAllocs()
		for b.Loop() {
			match, _ := m.FindBestMatch(path)
			runtime.KeepAlive(match)
		}
	})
}
