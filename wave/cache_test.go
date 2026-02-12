package wave

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheCachesInProductionMode(t *testing.T) {
	setWaveDevModeForTest(t, false)
	calls := 0
	c := newCache(func() (int, error) {
		calls++
		return calls, nil
	})

	first, err := c.get()
	if err != nil {
		t.Fatalf("unexpected error on first cache get: %v", err)
	}
	second, err := c.get()
	if err != nil {
		t.Fatalf("unexpected error on second cache get: %v", err)
	}

	if first != 1 || second != 1 {
		t.Fatalf("expected cached value to remain stable in production, got first=%d second=%d", first, second)
	}
	if calls != 1 {
		t.Fatalf("expected init function to run once in production, ran %d times", calls)
	}
}

func TestCacheRecomputesInDevMode(t *testing.T) {
	setWaveDevModeForTest(t, true)
	calls := 0
	c := newCache(func() (int, error) {
		calls++
		return calls, nil
	})

	first, err := c.get()
	if err != nil {
		t.Fatalf("unexpected error on first cache get: %v", err)
	}
	second, err := c.get()
	if err != nil {
		t.Fatalf("unexpected error on second cache get: %v", err)
	}

	if first != 1 || second != 2 {
		t.Fatalf("expected cache to recompute in dev mode, got first=%d second=%d", first, second)
	}
	if calls != 2 {
		t.Fatalf("expected init function to run for each dev access, ran %d times", calls)
	}
}

func TestCacheMapCachesSuccessfulResultsInProductionMode(t *testing.T) {
	setWaveDevModeForTest(t, false)
	calls := 0
	cm := newCacheMap(func(key string) (string, error) {
		calls++
		if key == "fail" {
			return "", fmt.Errorf("fail")
		}
		return key + "-value", nil
	})

	first, err := cm.get("ok")
	if err != nil {
		t.Fatalf("unexpected error for successful key: %v", err)
	}
	second, err := cm.get("ok")
	if err != nil {
		t.Fatalf("unexpected error for cached key: %v", err)
	}

	if first != "ok-value" || second != "ok-value" {
		t.Fatalf("unexpected cached values: first=%q second=%q", first, second)
	}
	if calls != 1 {
		t.Fatalf("expected success result to be cached, calls=%d", calls)
	}

	_, err = cm.get("fail")
	if err == nil {
		t.Fatal("expected failure key to return error")
	}
	_, err = cm.get("fail")
	if err == nil {
		t.Fatal("expected repeated failure key to return error")
	}
	if calls != 3 {
		t.Fatalf("expected failures to be recomputed (not cached), calls=%d", calls)
	}
}

func TestCacheMapRecomputesInDevMode(t *testing.T) {
	setWaveDevModeForTest(t, true)
	calls := 0
	cm := newCacheMap(func(key string) (int, error) {
		calls++
		return calls, nil
	})

	first, err := cm.get("any")
	if err != nil {
		t.Fatalf("unexpected error on first map get: %v", err)
	}
	second, err := cm.get("any")
	if err != nil {
		t.Fatalf("unexpected error on second map get: %v", err)
	}

	if first != 1 || second != 2 {
		t.Fatalf("expected recomputation in dev mode, got first=%d second=%d", first, second)
	}
	if calls != 2 {
		t.Fatalf("expected two invocations in dev mode, calls=%d", calls)
	}
}

func TestCacheCachesErrorsInProductionMode(t *testing.T) {
	setWaveDevModeForTest(t, false)
	calls := 0
	c := newCache(func() (string, error) {
		calls++
		return "", fmt.Errorf("boom")
	})

	_, firstErr := c.get()
	if firstErr == nil {
		t.Fatal("expected first cache call to return error")
	}
	_, secondErr := c.get()
	if secondErr == nil {
		t.Fatal("expected second cache call to return cached error")
	}
	if firstErr.Error() != secondErr.Error() {
		t.Fatalf("expected cached error to remain stable, got %q and %q", firstErr, secondErr)
	}
	if calls != 1 {
		t.Fatalf("expected error to be cached in production mode, calls=%d", calls)
	}
}

func TestCacheRecomputesErrorsInDevMode(t *testing.T) {
	setWaveDevModeForTest(t, true)
	calls := 0
	c := newCache(func() (string, error) {
		calls++
		return "", fmt.Errorf("boom-%d", calls)
	})

	_, firstErr := c.get()
	if firstErr == nil {
		t.Fatal("expected first cache call to return error")
	}
	_, secondErr := c.get()
	if secondErr == nil {
		t.Fatal("expected second cache call to return error")
	}
	if firstErr.Error() == secondErr.Error() {
		t.Fatalf("expected dev-mode cache to recompute error, got identical error %q", firstErr)
	}
	if calls != 2 {
		t.Fatalf("expected dev-mode cache to execute twice, calls=%d", calls)
	}
}

func TestCacheMapRunsInitializerOncePerKeyInProductionUnderConcurrency(t *testing.T) {
	setWaveDevModeForTest(t, false)

	var calls atomic.Int32
	cm := newCacheMap(func(key string) (string, error) {
		time.Sleep(10 * time.Millisecond)
		callCount := calls.Add(1)
		return fmt.Sprintf("%s-%d", key, callCount), nil
	})

	const goroutines = 20
	results := make([]string, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			val, err := cm.get("shared")
			if err != nil {
				t.Errorf("cm.get returned error: %v", err)
				return
			}
			results[i] = val
		}(i)
	}

	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected single initializer call for shared key, got %d", got)
	}
	for i, val := range results {
		if val != "shared-1" {
			t.Fatalf("unexpected result at index %d: %q", i, val)
		}
	}
}

func TestCacheMapErrorDoesNotPoisonFutureSuccessInProduction(t *testing.T) {
	setWaveDevModeForTest(t, false)
	callCount := 0
	cm := newCacheMap(func(key string) (string, error) {
		callCount++
		if callCount == 1 {
			return "", fmt.Errorf("transient failure")
		}
		return fmt.Sprintf("%s-success-%d", key, callCount), nil
	})

	_, firstErr := cm.get("recovering-key")
	if firstErr == nil {
		t.Fatal("expected first cache map call to fail")
	}

	secondValue, secondErr := cm.get("recovering-key")
	if secondErr != nil {
		t.Fatalf("expected second cache map call to succeed, got error: %v", secondErr)
	}
	if secondValue != "recovering-key-success-2" {
		t.Fatalf("unexpected second cache map value: %q", secondValue)
	}

	thirdValue, thirdErr := cm.get("recovering-key")
	if thirdErr != nil {
		t.Fatalf("expected third cache map call to use cached success, got error: %v", thirdErr)
	}
	if thirdValue != secondValue {
		t.Fatalf("expected cached success to remain stable, got second=%q third=%q", secondValue, thirdValue)
	}
	if callCount != 2 {
		t.Fatalf("expected initializer to run twice (failure + recovery), got %d calls", callCount)
	}
}
