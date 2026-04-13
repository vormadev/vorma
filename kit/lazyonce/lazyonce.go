package lazyonce

import "sync"

type Cache[T any] struct {
	get       func() T
	init_once sync.Once
}

func (v *Cache[T]) Get(initFunc func() T) T {
	v.init_once.Do(func() { v.get = sync.OnceValue(initFunc) })
	return v.get()
}

func Func(fn func()) func() {
	return sync.OnceFunc(fn)
}

func Getter[T any](fn func() T) func() T {
	return sync.OnceValue(fn)
}

func GetterWithErr[T any](fn func() (T, error)) func() (T, error) {
	return sync.OnceValues(fn)
}
