package wave

import "sync"

// cache holds a lazily-initialized value that is cached in prod but
// recomputed on every access in dev mode.
type cache[T any] struct {
	val       T
	err       error
	once      sync.Once
	fn        func() (T, error)
	isDevMode func() bool
}

func newCache[T any](fn func() (T, error)) *cache[T] {
	return newCacheWithModeResolver(fn, GetIsDev)
}

func newCacheWithModeResolver[T any](
	fn func() (T, error),
	isDevMode func() bool,
) *cache[T] {
	if isDevMode == nil {
		isDevMode = GetIsDev
	}
	return &cache[T]{
		fn:        fn,
		isDevMode: isDevMode,
	}
}

func (c *cache[T]) get() (T, error) {
	if c.isDevMode() {
		return c.fn()
	}
	c.once.Do(func() { c.val, c.err = c.fn() })
	return c.val, c.err
}

// cacheMap holds lazily-initialized keyed values that are cached in prod
// but recomputed on every access in dev mode.
type cacheMap[K comparable, V any] struct {
	m           sync.Map
	fn          func(K) (V, error)
	shouldCache func(V, error) bool
	isDevMode   func() bool
}

type cacheMapEntry[V any] struct {
	once sync.Once
	val  V
	err  error
}

func newCacheMap[K comparable, V any](fn func(K) (V, error)) *cacheMap[K, V] {
	return newCacheMapWithPolicyAndModeResolver(fn, nil, GetIsDev)
}

func newCacheMapWithPolicyAndModeResolver[K comparable, V any](
	fn func(K) (V, error),
	shouldCache func(V, error) bool,
	isDevMode func() bool,
) *cacheMap[K, V] {
	if shouldCache == nil {
		shouldCache = func(_ V, err error) bool {
			return err == nil
		}
	}
	if isDevMode == nil {
		isDevMode = GetIsDev
	}

	return &cacheMap[K, V]{
		fn:          fn,
		shouldCache: shouldCache,
		isDevMode:   isDevMode,
	}
}

func (c *cacheMap[K, V]) get(key K) (V, error) {
	if c.isDevMode() {
		return c.fn(key)
	}

	entryAny, _ := c.m.LoadOrStore(key, &cacheMapEntry[V]{})
	entry := entryAny.(*cacheMapEntry[V])

	entry.once.Do(func() {
		entry.val, entry.err = c.fn(key)
		if c.shouldCache != nil && !c.shouldCache(entry.val, entry.err) {
			c.m.Delete(key)
		}
	})

	if entry.err != nil {
		var zero V
		return zero, entry.err
	}

	return entry.val, nil
}
