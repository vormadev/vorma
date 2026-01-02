// Package safecache provides a generic, thread-safe, lazily initiated cache that
// ensures initialization occurs only once unless bypass is requested.
package safecache

import (
	"sync"

	"github.com/vormadev/vorma/kit/typed"
)

// Cache is a generic, thread-safe cache that ensures initialization occurs only
// once unless bypass is requested.
type Cache[T any] struct {
	val        T
	once       sync.Once
	initFunc   func() (T, error)
	bypassFunc func() bool
}

// New creates a new Cache instance. If bypassFunc is provided and returns true,
// initFunc will run every time. Panics if initFunc is nil.
func New[T any](initFunc func() (T, error), bypassFunc func() bool) *Cache[T] {
	if initFunc == nil {
		panic("initFunc must not be nil")
	}
	return &Cache[T]{
		initFunc:   initFunc,
		bypassFunc: bypassFunc,
	}
}

// Get retrieves the cached value, initializing it if necessary or bypassing the cache.
func (c *Cache[T]) Get() (T, error) {
	if c.bypassFunc != nil && (c.bypassFunc)() {
		return c.initFunc()
	}

	var err error
	c.once.Do(func() { c.val, err = c.initFunc() })

	return c.val, err
}

// mapInitFunc defines the initialization function for each key in the CacheMap.
type mapInitFunc[K any, V any] func(key K) (V, error)

// mapBypassFunc defines the bypass function for each key in the CacheMap.
type mapBypassFunc[K any] func(key K) bool

// mapToKeyFunc defines the function to derive the key used to store/retrieve
// values in the CacheMap.
type mapToKeyFunc[K any, DK comparable] func(key K) DK

// CacheMap is a generic, thread-safe cache map that caches values based on
// derived keys.
type CacheMap[K any, DK comparable, V any] struct {
	cache         typed.SyncMap[DK, *Cache[V]]
	mapInitFunc   mapInitFunc[K, V]
	mapBypassFunc mapBypassFunc[K]
	mapToKeyFunc  mapToKeyFunc[K, DK]
}

// NewMap creates a new CacheMap instance. If bypassFunc is provided and returns
// true, initFunc will run every time. Panics if initFunc or mapToKeyFunc is nil.
func NewMap[K any, DK comparable, V any](
	initFunc mapInitFunc[K, V],
	mapToKeyFunc mapToKeyFunc[K, DK],
	bypassFunc mapBypassFunc[K],
) *CacheMap[K, DK, V] {
	if initFunc == nil {
		panic("initFunc must not be nil")
	}
	if mapToKeyFunc == nil {
		panic("mapToKeyFunc must not be nil")
	}
	return &CacheMap[K, DK, V]{
		cache:         typed.SyncMap[DK, *Cache[V]]{},
		mapInitFunc:   initFunc,
		mapToKeyFunc:  mapToKeyFunc,
		mapBypassFunc: bypassFunc,
	}
}

// Get retrieves the cached value for the given key, initializing it if necessary
// or bypassing the cache.
func (c *CacheMap[K, DK, V]) Get(key K) (V, error) {
	if c.mapBypassFunc != nil && (c.mapBypassFunc)(key) {
		return c.mapInitFunc(key)
	}

	itemCache, _ := c.cache.LoadOrStore(
		c.mapToKeyFunc(key),
		New(func() (V, error) { return c.mapInitFunc(key) }, nil),
	)

	return itemCache.Get()
}
