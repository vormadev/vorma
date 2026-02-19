package pathnorm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/wave/internal/waveshared"
)

// __TODO get rid of these pointless wrappers. totally inappropriate and wasteful indirection.

func TrimAndCleanPath(path string) string {
	return waveshared.TrimAndCleanPath(path)
}

func Absolute(path string) string {
	return waveshared.Absolute(path)
}

func AbsoluteSlash(path string) string {
	return waveshared.AbsoluteSlash(path)
}

func AbsoluteDirectory(path string) string {
	return waveshared.AbsoluteDirectory(path)
}

func PathsReferToSameLocation(pathA string, pathB string) bool {
	return waveshared.PathsReferToSameLocation(pathA, pathB)
}

func CanonicalizePathForLocationComparison(path string) string {
	return waveshared.CanonicalizePathForLocationComparison(path)
}

func TestTrimAndCleanPath(t *testing.T) {
	t.Run(
		"empty and whitespace-only paths normalize to empty string",
		func(t *testing.T) {
			if got := TrimAndCleanPath(""); got != "" {
				t.Fatalf("TrimAndCleanPath(\"\") = %q, want empty", got)
			}
			if got := TrimAndCleanPath("   "); got != "" {
				t.Fatalf("TrimAndCleanPath(whitespace) = %q, want empty", got)
			}
		},
	)

	t.Run(
		"relative path is trimmed and cleaned without absolutizing",
		func(t *testing.T) {
			rawRelativePath := "  ./backend/./main.go  "
			wantRelativePath := filepath.Clean("./backend/main.go")
			if got := TrimAndCleanPath(rawRelativePath); got != wantRelativePath {
				t.Fatalf(
					"TrimAndCleanPath(%q) = %q, want %q",
					rawRelativePath,
					got,
					wantRelativePath,
				)
			}
		},
	)

	t.Run("absolute path is trimmed and cleaned", func(t *testing.T) {
		root := t.TempDir()
		rawAbsolutePath := "  " + filepath.Join(
			root,
			"backend",
			".",
			"main.go",
		) + "  "
		wantAbsolutePath := filepath.Join(root, "backend", "main.go")
		if got := TrimAndCleanPath(rawAbsolutePath); got != wantAbsolutePath {
			t.Fatalf(
				"TrimAndCleanPath(%q) = %q, want %q",
				rawAbsolutePath,
				got,
				wantAbsolutePath,
			)
		}
	})
}

func TestAbsoluteDirectory(t *testing.T) {
	root := t.TempDir()
	configDirectoryPath := filepath.Join(root, "backend")
	configFilePath := filepath.Join(configDirectoryPath, "wave.config.json")
	missingConfigPath := filepath.Join(
		configDirectoryPath,
		"missing.config.json",
	)

	if err := os.MkdirAll(configDirectoryPath, 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(configFilePath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	if got := AbsoluteDirectory(configDirectoryPath); got != configDirectoryPath {
		t.Fatalf(
			"AbsoluteDirectory(existing dir) = %q, want %q",
			got,
			configDirectoryPath,
		)
	}

	if got := AbsoluteDirectory(configFilePath); got != configDirectoryPath {
		t.Fatalf(
			"AbsoluteDirectory(existing file) = %q, want %q",
			got,
			configDirectoryPath,
		)
	}

	if got := AbsoluteDirectory(missingConfigPath); got != configDirectoryPath {
		t.Fatalf(
			"AbsoluteDirectory(missing path) = %q, want %q",
			got,
			configDirectoryPath,
		)
	}
}

func TestAbsoluteSlash(t *testing.T) {
	t.Run(
		"empty and whitespace-only paths normalize to empty",
		func(t *testing.T) {
			if got := AbsoluteSlash(""); got != "" {
				t.Fatalf("AbsoluteSlash(\"\") = %q, want empty", got)
			}
			if got := AbsoluteSlash("   "); got != "" {
				t.Fatalf("AbsoluteSlash(whitespace) = %q, want empty", got)
			}
		},
	)

	t.Run(
		"absolute path is normalized with forward slashes",
		func(t *testing.T) {
			root := t.TempDir()
			rawPath := "  " + filepath.Join(
				root,
				"backend",
				".",
				"wave.config.json",
			) + "  "
			wantPath := filepath.ToSlash(
				filepath.Join(root, "backend", "wave.config.json"),
			)
			if got := AbsoluteSlash(rawPath); got != wantPath {
				t.Fatalf(
					"AbsoluteSlash(%q) = %q, want %q",
					rawPath,
					got,
					wantPath,
				)
			}
		},
	)
}

func TestPathsReferToSameLocation(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")
	equivalentConfigPath := filepath.Join(
		root,
		"backend",
		".",
		"wave.config.json",
	)
	otherConfigPath := filepath.Join(root, "backend", "other.config.json")

	if !PathsReferToSameLocation(configPath, equivalentConfigPath) {
		t.Fatalf(
			"expected %q and %q to refer to same location",
			configPath,
			equivalentConfigPath,
		)
	}

	if PathsReferToSameLocation(configPath, otherConfigPath) {
		t.Fatalf(
			"did not expect %q and %q to refer to same location",
			configPath,
			otherConfigPath,
		)
	}

	if PathsReferToSameLocation("", equivalentConfigPath) {
		t.Fatal("expected empty path to fail same-location comparison")
	}
}

func TestPathsReferToSameLocation_FollowsSymlinkAliases(t *testing.T) {
	root := t.TempDir()
	targetDirectoryPath := filepath.Join(root, "target")
	aliasDirectoryPath := filepath.Join(root, "alias")
	targetFilePath := filepath.Join(targetDirectoryPath, "styles.css")
	aliasFilePath := filepath.Join(aliasDirectoryPath, "styles.css")

	if err := os.MkdirAll(targetDirectoryPath, 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	if err := os.WriteFile(targetFilePath, []byte("body{color:red;}"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("create directory symlink: %v", err)
	}

	if !PathsReferToSameLocation(targetFilePath, aliasFilePath) {
		t.Fatalf(
			"expected symlink aliases %q and %q to refer to same location",
			targetFilePath,
			aliasFilePath,
		)
	}
}

func TestPathsReferToSameLocation_FollowsSymlinkAliasesForMissingLeaf(
	t *testing.T,
) {
	root := t.TempDir()
	targetDirectoryPath := filepath.Join(root, "target")
	aliasDirectoryPath := filepath.Join(root, "alias")
	targetMissingFilePath := filepath.Join(targetDirectoryPath, "missing.css")
	aliasMissingFilePath := filepath.Join(aliasDirectoryPath, "missing.css")

	if err := os.MkdirAll(targetDirectoryPath, 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("create directory symlink: %v", err)
	}

	if !PathsReferToSameLocation(targetMissingFilePath, aliasMissingFilePath) {
		t.Fatalf(
			"expected missing-file symlink aliases %q and %q to refer to same location",
			targetMissingFilePath,
			aliasMissingFilePath,
		)
	}
}

func TestCanonicalizePathForLocationComparison(t *testing.T) {
	t.Run("empty path returns empty canonical path", func(t *testing.T) {
		if got := CanonicalizePathForLocationComparison(""); got != "" {
			t.Fatalf(
				"expected empty canonical path for empty input, got %q",
				got,
			)
		}
	})

	t.Run(
		"missing leaf under symlink resolves through parent directory",
		func(t *testing.T) {
			root := t.TempDir()
			targetDirectoryPath := filepath.Join(root, "target")
			aliasDirectoryPath := filepath.Join(root, "alias")
			targetMissingFilePath := filepath.Join(
				targetDirectoryPath,
				"missing.css",
			)
			aliasMissingFilePath := filepath.Join(
				aliasDirectoryPath,
				"missing.css",
			)

			if err := os.MkdirAll(targetDirectoryPath, 0o755); err != nil {
				t.Fatalf("create target directory: %v", err)
			}
			if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
				t.Fatalf("create alias directory symlink: %v", err)
			}

			canonicalizedAliasPath := CanonicalizePathForLocationComparison(
				aliasMissingFilePath,
			)
			if canonicalizedAliasPath == "" {
				t.Fatalf(
					"expected non-empty canonical path for alias path %q",
					aliasMissingFilePath,
				)
			}
			if !PathsReferToSameLocation(
				canonicalizedAliasPath,
				targetMissingFilePath,
			) {
				t.Fatalf(
					"expected canonical path %q to refer to target missing path %q",
					canonicalizedAliasPath,
					targetMissingFilePath,
				)
			}
		},
	)
}
