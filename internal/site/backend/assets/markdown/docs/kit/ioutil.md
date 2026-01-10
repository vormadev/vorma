---
title: ioutil
description: I/O utilities with size-limited reading.
---

```go
import "github.com/vormadev/vorma/kit/ioutil"
```

## Constants

```go
const (
    OneKB uint64 = 1024
    OneMB        = 1024 * OneKB
    OneGB        = 1024 * OneMB
)
```

## Errors

```go
var ErrReadLimitExceeded error
```

## Functions

Read up to limit bytes; returns `ErrReadLimitExceeded` if data exceeds limit:

```go
func ReadLimited(r io.Reader, limit uint64) ([]byte, error)
```

On overflow, returns truncated data (up to limit) along with the error.

Example:

```go
data, err := ioutil.ReadLimited(r, 5*ioutil.OneMB)
if errors.Is(err, ioutil.ErrReadLimitExceeded) {
    // handle oversized input
}
```
