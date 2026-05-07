package fswatcher

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/globset"
	"github.com/vormadev/vorma/kit/set"
)

type watch_plan struct {
	match_set *globset.Set
	rules     globset.Rules
	roots     []watch_root
}

type watch_root struct {
	path    string
	dynamic bool
}

type pattern_prefix_state struct {
	pattern_index int
	prefix_index  int
}

func new_watch_plan(raw_patterns []string) (*watch_plan, error) {
	patterns := make([]string, len(raw_patterns))
	for i, raw_pattern := range raw_patterns {
		patterns[i] = strings.TrimSpace(raw_pattern)
	}
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	match_set, err := globset.Compile(patterns)
	if err != nil {
		return nil, fmt.Errorf("compile watch patterns: %w", err)
	}

	rules := match_set.Rules()
	roots := set.New[watch_root]()
	for _, rule := range rules {
		if rule.Excluded {
			continue
		}
		roots.Add(watch_root_for_rule(rule))
	}

	root_slice := roots.Slice()
	slices.SortFunc(root_slice, func(a, b watch_root) int {
		if a.path != b.path {
			return strings.Compare(a.path, b.path)
		}
		if a.dynamic == b.dynamic {
			return 0
		}
		if a.dynamic {
			return 1
		}
		return -1
	})

	return &watch_plan{
		match_set: match_set,
		rules:     rules,
		roots:     root_slice,
	}, nil
}

func (p *watch_plan) should_emit(path string, is_dir bool) bool {
	rel_path := watch_rel_path(path)
	if is_dir {
		rel_path += "/"
	}
	return p.match_set.Match(rel_path)
}

func (p *watch_plan) should_watch_dir(path string) bool {
	rel_path := watch_rel_path(path)
	return p.could_match_in_subtree(rel_path)
}

func (p *watch_plan) could_match_in_subtree(dir string) bool {
	dir = normalize_watch_path(dir)

	last_cover_index := -1
	last_cover_excluded := false
	for i, rule := range p.rules {
		if rule_covers_subtree(rule, dir) {
			last_cover_index = i
			last_cover_excluded = rule.Excluded
		}
	}

	if last_cover_index >= 0 && last_cover_excluded {
		for _, rule := range p.rules[last_cover_index+1:] {
			if !rule.Excluded && rule_can_match_in_subtree(rule, dir) {
				return true
			}
		}
		return false
	}

	for _, rule := range p.rules {
		if !rule.Excluded && rule_can_match_in_subtree(rule, dir) {
			return true
		}
	}
	return false
}

func watch_root_for_rule(rule globset.Rule) watch_root {
	if rule.Pattern == "." {
		return watch_root{path: "."}
	}

	pattern_segments := split_watch_path(rule.Pattern)
	literal_segments := make([]string, 0, len(pattern_segments))
	for _, segment := range pattern_segments {
		if segment_has_glob(segment) {
			break
		}
		literal_segments = append(literal_segments, segment)
	}

	if len(literal_segments) == 0 {
		return watch_root{path: "."}
	}

	root_path := path.Join(literal_segments...)
	if len(literal_segments) == len(pattern_segments) {
		return watch_root{path: root_path, dynamic: true}
	}
	return watch_root{path: root_path}
}

func watch_rel_path(path string) string {
	path = fsutil.SysNorm(path)
	rel_path, err := filepath.Rel(".", path)
	if err != nil {
		return normalize_watch_path(path)
	}
	return normalize_watch_path(rel_path)
}

func normalize_watch_path(raw_path string) string {
	raw_path = strings.TrimSpace(raw_path)
	raw_path = filepath.ToSlash(raw_path)
	raw_path = strings.TrimSuffix(raw_path, "/")
	raw_path = path.Clean(raw_path)
	raw_path = strings.TrimPrefix(raw_path, "./")
	if raw_path == "" {
		return "."
	}
	return raw_path
}

func split_watch_path(path string) []string {
	path = normalize_watch_path(path)
	if path == "." {
		return nil
	}
	return strings.Split(path, "/")
}

func segment_has_glob(segment string) bool {
	return strings.ContainsAny(segment, "*?[{")
}

func rule_can_match_in_subtree(rule globset.Rule, dir string) bool {
	if rule.Pattern == "." {
		return true
	}

	dir_segments := split_watch_path(dir)
	if pattern_can_match_prefix(rule.Pattern, dir_segments) {
		return true
	}
	if rule.ChildPattern != "" &&
		pattern_can_match_prefix(rule.ChildPattern, dir_segments) {
		return true
	}
	if rule.DirOnly {
		child_pattern := path.Join(rule.Pattern, "**")
		return pattern_can_match_prefix(child_pattern, dir_segments)
	}
	return false
}

func rule_covers_subtree(rule globset.Rule, dir string) bool {
	if rule.Pattern == "." {
		return true
	}

	for _, ancestor := range watch_path_ancestors(dir) {
		if pattern_matches_path(rule.Pattern, ancestor) &&
			(rule.ChildPattern != "" || rule.DirOnly) {
			return true
		}
		if pattern_suffix_covers_subtree(rule.Pattern, ancestor) {
			return true
		}
	}
	return false
}

func watch_path_ancestors(path string) []string {
	path = normalize_watch_path(path)
	ancestors := []string{path}
	for path != "." {
		path = normalize_watch_path(filepath.Dir(path))
		ancestors = append(ancestors, path)
	}
	return ancestors
}

func pattern_suffix_covers_subtree(pattern string, dir string) bool {
	base, ok := strings.CutSuffix(pattern, "/**")
	if !ok {
		return false
	}
	if base == "" {
		return true
	}
	return pattern_matches_path(base, dir)
}

func pattern_matches_path(pattern string, path string) bool {
	pattern = normalize_watch_path(pattern)
	path = normalize_watch_path(path)
	if pattern == "." {
		return true
	}
	ok, _ := doublestar.Match(pattern, path)
	return ok
}

func pattern_can_match_prefix(pattern string, prefix_segments []string) bool {
	pattern_segments := split_watch_path(pattern)
	seen := map[pattern_prefix_state]bool{}

	var walk func(pattern_index, prefix_index int) bool
	walk = func(pattern_index, prefix_index int) bool {
		if prefix_index == len(prefix_segments) {
			return true
		}
		if pattern_index == len(pattern_segments) {
			return false
		}

		state := pattern_prefix_state{
			pattern_index: pattern_index,
			prefix_index:  prefix_index,
		}
		if seen[state] {
			return false
		}
		seen[state] = true

		pattern_segment := pattern_segments[pattern_index]
		if pattern_segment == "**" {
			if walk(pattern_index+1, prefix_index) {
				return true
			}
			return walk(pattern_index, prefix_index+1)
		}

		ok, _ := doublestar.Match(pattern_segment, prefix_segments[prefix_index])
		if !ok {
			return false
		}
		return walk(pattern_index+1, prefix_index+1)
	}

	return walk(0, 0)
}
