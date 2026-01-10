---
title: contextutil
description:
    Type-safe generic context value storage for context.Context and
    http.Request.
---

```go
import "github.com/vormadev/vorma/kit/contextutil"
```

## Store

Generic typed context value store:

```go
type Store[T any] struct{}
```

Create a store with a unique key:

```go
func NewStore[T any](key string) *Store[T]
```

## Methods

Add value to context:

```go
func (s *Store[T]) GetContextWithValue(c context.Context, val T) context.Context
```

Retrieve value from context (returns zero value if missing):

```go
func (s *Store[T]) GetValueFromContext(c context.Context) T
```

Add value to request's context:

```go
func (s *Store[T]) GetRequestWithContext(r *http.Request, val T) *http.Request
```

## Example

```go
var userStore = contextutil.NewStore[User]("user")

// Set
ctx = userStore.GetContextWithValue(ctx, user)

// Get
user := userStore.GetValueFromContext(ctx)
```
