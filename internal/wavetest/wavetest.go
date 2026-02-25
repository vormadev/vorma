// Package wavetest centralizes shared test fixtures for wave.ParsedConfig and
// related helpers used across Go test packages in this repository.
package wavetest

import (
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/wave"
)

// NewDiscardLogger returns a logger that discards all output.
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// NewParsedConfigAtRoot builds a default ParsedConfig rooted at root.
func NewParsedConfigAtRoot(root string) *wave.ParsedConfig {
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	SetStaticAssetDirectories(
		cfg,
		filepath.Join(root, "static", "public"),
		filepath.Join(root, "static", "private"),
	)
	cfg.Dist.Root = cfg.Core.DistDir
	return cfg
}

// SetStaticAssetDirectories sets cfg.Core.StaticAssetDirs using explicit
// directory paths.
func SetStaticAssetDirectories(
	cfg *wave.ParsedConfig,
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
	core *wave.CoreConfig,
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
		Public:  publicDir,
		Private: privateDir,
	}
}

// SetCSSEntryFiles sets cfg.Core.CSSEntryFiles using explicit critical and
// non-critical CSS entry paths.
func SetCSSEntryFiles(
	cfg *wave.ParsedConfig,
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
	core *wave.CoreConfig,
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
		Critical:    criticalEntry,
		NonCritical: nonCriticalEntry,
	}
}

// EnsureViteConfig allocates cfg.Vite when tests require a non-nil Vite
// section and the fixture omitted it.
func EnsureViteConfig(
	tb testing.TB,
	cfg *wave.ParsedConfig,
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
