package wave

import (
	"fmt"
	"testing"
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
