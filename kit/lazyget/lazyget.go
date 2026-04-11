package lazyget

import "sync"

type Cache[T any] struct {
	get       func() T
	init_once sync.Once
}

func (v *Cache[T]) Get(initFunc func() T) T {
	v.init_once.Do(func() { v.get = sync.OnceValue(initFunc) })
	return v.get()
}

// Extremely light and arguably pointless wrapper over sync.OnceValue
func Getter[T any](fn func() T) func() T {
	return sync.OnceValue(fn)
}
