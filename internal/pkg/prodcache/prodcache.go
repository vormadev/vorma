package prodcache

import (
	"sync"
	"sync/atomic"
)

// Cache is a lazy-init cache that skips caching in dev mode.
// In prod: value is computed once, cached forever (errors are not cached).
// In dev: value is recomputed on every call (so data is never stale).
type Cache[T any] struct {
	val    atomic.Pointer[cache_result[T]]
	mu     sync.Mutex
	fn     func() (T, error)
	is_dev func() bool
}

type cache_result[T any] struct {
	val T
}

func New[T any](is_dev func() bool, fn func() (T, error)) *Cache[T] {
	return &Cache[T]{fn: fn, is_dev: is_dev}
}

func (c *Cache[T]) Get() (T, error) {
	if c.is_dev() {
		return c.fn()
	}
	if r := c.val.Load(); r != nil {
		return r.val, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if r := c.val.Load(); r != nil {
		return r.val, nil
	}
	val, err := c.fn()
	if err == nil {
		c.val.Store(&cache_result[T]{val: val})
	}
	return val, err
}
