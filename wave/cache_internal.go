package wave

import "sync"

// cache holds a lazily-initialized value that is cached in prod but
// recomputed on every access in dev mode.
type cache[T any] struct {
	val  T
	err  error
	once sync.Once
	fn   func() (T, error)
}

func newCache[T any](fn func() (T, error)) *cache[T] {
	return &cache[T]{fn: fn}
}

func (c *cache[T]) get() (T, error) {
	if GetIsDev() {
		return c.fn()
	}
	c.once.Do(func() { c.val, c.err = c.fn() })
	return c.val, c.err
}

// cacheMap holds lazily-initialized keyed values that are cached in prod
// but recomputed on every access in dev mode.
type cacheMap[K comparable, V any] struct {
	m  sync.Map
	fn func(K) (V, error)
}

type cacheMapEntry[V any] struct {
	once sync.Once
	val  V
	err  error
}

func newCacheMap[K comparable, V any](fn func(K) (V, error)) *cacheMap[K, V] {
	return &cacheMap[K, V]{fn: fn}
}

func (c *cacheMap[K, V]) get(key K) (V, error) {
	if GetIsDev() {
		return c.fn(key)
	}

	entryAny, _ := c.m.LoadOrStore(key, &cacheMapEntry[V]{})
	entry := entryAny.(*cacheMapEntry[V])

	entry.once.Do(func() {
		entry.val, entry.err = c.fn(key)
		if entry.err != nil {
			c.m.Delete(key)
		}
	})

	if entry.err != nil {
		var zero V
		return zero, entry.err
	}

	return entry.val, nil
}
