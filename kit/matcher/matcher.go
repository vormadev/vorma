package matcher

import (
	"strings"

	"github.com/vormadev/vorma/kit/genericsutil"
)

type (
	Params = map[string]string

	pattern     = string
	segType     = string
	patternsMap = map[pattern]*RegisteredPattern
	matchesMap  = map[pattern]*Match
)

type Matcher struct {
	staticPatterns  patternsMap
	dynamicPatterns patternsMap
	rootNode        *segmentNode

	explicitIndexSegment   string
	dynamicParamPrefixRune rune
	splatSegmentRune       rune

	slashIndexSegment         string
	usingExplicitIndexSegment bool

	quiet bool
}

func (m *Matcher) GetExplicitIndexSegment() string {
	return m.explicitIndexSegment
}
func (m *Matcher) GetDynamicParamPrefixRune() rune {
	return m.dynamicParamPrefixRune
}
func (m *Matcher) GetSplatSegmentRune() rune {
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
	DynamicParamPrefixRune rune // Optional. Defaults to ':'.
	SplatSegmentRune       rune // Optional. Defaults to '*'.

	// Optional. Defaults to empty string (effectively a trailing slash in the pattern).
	// Could also be something like "_index" if preferred by the user.
	ExplicitIndexSegment string

	Quiet bool // Optional. Defaults to false. Set to true if you want to quash warnings.
}

func New(opts *Options) *Matcher {
	var instance = new(Matcher)

	instance.staticPatterns = make(patternsMap)
	instance.dynamicPatterns = make(patternsMap)
	instance.rootNode = new(segmentNode)

	mungedOpts := mungeOptsToDefaults(opts)

	instance.explicitIndexSegment = mungedOpts.ExplicitIndexSegment
	instance.dynamicParamPrefixRune = mungedOpts.DynamicParamPrefixRune
	instance.splatSegmentRune = mungedOpts.SplatSegmentRune
	instance.quiet = mungedOpts.Quiet

	instance.slashIndexSegment = "/" + instance.explicitIndexSegment
	instance.usingExplicitIndexSegment = instance.explicitIndexSegment != ""

	return instance
}

func mungeOptsToDefaults(opts *Options) Options {
	if opts == nil {
		opts = new(Options)
	}

	copy := *opts

	if strings.Contains(copy.ExplicitIndexSegment, "/") {
		panic("explicit index segment cannot contain a slash")
	}

	copy.DynamicParamPrefixRune = genericsutil.OrDefault(copy.DynamicParamPrefixRune, ':')
	copy.SplatSegmentRune = genericsutil.OrDefault(copy.SplatSegmentRune, '*')
	copy.ExplicitIndexSegment = genericsutil.OrDefault(copy.ExplicitIndexSegment, "")

	return copy
}
