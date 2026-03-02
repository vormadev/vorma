// Package wavecache provides low-level runtime cache primitives shared by Wave
// runtime services.
package wavecache

import "sync"

// ValueCache caches one lazily computed value in production mode, while
// recomputing on every access in development mode.
type ValueCache[T any] struct {
	val       T
	err       error
	once      sync.Once
	fn        func() (T, error)
	isDevMode func() bool
}

// NewValueCache creates a ValueCache with production-only caching behavior.
func NewValueCache[T any](fn func() (T, error)) *ValueCache[T] {
	return NewValueCacheWithModeResolver(fn, nil)
}

// NewValueCacheWithModeResolver creates a ValueCache with one mode resolver.
func NewValueCacheWithModeResolver[T any](
	fn func() (T, error),
	isDevMode func() bool,
) *ValueCache[T] {
	if isDevMode == nil {
		isDevMode = func() bool { return false }
	}
	return &ValueCache[T]{
		fn:        fn,
		isDevMode: isDevMode,
	}
}

// Get resolves and returns the cached value.
func (cache *ValueCache[T]) Get() (T, error) {
	if cache.isDevMode() {
		return cache.fn()
	}
	cache.once.Do(func() { cache.val, cache.err = cache.fn() })
	return cache.val, cache.err
}

// KeyedCache caches keyed values in production mode, while recomputing on each
// key access in development mode.
type KeyedCache[K comparable, V any] struct {
	entries     sync.Map
	fn          func(K) (V, error)
	shouldCache func(V, error) bool
	isDevMode   func() bool
}

type keyedCacheEntry[V any] struct {
	once sync.Once
	val  V
	err  error
}

// NewKeyedCache creates a KeyedCache with default successful-result caching.
func NewKeyedCache[K comparable, V any](
	fn func(K) (V, error),
) *KeyedCache[K, V] {
	return NewKeyedCacheWithPolicyAndModeResolver(fn, nil, nil)
}

// NewKeyedCacheWithPolicyAndModeResolver creates a KeyedCache with custom
// retention and mode policy.
func NewKeyedCacheWithPolicyAndModeResolver[K comparable, V any](
	fn func(K) (V, error),
	shouldCache func(V, error) bool,
	isDevMode func() bool,
) *KeyedCache[K, V] {
	if shouldCache == nil {
		shouldCache = func(_ V, err error) bool { return err == nil }
	}
	if isDevMode == nil {
		isDevMode = func() bool { return false }
	}
	return &KeyedCache[K, V]{
		fn:          fn,
		shouldCache: shouldCache,
		isDevMode:   isDevMode,
	}
}

// Get resolves and returns one keyed value.
func (cache *KeyedCache[K, V]) Get(key K) (V, error) {
	if cache.isDevMode() {
		return cache.fn(key)
	}

	entryAny, _ := cache.entries.LoadOrStore(key, &keyedCacheEntry[V]{})
	entry := entryAny.(*keyedCacheEntry[V])

	entry.once.Do(func() {
		entry.val, entry.err = cache.fn(key)
		if cache.shouldCache != nil && !cache.shouldCache(entry.val, entry.err) {
			cache.entries.Delete(key)
		}
	})

	if entry.err != nil {
		var zero V
		return zero, entry.err
	}

	return entry.val, nil
}
