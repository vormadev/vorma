package tooling

import (
	"log/slog"
	"sync"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/wave"
)

type cssProcessor struct {
	cfg *wave.ParsedConfig
	log *slog.Logger
	b   *Builder

	mu                                             sync.RWMutex
	criticalCtx                                    esbuild.BuildContext
	normalCtx                                      esbuild.BuildContext
	hasCriticalCtx                                 bool
	hasNormalCtx                                   bool
	criticalEntry                                  string
	normalEntry                                    string
	criticalCtxIsDev                               bool
	normalCtxIsDev                                 bool
	criticalImports                                map[string]struct{}
	normalImports                                  map[string]struct{}
	cachedCriticalCSSHotReloadOutput               string
	hasCachedCriticalCSSHotReloadOutput            bool
	criticalCSSHotReloadOutputInvalidatedByRebuild bool
	cachedNormalCSSHotReloadURL                    string
	hasCachedNormalCSSHotReloadURL                 bool
	normalCSSHotReloadOutputInvalidatedByRebuild   bool

	// Cached file map for URL resolution during build
	cachedFileMap   wave.FileMap
	cachedFileMapMu sync.Mutex
}

type cssBuildNature string

const (
	cssBuildNatureCritical cssBuildNature = "critical"
	cssBuildNatureNormal   cssBuildNature = "normal"
)

// newCSSProcessor creates a CSS processor with the builder reference
func newCSSProcessor(cfg *wave.ParsedConfig, log *slog.Logger, b *Builder) *cssProcessor {
	if log == nil {
		log = colorlog.New("wave")
	}

	return &cssProcessor{
		cfg:             cfg,
		log:             log,
		b:               b,
		criticalImports: make(map[string]struct{}),
		normalImports:   make(map[string]struct{}),
	}
}
