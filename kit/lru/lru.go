// Package lru implements a generic Least Recently Used (LRU) cache
// with support for "spam" items and TTL (Time-To-Live). The cache maintains a maximum
// number of items, evicting the least recently used item when the
// limit is reached. Items can be added, retrieved, and deleted from
// the cache. Regular items are moved to the front of the cache when
// accessed or updated, while "spam" items maintain their position.
// Items can expire based on their TTL settings.
// This allows for preferential treatment of non-spam items in terms
// of retention, while still caching spam items. The cache is safe
// for concurrent use.
package lru

import (
	"container/list"
	"sync"
	"time"
)

// item represents a key-value pair in the cache.
type item[K comparable, V any] struct {
	key       K
	value     V
	element   *list.Element
	isSpam    bool
	expiresAt time.Time
}

// Cache is a generic LRU cache that supports "spam" items and TTL.
// Caches should be created by calling NewCache or NewCacheWithTTL.
type Cache[K comparable, V any] struct {
	mu          sync.RWMutex
	items       map[K]*item[K, V]
	order       *list.List
	maxItems    int
	defaultTTL  time.Duration
	cleanupDone chan struct{} // Used to signal when the cleanup goroutine is done
	closeOnce   sync.Once
}

// NewCache creates a new LRU cache with the specified maximum number of items.
func NewCache[K comparable, V any](maxItems int) *Cache[K, V] {
	return NewCacheWithTTL[K, V](maxItems, 0) // 0 means no default TTL
}

// NewCacheWithTTL creates a new LRU cache with the specified maximum number of items
// and default TTL duration. When you are done with the cache, you should call Close
// to stop the background cleanup goroutine.
func NewCacheWithTTL[K comparable, V any](maxItems int, defaultTTL time.Duration) *Cache[K, V] {
	if maxItems < 0 {
		maxItems = 0
	}

	cache := &Cache[K, V]{
		items:       make(map[K]*item[K, V]),
		order:       list.New(),
		maxItems:    maxItems,
		defaultTTL:  defaultTTL,
		cleanupDone: make(chan struct{}),
	}

	// Only start the cleanup goroutine if a default TTL is set
	if defaultTTL > 0 {
		go cache.startCleanupLoop()
	}

	return cache
}

// Get retrieves an item from the cache, moving non-spam items to the front.
// It returns the value and a boolean indicating whether the key was found.
// If the item has expired, it will be removed and not returned.
func (c *Cache[K, V]) Get(key K) (v V, found bool) {
	c.mu.RLock()
	itm, found := c.items[key]
	c.mu.RUnlock()

	if !found {
		return
	}

	// Check if the item has expired
	if !itm.expiresAt.IsZero() && time.Now().After(itm.expiresAt) {
		c.Delete(key)
		var zero V
		return zero, false
	}

	if !itm.isSpam {
		c.mu.Lock()
		c.order.MoveToFront(itm.element)
		c.mu.Unlock()
	}

	return itm.value, true
}

// Set adds or updates an item in the cache, evicting the LRU item if necessary.
// The isSpam parameter determines whether the item should be treated as spam.
// If the item is spam, it will not be moved to the front of the cache.
// Uses the default TTL if one was specified when creating the cache.
func (c *Cache[K, V]) Set(key K, value V, isSpam bool) {
	c.SetWithTTL(key, value, isSpam, c.defaultTTL)
}

// SetWithTTL adds or updates an item in the cache with a specific TTL value.
// A ttl of 0 means the item will not expire based on time.
func (c *Cache[K, V]) SetWithTTL(key K, value V, isSpam bool, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	if itm, found := c.items[key]; found {
		itm.value = value
		itm.isSpam = isSpam
		itm.expiresAt = expiresAt
		if !isSpam {
			c.order.MoveToFront(itm.element)
		}
		return
	}

	if c.maxItems <= 0 {
		return
	}

	if len(c.items) >= c.maxItems {
		c.evict()
	}

	itm := &item[K, V]{
		key:       key,
		value:     value,
		isSpam:    isSpam,
		expiresAt: expiresAt,
	}
	element := c.order.PushFront(itm)
	itm.element = element
	c.items[key] = itm
}

// Delete removes an item from the cache if it exists.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	itm, found := c.items[key]
	if !found {
		return
	}

	delete(c.items, key)
	c.order.Remove(itm.element)
}

// evict removes the least recently used item from the cache.
func (c *Cache[K, V]) evict() {
	back := c.order.Back()
	if back != nil {
		itm := back.Value.(*item[K, V])
		delete(c.items, itm.key)
		c.order.Remove(back)
	}
}

// CleanupExpired removes all expired items from the cache.
func (c *Cache[K, V]) CleanupExpired() {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	var keysToDelete []K

	// First collect keys to delete to avoid modifying the map during iteration
	for key, itm := range c.items {
		if !itm.expiresAt.IsZero() && now.After(itm.expiresAt) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Then delete those keys
	for _, key := range keysToDelete {
		if itm, found := c.items[key]; found {
			delete(c.items, key)
			c.order.Remove(itm.element)
		}
	}
}

// startCleanupLoop runs a goroutine that periodically cleans up expired items.
// The cleanup interval is set to half of the default TTL to ensure timely cleanups.
func (c *Cache[K, V]) startCleanupLoop() {
	// Use half the TTL as cleanup interval, but cap at 1 minute maximum
	interval := max(min(c.defaultTTL/2, time.Minute), time.Second)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.CleanupExpired()
		case <-c.cleanupDone:
			return
		}
	}
}

// Close stops the background cleanup goroutine if it's running.
// It should be called when the cache is no longer needed to prevent resource leaks.
func (c *Cache[K, V]) Close() {
	if c.defaultTTL > 0 {
		c.closeOnce.Do(func() { close(c.cleanupDone) })
	}
}
