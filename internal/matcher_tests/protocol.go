package main

import (
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/vormadev/vorma/kit/matcher"
)

const (
	op_parse_segments      = "parseSegments"
	op_matcher_config      = "matcherConfig"
	op_normalize_pattern   = "normalizePattern"
	op_register_pattern    = "registerPattern"
	op_find_best_match     = "findBestMatch"
	op_find_nested_matches = "findNestedMatches"
	op_path_helpers        = "pathHelpers"
	op_join_patterns       = "joinPatterns"
)

type protocol_options struct {
	DynamicParamPrefix             string `json:"dynamicParamPrefix,omitempty"`
	SplatSegmentIdentifier         string `json:"splatSegmentIdentifier,omitempty"`
	ExplicitIndexSegmentIdentifier string `json:"explicitIndexSegmentIdentifier,omitempty"`
	Quiet                          bool   `json:"quiet,omitempty"`
}

type protocol_request struct {
	Operation string           `json:"operation"`
	Options   protocol_options `json:"options"`
	Patterns  []string         `json:"patterns,omitempty"`
	Pattern   string           `json:"pattern,omitempty"`
	Path      string           `json:"path,omitempty"`
	Suffix    string           `json:"suffix,omitempty"`
}

type protocol_response[T any] struct {
	OK  bool   `json:"ok"`
	Err string `json:"err,omitempty"`
	Val T      `json:"val,omitempty"`
}

type segment_snapshot struct {
	NormalizedVal string `json:"normalizedVal"`
	SegType       string `json:"segType"`
}

type pattern_snapshot struct {
	OriginalPattern    string             `json:"originalPattern"`
	NormalizedPattern  string             `json:"normalizedPattern"`
	NormalizedSegments []segment_snapshot `json:"normalizedSegments"`
}

type matcher_config_snapshot struct {
	DynamicParamPrefix             string `json:"dynamicParamPrefix"`
	SplatSegmentIdentifier         string `json:"splatSegmentIdentifier"`
	ExplicitIndexSegmentIdentifier string `json:"explicitIndexSegmentIdentifier"`
}

type path_helpers_snapshot struct {
	HasLeadingSlash               bool   `json:"hasLeadingSlash"`
	HasTrailingSlash              bool   `json:"hasTrailingSlash"`
	EnsureLeadingSlash            string `json:"ensureLeadingSlash"`
	EnsureTrailingSlash           string `json:"ensureTrailingSlash"`
	EnsureLeadingAndTrailingSlash string `json:"ensureLeadingAndTrailingSlash"`
	StripLeadingSlash             string `json:"stripLeadingSlash"`
	StripTrailingSlash            string `json:"stripTrailingSlash"`
}

type best_match_snapshot struct {
	Found             bool              `json:"found"`
	RegisteredPattern *pattern_snapshot `json:"registeredPattern,omitempty"`
	Params            matcher.Params    `json:"params,omitempty"`
	SplatValues       []string          `json:"splatValues,omitempty"`
}

type nested_match_snapshot struct {
	RegisteredPattern *pattern_snapshot `json:"registeredPattern,omitempty"`
	Params            matcher.Params    `json:"params,omitempty"`
	SplatValues       []string          `json:"splatValues,omitempty"`
}

type nested_matches_snapshot struct {
	Found       bool                    `json:"found"`
	Params      matcher.Params          `json:"params,omitempty"`
	SplatValues []string                `json:"splatValues,omitempty"`
	Matches     []nested_match_snapshot `json:"matches,omitempty"`
}

func run_protocol(in io.Reader, out io.Writer) int {
	decoder := json.NewDecoder(in)
	for {
		var req protocol_request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return 0
			}
			write_response(out, protocol_response[any]{
				OK:  false,
				Err: err.Error(),
			})
			return 0
		}

		value, err := handle_protocol_request(req)
		if err != nil {
			write_response(out, protocol_response[any]{
				OK:  false,
				Err: err.Error(),
			})
			continue
		}

		write_response(out, protocol_response[any]{OK: true, Val: value})
	}
}

func handle_protocol_request(req protocol_request) (any, error) {
	if req.Operation == op_parse_segments {
		return matcher.ParseSegments(req.Path), nil
	}
	if req.Operation == op_path_helpers {
		return path_helpers_snapshot{
			HasLeadingSlash:               matcher.HasLeadingSlash(req.Path),
			HasTrailingSlash:              matcher.HasTrailingSlash(req.Path),
			EnsureLeadingSlash:            matcher.EnsureLeadingSlash(req.Path),
			EnsureTrailingSlash:           matcher.EnsureTrailingSlash(req.Path),
			EnsureLeadingAndTrailingSlash: matcher.EnsureLeadingAndTrailingSlash(req.Path),
			StripLeadingSlash:             matcher.StripLeadingSlash(req.Path),
			StripTrailingSlash:            matcher.StripTrailingSlash(req.Path),
		}, nil
	}

	m, err := matcher.New(req.Options.matcher_options())
	if err != nil {
		return nil, err
	}

	switch req.Operation {
	case op_matcher_config:
		return matcher_config_snapshot{
			DynamicParamPrefix:             string(m.DynamicParamPrefix()),
			SplatSegmentIdentifier:         string(m.SplatSegmentIdentifier()),
			ExplicitIndexSegmentIdentifier: m.ExplicitIndexSegmentIdentifier(),
		}, nil
	case op_normalize_pattern:
		rp, err := m.NormalizePattern(req.Pattern)
		if err != nil {
			return nil, err
		}
		return snapshot_pattern(rp), nil
	case op_join_patterns:
		rp, err := m.NormalizePattern(req.Pattern)
		if err != nil {
			return nil, err
		}
		return matcher.JoinPatterns(rp, req.Suffix), nil
	case op_register_pattern:
		return register_patterns(m, req.patterns_for_registration())
	case op_find_best_match:
		if _, err := register_patterns(m, req.Patterns); err != nil {
			return nil, err
		}
		return snapshot_best_match(m.FindBestMatch(req.Path)), nil
	case op_find_nested_matches:
		if _, err := register_patterns(m, req.Patterns); err != nil {
			return nil, err
		}
		return snapshot_nested_matches(m.FindNestedMatches(req.Path)), nil
	default:
		return nil, fmt.Errorf("unknown matcher conformance operation %q", req.Operation)
	}
}

func snapshot_pattern(rp *matcher.RegisteredPattern) *pattern_snapshot {
	if rp == nil {
		return nil
	}
	segments := rp.NormalizedSegments()
	out := make([]segment_snapshot, len(segments))
	for i, segment := range segments {
		out[i] = segment_snapshot{
			NormalizedVal: segment.NormalizedVal,
			SegType:       segment.Type,
		}
	}
	return &pattern_snapshot{
		OriginalPattern:    rp.OriginalPattern(),
		NormalizedPattern:  rp.NormalizedPattern(),
		NormalizedSegments: out,
	}
}

func (opts protocol_options) matcher_options() *matcher.Options {
	return &matcher.Options{
		DynamicParamPrefix:             first_rune(opts.DynamicParamPrefix),
		SplatSegmentIdentifier:         first_rune(opts.SplatSegmentIdentifier),
		ExplicitIndexSegmentIdentifier: opts.ExplicitIndexSegmentIdentifier,
		Quiet:                          opts.Quiet,
	}
}

func first_rune(s string) rune {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

func (req protocol_request) patterns_for_registration() []string {
	if len(req.Patterns) > 0 || req.Pattern == "" {
		return req.Patterns
	}
	return []string{req.Pattern}
}

func register_patterns(
	m *matcher.Matcher,
	patterns []string,
) (*pattern_snapshot, error) {
	var last *matcher.RegisteredPattern
	for _, pattern := range patterns {
		rp, err := m.RegisterPattern(pattern)
		if err != nil {
			return nil, err
		}
		last = rp
	}
	return snapshot_pattern(last), nil
}

func snapshot_best_match(
	match *matcher.BestMatch,
	found bool,
) best_match_snapshot {
	if !found || match == nil {
		return best_match_snapshot{}
	}
	return best_match_snapshot{
		Found:             true,
		RegisteredPattern: snapshot_pattern(match.RegisteredPattern),
		Params:            normalize_params(match.Params),
		SplatValues:       normalize_splat(match.SplatValues),
	}
}

func snapshot_nested_matches(
	results *matcher.FindNestedMatchesResults,
	found bool,
) nested_matches_snapshot {
	if !found || results == nil {
		return nested_matches_snapshot{}
	}
	matches := make([]nested_match_snapshot, len(results.Matches))
	for i, match := range results.Matches {
		matches[i] = nested_match_snapshot{
			RegisteredPattern: snapshot_pattern(match.RegisteredPattern),
			Params:            normalize_params(match.Params()),
			SplatValues:       normalize_splat(match.SplatValues()),
		}
	}
	return nested_matches_snapshot{
		Found:       true,
		Params:      normalize_params(results.Params),
		SplatValues: normalize_splat(results.SplatValues),
		Matches:     matches,
	}
}

func normalize_params(params matcher.Params) matcher.Params {
	if len(params) == 0 {
		return nil
	}
	return params
}

func normalize_splat(splat []string) []string {
	if len(splat) == 0 {
		return nil
	}
	return splat
}

func write_response[T any](out io.Writer, response protocol_response[T]) {
	_ = json.NewEncoder(out).Encode(response)
}
