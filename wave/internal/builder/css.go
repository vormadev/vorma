package builder

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/esbuildutil"
	"github.com/vormadev/vorma/wave/internal/config"
)

type cssProcessor struct {
	cfg *config.Config
	log *slog.Logger
	b   *Builder

	mu              sync.RWMutex
	criticalCtx     esbuild.BuildContext
	normalCtx       esbuild.BuildContext
	hasCriticalCtx  bool
	hasNormalCtx    bool
	criticalImports map[string]struct{}
	normalImports   map[string]struct{}

	// Cached file map for URL resolution during build
	cachedFileMap   config.FileMap
	cachedFileMapMu sync.Mutex
}

// newCSSProcessor creates a CSS processor with the builder reference
func newCSSProcessor(cfg *config.Config, log *slog.Logger, b *Builder) *cssProcessor {
	return &cssProcessor{
		cfg:             cfg,
		log:             log,
		b:               b,
		criticalImports: make(map[string]struct{}),
		normalImports:   make(map[string]struct{}),
	}
}

// close disposes of esbuild contexts to prevent resource leaks
func (p *cssProcessor) close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.hasCriticalCtx {
		p.criticalCtx.Dispose()
		p.hasCriticalCtx = false
	}
	if p.hasNormalCtx {
		p.normalCtx.Dispose()
		p.hasNormalCtx = false
	}
	return nil
}

func (p *cssProcessor) buildAll(isDev bool) error {
	if err := p.buildCritical(isDev); err != nil {
		return fmt.Errorf("critical CSS: %w", err)
	}
	if err := p.buildNormal(isDev); err != nil {
		return fmt.Errorf("normal CSS: %w", err)
	}
	return nil
}

func (p *cssProcessor) buildCritical(isDev bool) error {
	return p.build("critical", isDev)
}

func (p *cssProcessor) buildNormal(isDev bool) error {
	return p.build("normal", isDev)
}

func (p *cssProcessor) build(nature string, isDev bool) error {
	p.cachedFileMapMu.Lock()
	p.cachedFileMap = nil
	p.cachedFileMapMu.Unlock()

	var entryPoint string
	if nature == "critical" {
		entryPoint = p.cfg.CriticalCSSEntry()
	} else {
		entryPoint = p.cfg.NonCriticalCSSEntry()
	}

	if entryPoint == "" {
		return nil
	}

	ctx, ctxErr := esbuild.Context(esbuild.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Bundle:            true,
		MinifyWhitespace:  !isDev,
		MinifyIdentifiers: !isDev,
		MinifySyntax:      !isDev,
		Write:             false,
		Metafile:          true,
		Plugins:           []esbuild.Plugin{p.urlResolverPlugin()},
	})
	if ctxErr != nil {
		return fmt.Errorf("esbuild context: %v", ctxErr.Errors)
	}

	p.mu.Lock()
	// Dispose of old context before replacing
	if nature == "critical" {
		if p.hasCriticalCtx {
			p.criticalCtx.Dispose()
		}
		p.criticalCtx = ctx
		p.hasCriticalCtx = true
	} else {
		if p.hasNormalCtx {
			p.normalCtx.Dispose()
		}
		p.normalCtx = ctx
		p.hasNormalCtx = true
	}
	p.mu.Unlock()

	result := ctx.Rebuild()
	if err := esbuildutil.CollectErrors(result); err != nil {
		return err
	}

	if len(result.OutputFiles) == 0 {
		return fmt.Errorf("esbuild produced no output files for %s CSS", nature)
	}

	// Track imports for file watching
	var metafile esbuildutil.ESBuildMetafileSubset
	if err := json.Unmarshal([]byte(result.Metafile), &metafile); err != nil {
		return fmt.Errorf("parse metafile: %w", err)
	}

	p.mu.Lock()
	if nature == "critical" {
		p.criticalImports = make(map[string]struct{})
		for filePath := range metafile.Inputs {
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				absPath = filePath
			}
			p.criticalImports[absPath] = struct{}{}
		}
	} else {
		p.normalImports = make(map[string]struct{})
		for filePath := range metafile.Inputs {
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				absPath = filePath
			}
			p.normalImports[absPath] = struct{}{}
		}
	}
	p.mu.Unlock()

	// Write output
	var outputPath, outputFileName string
	if nature == "critical" {
		outputPath = p.cfg.Dist.Internal()
		outputFileName = "critical.css"
	} else {
		outputPath = p.cfg.Dist.StaticPublic()

		// Clean up old files
		oldFiles, err := filepath.Glob(filepath.Join(outputPath, config.NormalCSSGlobPattern))
		if err != nil {
			p.log.Warn("failed to glob old CSS files", "error", err)
		}
		for _, old := range oldFiles {
			if err := os.Remove(old); err != nil {
				p.log.Warn("failed to remove old CSS file", "file", old, "error", err)
			}
		}

		outputFileName = hashBytes(result.OutputFiles[0].Contents, config.NormalCSSBaseName)

		// Write ref file atomically
		refPath := p.cfg.Dist.NormalCSSRef()
		if err := writeFileAtomicBytes(refPath, []byte(outputFileName)); err != nil {
			return fmt.Errorf("write CSS ref: %w", err)
		}
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// Write CSS file atomically
	return writeFileAtomicBytes(filepath.Join(outputPath, outputFileName), result.OutputFiles[0].Contents)
}

func (p *cssProcessor) urlResolverPlugin() esbuild.Plugin {
	return esbuild.Plugin{
		Name: "url-resolver",
		Setup: func(build esbuild.PluginBuild) {
			build.OnResolve(esbuild.OnResolveOptions{Filter: ".*", Namespace: "file"},
				func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					if args.Kind != esbuild.ResolveCSSURLToken {
						return esbuild.OnResolveResult{}, nil
					}

					u, err := url.Parse(args.Path)
					if err == nil && u.Scheme != "" {
						return esbuild.OnResolveResult{}, nil
					}
					if strings.HasPrefix(args.Path, "//") {
						return esbuild.OnResolveResult{}, nil
					}

					resolved := p.b.getPublicURLBuildtimeCached(args.Path)
					return esbuild.OnResolveResult{
						Path:     resolved,
						External: true,
					}, nil
				},
			)
		},
	}
}

func (p *cssProcessor) isCriticalFile(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	p.mu.RLock()
	_, ok := p.criticalImports[absPath]
	p.mu.RUnlock()
	return ok
}

func (p *cssProcessor) isNormalFile(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	p.mu.RLock()
	_, ok := p.normalImports[absPath]
	p.mu.RUnlock()
	return ok
}

// IsCriticalCSSFile checks if a path is a critical CSS file or import
func (b *Builder) IsCriticalCSSFile(path string) bool {
	return b.css.isCriticalFile(path)
}

// IsNormalCSSFile checks if a path is a normal CSS file or import
func (b *Builder) IsNormalCSSFile(path string) bool {
	return b.css.isNormalFile(path)
}

// IsCSSFile checks if a path is any CSS file tracked by the builder
func (b *Builder) IsCSSFile(path string) bool {
	return b.IsCriticalCSSFile(path) || b.IsNormalCSSFile(path)
}
