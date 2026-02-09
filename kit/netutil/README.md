# kit/netutil

`github.com/vormadev/vorma/kit/netutil`

Small networking helpers for two common backend tasks:

- picking a usable TCP port for local servers or tests
- safely detecting whether a request host points to localhost/loopback

## Import

```go
import "github.com/vormadev/vorma/kit/netutil"
```

## Quick Start

### Start a server on an available port

```go
port, err := netutil.GetFreePort(8080)
if err != nil {
	// You still get a fallback port back.
	log.Printf("port probe warning: %v", err)
}

addr := fmt.Sprintf(":%d", port)
log.Printf("starting on %s", addr)
if err := http.ListenAndServe(addr, handler); err != nil {
	log.Fatal(err)
}
```

`GetFreePort` strategy:

1. use `defaultPort` (or `8080` when `defaultPort == 0`) if available
2. scan up to the next 1024 ports
3. ask kernel for an ephemeral random port
4. if random lookup fails, return the default port with the error

### Gate localhost-only behavior

```go
if netutil.IsLocalhost(r.Host) {
	// allow local-only debug endpoint
	renderDebug(w, r)
	return
}
http.NotFound(w, r)
```

`IsLocalhost` returns `true` for:

- `localhost` (any case, with or without port)
- IPv4 loopback (`127.0.0.0/8`)
- IPv6 loopback (`::1`, bracketed or unbracketed)
- IPv4-mapped loopback IPv6 forms (for example `::ffff:127.0.0.1`)

It returns `false` for normal hostnames, private RFC1918 addresses, public IPs,
and malformed host values.

## API Notes

- `CheckAvailability` opens listeners on `tcp`, `tcp4`, and `tcp6`, on both
  `:<port>` and `localhost:<port>`. It only returns `true` if all probes
  succeed.
- Port availability checks are best-effort snapshots. Another process can claim
  a port between check and bind.
- `GetRandomFreePort` is useful for test setup, but you should still bind
  immediately after obtaining the port.

## Public API Coverage

- `func CheckAvailability(port int) bool`
- `func GetFreePort(defaultPort int) (int, error)`
- `func GetRandomFreePort() (port int, err error)`
- `func IsLocalhost(host string) bool`
