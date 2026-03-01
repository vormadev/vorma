package appsupervisor_test

import (
	"testing"
	"time"

	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/appsupervisor"
)

func TestDeriveReadinessWaitDelay(t *testing.T) {
	baseDelay := 20 * time.Millisecond
	maximumDelay := 500 * time.Millisecond
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
			Name:         "later attempt applies bounded linear backoff",
			AttemptIndex: 3,
			Expected:     80 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			derivedDelay := appsupervisor.DeriveReadinessWaitDelay(
				testCase.AttemptIndex,
				baseDelay,
				maximumDelay,
			)
			if derivedDelay != testCase.Expected {
				t.Fatalf(
					"DeriveReadinessWaitDelay(%d, %v, %v)=%v, want %v",
					testCase.AttemptIndex,
					baseDelay,
					maximumDelay,
					derivedDelay,
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
			shouldContinue := appsupervisor.ShouldContinueReadinessWait(
				testCase.Total,
				testCase.MaxTotal,
			)
			if shouldContinue != testCase.Expected {
				t.Fatalf(
					"ShouldContinueReadinessWait(%v, %v)=%t, want %t",
					testCase.Total,
					testCase.MaxTotal,
					shouldContinue,
					testCase.Expected,
				)
			}
		})
	}
}
