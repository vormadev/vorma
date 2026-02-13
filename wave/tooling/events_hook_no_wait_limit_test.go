package tooling

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRunNoWaitHookWithConcurrencyLimit_EnforcesExecutionCap(t *testing.T) {
	s := &server{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 2),
	}

	started := make(chan struct{}, 3)
	release := make(chan struct{})
	schedulingDone := make(chan struct{})

	for range 2 {
		s.runNoWaitHookWithConcurrencyLimit(func() {
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

	go func() {
		s.runNoWaitHookWithConcurrencyLimit(func() {
			started <- struct{}{}
			<-release
		})
		close(schedulingDone)
	}()

	select {
	case <-schedulingDone:
		t.Fatal("expected third no-wait hook scheduling to block while execution cap is full")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case <-schedulingDone:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for blocked no-wait hook scheduling to continue")
	}

	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for third no-wait hook to start after capacity was released")
	}
}

func TestRunNoWaitHookWithConcurrencyLimit_RunsHookAsynchronously(t *testing.T) {
	s := &server{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 1),
	}

	var ran atomic.Bool
	finished := make(chan struct{})

	s.runNoWaitHookWithConcurrencyLimit(func() {
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
	firstServer := &server{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 1),
	}
	secondServer := &server{
		concurrentNoWaitHookExecutionLimiter: make(chan struct{}, 1),
	}

	firstServerStarted := make(chan struct{}, 1)
	firstServerRelease := make(chan struct{})
	firstServer.runNoWaitHookWithConcurrencyLimit(func() {
		firstServerStarted <- struct{}{}
		<-firstServerRelease
	})

	select {
	case <-firstServerStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for first server hook to start")
	}

	secondServerHookFinished := make(chan struct{}, 1)
	secondServer.runNoWaitHookWithConcurrencyLimit(func() {
		secondServerHookFinished <- struct{}{}
	})

	select {
	case <-secondServerHookFinished:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for second server hook while first server limiter is full")
	}

	close(firstServerRelease)
}
