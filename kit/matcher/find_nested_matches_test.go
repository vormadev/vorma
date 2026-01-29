package matcher

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

var NestedPatterns = []string{
	"/_index",                                         // Index
	"/articles/_index",                                // Index
	"/articles/test/articles/_index",                  // Index
	"/bear/_index",                                    // Index
	"/dashboard/_index",                               // Index
	"/dashboard/customers/_index",                     // Index
	"/dashboard/customers/:customer_id/_index",        // Index
	"/dashboard/customers/:customer_id/orders/_index", // Index
	"/dynamic-index/:pagename/_index",                 // Index
	"/lion/_index",                                    // Index
	"/tiger/_index",                                   // Index
	"/tiger/:tiger_id/_index",                         // Index

	// NOTE: This will evaluate to an empty string -- should match to everything
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

	// for when you don't care about dynamic params but still want to match exactly one segment
	"/a/b/:",
	"/c/d/e/:_",
	"/f/g/h/i/:/:",
	"/j/k/l/m/n/:_/:_",
}

type TestNestedScenario struct {
	Path            string
	ExpectedMatches []string
	SplatValues     []string
	Params          Params
}

var NestedScenarios = []TestNestedScenario{
	{
		Path:        "/does-not-exist",
		SplatValues: []string{"does-not-exist"},
		ExpectedMatches: []string{
			"",
			"/*",
		},
	},
	{
		Path:        "/this-should-be-ignored",
		SplatValues: []string{"this-should-be-ignored"},
		ExpectedMatches: []string{
			"",
			"/*",
		},
	},
	{
		Path: "/",
		ExpectedMatches: []string{
			"",
			"/",
		},
	},
	{
		Path: "/lion",
		ExpectedMatches: []string{
			"",
			"/lion",
			"/lion/",
		},
	},
	{
		Path:        "/lion/123",
		SplatValues: []string{"123"},
		ExpectedMatches: []string{
			"",
			"/lion",
			"/lion/*",
		},
	},
	{
		Path:        "/lion/123/456",
		SplatValues: []string{"123", "456"},
		ExpectedMatches: []string{
			"",
			"/lion",
			"/lion/*",
		},
	},
	{
		Path:        "/lion/123/456/789",
		SplatValues: []string{"123", "456", "789"},
		ExpectedMatches: []string{
			"",
			"/lion",
			"/lion/*",
		},
	},
	{
		Path: "/tiger",
		ExpectedMatches: []string{
			"",
			"/tiger",
			"/tiger/",
		},
	},
	{
		Path:   "/tiger/123",
		Params: Params{"tiger_id": "123"},
		ExpectedMatches: []string{
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/",
		},
	},
	{
		Path:   "/tiger/123/456",
		Params: Params{"tiger_id": "123", "tiger_cub_id": "456"},
		ExpectedMatches: []string{
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/:tiger_cub_id",
		},
	},
	{
		Path:        "/tiger/123/456/789",
		Params:      Params{"tiger_id": "123"},
		SplatValues: []string{"456", "789"},
		ExpectedMatches: []string{
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/*",
		},
	},
	{
		Path: "/bear",
		ExpectedMatches: []string{
			"",
			"/bear",
			"/bear/",
		},
	},
	{
		Path:   "/bear/123",
		Params: Params{"bear_id": "123"},
		ExpectedMatches: []string{
			"",
			"/bear",
			"/bear/:bear_id",
		},
	},
	{
		Path:        "/bear/123/456",
		Params:      Params{"bear_id": "123"},
		SplatValues: []string{"456"},
		ExpectedMatches: []string{
			"",
			"/bear",
			"/bear/:bear_id",
			"/bear/:bear_id/*",
		},
	},
	{
		Path:        "/bear/123/456/789",
		Params:      Params{"bear_id": "123"},
		SplatValues: []string{"456", "789"},
		ExpectedMatches: []string{
			"",
			"/bear",
			"/bear/:bear_id",
			"/bear/:bear_id/*",
		},
	},
	{
		Path: "/dashboard",
		ExpectedMatches: []string{
			"",
			"/dashboard",
			"/dashboard/",
		},
	},
	{
		Path:        "/dashboard/asdf",
		SplatValues: []string{"asdf"},
		ExpectedMatches: []string{
			"",
			"/dashboard",
			"/dashboard/*",
		},
	},
	{
		Path: "/dashboard/customers",
		ExpectedMatches: []string{
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/",
		},
	},
	{
		Path:   "/dashboard/customers/123",
		Params: Params{"customer_id": "123"},
		ExpectedMatches: []string{
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/",
		},
	},
	{
		Path:   "/dashboard/customers/123/orders",
		Params: Params{"customer_id": "123"},
		ExpectedMatches: []string{
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/orders",
			"/dashboard/customers/:customer_id/orders/",
		},
	},
	{
		Path:   "/dashboard/customers/123/orders/456",
		Params: Params{"customer_id": "123", "order_id": "456"},
		ExpectedMatches: []string{
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/orders",
			"/dashboard/customers/:customer_id/orders/:order_id",
		},
	},
	{
		Path: "/articles",
		ExpectedMatches: []string{
			"",
			"/articles/",
		},
	},
	{
		Path:        "/articles/bob",
		SplatValues: []string{"articles", "bob"},
		ExpectedMatches: []string{
			"",
			"/*",
		},
	},
	{
		Path:        "/articles/test",
		SplatValues: []string{"articles", "test"},
		ExpectedMatches: []string{
			"",
			"/*",
		},
	},
	{
		Path: "/articles/test/articles",
		ExpectedMatches: []string{
			"",
			"/articles/test/articles/",
		},
	},
	{
		Path: "/dynamic-index/index",
		ExpectedMatches: []string{
			"",
			// no underscore prefix, so not really an index!
			"/dynamic-index/index",
		},
	},

	/*
		"/a/b/:",
		"/c/d/e/:_",
		"/f/g/h/i/:/:",
		"/j/k/l/m/n/:_/:_",
	*/
	{
		Path: "/a/b/hi",
		ExpectedMatches: []string{
			"",
			"/a/b/:",
		},
		Params: Params{"": "hi"},
	},
	{
		Path: "/c/d/e/hi",
		ExpectedMatches: []string{
			"",
			"/c/d/e/:_",
		},
		Params: Params{"_": "hi"},
	},
	{
		Path: "/f/g/h/i/hi/hi2",
		ExpectedMatches: []string{
			"",
			"/f/g/h/i/:/:",
		},
		Params: Params{"": "hi2"},
	},
	{
		Path: "/j/k/l/m/n/hi/hi2",
		ExpectedMatches: []string{
			"",
			"/j/k/l/m/n/:_/:_",
		},
		Params: Params{"_": "hi2"},
	},
}

func TestFindAllMatches(t *testing.T) {
	for _, opts := range differentOptsToTest {
		m := New(opts)

		for _, p := range modifyPatternsToOpts(NestedPatterns, "_index", opts) {
			m.RegisterPattern(p)
		}

		for _, tc := range NestedScenarios {
			t.Run(tc.Path, func(t *testing.T) {
				results, ok := m.FindNestedMatches(tc.Path)

				if !equalParams(tc.Params, results.Params) {
					t.Errorf("Expected params %v, got %v", tc.Params, results.Params)
				}
				if !equalSplat(tc.SplatValues, results.SplatValues) {
					t.Errorf("Expected splat values %v, got %v", tc.SplatValues, results.SplatValues)
				}

				actualMatches := results.Matches

				var errors []string

				// Check if there's a failure
				expectedCount := len(tc.ExpectedMatches)
				actualCount := len(actualMatches)

				fail := (!ok && expectedCount > 0) || (expectedCount != actualCount)

				// Compare each matched pattern
				for i := range max(expectedCount, actualCount) {
					if i < expectedCount && i < actualCount {
						expected := tc.ExpectedMatches[i]
						actual := actualMatches[i]

						// ---- Use helper functions to compare maps/slices ----
						if expected != actual.normalizedPattern {
							fail = true
							break
						}
					} else {
						fail = true
						break
					}
				}

				// Only output errors if a failure occurred
				if fail {
					errors = append(errors, fmt.Sprintf("\n===== Path: %q =====", tc.Path))

					// Expected matches exist but got none
					if !ok && expectedCount > 0 {
						errors = append(errors, "Expected matches but got none.")
					}

					// Length mismatch
					if expectedCount != actualCount {
						errors = append(errors, fmt.Sprintf("Expected %d matches, got %d", expectedCount, actualCount))
					}

					// Always output all expected and actual matches for debugging
					errors = append(errors, "Expected Matches:")
					for i, expected := range tc.ExpectedMatches {
						errors = append(errors, fmt.Sprintf(
							"  [%d] {Pattern: %q}",
							i, expected,
						))
					}

					errors = append(errors, "Actual Matches:")
					for i, actual := range actualMatches {
						errors = append(errors, fmt.Sprintf(
							"  [%d] {Pattern: %q}",
							i, actual.normalizedPattern,
						))
					}

					// Print only if something went wrong
					t.Error(strings.Join(errors, "\n"))
				}
			})
		}
	}
}

func TestFindAllMatchesAdditionalScenarios(t *testing.T) {
	testCases := []struct {
		name            string
		patterns        []string
		path            string
		expectMatch     bool
		expectedMatches []string
	}{
		{
			name:            "Invalid match with unhandled segment",
			patterns:        []string{"/", "/:slug", "/_index", "/app"},
			path:            "/settings/account",
			expectMatch:     false,
			expectedMatches: []string{},
		},
		{
			name:            "Deeper Invalid 'Almost' Match",
			patterns:        []string{"/dashboard/customers"},
			path:            "/dashboard/customers/reports",
			expectMatch:     false,
			expectedMatches: []string{},
		},
		{
			name:            "Splat as the Only Full Match",
			patterns:        []string{"/files/*", "/files/images"},
			path:            "/files/documents/report.pdf",
			expectMatch:     true,
			expectedMatches: []string{"/files/*"},
		},
		{
			name:            "Index Segment Edge Case with Extra Segment",
			patterns:        []string{"/articles/_index"},
			path:            "/articles/some-topic",
			expectMatch:     false,
			expectedMatches: []string{},
		},
		{
			name:            "No Root Fallback for Multi-Segment Path",
			patterns:        []string{"/"},
			path:            "/some/random/path",
			expectMatch:     false,
			expectedMatches: []string{},
		},
		{
			name:            "A",
			patterns:        []string{"/"},
			path:            "/",
			expectMatch:     true,
			expectedMatches: []string{"/"},
		},
		{
			name:            "B",
			patterns:        []string{"/*"},
			path:            "/",
			expectMatch:     true,
			expectedMatches: []string{"/*"},
		},
		{
			name:            "C",
			patterns:        []string{"/_index"},
			path:            "/",
			expectMatch:     true,
			expectedMatches: []string{"/_index"},
		},
		{
			name:            "AB",
			patterns:        []string{"/", "/*"},
			path:            "/",
			expectMatch:     true,
			expectedMatches: []string{"/", "/*"},
		},
		{
			name:            "AC",
			patterns:        []string{"/", "/_index"},
			path:            "/",
			expectMatch:     true,
			expectedMatches: []string{"/", "/_index"},
		},
		{
			name:            "BC",
			patterns:        []string{"/*", "/_index"},
			path:            "/",
			expectMatch:     true,
			expectedMatches: []string{"/_index"},
		},
		{
			name:            "ABC",
			patterns:        []string{"/", "/*", "/_index"},
			path:            "/",
			expectMatch:     true,
			expectedMatches: []string{"/", "/_index"},
		},
		{
			name:            "A-docs",
			patterns:        []string{"/"},
			path:            "/docs",
			expectMatch:     false,
			expectedMatches: []string{},
		},
		{
			name:            "B-docs",
			patterns:        []string{"/*"},
			path:            "/docs",
			expectMatch:     true,
			expectedMatches: []string{"/*"},
		},
		{
			name:            "C-docs",
			patterns:        []string{"/_index"},
			path:            "/docs",
			expectMatch:     false,
			expectedMatches: []string{},
		},
		{
			name:            "AB-docs",
			patterns:        []string{"/", "/*"},
			path:            "/docs",
			expectMatch:     true,
			expectedMatches: []string{"/", "/*"},
		},
		{
			name:            "AC-docs",
			patterns:        []string{"/", "/_index"},
			path:            "/docs",
			expectMatch:     false,
			expectedMatches: []string{},
		},
		{
			name:            "BC-docs",
			patterns:        []string{"/*", "/_index"},
			path:            "/docs",
			expectMatch:     true,
			expectedMatches: []string{"/*"},
		},
		{
			name:            "ABC-docs",
			patterns:        []string{"/", "/*", "/_index"},
			path:            "/docs",
			expectMatch:     true,
			expectedMatches: []string{"/", "/*"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(&Options{ExplicitIndexSegment: "_index", Quiet: true})
			for _, p := range tc.patterns {
				m.RegisterPattern(p)
			}

			results, ok := m.FindNestedMatches(tc.path)

			if ok != tc.expectMatch {
				t.Errorf("Expected match result %v for path %q, but got %v", tc.expectMatch, tc.path, ok)
			}

			// If no match was expected, ensure the results are truly empty.
			if !tc.expectMatch {
				if results != nil && len(results.Matches) != 0 {
					t.Errorf("Expected no matches for path %q, but got %d matches", tc.path, len(results.Matches))
					for i, match := range results.Matches {
						t.Logf("  [%d] %q", i, match.originalPattern)
					}
				}
			}

			// If a match was expected, check the specific patterns that matched
			if tc.expectMatch {
				if results == nil || len(results.Matches) == 0 {
					t.Errorf("Expected matches for path %q, but got none", tc.path)
				} else if tc.expectedMatches != nil {
					// Check that we got the expected patterns
					actualPatterns := make([]string, len(results.Matches))
					for i, match := range results.Matches {
						actualPatterns[i] = match.originalPattern
					}

					if len(actualPatterns) != len(tc.expectedMatches) {
						t.Errorf("Path %q: expected %d matches %v, got %d matches %v",
							tc.path, len(tc.expectedMatches), tc.expectedMatches,
							len(actualPatterns), actualPatterns)
					} else {
						for i, expected := range tc.expectedMatches {
							if actualPatterns[i] != expected {
								t.Errorf("Path %q: at position [%d], expected %q, got %q",
									tc.path, i, expected, actualPatterns[i])
							}
						}
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helper functions to treat nil maps/slices as empty, avoiding false mismatches
// ---------------------------------------------------------------------------

func equalParams(a, b Params) bool {
	// Consider nil and empty as the same
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func equalSplat(a, b []string) bool {
	// Consider nil and empty slice as the same
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func modifyPatternsToOpts(incomingPatterns []string, incomingIndexSegment string, opts_ *Options) []string {
	opts := mungeOptsToDefaults(opts_)

	m := New(&Options{ExplicitIndexSegment: incomingIndexSegment, Quiet: true})

	rps := make([]*RegisteredPattern, len(incomingPatterns))
	for i, p := range incomingPatterns {
		rps[i] = m.NormalizePattern(p)
	}

	newPatterns := make([]string, 0, len(rps))

	for _, rp := range rps {
		var sb strings.Builder

		for _, seg := range rp.normalizedSegments {
			sb.WriteString("/")
			switch seg.segType {
			case segTypes.static:
				sb.WriteString(seg.normalizedVal)
			case segTypes.dynamic:
				sb.WriteString(string(opts.DynamicParamPrefixRune))
				sb.WriteString(seg.normalizedVal[1:])
			case segTypes.splat:
				sb.WriteString(string(opts.SplatSegmentRune))
			case segTypes.index:
				sb.WriteString(string(opts.ExplicitIndexSegment))
			}
		}

		newPatterns = append(newPatterns, sb.String())
	}

	return newPatterns
}

/////////////////////////////////////////////////////////////////////
/////// BENCHMARKS
/////////////////////////////////////////////////////////////////////

func setupNestedMatcherForBenchmark() *Matcher {
	m := New(&Options{Quiet: true})

	for _, pattern := range NestedPatterns {
		m.RegisterPattern(pattern)
	}
	return m
}

func generateNestedPathsForBenchmark() []string {
	return []string{
		"/",                                   // Root index
		"/dashboard",                          // Static path with index
		"/dashboard/customers",                // Nested static path
		"/dashboard/customers/123",            // Path with params
		"/dashboard/customers/123/orders",     // Deep nested path
		"/dashboard/customers/123/orders/456", // Deep nested path with multiple params
		"/tiger",                              // Another static path
		"/tiger/123",                          // Dynamic path
		"/tiger/123/456",                      // Dynamic path with multiple params
		"/tiger/123/456/789",                  // Path with splat
		"/bear/123/456/789",                   // Different path with splat
		"/articles/test/articles",             // Deeply nested static path
		"/does-not-exist",                     // Non-existent path (tests splat handling)
		"/dashboard/unknown/path",             // Tests dashboard splat path
	}
}

func BenchmarkFindNestedMatches(b *testing.B) {
	cases := []struct {
		name     string
		pathType string
		paths    []string
	}{
		{
			name:     "StaticPatterns",
			pathType: "static",
			paths:    []string{"/", "/dashboard", "/dashboard/customers", "/tiger", "/lion"},
		},
		{
			name:     "DynamicPatterns",
			pathType: "dynamic",
			paths: []string{
				"/dashboard/customers/123",
				"/dashboard/customers/456/orders",
				"/tiger/123",
				"/bear/123",
			},
		},
		{
			name:     "DeepNestedPatterns",
			pathType: "deep",
			paths: []string{
				"/dashboard/customers/123/orders/456",
				"/tiger/123/456/789",
				"/bear/123/456/789",
				"/articles/test/articles",
			},
		},
		{
			name:     "SplatPatterns",
			pathType: "splat",
			paths: []string{
				"/does-not-exist",
				"/dashboard/unknown/path",
				"/tiger/123/456/789/extra",
				"/bear/123/456/789/extra",
			},
		},
		{
			name:     "MixedPatterns",
			pathType: "mixed",
			paths:    generateNestedPathsForBenchmark(),
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			m := setupNestedMatcherForBenchmark()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				path := tc.paths[i%len(tc.paths)]
				matches, _ := m.FindNestedMatches(path)
				runtime.KeepAlive(matches)
			}
		})
	}
}

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

	testCases := []struct {
		name              string
		path              string
		expectedMatches   []string
		unexpectedMatches []string
	}{
		{
			name: "about with trailing slash",
			path: "/about/",
			expectedMatches: []string{
				"/",
				"/about",
			},
			unexpectedMatches: []string{
				"/about/:id",
				"/about/*",
			},
		},
		{
			name: "about without trailing slash",
			path: "/about",
			expectedMatches: []string{
				"/",
				"/about",
			},
			unexpectedMatches: []string{
				"/about/:id",
				"/about/*",
			},
		},
		{
			name: "about with actual id",
			path: "/about/123",
			expectedMatches: []string{
				"/",
				"/about/:id",
			},
			unexpectedMatches: []string{
				"/about/*",
			},
		},
		{
			name: "about location exact match",
			path: "/about/location",
			expectedMatches: []string{
				"/",
				"/about",
				"/about/location",
			},
			unexpectedMatches: []string{
				"/_index",
				"/about/:id", // exact match should take precedence
				"/about/*",   // exact match should take precedence
			},
		},
		{
			name: "about with multiple segments",
			path: "/about/something/else",
			expectedMatches: []string{
				"/",
				"/about/*", // should catch multiple segments
			},
			unexpectedMatches: []string{
				"/about/:id",      // only handles one segment
				"/about/location", // not an exact match
			},
		},
	}

	m := New(&Options{ExplicitIndexSegment: "_index", Quiet: true})
	for _, p := range patterns {
		m.RegisterPattern(p)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, ok := m.FindNestedMatches(tc.path)

			if !ok {
				t.Errorf("Path %q: expected to find matches, but got none", tc.path)
			}

			// Extract the patterns from results
			actualPatterns := make([]*Match, len(results.Matches))
			copy(actualPatterns, results.Matches)

			// Check for expected patterns
			for _, expected := range tc.expectedMatches {
				found := false
				for _, actual := range actualPatterns {
					if actual.originalPattern == expected {
						found = true
						break
					}
				}
				if !found && expected != "" { // ignore empty string check for now
					t.Errorf("Path %q: expected pattern %q to match, but it didn't",
						tc.path, expected)
				}
			}

			// Check for unexpected patterns
			for _, unexpected := range tc.unexpectedMatches {
				for _, actual := range actualPatterns {
					if actual.originalPattern == unexpected {
						t.Errorf("Path %q: pattern %q should NOT match, but it did",
							tc.path, unexpected)
					}
				}
			}

			// Log actual matches for debugging
			if t.Failed() {
				t.Logf("Actual matches for %q:", tc.path)
				for i, pattern := range actualPatterns {
					t.Logf("  [%d] %q", i, pattern.originalPattern)
				}

				// Also log params if this is the trailing slash case
				if tc.path == "/about/" && results.Params != nil && len(results.Params) > 0 {
					t.Logf("Params captured: %+v", results.Params)
				}
			}
		})
	}
}

func TestPartialMatchingWithGaps(t *testing.T) {
	t.Run("should match parent and deeply nested route without intermediate routes", func(t *testing.T) {
		m := New(&Options{ExplicitIndexSegment: "_index"})

		// Register only the parent and the deeply nested route
		// NOT registering /bob/larry or /bob/larry/susan
		m.RegisterPattern("/bob")
		m.RegisterPattern("/bob/larry/susan/jeff")

		// Try to match the full path
		results, ok := m.FindNestedMatches("/bob/larry/susan/jeff")

		if !ok {
			t.Fatal("Expected to find matches for /bob/larry/susan/jeff")
		}

		if len(results.Matches) != 2 {
			t.Errorf("Expected 2 matches, got %d", len(results.Matches))
			for i, match := range results.Matches {
				t.Logf("  [%d] %q", i, match.originalPattern)
			}
		}

		// Check that we got the right patterns
		foundBob := false
		foundJeff := false
		for _, match := range results.Matches {
			if match.originalPattern == "/bob" {
				foundBob = true
			}
			if match.originalPattern == "/bob/larry/susan/jeff" {
				foundJeff = true
			}
		}

		if !foundBob {
			t.Error("Expected /bob to match")
		}
		if !foundJeff {
			t.Error("Expected /bob/larry/susan/jeff to match")
		}
	})

	t.Run("should not match intermediate paths that aren't registered", func(t *testing.T) {
		m := New(&Options{ExplicitIndexSegment: "_index"})

		m.RegisterPattern("/bob")
		m.RegisterPattern("/bob/larry/susan/jeff")

		// Try to match an intermediate path
		results, ok := m.FindNestedMatches("/bob/larry")

		// This should NOT find a match because /bob/larry isn't registered
		// and /bob/larry/susan/jeff doesn't match
		if ok {
			t.Error("Should not find matches for /bob/larry when only /bob and /bob/larry/susan/jeff are registered")
			for i, match := range results.Matches {
				t.Logf("  Found unexpected match [%d] %q", i, match.originalPattern)
			}
		}
	})
}

// TestMatchOrderingDeterminism checks that match ordering and params
// are consistent across many iterations, exposing any non-determinism
// from map iteration order.
func TestMatchOrderingDeterminism(t *testing.T) {
	t.Run("static vs dynamic same depth", func(t *testing.T) {
		var firstParams Params
		var firstOrder []string

		for i := range 1000 {
			m := New(&Options{Quiet: true})
			m.RegisterPattern("/api/v1")
			m.RegisterPattern("/api/:version")

			results, ok := m.FindNestedMatches("/api/v1")
			if !ok {
				t.Fatal("Expected matches")
			}

			currentOrder := make([]string, len(results.Matches))
			for j, match := range results.Matches {
				currentOrder[j] = match.normalizedPattern
			}

			if i == 0 {
				firstParams = results.Params
				firstOrder = currentOrder
				continue
			}

			if !reflect.DeepEqual(results.Params, firstParams) {
				t.Fatalf("Iteration %d: params inconsistent. First: %v, Now: %v",
					i, firstParams, results.Params)
			}

			if !reflect.DeepEqual(currentOrder, firstOrder) {
				t.Fatalf("Iteration %d: match order inconsistent. First: %v, Now: %v",
					i, firstOrder, currentOrder)
			}
		}
	})

	t.Run("multiple dynamic same depth", func(t *testing.T) {
		var firstParams Params

		for i := range 1000 {
			m := New(&Options{Quiet: true})
			m.RegisterPattern("/users/:id")
			m.RegisterPattern("/users/:user_id")

			results, ok := m.FindNestedMatches("/users/123")
			if !ok {
				t.Fatal("Expected matches")
			}

			if i == 0 {
				firstParams = results.Params
				if len(firstParams) == 0 {
					t.Fatal("Expected params from dynamic match")
				}
				continue
			}

			if !reflect.DeepEqual(results.Params, firstParams) {
				t.Fatalf("Iteration %d: params inconsistent. First: %v, Now: %v",
					i, firstParams, results.Params)
			}
		}
	})

	t.Run("three patterns same depth", func(t *testing.T) {
		var firstParams Params
		var firstOrder []string

		for i := range 1000 {
			m := New(&Options{Quiet: true})
			m.RegisterPattern("/a/b")
			m.RegisterPattern("/a/:p")
			m.RegisterPattern("/:x/b")

			results, ok := m.FindNestedMatches("/a/b")
			if !ok {
				t.Fatal("Expected matches")
			}

			currentOrder := make([]string, len(results.Matches))
			for j, match := range results.Matches {
				currentOrder[j] = match.normalizedPattern
			}

			if i == 0 {
				firstParams = results.Params
				firstOrder = currentOrder
				continue
			}

			if !reflect.DeepEqual(results.Params, firstParams) {
				t.Fatalf("Iteration %d: params inconsistent. First: %v, Now: %v",
					i, firstParams, results.Params)
			}

			if !reflect.DeepEqual(currentOrder, firstOrder) {
				t.Fatalf("Iteration %d: match order inconsistent. First: %v, Now: %v",
					i, firstOrder, currentOrder)
			}
		}
	})
}
