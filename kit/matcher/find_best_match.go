package matcher

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

	m.dfsBest(m.rootNode, segments, 0, 0, best, &bestScore, &foundMatch, hasTrailingSlash)

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
			if depth == len(segments) || node.nodeType == nodeSplat || atNormalEnd {
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
			m.dfsBest(child, segments, depth+1, score+scoreStaticMatch, best, bestScore, foundMatch, checkTrailingSlash)

			if *foundMatch && depth+1 == len(segments) && child.pattern != "" {
				return
			}
		}
	}

	for _, child := range node.dynChildren {
		switch child.nodeType {
		case nodeDynamic:
			// Don't match empty segments to dynamic parameters
			if segments[depth] != "" {
				m.dfsBest(child, segments, depth+1, score+scoreDynamic, best, bestScore, foundMatch, checkTrailingSlash)
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
