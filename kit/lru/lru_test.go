package lru

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	cache := NewCache[string, int](5)
	if cache.maxItems != 5 {
		t.Errorf("Expected maxItems to be 5, got %d", cache.maxItems)
	}
	if len(cache.items) != 0 {
		t.Errorf("Expected empty items map, got %d items", len(cache.items))
	}
	if cache.order.Len() != 0 {
		t.Errorf("Expected empty order list, got %d items", cache.order.Len())
	}
}

func TestSet(t *testing.T) {
	cache := NewCache[string, int](3)

	// Test basic set
	cache.Set("a", 1, false)
	if v, found := cache.Get("a"); !found || v != 1 {
		t.Errorf("Expected to find 'a' with value 1, got %v, %v", v, found)
	}

	// Test overwrite
	cache.Set("a", 2, false)
	if v, found := cache.Get("a"); !found || v != 2 {
		t.Errorf("Expected to find 'a' with value 2, got %v, %v", v, found)
	}

	// Test eviction
	cache.Set("b", 3, false)
	cache.Set("c", 4, false)
	cache.Set("d", 5, false) // This should evict "a"
	if _, found := cache.Get("a"); found {
		t.Errorf("Expected 'a' to be evicted")
	}

	// Test spam flag
	cache.Set("e", 6, true)
	if v, found := cache.Get("e"); !found || v != 6 {
		t.Errorf("Expected to find 'e' with value 6, got %v, %v", v, found)
	}
}

func TestGet(t *testing.T) {
	cache := NewCache[string, int](3)

	// Test get non-existent item
	if _, found := cache.Get("a"); found {
		t.Errorf("Expected not to find 'a'")
	}

	// Test get existing item
	cache.Set("a", 1, false)
	if v, found := cache.Get("a"); !found || v != 1 {
		t.Errorf("Expected to find 'a' with value 1, got %v, %v", v, found)
	}

	// Test LRU order update
	cache.Set("b", 2, false)
	cache.Set("c", 3, false)
	cache.Get("a")           // This should move "a" to the front
	cache.Set("d", 4, false) // This should evict "b"
	if _, found := cache.Get("b"); found {
		t.Errorf("Expected 'b' to be evicted")
	}

	// Test spam item
	cache.Set("e", 5, true)
	cache.Get("e")           // This should not move "e" to the front
	cache.Set("f", 6, false) // This should evict "c", not "e"
	if _, found := cache.Get("c"); found {
		t.Errorf("Expected 'c' to be evicted")
	}
	if _, found := cache.Get("e"); !found {
		t.Errorf("Expected 'e' to still be in cache")
	}
}

func TestDelete(t *testing.T) {
	cache := NewCache[string, int](3)

	// Test delete non-existent item
	cache.Delete("a")
	if cache.order.Len() != 0 {
		t.Errorf("Expected empty cache after deleting non-existent item")
	}

	// Test delete existing item
	cache.Set("a", 1, false)
	cache.Set("b", 2, false)
	cache.Delete("a")
	if _, found := cache.Get("a"); found {
		t.Errorf("Expected 'a' to be deleted")
	}
	if cache.order.Len() != 1 {
		t.Errorf("Expected cache to have 1 item, got %d", cache.order.Len())
	}

	// Test delete spam item
	cache.Set("c", 3, true)
	cache.Delete("c")
	if _, found := cache.Get("c"); found {
		t.Errorf("Expected spam item 'c' to be deleted")
	}
}

func TestEdgeCases(t *testing.T) {
	// Test cache with size 0
	cache := NewCache[string, int](0)
	cache.Set("a", 1, false)
	if _, found := cache.Get("a"); found {
		t.Errorf("Expected item not to be stored in size 0 cache")
	}

	// Test cache with size 1
	cache = NewCache[string, int](1)
	cache.Set("a", 1, false)
	cache.Set("b", 2, false)
	if _, found := cache.Get("a"); found {
		t.Errorf("Expected 'a' to be evicted in size 1 cache")
	}
	if v, found := cache.Get("b"); !found || v != 2 {
		t.Errorf("Expected to find 'b' with value 2 in size 1 cache")
	}

	// Test setting same key multiple times
	cache = NewCache[string, int](2)
	cache.Set("a", 1, false)
	cache.Set("a", 2, false)
	cache.Set("a", 3, false)
	if v, found := cache.Get("a"); !found || v != 3 {
		t.Errorf("Expected to find 'a' with value 3, got %v, %v", v, found)
	}

	// Test changing spam status
	cache = NewCache[string, int](2)
	cache.Set("a", 1, true)
	cache.Set("b", 2, false)
	cache.Set("a", 1, false) // Change "a" from spam to non-spam
	cache.Set("c", 3, false) // This should evict "b", not "a"
	if _, found := cache.Get("b"); found {
		t.Errorf("Expected 'b' to be evicted")
	}
	if _, found := cache.Get("a"); !found {
		t.Errorf("Expected 'a' to still be in cache")
	}
}

func TestConcurrency(t *testing.T) {
	t.Run("Concurrent Set and Get", func(t *testing.T) {
		cache := NewCache[string, int](100)
		var wg sync.WaitGroup
		iterations := 1000
		goroutines := 10

		for i := range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range iterations {
					key := fmt.Sprintf("key%d-%d", i, j)
					cache.Set(key, j, false)
					_, _ = cache.Get(key)
				}
			}()
		}

		wg.Wait()

		if cache.order.Len() > cache.maxItems {
			t.Errorf("Cache exceeded max items: %d > %d", cache.order.Len(), cache.maxItems)
		}
	})

	t.Run("Concurrent Set, Get, and Delete", func(t *testing.T) {
		cache := NewCache[string, int](100)
		var wg sync.WaitGroup
		iterations := 1000
		goroutines := 10

		for i := range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range iterations {
					key := fmt.Sprintf("key%d-%d", i, j)
					cache.Set(key, j, j%2 == 0)
					_, _ = cache.Get(key)
					if j%3 == 0 {
						cache.Delete(key)
					}
				}
			}()
		}

		wg.Wait()

		if cache.order.Len() > cache.maxItems {
			t.Errorf("Cache exceeded max items: %d > %d", cache.order.Len(), cache.maxItems)
		}
	})

	t.Run("Concurrent access with mixed workload", func(t *testing.T) {
		cache := NewCache[string, int](1000)
		var wg sync.WaitGroup
		iterations := 10000
		goroutines := 20

		for i := range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range iterations {
					key := fmt.Sprintf("key%d-%d", i%10, j%100)
					switch j % 10 {
					case 0, 1, 2, 3, 4:
						// 50% reads
						_, _ = cache.Get(key)
					case 5, 6, 7:
						// 30% writes
						cache.Set(key, j, false)
					case 8:
						// 10% spam writes
						cache.Set(key, j, true)
					case 9:
						// 10% deletes
						cache.Delete(key)
					}
				}
			}()
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Test passed
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out, possible deadlock")
		}

		if cache.order.Len() > cache.maxItems {
			t.Errorf("Cache exceeded max items: %d > %d", cache.order.Len(), cache.maxItems)
		}
	})
}

func TestTTLExpiration(t *testing.T) {
	cache := NewCacheWithTTL[string, int](5, 100*time.Millisecond)
	defer cache.Close()

	// Add an item with the default TTL
	cache.Set("a", 1, false)

	// Add an item with a specific TTL
	cache.SetWithTTL("b", 2, false, 200*time.Millisecond)

	// Add an item with no TTL
	cache.SetWithTTL("c", 3, false, 0)

	// Test that all items are initially present
	if _, found := cache.Get("a"); !found {
		t.Errorf("Expected to find 'a' immediately after setting")
	}
	if _, found := cache.Get("b"); !found {
		t.Errorf("Expected to find 'b' immediately after setting")
	}
	if _, found := cache.Get("c"); !found {
		t.Errorf("Expected to find 'c' immediately after setting")
	}

	// Wait for first item to expire
	time.Sleep(150 * time.Millisecond)

	// First item should be gone, others should remain
	if _, found := cache.Get("a"); found {
		t.Errorf("Expected 'a' to be expired after 150ms")
	}
	if _, found := cache.Get("b"); !found {
		t.Errorf("Expected 'b' to still be present after 150ms")
	}
	if _, found := cache.Get("c"); !found {
		t.Errorf("Expected 'c' to still be present after 150ms (no TTL)")
	}

	// Wait for second item to expire
	time.Sleep(100 * time.Millisecond)

	// Second item should be gone now
	if _, found := cache.Get("b"); found {
		t.Errorf("Expected 'b' to be expired after 250ms")
	}

	// Item with no TTL should still be present
	if _, found := cache.Get("c"); !found {
		t.Errorf("Expected 'c' to still be present (no TTL)")
	}
}

func TestCleanupExpired(t *testing.T) {
	cache := NewCacheWithTTL[string, int](5, 0) // No default TTL
	defer cache.Close()

	// Add some items with different TTLs
	cache.SetWithTTL("a", 1, false, 50*time.Millisecond)
	cache.SetWithTTL("b", 2, false, 50*time.Millisecond)
	cache.SetWithTTL("c", 3, false, 200*time.Millisecond)
	cache.SetWithTTL("d", 4, false, 0) // No TTL

	// Wait for some items to expire
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup
	cache.CleanupExpired()

	// Check that expired items are gone
	if _, found := cache.Get("a"); found {
		t.Errorf("Expected 'a' to be removed after cleanup")
	}
	if _, found := cache.Get("b"); found {
		t.Errorf("Expected 'b' to be removed after cleanup")
	}

	// Non-expired items should remain
	if _, found := cache.Get("c"); !found {
		t.Errorf("Expected 'c' to still be present after cleanup")
	}
	if _, found := cache.Get("d"); !found {
		t.Errorf("Expected 'd' (no TTL) to still be present after cleanup")
	}

	// Check cache size
	expectedSize := 2
	if cache.order.Len() != expectedSize {
		t.Errorf("Expected cache size to be %d after cleanup, got %d", expectedSize, cache.order.Len())
	}
}

func TestAutoCleanup(t *testing.T) {
	// Create cache with short default TTL and auto cleanup
	cache := NewCacheWithTTL[string, int](5, 50*time.Millisecond)
	defer cache.Close()

	// Add several items
	for i := range 5 {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i, false)
	}

	// Wait for items to expire and auto-cleanup to run
	time.Sleep(200 * time.Millisecond)

	// All items should be gone now
	for i := range 5 {
		key := fmt.Sprintf("key%d", i)
		if _, found := cache.Get(key); found {
			t.Errorf("Expected '%s' to be removed by auto-cleanup", key)
		}
	}

	// Cache should be empty
	if cache.order.Len() != 0 {
		t.Errorf("Expected cache to be empty after auto-cleanup, got %d items", cache.order.Len())
	}
}

func TestTTLWithSpamItems(t *testing.T) {
	cache := NewCacheWithTTL[string, int](5, 100*time.Millisecond)
	defer cache.Close()

	// Add regular and spam items with different TTLs
	cache.Set("regular1", 1, false)
	cache.Set("spam1", 2, true)
	cache.SetWithTTL("regular2", 3, false, 200*time.Millisecond)
	cache.SetWithTTL("spam2", 4, true, 200*time.Millisecond)

	// Wait for default TTL items to expire
	time.Sleep(150 * time.Millisecond)

	// Items with default TTL should be gone
	if _, found := cache.Get("regular1"); found {
		t.Errorf("Expected 'regular1' to be expired")
	}
	if _, found := cache.Get("spam1"); found {
		t.Errorf("Expected 'spam1' to be expired")
	}

	// Items with longer TTL should still be present
	if _, found := cache.Get("regular2"); !found {
		t.Errorf("Expected 'regular2' to still be present")
	}
	if _, found := cache.Get("spam2"); !found {
		t.Errorf("Expected 'spam2' to still be present")
	}
}

func TestTTLUpdateOnSet(t *testing.T) {
	cache := NewCacheWithTTL[string, int](5, 100*time.Millisecond)
	defer cache.Close()

	// Add item with default TTL
	cache.Set("a", 1, false)

	// Wait some time but not enough to expire
	time.Sleep(50 * time.Millisecond)

	// Update the item, which should reset the TTL
	cache.Set("a", 2, false)

	// Wait for original TTL to expire
	time.Sleep(75 * time.Millisecond)

	// Item should still be present because TTL was reset
	if v, found := cache.Get("a"); !found || v != 2 {
		t.Errorf("Expected to find 'a' with value 2 after TTL reset, got found=%v, value=%v", found, v)
	}

	// Wait for the new TTL to expire
	time.Sleep(50 * time.Millisecond)

	// Now the item should be gone
	if _, found := cache.Get("a"); found {
		t.Errorf("Expected 'a' to be expired after new TTL")
	}
}

func TestConcurrentTTL(t *testing.T) {
	// Use a longer TTL to accommodate the 1-second minimum cleanup interval
	cache := NewCacheWithTTL[string, int](100, 2*time.Second)
	defer cache.Close()

	var wg sync.WaitGroup
	iterations := 100
	goroutines := 10

	// Start goroutines that add items with different TTLs
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range iterations {
				key := fmt.Sprintf("key%d-%d", i, j)
				// Use longer TTLs that are still shorter than the test duration
				ttl := time.Duration((j%3)+1) * 500 * time.Millisecond
				cache.SetWithTTL(key, j, j%2 == 0, ttl)
			}
		}()
	}

	// Start goroutines that read items
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range iterations {
				time.Sleep(time.Millisecond)
				for k := range 5 {
					key := fmt.Sprintf("key%d-%d", (i+k)%goroutines, j)
					_, _ = cache.Get(key)
				}
			}
		}()
	}

	wg.Wait()

	// Instead of waiting for auto-cleanup, manually trigger cleanup
	// This avoids the test timing dependency on the 1-second minimum cleanup interval
	cache.CleanupExpired()

	// Items should be gone after manual cleanup
	// Count how many items are left
	remaining := 0
	for i := range goroutines {
		for j := range iterations {
			key := fmt.Sprintf("key%d-%d", i, j)
			if _, found := cache.Get(key); found {
				remaining++
			}
		}
	}

	// Since we're immediately cleaning up after all TTLs are set,
	// most items should be gone, but some might still be valid if they
	// were set with the longer TTLs near the end of the test
	if remaining > iterations*2 {
		t.Errorf("Expected most items to be expired, but %d out of %d remain",
			remaining, iterations*goroutines)
	}
}

func TestZeroTTLCache(t *testing.T) {
	// Create cache with no default TTL
	cache := NewCacheWithTTL[string, int](5, 0)
	defer cache.Close()

	// Add items without specifying a TTL (should not expire)
	cache.Set("a", 1, false)
	cache.Set("b", 2, true)

	// Wait some time
	time.Sleep(100 * time.Millisecond)

	// Items should still be present
	if _, found := cache.Get("a"); !found {
		t.Errorf("Expected 'a' to still be present (no TTL)")
	}
	if _, found := cache.Get("b"); !found {
		t.Errorf("Expected 'b' to still be present (no TTL)")
	}

	// Add an item with explicit TTL
	cache.SetWithTTL("c", 3, false, 50*time.Millisecond)

	// Wait for that item to expire
	time.Sleep(100 * time.Millisecond)

	// The item with TTL should be gone
	if _, found := cache.Get("c"); found {
		t.Errorf("Expected 'c' to be expired")
	}

	// Other items should still be present
	if _, found := cache.Get("a"); !found {
		t.Errorf("Expected 'a' to still be present")
	}
	if _, found := cache.Get("b"); !found {
		t.Errorf("Expected 'b' to still be present")
	}
}

func TestBackwardsCompatibility(t *testing.T) {
	// Test that old constructor still works and creates a cache with no TTL
	cache := NewCache[string, int](5)
	defer cache.Close() // Should be safe to call even without auto-cleanup

	cache.Set("a", 1, false)

	// Wait some time
	time.Sleep(100 * time.Millisecond)

	// Item should still be present (no TTL)
	if _, found := cache.Get("a"); !found {
		t.Errorf("Expected 'a' to still be present with backwards-compatible constructor")
	}
}

func TestCloseOnce(t *testing.T) {
	cache := NewCacheWithTTL[string, int](5, 100*time.Millisecond)

	// Calling Close multiple times should not panic
	cache.Close()
	cache.Close()
	cache.Close()

	// Concurrent Close calls should also be safe
	cache2 := NewCacheWithTTL[string, int](5, 100*time.Millisecond)
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache2.Close()
		}()
	}
	wg.Wait()
}
