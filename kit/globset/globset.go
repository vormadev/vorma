package globset

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type Set struct {
	rules Rules
}

type Rule struct {
	Raw          string
	Pattern      string
	ChildPattern string
	Excluded     bool
	DirOnly      bool
	Anchored     bool
	HasGlob      bool
}

type Rules []Rule

func Parse(raw_rule string) (Rule, bool, error) {
	raw_rule = strings.TrimSpace(raw_rule)
	if raw_rule == "" {
		return Rule{}, false, nil
	}

	excluded := strings.HasPrefix(raw_rule, "!")
	pattern := strings.TrimPrefix(raw_rule, "!")
	pattern = strings.TrimSpace(pattern)
	pattern = filepath.ToSlash(pattern)

	if pattern == "" {
		return Rule{}, false, nil
	}

	dir_only := strings.HasSuffix(pattern, "/")
	if dir_only {
		pattern = strings.TrimSuffix(pattern, "/")
	}

	if after, ok := strings.CutPrefix(pattern, "./"); ok {
		pattern = "/" + after
	}

	anchored := strings.HasPrefix(pattern, "/")

	if pattern != "." && !strings.HasPrefix(pattern, "**") {
		if anchored {
			pattern = strings.TrimPrefix(pattern, "/")
		} else if !strings.Contains(pattern, "/") {
			pattern = "**/" + pattern
		}
	}

	if pattern == "" {
		return Rule{}, false, nil
	}

	if !doublestar.ValidatePattern(pattern) {
		return Rule{}, false, fmt.Errorf("bad pattern")
	}

	child_pattern := ""
	if !dir_only && pattern != "." && !strings.HasSuffix(pattern, "**") {
		child_pattern = pattern + "/**"
	}

	return Rule{
		Raw:          raw_rule,
		Pattern:      pattern,
		ChildPattern: child_pattern,
		Excluded:     excluded,
		DirOnly:      dir_only,
		Anchored:     anchored,
		HasGlob:      strings.ContainsAny(pattern, "*?[{"),
	}, true, nil
}

func Compile(raw_rules []string) (*Set, error) {
	rules := make(Rules, 0, len(raw_rules))

	for i, raw_rule := range raw_rules {
		rule, ok, err := Parse(raw_rule)
		if err != nil {
			return nil, fmt.Errorf("invalid rule %d (%q): %w", i, raw_rule, err)
		}
		if !ok {
			continue
		}
		rules = append(rules, rule)
	}

	return &Set{rules: rules}, nil
}

func MustCompile(raw_rules []string) *Set {
	set, err := Compile(raw_rules)
	if err != nil {
		panic(err)
	}
	return set
}

func (rules Rules) Compile() *Set {
	return &Set{rules: rules}
}

func (rule Rule) HasSegment(name string) bool {
	for segment := range strings.SplitSeq(rule.Pattern, "/") {
		if segment == "" || segment == "**" {
			continue
		}

		ok, _ := doublestar.Match(segment, name)
		if ok {
			return true
		}
	}

	return false
}

func (s *Set) Rules() Rules { return s.rules }

func (s *Set) Match(path string) bool {
	path, is_dir := normalize_path(path)
	matched := false

	for _, rule := range s.rules {
		if rule.Pattern == "." {
			matched = !rule.Excluded
			continue
		}

		if matches_rule(rule, path, is_dir) {
			matched = !rule.Excluded
		}
	}

	return matched
}

func normalize_path(_path string) (string, bool) {
	_path = strings.TrimSpace(_path)
	_path = filepath.ToSlash(_path)

	is_dir := strings.HasSuffix(_path, "/")
	_path = strings.TrimSuffix(_path, "/")
	_path = path.Clean(_path)
	_path = strings.TrimPrefix(_path, "./")

	if _path == "." || _path == "" {
		return ".", is_dir
	}

	return _path, is_dir
}

func matches_rule(rule Rule, path string, is_dir bool) bool {
	if rule.DirOnly {
		if is_dir {
			if matches_exact(rule.Pattern, path, rule.Anchored) {
				return true
			}
		}
		return matches_parent(rule.Pattern, path, rule.Anchored)
	}

	if matches_exact(rule.Pattern, path, rule.Anchored) {
		return true
	}

	if rule.ChildPattern == "" {
		return false
	}

	return matches_child(rule, path)
}

func matches_exact(pattern, path string, anchored bool) bool {
	if anchored && !strings.Contains(pattern, "/") && strings.Contains(path, "/") {
		return false
	}

	ok, _ := doublestar.Match(pattern, path)
	return ok
}

func matches_child(rule Rule, path string) bool {
	if rule.Anchored && !strings.Contains(rule.Pattern, "/") {
		before, _, ok := strings.Cut(path, "/")
		if !ok {
			return false
		}
		first := before
		ok, _ = doublestar.Match(rule.Pattern, first)
		return ok
	}

	ok, _ := doublestar.Match(rule.ChildPattern, path)
	return ok
}

func matches_parent(pattern, _path string, anchored bool) bool {
	dir := _path
	for {
		dir = path.Dir(dir)
		if dir == "." {
			return false
		}
		if matches_exact(pattern, dir, anchored) {
			return true
		}
	}
}
