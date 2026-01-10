---
title: jsonutil
description: Generic JSON serialization and parsing helpers.
---

```go
import "github.com/vormadev/vorma/kit/jsonutil"
```

## Types

```go
type JSONString string
```

## Functions

```go
func Serialize(v any) ([]byte, error)
func Parse[T any](data []byte) (T, error)
```

Example:

```go
data, err := jsonutil.Serialize(myStruct)
parsed, err := jsonutil.Parse[MyStruct](data)
```
