package lazyget

import "sync"

type Cache[T any] struct {
	get      func() T
	initOnce sync.Once
}

func (v *Cache[T]) Get(initFunc func() T) T {
	v.initOnce.Do(func() { v.get = sync.OnceValue(initFunc) })
	return v.get()
}

// Deprecated: use sync.OnceValue directly for package-level lazy getters.
func New[T any](fn func() T) func() T { return sync.OnceValue(fn) }
