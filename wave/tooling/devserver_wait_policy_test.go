package tooling

import (
	"testing"
	"time"

	"github.com/vormadev/vorma/wave/tooling/devserver"
)

func TestDeriveReadinessWaitDelay(t *testing.T) {
	baseDelay := 20 * time.Millisecond
	testCases := []struct {
		Name         string
		AttemptIndex int
		Expected     time.Duration
	}{
		{
			Name:         "first attempt uses base delay",
			AttemptIndex: 0,
			Expected:     20 * time.Millisecond,
		},
		{
			Name:         "later attempt increases linearly",
			AttemptIndex: 3,
			Expected:     80 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			if got := devserver.DeriveReadinessWaitDelay(testCase.AttemptIndex, baseDelay); got != testCase.Expected {
				t.Fatalf(
					"deriveReadinessWaitDelay(%d, %v)=%v, want %v",
					testCase.AttemptIndex,
					baseDelay,
					got,
					testCase.Expected,
				)
			}
		})
	}
}

func TestShouldContinueReadinessWait(t *testing.T) {
	testCases := []struct {
		Name     string
		Total    time.Duration
		MaxTotal time.Duration
		Expected bool
	}{
		{
			Name:     "under max total continues",
			Total:    1 * time.Second,
			MaxTotal: 2 * time.Second,
			Expected: true,
		},
		{
			Name:     "at max total continues",
			Total:    2 * time.Second,
			MaxTotal: 2 * time.Second,
			Expected: true,
		},
		{
			Name:     "over max total stops",
			Total:    3 * time.Second,
			MaxTotal: 2 * time.Second,
			Expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			if got := devserver.ShouldContinueReadinessWait(testCase.Total, testCase.MaxTotal); got != testCase.Expected {
				t.Fatalf(
					"shouldContinueReadinessWait(%v, %v)=%t, want %t",
					testCase.Total,
					testCase.MaxTotal,
					got,
					testCase.Expected,
				)
			}
		})
	}
}
