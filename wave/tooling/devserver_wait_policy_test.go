package tooling

import (
	"testing"
	"time"
)

func TestDeriveReadinessWaitDelay(t *testing.T) {
	baseDelay := 20 * time.Millisecond
	testCases := []struct {
		name         string
		attemptIndex int
		expected     time.Duration
	}{
		{
			name:         "first attempt uses base delay",
			attemptIndex: 0,
			expected:     20 * time.Millisecond,
		},
		{
			name:         "later attempt increases linearly",
			attemptIndex: 3,
			expected:     80 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := deriveReadinessWaitDelay(testCase.attemptIndex, baseDelay); got != testCase.expected {
				t.Fatalf(
					"deriveReadinessWaitDelay(%d, %v)=%v, want %v",
					testCase.attemptIndex,
					baseDelay,
					got,
					testCase.expected,
				)
			}
		})
	}
}

func TestShouldContinueReadinessWait(t *testing.T) {
	testCases := []struct {
		name     string
		total    time.Duration
		maxTotal time.Duration
		expected bool
	}{
		{
			name:     "under max total continues",
			total:    1 * time.Second,
			maxTotal: 2 * time.Second,
			expected: true,
		},
		{
			name:     "at max total continues",
			total:    2 * time.Second,
			maxTotal: 2 * time.Second,
			expected: true,
		},
		{
			name:     "over max total stops",
			total:    3 * time.Second,
			maxTotal: 2 * time.Second,
			expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := shouldContinueReadinessWait(testCase.total, testCase.maxTotal); got != testCase.expected {
				t.Fatalf(
					"shouldContinueReadinessWait(%v, %v)=%t, want %t",
					testCase.total,
					testCase.maxTotal,
					got,
					testCase.expected,
				)
			}
		})
	}
}
