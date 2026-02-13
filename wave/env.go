package wave

import (
	"os"
	"strconv"
	"sync"

	"github.com/vormadev/vorma/kit/netutil"
)

const (
	envMode              = "WAVE_MODE"
	envModeDev           = "development"
	envPort              = "PORT"
	envPortSet           = "WAVE_PORT_HAS_BEEN_SET"
	envRefreshServerPort = "WAVE_REFRESH_SERVER_PORT"
)

func GetIsDev() bool {
	return os.Getenv(envMode) == envModeDev
}

func SetModeToDev() {
	os.Setenv(envMode, envModeDev)
}

func GetPort() int {
	p, err := strconv.Atoi(os.Getenv(envPort))
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
}

func SetPort(port int) {
	os.Setenv(envPort, strconv.Itoa(port))
}

type PortResolver struct {
	resolvePortOnce sync.Once
	resolvedPort    int
}

func NewPortResolver() *PortResolver {
	return &PortResolver{}
}

var defaultPortResolver = NewPortResolver()

func getDefaultPortResolver() *PortResolver {
	if defaultPortResolver == nil {
		defaultPortResolver = NewPortResolver()
	}
	return defaultPortResolver
}

// MustGetPort returns the application port.
// In dev mode, finds a free port if needed.
func MustGetPort() int {
	return getDefaultPortResolver().MustGetPort()
}

// MustGetPort returns a cached application port scoped to this resolver.
// In dev mode, finds a free port if needed.
func (resolver *PortResolver) MustGetPort() int {
	if resolver == nil {
		return getDefaultPortResolver().MustGetPort()
	}

	resolver.resolvePortOnce.Do(func() {
		resolver.resolvedPort = resolvePortFromEnvironment()
	})

	return resolver.resolvedPort
}

func resolvePortFromEnvironment() int {
	if !GetIsDev() || os.Getenv(envPortSet) == "true" {
		port := GetPort()
		if port <= 0 {
			return 8080
		}
		return port
	}

	defaultPort := GetPort()
	if defaultPort <= 0 {
		defaultPort = 8080
	}

	port, err := netutil.GetFreePort(defaultPort)
	if err != nil {
		port = defaultPort
	}

	SetPort(port)
	os.Setenv(envPortSet, "true")
	return port
}

func GetRefreshServerPort() int {
	p, err := strconv.Atoi(os.Getenv(envRefreshServerPort))
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
}

func SetRefreshServerPort(port int) {
	os.Setenv(envRefreshServerPort, strconv.Itoa(port))
}
