package routeparse

import (
	"strings"
	"testing"
)

func TestNormalizeRouteDefinitionPatternsInInputOrder(t *testing.T) {
	t.Run("keeps valid patterns in original input order", func(t *testing.T) {
		inputPatterns := []string{
			"frontend/src/routes/core.vorma.routes.ts",
			"frontend/src/routes/extra.vorma.routes.ts",
		}

		gotPatterns, err := NormalizeRouteDefinitionPatternsInInputOrder(
			inputPatterns,
		)
		if err != nil {
			t.Fatalf(
				"normalizeRouteDefinitionPatternsInInputOrder returned error: %v",
				err,
			)
		}
		if len(gotPatterns) != len(inputPatterns) {
			t.Fatalf(
				"len(gotPatterns) = %d, want %d",
				len(gotPatterns),
				len(inputPatterns),
			)
		}
		for index, inputPattern := range inputPatterns {
			if gotPatterns[index] != inputPattern {
				t.Fatalf(
					"gotPatterns[%d] = %q, want %q",
					index,
					gotPatterns[index],
					inputPattern,
				)
			}
		}
	})

	t.Run("fails on empty or whitespace-only patterns", func(t *testing.T) {
		_, err := NormalizeRouteDefinitionPatternsInInputOrder(
			[]string{"", "  "},
		)
		if err == nil {
			t.Fatal("expected error for empty or whitespace-only patterns")
		}
		if !strings.Contains(err.Error(), "cannot be empty or whitespace") {
			t.Fatalf(
				"error = %q, expected empty-or-whitespace validation message",
				err,
			)
		}
	})

	t.Run(
		"fails on surrounding whitespace instead of trimming",
		func(t *testing.T) {
			_, err := NormalizeRouteDefinitionPatternsInInputOrder(
				[]string{" frontend/src/routes/core.vorma.routes.ts "},
			)
			if err == nil {
				t.Fatal(
					"expected error for pattern with surrounding whitespace",
				)
			}
			if !strings.Contains(
				err.Error(),
				"must not contain surrounding whitespace",
			) {
				t.Fatalf(
					"error = %q, expected surrounding-whitespace validation message",
					err,
				)
			}
		},
	)

	t.Run(
		"fails on duplicates instead of silently deduplicating",
		func(t *testing.T) {
			_, err := NormalizeRouteDefinitionPatternsInInputOrder(
				[]string{
					"frontend/src/routes/core.vorma.routes.ts",
					"frontend/src/routes/core.vorma.routes.ts",
				},
			)
			if err == nil {
				t.Fatal(
					"expected error for duplicate route definition patterns",
				)
			}
			if !strings.Contains(err.Error(), "duplicates an earlier pattern") {
				t.Fatalf(
					"error = %q, expected duplicate-pattern validation message",
					err,
				)
			}
		},
	)
}
