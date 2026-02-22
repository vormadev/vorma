// Package wavecore centralizes Wave-internal helpers used by runtime and tooling.
//
// The package intentionally combines:
// - runtime mode and port resolution
// - filesystem path normalization and location comparison
// - browser/public URL reference normalization
//
// Keeping these contracts together prevents subtle cross-package drift in core
// runtime semantics.
package wavecore

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/netutil"
)

const (
	// EnvMode stores the current Wave runtime mode.
	EnvMode = "__WAVE_MODE"
	// EnvModeDev is the development-mode value for EnvMode.
	EnvModeDev = "development"
	// EnvPort is the application runtime port environment variable.
	EnvPort = "PORT"
	// EnvPortSet marks that Wave has already resolved and set EnvPort in this
	// process.
	EnvPortSet = "__WAVE_PORT_HAS_BEEN_SET"
)

// Resolver caches resolved runtime port values for a given mode view.
type Resolver struct {
	resolvePortOnce sync.Once
	resolvedPort    int
	isDevMode       bool
	hasModeSnapshot bool
	getFreePort     func(int) (int, error)
}

// NewResolver constructs a resolver that reads mode dynamically from env.
func NewResolver() *Resolver {
	return &Resolver{
		getFreePort: netutil.GetFreePort,
	}
}

// NewResolverForMode constructs a resolver pinned to an explicit mode snapshot.
func NewResolverForMode(isDev bool) *Resolver {
	return &Resolver{
		isDevMode:       isDev,
		hasModeSnapshot: true,
		getFreePort:     netutil.GetFreePort,
	}
}

// NewResolverWithFreePortResolver constructs a resolver with one injected free-port
// lookup function. This is primarily used when callers need deterministic behavior.
func NewResolverWithFreePortResolver(
	getFreePort func(int) (int, error),
) *Resolver {
	if getFreePort == nil {
		getFreePort = netutil.GetFreePort
	}
	return &Resolver{
		getFreePort: getFreePort,
	}
}

// MustGetPort returns the runtime port.
// In dev mode, empty PORT uses framework default base-port selection.
// It panics in dev mode when PORT is invalid or a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func (resolver *Resolver) MustGetPort() int {
	if resolver == nil {
		return NewResolver().MustGetPort()
	}

	resolver.resolvePortOnce.Do(func() {
		resolver.resolvedPort = resolvePortFromEnvironment(
			resolver.isDevModeForResolution(),
			resolver.getFreePort,
		)
	})

	return resolver.resolvedPort
}

func (resolver *Resolver) isDevModeForResolution() bool {
	if resolver == nil {
		return GetIsDev()
	}
	if resolver.hasModeSnapshot {
		return resolver.isDevMode
	}
	return GetIsDev()
}

// ParseEnvPort parses EnvPort and returns 0 when unset or invalid.
func ParseEnvPort() int {
	port, parseError := strconv.Atoi(os.Getenv(EnvPort))
	if parseError != nil || port <= 0 || port > 65535 {
		return 0
	}
	return port
}

// SetEnvPort writes the runtime port into EnvPort.
func SetEnvPort(port int) {
	os.Setenv(EnvPort, strconv.Itoa(port))
}

// GetIsDev reports whether EnvMode is currently set to development.
func GetIsDev() bool {
	return os.Getenv(EnvMode) == EnvModeDev
}

// SetModeToDev marks EnvMode as development.
func SetModeToDev() {
	os.Setenv(EnvMode, EnvModeDev)
}

// TrimAndCleanPath trims surrounding whitespace and applies filepath.Clean.
func TrimAndCleanPath(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}
	return filepath.Clean(trimmedPath)
}

// Absolute resolves one path to an absolute cleaned form.
func Absolute(path string) string {
	cleanedPath := TrimAndCleanPath(path)
	if cleanedPath == "" {
		return ""
	}

	absolutePath, absoluteError := filepath.Abs(cleanedPath)
	if absoluteError != nil {
		return cleanedPath
	}
	return filepath.Clean(absolutePath)
}

// AbsoluteSlash resolves one absolute path and normalizes separators to '/'.
func AbsoluteSlash(path string) string {
	normalizedPath := Absolute(path)
	if normalizedPath == "" {
		return ""
	}
	return filepath.ToSlash(normalizedPath)
}

// AbsoluteDirectory resolves an input path to an absolute directory path.
func AbsoluteDirectory(path string) string {
	normalizedPath := Absolute(path)
	if normalizedPath == "" {
		return ""
	}

	fileInfo, statError := os.Stat(normalizedPath)
	if statError == nil && fileInfo.IsDir() {
		return normalizedPath
	}
	return filepath.Dir(normalizedPath)
}

// PathsReferToSameLocation reports whether both paths normalize to the same location.
func PathsReferToSameLocation(pathA string, pathB string) bool {
	normalizedPathA := CanonicalizePathForLocationComparison(pathA)
	normalizedPathB := CanonicalizePathForLocationComparison(pathB)
	return normalizedPathA != "" &&
		normalizedPathB != "" &&
		normalizedPathA == normalizedPathB
}

// CanonicalizePathForLocationComparison resolves one path for location equality checks.
func CanonicalizePathForLocationComparison(path string) string {
	normalizedPath := Absolute(path)
	if normalizedPath == "" {
		return ""
	}
	return canonicalizeNormalizedPathForLocationComparison(normalizedPath)
}

func canonicalizeNormalizedPathForLocationComparison(normalizedPath string) string {
	resolvedPath, resolveError := filepath.EvalSymlinks(normalizedPath)
	if resolveError == nil && resolvedPath != "" {
		return filepath.Clean(resolvedPath)
	}

	parentPath := filepath.Dir(normalizedPath)
	if parentPath != "" && parentPath != normalizedPath {
		resolvedParentPath, parentResolveError := filepath.EvalSymlinks(parentPath)
		if parentResolveError == nil && resolvedParentPath != "" {
			return filepath.Clean(
				filepath.Join(resolvedParentPath, filepath.Base(normalizedPath)),
			)
		}
	}
	return normalizedPath
}

// ResolveFromReferencedPath resolves a referenced asset path under publicPathPrefix.
// It trims, normalizes, and rejects empty/effectively-empty referenced paths.
func ResolveFromReferencedPath(
	publicPathPrefix string,
	referencedPath string,
) string {
	trimmedReferencedPath := strings.TrimSpace(referencedPath)
	if trimmedReferencedPath == "" {
		return ""
	}

	normalizedReferencedPath := strings.TrimPrefix(
		path.Clean("/"+trimmedReferencedPath),
		"/",
	)
	if normalizedReferencedPath == "" || normalizedReferencedPath == "." {
		return ""
	}

	return matcher.EnsureLeadingSlash(
		path.Join(publicPathPrefix, normalizedReferencedPath),
	)
}

// resolvePortFromEnvironment resolves the runtime port from env and mode.
// In dev mode, empty PORT uses framework default base-port selection.
// It panics in dev mode when PORT is invalid or a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func resolvePortFromEnvironment(
	isDevMode bool,
	getFreePort func(int) (int, error),
) int {
	if getFreePort == nil {
		getFreePort = netutil.GetFreePort
	}

	if !isDevMode || os.Getenv(EnvPortSet) == "true" {
		port := ParseEnvPort()
		if port <= 0 {
			panic(
				fmt.Errorf("invalid %s value %q", EnvPort, os.Getenv(EnvPort)),
			)
		}
		return port
	}

	defaultPort, defaultPortError := resolveDevBasePortForLookup()
	if defaultPortError != nil {
		panic(defaultPortError)
	}

	port, resolveFreePortError := getFreePort(defaultPort)
	if resolveFreePortError != nil {
		panic(
			fmt.Errorf(
				"failed to resolve free dev port from %d: %w",
				defaultPort,
				resolveFreePortError,
			),
		)
	}

	SetEnvPort(port)
	os.Setenv(EnvPortSet, "true")
	return port
}

func resolveDevBasePortForLookup() (int, error) {
	rawPort := os.Getenv(EnvPort)
	if rawPort == "" {
		return 0, nil
	}

	port, parseError := strconv.Atoi(rawPort)
	if parseError != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid %s value %q", EnvPort, rawPort)
	}

	return port, nil
}
