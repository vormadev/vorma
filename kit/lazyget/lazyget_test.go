package lazyget

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestValue_Get(t *testing.T) {
	t.Run("Basic functionality", func(t *testing.T) {
		callCount := 0
		var v Cache[int]
		initFunc := func() int {
			callCount++
			return 42
		}
		for range 5 {
			if got := v.Get(initFunc); got != 42 {
				t.Errorf("Value.Get() = %v, want 42", got)
			}
		}
		if callCount != 1 {
			t.Errorf("initFunc called %d times, want 1", callCount)
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		const goroutines = 100
		callCount := 0
		var v Cache[int]
		initFunc := func() int {
			time.Sleep(10 * time.Millisecond)
			callCount++
			return 42
		}

		var wg sync.WaitGroup
		wg.Add(goroutines)
		for range goroutines {
			go func() {
				defer wg.Done()
				if got := v.Get(initFunc); got != 42 {
					t.Errorf("Value.Get() = %v, want 42", got)
				}
			}()
		}
		wg.Wait()

		if callCount != 1 {
			t.Errorf("initFunc called %d times, want 1", callCount)
		}
	})

	t.Run("Nil initFunc", func(t *testing.T) {
		var v Cache[*int]
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic")
			}
		}()
		v.Get(nil)
	})

	t.Run("Different types", func(t *testing.T) {
		testCases := []struct {
			name string
			c    func() any
			want any
		}{
			{"string", New(func() any { return "hello" }), "hello"},
			{"int", New(func() any { return 42 }), 42},
			{"slice", New(func() any { return []int{1, 2, 3} }), []int{1, 2, 3}},
			{"struct", New(func() any { return struct{ X int }{X: 10} }), struct{ X int }{X: 10}},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := tc.c()
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("Value.Get() = %v, want %v", got, tc.want)
				}
			})
		}
	})
}

func TestNew(t *testing.T) {
	t.Run("Creates new getter", func(t *testing.T) {
		getter := New(func() int { return 42 })
		if getter == nil {
			t.Fatal("New() returned nil")
		}
		if got := getter(); got != 42 {
			t.Errorf("getter() = %v, want 42", got)
		}
	})

	t.Run("Nil initFunc", func(t *testing.T) {
		getter := New[int](nil)
		if getter == nil {
			t.Fatal("New() returned nil")
		}
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic")
			}
		}()
		getter()
	})
}

func TestValue_RaceCondition(t *testing.T) {
	var v Cache[int]
	initFunc := func() int {
		time.Sleep(10 * time.Millisecond)
		return 42
	}

	done := make(chan bool)
	go func() {
		v.Get(initFunc)
		done <- true
	}()
	go func() {
		v.Get(initFunc)
		done <- true
	}()

	<-done
	<-done

	if got := v.Get(initFunc); got != 42 {
		t.Errorf("Value.Get() = %v, want 42", got)
	}
}
