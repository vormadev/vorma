---
title: id
description:
    Cryptographically random string ID generation with uniform distribution.
---

```go
import "github.com/vormadev/vorma/kit/id"
```

## Functions

Generate single ID (default charset: `0-9A-Za-z`):

```go
func New(idLen uint8, optionalCharset ...string) (string, error)
```

Generate multiple IDs:

```go
func NewMulti(idLen uint8, quantity uint8, optionalCharset ...string) ([]string, error)
```

## Example

```go
id, err := id.New(16)                      // "Xk9mPq2RsTvWyZ1a"
id, err := id.New(8, "0123456789")         // "48293716"
ids, err := id.NewMulti(12, 5)             // 5 IDs of length 12
```
