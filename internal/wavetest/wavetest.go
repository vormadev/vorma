// Package wavetest centralizes shared test fixtures for waveconfig.ParsedConfig and
// related helpers used across Go test packages in this repository.
package wavetest

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/vormadev/vorma/wave/waveconfig"
)

// NewDiscardLogger returns a logger that discards all output.
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// NewParsedConfigAtRoot builds a default ParsedConfig rooted at root.
func NewParsedConfigAtRoot(root string) *waveconfig.ParsedConfig {
	distDir := MustCWDRelativePath(filepath.Join(root, "dist"))
	cfg := &waveconfig.ParsedConfig{
		Core: &waveconfig.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      distDir,
		},
		Watch: &waveconfig.WatchConfig{
			WatchRoot: MustCWDRelativePath(root),
		},
	}
	SetStaticAssetDirectories(
		cfg,
		filepath.Join(root, "static", "public"),
		filepath.Join(root, "static", "private"),
	)
	cfg.Dist.Root = filepath.Clean(distDir)
	return cfg
}

// SetStaticAssetDirectories sets cfg.Core.StaticAssetDirs using explicit
// directory paths.
func SetStaticAssetDirectories(
	cfg *waveconfig.ParsedConfig,
	publicDir string,
	privateDir string,
) {
	if cfg == nil || cfg.Core == nil {
		panic(
			"wavetest.SetStaticAssetDirectories requires non-nil cfg and cfg.Core",
		)
	}
	SetCoreStaticAssetDirectories(cfg.Core, publicDir, privateDir)
}

// SetCoreStaticAssetDirectories sets core.StaticAssetDirs using explicit
// directory paths.
func SetCoreStaticAssetDirectories(
	core *waveconfig.CoreConfig,
	publicDir string,
	privateDir string,
) {
	if core == nil {
		panic("wavetest.SetCoreStaticAssetDirectories requires non-nil core")
	}
	core.StaticAssetDirs = struct {
		Private string `json:"Private"`
		Public  string `json:"Public"`
	}{
		Public:  MustCWDRelativePath(publicDir),
		Private: MustCWDRelativePath(privateDir),
	}
}

// SetCSSEntryFiles sets cfg.Core.CSSEntryFiles using explicit critical and
// non-critical CSS entry paths.
func SetCSSEntryFiles(
	cfg *waveconfig.ParsedConfig,
	criticalEntry string,
	nonCriticalEntry string,
) {
	if cfg == nil || cfg.Core == nil {
		panic("wavetest.SetCSSEntryFiles requires non-nil cfg and cfg.Core")
	}
	SetCoreCSSEntryFiles(cfg.Core, criticalEntry, nonCriticalEntry)
}

// SetCoreCSSEntryFiles sets core.CSSEntryFiles using explicit critical and
// non-critical CSS entry paths.
func SetCoreCSSEntryFiles(
	core *waveconfig.CoreConfig,
	criticalEntry string,
	nonCriticalEntry string,
) {
	if core == nil {
		panic("wavetest.SetCoreCSSEntryFiles requires non-nil core")
	}
	core.CSSEntryFiles = struct {
		Critical    string `json:"Critical,omitempty"`
		NonCritical string `json:"NonCritical,omitempty"`
	}{
		Critical:    MustCWDRelativePath(criticalEntry),
		NonCritical: MustCWDRelativePath(nonCriticalEntry),
	}
}

// MustCWDRelativePath converts one absolute filesystem path into a path
// relative to the current working directory. Relative inputs are cleaned and
// returned unchanged.
func MustCWDRelativePath(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}
	if !filepath.IsAbs(trimmedPath) {
		return filepath.Clean(trimmedPath)
	}

	currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
	if currentWorkingDirectoryError != nil {
		panic(
			fmt.Sprintf(
				"wavetest.MustCWDRelativePath: resolve current working directory: %v",
				currentWorkingDirectoryError,
			),
		)
	}

	relativePath, relativePathError := filepath.Rel(
		currentWorkingDirectory,
		trimmedPath,
	)
	if relativePathError != nil {
		panic(
			fmt.Sprintf(
				"wavetest.MustCWDRelativePath: make %q relative to cwd %q: %v",
				trimmedPath,
				currentWorkingDirectory,
				relativePathError,
			),
		)
	}

	return filepath.Clean(relativePath)
}

// NewWorkspaceTempDir allocates a temporary directory under the current
// workspace-local test root so tests can use CWD-relative config paths while
// keeping cleanup deterministic.
func NewWorkspaceTempDir(tb testing.TB, prefix string) string {
	tb.Helper()

	workspaceTempRoot := filepath.Join(".", ".local.testtmp")
	workspaceTempRootError := os.MkdirAll(workspaceTempRoot, 0o755)
	if workspaceTempRootError != nil {
		tb.Fatalf(
			"wavetest.NewWorkspaceTempDir: create workspace temp root: %v",
			workspaceTempRootError,
		)
	}
	currentProcessID := os.Getpid()
	cleanupWorkspaceTempProcessRoots(workspaceTempRoot, currentProcessID)
	currentProcessRoot := filepath.Join(
		workspaceTempRoot,
		fmt.Sprintf("pid-%d", currentProcessID),
	)
	currentProcessRootError := os.MkdirAll(currentProcessRoot, 0o755)
	if currentProcessRootError != nil {
		tb.Fatalf(
			"wavetest.NewWorkspaceTempDir: create current process temp root: %v",
			currentProcessRootError,
		)
	}

	temporaryDirectory, temporaryDirectoryError := os.MkdirTemp(
		currentProcessRoot,
		workspaceTempPrefixWithLocalIgnoreMarker(prefix),
	)
	if temporaryDirectoryError != nil {
		tb.Fatalf(
			"wavetest.NewWorkspaceTempDir: create temporary directory: %v",
			temporaryDirectoryError,
		)
	}
	absoluteTemporaryDirectory := temporaryDirectory
	if !filepath.IsAbs(absoluteTemporaryDirectory) {
		resolvedTemporaryDirectory, resolvedTemporaryDirectoryError := filepath.Abs(
			absoluteTemporaryDirectory,
		)
		if resolvedTemporaryDirectoryError != nil {
			tb.Fatalf(
				"wavetest.NewWorkspaceTempDir: resolve absolute path for %q: %v",
				temporaryDirectory,
				resolvedTemporaryDirectoryError,
			)
		}
		absoluteTemporaryDirectory = resolvedTemporaryDirectory
	}
	tb.Cleanup(func() {
		_ = os.RemoveAll(absoluteTemporaryDirectory)
		removeDirectoryIfEmpty(currentProcessRoot)
		removeDirectoryIfEmpty(workspaceTempRoot)
	})

	return absoluteTemporaryDirectory
}

func workspaceTempPrefixWithLocalIgnoreMarker(prefix string) string {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return "workspace.local.temp-"
	}
	if strings.Contains(trimmedPrefix, ".local.") {
		return trimmedPrefix
	}
	trimmedPrefix = strings.TrimSuffix(trimmedPrefix, "-")
	return trimmedPrefix + ".local.temp-"
}

func cleanupWorkspaceTempProcessRoots(
	workspaceTempRoot string,
	currentProcessID int,
) {
	rootEntries, rootEntriesError := os.ReadDir(workspaceTempRoot)
	if rootEntriesError != nil {
		return
	}
	for _, rootEntry := range rootEntries {
		if !rootEntry.IsDir() {
			continue
		}
		entryName := rootEntry.Name()
		if !strings.HasPrefix(entryName, "pid-") {
			continue
		}
		entryPIDText := strings.TrimPrefix(entryName, "pid-")
		entryPID, entryPIDError := strconv.Atoi(entryPIDText)
		if entryPIDError != nil {
			continue
		}
		if entryPID <= 0 || entryPID == currentProcessID {
			continue
		}
		if isProcessRunning(entryPID) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(workspaceTempRoot, entryName))
	}
}

func isProcessRunning(processID int) bool {
	if processID <= 0 {
		return false
	}
	killError := syscall.Kill(processID, 0)
	return killError == nil || killError == syscall.EPERM
}

func removeDirectoryIfEmpty(directoryPath string) {
	directoryEntries, directoryEntriesError := os.ReadDir(directoryPath)
	if directoryEntriesError != nil {
		return
	}
	if len(directoryEntries) != 0 {
		return
	}
	_ = os.Remove(directoryPath)
}

// EnsureViteConfig allocates cfg.Vite when tests require a non-nil Vite
// section and the fixture omitted it.
func EnsureViteConfig(
	tb testing.TB,
	cfg *waveconfig.ParsedConfig,
) {
	tb.Helper()
	if cfg == nil {
		tb.Fatal("expected non-nil config")
		return
	}
	if cfg.Vite != nil {
		return
	}

	cfgValue := reflect.ValueOf(cfg).Elem()
	viteField := cfgValue.FieldByName("Vite")
	if !viteField.IsValid() || !viteField.CanSet() || !viteField.IsNil() {
		tb.Fatal("expected settable nil Vite field")
		return
	}
	viteField.Set(reflect.New(viteField.Type().Elem()))
}
