// buyer beware
package repoconcat

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
)

type Config struct {
	Output       string   // Output file path
	IncludeDirs  []string // Directory patterns to scan (supports globs)
	IncludeFiles []string // File patterns to include (supports globs)
	IgnoreDirs   []string // Directory patterns to skip (matched against full path)
	IgnoreFiles  []string // File patterns to skip (matched against full path)
	Verbose      bool     // Log included files and folders
}

// DefaultIgnoreDirs are directory patterns excluded by default.
// Override by setting your own IgnoreDirs (these are additive).
var DefaultIgnoreDirs = []string{
	"**/node_modules", "**/.git", "**/.vscode",
}

// DefaultIgnoreFiles are file patterns excluded by default.
var DefaultIgnoreFiles = []string{
	"**/.DS_Store", "**/.gitignore", "**/.gitignore.local",
	"**/*.svg", "**/go.sum", "**/package-lock.json",
	"**/yarn.lock", "**/pnpm-lock.yaml", "**/bun.lockb",
}

func isTextFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	buf := make([]byte, 8192)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	return utf8.Valid(buf[:n])
}

func matchesPattern(pattern, path string) (bool, error) {
	if pattern == "" {
		return false, nil
	}
	return doublestar.Match(pattern, path)
}

func normalizePath(path string) string {
	path = filepath.Clean(path)
	path = filepath.ToSlash(path)
	path = strings.TrimPrefix(path, "./")
	return path
}

func matchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := matchesPattern(normalizePath(pattern), path)
		if err != nil {
			// Invalid pattern, skip it but could log in verbose mode
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// matchesAnyPatternOrParent checks if path or any parent dir matches patterns
func matchesAnyPatternOrParent(path string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = normalizePath(pattern)
		matched, err := matchesPattern(pattern, path)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
		// Also check if path is inside a matched directory
		matched, err = matchesPattern(pattern+"/**", path)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// expandGlob expands a glob pattern to matching paths.
// If the pattern contains no glob characters, it returns the pattern as-is
// (to be validated later by os.Stat).
func expandGlob(pattern string) ([]string, error) {
	// Check if pattern contains glob characters
	if !strings.ContainsAny(pattern, "*?[{") {
		return []string{pattern}, nil
	}
	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
	}
	return matches, nil
}

func Concat(cfg Config) {
	// Validate config
	if len(cfg.IncludeDirs) == 0 && len(cfg.IncludeFiles) == 0 {
		log.Println("Warning: no IncludeDirs or IncludeFiles specified, nothing to concatenate")
		return
	}

	outFile, err := os.Create(cfg.Output)
	if err != nil {
		log.Fatalf("creating output file: %v", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	defer writer.Flush()

	absOutput, err := filepath.Abs(cfg.Output)
	if err != nil {
		log.Fatalf("resolving output path: %v", err)
	}

	writtenFiles := make(map[string]bool)
	var includedFiles, skippedBinary, skippedDuplicate int

	writeIfNew := func(absPath, diskPath, displayPath string, info os.FileInfo) {
		if writtenFiles[absPath] {
			skippedDuplicate++
			return
		}
		if !isTextFile(diskPath) {
			skippedBinary++
			if cfg.Verbose {
				fmt.Printf("[SKIP] %s (binary)\n", displayPath)
			}
			return
		}
		if err := writeFile(writer, diskPath, displayPath, info, cfg.Verbose); err == nil {
			writtenFiles[absPath] = true
			includedFiles++
		}
	}

	fmt.Printf("Starting concatenation to '%s'...\n", cfg.Output)

	// Process individual files first
	for _, filePattern := range cfg.IncludeFiles {
		// Expand glob patterns
		matches, err := expandGlob(filePattern)
		if err != nil {
			fmt.Printf("[ERROR] %s: %v\n", filePattern, err)
			continue
		}
		if len(matches) == 0 {
			if cfg.Verbose {
				fmt.Printf("[SKIP] %s (no matches)\n", filePattern)
			}
			continue
		}

		for _, filePath := range matches {
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				fmt.Printf("[ERROR] %s: %v\n", filePath, err)
				continue
			}
			if absPath == absOutput {
				continue
			}

			info, err := os.Stat(filePath)
			if err != nil {
				if cfg.Verbose {
					fmt.Printf("[SKIP] %s (error: %v)\n", filePath, err)
				}
				continue
			}
			if info.IsDir() {
				if cfg.Verbose {
					fmt.Printf("[SKIP] %s (is a directory)\n", filePath)
				}
				continue
			}

			normalizedPath := normalizePath(filePath)

			// Check user patterns (full path)
			if matchesAnyPattern(normalizedPath, cfg.IgnoreFiles) {
				continue
			}
			if matchesAnyPattern(normalizedPath, DefaultIgnoreFiles) {
				continue
			}

			// Check gitignore patterns for IncludeFiles
			fileDir := filepath.Dir(filePath)
			var gitRoot string
			for _, dir := range cfg.IncludeDirs {
				absDir, err := filepath.Abs(dir)
				if err != nil {
					continue
				}
				if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) {
					gitRoot = dir
					break
				}
			}
			if gitRoot == "" {
				gitRoot = fileDir
			}

			gitignoreCache := make(map[string][]string)
			patterns := getGitignorePatterns(filePath, gitRoot, gitignoreCache)
			relToRoot, err := filepath.Rel(gitRoot, filePath)
			if err == nil {
				relToRoot = normalizePath(relToRoot)
				if matchesAnyPatternOrParent(relToRoot, patterns) {
					continue
				}
			}

			writeIfNew(absPath, filePath, normalizedPath, info)
		}
	}

	// Process directories (with glob expansion)
	for _, dirPattern := range cfg.IncludeDirs {
		// Expand glob patterns
		matches, err := expandGlob(dirPattern)
		if err != nil {
			fmt.Printf("[ERROR] %s: %v\n", dirPattern, err)
			continue
		}
		if len(matches) == 0 {
			if cfg.Verbose {
				fmt.Printf("[SKIP] %s (no matches)\n", dirPattern)
			}
			continue
		}

		for _, rootDir := range matches {
			info, err := os.Stat(rootDir)
			if err != nil {
				fmt.Printf("[ERROR] %s: %v\n", rootDir, err)
				continue
			}
			if !info.IsDir() {
				if cfg.Verbose {
					fmt.Printf("[SKIP] %s (not a directory)\n", rootDir)
				}
				continue
			}

			rootDir = filepath.Clean(rootDir)
			gitignoreCache := make(map[string][]string)

			err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				absPath, err := filepath.Abs(path)
				if err != nil {
					fmt.Printf("[ERROR] %s: %v\n", path, err)
					return nil
				}
				if absPath == absOutput {
					return nil
				}

				fullPath := normalizePath(path)

				relPath, err := filepath.Rel(rootDir, path)
				if err != nil {
					return nil
				}
				relPath = normalizePath(relPath)

				if info.IsDir() {
					// User patterns: full path, also match children
					if matchesAnyPatternOrParent(fullPath, cfg.IgnoreDirs) {
						return filepath.SkipDir
					}
					// Default patterns: match anywhere
					if matchesAnyPatternOrParent(relPath, DefaultIgnoreDirs) {
						return filepath.SkipDir
					}
					// Gitignore patterns
					patterns := getGitignorePatterns(path, rootDir, gitignoreCache)
					if matchesAnyPatternOrParent(relPath, patterns) {
						return filepath.SkipDir
					}
					return nil
				}

				if writtenFiles[absPath] {
					skippedDuplicate++
					return nil
				}

				if matchesAnyPatternOrParent(fullPath, cfg.IgnoreFiles) {
					return nil
				}
				if matchesAnyPatternOrParent(relPath, DefaultIgnoreFiles) {
					return nil
				}
				patterns := getGitignorePatterns(path, rootDir, gitignoreCache)
				if matchesAnyPatternOrParent(relPath, patterns) {
					return nil
				}

				writeIfNew(absPath, path, fullPath, info)
				return nil
			})

			if err != nil {
				log.Fatalf("error walking directory '%s': %v", rootDir, err)
			}
		}
	}

	// Always print summary
	fmt.Printf("\nSummary: %d included", includedFiles)
	if skippedBinary > 0 {
		fmt.Printf(", %d binary skipped", skippedBinary)
	}
	if skippedDuplicate > 0 {
		fmt.Printf(", %d duplicates skipped", skippedDuplicate)
	}
	fmt.Println()
}

func writeFile(writer *bufio.Writer, path, displayPath string, info os.FileInfo, verbose bool) error {
	if verbose {
		fmt.Printf("[FILE] %s (%.2f KB)\n", displayPath, float64(info.Size())/1024)
	}

	fmt.Fprintf(writer, "\n%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(writer, "FILE: %s\n", displayPath)
	fmt.Fprintf(writer, "%s\n\n", strings.Repeat("=", 80))

	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(writer, "[ERROR READING FILE: %v]\n", err)
		return err
	}
	defer file.Close()

	if _, err = io.Copy(writer, file); err != nil {
		fmt.Fprintf(writer, "\n[ERROR COPYING FILE: %v]\n", err)
		return err
	}

	fmt.Fprintln(writer)
	return nil
}

func getGitignorePatterns(path, root string, cache map[string][]string) []string {
	var allPatterns []string
	dir := filepath.Dir(path)

	for {
		if patterns, exists := cache[dir]; exists {
			allPatterns = append(allPatterns, patterns...)
		} else {
			var dirPatterns []string
			for _, filename := range []string{".gitignore", ".gitignore.local"} {
				gitignorePath := filepath.Join(dir, filename)
				if patterns := loadGitignoreFile(gitignorePath, dir, root); patterns != nil {
					dirPatterns = append(dirPatterns, patterns...)
				}
			}
			cache[dir] = dirPatterns
			allPatterns = append(allPatterns, dirPatterns...)
		}

		if dir == root || dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}
	return allPatterns
}

func loadGitignoreFile(path, gitignoreDir, root string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)

	relDir, err := filepath.Rel(root, gitignoreDir)
	if err != nil {
		return nil
	}
	relDir = normalizePath(relDir)
	if relDir == "." {
		relDir = ""
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "!") {
			continue
		}

		pattern := line
		isDir := strings.HasSuffix(pattern, "/")
		if isDir {
			pattern = strings.TrimSuffix(pattern, "/")
		}

		var fullPattern string
		if strings.HasPrefix(pattern, "/") {
			// Absolute to gitignore location
			pattern = strings.TrimPrefix(pattern, "/")
			if relDir != "" {
				fullPattern = relDir + "/" + pattern
			} else {
				fullPattern = pattern
			}
		} else {
			// Relative: can match anywhere below gitignore location
			if relDir != "" {
				fullPattern = relDir + "/**/" + pattern
			} else {
				fullPattern = "**/" + pattern
			}
		}

		if fullPattern == "" {
			continue
		}

		patterns = append(patterns, fullPattern)
	}
	return patterns
}
