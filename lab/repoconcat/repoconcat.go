package repoconcat

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
)

// Buffer pool for isTextFile
var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 512)
		return &buf
	},
}

// Pre-computed separators
var (
	separator     = strings.Repeat("=", 80)
	dashSeparator = strings.Repeat("—", 80)
)

// Buffer pool for file copying (64KB)
var copyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 64*1024)
		return &buf
	},
}

// Options configures optional behavior for Concat.
type Options struct {
	// If true, suppress file-level logging output.
	Quiet bool
}

var defaultExclude = []string{
	"!**/node_modules/**",
	"!**/.git/**",
	"!**/.vscode/**",
	"!**/.DS_Store",
	"!**/.gitignore",
	"!**/.gitignore.local",
	"!**/*.svg",
	"!**/go.sum",
	"!**/package-lock.json",
	"!**/yarn.lock",
	"!**/pnpm-lock.yaml",
	"!**/bun.lockb",
}

var defaultExcludedRoots = map[string]bool{
	"node_modules": true,
	".git":         true,
	".vscode":      true,
}

var defaultExcludedFiles = map[string]bool{
	".DS_Store":         true,
	".gitignore":        true,
	".gitignore.local":  true,
	"go.sum":            true,
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"bun.lockb":         true,
}

// compiledPattern holds a pre-processed pattern with its child pattern for directory matching
type compiledPattern struct {
	pattern  string
	childPat string // pattern + "/**" for matching files inside matching directories
	neg      bool
	dirOnly  bool // trailing slash pattern
}

type patternSet struct {
	broad    []compiledPattern
	specific []compiledPattern
}

// MustConcat calls Concat and panics on error.
//
// Patterns use glob syntax with gitignore-style semantics:
//   - Patterns without `/` match anywhere (e.g., `build` matches `./build` and `./src/build`).
//   - Leading `/` or `./` anchors to root (e.g., `/build` or `./build` matches only `./build`).
//   - Trailing `/` means directory (for inclusion: match contents; for exclusion: don't match files of same name).
//   - Prefix `!` for negation (e.g., `!*.log` excludes log files).
//   - Last match wins.
func MustConcat(output string, patterns []string, opts ...Options) {
	if err := Concat(output, patterns, opts...); err != nil {
		panic(err)
	}
}

// Concat concatenates files matching the given patterns into output.
//
// Patterns use glob syntax with gitignore-style semantics:
//   - Patterns without `/` match anywhere (e.g., `build` matches `./build` and `./src/build`).
//   - Leading `/` or `./` anchors to root (e.g., `/build` or `./build` matches only `./build`).
//   - Trailing `/` means directory (for inclusion: match contents; for exclusion: don't match files of same name).
//   - Prefix `!` for negation (e.g., `!*.log` excludes log files).
//   - Last match wins.
func Concat(output string, patterns []string, opts ...Options) error {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	outFile, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	for i := range patterns {
		patterns[i] = strings.TrimSpace(patterns[i])
	}

	outStat, _ := outFile.Stat()
	writer := bufio.NewWriter(outFile)

	roots := extractRoots(patterns)
	userPatterns := compileUserPatterns(patterns)
	defaultPatterns := compileDefaults()

	cwd, _ := os.Getwd()
	absOutput, _ := filepath.Abs(output)
	relOutput, _ := filepath.Rel(cwd, absOutput)
	if relOutput == "" {
		relOutput = output
	}

	var log strings.Builder
	log.Grow(4096) // Pre-allocate for typical output
	var included, skippedBinary int
	seen := make(map[string]bool, 256) // Pre-size for typical repo
	gitignoreCache := make(map[string][]compiledPattern, 32)
	patternCache := make(map[string][]compiledPattern, 64)

	// Check which default-excluded dirs user explicitly included
	overriddenDirs := make(map[string]bool)
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			continue
		}
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimSuffix(p, "/")
		p = strings.TrimSuffix(p, "/**")
		parts := strings.SplitN(p, "/", 2)
		if defaultExcludedRoots[parts[0]] {
			overriddenDirs[parts[0]] = true
		}
	}

	var lastDir string
	var lastPatterns []compiledPattern

	for _, root := range roots {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.Type()&os.ModeSymlink != 0 {
				return nil
			}

			// Early prune excluded directories (unless user overrode)
			if d.IsDir() {
				name := d.Name()
				if (name == ".git" || name == "node_modules" || name == ".vscode") && !overriddenDirs[name] {
					return filepath.SkipDir
				}
				return nil
			}

			if seen[path] {
				return nil
			}
			seen[path] = true

			info, err := d.Info()
			if err != nil {
				return nil
			}

			if outStat != nil && os.SameFile(info, outStat) {
				return nil
			}

			relPath := filepath.ToSlash(path)
			dir := filepath.Dir(path)

			// Fast path: same directory as last file
			var pats []compiledPattern
			if dir == lastDir {
				pats = lastPatterns
			} else {
				var ok bool
				pats, ok = patternCache[dir]
				if !ok {
					gitignore := getGitignorePatterns(dir, gitignoreCache)
					pats = combinePatterns(userPatterns, defaultPatterns, gitignore)
					patternCache[dir] = pats
				}
				lastDir = dir
				lastPatterns = pats
			}

			if !matchPatterns(pats, relPath) {
				return nil
			}

			if !isTextFile(path) {
				skippedBinary++
				return nil
			}

			if !opt.Quiet {
				log.WriteString(relPath)
				log.WriteString(" (")
				log.WriteString(formatSize(info.Size()))
				log.WriteString(")\n")
			}
			if writeFile(writer, path, relPath) == nil {
				included++
			}
			return nil
		})
	}

	writer.Flush()

	if !opt.Quiet {
		outInfo, _ := os.Stat(output)
		var sizeStr string
		if outInfo != nil {
			sizeStr = formatSize(outInfo.Size())
		}

		fmt.Println()
		fmt.Println("repoconcat | " + relOutput)
		fmt.Println(dashSeparator)
		fmt.Print(log.String())
		fmt.Println(dashSeparator)
		fmt.Printf("%d files (%s), %d binary skipped\n", included, sizeStr, skippedBinary)
		fmt.Println()
	}
	return nil
}

func compilePattern(pattern string) compiledPattern {
	neg := strings.HasPrefix(pattern, "!")
	pat := strings.TrimPrefix(pattern, "!")
	dirOnly := strings.HasSuffix(pat, "/")
	if dirOnly {
		pat = strings.TrimSuffix(pat, "/")
	}

	childPat := ""
	if !strings.HasSuffix(pat, "**") {
		childPat = pat + "/**"
	}

	return compiledPattern{
		pattern:  pat,
		childPat: childPat,
		neg:      neg,
		dirOnly:  dirOnly,
	}
}

func compileDefaults() []compiledPattern {
	out := make([]compiledPattern, len(defaultExclude))
	for i, p := range defaultExclude {
		out[i] = compilePattern(p)
	}
	return out
}

func compileUserPatterns(patterns []string) patternSet {
	var ps patternSet
	for _, p := range patterns {
		norm := normalizePattern(p)
		cp := compilePattern(norm)

		if cp.neg || isOverridePattern(p) {
			ps.specific = append(ps.specific, cp)
			continue
		}

		pat := strings.TrimPrefix(norm, "!")
		if !strings.ContainsAny(pat, "*") && !strings.HasSuffix(p, "/") && pat != "." {
			ps.specific = append(ps.specific, cp)
			continue
		}

		ps.broad = append(ps.broad, cp)
	}
	return ps
}

func normalizePattern(pattern string) string {
	neg := strings.HasPrefix(pattern, "!")
	pat := strings.TrimPrefix(pattern, "!")

	// Treat "./" as equivalent to "/" (anchored to root)
	if strings.HasPrefix(pat, "./") {
		pat = "/" + pat[2:]
	}

	if pat == "." || strings.HasPrefix(pat, "**") {
		if neg {
			return "!" + pat
		}
		return pat
	}

	anchored := strings.HasPrefix(pat, "/")
	if anchored {
		pat = pat[1:]
	} else {
		trimmed := strings.TrimSuffix(pat, "/")
		if !strings.Contains(trimmed, "/") {
			pat = "**/" + pat
		}
	}

	if neg {
		return "!" + pat
	}
	return pat
}

func isOverridePattern(pattern string) bool {
	pat := strings.TrimPrefix(pattern, "!")
	pat = strings.TrimPrefix(pat, "/")
	pat = strings.TrimPrefix(pat, "./")
	pat = strings.TrimPrefix(pat, "**/")

	parts := strings.SplitN(pat, "/", 2)
	if defaultExcludedRoots[parts[0]] {
		return true
	}

	filename := filepath.Base(strings.TrimSuffix(pat, "/"))
	return defaultExcludedFiles[filename]
}

func combinePatterns(user patternSet, defaults, gitignore []compiledPattern) []compiledPattern {
	total := len(user.broad) + len(defaults) + len(gitignore) + len(user.specific)
	out := make([]compiledPattern, 0, total)
	out = append(out, user.broad...)
	out = append(out, defaults...)
	out = append(out, gitignore...)
	out = append(out, user.specific...)
	return out
}

func matchPatterns(patterns []compiledPattern, path string) bool {
	matched := false
	for i := range patterns {
		p := &patterns[i]

		if p.pattern == "." {
			matched = !p.neg
			continue
		}

		var ok bool
		if p.dirOnly {
			// Trailing slash: only match if a parent directory matches
			// (not the file itself, even if it has the same name)
			ok = parentMatches(p.pattern, path)
		} else {
			// Check direct match
			ok, _ = doublestar.Match(p.pattern, path)
			// Check if inside a matching directory
			if !ok && p.childPat != "" {
				ok, _ = doublestar.Match(p.childPat, path)
			}
		}

		if ok {
			matched = !p.neg
		}
	}
	return matched
}

func parentMatches(pattern, path string) bool {
	dir := path
	for {
		dir = filepath.ToSlash(filepath.Dir(dir))
		if dir == "." {
			break
		}
		if ok, _ := doublestar.Match(pattern, dir); ok {
			return true
		}
	}
	return false
}

func extractRoots(patterns []string) []string {
	var roots []string
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			continue
		}
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimPrefix(p, "./")
		parts := strings.Split(p, "/")
		var lit []string
		for _, part := range parts {
			if strings.ContainsAny(part, "*?[{") {
				break
			}
			lit = append(lit, part)
		}
		root := "."
		if len(lit) > 0 {
			candidate := strings.Join(lit, "/")
			if candidate != "" && candidate != "." {
				root = strings.TrimSuffix(candidate, "/")
			}
		}
		if root == "" {
			root = "."
		}
		found := false
		for _, r := range roots {
			if r == root || r == "." || strings.HasPrefix(root, r+"/") {
				found = true
				break
			}
		}
		if !found {
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 {
		return []string{"."}
	}
	return roots
}

func getGitignorePatterns(dir string, cache map[string][]compiledPattern) []compiledPattern {
	absDir, _ := filepath.Abs(dir)
	if patterns, ok := cache[absDir]; ok {
		return patterns
	}

	cwd, _ := os.Getwd()

	// Walk up looking for cached parent first
	var uncached []string
	curr := absDir
	var parentPatterns []compiledPattern

	for {
		if cached, ok := cache[curr]; ok {
			parentPatterns = cached
			break
		}
		uncached = append(uncached, curr)
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	// Process uncached directories from root toward target
	for i := len(uncached) - 1; i >= 0; i-- {
		curr := uncached[i]
		var dirPatterns []compiledPattern
		for _, name := range []string{".gitignore", ".gitignore.local"} {
			path := filepath.Join(curr, name)
			relBase, _ := filepath.Rel(cwd, curr)
			if relBase == "." {
				relBase = ""
			}
			dirPatterns = append(dirPatterns, parseGitignore(path, relBase)...)
		}
		combined := make([]compiledPattern, 0, len(parentPatterns)+len(dirPatterns))
		combined = append(combined, parentPatterns...)
		combined = append(combined, dirPatterns...)
		cache[curr] = combined
		parentPatterns = combined
	}

	return cache[absDir]
}

func parseGitignore(path, base string) []compiledPattern {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []compiledPattern
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		neg := line[0] == '!'
		pattern := strings.TrimPrefix(line, "!")

		anchored := pattern[0] == '/'
		pattern = strings.TrimPrefix(pattern, "/")

		trimmed := strings.TrimSuffix(pattern, "/")
		if strings.Contains(trimmed, "/") {
			anchored = true
		}

		var finalPattern string
		if anchored {
			if base == "" {
				finalPattern = pattern
			} else {
				finalPattern = base + "/" + pattern
			}
		} else {
			finalPattern = "**/" + pattern
		}

		// Gitignore: negation means include, non-negation means exclude
		if neg {
			patterns = append(patterns, compilePattern(finalPattern))
		} else {
			patterns = append(patterns, compilePattern("!"+finalPattern))
		}
	}
	return patterns
}

func isTextFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	bufPtr := bufPool.Get().(*[]byte)
	buf := *bufPtr
	defer bufPool.Put(bufPtr)

	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	if n == 0 {
		return true
	}

	mimeType := http.DetectContentType(buf[:n])

	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	switch {
	case strings.HasPrefix(mimeType, "application/json"),
		strings.HasPrefix(mimeType, "application/xml"),
		strings.HasPrefix(mimeType, "application/javascript"),
		strings.HasPrefix(mimeType, "application/x-javascript"):
		return true
	}

	if mimeType == "application/octet-stream" {
		for i := 0; i < n; i++ {
			if buf[i] == 0 {
				return false
			}
		}
		return utf8.Valid(buf[:n])
	}

	return false
}

func writeFile(w *bufio.Writer, path, display string) error {
	w.WriteString(separator)
	w.WriteByte('\n')
	w.WriteString("FILE: ")
	w.WriteString(display)
	w.WriteByte('\n')
	w.WriteString(separator)
	w.WriteByte('\n')

	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(w, "[ERROR READING FILE: %v]\n", err)
		return err
	}
	defer file.Close()

	bufPtr := copyBufPool.Get().(*[]byte)
	_, err = io.CopyBuffer(w, file, *bufPtr)
	copyBufPool.Put(bufPtr)

	if err != nil {
		fmt.Fprintf(w, "[ERROR COPYING FILE: %v]\n", err)
	}

	w.WriteByte('\n')
	return err
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
