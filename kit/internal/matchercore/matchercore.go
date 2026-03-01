// Package matchercore provides the shared route-pattern matching engine used by
// matcher and nestedmatcher.
//
// It centralizes normalization, registration, and matching semantics so both
// public facades stay behaviorally identical while avoiding duplicated state
// machines.
package matchercore

import (
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/genericsutil"
)

type (
	// Params stores dynamic path parameter values extracted from a match.
	Params = map[string]string

	pattern     = string
	patternsMap = map[pattern]*RegisteredPattern
	matchesMap  = map[pattern]*Match
)

// SegmentType identifies what kind of route segment is represented.
type SegmentType string

// Segment is a normalized pattern segment and its classification.
type Segment struct {
	NormalizedVal string
	SegType       SegmentType
}

// Matcher is the shared matching engine used by matcher and nestedmatcher.
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

// ExplicitIndexSegmentIdentifier returns the configured explicit index marker.
func (m *Matcher) ExplicitIndexSegmentIdentifier() string {
	return m.explicitIndexSegment
}

// DynamicParamPrefix returns the configured dynamic parameter prefix rune.
func (m *Matcher) DynamicParamPrefix() rune {
	return m.dynamicParamPrefixRune
}

// SplatSegmentIdentifier returns the configured splat segment identifier rune.
func (m *Matcher) SplatSegmentIdentifier() rune {
	return m.splatSegmentRune
}

// Match represents one nested match entry with optional params and splat values.
type Match struct {
	*RegisteredPattern
	params      Params
	splatValues []string
}

// Params returns a defensive copy of dynamic params for this match.
func (m *Match) Params() Params {
	if m == nil || len(m.params) == 0 {
		return nil
	}
	paramsCopy := make(Params, len(m.params))
	maps.Copy(paramsCopy, m.params)
	return paramsCopy
}

// SplatValues returns a defensive copy of splat values for this match.
func (m *Match) SplatValues() []string {
	if m == nil || len(m.splatValues) == 0 {
		return nil
	}
	splatCopy := make([]string, len(m.splatValues))
	copy(splatCopy, m.splatValues)
	return splatCopy
}

// BestMatch represents the best single route match for a real path.
type BestMatch struct {
	*RegisteredPattern
	Params      Params
	SplatValues []string

	score uint16
}

// FindNestedMatchesResults stores nested route matches and top-level params.
type FindNestedMatchesResults struct {
	Params      Params
	SplatValues []string
	Matches     []*Match
}

// Options configures matching semantics.
type Options struct {
	// Optional. Defaults to ':'.
	DynamicParamPrefix rune
	// Optional. Defaults to '*'.
	SplatSegmentIdentifier rune
	// Optional. Defaults to empty string (effectively trailing slash index).
	ExplicitIndexSegmentIdentifier string
	// Optional. Defaults to false.
	Quiet bool
}

// New constructs a matcher engine with defaulted options.
func New(opts *Options) *Matcher {
	instance := new(Matcher)

	instance.staticPatterns = make(patternsMap)
	instance.dynamicPatterns = make(patternsMap)
	instance.rootNode = new(segmentNode)

	mungedOpts := mungeOptsToDefaults(opts)

	instance.explicitIndexSegment = mungedOpts.ExplicitIndexSegmentIdentifier
	instance.dynamicParamPrefixRune = mungedOpts.DynamicParamPrefix
	instance.splatSegmentRune = mungedOpts.SplatSegmentIdentifier
	instance.quiet = mungedOpts.Quiet

	instance.slashIndexSegment = "/" + instance.explicitIndexSegment
	instance.usingExplicitIndexSegmentIdentifier =
		instance.explicitIndexSegment != ""

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

	copy.DynamicParamPrefix = genericsutil.OrDefault(
		copy.DynamicParamPrefix,
		':',
	)
	copy.SplatSegmentIdentifier =
		genericsutil.OrDefault(copy.SplatSegmentIdentifier, '*')
	copy.ExplicitIndexSegmentIdentifier =
		genericsutil.OrDefault(copy.ExplicitIndexSegmentIdentifier, "")

	return copy
}

const (
	nodeStatic       uint8 = 0
	nodeDynamic      uint8 = 1
	nodeSplat        uint8 = 2
	scoreStaticMatch       = 2
	scoreDynamic           = 1
)

// RegisteredPattern is the matcher-normalized metadata for one pattern.
type RegisteredPattern struct {
	originalPattern          string
	normalizedPattern        string
	normalizedSegments       []Segment
	lastSegType              SegmentType
	lastSegIsNonRootSplat    bool
	lastSegIsIndex           bool
	numberOfDynamicParamSegs uint8
}

// NormalizedPattern returns the normalized pattern shape.
func (rp *RegisteredPattern) NormalizedPattern() string {
	return rp.normalizedPattern
}

// NormalizedSegments returns a defensive copy of normalized segments.
func (rp *RegisteredPattern) NormalizedSegments() []Segment {
	out := make([]Segment, len(rp.normalizedSegments))
	copy(out, rp.normalizedSegments)
	return out
}

// OriginalPattern returns the original pattern string as registered.
func (rp *RegisteredPattern) OriginalPattern() string {
	return rp.originalPattern
}

// HasLeadingSlash reports whether pattern starts with '/'.
func HasLeadingSlash(pattern string) bool {
	return len(pattern) > 0 && pattern[0] == '/'
}

// HasTrailingSlash reports whether pattern ends with '/'.
func HasTrailingSlash(pattern string) bool {
	return len(pattern) > 0 && pattern[len(pattern)-1] == '/'
}

// EnsureLeadingSlash ensures a leading slash exists.
func EnsureLeadingSlash(pattern string) string {
	if !HasLeadingSlash(pattern) {
		return "/" + pattern
	}
	return pattern
}

// EnsureTrailingSlash ensures a trailing slash exists.
func EnsureTrailingSlash(pattern string) string {
	if !HasTrailingSlash(pattern) {
		return pattern + "/"
	}
	return pattern
}

// EnsureLeadingAndTrailingSlash ensures both leading and trailing slashes.
func EnsureLeadingAndTrailingSlash(pattern string) string {
	return EnsureLeadingSlash(EnsureTrailingSlash(pattern))
}

// StripLeadingSlash removes a leading slash when present.
func StripLeadingSlash(pattern string) string {
	if HasLeadingSlash(pattern) {
		return pattern[1:]
	}
	return pattern
}

// StripTrailingSlash removes a trailing slash when present.
func StripTrailingSlash(pattern string) string {
	if HasTrailingSlash(pattern) {
		return pattern[:len(pattern)-1]
	}
	return pattern
}

// JoinPatterns joins a registered pattern to another pattern segment.
func JoinPatterns(rp *RegisteredPattern, pattern string) string {
	var sb strings.Builder
	base := rp.normalizedPattern
	sb.WriteString(base)

	patternHasLeadingSlash := HasLeadingSlash(pattern)

	if HasTrailingSlash(base) && patternHasLeadingSlash {
		pattern = pattern[1:]
	} else if !patternHasLeadingSlash {
		sb.WriteString("/")
	}

	sb.WriteString(pattern)

	return sb.String()
}

var segTypes = struct {
	splat   SegmentType
	static  SegmentType
	dynamic SegmentType
	index   SegmentType
}{
	splat:   "splat",
	static:  "static",
	dynamic: "dynamic",
	index:   "index",
}

// NormalizePattern validates and normalizes one input pattern.
func (m *Matcher) NormalizePattern(originalPattern string) *RegisteredPattern {
	normalizedPattern := originalPattern

	if m.usingExplicitIndexSegmentIdentifier {
		if strings.HasSuffix(normalizedPattern, "/") {
			if normalizedPattern != "/" {
				log.Panicf(
					"Error with pattern '%s'. With the exception of any absolute root pattern ('/'), trailing slashes are not permitted when using an explicit index segment. If you intend to make this an index route, add your explicit index segment. Otherwise, remove the trailing slash.",
					originalPattern,
				)
			}
			normalizedPattern = strings.TrimRight(normalizedPattern, "/")
		}
		if strings.HasSuffix(normalizedPattern, m.slashIndexSegment) {
			normalizedPattern = strings.TrimSuffix(
				normalizedPattern,
				m.explicitIndexSegment,
			)
		}
	}

	rawSegments := ParseSegments(normalizedPattern)
	segments := make([]Segment, 0, len(rawSegments))

	var numberOfDynamicParamSegs uint8

	for _, seg := range rawSegments {
		normalizedVal := seg

		segType := m.getSegmentTypeAssumeNormalized(seg)
		if segType == segTypes.dynamic {
			numberOfDynamicParamSegs++
			normalizedVal = ":" + seg[1:]
		}
		if segType == segTypes.splat {
			normalizedVal = "*"
		}

		segments = append(segments, Segment{
			NormalizedVal: normalizedVal,
			SegType:       segType,
		})
	}

	segLen := len(segments)
	var lastType SegmentType
	if segLen > 0 {
		lastType = segments[segLen-1].SegType
	}

	var finalNormalizedPatternBuilder strings.Builder
	finalNormalizedPatternBuilder.WriteString("/")
	for i, seg := range segments {
		finalNormalizedPatternBuilder.WriteString(seg.NormalizedVal)
		if i < segLen-1 {
			finalNormalizedPatternBuilder.WriteString("/")
		}
	}

	finalNormalizedPattern := finalNormalizedPatternBuilder.String()

	if strings.HasSuffix(finalNormalizedPattern, "/") &&
		lastType != segTypes.index {
		finalNormalizedPattern = strings.TrimRight(finalNormalizedPattern, "/")
	}

	return &RegisteredPattern{
		originalPattern:          originalPattern,
		normalizedPattern:        finalNormalizedPattern,
		normalizedSegments:       segments,
		lastSegType:              lastType,
		lastSegIsNonRootSplat:    lastType == segTypes.splat && segLen > 1,
		lastSegIsIndex:           lastType == segTypes.index,
		numberOfDynamicParamSegs: numberOfDynamicParamSegs,
	}
}

// RegisterPattern registers one pattern into static/dynamic indices.
func (m *Matcher) RegisterPattern(originalPattern string) *RegisteredPattern {
	normalizedPattern := m.NormalizePattern(originalPattern)

	if existingPattern, isAlreadyRegistered :=
		m.staticPatterns[normalizedPattern.normalizedPattern]; isAlreadyRegistered {
		if existingPattern.originalPattern == originalPattern {
			return existingPattern
		}

		panic(fmt.Sprintf(
			`normalized pattern collision: "%s" and "%s" both normalize to "%s"`,
			originalPattern,
			existingPattern.originalPattern,
			normalizedPattern.normalizedPattern,
		))
	}
	if existingPattern, isAlreadyRegistered :=
		m.dynamicPatterns[normalizedPattern.normalizedPattern]; isAlreadyRegistered {
		if existingPattern.originalPattern == originalPattern {
			return existingPattern
		}

		panic(fmt.Sprintf(
			`normalized pattern collision: "%s" and "%s" both normalize to "%s"`,
			originalPattern,
			existingPattern.originalPattern,
			normalizedPattern.normalizedPattern,
		))
	}

	if getIsStatic(normalizedPattern.normalizedSegments) {
		m.staticPatterns[normalizedPattern.normalizedPattern] = normalizedPattern
		return normalizedPattern
	}

	m.dynamicPatterns[normalizedPattern.normalizedPattern] = normalizedPattern

	current := m.rootNode
	var nodeScore int

	for i, segment := range normalizedPattern.normalizedSegments {
		child := current.findOrCreateChild(segment.NormalizedVal)
		switch {
		case segment.SegType == segTypes.dynamic:
			nodeScore += scoreDynamic
		case segment.SegType != segTypes.splat:
			nodeScore += scoreStaticMatch
		}

		if i == len(normalizedPattern.normalizedSegments)-1 {
			child.finalScore = nodeScore
			child.pattern = normalizedPattern.normalizedPattern
		}

		current = child
	}

	return normalizedPattern
}

func (m *Matcher) getSegmentTypeAssumeNormalized(segment string) SegmentType {
	switch {
	case segment == "":
		return segTypes.index
	case len(segment) == 1 && segment == string(m.splatSegmentRune):
		return segTypes.splat
	case len(segment) > 0 && segment[0] == byte(m.dynamicParamPrefixRune):
		return segTypes.dynamic
	default:
		return segTypes.static
	}
}

func getIsStatic(segments []Segment) bool {
	if len(segments) > 0 {
		for _, segment := range segments {
			switch segment.SegType {
			case segTypes.splat:
				return false
			case segTypes.dynamic:
				return false
			}
		}
	}
	return true
}

type segmentNode struct {
	pattern     string
	nodeType    uint8
	children    map[string]*segmentNode
	dynChildren []*segmentNode
	paramName   string
	finalScore  int
}

func (sn *segmentNode) findOrCreateChild(seg string) *segmentNode {
	if len(seg) == 0 {
		if sn.children == nil {
			sn.children = make(map[string]*segmentNode)
		}
		if child, exists := sn.children[""]; exists {
			return child
		}
		child := &segmentNode{nodeType: nodeStatic}
		sn.children[""] = child
		return child
	}

	switch seg[0] {
	case ':':
		for _, child := range sn.dynChildren {
			if child.nodeType == nodeDynamic && child.paramName == seg[1:] {
				return child
			}
		}
		child := &segmentNode{nodeType: nodeDynamic, paramName: seg[1:]}
		sn.dynChildren = append(sn.dynChildren, child)
		return child
	case '*':
		for _, child := range sn.dynChildren {
			if child.nodeType == nodeSplat {
				return child
			}
		}
		child := &segmentNode{nodeType: nodeSplat}
		sn.dynChildren = append(sn.dynChildren, child)
		return child
	default:
		if sn.children == nil {
			sn.children = make(map[string]*segmentNode)
		}
		if child, exists := sn.children[seg]; exists {
			return child
		}
		child := &segmentNode{nodeType: nodeStatic}
		sn.children[seg] = child
		return child
	}
}

// ParseSegments splits a path into matcher segments.
func ParseSegments(path string) []string {
	if path == "" {
		return []string{}
	}
	if path == "/" {
		return []string{""}
	}

	startIdx := 0
	if path[0] == '/' {
		startIdx = 1
	}

	var maxSegments int
	for i := startIdx; i < len(path); i++ {
		if path[i] == '/' {
			maxSegments++
		}
	}
	if len(path) > 0 {
		maxSegments++
	}
	if maxSegments == 0 {
		return nil
	}

	segs := make([]string, 0, maxSegments)
	start := startIdx

	for i := startIdx; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				segs = append(segs, path[start:i])
			}
			start = i + 1
		}
	}

	if start < len(path) {
		segs = append(segs, path[start:])
	}

	if len(path) > 0 && path[len(path)-1] == '/' {
		segs = append(segs, "")
	}

	return segs
}

// FindBestMatch resolves the highest-scoring single match for realPath.
func (m *Matcher) FindBestMatch(realPath string) (*BestMatch, bool) {
	if rr, ok := m.staticPatterns[realPath]; ok {
		return &BestMatch{RegisteredPattern: rr}, true
	}

	segments := ParseSegments(realPath)
	hasTrailingSlash := len(realPath) > 0 && realPath[len(realPath)-1] == '/'

	if hasTrailingSlash {
		pathWithoutTrailingSlash := realPath[:len(realPath)-1]
		if rr, ok := m.staticPatterns[pathWithoutTrailingSlash]; ok {
			return &BestMatch{RegisteredPattern: rr}, true
		}
	}

	best := new(BestMatch)
	var bestScore uint16
	foundMatch := false

	m.dfsBest(
		m.rootNode,
		segments,
		0,
		0,
		best,
		&bestScore,
		&foundMatch,
		hasTrailingSlash,
	)

	if !foundMatch {
		return nil, false
	}

	if best.numberOfDynamicParamSegs > 0 {
		params := make(Params, best.numberOfDynamicParamSegs)
		for i, seg := range best.normalizedSegments {
			if seg.SegType == segTypes.dynamic {
				params[seg.NormalizedVal[1:]] = segments[i]
			}
		}
		best.Params = params
	}

	if best.normalizedPattern == "/*" || best.lastSegIsNonRootSplat {
		best.SplatValues = segments[len(best.normalizedSegments)-1:]
	}

	return best, true
}

func (m *Matcher) dfsBest(
	node *segmentNode,
	segments []string,
	depth int,
	score uint16,
	best *BestMatch,
	bestScore *uint16,
	foundMatch *bool,
	checkTrailingSlash bool,
) {
	atNormalEnd := checkTrailingSlash && depth == len(segments)-1

	if len(node.pattern) > 0 {
		if rp, ok := m.dynamicPatterns[node.pattern]; ok {
			if depth == len(segments) || node.nodeType == nodeSplat ||
				atNormalEnd {
				if !*foundMatch || score > *bestScore {
					best.RegisteredPattern = rp
					best.score = score
					*bestScore = score
					*foundMatch = true
				}
			}
		}
	}

	if depth >= len(segments) {
		return
	}

	if node.children != nil {
		if child, ok := node.children[segments[depth]]; ok {
			m.dfsBest(
				child,
				segments,
				depth+1,
				score+scoreStaticMatch,
				best,
				bestScore,
				foundMatch,
				checkTrailingSlash,
			)

			if *foundMatch && depth+1 == len(segments) && child.pattern != "" {
				return
			}
		}
	}

	for _, child := range node.dynChildren {
		switch child.nodeType {
		case nodeDynamic:
			if segments[depth] != "" {
				m.dfsBest(
					child,
					segments,
					depth+1,
					score+scoreDynamic,
					best,
					bestScore,
					foundMatch,
					checkTrailingSlash,
				)
			}
		case nodeSplat:
			if len(child.pattern) > 0 {
				if rp := m.dynamicPatterns[child.pattern]; rp != nil {
					if !*foundMatch {
						best.RegisteredPattern = rp
						*foundMatch = true
					}
				}
			}
		}
	}
}

// FindNestedMatches resolves all nested matches for realPath.
func (m *Matcher) FindNestedMatches(
	realPath string,
) (*FindNestedMatchesResults, bool) {
	realPath = StripTrailingSlash(realPath)

	realSegments := ParseSegments(realPath)
	realSegmentsLen := len(realSegments)
	matches := make(matchesMap)

	emptyRR, hasEmptyRR := m.staticPatterns[""]
	if hasEmptyRR {
		matches[emptyRR.normalizedPattern] = &Match{RegisteredPattern: emptyRR}
	}

	if realPath == "" {
		if rr, ok := m.staticPatterns["/"]; ok {
			matches[rr.normalizedPattern] = &Match{RegisteredPattern: rr}
		} else if rr, ok := m.dynamicPatterns["/*"]; ok {
			matches["/*"] = &Match{
				RegisteredPattern: rr,
				splatValues:       []string{},
			}
		}
		return flattenAndSortMatches(matches, realPath, realSegmentsLen)
	}

	var patternBuilder strings.Builder
	patternBuilder.Grow(len(realPath) + 1)
	var foundFullStatic bool
	for i := range realSegments {
		patternBuilder.WriteString("/")
		patternBuilder.WriteString(realSegments[i])
		if rr, ok := m.staticPatterns[patternBuilder.String()]; ok {
			matches[rr.normalizedPattern] = &Match{RegisteredPattern: rr}
			if i == realSegmentsLen-1 {
				foundFullStatic = true
			}
		}
		if i == realSegmentsLen-1 {
			patternBuilder.WriteString("/")
			if rr, ok := m.staticPatterns[patternBuilder.String()]; ok {
				matches[rr.normalizedPattern] = &Match{RegisteredPattern: rr}
			}
		}
	}

	if !foundFullStatic {
		if rr, ok := m.dynamicPatterns["/*"]; ok {
			matches["/*"] = &Match{
				RegisteredPattern: rr,
				splatValues:       realSegments,
			}
		}

		params := make(Params)
		m.dfsNestedMatches(m.rootNode, realSegments, 0, params, matches)
	}

	if _, ok := matches["/*"]; ok {
		if hasEmptyRR {
			if len(matches) > 2 {
				delete(matches, "/*")
			}
		} else if len(matches) > 1 {
			delete(matches, "/*")
		}
	}

	if len(matches) < 2 {
		return flattenAndSortMatches(matches, realPath, realSegmentsLen)
	}

	var longestSegmentLen int
	var longestIndexMatch *Match
	var longestDynamicMatch *Match
	var longestSplatMatch *Match
	for _, match := range matches {
		segmentsLen := len(match.normalizedSegments)
		if segmentsLen > longestSegmentLen {
			longestSegmentLen = segmentsLen
			longestIndexMatch = nil
			longestDynamicMatch = nil
			longestSplatMatch = nil
		}
		if segmentsLen == longestSegmentLen {
			switch match.lastSegType {
			case segTypes.index:
				longestIndexMatch = match
			case segTypes.dynamic:
				longestDynamicMatch = match
			case segTypes.splat:
				longestSplatMatch = match
			}
		}
	}

	for pattern, match := range matches {
		if len(match.normalizedSegments) < longestSegmentLen {
			if match.lastSegIsNonRootSplat || match.lastSegIsIndex {
				delete(matches, pattern)
			}
		}
	}

	if len(matches) < 2 {
		return flattenAndSortMatches(matches, realPath, realSegmentsLen)
	}

	longestSegmentTypeCount := 0
	if longestIndexMatch != nil {
		longestSegmentTypeCount++
	}
	if longestDynamicMatch != nil {
		longestSegmentTypeCount++
	}
	if longestSplatMatch != nil {
		longestSegmentTypeCount++
	}

	if longestSegmentTypeCount > 1 {
		if longestIndexMatch != nil {
			delete(matches, longestIndexMatch.normalizedPattern)
		}

		dynamicExists := longestDynamicMatch != nil
		splatExists := longestSplatMatch != nil

		if realSegmentsLen == longestSegmentLen && dynamicExists &&
			splatExists {
			delete(
				matches,
				longestSplatMatch.normalizedPattern,
			)
		}
		if realSegmentsLen > longestSegmentLen && splatExists && dynamicExists {
			delete(
				matches,
				longestDynamicMatch.normalizedPattern,
			)
		}
	}

	return flattenAndSortMatches(matches, realPath, realSegmentsLen)
}

func (m *Matcher) dfsNestedMatches(
	node *segmentNode,
	segments []string,
	depth int,
	params Params,
	matches matchesMap,
) {
	if len(node.pattern) > 0 {
		if rp := m.dynamicPatterns[node.pattern]; rp != nil {
			if node.pattern != "/*" {
				var paramsCopy Params
				if len(params) > 0 {
					paramsCopy = make(Params, len(params))
					maps.Copy(paramsCopy, params)
				}

				var splatValues []string
				if node.nodeType == nodeSplat && depth < len(segments) {
					splatValues = make([]string, len(segments)-depth)
					copy(splatValues, segments[depth:])
				}

				match := &Match{
					RegisteredPattern: rp,
					params:            paramsCopy,
					splatValues:       splatValues,
				}
				matches[node.pattern] = match

				if depth == len(segments) {
					indexPattern := node.pattern + "/"
					if rp, ok := m.dynamicPatterns[indexPattern]; ok {
						matches[indexPattern] = &Match{
							RegisteredPattern: rp,
							params:            paramsCopy,
						}
					}
				}
			}
		}
	}

	if depth >= len(segments) {
		return
	}

	seg := segments[depth]

	if node.children != nil {
		if child, ok := node.children[seg]; ok {
			m.dfsNestedMatches(child, segments, depth+1, params, matches)
		}
	}

	for _, child := range node.dynChildren {
		switch child.nodeType {
		case nodeDynamic:
			oldVal, hadVal := params[child.paramName]
			params[child.paramName] = seg

			m.dfsNestedMatches(child, segments, depth+1, params, matches)

			if hadVal {
				params[child.paramName] = oldVal
			} else {
				delete(params, child.paramName)
			}
		case nodeSplat:
			m.dfsNestedMatches(child, segments, depth, params, matches)
		}
	}
}

func flattenAndSortMatches(
	matches matchesMap,
	realPath string,
	realSegmentLen int,
) (*FindNestedMatchesResults, bool) {
	matchCount := len(matches)
	if matchCount == 0 {
		return nil, false
	}

	results := make([]*Match, 0, matchCount)
	for _, match := range matches {
		results = append(results, match)
	}

	if matchCount > 1 {
		slices.SortFunc(results, func(left, right *Match) int {
			if left.lastSegIsIndex != right.lastSegIsIndex {
				if left.lastSegIsIndex {
					return 1
				}
				return -1
			}

			lenDiff := len(left.normalizedSegments) - len(right.normalizedSegments)
			if lenDiff != 0 {
				return lenDiff
			}

			return strings.Compare(left.normalizedPattern, right.normalizedPattern)
		})
	}

	isNotSlashRoute := realPath != "" && realPath != "/"
	if isNotSlashRoute && len(results) == 1 &&
		results[0].normalizedPattern == "" {
		return nil, false
	}

	lastMatch := results[len(results)-1]

	if !lastMatch.lastSegIsNonRootSplat && lastMatch.normalizedPattern != "/*" {
		patternSegmentsLen := len(lastMatch.normalizedSegments)

		if patternSegmentsLen < realSegmentLen {
			return nil, false
		}

		if patternSegmentsLen == realSegmentLen &&
			lastMatch.numberOfDynamicParamSegs > 0 &&
			len(lastMatch.params) == 0 {
			return nil, false
		}
	}

	return &FindNestedMatchesResults{
		Params:      lastMatch.params,
		SplatValues: lastMatch.splatValues,
		Matches:     results,
	}, true
}
