# kit/id

`github.com/vormadev/vorma/kit/id`

Cryptographically random string ID generation with uniform character selection.

## Import

```go
import "github.com/vormadev/vorma/kit/id"
```

## Quick Start

```go
v, err := id.New(16) // default [0-9A-Za-z]
```

Custom charset:

```go
numeric, err := id.New(8, "0123456789")
```

Batch generation:

```go
values, err := id.NewMulti(12, 5)
```

## Charset Rules

- Optional charset argument count: at most one.
- Charset length: `1..255`.
- Charset must be single-byte ASCII only.

Invalid charset input returns an error.

## Length and Quantity

- `idLen` is `uint8` (`0..255`).
- `quantity` is `uint8` (`0..255`).
- `New(0, ...)` returns `""` with no error (after charset validation).
- `NewMulti(..., 0, ...)` returns an empty slice.

## Randomness and Bias

Generation uses `crypto/rand` plus rejection sampling so character distribution
stays uniform even when charset size does not divide 256.

## API Reference

- `func New(idLen uint8, optionalCharset ...string) (string, error)`
- `func NewMulti(idLen uint8, quantity uint8, optionalCharset ...string) ([]string, error)`
