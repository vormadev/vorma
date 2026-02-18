// Package waveport centralizes Wave runtime port and mode environment handling.
//
// This package owns:
// - `PORT` parsing/validation
// - dev vs non-dev mode resolution behavior
// - per-resolver cached port resolution
//
// Keeping this logic in one place avoids inconsistent port semantics between
// runtime and tooling components.
package waveport

import (
	"fmt"
	"os"
	"strconv"
	"sync"

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
}

// NewResolver constructs a resolver that reads mode dynamically from env.
func NewResolver() *Resolver { return &Resolver{} }

// NewResolverForMode constructs a resolver pinned to an explicit mode snapshot.
func NewResolverForMode(isDev bool) *Resolver {
	return &Resolver{
		isDevMode:       isDev,
		hasModeSnapshot: true,
	}
}

var defaultResolver = NewResolver()
var getFreePort = netutil.GetFreePort

// GetDefaultResolver returns the process-wide shared port resolver.
func GetDefaultResolver() *Resolver {
	if defaultResolver == nil {
		defaultResolver = NewResolver()
	}
	return defaultResolver
}

// MustGetPort returns the runtime port.
// In dev mode, empty PORT uses framework default base-port selection.
// It panics in dev mode when PORT is invalid or a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func (resolver *Resolver) MustGetPort() int {
	if resolver == nil {
		return GetDefaultResolver().MustGetPort()
	}

	resolver.resolvePortOnce.Do(func() {
		resolver.resolvedPort = resolvePortFromEnvironment(
			resolver.isDevModeForResolution(),
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
	p, err := strconv.Atoi(os.Getenv(EnvPort))
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
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

// resolvePortFromEnvironment resolves the runtime port from env and mode.
// In dev mode, empty PORT uses framework default base-port selection.
// It panics in dev mode when PORT is invalid or a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func resolvePortFromEnvironment(isDevMode bool) int {
	if !isDevMode || os.Getenv(EnvPortSet) == "true" {
		port := ParseEnvPort()
		if port <= 0 {
			panic(
				fmt.Errorf("invalid %s value %q", EnvPort, os.Getenv(EnvPort)),
			)
		}
		return port
	}

	defaultPort, err := resolveDevBasePortForLookup()
	if err != nil {
		panic(err)
	}

	port, err := getFreePort(defaultPort)
	if err != nil {
		panic(
			fmt.Errorf(
				"failed to resolve free dev port from %d: %w",
				defaultPort,
				err,
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

	port, err := strconv.Atoi(rawPort)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid %s value %q", EnvPort, rawPort)
	}

	return port, nil
}

// ResetDefaultResolverForTest resets the shared resolver to a fresh instance.
func ResetDefaultResolverForTest() {
	defaultResolver = NewResolver()
}

// SetGetFreePortForTest replaces the free-port resolver hook and returns a
// restore function.
func SetGetFreePortForTest(getFreePortFunc func(int) (int, error)) func() {
	previousGetFreePort := getFreePort
	getFreePort = getFreePortFunc
	return func() {
		getFreePort = previousGetFreePort
	}
}
