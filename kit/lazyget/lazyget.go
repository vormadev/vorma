package lazyget

import "sync"

type Cache[T any] struct {
	val  T
	once sync.Once
}

func (v *Cache[T]) Get(initFunc func() T) T {
	v.once.Do(func() { v.val = initFunc() })
	return v.val
}

func New[T any](fn func() T) func() T {
	var v Cache[T]
	return func() T { return v.Get(fn) }
}
