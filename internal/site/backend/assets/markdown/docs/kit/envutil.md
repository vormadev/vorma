---
title: envutil
description: Environment variable getters with default values and type parsing.
---

```go
import "github.com/vormadev/vorma/kit/envutil"
```

## Functions

All functions return `defaultValue` if the env var is unset or unparseable.

```go
func GetStr(key string, defaultValue string) string
func GetInt(key string, defaultValue int) int
func GetBool(key string, defaultValue bool) bool
```

Example:

```go
port := envutil.GetInt("PORT", 8080)
debug := envutil.GetBool("DEBUG", false)
```
