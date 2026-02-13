package tooling

import (
	"net/url"
	"path/filepath"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

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
	normalizedPath := normalizeCSSFilePathForImportTracking(path)
	if normalizedPath == "" {
		return false
	}

	p.mu.RLock()
	_, ok := p.criticalImports[normalizedPath]
	p.mu.RUnlock()
	return ok
}

func (p *cssProcessor) isNormalFile(path string) bool {
	normalizedPath := normalizeCSSFilePathForImportTracking(path)
	if normalizedPath == "" {
		return false
	}

	p.mu.RLock()
	_, ok := p.normalImports[normalizedPath]
	p.mu.RUnlock()
	return ok
}

func normalizeCSSFilePathForImportTracking(filePath string) string {
	absoluteFilePath, absolutePathError := filepath.Abs(filePath)
	if absolutePathError != nil {
		absoluteFilePath = filepath.Clean(filePath)
	}

	resolvedFilePath, resolveError := filepath.EvalSymlinks(absoluteFilePath)
	if resolveError == nil && resolvedFilePath != "" {
		return filepath.Clean(resolvedFilePath)
	}

	return filepath.Clean(absoluteFilePath)
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
