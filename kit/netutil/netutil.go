package netutil

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

// GetFreePort returns a free port number. If the default port
// is not available, it will try to find a free port, checking
// at most the next 1024 ports. If no free port is found, it
// will get a random free port. If that fails, it will return
// the default port. If the default port is set to 0, 8080 will
// be used instead.
func GetFreePort(defaultPort int) (int, error) {
	if defaultPort <= 0 || defaultPort > 65535 {
		defaultPort = 8080
	}

	if CheckAvailability(defaultPort) {
		return defaultPort, nil
	}

	for i := 1; i <= 1024; i++ {
		port := defaultPort + i
		if port > 65535 {
			break
		}
		if CheckAvailability(port) {
			return port, nil
		}
	}

	port, err := GetRandomFreePort()
	if err != nil {
		return defaultPort, err
	}

	return port, nil
}

func CheckAvailability(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}

	addr := fmt.Sprintf(":%d", port)

	addrsToCheck := []string{addr, "localhost" + addr}
	networksToCheck := []string{"tcp", "tcp4", "tcp6"}

	var successfulProbe bool
	for _, network := range networksToCheck {
		for _, addr := range addrsToCheck {
			ln, err := net.Listen(network, addr)
			if err != nil {
				if canIgnoreListenError(err) {
					continue
				}
				return false
			}
			ln.Close()
			successfulProbe = true
		}
	}

	return successfulProbe
}

func GetRandomFreePort() (port int, err error) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		ln, err = net.Listen("tcp", "localhost:0")
		if err != nil {
			return 0, err
		}
	}
	defer ln.Close()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("expected TCP listener address, got %T", ln.Addr())
	}
	return tcpAddr.Port, nil
}

func canIgnoreListenError(err error) bool {
	return errors.Is(err, syscall.EAFNOSUPPORT) ||
		errors.Is(err, syscall.EPROTONOSUPPORT) ||
		errors.Is(err, syscall.EADDRNOTAVAIL)
}

func IsLocalhost(host string) bool {
	splitHost, _, err := net.SplitHostPort(host)
	if err != nil {
		splitHost = host
		if strings.HasPrefix(splitHost, "[") && strings.HasSuffix(splitHost, "]") {
			splitHost = splitHost[1 : len(splitHost)-1]
		}
	}
	if strings.EqualFold(splitHost, "localhost") {
		return true
	}
	ip := net.ParseIP(splitHost)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
