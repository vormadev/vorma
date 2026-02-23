// Package testutil provides shared matcher test helpers.
//
// It keeps test-only pattern transformation logic in one place so matcher and
// nestedmatcher behavior matrices do not drift.
package testutil

import (
	"strings"

	"github.com/vormadev/vorma/kit/internal/matchercore"
)

// PatternOptionShape describes option fields relevant to pattern
// transformation in matcher test suites.
type PatternOptionShape struct {
	DynamicParamPrefix             rune
	SplatSegmentIdentifier         rune
	ExplicitIndexSegmentIdentifier string
}

// NormalizePatternOptionShape applies matcher option defaults and validates
// index-segment shape rules used by tests.
func NormalizePatternOptionShape(
	optionShape PatternOptionShape,
) PatternOptionShape {
	copy := optionShape
	if strings.Contains(copy.ExplicitIndexSegmentIdentifier, "/") {
		panic("explicit index segment cannot contain a slash")
	}
	if copy.DynamicParamPrefix == 0 {
		copy.DynamicParamPrefix = ':'
	}
	if copy.SplatSegmentIdentifier == 0 {
		copy.SplatSegmentIdentifier = '*'
	}
	return copy
}

// RewritePatternsForOptionShape rewrites canonical test patterns to match the
// provided option shape and index segment mode.
func RewritePatternsForOptionShape(
	patterns []string,
	incomingIndexSegment string,
	optionShape PatternOptionShape,
) []string {
	normalizedOptionShape := NormalizePatternOptionShape(optionShape)

	matcherInstance := matchercore.New(&matchercore.Options{
		ExplicitIndexSegmentIdentifier: incomingIndexSegment,
		Quiet:                          true,
	})

	registeredPatterns := make([]*matchercore.RegisteredPattern, len(patterns))
	for i, pattern := range patterns {
		registeredPatterns[i] = matcherInstance.NormalizePattern(pattern)
	}

	rewrittenPatterns := make([]string, 0, len(registeredPatterns))
	for _, registeredPattern := range registeredPatterns {
		var builder strings.Builder
		for _, segment := range registeredPattern.NormalizedSegments() {
			builder.WriteString("/")
			switch segment.SegType {
			case matchercore.SegmentType("static"):
				builder.WriteString(segment.NormalizedVal)
			case matchercore.SegmentType("dynamic"):
				builder.WriteString(
					string(normalizedOptionShape.DynamicParamPrefix),
				)
				builder.WriteString(segment.NormalizedVal[1:])
			case matchercore.SegmentType("splat"):
				builder.WriteString(
					string(normalizedOptionShape.SplatSegmentIdentifier),
				)
			case matchercore.SegmentType("index"):
				builder.WriteString(
					normalizedOptionShape.ExplicitIndexSegmentIdentifier,
				)
			}
		}
		rewrittenPatterns = append(rewrittenPatterns, builder.String())
	}

	return rewrittenPatterns
}
