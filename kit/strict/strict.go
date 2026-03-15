package strict

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

/////////////////////////////////////////////////////////////////////
/////// Filepath Types
/////////////////////////////////////////////////////////////////////

type CWDRelPath string
type MachAbsPath string

func (p CWDRelPath) Str() string  { return string(p) }
func (p MachAbsPath) Str() string { return string(p) }

// Panics if absolute.
// Runs `strings.TrimSpace`, `filepath.Clean`, `filepath.FromSlash`,
// and casts as `strict.CWDRelPath`.
func MustNormalize[P string | CWDRelPath](path P) CWDRelPath {
	if filepath.IsAbs(string(path)) {
		panic(
			"[strict.MustNormalize]: path must be CWD-relative, not absolute: " + path,
		)
	}
	return CWDRelPath(
		filepath.FromSlash(filepath.Clean(strings.TrimSpace(string(path)))),
	)
}

// Panics if absolute.
// Runs `strings.TrimSpace`, `filepath.Clean`, `filepath.FromSlash`,
// and casts as `strict.CWDRelPath`.
func (p CWDRelPath) MustNormalize() CWDRelPath {
	return MustNormalize(p)
}

func (p CWDRelPath) Join(elem ...string) CWDRelPath {
	return CWDRelPath(filepath.Join(append([]string{string(p)}, elem...)...))
}

func (p CWDRelPath) Dir() CWDRelPath {
	return CWDRelPath(filepath.Dir(string(p)))
}

// IsDir stats the path and returns true if it exists and is a directory.
// Returns an error if the path does not exist or cannot be statted.
func (p CWDRelPath) IsDir() (bool, error) {
	info, err := os.Stat(string(p))
	if err != nil {
		return false, fmt.Errorf("stat %q: %w", string(p), err)
	}
	return info.IsDir(), nil
}

// IsFile stats the path and returns true if it exists and is not a directory.
// Returns an error if the path does not exist or cannot be statted.
func (p CWDRelPath) IsFile() (bool, error) {
	info, err := os.Stat(string(p))
	if err != nil {
		return false, fmt.Errorf("stat %q: %w", string(p), err)
	}
	return !info.IsDir(), nil
}

/////////////////////////////////////////////////////////////////////
/////// Port Types
/////////////////////////////////////////////////////////////////////

type Port uint16
