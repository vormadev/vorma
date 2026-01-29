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
	if err != nil {
		return 0
	}
	return p
}

func SetPort(port int) {
	os.Setenv(envPort, strconv.Itoa(port))
}

var (
	appPortOnce   sync.Once
	appPortResult int
)

// MustGetPort returns the application port.
// In dev mode, finds a free port if needed.
func MustGetPort() int {
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

// MustGetAppPort is an alias for MustGetPort for backward compatibility.
var MustGetAppPort = MustGetPort

func GetRefreshServerPort() int {
	p, err := strconv.Atoi(os.Getenv(envRefreshServerPort))
	if err != nil {
		return 0
	}
	return p
}

func SetRefreshServerPort(port int) {
	os.Setenv(envRefreshServerPort, strconv.Itoa(port))
}
