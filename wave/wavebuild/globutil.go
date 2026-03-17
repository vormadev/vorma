package wavebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/kit/strict"
)

func extension(p strict.CWDRelPath) string {
	return filepath.Ext(p.Str())
}

func path_match(
	pattern strict.CWDRelPath,
	path strict.CWDRelPath,
) (bool, error) {
	match, err := doublestar.PathMatch(pattern.Str(), path.Str())
	if err != nil {
		return false, fmt.Errorf(
			"invalid pattern: %w", err,
		)
	}
	return match, nil
}

func check_overlap(a strict.CWDRelPath, b strict.CWDRelPath) bool {
	return check_within_or_equal(a, b) || check_within_or_equal(b, a)
}

func check_within_or_equal(
	base strict.CWDRelPath,
	path_in_question strict.CWDRelPath,
) bool {
	if base == path_in_question {
		return true
	}
	matches, err := path_match(to_catch_all_pattern(base), path_in_question)
	if err != nil {
		// if the pattern is invalid, be conservative and assume it matches
		return true
	}
	return matches
}

func to_catch_all_pattern(p strict.CWDRelPath) strict.CWDRelPath {
	if strings.HasSuffix(strings.TrimSpace(p.Str()), "**/*") {
		return p
	}
	return p.Join("**/*")
}

func to_catch_all_pattern_if_dir(p strict.CWDRelPath) strict.CWDRelPath {
	if ok, err := p.IsDir(); err == nil && ok {
		return to_catch_all_pattern(p)
	}
	return p
}
