// Package tooling contains Wave build-time and development-time orchestration.
//
// It is intentionally separate from package wave runtime APIs so production
// binaries can depend on runtime functionality without pulling in build/dev
// tool dependencies.
//
// Major responsibilities include:
// - static asset processing and file mapping
// - CSS/Vite build integration
// - devserver lifecycle, watch pipelines, and restart orchestration
// - config validation and schema generation
//
// Internal subpackages define explicit boundaries for isolated concerns. For
// example, watcher pre/post classification decisions live in
// internal/watchereventclassification.
package tooling

import (
	"log/slog"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave"
)

// Builder handles build operations. It is safe to reuse across multiple builds.
type Builder struct {
	cfg *wave.ParsedConfig
	log *slog.Logger
	css *cssProcessor
}

// BuildOpts configures a build
type BuildOpts struct {
	CompileGo    bool
	IsDev        bool
	IsRebuild    bool
	FileOnlyMode bool // Skip hooks and binary
}

// NewBuilder creates a new Builder
func NewBuilder(cfg *wave.ParsedConfig, log *slog.Logger) *Builder {
	if log == nil {
		log = colorlog.New("wave")
	}

	b := &Builder{
		cfg: cfg,
		log: log,
	}
	b.css = newCSSProcessor(cfg, log, b)
	return b
}

// Close releases resources held by the builder (e.g., esbuild contexts).
// Should be called when the builder is no longer needed.
func (b *Builder) Close() error {
	if b.css != nil {
		return b.css.close()
	}
	return nil
}

// Config returns a defensive read-only config snapshot.
// Unstable internal callback/schema fields are omitted.
func (b *Builder) Config() *wave.ParsedConfig {
	return b.cfg.Clone()
}

// RegisterSchemaSection adds a custom section to the generated JSON schema.
// This allows frameworks to extend wave.config.json with their own configuration
// while maintaining IDE autocomplete support.
func (b *Builder) RegisterSchemaSection(
	name string,
	schema jsonschema.Entry,
) {
	if b.cfg.FrameworkSchemaExtensions == nil {
		b.cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	b.cfg.FrameworkSchemaExtensions[name] = schema
}
