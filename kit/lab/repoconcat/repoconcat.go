package repoconcat

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/vormadev/vorma/kit/globset"
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

type patternSet struct {
	broad           globset.Rules
	specific        globset.Rules
	overridden_dirs map[string]bool
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
func Concat(
	output string,
	patterns []string,
	opts ...Options,
) (returnErr error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	outFile, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		closeErr := outFile.Close()
		if closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("closing output file: %w", closeErr)
		}
	}()

	normalizedPatterns := make([]string, len(patterns))
	for i := range patterns {
		normalizedPatterns[i] = strings.TrimSpace(patterns[i])
	}

	outStat, _ := outFile.Stat()
	writer := bufio.NewWriter(outFile)

	roots := extractRoots(normalizedPatterns)
	userPatterns := compileUserPatterns(normalizedPatterns)
	defaultPatterns, err := globset.Compile(defaultExclude)
	if err != nil {
		return fmt.Errorf("compile default rules: %w", err)
	}

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
	gitignoreCache := make(map[string]globset.Rules, 32)
	patternCache := make(map[string]*globset.Set, 64)

	var lastDir string
	var lastPatterns *globset.Set

	for _, root := range roots {
		walkErr := filepath.WalkDir(
			root,
			func(path string, d os.DirEntry, err error) error {
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						return nil
					}
					return err
				}

				if d.Type()&os.ModeSymlink != 0 {
					return nil
				}

				// Early prune excluded directories (unless user overrode)
				if d.IsDir() {
					name := d.Name()
					if (name == ".git" || name == "node_modules" || name == ".vscode") &&
						!userPatterns.overridden_dirs[name] {
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
				var pats *globset.Set
				if dir == lastDir {
					pats = lastPatterns
				} else {
					var ok bool
					pats, ok = patternCache[dir]
					if !ok {
						gitignore := getGitignorePatterns(dir, gitignoreCache)
						rules := combinePatterns(
							userPatterns,
							defaultPatterns.Rules(),
							gitignore,
						)
						pats = rules.Compile()
						patternCache[dir] = pats
					}
					lastDir = dir
					lastPatterns = pats
				}

				if !pats.Match(relPath) {
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
				if writeErr := writeFile(writer, path, relPath); writeErr != nil {
					return writeErr
				}
				included++
				return nil
			},
		)
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("walk root %q: %w", root, walkErr)
		}
	}

	if flushErr := writer.Flush(); flushErr != nil {
		return fmt.Errorf("flushing output writer: %w", flushErr)
	}

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
		fmt.Printf(
			"%d files (%s), %d binary skipped\n",
			included,
			sizeStr,
			skippedBinary,
		)
		fmt.Println()
	}
	return nil
}

func compileUserPatterns(patterns []string) patternSet {
	ps := patternSet{
		overridden_dirs: make(map[string]bool),
	}

	for _, p := range patterns {
		rule, ok, err := globset.Parse(p)
		if err != nil || !ok {
			continue
		}

		if !rule.Excluded {
			for root := range defaultExcludedRoots {
				if rule.HasSegment(root) {
					ps.overridden_dirs[root] = true
				}
			}
		}

		if rule.Excluded || isOverridePattern(rule) {
			ps.specific = append(ps.specific, rule)
			continue
		}

		if !rule.HasGlob && !rule.DirOnly && rule.Pattern != "." {
			ps.specific = append(ps.specific, rule)
			continue
		}

		ps.broad = append(ps.broad, rule)
	}
	return ps
}
func isOverridePattern(rule globset.Rule) bool {
	pat := strings.TrimPrefix(rule.Pattern, "**/")
	parts := strings.SplitN(pat, "/", 2)
	if defaultExcludedRoots[parts[0]] {
		return true
	}

	filename := filepath.Base(pat)
	return defaultExcludedFiles[filename]
}

func combinePatterns(
	user patternSet,
	defaults, gitignore globset.Rules,
) globset.Rules {
	total := len(user.broad) + len(defaults) + len(gitignore) + len(user.specific)
	out := make(globset.Rules, 0, total)
	out = append(out, user.broad...)
	out = append(out, defaults...)
	out = append(out, gitignore...)
	out = append(out, user.specific...)
	return out
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

func getGitignorePatterns(
	dir string,
	cache map[string]globset.Rules,
) globset.Rules {
	absDir, _ := filepath.Abs(dir)
	if patterns, ok := cache[absDir]; ok {
		return patterns
	}

	cwd, _ := os.Getwd()

	// Walk up looking for cached parent first
	var uncached []string
	curr := absDir
	var parentPatterns globset.Rules

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
		var dirPatterns globset.Rules
		for _, name := range []string{".gitignore", ".gitignore.local"} {
			path := filepath.Join(curr, name)
			relBase, _ := filepath.Rel(cwd, curr)
			if relBase == "." {
				relBase = ""
			}
			dirPatterns = append(dirPatterns, parseGitignore(path, relBase)...)
		}
		combined := make(globset.Rules, 0, len(parentPatterns)+len(dirPatterns))
		combined = append(combined, parentPatterns...)
		combined = append(combined, dirPatterns...)
		cache[curr] = combined
		parentPatterns = combined
	}

	return cache[absDir]
}

func parseGitignore(path, base string) globset.Rules {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns globset.Rules
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		neg := line[0] == '!'
		pattern := strings.TrimPrefix(line, "!")
		if pattern == "" {
			continue
		}

		anchored := strings.HasPrefix(pattern, "/")
		pattern = strings.TrimPrefix(pattern, "/")
		if pattern == "" {
			continue
		}

		trimmed := strings.TrimSuffix(pattern, "/")
		if strings.Contains(trimmed, "/") {
			anchored = true
		}

		var finalPattern string
		if anchored {
			if base == "" {
				finalPattern = "/" + pattern
			} else {
				finalPattern = "/" + base + "/" + pattern
			}
		} else {
			finalPattern = "**/" + pattern
		}

		var rule string
		if neg {
			rule = finalPattern
		} else {
			rule = "!" + finalPattern
		}

		compiled, ok, err := globset.Parse(rule)
		if err == nil && ok {
			patterns = append(patterns, compiled)
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
		for i := range n {
			if buf[i] == 0 {
				return false
			}
		}
		return utf8.Valid(buf[:n])
	}

	return false
}

func writeFile(w *bufio.Writer, path, display string) error {
	if _, err := w.WriteString(separator); err != nil {
		return fmt.Errorf("write leading separator for %s: %w", display, err)
	}
	if err := w.WriteByte('\n'); err != nil {
		return fmt.Errorf(
			"write leading separator newline for %s: %w",
			display,
			err,
		)
	}
	if _, err := w.WriteString("FILE: "); err != nil {
		return fmt.Errorf("write file label for %s: %w", display, err)
	}
	if _, err := w.WriteString(display); err != nil {
		return fmt.Errorf("write file path label for %s: %w", display, err)
	}
	if err := w.WriteByte('\n'); err != nil {
		return fmt.Errorf("write file label newline for %s: %w", display, err)
	}
	if _, err := w.WriteString(separator); err != nil {
		return fmt.Errorf("write trailing separator for %s: %w", display, err)
	}
	if err := w.WriteByte('\n'); err != nil {
		return fmt.Errorf(
			"write trailing separator newline for %s: %w",
			display,
			err,
		)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", display, err)
	}
	defer file.Close()

	bufPtr := copyBufPool.Get().(*[]byte)
	_, err = io.CopyBuffer(w, file, *bufPtr)
	copyBufPool.Put(bufPtr)

	if err != nil {
		return fmt.Errorf("copy %s: %w", display, err)
	}

	if err := w.WriteByte('\n'); err != nil {
		return fmt.Errorf(
			"write file trailing newline for %s: %w",
			display,
			err,
		)
	}
	return nil
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
