// Package nestedmatcher provides nested route matching semantics where one real
// path can resolve to multiple hierarchical route patterns.
//
// It is intended for loader/layout-style routing where parent and leaf routes
// are accumulated together as one ordered match stack.
package nestedmatcher

import "github.com/vormadev/vorma/kit/internal/matchercore"

type (
	// Params stores dynamic path parameter values extracted from a match.
	Params = matchercore.Params
	// SegmentType identifies what kind of route segment is represented.
	SegmentType = matchercore.SegmentType
	// Segment is a normalized pattern segment and its classification.
	Segment = matchercore.Segment
	// RegisteredPattern is normalized metadata for one registered pattern.
	RegisteredPattern = matchercore.RegisteredPattern
	// Match represents one nested route match.
	Match = matchercore.Match
	// Results stores nested match resolution output.
	Results = matchercore.FindNestedMatchesResults
	// Options configures nested matcher behavior.
	Options = matchercore.Options
)

// Matcher resolves nested route matches.
type Matcher struct {
	engine *matchercore.Matcher
}

// New constructs a nested matcher with defaulted options.
func New(opts *Options) *Matcher {
	return &Matcher{engine: matchercore.New(opts)}
}

// ExplicitIndexSegmentIdentifier returns the explicit index marker.
func (m *Matcher) ExplicitIndexSegmentIdentifier() string {
	return m.engine.ExplicitIndexSegmentIdentifier()
}

// DynamicParamPrefix returns the dynamic parameter prefix rune.
func (m *Matcher) DynamicParamPrefix() rune {
	return m.engine.DynamicParamPrefix()
}

// SplatSegmentIdentifier returns the splat segment identifier rune.
func (m *Matcher) SplatSegmentIdentifier() rune {
	return m.engine.SplatSegmentIdentifier()
}

// NormalizePattern validates and normalizes one input pattern.
func (m *Matcher) NormalizePattern(originalPattern string) *RegisteredPattern {
	return m.engine.NormalizePattern(originalPattern)
}

// RegisterPattern registers one pattern for nested matching.
func (m *Matcher) RegisterPattern(originalPattern string) *RegisteredPattern {
	return m.engine.RegisterPattern(originalPattern)
}

// FindMatches resolves all nested matches for a real path.
func (m *Matcher) FindMatches(realPath string) (*Results, bool) {
	return m.engine.FindNestedMatches(realPath)
}
