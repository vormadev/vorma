// Package matcher provides route pattern matching with support for static,
// dynamic, and splat segments. It supports two resolution modes:
//
//   - FindBestMatch: resolves the single highest-scoring route (for traditional routing).
//   - FindNestedMatches: resolves all hierarchical ancestor matches (for loader/layout routing).
package matcher

import (
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC TYPES
/////////////////////////////////////////////////////////////////////

// Params stores dynamic path parameter values extracted from a match.
type Params = map[string]string

// Segment is a normalized route segment and its classification.
type Segment struct {
	NormalizedVal string
	Type          string // "static", "dynamic", "splat", "index"
}

// Options configures matching semantics.
type Options struct {
	DynamicParamPrefix             rune   // Defaults to ':'.
	SplatSegmentIdentifier         rune   // Defaults to '*'.
	ExplicitIndexSegmentIdentifier string // Defaults to "" (trailing slash = index).
	Quiet                          bool   // Suppress duplicate-registration warnings.
}

// Matcher stores registered patterns and resolves paths against them.
type Matcher struct {
	static_patterns  patterns_map
	dynamic_patterns patterns_map
	root_node        *segment_node

	explicit_index_segment       string
	dynamic_param_prefix         rune
	splat_segment_id             rune
	slash_index_segment          string
	using_explicit_index_segment bool
	quiet                        bool
}

// RegisteredPattern is the normalized metadata for one registered pattern.
type RegisteredPattern struct {
	original_pattern           string
	normalized_pattern         string
	normalized_segments        []segment
	last_seg_type              seg_type
	last_seg_is_non_root_splat bool
	last_seg_is_index          bool
	num_dynamic_param_segs     uint8
}

// BestMatch is the highest-scoring single match for a real path.
type BestMatch struct {
	*RegisteredPattern
	Params      Params
	SplatValues []string
	score       uint16
}

// NestedMatch represents one entry in a nested match result set.
type NestedMatch struct {
	*RegisteredPattern
	params       Params
	splat_values []string
}

// FindNestedMatchesResults stores all resolved nested matches.
type FindNestedMatchesResults struct {
	Params      Params
	SplatValues []string
	Matches     []*NestedMatch
}

/////////////////////////////////////////////////////////////////////
/////// CONSTRUCTOR
/////////////////////////////////////////////////////////////////////

// New constructs a Matcher with defaulted options.
func New(opts *Options) *Matcher {
	if opts == nil {
		opts = new(Options)
	}
	if strings.Contains(opts.ExplicitIndexSegmentIdentifier, "/") {
		panic("explicit index segment cannot contain a slash")
	}

	m := &Matcher{
		static_patterns:      make(patterns_map),
		dynamic_patterns:     make(patterns_map),
		root_node:            new(segment_node),
		dynamic_param_prefix: or_default(opts.DynamicParamPrefix, ':'),
		splat_segment_id: or_default(
			opts.SplatSegmentIdentifier,
			'*',
		),
		explicit_index_segment:       opts.ExplicitIndexSegmentIdentifier,
		using_explicit_index_segment: opts.ExplicitIndexSegmentIdentifier != "",
		quiet:                        opts.Quiet,
	}
	m.slash_index_segment = "/" + m.explicit_index_segment
	return m
}

/////////////////////////////////////////////////////////////////////
/////// PUBLIC MATCHER METHODS
/////////////////////////////////////////////////////////////////////

// ExplicitIndexSegmentIdentifier returns the configured explicit index marker.
func (m *Matcher) ExplicitIndexSegmentIdentifier() string { return m.explicit_index_segment }

// DynamicParamPrefix returns the configured dynamic parameter prefix rune.
func (m *Matcher) DynamicParamPrefix() rune { return m.dynamic_param_prefix }

// SplatSegmentIdentifier returns the configured splat segment identifier rune.
func (m *Matcher) SplatSegmentIdentifier() rune { return m.splat_segment_id }

// NormalizePattern validates and normalizes one input pattern.
func (m *Matcher) NormalizePattern(original string) *RegisteredPattern {
	normalized := original

	if m.using_explicit_index_segment {
		if strings.HasSuffix(normalized, "/") {
			if normalized != "/" {
				log.Panicf(
					"Error with pattern '%s'. With the exception of any absolute root pattern ('/'), "+
						"trailing slashes are not permitted when using an explicit index segment.",
					original,
				)
			}
			normalized = strings.TrimRight(normalized, "/")
		}
		if strings.HasSuffix(normalized, m.slash_index_segment) {
			normalized = strings.TrimSuffix(
				normalized,
				m.explicit_index_segment,
			)
		}
	}

	raw_segs := ParseSegments(normalized)
	segs := make([]segment, 0, len(raw_segs))
	var num_dynamic uint8

	for _, s := range raw_segs {
		val := s
		st := m.classify_segment(s)
		if st == seg_types.dynamic {
			num_dynamic++
			val = ":" + s[1:]
		}
		if st == seg_types.splat {
			val = "*"
		}
		segs = append(segs, segment{normalized_val: val, seg_type: st})
	}

	seg_len := len(segs)
	var last_type seg_type
	if seg_len > 0 {
		last_type = segs[seg_len-1].seg_type
	}

	var sb strings.Builder
	sb.WriteString("/")
	for i, seg := range segs {
		sb.WriteString(seg.normalized_val)
		if i < seg_len-1 {
			sb.WriteString("/")
		}
	}
	final := sb.String()
	if strings.HasSuffix(final, "/") && last_type != seg_types.index {
		final = strings.TrimRight(final, "/")
	}

	return &RegisteredPattern{
		original_pattern:           original,
		normalized_pattern:         final,
		normalized_segments:        segs,
		last_seg_type:              last_type,
		last_seg_is_non_root_splat: last_type == seg_types.splat && seg_len > 1,
		last_seg_is_index:          last_type == seg_types.index,
		num_dynamic_param_segs:     num_dynamic,
	}
}

// RegisterPattern registers one pattern into the matcher. Panics on
// normalized-pattern collision from different original strings. Returns
// the existing entry when the same original is registered twice.
func (m *Matcher) RegisterPattern(original string) *RegisteredPattern {
	rp := m.NormalizePattern(original)

	// Check for existing registration / collision.
	for _, store := range [2]patterns_map{m.static_patterns, m.dynamic_patterns} {
		if existing, ok := store[rp.normalized_pattern]; ok {
			if existing.original_pattern == original {
				return existing
			}
			panic(fmt.Sprintf(
				`normalized pattern collision: "%s" and "%s" both normalize to "%s"`,
				original,
				existing.original_pattern,
				rp.normalized_pattern,
			))
		}
		if !is_static(rp.normalized_segments) {
			shape_key := rp.shape_key()
			for _, existing := range store {
				if existing.shape_key() != shape_key {
					continue
				}
				panic(fmt.Sprintf(
					`route shape collision: "%s" and "%s" both match the same paths`,
					original,
					existing.original_pattern,
				))
			}
		}
	}

	if is_static(rp.normalized_segments) {
		m.static_patterns[rp.normalized_pattern] = rp
		return rp
	}

	m.dynamic_patterns[rp.normalized_pattern] = rp

	current := m.root_node
	var node_score int
	for i, seg := range rp.normalized_segments {
		child := current.find_or_create_child(seg.normalized_val)
		switch {
		case seg.seg_type == seg_types.dynamic:
			node_score += score_dynamic
		case seg.seg_type != seg_types.splat:
			node_score += score_static
		}
		if i == len(rp.normalized_segments)-1 {
			child.pattern = rp.normalized_pattern
			child.final_score = node_score
		}
		current = child
	}

	return rp
}

// FindBestMatch resolves the single highest-scoring match for a real path.
func (m *Matcher) FindBestMatch(real_path string) (*BestMatch, bool) {
	if rr, ok := m.static_patterns[real_path]; ok {
		return &BestMatch{RegisteredPattern: rr}, true
	}

	segments := ParseSegments(real_path)
	has_trailing := len(real_path) > 0 && real_path[len(real_path)-1] == '/'

	if has_trailing {
		if rr, ok := m.static_patterns[real_path[:len(real_path)-1]]; ok {
			return &BestMatch{RegisteredPattern: rr}, true
		}
	}

	best := new(BestMatch)
	found := false

	m.dfs_best(
		m.root_node,
		segments,
		0,
		0,
		best,
		&found,
		has_trailing,
	)

	if !found {
		return nil, false
	}

	if best.num_dynamic_param_segs > 0 {
		params := make(Params, best.num_dynamic_param_segs)
		for i, seg := range best.normalized_segments {
			if seg.seg_type == seg_types.dynamic {
				params[seg.normalized_val[1:]] = segments[i]
			}
		}
		best.Params = params
	}

	if best.normalized_pattern == "/*" || best.last_seg_is_non_root_splat {
		best.SplatValues = segments[len(best.normalized_segments)-1:]
	}

	return best, true
}

// FindNestedMatches resolves all hierarchical ancestor matches for a real path.
func (m *Matcher) FindNestedMatches(
	real_path string,
) (*FindNestedMatchesResults, bool) {
	real_path = StripTrailingSlash(real_path)
	real_segs := ParseSegments(real_path)
	real_segs_len := len(real_segs)
	matches := make(matches_map)

	empty_rr, has_empty := m.static_patterns[""]
	if has_empty {
		matches[empty_rr.normalized_pattern] = &NestedMatch{
			RegisteredPattern: empty_rr,
		}
	}

	if real_path == "" {
		if rr, ok := m.static_patterns["/"]; ok {
			matches[rr.normalized_pattern] = &NestedMatch{RegisteredPattern: rr}
		} else if rr, ok := m.dynamic_patterns["/*"]; ok {
			matches["/*"] = &NestedMatch{RegisteredPattern: rr, splat_values: []string{}}
		}
		return flatten_and_sort(matches, real_path, real_segs_len)
	}

	var pb strings.Builder
	pb.Grow(len(real_path) + 1)
	var found_full_static bool
	for i := range real_segs {
		pb.WriteString("/")
		pb.WriteString(real_segs[i])
		if rr, ok := m.static_patterns[pb.String()]; ok {
			matches[rr.normalized_pattern] = &NestedMatch{RegisteredPattern: rr}
			if i == real_segs_len-1 {
				found_full_static = true
			}
		}
		if i == real_segs_len-1 {
			pb.WriteString("/")
			if rr, ok := m.static_patterns[pb.String()]; ok {
				matches[rr.normalized_pattern] = &NestedMatch{
					RegisteredPattern: rr,
				}
			}
		}
	}

	if !found_full_static {
		if rr, ok := m.dynamic_patterns["/*"]; ok {
			matches["/*"] = &NestedMatch{
				RegisteredPattern: rr,
				splat_values:      real_segs,
			}
		}
		params := make(Params)
		m.dfs_nested(m.root_node, real_segs, 0, params, matches)
	}

	// Prune catch-all when better matches exist.
	if _, ok := matches["/*"]; ok {
		if has_empty {
			if len(matches) > 2 {
				delete(matches, "/*")
			}
		} else if len(matches) > 1 {
			delete(matches, "/*")
		}
	}

	if len(matches) < 2 {
		return flatten_and_sort(matches, real_path, real_segs_len)
	}

	// Track the three possible longest-segment-length match types.
	var longest_len int
	var longest_index, longest_dynamic, longest_splat *NestedMatch
	for _, match := range matches {
		seg_len := len(match.normalized_segments)
		if seg_len > longest_len {
			longest_len = seg_len
			longest_index, longest_dynamic, longest_splat = nil, nil, nil
		}
		if seg_len == longest_len {
			switch match.last_seg_type {
			case seg_types.index:
				longest_index = match
			case seg_types.dynamic:
				longest_dynamic = match
			case seg_types.splat:
				longest_splat = match
			}
		}
	}

	// Remove shorter splats and indexes.
	for pat, match := range matches {
		if len(match.normalized_segments) < longest_len {
			if match.last_seg_is_non_root_splat || match.last_seg_is_index {
				delete(matches, pat)
			}
		}
	}

	if len(matches) < 2 {
		return flatten_and_sort(matches, real_path, real_segs_len)
	}

	// Disambiguate longest-segment matches.
	type_count := 0
	if longest_index != nil {
		type_count++
	}
	if longest_dynamic != nil {
		type_count++
	}
	if longest_splat != nil {
		type_count++
	}
	if type_count > 1 {
		if longest_index != nil {
			delete(matches, longest_index.normalized_pattern)
		}
		has_dyn := longest_dynamic != nil
		has_spl := longest_splat != nil
		if real_segs_len == longest_len && has_dyn && has_spl {
			for pat, match := range matches {
				if len(match.normalized_segments) == longest_len &&
					match.last_seg_type == seg_types.splat {
					delete(matches, pat)
				}
			}
		}
		if real_segs_len > longest_len && has_spl && has_dyn {
			for pat, match := range matches {
				if len(match.normalized_segments) == longest_len &&
					match.last_seg_type == seg_types.dynamic {
					delete(matches, pat)
				}
			}
		}
	}

	return flatten_and_sort(matches, real_path, real_segs_len)
}

/////////////////////////////////////////////////////////////////////
/////// PUBLIC REGISTERED-PATTERN METHODS
/////////////////////////////////////////////////////////////////////

// OriginalPattern returns the pattern as originally registered.
func (rp *RegisteredPattern) OriginalPattern() string { return rp.original_pattern }

// NormalizedPattern returns the normalized pattern string.
func (rp *RegisteredPattern) NormalizedPattern() string { return rp.normalized_pattern }

// NormalizedSegments returns a copy of the normalized segment list.
func (rp *RegisteredPattern) NormalizedSegments() []Segment {
	out := make([]Segment, len(rp.normalized_segments))
	for i, s := range rp.normalized_segments {
		out[i] = Segment{
			NormalizedVal: s.normalized_val,
			Type:          string(s.seg_type),
		}
	}
	return out
}

func (rp *RegisteredPattern) shape_key() string {
	var sb strings.Builder
	for i, seg := range rp.normalized_segments {
		if i > 0 {
			sb.WriteString("/")
		}
		switch seg.seg_type {
		case seg_types.dynamic:
			sb.WriteString("D:")
		case seg_types.splat:
			sb.WriteString("P:")
		case seg_types.index:
			sb.WriteString("I:")
		default:
			sb.WriteString("S:")
			sb.WriteString(seg.normalized_val)
		}
	}
	return sb.String()
}

/////////////////////////////////////////////////////////////////////
/////// PUBLIC MATCH METHODS
/////////////////////////////////////////////////////////////////////

// Params returns a defensive copy of dynamic params for this match.
func (m *NestedMatch) Params() Params {
	if m == nil || len(m.params) == 0 {
		return nil
	}
	out := make(Params, len(m.params))
	maps.Copy(out, m.params)
	return out
}

// SplatValues returns a defensive copy of splat values for this match.
func (m *NestedMatch) SplatValues() []string {
	if m == nil || len(m.splat_values) == 0 {
		return nil
	}
	out := make([]string, len(m.splat_values))
	copy(out, m.splat_values)
	return out
}

/////////////////////////////////////////////////////////////////////
/////// PUBLIC UTILITY FUNCTIONS
/////////////////////////////////////////////////////////////////////

// ParseSegments splits a URL path into route segments.
func ParseSegments(path string) []string {
	if path == "" {
		return []string{}
	}
	if path == "/" {
		return []string{""}
	}

	start_idx := 0
	if path[0] == '/' {
		start_idx = 1
	}

	var max_segs int
	for i := start_idx; i < len(path); i++ {
		if path[i] == '/' {
			max_segs++
		}
	}
	if len(path) > 0 {
		max_segs++
	}
	if max_segs == 0 {
		return nil
	}

	segs := make([]string, 0, max_segs)
	start := start_idx
	for i := start_idx; i < len(path); i++ {
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
	if path[len(path)-1] == '/' {
		segs = append(segs, "")
	}
	return segs
}

func HasLeadingSlash(p string) bool  { return len(p) > 0 && p[0] == '/' }
func HasTrailingSlash(p string) bool { return len(p) > 0 && p[len(p)-1] == '/' }
func EnsureLeadingSlash(p string) string {
	if !HasLeadingSlash(p) {
		return "/" + p
	}
	return p
}
func EnsureTrailingSlash(p string) string {
	if !HasTrailingSlash(p) {
		return p + "/"
	}
	return p
}
func StripLeadingSlash(p string) string {
	if HasLeadingSlash(p) {
		return p[1:]
	}
	return p
}
func StripTrailingSlash(p string) string {
	if HasTrailingSlash(p) {
		return p[:len(p)-1]
	}
	return p
}

// EnsureLeadingAndTrailingSlash ensures both leading and trailing slashes.
func EnsureLeadingAndTrailingSlash(p string) string {
	return EnsureLeadingSlash(EnsureTrailingSlash(p))
}

// JoinPatterns joins a registered pattern base with a suffix pattern.
func JoinPatterns(rp *RegisteredPattern, pattern string) string {
	var sb strings.Builder
	base := rp.normalized_pattern
	sb.WriteString(base)
	has_lead := HasLeadingSlash(pattern)
	if HasTrailingSlash(base) && has_lead {
		pattern = pattern[1:]
	} else if !has_lead {
		sb.WriteString("/")
	}
	sb.WriteString(pattern)
	return sb.String()
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE TYPES AND CONSTANTS
/////////////////////////////////////////////////////////////////////

type (
	seg_type     = string
	patterns_map = map[string]*RegisteredPattern
	matches_map  = map[string]*NestedMatch
)

type segment struct {
	normalized_val string
	seg_type       seg_type
}

func (s segment) best_match_rank() uint16 {
	switch s.seg_type {
	case seg_types.static, seg_types.index:
		return score_static
	case seg_types.dynamic:
		return score_dynamic
	default:
		return 0
	}
}

var seg_types = struct {
	splat   seg_type
	static  seg_type
	dynamic seg_type
	index   seg_type
}{
	splat:   "splat",
	static:  "static",
	dynamic: "dynamic",
	index:   "index",
}

const (
	node_static   uint8 = 0
	node_dynamic  uint8 = 1
	node_splat    uint8 = 2
	score_static        = 2
	score_dynamic       = 1
)

type segment_node struct {
	pattern      string
	node_type    uint8
	children     map[string]*segment_node
	dyn_children []*segment_node
	param_name   string
	final_score  int
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE HELPERS
/////////////////////////////////////////////////////////////////////

func or_default[T comparable](val, fallback T) T {
	var zero T
	if val == zero {
		return fallback
	}
	return val
}

func (m *Matcher) classify_segment(seg string) seg_type {
	switch {
	case seg == "":
		return seg_types.index
	case len(seg) == 1 && seg == string(m.splat_segment_id):
		return seg_types.splat
	case len(seg) > 0 && seg[0] == byte(m.dynamic_param_prefix):
		return seg_types.dynamic
	default:
		return seg_types.static
	}
}

func (m *BestMatch) better_than(other *BestMatch) bool {
	if other == nil || other.RegisteredPattern == nil {
		return true
	}
	if m.score != other.score {
		return m.score > other.score
	}
	for i := range min(len(m.normalized_segments), len(other.normalized_segments)) {
		left := m.normalized_segments[i].best_match_rank()
		right := other.normalized_segments[i].best_match_rank()
		if left != right {
			return left > right
		}
	}
	if m.last_seg_type != other.last_seg_type {
		if m.last_seg_type == seg_types.splat {
			return false
		}
		if other.last_seg_type == seg_types.splat {
			return true
		}
	}
	if len(m.normalized_segments) != len(other.normalized_segments) {
		return len(m.normalized_segments) > len(other.normalized_segments)
	}
	return false
}

func is_static(segs []segment) bool {
	for _, s := range segs {
		if s.seg_type == seg_types.splat || s.seg_type == seg_types.dynamic {
			return false
		}
	}
	return true
}

/////////////////////////////////////////////////////////////////////
/////// SEGMENT NODE TREE
/////////////////////////////////////////////////////////////////////

func (n *segment_node) find_or_create_child(seg string) *segment_node {
	if len(seg) == 0 {
		if n.children == nil {
			n.children = make(map[string]*segment_node)
		}
		if child, ok := n.children[""]; ok {
			return child
		}
		child := &segment_node{node_type: node_static}
		n.children[""] = child
		return child
	}

	switch seg[0] {
	case ':':
		for _, child := range n.dyn_children {
			if child.node_type == node_dynamic && child.param_name == seg[1:] {
				return child
			}
		}
		child := &segment_node{node_type: node_dynamic, param_name: seg[1:]}
		n.dyn_children = append(n.dyn_children, child)
		return child
	case '*':
		for _, child := range n.dyn_children {
			if child.node_type == node_splat {
				return child
			}
		}
		child := &segment_node{node_type: node_splat}
		n.dyn_children = append(n.dyn_children, child)
		return child
	default:
		if n.children == nil {
			n.children = make(map[string]*segment_node)
		}
		if child, ok := n.children[seg]; ok {
			return child
		}
		child := &segment_node{node_type: node_static}
		n.children[seg] = child
		return child
	}
}

/////////////////////////////////////////////////////////////////////
/////// DFS: BEST MATCH
/////////////////////////////////////////////////////////////////////

func (m *Matcher) dfs_best(
	node *segment_node,
	segments []string,
	depth int,
	score uint16,
	best *BestMatch,
	found *bool,
	check_trailing bool,
) {
	at_normal_end := check_trailing && depth == len(segments)-1

	if len(node.pattern) > 0 {
		if rp, ok := m.dynamic_patterns[node.pattern]; ok {
			if depth == len(segments) || node.node_type == node_splat ||
				at_normal_end {
				candidate := BestMatch{RegisteredPattern: rp, score: score}
				if !*found || candidate.better_than(best) {
					*best = candidate
					*found = true
				}
			}
		}
	}

	if depth >= len(segments) {
		return
	}

	if node.children != nil {
		if child, ok := node.children[segments[depth]]; ok {
			m.dfs_best(
				child,
				segments,
				depth+1,
				score+score_static,
				best,
				found,
				check_trailing,
			)
			if *found && depth+1 == len(segments) && child.pattern != "" {
				return
			}
		}
	}

	for _, child := range node.dyn_children {
		switch child.node_type {
		case node_dynamic:
			if segments[depth] != "" {
				m.dfs_best(
					child,
					segments,
					depth+1,
					score+score_dynamic,
					best,
					found,
					check_trailing,
				)
			}
		case node_splat:
			if len(child.pattern) > 0 {
				if rp := m.dynamic_patterns[child.pattern]; rp != nil {
					candidate := BestMatch{RegisteredPattern: rp, score: score}
					if !*found || candidate.better_than(best) {
						*best = candidate
						*found = true
					}
				}
			}
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// DFS: NESTED MATCHES
/////////////////////////////////////////////////////////////////////

func (m *Matcher) dfs_nested(
	node *segment_node,
	segments []string,
	depth int,
	params Params,
	matches matches_map,
) {
	if len(node.pattern) > 0 {
		if rp := m.dynamic_patterns[node.pattern]; rp != nil {
			if node.pattern != "/*" {
				var params_copy Params
				if len(params) > 0 {
					params_copy = make(Params, len(params))
					maps.Copy(params_copy, params)
				}

				var sv []string
				if node.node_type == node_splat && depth < len(segments) {
					sv = make([]string, len(segments)-depth)
					copy(sv, segments[depth:])
				}

				matches[node.pattern] = &NestedMatch{
					RegisteredPattern: rp,
					params:            params_copy,
					splat_values:      sv,
				}

				if depth == len(segments) {
					idx_pattern := node.pattern + "/"
					if irp, ok := m.dynamic_patterns[idx_pattern]; ok {
						matches[idx_pattern] = &NestedMatch{
							RegisteredPattern: irp,
							params:            params_copy,
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
			m.dfs_nested(child, segments, depth+1, params, matches)
		}
	}

	for _, child := range node.dyn_children {
		switch child.node_type {
		case node_dynamic:
			old_val, had := params[child.param_name]
			params[child.param_name] = seg
			m.dfs_nested(child, segments, depth+1, params, matches)
			if had {
				params[child.param_name] = old_val
			} else {
				delete(params, child.param_name)
			}
		case node_splat:
			m.dfs_nested(child, segments, depth, params, matches)
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// FLATTEN AND SORT NESTED MATCHES
/////////////////////////////////////////////////////////////////////

func flatten_and_sort(
	matches matches_map,
	real_path string,
	real_seg_len int,
) (*FindNestedMatchesResults, bool) {
	count := len(matches)
	if count == 0 {
		return nil, false
	}

	results := make([]*NestedMatch, 0, count)
	for _, match := range matches {
		results = append(results, match)
	}

	if count > 1 {
		slices.SortFunc(results, func(a, b *NestedMatch) int {
			if a.last_seg_is_index != b.last_seg_is_index {
				if a.last_seg_is_index {
					return 1
				}
				return -1
			}
			if d := len(a.normalized_segments) - len(b.normalized_segments); d != 0 {
				return d
			}
			return strings.Compare(a.normalized_pattern, b.normalized_pattern)
		})
	}

	is_not_slash := real_path != "" && real_path != "/"
	if is_not_slash && len(results) == 1 &&
		results[0].normalized_pattern == "" {
		return nil, false
	}

	last := results[len(results)-1]

	if !last.last_seg_is_non_root_splat && last.normalized_pattern != "/*" {
		pat_seg_len := len(last.normalized_segments)
		if pat_seg_len < real_seg_len {
			return nil, false
		}
		if pat_seg_len == real_seg_len && last.num_dynamic_param_segs > 0 &&
			len(last.params) == 0 {
			return nil, false
		}
	}

	return &FindNestedMatchesResults{
		Params:      last.params,
		SplatValues: last.splat_values,
		Matches:     results,
	}, true
}
