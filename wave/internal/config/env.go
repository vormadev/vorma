package config

import (
	"os"
	"strconv"
	"sync"

	"github.com/vormadev/vorma/kit/netutil"
)

// Environment variable keys
const (
	envMode              = "WAVE_MODE"
	envModeDev           = "development"
	envPort              = "PORT"
	envPortSet           = "WAVE_PORT_HAS_BEEN_SET"
	envRefreshServerPort = "WAVE_REFRESH_SERVER_PORT"
)

// GetIsDev returns true if running in development mode
func GetIsDev() bool {
	return os.Getenv(envMode) == envModeDev
}

// SetModeToDev sets the environment to development mode
func SetModeToDev() {
	os.Setenv(envMode, envModeDev)
}

// GetPort returns the configured port, or 0 if not set
func GetPort() int {
	p, err := strconv.Atoi(os.Getenv(envPort))
	if err != nil {
		return 0
	}
	return p
}

// SetPort sets the PORT environment variable
func SetPort(port int) {
	os.Setenv(envPort, strconv.Itoa(port))
}

var (
	appPortOnce   sync.Once
	appPortResult int
)

// MustGetAppPort returns the application port.
// In dev mode, finds a free port if needed.
func MustGetAppPort() int {
	appPortOnce.Do(func() {
		if !GetIsDev() || os.Getenv(envPortSet) == "true" {
			p := GetPort()
			if p <= 0 {
				appPortResult = 8080
			} else {
				appPortResult = p
			}
			return
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
		appPortResult = port
	})

	return appPortResult
}

// GetRefreshServerPort returns the dev refresh server port
func GetRefreshServerPort() int {
	p, err := strconv.Atoi(os.Getenv(envRefreshServerPort))
	if err != nil {
		return 0
	}
	return p
}

// SetRefreshServerPort sets the refresh server port env var
func SetRefreshServerPort(port int) {
	os.Setenv(envRefreshServerPort, strconv.Itoa(port))
}
