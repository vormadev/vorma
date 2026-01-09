package typed

import (
	"sync"
	"sync/atomic"
)

// Do not instantiate syncMap directly; use NewSyncMap.
type SyncMap__[K comparable, V any] struct {
	m atomic.Pointer[sync.Map]
}

func NewSyncMap[K comparable, V any]() *SyncMap__[K, V] {
	sm := &SyncMap__[K, V]{}
	sm.m.Store(&sync.Map{})
	return sm
}

func (sm *SyncMap__[K, V]) Load(key K) (value V, ok bool) {
	v, ok := sm.m.Load().Load(key)
	if !ok {
		return value, false
	}
	return v.(V), true
}

func (sm *SyncMap__[K, V]) Store(key K, value V) {
	sm.m.Load().Store(key, value)
}

func (sm *SyncMap__[K, V]) Delete(key K) {
	sm.m.Load().Delete(key)
}

func (sm *SyncMap__[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	v, loaded := sm.m.Load().LoadOrStore(key, value)
	return v.(V), loaded
}

func (sm *SyncMap__[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := sm.m.Load().LoadAndDelete(key)
	if !loaded {
		return value, false
	}
	return v.(V), loaded
}

func (sm *SyncMap__[K, V]) Range(f func(key K, value V) bool) {
	sm.m.Load().Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

func (sm *SyncMap__[K, V]) Clear() {
	sm.m.Store(&sync.Map{})
}
