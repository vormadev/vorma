package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/vormadev/vorma/kit/matcher"
)

var matcher_test_backend_once sync.Once
var matcher_test_backend_cache []matcher_test_backend
var matcher_test_backend_err error
var matcher_test_workers []*matcher_test_backend_worker

type matcher_test_backend struct {
	name   string
	dir    string
	args   []string
	worker *matcher_test_backend_worker
}

type matcher_test_backend_worker struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr bytes.Buffer
}

type matcher_test_matcher struct {
	tb       testing.TB
	backend  matcher_test_backend
	opts     *matcher.Options
	patterns []string
}

type matcher_test_pattern struct {
	original_pattern    string
	normalized_pattern  string
	normalized_segments []matcher.Segment
}

func (p *matcher_test_pattern) OriginalPattern() string {
	if p == nil {
		return ""
	}
	return p.original_pattern
}

func (p *matcher_test_pattern) NormalizedPattern() string {
	if p == nil {
		return ""
	}
	return p.normalized_pattern
}

func (p *matcher_test_pattern) NormalizedSegments() []matcher.Segment {
	if p == nil {
		return nil
	}
	out := make([]matcher.Segment, len(p.normalized_segments))
	copy(out, p.normalized_segments)
	return out
}

type matcher_test_best_match struct {
	*matcher_test_pattern
	Params      matcher.Params
	SplatValues []string
}

type matcher_test_nested_match struct {
	*matcher_test_pattern
	params       matcher.Params
	splat_values []string
}

func (m *matcher_test_nested_match) Params() matcher.Params {
	if m == nil || len(m.params) == 0 {
		return nil
	}
	out := make(matcher.Params, len(m.params))
	maps.Copy(out, m.params)
	return out
}

func (m *matcher_test_nested_match) SplatValues() []string {
	if m == nil || len(m.splat_values) == 0 {
		return nil
	}
	out := make([]string, len(m.splat_values))
	copy(out, m.splat_values)
	return out
}

type matcher_test_nested_results struct {
	Params      matcher.Params
	SplatValues []string
	Matches     []*matcher_test_nested_match
}

type matcher_test_config struct {
	dynamic_param_prefix              string
	splat_segment_identifier          string
	explicit_index_segment_identifier string
}

func run_matcher_backends(
	t *testing.T,
	fn func(t *testing.T, backend matcher_test_backend),
) {
	t.Helper()

	for _, backend := range matcher_test_backends(t) {
		t.Run(backend.name, func(t *testing.T) {
			fn(t, backend)
		})
	}
}

func matcher_test_backends(t *testing.T) []matcher_test_backend {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	matcher_test_backend_once.Do(func() {
		matcher_test_backend_cache, matcher_test_backend_err =
			start_matcher_test_backends(wd)
	})
	if matcher_test_backend_err != nil {
		t.Fatalf("%v", matcher_test_backend_err)
	}
	return matcher_test_backend_cache
}

func TestMain(m *testing.M) {
	code := m.Run()
	for _, worker := range matcher_test_workers {
		worker.close()
	}
	os.Exit(code)
}

func start_matcher_test_backends(
	wd string,
) ([]matcher_test_backend, error) {
	go_backend, err := start_matcher_test_backend(matcher_test_backend{
		name: "go",
		dir:  wd,
		args: []string{"go", "run", "."},
	})
	if err != nil {
		return nil, err
	}
	ts_backend, err := start_matcher_test_backend(matcher_test_backend{
		name: "ts",
		dir:  wd,
		args: []string{"node", "ts_cli.ts"},
	})
	if err != nil {
		return nil, err
	}
	return []matcher_test_backend{go_backend, ts_backend}, nil
}

func start_matcher_test_backend(
	backend matcher_test_backend,
) (matcher_test_backend, error) {
	cmd := exec.Command(backend.args[0], backend.args[1:]...)
	cmd.Dir = backend.dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return matcher_test_backend{}, fmt.Errorf(
			"open %s matcher backend stdin: %w",
			backend.name,
			err,
		)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return matcher_test_backend{}, fmt.Errorf(
			"open %s matcher backend stdout: %w",
			backend.name,
			err,
		)
	}
	worker := &matcher_test_backend_worker{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}
	cmd.Stderr = &worker.stderr
	if err := cmd.Start(); err != nil {
		return matcher_test_backend{}, fmt.Errorf(
			"start %s matcher backend: %w",
			backend.name,
			err,
		)
	}
	matcher_test_workers = append(matcher_test_workers, worker)
	backend.worker = worker
	return backend, nil
}

func new_matcher_test_matcher(
	tb testing.TB,
	backend matcher_test_backend,
	opts *matcher.Options,
) *matcher_test_matcher {
	tb.Helper()
	return &matcher_test_matcher{
		tb:      tb,
		backend: backend,
		opts:    opts,
	}
}

func must_new_go_matcher(tb testing.TB, opts *matcher.Options) *matcher.Matcher {
	tb.Helper()
	m, err := matcher.New(opts)
	if err != nil {
		tb.Fatalf("New() error: %v", err)
	}
	return m
}

func must_register_go_pattern(
	tb testing.TB,
	m *matcher.Matcher,
	pattern string,
) *matcher.RegisteredPattern {
	tb.Helper()
	rp, err := m.RegisterPattern(pattern)
	if err != nil {
		tb.Fatalf("RegisterPattern(%q) error: %v", pattern, err)
	}
	return rp
}

func must_register_test_pattern(
	tb testing.TB,
	m *matcher_test_matcher,
	pattern string,
) *matcher_test_pattern {
	tb.Helper()
	rp, err := m.RegisterPattern(pattern)
	if err != nil {
		tb.Fatalf("RegisterPattern(%q) error: %v", pattern, err)
	}
	return rp
}

func (m *matcher_test_matcher) NormalizePattern(
	pattern string,
) (*matcher_test_pattern, error) {
	var snapshot pattern_snapshot
	err := m.backend.call(m.tb, protocol_request{
		Operation: op_normalize_pattern,
		Options:   matcher_test_options(m.opts),
		Pattern:   pattern,
	}, &snapshot)
	if err != nil {
		return nil, err
	}
	return matcher_test_pattern_from_snapshot(&snapshot), nil
}

func (m *matcher_test_matcher) RegisterPattern(
	pattern string,
) (*matcher_test_pattern, error) {
	patterns := append(append([]string(nil), m.patterns...), pattern)
	var snapshot pattern_snapshot
	err := m.backend.call(m.tb, protocol_request{
		Operation: op_register_pattern,
		Options:   matcher_test_options(m.opts),
		Patterns:  patterns,
	}, &snapshot)
	if err != nil {
		return nil, err
	}
	m.patterns = patterns
	return matcher_test_pattern_from_snapshot(&snapshot), nil
}

func (m *matcher_test_matcher) FindBestMatch(
	path string,
) (*matcher_test_best_match, bool) {
	var snapshot best_match_snapshot
	if err := m.backend.call(m.tb, protocol_request{
		Operation: op_find_best_match,
		Options:   matcher_test_options(m.opts),
		Patterns:  m.patterns,
		Path:      path,
	}, &snapshot); err != nil {
		m.tb.Fatalf("FindBestMatch(%q) error: %v", path, err)
	}
	if !snapshot.Found {
		return nil, false
	}
	return &matcher_test_best_match{
		matcher_test_pattern: matcher_test_pattern_from_snapshot(
			snapshot.RegisteredPattern,
		),
		Params:      normalize_test_params(snapshot.Params),
		SplatValues: normalize_test_splat(snapshot.SplatValues),
	}, true
}

func (m *matcher_test_matcher) FindNestedMatches(
	path string,
) (*matcher_test_nested_results, bool) {
	var snapshot nested_matches_snapshot
	if err := m.backend.call(m.tb, protocol_request{
		Operation: op_find_nested_matches,
		Options:   matcher_test_options(m.opts),
		Patterns:  m.patterns,
		Path:      path,
	}, &snapshot); err != nil {
		m.tb.Fatalf("FindNestedMatches(%q) error: %v", path, err)
	}
	if !snapshot.Found {
		return nil, false
	}
	matches := make([]*matcher_test_nested_match, len(snapshot.Matches))
	for i, match := range snapshot.Matches {
		matches[i] = &matcher_test_nested_match{
			matcher_test_pattern: matcher_test_pattern_from_snapshot(
				match.RegisteredPattern,
			),
			params:       normalize_test_params(match.Params),
			splat_values: normalize_test_splat(match.SplatValues),
		}
	}
	return &matcher_test_nested_results{
		Params:      normalize_test_params(snapshot.Params),
		SplatValues: normalize_test_splat(snapshot.SplatValues),
		Matches:     matches,
	}, true
}

func (backend matcher_test_backend) parse_segments(
	tb testing.TB,
	path string,
) []string {
	var segments []string
	if err := backend.call(tb, protocol_request{
		Operation: op_parse_segments,
		Path:      path,
	}, &segments); err != nil {
		tb.Fatalf("ParseSegments(%q) error: %v", path, err)
	}
	return segments
}

func (backend matcher_test_backend) matcher_config(
	tb testing.TB,
	opts *matcher.Options,
) matcher_test_config {
	var snapshot matcher_config_snapshot
	if err := backend.call(tb, protocol_request{
		Operation: op_matcher_config,
		Options:   matcher_test_options(opts),
	}, &snapshot); err != nil {
		tb.Fatalf("matcher config error: %v", err)
	}
	return matcher_test_config{
		dynamic_param_prefix:              snapshot.DynamicParamPrefix,
		splat_segment_identifier:          snapshot.SplatSegmentIdentifier,
		explicit_index_segment_identifier: snapshot.ExplicitIndexSegmentIdentifier,
	}
}

func (backend matcher_test_backend) path_helpers(
	tb testing.TB,
	path string,
) path_helpers_snapshot {
	var snapshot path_helpers_snapshot
	if err := backend.call(tb, protocol_request{
		Operation: op_path_helpers,
		Path:      path,
	}, &snapshot); err != nil {
		tb.Fatalf("path helpers(%q) error: %v", path, err)
	}
	return snapshot
}

func (backend matcher_test_backend) join_patterns(
	tb testing.TB,
	opts *matcher.Options,
	pattern string,
	suffix string,
) string {
	var joined string
	if err := backend.call(tb, protocol_request{
		Operation: op_join_patterns,
		Options:   matcher_test_options(opts),
		Pattern:   pattern,
		Suffix:    suffix,
	}, &joined); err != nil {
		tb.Fatalf("JoinPatterns(%q, %q) error: %v", pattern, suffix, err)
	}
	return joined
}

func (backend matcher_test_backend) call(
	tb testing.TB,
	req protocol_request,
	out any,
) error {
	tb.Helper()

	raw, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	if backend.worker == nil {
		return fmt.Errorf("%s matcher backend is not running", backend.name)
	}
	output, err := backend.worker.call(raw)
	if err != nil {
		return fmt.Errorf(
			"call %s matcher backend: %w; stderr: %s",
			backend.name,
			err,
			backend.worker.stderr.String(),
		)
	}

	var response protocol_response[json.RawMessage]
	if err := json.Unmarshal(output, &response); err != nil {
		return fmt.Errorf("decode backend response %q: %w", string(output), err)
	}
	if !response.OK {
		return fmt.Errorf("%s", response.Err)
	}
	if out != nil && len(response.Val) > 0 {
		if err := json.Unmarshal(response.Val, out); err != nil {
			return fmt.Errorf(
				"decode backend response val %q: %w",
				string(response.Val),
				err,
			)
		}
	}
	return nil
}

func (worker *matcher_test_backend_worker) call(raw []byte) ([]byte, error) {
	worker.mu.Lock()
	defer worker.mu.Unlock()

	if _, err := worker.stdin.Write(append(raw, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}
	output, err := worker.stdout.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return output, nil
}

func (worker *matcher_test_backend_worker) close() {
	worker.mu.Lock()
	defer worker.mu.Unlock()

	_ = worker.stdin.Close()
	_ = worker.cmd.Wait()
}

func matcher_test_options(opts *matcher.Options) protocol_options {
	if opts == nil {
		return protocol_options{}
	}
	return protocol_options{
		DynamicParamPrefix:             rune_string(opts.DynamicParamPrefix),
		SplatSegmentIdentifier:         rune_string(opts.SplatSegmentIdentifier),
		ExplicitIndexSegmentIdentifier: opts.ExplicitIndexSegmentIdentifier,
		Quiet:                          opts.Quiet,
	}
}

func rune_string(r rune) string {
	if r == 0 || r == utf8.RuneError {
		return ""
	}
	return string(r)
}

func matcher_test_pattern_from_snapshot(
	snapshot *pattern_snapshot,
) *matcher_test_pattern {
	if snapshot == nil {
		return nil
	}
	segments := make([]matcher.Segment, len(snapshot.NormalizedSegments))
	for i, segment := range snapshot.NormalizedSegments {
		segments[i] = matcher.Segment{
			NormalizedVal: segment.NormalizedVal,
			Type:          segment.SegType,
		}
	}
	return &matcher_test_pattern{
		original_pattern:    snapshot.OriginalPattern,
		normalized_pattern:  snapshot.NormalizedPattern,
		normalized_segments: segments,
	}
}

func normalize_test_params(params matcher.Params) matcher.Params {
	if len(params) == 0 {
		return nil
	}
	return params
}

func normalize_test_splat(splat []string) []string {
	if len(splat) == 0 {
		return nil
	}
	return splat
}
