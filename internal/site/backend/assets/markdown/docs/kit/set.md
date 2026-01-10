---
title: set
description: Generic set implementation using map[T]struct{}.
---

```go
import "github.com/vormadev/vorma/kit/set"
```

## Type

```go
type Set[T comparable] map[T]struct{}
```

## Functions

```go
func New[T comparable]() Set[T]
func (s Set[T]) Add(val T) Set[T]  // chainable
func (s Set[T]) Contains(val T) bool
```

## Example

```go
s := set.New[string]().Add("a").Add("b")
s.Contains("a")  // true
s.Contains("c")  // false
```
