package css

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling_3/toolingshared"
)

// Processor builds and resolves CSS outputs for Wave build and dev workflows.
type Processor struct {
	cfg *wave.ParsedConfig
	log *slog.Logger

	mu sync.RWMutex

	cachedCriticalCSS string
	cachedNormalURL   string
}

// BuildOptions selects which CSS pipelines to execute.
type BuildOptions struct {
	BuildCriticalCSS bool
	BuildNormalCSS   bool
}

// NewProcessor creates a CSS processor for one config.
func NewProcessor(cfg *wave.ParsedConfig, log *slog.Logger) *Processor {
	if log == nil {
		log = slog.Default()
	}
	return &Processor{cfg: cfg, log: log}
}

// Build executes requested CSS pipelines and updates in-memory cache.
func (processor *Processor) Build(options BuildOptions) error {
	if processor == nil || processor.cfg == nil {
		return errors.New("css processor config is nil")
	}

	if options.BuildCriticalCSS {
		if criticalBuildError := processor.buildCriticalCSS(); criticalBuildError != nil {
			return criticalBuildError
		}
	}
	if options.BuildNormalCSS {
		if normalBuildError := processor.buildNormalCSS(); normalBuildError != nil {
			return normalBuildError
		}
	}
	return nil
}

// IsCriticalCSSFile reports whether path is configured critical CSS entry.
func (processor *Processor) IsCriticalCSSFile(path string) bool {
	if processor == nil || processor.cfg == nil {
		return false
	}
	entryPath := processor.cfg.CriticalCSSEntry()
	if entryPath == "" {
		return false
	}
	return samePath(path, entryPath)
}

// IsNormalCSSFile reports whether path is configured non-critical CSS entry.
func (processor *Processor) IsNormalCSSFile(path string) bool {
	if processor == nil || processor.cfg == nil {
		return false
	}
	entryPath := processor.cfg.NonCriticalCSSEntry()
	if entryPath == "" {
		return false
	}
	return samePath(path, entryPath)
}

// CriticalCSS returns cached critical CSS text when available.
func (processor *Processor) CriticalCSS() (string, bool) {
	processor.mu.RLock()
	defer processor.mu.RUnlock()
	if strings.TrimSpace(processor.cachedCriticalCSS) == "" {
		return "", false
	}
	return processor.cachedCriticalCSS, true
}

// NormalCSSURL returns cached normal CSS public URL when available.
func (processor *Processor) NormalCSSURL() (string, bool) {
	processor.mu.RLock()
	defer processor.mu.RUnlock()
	if strings.TrimSpace(processor.cachedNormalURL) == "" {
		return "", false
	}
	return processor.cachedNormalURL, true
}

// ReloadCachedOutputs loads CSS cache values from output files.
func (processor *Processor) ReloadCachedOutputs() {
	if processor == nil || processor.cfg == nil {
		return
	}

	if criticalCSSBytes, criticalReadError := os.ReadFile(processor.cfg.Dist.CriticalCSS()); criticalReadError == nil {
		processor.mu.Lock()
		processor.cachedCriticalCSS = string(criticalCSSBytes)
		processor.mu.Unlock()
	}

	normalRefBytes, normalRefReadError := os.ReadFile(
		processor.cfg.Dist.NormalCSSRef(),
	)
	if normalRefReadError == nil {
		normalRef := strings.TrimSpace(string(normalRefBytes))
		if normalRef != "" {
			normalURL := joinPublicURL(
				processor.cfg.PublicPathPrefix(),
				normalRef,
			)
			processor.mu.Lock()
			processor.cachedNormalURL = normalURL
			processor.mu.Unlock()
		}
	}
}

// buildCriticalCSS builds the critical CSS pipeline and writes output file.
func (processor *Processor) buildCriticalCSS() error {
	entryPath := processor.cfg.CriticalCSSEntry()
	outputPath := processor.cfg.Dist.CriticalCSS()
	if entryPath == "" {
		processor.mu.Lock()
		processor.cachedCriticalCSS = ""
		processor.mu.Unlock()
		_ = os.Remove(outputPath)
		return nil
	}

	buildOutput, buildError := buildSingleCSSEntry(entryPath)
	if buildError != nil {
		return fmt.Errorf("build critical css %q: %w", entryPath, buildError)
	}

	if writeError := toolingshared.WriteFileAtomically(outputPath, []byte(buildOutput), 0o644); writeError != nil {
		return writeError
	}

	processor.mu.Lock()
	processor.cachedCriticalCSS = buildOutput
	processor.mu.Unlock()
	processor.log.Info(
		"built critical css",
		"entry",
		entryPath,
		"out",
		outputPath,
	)
	return nil
}

// buildNormalCSS builds non-critical CSS output and writes ref metadata.
func (processor *Processor) buildNormalCSS() error {
	entryPath := processor.cfg.NonCriticalCSSEntry()
	refPath := processor.cfg.Dist.NormalCSSRef()
	outputDirectoryPath := processor.cfg.Dist.StaticPublic()
	if entryPath == "" {
		processor.mu.Lock()
		processor.cachedNormalURL = ""
		processor.mu.Unlock()
		_ = os.Remove(refPath)
		return nil
	}

	buildOutput, buildError := buildSingleCSSEntry(entryPath)
	if buildError != nil {
		return fmt.Errorf("build normal css %q: %w", entryPath, buildError)
	}

	hashSum := sha256.Sum256([]byte(buildOutput))
	hashPrefix := hex.EncodeToString(hashSum[:])[:16]
	fileName := fmt.Sprintf("vorma_internal_normal_%s.css", hashPrefix)
	outputFilePath := filepath.Join(outputDirectoryPath, fileName)

	if writeError := toolingshared.WriteFileAtomically(outputFilePath, []byte(buildOutput), 0o644); writeError != nil {
		return writeError
	}
	if writeRefError := toolingshared.WriteFileAtomically(refPath, []byte(fileName), 0o644); writeRefError != nil {
		return writeRefError
	}

	normalURL := joinPublicURL(processor.cfg.PublicPathPrefix(), fileName)
	processor.mu.Lock()
	processor.cachedNormalURL = normalURL
	processor.mu.Unlock()
	processor.log.Info(
		"built normal css",
		"entry",
		entryPath,
		"out",
		outputFilePath,
		"url",
		normalURL,
	)
	return nil
}

// BuildCriticalCSSOnly builds only critical CSS pipeline.
func (processor *Processor) BuildCriticalCSSOnly() error {
	return processor.Build(BuildOptions{BuildCriticalCSS: true})
}

// BuildNormalCSSOnly builds only non-critical CSS pipeline.
func (processor *Processor) BuildNormalCSSOnly() error {
	return processor.Build(BuildOptions{BuildNormalCSS: true})
}

// buildSingleCSSEntry executes CSS-only esbuild pipeline and returns output bytes as string.
func buildSingleCSSEntry(entryPath string) (string, error) {
	buildResult := api.Build(api.BuildOptions{
		EntryPoints:       []string{entryPath},
		Bundle:            true,
		Write:             false,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		LogLevel:          api.LogLevelSilent,
		Loader: map[string]api.Loader{
			".css": api.LoaderCSS,
		},
	})
	if len(buildResult.Errors) > 0 {
		return "", errors.New(buildResult.Errors[0].Text)
	}
	if len(buildResult.OutputFiles) == 0 {
		return "", errors.New("esbuild produced no CSS output files")
	}
	return string(buildResult.OutputFiles[0].Contents), nil
}

// joinPublicURL joins public path prefix and relative asset path.
func joinPublicURL(publicPathPrefix string, relativePath string) string {
	resolvedPrefix := strings.TrimSpace(publicPathPrefix)
	if resolvedPrefix == "" {
		resolvedPrefix = "/"
	}
	if !strings.HasPrefix(resolvedPrefix, "/") {
		resolvedPrefix = "/" + resolvedPrefix
	}
	if resolvedPrefix != "/" && !strings.HasSuffix(resolvedPrefix, "/") {
		resolvedPrefix += "/"
	}

	normalizedRelativePath := strings.TrimPrefix(
		strings.ReplaceAll(relativePath, "\\", "/"),
		"/",
	)
	if resolvedPrefix == "/" {
		return "/" + normalizedRelativePath
	}
	return resolvedPrefix + normalizedRelativePath
}

// samePath reports whether two paths resolve to same cleaned absolute path.
func samePath(leftPath string, rightPath string) bool {
	leftAbsolutePath, leftError := filepath.Abs(leftPath)
	rightAbsolutePath, rightError := filepath.Abs(rightPath)
	if leftError != nil || rightError != nil {
		return filepath.Clean(leftPath) == filepath.Clean(rightPath)
	}
	return filepath.Clean(leftAbsolutePath) == filepath.Clean(rightAbsolutePath)
}

// ValidateCSSConfig validates CSS entry path configuration semantics.
func ValidateCSSConfig(cfg *wave.ParsedConfig) error {
	if cfg == nil || cfg.Core == nil {
		return errors.New("config or core config is nil")
	}

	criticalEntry := cfg.CriticalCSSEntry()
	normalEntry := cfg.NonCriticalCSSEntry()
	if strings.TrimSpace(criticalEntry) == "" &&
		strings.TrimSpace(normalEntry) == "" {
		return nil
	}

	if strings.TrimSpace(criticalEntry) != "" {
		if !strings.HasSuffix(strings.ToLower(criticalEntry), ".css") {
			return fmt.Errorf(
				"critical css entry must end with .css: %q",
				criticalEntry,
			)
		}
	}
	if strings.TrimSpace(normalEntry) != "" {
		if !strings.HasSuffix(strings.ToLower(normalEntry), ".css") {
			return fmt.Errorf(
				"non-critical css entry must end with .css: %q",
				normalEntry,
			)
		}
	}

	if criticalEntry != "" && normalEntry != "" &&
		samePath(criticalEntry, normalEntry) {
		return errors.New(
			"critical and non-critical css entries must not point to same file",
		)
	}

	return nil
}
