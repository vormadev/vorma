package coalescecmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/lockfile"
)

func TestRun_RequiresKey(t *testing.T) {
	runError := Run(Options{
		Func:                   func() error { return nil },
		StateRootDirectoryPath: t.TempDir(),
	})
	if runError == nil {
		t.Fatal("expected key validation error")
	}
	if !strings.Contains(runError.Error(), "key is required") {
		t.Fatalf("expected key validation error, got %v", runError)
	}
}

func TestRun_RequiresFunc(t *testing.T) {
	runError := Run(Options{
		Key:                    "buildts",
		StateRootDirectoryPath: t.TempDir(),
	})
	if runError == nil {
		t.Fatal("expected func validation error")
	}
	if !strings.Contains(runError.Error(), "func is required") {
		t.Fatalf("expected func validation error, got %v", runError)
	}
}

func TestRun_ConcurrentCallersExecuteOwnerFunctionOnce(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()

	var executionCount int32
	ownerFunction := func() error {
		atomic.AddInt32(&executionCount, 1)
		time.Sleep(220 * time.Millisecond)
		return nil
	}

	const concurrentCallerCount = 10
	errorsByCaller := make([]error, concurrentCallerCount)

	startGate := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(concurrentCallerCount)

	for callerIndex := 0; callerIndex < concurrentCallerCount; callerIndex++ {
		go func(callerIndex int) {
			defer waitGroup.Done()
			<-startGate

			errorsByCaller[callerIndex] = Run(Options{
				Key:                      "buildts",
				Func:                     ownerFunction,
				StateRootDirectoryPath:   stateRootDirectoryPath,
				OwnerWaitPollingInterval: 10 * time.Millisecond,
			})
		}(callerIndex)
	}

	close(startGate)
	waitGroup.Wait()

	for callerIndex, runError := range errorsByCaller {
		if runError != nil {
			t.Fatalf("caller %d returned unexpected error: %v", callerIndex, runError)
		}
	}

	if executionCount != 1 {
		t.Fatalf("expected exactly one owner execution, got %d", executionCount)
	}
}

func TestRun_ConcurrentCallersShareOwnerFailure(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()

	var executionCount int32
	ownerFunction := func() error {
		atomic.AddInt32(&executionCount, 1)
		time.Sleep(200 * time.Millisecond)
		return errors.New("synthetic-owner-failure")
	}

	const concurrentCallerCount = 8
	errorsByCaller := make([]error, concurrentCallerCount)

	startGate := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(concurrentCallerCount)

	for callerIndex := 0; callerIndex < concurrentCallerCount; callerIndex++ {
		go func(callerIndex int) {
			defer waitGroup.Done()
			<-startGate

			errorsByCaller[callerIndex] = Run(Options{
				Key:                      "buildts",
				Func:                     ownerFunction,
				StateRootDirectoryPath:   stateRootDirectoryPath,
				OwnerWaitPollingInterval: 10 * time.Millisecond,
			})
		}(callerIndex)
	}

	close(startGate)
	waitGroup.Wait()

	for callerIndex, runError := range errorsByCaller {
		if runError == nil {
			t.Fatalf("caller %d expected owner failure but got nil", callerIndex)
		}
		if !strings.Contains(runError.Error(), "synthetic-owner-failure") {
			t.Fatalf("caller %d expected owner failure message, got %v", callerIndex, runError)
		}
	}

	if executionCount != 1 {
		t.Fatalf("expected exactly one owner execution, got %d", executionCount)
	}
}

func TestRun_RunsAgainAfterPreviousCompletion(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()

	var executionCount int32
	ownerFunction := func() error {
		atomic.AddInt32(&executionCount, 1)
		return nil
	}

	firstRunError := Run(Options{
		Key:                      "buildts",
		Func:                     ownerFunction,
		StateRootDirectoryPath:   stateRootDirectoryPath,
		OwnerWaitPollingInterval: 10 * time.Millisecond,
	})
	if firstRunError != nil {
		t.Fatalf("first run returned error: %v", firstRunError)
	}

	secondRunError := Run(Options{
		Key:                      "buildts",
		Func:                     ownerFunction,
		StateRootDirectoryPath:   stateRootDirectoryPath,
		OwnerWaitPollingInterval: 10 * time.Millisecond,
	})
	if secondRunError != nil {
		t.Fatalf("second run returned error: %v", secondRunError)
	}

	if executionCount != 2 {
		t.Fatalf("expected two owner executions across sequential calls, got %d", executionCount)
	}
}

func TestRun_FailIfRunningReturnsErrAlreadyRunning(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()

	ownerStarted := make(chan struct{})
	ownerRelease := make(chan struct{})
	ownerDone := make(chan error, 1)

	ownerFunctionCalled := int32(0)
	go func() {
		ownerDone <- Run(Options{
			Key:           "release",
			FailIfRunning: []string{"release"},
			Func: func() error {
				atomic.AddInt32(&ownerFunctionCalled, 1)
				close(ownerStarted)
				<-ownerRelease
				return nil
			},
			StateRootDirectoryPath:   stateRootDirectoryPath,
			OwnerWaitPollingInterval: 10 * time.Millisecond,
		})
	}()

	<-ownerStarted

	secondFunctionCalled := int32(0)
	secondRunError := Run(Options{
		Key:           "release",
		FailIfRunning: []string{"release"},
		Func: func() error {
			atomic.AddInt32(&secondFunctionCalled, 1)
			return nil
		},
		StateRootDirectoryPath:   stateRootDirectoryPath,
		OwnerWaitPollingInterval: 10 * time.Millisecond,
	})
	if secondRunError == nil {
		t.Fatal("expected fail-if-running call to return an error")
	}
	if !errors.Is(secondRunError, ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", secondRunError)
	}
	if secondFunctionCalled != 0 {
		t.Fatalf("expected second owner function to never execute, got count=%d", secondFunctionCalled)
	}

	close(ownerRelease)
	ownerRunError := <-ownerDone
	if ownerRunError != nil {
		t.Fatalf("owner run returned unexpected error: %v", ownerRunError)
	}
	if ownerFunctionCalled != 1 {
		t.Fatalf("expected owner function to run once, got count=%d", ownerFunctionCalled)
	}
}

func TestRun_FailIfRunningStillRunsWhenNoOwnerIsActive(t *testing.T) {
	executionCount := int32(0)

	runError := Run(Options{
		Key:           "release",
		FailIfRunning: []string{"release"},
		Func: func() error {
			atomic.AddInt32(&executionCount, 1)
			return nil
		},
		StateRootDirectoryPath:   t.TempDir(),
		OwnerWaitPollingInterval: 10 * time.Millisecond,
	})
	if runError != nil {
		t.Fatalf("expected run to succeed when no owner is active, got %v", runError)
	}
	if executionCount != 1 {
		t.Fatalf("expected one execution, got %d", executionCount)
	}
}

func TestRun_FailIfRunningReturnsErrAlreadyRunningForOtherActiveKey(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()

	ownerStarted := make(chan struct{})
	ownerRelease := make(chan struct{})
	ownerDone := make(chan error, 1)

	go func() {
		ownerDone <- Run(Options{
			Key: "tsinstall",
			Func: func() error {
				close(ownerStarted)
				<-ownerRelease
				return nil
			},
			StateRootDirectoryPath:   stateRootDirectoryPath,
			OwnerWaitPollingInterval: 10 * time.Millisecond,
		})
	}()

	<-ownerStarted

	secondFunctionCalled := int32(0)
	secondRunError := Run(Options{
		Key:           "nuke-node-modules",
		FailIfRunning: []string{"tsinstall"},
		Func: func() error {
			atomic.AddInt32(&secondFunctionCalled, 1)
			return nil
		},
		StateRootDirectoryPath:   stateRootDirectoryPath,
		OwnerWaitPollingInterval: 10 * time.Millisecond,
	})
	if secondRunError == nil {
		t.Fatal("expected fail-if-running call to return an error")
	}
	if !errors.Is(secondRunError, ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", secondRunError)
	}
	if secondFunctionCalled != 0 {
		t.Fatalf("expected second owner function to never execute, got count=%d", secondFunctionCalled)
	}
	if !strings.Contains(secondRunError.Error(), `key "tsinstall"`) {
		t.Fatalf("expected error to include active conflict key, got %v", secondRunError)
	}

	close(ownerRelease)
	ownerRunError := <-ownerDone
	if ownerRunError != nil {
		t.Fatalf("owner run returned unexpected error: %v", ownerRunError)
	}
}

func TestMustRun_DoesNotPanicOnSuccess(t *testing.T) {
	didPanic := false
	func() {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()

		MustRun(Options{
			Key: "buildts",
			Func: func() error {
				return nil
			},
			StateRootDirectoryPath:   t.TempDir(),
			OwnerWaitPollingInterval: 10 * time.Millisecond,
		})
	}()

	if didPanic {
		t.Fatal("expected MustRun success path to avoid panic")
	}
}

func TestMustRun_PanicsOnError(t *testing.T) {
	panicValue := any(nil)
	func() {
		defer func() {
			panicValue = recover()
		}()

		MustRun(Options{
			Key: "buildts",
			Func: func() error {
				return errors.New("synthetic-owner-failure")
			},
			StateRootDirectoryPath:   t.TempDir(),
			OwnerWaitPollingInterval: 10 * time.Millisecond,
		})
	}()

	if panicValue == nil {
		t.Fatal("expected MustRun to panic when Run returns an error")
	}

	panicString := panicValue.(string)
	if !strings.Contains(panicString, "coalescecmd.MustRun:") {
		t.Fatalf("expected MustRun panic prefix, got %q", panicString)
	}
	if !strings.Contains(panicString, "synthetic-owner-failure") {
		t.Fatalf("expected MustRun panic to include owner failure, got %q", panicString)
	}
}

func TestRun_PanicsWhenStateRootDirectoryPathIsMissing(t *testing.T) {
	panicValue := any(nil)
	func() {
		defer func() {
			panicValue = recover()
		}()

		_ = Run(Options{
			Key: "buildts",
			Func: func() error {
				return nil
			},
		})
	}()

	if panicValue == nil {
		t.Fatal("expected Run to panic when StateRootDirectoryPath is missing")
	}
	panicMessage := fmt.Sprint(panicValue)
	if !strings.Contains(panicMessage, "StateRootDirectoryPath is required") {
		t.Fatalf("expected panic to mention StateRootDirectoryPath, got %q", panicMessage)
	}
}

func TestRun_CleansUpStaleDoneStateForOtherKeys(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()

	staleKeyStatePath, staleKeyLockPath := buildRunFilePaths(
		stateRootDirectoryPath,
		"stale-key",
	)
	if makeDirectoryError := os.MkdirAll(
		filepath.Dir(staleKeyStatePath),
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf("create state directory: %v", makeDirectoryError)
	}
	staleDoneState := runStateFile{
		SchemaVersion: runStateSchemaVersion,
		Key:           "stale-key",
		RunID:         "stale-run",
		OwnerPID:      1234,
		Status:        runStateStatusDone,
		StartedAtUTC:  time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano),
		FinishedAtUTC: time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano),
	}
	if writeStateError := writeRunStateFile(
		staleKeyStatePath,
		staleDoneState,
	); writeStateError != nil {
		t.Fatalf("write stale run state: %v", writeStateError)
	}

	runError := Run(Options{
		Key:                    "fresh-key",
		StateRootDirectoryPath: stateRootDirectoryPath,
		Func: func() error {
			return nil
		},
		OwnerWaitPollingInterval: 10 * time.Millisecond,
	})
	if runError != nil {
		t.Fatalf("run returned error: %v", runError)
	}

	if _, statStateError := os.Stat(staleKeyStatePath); !os.IsNotExist(statStateError) {
		t.Fatalf("expected stale state file cleanup, stat error=%v", statStateError)
	}
	if _, statLockError := os.Stat(staleKeyLockPath); !os.IsNotExist(statLockError) {
		t.Fatalf("expected stale lock file cleanup, stat error=%v", statLockError)
	}
}

func TestRun_CleansUpInvalidStateFileForOtherKeys(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()

	invalidKeyStatePath, invalidKeyLockPath := buildRunFilePaths(
		stateRootDirectoryPath,
		"invalid-key",
	)
	if makeDirectoryError := os.MkdirAll(
		filepath.Dir(invalidKeyStatePath),
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf("create state directory: %v", makeDirectoryError)
	}
	if writeInvalidStateError := os.WriteFile(
		invalidKeyStatePath,
		[]byte("{ this-is-not-valid-json }\n"),
		0o644,
	); writeInvalidStateError != nil {
		t.Fatalf("write invalid run state: %v", writeInvalidStateError)
	}

	ownerRuns := 0
	runError := Run(Options{
		Key:                    "fresh-key",
		StateRootDirectoryPath: stateRootDirectoryPath,
		Func: func() error {
			ownerRuns += 1
			return nil
		},
		OwnerWaitPollingInterval: 10 * time.Millisecond,
	})
	if runError != nil {
		t.Fatalf("run returned error: %v", runError)
	}
	if ownerRuns != 1 {
		t.Fatalf("expected owner runs=1, got %d", ownerRuns)
	}

	if _, statStateError := os.Stat(invalidKeyStatePath); !os.IsNotExist(statStateError) {
		t.Fatalf("expected invalid state file cleanup, stat error=%v", statStateError)
	}
	if _, statLockError := os.Stat(invalidKeyLockPath); !os.IsNotExist(statLockError) {
		t.Fatalf("expected invalid lock file cleanup, stat error=%v", statLockError)
	}
}

func TestWaitForOwnerCompletion_JoinsWhenDoneStateChangesWithoutObservedRunningState(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()
	runStatePath, runLockPath := buildRunFilePaths(stateRootDirectoryPath, "buildts")

	ownerLock := lockfile.NewPIDLock(runLockPath)
	if acquireOwnerLockError := ownerLock.Acquire(); acquireOwnerLockError != nil {
		t.Fatalf("acquire owner lock: %v", acquireOwnerLockError)
	}

	initialDoneState := runStateFile{
		SchemaVersion: runStateSchemaVersion,
		Key:           "buildts",
		RunID:         "stale-run-id",
		OwnerPID:      1234,
		Status:        runStateStatusDone,
		StartedAtUTC:  time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano),
		FinishedAtUTC: time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano),
	}
	if writeInitialStateError := writeRunStateFile(
		runStatePath,
		initialDoneState,
	); writeInitialStateError != nil {
		t.Fatalf("write initial done state: %v", writeInitialStateError)
	}

	ownerTransitionErrorChannel := make(chan error, 1)
	go func() {
		time.Sleep(40 * time.Millisecond)

		currentDoneState := initialDoneState
		currentDoneState.RunID = "current-owner-run-id"
		currentDoneState.StartedAtUTC = time.Now().UTC().Add(-200 * time.Millisecond).Format(time.RFC3339Nano)
		currentDoneState.FinishedAtUTC = time.Now().UTC().Format(time.RFC3339Nano)
		currentDoneState.OwnerErrorMessage = "synthetic-owner-failure"

		if writeCurrentStateError := writeRunStateFile(
			runStatePath,
			currentDoneState,
		); writeCurrentStateError != nil {
			ownerTransitionErrorChannel <- writeCurrentStateError
			return
		}

		ownerTransitionErrorChannel <- ownerLock.Release()
	}()

	waitResult, waitError := waitForOwnerCompletion(waitForOwnerCompletionInput{
		runStatePath:             runStatePath,
		runLock:                  lockfile.NewPIDLock(runLockPath),
		ownerWaitPollingInterval: 5 * time.Millisecond,
	})
	if waitError != nil {
		t.Fatalf("wait for owner completion: %v", waitError)
	}

	if ownerTransitionError := <-ownerTransitionErrorChannel; ownerTransitionError != nil {
		t.Fatalf("owner transition: %v", ownerTransitionError)
	}

	if !waitResult.joined {
		t.Fatal("expected joiner to treat changed done state as owner completion")
	}
	if waitResult.ownerError == nil {
		t.Fatal("expected joiner to observe owner error")
	}
	if !strings.Contains(waitResult.ownerError.Error(), "synthetic-owner-failure") {
		t.Fatalf("expected owner error message, got %v", waitResult.ownerError)
	}
}

func TestWaitForOwnerCompletion_DoesNotJoinUnchangedDoneStateWithoutObservedRunningState(t *testing.T) {
	stateRootDirectoryPath := t.TempDir()
	runStatePath, runLockPath := buildRunFilePaths(stateRootDirectoryPath, "buildts")

	ownerLock := lockfile.NewPIDLock(runLockPath)
	if acquireOwnerLockError := ownerLock.Acquire(); acquireOwnerLockError != nil {
		t.Fatalf("acquire owner lock: %v", acquireOwnerLockError)
	}

	unchangedDoneState := runStateFile{
		SchemaVersion: runStateSchemaVersion,
		Key:           "buildts",
		RunID:         "stale-run-id",
		OwnerPID:      1234,
		Status:        runStateStatusDone,
		StartedAtUTC:  time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano),
		FinishedAtUTC: time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano),
	}
	if writeStateError := writeRunStateFile(
		runStatePath,
		unchangedDoneState,
	); writeStateError != nil {
		t.Fatalf("write unchanged done state: %v", writeStateError)
	}

	ownerReleaseErrorChannel := make(chan error, 1)
	go func() {
		time.Sleep(40 * time.Millisecond)
		ownerReleaseErrorChannel <- ownerLock.Release()
	}()

	waitResult, waitError := waitForOwnerCompletion(waitForOwnerCompletionInput{
		runStatePath:             runStatePath,
		runLock:                  lockfile.NewPIDLock(runLockPath),
		ownerWaitPollingInterval: 5 * time.Millisecond,
	})
	if waitError != nil {
		t.Fatalf("wait for owner completion: %v", waitError)
	}

	if ownerReleaseError := <-ownerReleaseErrorChannel; ownerReleaseError != nil {
		t.Fatalf("release owner lock: %v", ownerReleaseError)
	}

	if waitResult.joined {
		t.Fatal("expected unchanged done state to remain non-joinable")
	}
	if waitResult.ownerError != nil {
		t.Fatalf("expected nil owner error when not joined, got %v", waitResult.ownerError)
	}
}

func TestBuildRunFileBaseNameFromCoalescingKey_UsesLowercaseBase32WithoutPadding(t *testing.T) {
	firstHash := buildRunFileBaseNameFromCoalescingKey("internal-cmd-buildts")
	secondHash := buildRunFileBaseNameFromCoalescingKey("internal-cmd-buildts")
	differentHash := buildRunFileBaseNameFromCoalescingKey("internal-cmd-release")

	if firstHash != secondHash {
		t.Fatalf("expected deterministic hash output, got %q and %q", firstHash, secondHash)
	}
	if len(firstHash) != 13 {
		t.Fatalf("expected 13-character hash from 8-byte digest, got %q", firstHash)
	}
	if strings.Contains(firstHash, "=") {
		t.Fatalf("expected unpadded base32 output, got %q", firstHash)
	}
	if strings.ToLower(firstHash) != firstHash {
		t.Fatalf("expected lowercase base32 output, got %q", firstHash)
	}
	for _, hashCharacter := range firstHash {
		if (hashCharacter >= 'a' && hashCharacter <= 'z') ||
			(hashCharacter >= '2' && hashCharacter <= '7') {
			continue
		}
		t.Fatalf("expected base32 character set [a-z2-7], got %q", firstHash)
	}
	if firstHash == differentHash {
		t.Fatalf("expected different keys to produce different hash outputs, got %q", firstHash)
	}
}
