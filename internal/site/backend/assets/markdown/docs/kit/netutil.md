---
title: netutil
description:
    Network utilities for port availability checking and localhost detection.
---

```go
import "github.com/vormadev/vorma/kit/netutil"
```

## Port Discovery

Find available port (tries default, then next 1024, then random):

```go
func GetFreePort(defaultPort int) (int, error)
```

Check if port is available (tcp/tcp4/tcp6 on localhost):

```go
func CheckAvailability(port int) bool
```

Get random free port from kernel:

```go
func GetRandomFreePort() (int, error)
```

## Localhost Detection

Returns true for "localhost", loopback IPs (127.x.x.x, ::1); handles host:port
format:

```go
func IsLocalhost(host string) bool
```

## Example

```go
port, err := netutil.GetFreePort(8080)
if err != nil {
    log.Printf("warning: %v, using %d anyway", err, port)
}
server.ListenAndServe(fmt.Sprintf(":%d", port))
```
