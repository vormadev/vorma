// Package wave2 is the app-facing runtime API for Wave2 applications.
//
// This package intentionally excludes build/dev orchestration concerns so
// production applications keep a lean runtime dependency surface.
package wave2

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
)

// RuntimeConfigPath is the fixed runtime-config artifact path relative to
// dist-static root.
const RuntimeConfigPath = "wave_owned/runtime_cfg.json"

// RuntimeConfig is the generated runtime-config artifact loaded by Wave2.
type RuntimeConfig struct {
	PublicPathPrefix string `json:"public_path_prefix"`
}

// Wave is the app-facing runtime surface.
type Wave struct {
	dist_static_fs fs.FS
	logger         *slog.Logger
	runtime_cfg    RuntimeConfig
}

// Options configures Wave2 runtime construction from generated dist artifacts.
type Options struct {
	// DistStaticFS is the dist static filesystem root containing generated
	// Wave2 runtime artifacts.
	//
	// In production, you probably want to use `embed.FS`.
	// In development, leave this field null (the dev server doesn't even use it,
	// and you can improve speed by avoid the `embed.FS` compilation overhead).
	DistStaticFS fs.FS

	// Logger is the optional runtime logger used by Wave2.
	Logger *slog.Logger
}

// New constructs one Wave2 runtime instance from generated dist artifacts.
func New(opts Options) *Wave {
	if opts.DistStaticFS == nil {
		panic("wave2.New: Options.DistStaticFS is required")
	}

	runtime_cfg, err := read_runtime_cfg(opts.DistStaticFS)
	if err != nil {
		panic("wave2.New: " + err.Error())
	}

	return &Wave{
		dist_static_fs: opts.DistStaticFS,
		logger:         opts.Logger,
		runtime_cfg:    runtime_cfg,
	}
}

// Logger returns the configured logger.
func (w *Wave) Logger() *slog.Logger {
	if w == nil {
		return nil
	}
	return w.logger
}

// PublicPathPrefix returns the generated public path prefix.
func (w *Wave) PublicPathPrefix() string {
	if w == nil {
		return ""
	}
	return w.runtime_cfg.PublicPathPrefix
}

func read_runtime_cfg(dist_static_fs fs.FS) (RuntimeConfig, error) {
	runtime_cfg_json, err := fs.ReadFile(
		dist_static_fs,
		RuntimeConfigPath,
	)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf(
			"read runtime config %q: %w",
			RuntimeConfigPath,
			err,
		)
	}

	var runtime_cfg RuntimeConfig
	if err := json.Unmarshal(
		runtime_cfg_json,
		&runtime_cfg,
	); err != nil {
		return RuntimeConfig{}, fmt.Errorf(
			"parse runtime config %q: %w",
			RuntimeConfigPath,
			err,
		)
	}

	if strings.TrimSpace(runtime_cfg.PublicPathPrefix) == "" {
		return RuntimeConfig{}, fmt.Errorf(
			"parse runtime config %q: missing public_path_prefix",
			RuntimeConfigPath,
		)
	}

	return runtime_cfg, nil
}
