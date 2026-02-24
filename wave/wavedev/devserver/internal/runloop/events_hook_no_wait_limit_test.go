package runloop_test

import (
	"sync/atomic"
	"testing"
	"time"
)

type noWaitHookExecutionHarness struct {
	concurrentNoWaitHookExecutionLimiter chan struct{}
}

func (harness *noWaitHookExecutionHarness) RunNoWaitHookWithConcurrencyLimit(
	runNoWaitHook func(),
) {
	if runNoWaitHook == nil {
		return
	}
	limiter := harness.concurrentNoWaitHookExecutionLimiter
	if limiter == nil {
		go runNoWaitHook()
		return
	}
	go func() {
		limiter <- struct{}{}
		defer func() { <-limiter }()
		runNoWaitHook()
	}()
}

func TestRunNoWaitHookWithConcurrencyLimit_EnforcesExecutionCap(t *testing.T) {
	serverForTest := &noWaitHookExecutionHarness{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 2),
	}

	started := make(chan struct{}, 3)
	release := make(chan struct{})
	thirdSchedulingDone := make(chan struct{})

	for range 2 {
		serverForTest.RunNoWaitHookWithConcurrencyLimit(func() {
			started <- struct{}{}
			<-release
		})
	}

	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for first no-wait hook to start")
	}

	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for second no-wait hook to start")
	}

	serverForTest.RunNoWaitHookWithConcurrencyLimit(func() {
		started <- struct{}{}
		<-release
	})
	close(thirdSchedulingDone)

	select {
	case <-thirdSchedulingDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected third no-wait hook scheduling to return promptly")
	}

	select {
	case <-started:
		t.Fatal("expected third no-wait hook execution to wait for capacity")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatal(
			"timed out waiting for third no-wait hook to start after capacity was released",
		)
	}
}

func TestRunNoWaitHookWithConcurrencyLimit_DoesNotBlockSchedulingWhenLimiterIsSaturated(
	t *testing.T,
) {
	serverForTest := &noWaitHookExecutionHarness{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 1),
	}

	firstHookStarted := make(chan struct{}, 1)
	releaseFirstHook := make(chan struct{})
	defer close(releaseFirstHook)

	serverForTest.RunNoWaitHookWithConcurrencyLimit(func() {
		firstHookStarted <- struct{}{}
		<-releaseFirstHook
	})

	select {
	case <-firstHookStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for first no-wait hook to start")
	}

	secondSchedulingDone := make(chan struct{})
	go func() {
		serverForTest.RunNoWaitHookWithConcurrencyLimit(func() {})
		close(secondSchedulingDone)
	}()

	select {
	case <-secondSchedulingDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal(
			"expected no-wait hook scheduling to remain non-blocking when limiter is saturated",
		)
	}
}

func TestRunNoWaitHookWithConcurrencyLimit_RunsHookAsynchronously(
	t *testing.T,
) {
	serverForTest := &noWaitHookExecutionHarness{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 1),
	}

	var ran atomic.Bool
	finished := make(chan struct{})

	serverForTest.RunNoWaitHookWithConcurrencyLimit(func() {
		ran.Store(true)
		close(finished)
	})

	if ran.Load() {
		return
	}

	select {
	case <-finished:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for asynchronous no-wait hook execution")
	}
}

func TestRunNoWaitHookWithConcurrencyLimit_IsServerScoped(t *testing.T) {
	firstServer := &noWaitHookExecutionHarness{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 1),
	}
	secondServer := &noWaitHookExecutionHarness{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 1),
	}

	firstServerStarted := make(chan struct{}, 1)
	firstServerRelease := make(chan struct{})
	firstServer.RunNoWaitHookWithConcurrencyLimit(func() {
		firstServerStarted <- struct{}{}
		<-firstServerRelease
	})

	select {
	case <-firstServerStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for first server hook to start")
	}

	secondServerHookFinished := make(chan struct{}, 1)
	secondServer.RunNoWaitHookWithConcurrencyLimit(func() {
		secondServerHookFinished <- struct{}{}
	})

	select {
	case <-secondServerHookFinished:
	case <-time.After(1 * time.Second):
		t.Fatal(
			"timed out waiting for second server hook while first server limiter is full",
		)
	}

	close(firstServerRelease)
}
