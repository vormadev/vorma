package typed

import (
	"strconv"
	"sync"
	"testing"
)

func TestSyncMap_BasicOperations(t *testing.T) {
	m := NewSyncMap[string, int]()

	// Test storing and loading values
	m.Store("foo", 42)
	value, ok := m.Load("foo")
	if !ok || value != 42 {
		t.Errorf("expected 42, got %d", value)
	}

	// Test overwriting a value
	m.Store("foo", 100)
	value, ok = m.Load("foo")
	if !ok || value != 100 {
		t.Errorf("expected 100, got %d", value)
	}

	// Test deleting a value
	m.Delete("foo")
	_, ok = m.Load("foo")
	if ok {
		t.Error("expected key 'foo' to be deleted")
	}
}

func TestSyncMap_LoadOrStore(t *testing.T) {
	m := NewSyncMap[string, int]()

	// Test loading or storing a new value
	actual, loaded := m.LoadOrStore("foo", 42)
	if loaded || actual != 42 {
		t.Errorf("expected 42, got %d (loaded: %v)", actual, loaded)
	}

	// Test loading an existing value
	actual, loaded = m.LoadOrStore("foo", 100)
	if !loaded || actual != 42 {
		t.Errorf("expected 42, got %d (loaded: %v)", actual, loaded)
	}
}

func TestSyncMap_LoadAndDelete(t *testing.T) {
	m := NewSyncMap[string, int]()

	// Test loading and deleting a value
	m.Store("foo", 42)
	value, loaded := m.LoadAndDelete("foo")
	if !loaded || value != 42 {
		t.Errorf("expected 42, got %d (loaded: %v)", value, loaded)
	}

	// Test that the value has been deleted
	_, ok := m.Load("foo")
	if ok {
		t.Error("expected key 'foo' to be deleted")
	}

	// Test deleting a non-existent key
	_, loaded = m.LoadAndDelete("bar")
	if loaded {
		t.Error("expected 'bar' not to be loaded")
	}
}

func TestSyncMap_Range(t *testing.T) {
	m := NewSyncMap[string, int]()

	// Store multiple values
	m.Store("foo", 42)
	m.Store("bar", 100)

	// Test iterating over the map
	found := map[string]int{}
	m.Range(func(key string, value int) bool {
		found[key] = value
		return true
	})

	if len(found) != 2 || found["foo"] != 42 || found["bar"] != 100 {
		t.Errorf("unexpected values from Range: %+v", found)
	}

	// Test early exit from Range
	counter := 0
	m.Range(func(key string, value int) bool {
		counter++
		return false
	})
	if counter != 1 {
		t.Errorf("expected Range to stop after one iteration, got %d", counter)
	}
}

func TestSyncMap_Concurrency(t *testing.T) {
	m := NewSyncMap[string, int]()
	wg := sync.WaitGroup{}

	// Test concurrent access
	for i := range 1000 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key" + strconv.Itoa(i)
			m.Store(key, i)
			value, ok := m.Load(key)
			if !ok || value != i {
				t.Errorf("expected %d, got %d", i, value)
			}
		}(i)
	}

	wg.Wait()

	// Test that all keys are present after concurrent access
	for i := range 1000 {
		key := "key" + strconv.Itoa(i)
		value, ok := m.Load(key)
		if !ok || value != i {
			t.Errorf("expected %d, got %d", i, value)
		}
	}
}
