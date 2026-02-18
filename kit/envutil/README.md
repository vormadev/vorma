# kit/envutil

`github.com/vormadev/vorma/kit/envutil`

Small helpers for reading environment variables with defaults.

## Import

```go
import "github.com/vormadev/vorma/kit/envutil"
```

## Quick Start

```go
port := envutil.GetInt("PORT", 8080)
debug := envutil.GetBool("DEBUG", false)
service := envutil.GetStr("SERVICE_NAME", "api")
```

## Behavior

- If a variable is unset, the default is returned.
- If parse fails (`Int`/`Bool`), the default is returned.
- No function returns an error.

Parsing details:

- `Int` uses `strconv.Atoi`.
- `Bool` uses `strconv.ParseBool` (`1/0`, `t/f`, `true/false`,
  case-insensitive).

## When Not To Use

Do not use this package for strict config validation flows where invalid values
must fail startup loudly.  
In those cases, parse explicitly and return/report errors.

## API Reference

- `func GetStr(key string, defaultValue string) string`
- `func GetInt(key string, defaultValue int) int`
- `func GetBool(key string, defaultValue bool) bool`
