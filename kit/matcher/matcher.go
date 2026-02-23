// Package matcher provides route pattern matching with best-match semantics.
//
// It is intended for routers that need exactly one resolved route per request
// path, including support for dynamic params and splats.
package matcher

import "github.com/vormadev/vorma/kit/internal/matchercore"

type (
	// Params stores dynamic path parameter values extracted from a match.
	Params = matchercore.Params
	// SegmentType identifies the kind of one route segment.
	SegmentType = matchercore.SegmentType
	// Segment is a normalized pattern segment and its classification.
	Segment = matchercore.Segment
	// RegisteredPattern is normalized metadata for one registered pattern.
	RegisteredPattern = matchercore.RegisteredPattern
	// BestMatch is the highest-scoring single match for a path.
	BestMatch = matchercore.BestMatch
)

// Options configures matcher behavior.
type Options struct {
	// Optional. Defaults to ':'.
	DynamicParamPrefix rune
	// Optional. Defaults to '*'.
	SplatSegmentIdentifier rune
	// Optional. Defaults to false.
	Quiet bool
}

// Matcher resolves route patterns for one option configuration.
type Matcher struct {
	engine *matchercore.Matcher
}

// New constructs a matcher with defaulted options.
func New(opts *Options) *Matcher {
	if opts == nil {
		return &Matcher{engine: matchercore.New(nil)}
	}
	return &Matcher{
		engine: matchercore.New(&matchercore.Options{
			DynamicParamPrefix:     opts.DynamicParamPrefix,
			SplatSegmentIdentifier: opts.SplatSegmentIdentifier,
			Quiet:                  opts.Quiet,
		}),
	}
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

// RegisterPattern registers one pattern into matcher indices.
func (m *Matcher) RegisterPattern(originalPattern string) *RegisteredPattern {
	return m.engine.RegisterPattern(originalPattern)
}

// FindBestMatch resolves the highest-scoring single match for realPath.
func (m *Matcher) FindBestMatch(realPath string) (*BestMatch, bool) {
	return m.engine.FindBestMatch(realPath)
}

// ParseSegments splits a path into matcher segments.
func ParseSegments(path string) []string {
	return matchercore.ParseSegments(path)
}

// HasLeadingSlash reports whether pattern starts with '/'.
func HasLeadingSlash(pattern string) bool {
	return matchercore.HasLeadingSlash(pattern)
}

// HasTrailingSlash reports whether pattern ends with '/'.
func HasTrailingSlash(pattern string) bool {
	return matchercore.HasTrailingSlash(pattern)
}

// EnsureLeadingSlash ensures a leading slash exists.
func EnsureLeadingSlash(pattern string) string {
	return matchercore.EnsureLeadingSlash(pattern)
}

// EnsureTrailingSlash ensures a trailing slash exists.
func EnsureTrailingSlash(pattern string) string {
	return matchercore.EnsureTrailingSlash(pattern)
}

// EnsureLeadingAndTrailingSlash ensures both leading and trailing slashes.
func EnsureLeadingAndTrailingSlash(pattern string) string {
	return matchercore.EnsureLeadingAndTrailingSlash(pattern)
}

// StripLeadingSlash removes a leading slash when present.
func StripLeadingSlash(pattern string) string {
	return matchercore.StripLeadingSlash(pattern)
}

// StripTrailingSlash removes a trailing slash when present.
func StripTrailingSlash(pattern string) string {
	return matchercore.StripTrailingSlash(pattern)
}

// JoinPatterns joins a registered pattern with a pattern suffix.
func JoinPatterns(rp *RegisteredPattern, pattern string) string {
	return matchercore.JoinPatterns(rp, pattern)
}
