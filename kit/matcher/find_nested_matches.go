package matcher

import (
	"maps"
	"slices"
	"strings"
)

type FindNestedMatchesResults struct {
	Params      Params
	SplatValues []string
	Matches     []*Match
}

func (m *Matcher) FindNestedMatches(realPath string) (*FindNestedMatchesResults, bool) {
	realPath = StripTrailingSlash(realPath)

	realSegments := ParseSegments(realPath)
	matches := make(matchesMap)

	emptyRR, hasEmptyRR := m.staticPatterns[""]

	if hasEmptyRR {
		matches[emptyRR.normalizedPattern] = &Match{RegisteredPattern: emptyRR}
	}

	realSegmentsLen := len(realSegments)

	if realPath == "" {
		if rr, ok := m.staticPatterns["/"]; ok {
			matches[rr.normalizedPattern] = &Match{RegisteredPattern: rr}
		} else {
			if rr, ok := m.dynamicPatterns["/*"]; ok {
				matches["/*"] = &Match{
					RegisteredPattern: rr,
					splatValues:       []string{},
				}
			}
		}
		return flattenAndSortMatches(matches, realPath, realSegmentsLen)
	}

	var pb strings.Builder
	pb.Grow(len(realPath) + 1)
	var foundFullStatic bool
	for i := range realSegments {
		pb.WriteString("/")
		pb.WriteString(realSegments[i])
		if rr, ok := m.staticPatterns[pb.String()]; ok {
			matches[rr.normalizedPattern] = &Match{RegisteredPattern: rr}
			if i == realSegmentsLen-1 {
				foundFullStatic = true
			}
		}
		if i == realSegmentsLen-1 {
			pb.WriteString("/")
			if rr, ok := m.staticPatterns[pb.String()]; ok {
				matches[rr.normalizedPattern] = &Match{RegisteredPattern: rr}
			}
		}
	}

	if !foundFullStatic {
		// For the catch-all pattern (e.g., "/*"), handle it specially
		if rr, ok := m.dynamicPatterns["/*"]; ok {
			matches["/*"] = &Match{
				RegisteredPattern: rr,
				splatValues:       realSegments,
			}
		}

		// DFS for the rest of the matches
		params := make(Params)
		m.dfsNestedMatches(m.rootNode, realSegments, 0, params, matches)
	}

	// if there are multiple matches and a catch-all, remove the catch-all
	// UNLESS the sole other match is an empty str pattern
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
	longestSegmentMatches := make(matchesMap)
	for _, match := range matches {
		if len(match.normalizedSegments) > longestSegmentLen {
			longestSegmentLen = len(match.normalizedSegments)
		}
	}
	for _, match := range matches {
		if len(match.normalizedSegments) == longestSegmentLen {
			longestSegmentMatches[match.lastSegType] = match
		}
	}

	// if there is any splat or index with a segment length shorter than longest segment length, remove it
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

	// if the longest segment length items are (1) dynamic, (2) splat, or (3) index, remove them as follows:
	// - if the realSegmentLen equals the longest segment length, prioritize dynamic, then splat, and always remove index
	// - if the realSegmentLen is greater than the longest segment length, prioritize splat, and always remove dynamic and index
	if len(longestSegmentMatches) > 1 {
		if match, indexExists := longestSegmentMatches[segTypes.index]; indexExists {
			delete(matches, match.normalizedPattern)
		}

		_, dynamicExists := longestSegmentMatches[segTypes.dynamic]
		_, splatExists := longestSegmentMatches[segTypes.splat]

		if realSegmentsLen == longestSegmentLen && dynamicExists && splatExists {
			delete(matches, longestSegmentMatches[segTypes.splat].normalizedPattern)
		}
		if realSegmentsLen > longestSegmentLen && splatExists && dynamicExists {
			delete(matches, longestSegmentMatches[segTypes.dynamic].normalizedPattern)
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
			// Don't process the ultimate catch-all here
			if node.pattern != "/*" {
				// Copy params
				paramsCopy := make(Params, len(params))
				maps.Copy(paramsCopy, params)

				var splatValues []string
				if node.nodeType == nodeSplat && depth < len(segments) {
					// For splat nodes, collect all remaining segments
					splatValues = make([]string, len(segments)-depth)
					copy(splatValues, segments[depth:])
				}

				match := &Match{
					RegisteredPattern: rp,
					params:            paramsCopy,
					splatValues:       splatValues,
				}
				matches[node.pattern] = match

				// Check for index segment if we're at the exact depth
				if depth == len(segments) {
					var sb strings.Builder
					sb.Grow(len(node.pattern) + 1)
					sb.WriteString(node.pattern)
					sb.WriteByte('/')
					indexPattern := sb.String()
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

	// If we've consumed all segments, stop
	if depth >= len(segments) {
		return
	}

	seg := segments[depth]

	// Try static children
	if node.children != nil {
		if child, ok := node.children[seg]; ok {
			m.dfsNestedMatches(child, segments, depth+1, params, matches)
		}
	}

	// Try dynamic/splat children
	for _, child := range node.dynChildren {
		switch child.nodeType {
		case nodeDynamic:
			// Backtracking pattern for dynamic
			oldVal, hadVal := params[child.paramName]
			params[child.paramName] = seg

			m.dfsNestedMatches(child, segments, depth+1, params, matches)

			if hadVal {
				params[child.paramName] = oldVal
			} else {
				delete(params, child.paramName)
			}

		case nodeSplat:
			// For splat nodes, we collect remaining segments and don't increment depth
			m.dfsNestedMatches(child, segments, depth, params, matches)
		}
	}
}

func flattenAndSortMatches(
	matches matchesMap,
	realPath string,
	realSegmentLen int,
) (*FindNestedMatchesResults, bool) {
	var results []*Match
	for _, match := range matches {
		results = append(results, match)
	}

	slices.SortStableFunc(results, func(i, j *Match) int {
		// if any match is an index, it should be last
		if i.lastSegIsIndex {
			return 1
		}
		if j.lastSegIsIndex {
			return -1
		}

		// else sort by segment length
		lenDiff := len(i.normalizedSegments) - len(j.normalizedSegments)
		if lenDiff != 0 {
			return lenDiff
		}

		// Tiebreaker for determinism when segment lengths are equal
		return strings.Compare(i.normalizedPattern, j.normalizedPattern)
	})

	if len(results) == 0 {
		return nil, false
	}

	// if not slash route and solely matched "", then invalid
	isNotSlashRoute := realPath != "" && realPath != "/"
	if isNotSlashRoute && len(results) == 1 && results[0].normalizedPattern == "" {
		return nil, false
	}

	lastMatch := results[len(results)-1]

	// Check if the last match can consume all segments.
	if !lastMatch.lastSegIsNonRootSplat && lastMatch.normalizedPattern != "/*" {
		// For non-splat patterns, check if pattern depth matches
		// real segment count.
		// Dynamic patterns should have filled params if they matched.
		patternSegmentsLen := len(lastMatch.normalizedSegments)

		// If pattern has fewer segments than the real path and it's
		// not a splat, it's a partial match.
		if patternSegmentsLen < realSegmentLen {
			return nil, false
		}

		// If it's a dynamic pattern at the right depth, it should
		// have params.
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
