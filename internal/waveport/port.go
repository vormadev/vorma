package waveport

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/vormadev/vorma/kit/netutil"
)

const (
	EnvMode    = "__WAVE_MODE"
	EnvModeDev = "development"
	EnvPort    = "PORT"
	EnvPortSet = "__WAVE_PORT_HAS_BEEN_SET"
)

type Resolver struct {
	resolvePortOnce sync.Once
	resolvedPort    int
}

func NewResolver() *Resolver { return &Resolver{} }

var defaultResolver = NewResolver()
var getFreePort = netutil.GetFreePort

func GetDefaultResolver() *Resolver {
	if defaultResolver == nil {
		defaultResolver = NewResolver()
	}
	return defaultResolver
}

// MustGetPort returns the runtime port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func (resolver *Resolver) MustGetPort() int {
	if resolver == nil {
		return GetDefaultResolver().MustGetPort()
	}

	resolver.resolvePortOnce.Do(func() {
		resolver.resolvedPort = resolvePortFromEnvironment()
	})

	return resolver.resolvedPort
}

func ParseEnvPort() int {
	p, err := strconv.Atoi(os.Getenv(EnvPort))
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
}

func SetEnvPort(port int) {
	os.Setenv(EnvPort, strconv.Itoa(port))
}

func GetIsDev() bool {
	return os.Getenv(EnvMode) == EnvModeDev
}

func SetModeToDev() {
	os.Setenv(EnvMode, EnvModeDev)
}

// resolvePortFromEnvironment resolves the runtime port from env and mode.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func resolvePortFromEnvironment() int {
	if !GetIsDev() || os.Getenv(EnvPortSet) == "true" {
		port := ParseEnvPort()
		if port <= 0 {
			panic(
				fmt.Errorf("invalid %s value %q", EnvPort, os.Getenv(EnvPort)),
			)
		}
		return port
	}

	defaultPort := ParseEnvPort()
	if defaultPort <= 0 {
		defaultPort = 8080
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

func ResetDefaultResolverForTest() {
	defaultResolver = NewResolver()
}

func SetGetFreePortForTest(getFreePortFunc func(int) (int, error)) func() {
	previousGetFreePort := getFreePort
	getFreePort = getFreePortFunc
	return func() {
		getFreePort = previousGetFreePort
	}
}
