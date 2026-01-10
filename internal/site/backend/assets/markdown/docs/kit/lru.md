---
title: lru
description: Generic LRU cache with TTL support and spam-item handling.
---

```go
import "github.com/vormadev/vorma/kit/lru"
```

## Constructor

```go
func NewCache[K comparable, V any](maxItems int) *Cache[K, V]
func NewCacheWithTTL[K comparable, V any](maxItems int, defaultTTL time.Duration) *Cache[K, V]
```

## Methods

```go
func (c *Cache[K, V]) Get(key K) (V, bool)
func (c *Cache[K, V]) Set(key K, value V, isSpam bool)
func (c *Cache[K, V]) SetWithTTL(key K, value V, isSpam bool, ttl time.Duration)
func (c *Cache[K, V]) Delete(key K)
func (c *Cache[K, V]) CleanupExpired()
func (c *Cache[K, V]) Close()
```

## Behavior

- Non-spam items move to front on access/update (standard LRU)
- Spam items maintain position (deprioritized for retention, drift toward
  eviction)
- TTL caches run background cleanup; call `Close()` when done
- Thread-safe

## Spam Use Case

Mark items from suspected bots/abusers as spam to prevent cache pollution. Spam
items are still cached (avoiding repeated expensive lookups), but their accesses
don't refresh their position—so they naturally evict first while legitimate
users' data stays hot.

## Example

```go
cache := lru.NewCacheWithTTL[string, User](1000, 15*time.Minute)
defer cache.Close()

cache.Set("user:123", user, false)       // normal item
cache.Set("user:bot", botUser, true)     // spam item (lower retention priority)

user, found := cache.Get("user:123")
```
