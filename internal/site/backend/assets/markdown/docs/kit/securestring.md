---
title: securestring
description:
    Base64-encoded wrapper over securebytes for string-safe encrypted values.
---

```go
import "github.com/vormadev/vorma/kit/securestring"
```

## Types

```go
type SecureString string          // base64-encoded encrypted value
const MaxBase64Size               // ~1.33MB limit
```

## Functions

Serialize value to base64-encoded encrypted string:

```go
func Serialize(ks *keyset.Keyset, rv securebytes.RawValue) (SecureString, error)
```

Parse base64-encoded encrypted string back to typed value:

```go
func Parse[T any](ks *keyset.Keyset, ss SecureString) (T, error)
```

## Example

```go
ss, err := securestring.Serialize(keys(), myStruct)
// ss is safe for cookies, URLs, JSON, etc.

result, err := securestring.Parse[MyStruct](keys(), ss)
```
