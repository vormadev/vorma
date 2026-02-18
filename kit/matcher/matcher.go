package matcher

import (
	"strings"

	"github.com/vormadev/vorma/kit/genericsutil"
)

type (
	Params = map[string]string

	pattern     = string
	patternsMap = map[pattern]*RegisteredPattern
	matchesMap  = map[pattern]*Match
)

type SegmentType string

type Segment struct {
	NormalizedVal string
	SegType       SegmentType
}

type Matcher struct {
	staticPatterns  patternsMap
	dynamicPatterns patternsMap
	rootNode        *segmentNode

	explicitIndexSegment   string
	dynamicParamPrefixRune rune
	splatSegmentRune       rune

	slashIndexSegment                   string
	usingExplicitIndexSegmentIdentifier bool

	quiet bool
}

func (m *Matcher) ExplicitIndexSegmentIdentifier() string {
	return m.explicitIndexSegment
}
func (m *Matcher) DynamicParamPrefix() rune {
	return m.dynamicParamPrefixRune
}
func (m *Matcher) SplatSegmentIdentifier() rune {
	return m.splatSegmentRune
}

type Match struct {
	*RegisteredPattern
	params      Params
	splatValues []string
}

type BestMatch struct {
	*RegisteredPattern
	Params      Params
	SplatValues []string

	score uint16
}

type Options struct {
	DynamicParamPrefix     rune // Optional. Defaults to ':'.
	SplatSegmentIdentifier rune // Optional. Defaults to '*'.

	// Optional. Defaults to empty string (effectively a trailing slash in the pattern).
	// Could also be something like "_index" if preferred by the user.
	ExplicitIndexSegmentIdentifier string

	Quiet bool // Optional. Defaults to false. Set to true if you want to quash warnings.
}

func New(opts *Options) *Matcher {
	var instance = new(Matcher)

	instance.staticPatterns = make(patternsMap)
	instance.dynamicPatterns = make(patternsMap)
	instance.rootNode = new(segmentNode)

	mungedOpts := mungeOptsToDefaults(opts)

	instance.explicitIndexSegment = mungedOpts.ExplicitIndexSegmentIdentifier
	instance.dynamicParamPrefixRune = mungedOpts.DynamicParamPrefix
	instance.splatSegmentRune = mungedOpts.SplatSegmentIdentifier
	instance.quiet = mungedOpts.Quiet

	instance.slashIndexSegment = "/" + instance.explicitIndexSegment
	instance.usingExplicitIndexSegmentIdentifier = instance.explicitIndexSegment != ""

	return instance
}

func mungeOptsToDefaults(opts *Options) Options {
	if opts == nil {
		opts = new(Options)
	}

	copy := *opts

	if strings.Contains(copy.ExplicitIndexSegmentIdentifier, "/") {
		panic("explicit index segment cannot contain a slash")
	}

	copy.DynamicParamPrefix = genericsutil.OrDefault(copy.DynamicParamPrefix, ':')
	copy.SplatSegmentIdentifier = genericsutil.OrDefault(copy.SplatSegmentIdentifier, '*')
	copy.ExplicitIndexSegmentIdentifier = genericsutil.OrDefault(copy.ExplicitIndexSegmentIdentifier, "")

	return copy
}
