package repoconcat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runConcat(t *testing.T, dir string, patterns []string) string {
	t.Helper()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	output := filepath.Join(dir, "output.txt")
	if err := Concat(output, patterns, Options{Quiet: true}); err != nil {
		t.Fatalf("Concat() error = %v", err)
	}

	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertIncluded(t *testing.T, output, path string) {
	t.Helper()
	if !strings.Contains(output, "FILE: "+path) {
		t.Errorf("expected %q in output", path)
	}
}

func assertExcluded(t *testing.T, output, path string) {
	t.Helper()
	if strings.Contains(output, "FILE: "+path) {
		t.Errorf("did not expect %q in output", path)
	}
}

// =============================================================================
// Basic pattern matching
// =============================================================================

func TestIncludeSingleDirectory(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/util.go": "package main",
		"lib/lib.go":  "package lib",
		"README.md":   "readme",
	})

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/util.go")
	assertExcluded(t, output, "lib/lib.go")
	assertExcluded(t, output, "README.md")
}

func TestIncludeMultipleDirectories(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":  "package main",
		"lib/lib.go":   "package lib",
		"test/test.go": "package test",
		"docs/doc.md":  "docs",
	})

	output := runConcat(t, dir, []string{"src/**", "lib/**"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "lib/lib.go")
	assertExcluded(t, output, "test/test.go")
	assertExcluded(t, output, "docs/doc.md")
}

func TestIncludeEverything(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"lib/lib.go":  "package lib",
		"README.md":   "readme",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "lib/lib.go")
	assertIncluded(t, output, "README.md")
}

func TestIncludeSpecificFile(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/util.go": "package main",
	})

	output := runConcat(t, dir, []string{"src/main.go"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/util.go")
}

func TestIncludeByExtension(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":   "package main",
		"src/style.css": "body {}",
		"lib/lib.go":    "package lib",
		"lib/lib.js":    "console.log()",
	})

	output := runConcat(t, dir, []string{"**/*.go"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "lib/lib.go")
	assertExcluded(t, output, "src/style.css")
	assertExcluded(t, output, "lib/lib.js")
}

// =============================================================================
// Single star vs double star
// =============================================================================

func TestSingleStarDoesNotCrossDirectories(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":     "package main",
		"src/util.go":     "package main",
		"src/sub/deep.go": "package sub",
	})

	output := runConcat(t, dir, []string{"src/*.go"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/util.go")
	assertExcluded(t, output, "src/sub/deep.go")
}

func TestMiddleDoublestarPattern(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/test.go":          "root test",
		"src/sub/test.go":      "nested test",
		"src/sub/deep/test.go": "deep test",
		"src/main.go":          "main",
		"test.go":              "root level",
	})

	output := runConcat(t, dir, []string{"src/**/test.go"})

	assertIncluded(t, output, "src/test.go")
	assertIncluded(t, output, "src/sub/test.go")
	assertIncluded(t, output, "src/sub/deep/test.go")
	assertExcluded(t, output, "src/main.go")
	assertExcluded(t, output, "test.go")
}

func TestPatternWithoutSlashMatchesAnywhere(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"main.go":         "package main",
		"src/util.go":     "package util",
		"src/sub/deep.go": "package sub",
		"README.md":       "readme",
	})

	output := runConcat(t, dir, []string{"*.go"})

	assertIncluded(t, output, "main.go")
	assertIncluded(t, output, "src/util.go")
	assertIncluded(t, output, "src/sub/deep.go")
	assertExcluded(t, output, "README.md")
}

func TestLeadingSlashWithWildcardAnchorsToRoot(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"main.go":         "package main",
		"util.go":         "package util",
		"src/nested.go":   "package src",
		"src/sub/deep.go": "package sub",
	})

	output := runConcat(t, dir, []string{"/*.go"})

	assertIncluded(t, output, "main.go")
	assertIncluded(t, output, "util.go")
	assertExcluded(t, output, "src/nested.go")
	assertExcluded(t, output, "src/sub/deep.go")
}

func TestBraceExpansion(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":  "package main",
		"src/app.ts":   "typescript",
		"src/style.js": "javascript",
		"lib/lib.go":   "package lib",
		"lib/util.ts":  "typescript",
	})

	output := runConcat(t, dir, []string{"**/*.{go,ts}"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/app.ts")
	assertIncluded(t, output, "lib/lib.go")
	assertIncluded(t, output, "lib/util.ts")
	assertExcluded(t, output, "src/style.js")
}

func TestBraceExpansionWithDirectories(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":  "package main",
		"lib/lib.go":   "package lib",
		"test/test.go": "package test",
		"docs/doc.md":  "documentation",
	})

	output := runConcat(t, dir, []string{"{src,lib}/**"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "lib/lib.go")
	assertExcluded(t, output, "test/test.go")
	assertExcluded(t, output, "docs/doc.md")
}

// =============================================================================
// Trailing slash directory inclusion
// =============================================================================

func TestTrailingSlashIncludesDirectoryContents(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":     "package main",
		"src/util.go":     "package main",
		"src/sub/deep.go": "package sub",
		"lib/lib.go":      "package lib",
	})

	output := runConcat(t, dir, []string{"src/"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/util.go")
	assertIncluded(t, output, "src/sub/deep.go")
	assertExcluded(t, output, "lib/lib.go")
}

func TestTrailingSlashEquivalentToDoublestar(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":     "package main",
		"src/sub/deep.go": "package sub",
	})

	outputSlash := runConcat(t, dir, []string{"src/"})
	outputStar := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, outputSlash, "src/main.go")
	assertIncluded(t, outputSlash, "src/sub/deep.go")
	assertIncluded(t, outputStar, "src/main.go")
	assertIncluded(t, outputStar, "src/sub/deep.go")
}

// =============================================================================
// Negation (exclusion)
// =============================================================================

func TestNegationExcludesSubdirectory(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":       "package main",
		"src/util.go":       "package main",
		"src/vendor/dep.go": "package dep",
		"src/vendor/lib.go": "package lib",
	})

	output := runConcat(t, dir, []string{"src/**", "!src/vendor/**"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/util.go")
	assertExcluded(t, output, "src/vendor/dep.go")
	assertExcluded(t, output, "src/vendor/lib.go")
}

func TestNegationExcludesSpecificFile(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":      "package main",
		"src/generated.go": "package main",
	})

	output := runConcat(t, dir, []string{"src/**", "!src/generated.go"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/generated.go")
}

func TestNegationExcludesByExtension(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":     "package main",
		"src/test.go":     "package main",
		"src/data.json":   "{}",
		"src/config.json": "{}",
	})

	output := runConcat(t, dir, []string{"src/**", "!**/*.json"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/test.go")
	assertExcluded(t, output, "src/data.json")
	assertExcluded(t, output, "src/config.json")
}

func TestNegationWithoutTrailingSlashExcludesDirectory(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"lib/lib.go":  "package lib",
		"lib/util.go": "package lib",
	})

	output := runConcat(t, dir, []string{".", "!lib"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "lib/lib.go")
	assertExcluded(t, output, "lib/util.go")
}

func TestNegationWithoutTrailingSlashExcludesBothFileAndDirectory(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":  "package main",
		"build/out.go": "package build",
		"src/build":    "file named build",
	})

	output := runConcat(t, dir, []string{".", "!build"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "build/out.go")
	assertExcluded(t, output, "src/build")
}

// =============================================================================
// Last match wins
// =============================================================================

func TestLastMatchWinsReinclude(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/vendor/dep.go":  "package dep",
		"src/vendor/keep.go": "package keep",
	})

	output := runConcat(t, dir, []string{
		"src/**",
		"!src/vendor/**",
		"src/vendor/keep.go",
	})

	assertExcluded(t, output, "src/vendor/dep.go")
	assertIncluded(t, output, "src/vendor/keep.go")
}

func TestLastMatchWinsExcludeAfterInclude(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/util.go": "package main",
	})

	output := runConcat(t, dir, []string{
		"src/**",
		"src/util.go",
		"!src/util.go",
	})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/util.go")
}

func TestLastMatchWinsMultipleLayers(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/vendor/internal/secret.go": "package secret",
	})

	output := runConcat(t, dir, []string{
		"src/**",
		"!src/vendor/**",
		"src/vendor/internal/**",
		"!src/vendor/internal/secret.go",
	})

	assertExcluded(t, output, "src/vendor/internal/secret.go")
}

// =============================================================================
// Trailing slash exclusion (directory-only patterns)
// =============================================================================

func TestTrailingSlashExcludesDirectoryContents(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":  "package main",
		"build/out.go": "package build",
		"build/bin.go": "package build",
	})

	output := runConcat(t, dir, []string{".", "!build/"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "build/out.go")
	assertExcluded(t, output, "build/bin.go")
}

func TestTrailingSlashDoesNotExcludeFileWithSameName(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"build/out.go": "package build",
		"src/build":    "a file named build",
	})

	output := runConcat(t, dir, []string{".", "!build/"})

	assertIncluded(t, output, "src/build")
	assertExcluded(t, output, "build/out.go")
}

func TestTrailingSlashWithDoublestar(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"build/out.go":     "package build",
		"src/build/out.go": "nested build",
		"lib/build":        "file named build",
	})

	output := runConcat(t, dir, []string{".", "!**/build/"})

	assertExcluded(t, output, "build/out.go")
	assertExcluded(t, output, "src/build/out.go")
	assertIncluded(t, output, "lib/build")
}

// =============================================================================
// Pattern anchoring (gitignore-aligned semantics)
// =============================================================================

func TestUnanchoredPatternMatchesAnywhere(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"build":         "root build file",
		"src/build":     "nested build file",
		"src/lib/build": "deeply nested",
	})

	output := runConcat(t, dir, []string{".", "!build"})

	assertExcluded(t, output, "build")
	assertExcluded(t, output, "src/build")
	assertExcluded(t, output, "src/lib/build")
}

func TestLeadingSlashAnchorsToRoot(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"build":     "root build file",
		"src/build": "nested build file",
	})

	output := runConcat(t, dir, []string{".", "!/build"})

	assertExcluded(t, output, "build")
	assertIncluded(t, output, "src/build")
}

func TestPatternWithSlashIsAnchored(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"foo/bar":     "root foo/bar",
		"src/foo/bar": "nested foo/bar",
	})

	output := runConcat(t, dir, []string{".", "!foo/bar"})

	assertExcluded(t, output, "foo/bar")
	assertIncluded(t, output, "src/foo/bar")
}

func TestTrailingSlashOnlyMatchesDirectoryAnywhere(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"build/out.go":     "root build dir",
		"src/build/out.go": "nested build dir",
		"lib/build":        "file named build",
	})

	output := runConcat(t, dir, []string{".", "!build/"})

	assertExcluded(t, output, "build/out.go")
	assertExcluded(t, output, "src/build/out.go")
	assertIncluded(t, output, "lib/build")
}

func TestLeadingAndTrailingSlashDirectoryAtRootOnly(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"build/out.go":     "root build dir",
		"src/build/out.go": "nested build dir",
	})

	output := runConcat(t, dir, []string{".", "!/build/"})

	assertExcluded(t, output, "build/out.go")
	assertIncluded(t, output, "src/build/out.go")
}

func TestDoublestarMatchesAnywhere(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"build":     "root build file",
		"src/build": "nested build file",
	})

	output := runConcat(t, dir, []string{".", "!**/build"})

	assertExcluded(t, output, "build")
	assertExcluded(t, output, "src/build")
}

// =============================================================================
// Gitignore integration
// =============================================================================

func TestGitignoreExcludes(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/tmp.log": "log file",
		".gitignore":  "*.log\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/tmp.log")
}

func TestGitignoreNegationReincludes(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/debug.log":     "debug",
		"src/error.log":     "error",
		"src/important.log": "important",
		".gitignore":        "*.log\n!important.log\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertExcluded(t, output, "src/debug.log")
	assertExcluded(t, output, "src/error.log")
	assertIncluded(t, output, "src/important.log")
}

func TestGitignoreLocalFile(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":      "package main",
		"src/secrets.txt":  "secret data",
		".gitignore.local": "secrets.txt\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/secrets.txt")
}

func TestUserPatternsOverrideGitignore(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/tmp.log": "log file",
		".gitignore":  "*.log\n",
	})

	output := runConcat(t, dir, []string{".", "src/tmp.log"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/tmp.log")
}

func TestNestedGitignore(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":      "package main",
		"src/tmp/cache.go": "cache",
		"tmp/other.go":     "other",
		"src/.gitignore":   "tmp/\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "tmp/other.go")
	assertExcluded(t, output, "src/tmp/cache.go")
}

func TestGitignoreHierarchyConflict(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/debug.log":     "debug",
		"src/important.log": "important",
		".gitignore":        "*.log\n",
		"src/.gitignore":    "!important.log\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertExcluded(t, output, "src/debug.log")
	assertIncluded(t, output, "src/important.log")
}

func TestGitignoreAnchoredPattern(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"build":      "root build",
		"src/build":  "nested build",
		".gitignore": "/build\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertExcluded(t, output, "build")
	assertIncluded(t, output, "src/build")
}

func TestGitignorePatternWithSlashIsAnchored(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"foo/bar":     "root foo/bar",
		"src/foo/bar": "nested foo/bar",
		".gitignore":  "foo/bar\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertExcluded(t, output, "foo/bar")
	assertIncluded(t, output, "src/foo/bar")
}

func TestGitignoreAnchoredInSubdirectory(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/build":      "src build",
		"src/lib/build":  "nested under src",
		"other/build":    "other build",
		"src/.gitignore": "/build\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertExcluded(t, output, "src/build")
	assertIncluded(t, output, "src/lib/build")
	assertIncluded(t, output, "other/build")
}

func TestGitignoreUnanchoredMatchesAnywhere(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"temp":         "root temp",
		"src/temp":     "nested temp",
		"src/lib/temp": "deeply nested temp",
		".gitignore":   "temp\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertExcluded(t, output, "temp")
	assertExcluded(t, output, "src/temp")
	assertExcluded(t, output, "src/lib/temp")
}

func TestGitignoreCommentsIgnored(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/tmp.log": "log",
		".gitignore":  "# this is a comment\n*.log\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/tmp.log")
}

func TestGitignoreBlankLinesIgnored(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/tmp.log": "log",
		"src/tmp.bak": "backup",
		".gitignore":  "*.log\n\n\n*.bak\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/tmp.log")
	assertExcluded(t, output, "src/tmp.bak")
}

func TestGitignoreDoesNotExcludeFileStartingWithHash(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/#file":   "file starting with hash",
		".gitignore":  "# comment\n*.log\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/#file")
}

// =============================================================================
// Default excludes
// =============================================================================

func TestDefaultExcludesNodeModules(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.js":                   "console.log()",
		"src/node_modules/dep/index.js": "module.exports = {}",
	})

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.js")
	assertExcluded(t, output, "src/node_modules/dep/index.js")
}

func TestDefaultExcludesGitDirectory(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":     "package main",
		"src/.git/config": "git config",
		"src/.git/HEAD":   "ref: refs/heads/main",
	})

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/.git/config")
	assertExcluded(t, output, "src/.git/HEAD")
}

func TestDefaultExcludesLockfiles(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":           "package main",
		"src/go.sum":            "checksum",
		"src/package-lock.json": "{}",
		"src/yarn.lock":         "lockfile",
	})

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/go.sum")
	assertExcluded(t, output, "src/package-lock.json")
	assertExcluded(t, output, "src/yarn.lock")
}

func TestDefaultExcludesApplyWithoutGitignore(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":               "package main",
		"node_modules/dep/index.js": "module.exports = {}",
		".git/config":               "git config",
		"go.sum":                    "checksum",
	})
	os.Remove(filepath.Join(dir, ".gitignore"))

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "node_modules/dep/index.js")
	assertExcluded(t, output, ".git/config")
	assertExcluded(t, output, "go.sum")
}

func TestUserCanOverrideDefaultExcludesFile(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/go.sum":  "checksum",
	})

	output := runConcat(t, dir, []string{"src/**", "src/go.sum"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/go.sum")
}

func TestUserCanOverrideDefaultExcludesDirectory(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.js":               "console.log()",
		"node_modules/dep/index.js": "module.exports = {}",
		"node_modules/dep/util.js":  "exports.util = {}",
	})

	output := runConcat(t, dir, []string{".", "node_modules/**"})

	assertIncluded(t, output, "src/main.js")
	assertIncluded(t, output, "node_modules/dep/index.js")
	assertIncluded(t, output, "node_modules/dep/util.js")
}

func TestUserCanOverrideDefaultExcludesDirectoryWithTrailingSlash(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.js":               "console.log()",
		"node_modules/dep/index.js": "module.exports = {}",
	})

	output := runConcat(t, dir, []string{".", "node_modules/"})

	assertIncluded(t, output, "src/main.js")
	assertIncluded(t, output, "node_modules/dep/index.js")
}

func TestUserCanOverrideDefaultExcludesSpecificFileInExcludedDir(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.js":               "console.log()",
		"node_modules/dep/index.js": "module.exports = {}",
		"node_modules/dep/util.js":  "exports.util = {}",
	})

	output := runConcat(t, dir, []string{".", "node_modules/dep/index.js"})

	assertIncluded(t, output, "src/main.js")
	assertIncluded(t, output, "node_modules/dep/index.js")
	assertExcluded(t, output, "node_modules/dep/util.js")
}

// =============================================================================
// Binary files
// =============================================================================

func TestBinaryFilesSkipped(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})
	binaryPath := filepath.Join(dir, "src/image.png")
	os.WriteFile(binaryPath, []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x00}, 0644)

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/image.png")
}

func TestBinaryFilesWithVariousMagicBytes(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})
	os.WriteFile(filepath.Join(dir, "image.png"), []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, 0644)
	os.WriteFile(filepath.Join(dir, "image.jpg"), []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}, 0644)
	os.WriteFile(filepath.Join(dir, "image.gif"), []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, 0644)
	os.WriteFile(filepath.Join(dir, "doc.pdf"), []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E, 0x34}, 0644)
	os.WriteFile(filepath.Join(dir, "binary"), []byte{0x7F, 0x45, 0x4C, 0x46, 0x00, 0x00, 0x00, 0x00}, 0644)

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "image.png")
	assertExcluded(t, output, "image.jpg")
	assertExcluded(t, output, "image.gif")
	assertExcluded(t, output, "doc.pdf")
	assertExcluded(t, output, "binary")
}

// =============================================================================
// Symlinks
// =============================================================================

func TestSymlinksToFilesSkipped(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/util.go": "package main",
	})

	linkPath := filepath.Join(dir, "src/link.go")
	targetPath := filepath.Join(dir, "src/util.go")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/util.go")
	assertExcluded(t, output, "src/link.go")
}

func TestSymlinksToDirectoriesSkipped(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"lib/lib.go":  "package lib",
	})

	linkPath := filepath.Join(dir, "src/liblink")
	targetPath := filepath.Join(dir, "lib")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/liblink/lib.go")
}

func TestSymlinkToParentDirectorySkipped(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})

	linkPath := filepath.Join(dir, "src/parent")
	if err := os.Symlink(dir, linkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.go")
	if strings.Count(output, "FILE: src/main.go") > 1 {
		t.Error("file included multiple times, possible symlink loop")
	}
}

// =============================================================================
// Edge cases
// =============================================================================

func TestEmptyPatternListProducesNoOutput(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})

	output := runConcat(t, dir, []string{})

	assertExcluded(t, output, "src/main.go")
	if strings.Contains(output, "FILE:") {
		t.Error("expected no files in output for empty pattern list")
	}
}

func TestDeeplyNestedStructure(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"a/b/c/d/e/f.go": "package f",
		"a/b/c/d/e/g.go": "package g",
		"a/b/x.go":       "package x",
	})

	output := runConcat(t, dir, []string{"a/**", "!a/b/c/**", "a/b/c/d/e/f.go"})

	assertIncluded(t, output, "a/b/x.go")
	assertIncluded(t, output, "a/b/c/d/e/f.go")
	assertExcluded(t, output, "a/b/c/d/e/g.go")
}

func TestEmptyDirectoriesIgnored(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})
	os.MkdirAll(filepath.Join(dir, "empty"), 0755)

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
}

func TestNoMatchingFiles(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})

	output := runConcat(t, dir, []string{"nonexistent/**"})

	assertExcluded(t, output, "src/main.go")
	if strings.Contains(output, "FILE:") {
		t.Error("expected no files in output")
	}
}

func TestHiddenFilesIncludedByDefault(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/.hidden": "hidden file",
		".config":     "config",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/.hidden")
	assertIncluded(t, output, ".config")
}

func TestHiddenFilesCanBeExcluded(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"src/.hidden": "hidden file",
		".config":     "config",
	})

	output := runConcat(t, dir, []string{".", "!.*", "!**/.*"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/.hidden")
	assertExcluded(t, output, ".config")
}

func TestSpecialCharactersInFilenames(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":         "package main",
		"src/file with space": "spaces",
		"src/file-with-dash":  "dashes",
		"src/file_with_under": "underscores",
	})

	output := runConcat(t, dir, []string{"src/**"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/file with space")
	assertIncluded(t, output, "src/file-with-dash")
	assertIncluded(t, output, "src/file_with_under")
}

func TestSingleFileAsOnlyPattern(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"README.md":   "readme content",
		"src/main.go": "package main",
	})

	output := runConcat(t, dir, []string{"README.md"})

	assertIncluded(t, output, "README.md")
	assertExcluded(t, output, "src/main.go")
}

func TestMultipleExtensionPatterns(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":    "package main",
		"src/app.ts":     "typescript",
		"src/style.css":  "css",
		"src/index.html": "html",
	})

	output := runConcat(t, dir, []string{"**/*.go", "**/*.ts"})

	assertIncluded(t, output, "src/main.go")
	assertIncluded(t, output, "src/app.ts")
	assertExcluded(t, output, "src/style.css")
	assertExcluded(t, output, "src/index.html")
}

// =============================================================================
// Interaction between gitignore and user patterns
// =============================================================================

func TestGitignoreAndUserPatternsLastMatchWins(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":  "package main",
		"src/tmp.log":  "log",
		"src/keep.log": "keep",
		".gitignore":   "*.log\n",
	})

	output := runConcat(t, dir, []string{"src/**", "src/keep.log"})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/tmp.log")
	assertIncluded(t, output, "src/keep.log")
}

func TestUserExclusionOverridesGitignoreInclusion(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/important.log": "important",
		".gitignore":        "*.log\n!important.log\n",
	})

	output := runConcat(t, dir, []string{".", "!src/important.log"})

	assertExcluded(t, output, "src/important.log")
}

func TestComplexGitignoreUserPatternInteraction(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":        "package main",
		"src/vendor/dep.go":  "vendor dep",
		"src/vendor/keep.go": "keep this",
		"build/out":          "build output",
		".gitignore":         "vendor/\nbuild/\n",
	})

	output := runConcat(t, dir, []string{
		".",
		"src/vendor/keep.go",
		"!build/",
	})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, "src/vendor/dep.go")
	assertIncluded(t, output, "src/vendor/keep.go")
	assertExcluded(t, output, "build/out")
}

// =============================================================================
// MustConcat
// =============================================================================

func TestMustConcatPanicsOnError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustConcat did not panic on error")
		}
	}()

	MustConcat("/nonexistent/path/output.txt", []string{"src/**"}, Options{Quiet: true})
}

func TestMustConcatSucceeds(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustConcat panicked unexpectedly: %v", r)
		}
	}()

	MustConcat(filepath.Join(dir, "output.txt"), []string{"src/**"}, Options{Quiet: true})
}

// =============================================================================
// Additional coverage
// =============================================================================

func TestDoublestarEquivalentToDot(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
		"lib/lib.go":  "package lib",
		"README.md":   "readme",
	})

	outputDot := runConcat(t, dir, []string{"."})
	outputStar := runConcat(t, dir, []string{"**"})

	assertIncluded(t, outputDot, "src/main.go")
	assertIncluded(t, outputDot, "lib/lib.go")
	assertIncluded(t, outputDot, "README.md")

	assertIncluded(t, outputStar, "src/main.go")
	assertIncluded(t, outputStar, "lib/lib.go")
	assertIncluded(t, outputStar, "README.md")
}

func TestOutputFileExcludedFromResults(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	output := filepath.Join(dir, "src/output.txt")
	if err := Concat(output, []string{"src/**"}, Options{Quiet: true}); err != nil {
		t.Fatalf("Concat() error = %v", err)
	}

	content, _ := os.ReadFile(output)
	result := string(content)

	assertIncluded(t, result, "src/main.go")
	assertExcluded(t, result, "src/output.txt")
}

func TestFileContentActuallyWritten(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main\n\nfunc main() {}\n",
	})

	output := runConcat(t, dir, []string{"src/**"})

	if !strings.Contains(output, "func main() {}") {
		t.Error("expected file content in output")
	}
}

func TestGitignoreFileItselfExcluded(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go":      "package main",
		".gitignore":       "*.log\n",
		".gitignore.local": "secrets\n",
	})

	output := runConcat(t, dir, []string{"."})

	assertIncluded(t, output, "src/main.go")
	assertExcluded(t, output, ".gitignore")
	assertExcluded(t, output, ".gitignore.local")
}

func TestOutputFileWithDifferentExtension(t *testing.T) {
	dir := setupTestDir(t, map[string]string{
		"src/main.go": "package main",
	})

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	output := filepath.Join(dir, "context.md")
	if err := Concat(output, []string{"."}, Options{Quiet: true}); err != nil {
		t.Fatalf("Concat() error = %v", err)
	}

	content, _ := os.ReadFile(output)
	result := string(content)

	assertIncluded(t, result, "src/main.go")
	assertExcluded(t, result, "context.md")
}
