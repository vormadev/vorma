package wave

import (
	"os"
	"strconv"

	"github.com/vormadev/vorma/internal/waveport"
)

const (
	envMode              = waveport.EnvMode
	envModeDev           = waveport.EnvModeDev
	envPort              = waveport.EnvPort
	envPortSet           = waveport.EnvPortSet
	envRefreshServerPort = "__WAVE_REFRESH_SERVER_PORT"
)

func GetIsDev() bool {
	return waveport.GetIsDev()
}

func SetModeToDev() {
	waveport.SetModeToDev()
}

func parseEnvPort() int {
	return waveport.ParseEnvPort()
}

type portResolver = waveport.Resolver

func newPortResolver() *portResolver {
	return waveport.NewResolver()
}

// MustGetPort returns the application runtime port.
// In dev mode it resolves and caches a framework-controlled free port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func MustGetPort() int {
	return waveport.GetDefaultResolver().MustGetPort()
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
