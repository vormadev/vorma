// buyer beware
package repoconcat

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
)

type Config struct {
	Output  string
	Include []string
	Exclude []string
	Quiet   bool
}

var defaultExclude = []string{
	"**/node_modules/**",
	"**/.git/**",
	"**/.vscode/**",
	"**/.DS_Store",
	"**/.gitignore",
	"**/.gitignore.local",
	"**/*.svg",
	"**/go.sum",
	"**/package-lock.json",
	"**/yarn.lock",
	"**/pnpm-lock.yaml",
	"**/bun.lockb",
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

func Concat(cfg Config) error {
	outFile, err := os.Create(cfg.Output)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	defer writer.Flush()

	absOutput, _ := filepath.Abs(cfg.Output)
	allExclude := append(defaultExclude, cfg.Exclude...)

	var includedFiles, skippedBinary int
	seen := make(map[string]bool)

	for _, pattern := range cfg.Include {
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			if !cfg.Quiet {
				fmt.Printf("[WARN] invalid pattern %q: %v\n", pattern, err)
			}
			continue
		}

		for _, match := range matches {
			if err := processPath(match, absOutput, allExclude, seen, writer, cfg.Quiet, &includedFiles, &skippedBinary); err != nil {
				return err
			}
		}
	}

	if !cfg.Quiet {
		fmt.Printf("\nSummary: %d files included, %d binary files skipped\n", includedFiles, skippedBinary)
	}
	return nil
}

func processPath(path, absOutput string, exclude []string, seen map[string]bool, writer *bufio.Writer, quiet bool, included, skippedBinary *int) error {
	absPath, _ := filepath.Abs(path)
	if absPath == absOutput {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	if info.IsDir() {
		return filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				relPath := filepath.ToSlash(p)
				for _, ex := range exclude {
					if matched, _ := doublestar.Match(ex, relPath); matched {
						return filepath.SkipDir
					}
					if matched, _ := doublestar.Match(ex, relPath+"/"); matched {
						return filepath.SkipDir
					}
				}
				return nil
			}
			return processFile(p, absOutput, exclude, seen, writer, quiet, included, skippedBinary)
		})
	}

	return processFile(path, absOutput, exclude, seen, writer, quiet, included, skippedBinary)
}

func processFile(path, absOutput string, exclude []string, seen map[string]bool, writer *bufio.Writer, quiet bool, included, skippedBinary *int) error {
	absPath, _ := filepath.Abs(path)
	if absPath == absOutput || seen[absPath] {
		return nil
	}
	seen[absPath] = true

	relPath := filepath.ToSlash(path)

	for _, ex := range exclude {
		if matched, _ := doublestar.Match(ex, relPath); matched {
			return nil
		}
	}

	if isGitignored(path, relPath) {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	if !isTextFile(path) {
		*skippedBinary++
		return nil
	}

	if err := writeFile(writer, path, relPath, info, quiet); err == nil {
		*included++
	}
	return nil
}

func isGitignored(absPath, relPath string) bool {
	dir := filepath.Dir(absPath)
	patterns := collectGitignorePatterns(dir)
	for _, pattern := range patterns {
		if matched, _ := doublestar.Match(pattern, relPath); matched {
			return true
		}
	}
	return false
}

func collectGitignorePatterns(dir string) []string {
	var patterns []string
	for {
		for _, name := range []string{".gitignore", ".gitignore.local"} {
			if p := loadGitignoreFile(filepath.Join(dir, name)); p != nil {
				patterns = append(patterns, p...)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return patterns
}

func loadGitignoreFile(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		pattern := line
		isDir := strings.HasSuffix(pattern, "/")
		if isDir {
			pattern = strings.TrimSuffix(pattern, "/")
		}

		if strings.HasPrefix(pattern, "/") {
			pattern = strings.TrimPrefix(pattern, "/")
		} else {
			pattern = "**/" + pattern
		}

		if isDir {
			patterns = append(patterns, pattern+"/**")
		}
		patterns = append(patterns, pattern)
	}
	return patterns
}

func writeFile(writer *bufio.Writer, path, displayPath string, info os.FileInfo, quiet bool) error {
	if !quiet {
		fmt.Printf("[FILE] %s (%.2f KB)\n", displayPath, float64(info.Size())/1024)
	}

	fmt.Fprintf(writer, "\n%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(writer, "FILE: %s\n", displayPath)
	fmt.Fprintf(writer, "%s\n\n", strings.Repeat("=", 80))

	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(writer, "[ERROR READING FILE: %v]\n", err)
		if !quiet {
			fmt.Printf("  ERROR: Could not read file: %v\n", err)
		}
		return err
	}
	defer file.Close()

	if _, err = io.Copy(writer, file); err != nil {
		fmt.Fprintf(writer, "\n[ERROR COPYING FILE: %v]\n", err)
		if !quiet {
			fmt.Printf("  ERROR: Could not copy file: %v\n", err)
		}
		return err
	}

	fmt.Fprintln(writer)
	return nil
}
