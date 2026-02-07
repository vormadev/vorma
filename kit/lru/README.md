# kit/lru

`github.com/vormadev/vorma/kit/lru`

Concurrent generic LRU cache with optional TTL expiration and spam-aware recency behavior.

## Import

```go
import "github.com/vormadev/vorma/kit/lru"
```

## Quick Start

```go
cache := lru.NewCacheWithTTL[string, User](1000, 15*time.Minute)
defer cache.Close()

cache.Set("user:123", user, false) // non-spam
u, ok := cache.Get("user:123")
_ = u
_ = ok
```

## Recency Behavior

- `isSpam=false`: `Get` and updates move item to front (hot keys retained).
- `isSpam=true`: accesses do not refresh recency (spam keys are easier to evict).

## TTL Behavior

- `NewCache(maxItems)`: no default TTL.
- `NewCacheWithTTL(maxItems, defaultTTL)`: `Set` uses `defaultTTL`.
- `SetWithTTL(..., ttl=0)`: item does not expire by time.
- Expired items are removed on `Get` and by cleanup sweeps.

Background cleanup:

- Starts only when `defaultTTL > 0`.
- Interval is `defaultTTL/2`, clamped to `[1s, 1m]`.
- Call `Close()` when done to stop the cleanup goroutine.

## Capacity Behavior

- `maxItems <= 0`: cache stores nothing.
- At capacity, insertion evicts least-recently-used item.

## API Reference

- `type Cache[K comparable, V any]`
- `func NewCache[K comparable, V any](maxItems int) *Cache[K, V]`
- `func NewCacheWithTTL[K comparable, V any](maxItems int, defaultTTL time.Duration) *Cache[K, V]`
- `func (c *Cache[K, V]) Get(key K) (v V, found bool)`
- `func (c *Cache[K, V]) Set(key K, value V, isSpam bool)`
- `func (c *Cache[K, V]) SetWithTTL(key K, value V, isSpam bool, ttl time.Duration)`
- `func (c *Cache[K, V]) Delete(key K)`
- `func (c *Cache[K, V]) CleanupExpired()`
- `func (c *Cache[K, V]) Close()`
