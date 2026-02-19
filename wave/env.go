package wave

import (
	"os"
	"strconv"

	"github.com/vormadev/vorma/wave/internal/waveshared"
)

const (
	envMode              = waveshared.EnvMode
	envModeDev           = waveshared.EnvModeDev
	envPort              = waveshared.EnvPort
	envPortSet           = waveshared.EnvPortSet
	envRefreshServerPort = "__WAVE_REFRESH_SERVER_PORT"
)

// GetIsDev reports whether Wave is running in development mode.
func GetIsDev() bool {
	return waveshared.GetIsDev()
}

// SetModeToDev marks the current process as development mode.
func SetModeToDev() {
	waveshared.SetModeToDev()
}

func parseEnvPort() int {
	return waveshared.ParseEnvPort()
}

type portResolver = waveshared.Resolver

func newPortResolver() *portResolver {
	return waveshared.NewResolverForMode(GetIsDev())
}

// MustGetPort returns the application runtime port.
// In dev mode it resolves and caches a framework-controlled free port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func MustGetPort() int {
	return waveshared.GetDefaultResolver().MustGetPort()
}

// GetRefreshServerPort returns the active refresh-server port from environment.
// It returns 0 when unset or invalid.
func GetRefreshServerPort() int {
	p, err := strconv.Atoi(os.Getenv(envRefreshServerPort))
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
}

// SetRefreshServerPort writes the refresh-server port into environment.
func SetRefreshServerPort(port int) {
	os.Setenv(envRefreshServerPort, strconv.Itoa(port))
}
