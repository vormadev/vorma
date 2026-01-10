---
title: reflectutil
description:
    Reflection utilities for interface checking, nil detection, and JSON tag
    parsing.
---

```go
import "github.com/vormadev/vorma/kit/reflectutil"
```

## Interface Checking

Check if type implements interface (also checks pointer-to-type):

```go
func ImplementsInterface(t reflect.Type, iface reflect.Type) bool
```

Get `reflect.Type` for an interface:

```go
func ToInterfaceReflectType[T any]() reflect.Type
```

Example:

```go
iface := reflectutil.ToInterfaceReflectType[io.Reader]()
if reflectutil.ImplementsInterface(t, iface) { ... }
```

## Nil Detection

Recursively check if value is nil or ultimately points to nil; treats `struct{}`
(None) as non-nil:

```go
func ExcludingNoneGetIsNilOrUltimatelyPointsToNil(v any) bool
```

## JSON Tags

Extract JSON field name from struct field tag (returns "" if "-"):

```go
func GetJSONFieldName(field reflect.StructField) string
```
