---
title: bytesutil
description:
    Go package for byte slice encoding/decoding utilities (base64, base32, gob).
---

```go
import "github.com/vormadev/vorma/kit/bytesutil"
```

## Base64 (standard, with padding)

```go
func ToBase64(b []byte) string
func FromBase64(s string) ([]byte, error)
```

## Base64 (URL-safe, no padding)

```go
func ToBase64URLRaw(b []byte) string
func FromBase64URLRaw(s string) ([]byte, error)
```

## Base32 (standard, no padding)

```go
func ToBase32Raw(b []byte) string
func FromBase32Raw(s string) ([]byte, error)
```

## Gob Serialization

Encode any value (errors on nil pointer):

```go
func ToGob(src any) ([]byte, error)
```

Decode into pointer destination (both args must be non-nil):

```go
func FromGobInto(gobBytes []byte, destPtr any) error
```

Generic decode:

```go
func FromGob[T any](gobBytes []byte) (T, error)
```

Example:

```go
data, err := bytesutil.FromGob[MyStruct](gobBytes)
```
