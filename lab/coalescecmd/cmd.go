// Package coalescecmd provides file-backed function coalescing so concurrent
// callers can share one owner execution result.
package coalescecmd

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/lockfile"
)

const (
	runStateSchemaVersion = 1
	runStateStatusRunning = "running"
	runStateStatusDone    = "done"

	runStateFileSuffix = ".state.json"
	runLockFileSuffix  = ".pid.lock"

	// DefaultPollInterval is the default interval while waiting on an active owner.
	DefaultPollInterval = 50 * time.Millisecond
	// staleCompletedRunMinimumAgeBeforeCleanup gates safe artifact cleanup.
	staleCompletedRunMinimumAgeBeforeCleanup = 1 * time.Second
)

var errInvalidRunStateFile = errors.New("invalid run state file")

// ErrAlreadyRunning reports that another owner is currently holding the key
// and FailIfRunning requested immediate failure instead of waiting.
var ErrAlreadyRunning = errors.New("coalesced run is already in progress for key")

// Options configures one coalesced function run.
type Options struct {
	Key string
	// Func executes only on the owner process. Joiners receive its final error.
	Func                     func() error
	FailIfRunning            []string
	StateRootDirectoryPath   string
	OwnerWaitPollingInterval time.Duration
}

// runStateFile stores owner progress and completion outcome.
type runStateFile struct {
	SchemaVersion     int    `json:"schemaVersion"`
	Key               string `json:"key"`
	RunID             string `json:"runID"`
	OwnerPID          int    `json:"ownerPID"`
	Status            string `json:"status"`
	StartedAtUTC      string `json:"startedAtUTC"`
	FinishedAtUTC     string `json:"finishedAtUTC,omitempty"`
	OwnerErrorMessage string `json:"ownerErrorMessage,omitempty"`
}

// resolvedOptions captures validated options with defaults applied.
type resolvedOptions struct {
	key                      string
	functionForOwner         func() error
	failIfRunningKeys        []string
	stateRootDirectoryPath   string
	ownerWaitPollingInterval time.Duration
}

type waitForOwnerCompletionInput struct {
	runStatePath             string
	runLock                  *lockfile.PIDLock
	ownerWaitPollingInterval time.Duration
}

type runFunctionAsOwnerInput struct {
	resolvedOptions             resolvedOptions
	runStatePath                string
	runLock                     *lockfile.PIDLock
	additionalFailIfRunningLock []namedLock
}

type namedLock struct {
	key  string
	lock *lockfile.PIDLock
}

// Run executes options.Func as the owner and coalesces concurrent callers onto
// the owner result for the same key.
func Run(options Options) error {
	resolvedOptions, resolveError := resolveOptions(options)
	if resolveError != nil {
		return resolveError
	}

	if createDirectoryError := os.MkdirAll(
		resolvedOptions.stateRootDirectoryPath,
		0o755,
	); createDirectoryError != nil {
		return fmt.Errorf(
			"create state root directory %q: %w",
			resolvedOptions.stateRootDirectoryPath,
			createDirectoryError,
		)
	}

	if cleanupError := cleanupStaleCompletedRunArtifactsForAllKeys(
		resolvedOptions.stateRootDirectoryPath,
	); cleanupError != nil {
		return cleanupError
	}

	runStatePath, runLockPath := buildRunFilePaths(
		resolvedOptions.stateRootDirectoryPath,
		resolvedOptions.key,
	)
	runLock := lockfile.NewPIDLock(runLockPath)

	for {
		acquireError := runLock.Acquire()
		if acquireError == nil {
			activeConflictKey, additionalLocks, conflictError := acquireAdditionalFailIfRunningLocks(
				resolvedOptions,
				resolvedOptions.key,
			)
			if conflictError != nil {
				_ = runLock.Release()
				return conflictError
			}
			if strings.TrimSpace(activeConflictKey) != "" {
				_ = runLock.Release()
				return fmt.Errorf("%w: key %q", ErrAlreadyRunning, activeConflictKey)
			}

			return runFunctionAsOwner(
				runFunctionAsOwnerInput{
					resolvedOptions:             resolvedOptions,
					runStatePath:                runStatePath,
					runLock:                     runLock,
					additionalFailIfRunningLock: additionalLocks,
				},
			)
		}
		if !errors.Is(acquireError, lockfile.ErrLockHeld) {
			return fmt.Errorf(
				"acquire coalescing lock for key %q: %w",
				resolvedOptions.key,
				acquireError,
			)
		}
		if hasFailIfRunningKey(resolvedOptions.failIfRunningKeys, resolvedOptions.key) {
			return fmt.Errorf("%w: key %q", ErrAlreadyRunning, resolvedOptions.key)
		}
		activeConflictKey, probeConflictError := findActiveFailIfRunningConflictKey(
			resolvedOptions,
			resolvedOptions.key,
		)
		if probeConflictError != nil {
			return probeConflictError
		}
		if strings.TrimSpace(activeConflictKey) != "" {
			return fmt.Errorf("%w: key %q", ErrAlreadyRunning, activeConflictKey)
		}

		waitResult, waitError := waitForOwnerCompletion(
			waitForOwnerCompletionInput{
				runStatePath:             runStatePath,
				runLock:                  runLock,
				ownerWaitPollingInterval: resolvedOptions.ownerWaitPollingInterval,
			},
		)
		if waitError != nil {
			return waitError
		}
		if waitResult.joined {
			return waitResult.ownerError
		}
	}
}

// MustRun executes Run and panics when Run returns a non-nil error.
func MustRun(options Options) {
	runError := Run(options)
	if runError != nil {
		panic(
			fmt.Sprintf(
				"coalescecmd.MustRun: %v",
				runError,
			),
		)
	}
}

func cleanupStaleCompletedRunArtifactsForAllKeys(
	stateRootDirectoryPath string,
) error {
	stateRootDirectoryEntries, readStateRootDirectoryError := os.ReadDir(
		stateRootDirectoryPath,
	)
	if readStateRootDirectoryError != nil {
		if os.IsNotExist(readStateRootDirectoryError) {
			return nil
		}
		return fmt.Errorf(
			"read state root directory %q for stale artifact cleanup: %w",
			stateRootDirectoryPath,
			readStateRootDirectoryError,
		)
	}

	for _, stateRootDirectoryEntry := range stateRootDirectoryEntries {
		if stateRootDirectoryEntry.IsDir() {
			continue
		}
		stateFileName := stateRootDirectoryEntry.Name()
		if !strings.HasSuffix(stateFileName, runStateFileSuffix) {
			continue
		}

		runStatePath := filepath.Join(
			stateRootDirectoryPath,
			stateFileName,
		)
		runFileBaseName := strings.TrimSuffix(stateFileName, runStateFileSuffix)
		runLockPath := filepath.Join(
			stateRootDirectoryPath,
			runFileBaseName+runLockFileSuffix,
		)
		runLock := lockfile.NewPIDLock(runLockPath)
		if runLock == nil {
			continue
		}

		if cleanupError := cleanupStaleCompletedRunArtifactsForOneKey(
			runStatePath,
			runLock,
		); cleanupError != nil {
			return cleanupError
		}
	}

	return nil
}

func cleanupStaleCompletedRunArtifactsForOneKey(
	runStatePath string,
	runLock *lockfile.PIDLock,
) error {
	if runLock == nil {
		return nil
	}

	acquireError := runLock.Acquire()
	if acquireError != nil {
		if errors.Is(acquireError, lockfile.ErrLockHeld) {
			return nil
		}
		return fmt.Errorf(
			"acquire lock for stale artifact cleanup: %w",
			acquireError,
		)
	}

	releaseLockAtFunctionExit := true
	defer func() {
		if releaseLockAtFunctionExit {
			_ = runLock.Release()
		}
	}()

	stateSnapshot, stateFound, readStateError := readRunStateFile(runStatePath)
	if readStateError != nil {
		if errors.Is(readStateError, errInvalidRunStateFile) {
			removeInvalidStateError := removeRunStateFileAndReleaseLock(
				runStatePath,
				runLock,
			)
			if removeInvalidStateError != nil {
				return removeInvalidStateError
			}
			releaseLockAtFunctionExit = false
			return nil
		}
		return readStateError
	}
	if !stateFound || stateSnapshot.Status != runStateStatusDone {
		return nil
	}

	finishedAtTimestamp := strings.TrimSpace(stateSnapshot.FinishedAtUTC)
	finishedAtUTC, parseFinishedAtError := time.Parse(
		time.RFC3339Nano,
		finishedAtTimestamp,
	)
	if parseFinishedAtError != nil {
		removeInvalidTimestampStateError := removeRunStateFileAndReleaseLock(
			runStatePath,
			runLock,
		)
		if removeInvalidTimestampStateError != nil {
			return removeInvalidTimestampStateError
		}
		releaseLockAtFunctionExit = false
		return nil
	}
	if time.Since(finishedAtUTC) < staleCompletedRunMinimumAgeBeforeCleanup {
		return nil
	}

	removeStaleStateError := removeRunStateFileAndReleaseLock(
		runStatePath,
		runLock,
	)
	if removeStaleStateError != nil {
		return removeStaleStateError
	}
	releaseLockAtFunctionExit = false

	return nil
}

func removeRunStateFileAndReleaseLock(
	runStatePath string,
	runLock *lockfile.PIDLock,
) error {
	removeStateError := os.Remove(runStatePath)
	if removeStateError != nil && !os.IsNotExist(removeStateError) {
		return fmt.Errorf(
			"remove stale run state file %q: %w",
			runStatePath,
			removeStateError,
		)
	}

	if runLock == nil {
		return nil
	}
	if releaseLockError := runLock.Release(); releaseLockError != nil {
		return fmt.Errorf(
			"release lock after stale artifact cleanup: %w",
			releaseLockError,
		)
	}
	return nil
}

func hasFailIfRunningKey(failIfRunningKeys []string, candidateKey string) bool {
	for _, failIfRunningKey := range failIfRunningKeys {
		if failIfRunningKey == candidateKey {
			return true
		}
	}
	return false
}

func findActiveFailIfRunningConflictKey(
	resolvedOptions resolvedOptions,
	primaryKey string,
) (string, error) {
	for _, failIfRunningKey := range resolvedOptions.failIfRunningKeys {
		if failIfRunningKey == primaryKey {
			continue
		}

		failIfRunningLock := lockfile.NewPIDLock(
			buildRunLockPath(
				resolvedOptions.stateRootDirectoryPath,
				failIfRunningKey,
			),
		)

		acquireError := failIfRunningLock.Acquire()
		if acquireError == nil {
			releaseError := failIfRunningLock.Release()
			if releaseError != nil {
				return "", fmt.Errorf(
					"release fail-if-running probe lock for key %q: %w",
					failIfRunningKey,
					releaseError,
				)
			}
			continue
		}
		if errors.Is(acquireError, lockfile.ErrLockHeld) {
			return failIfRunningKey, nil
		}
		return "", fmt.Errorf(
			"probe fail-if-running lock for key %q: %w",
			failIfRunningKey,
			acquireError,
		)
	}

	return "", nil
}

func acquireAdditionalFailIfRunningLocks(
	resolvedOptions resolvedOptions,
	primaryKey string,
) (string, []namedLock, error) {
	heldLocks := make([]namedLock, 0, len(resolvedOptions.failIfRunningKeys))
	releaseHeldLocks := func() error {
		return releaseNamedLocks(heldLocks)
	}

	for _, failIfRunningKey := range resolvedOptions.failIfRunningKeys {
		if failIfRunningKey == primaryKey {
			continue
		}

		failIfRunningLock := lockfile.NewPIDLock(
			buildRunLockPath(
				resolvedOptions.stateRootDirectoryPath,
				failIfRunningKey,
			),
		)

		acquireError := failIfRunningLock.Acquire()
		if acquireError == nil {
			heldLocks = append(heldLocks, namedLock{
				key:  failIfRunningKey,
				lock: failIfRunningLock,
			})
			continue
		}
		if errors.Is(acquireError, lockfile.ErrLockHeld) {
			_ = releaseHeldLocks()
			return failIfRunningKey, nil, nil
		}
		_ = releaseHeldLocks()
		return "", nil, fmt.Errorf(
			"acquire additional fail-if-running lock for key %q: %w",
			failIfRunningKey,
			acquireError,
		)
	}

	return "", heldLocks, nil
}

func releaseNamedLocks(heldLocks []namedLock) error {
	var releaseErrors []error
	for lockIndex := len(heldLocks) - 1; lockIndex >= 0; lockIndex -= 1 {
		heldLock := heldLocks[lockIndex]
		if heldLock.lock == nil {
			continue
		}
		releaseError := heldLock.lock.Release()
		if releaseError != nil {
			releaseErrors = append(
				releaseErrors,
				fmt.Errorf(
					"release fail-if-running lock for key %q: %w",
					heldLock.key,
					releaseError,
				),
			)
		}
	}
	if len(releaseErrors) == 0 {
		return nil
	}
	return errors.Join(releaseErrors...)
}

type waitForOwnerCompletionResult struct {
	joined     bool
	ownerError error
}

func waitForOwnerCompletion(
	input waitForOwnerCompletionInput,
) (waitForOwnerCompletionResult, error) {
	initialDoneRunID := ""
	initialStateSnapshot, initialStateFound, initialStateError := readRunStateFile(
		input.runStatePath,
	)
	if initialStateError != nil {
		return waitForOwnerCompletionResult{}, initialStateError
	}
	if initialStateFound && initialStateSnapshot.Status == runStateStatusDone {
		initialDoneRunID = strings.TrimSpace(initialStateSnapshot.RunID)
	}

	trackedRunID := ""
	sawRunningState := false

	for {
		stateSnapshot, stateFound, readStateError := readRunStateFile(input.runStatePath)
		if readStateError != nil {
			return waitForOwnerCompletionResult{}, readStateError
		}
		if stateFound {
			if stateSnapshot.Status == runStateStatusRunning {
				trackedRunID = stateSnapshot.RunID
				sawRunningState = true
			}
			if didObserveOwnerCompletionForCurrentWait(
				stateSnapshot,
				sawRunningState,
				trackedRunID,
				initialDoneRunID,
			) {
				return waitForOwnerCompletionResult{
					joined:     true,
					ownerError: ownerErrorFromState(stateSnapshot),
				}, nil
			}
		}

		acquireProbeError := input.runLock.Acquire()
		if acquireProbeError == nil {
			latestState, latestStateFound, latestStateError := readRunStateFile(
				input.runStatePath,
			)
			if latestStateError != nil {
				_ = input.runLock.Release()
				return waitForOwnerCompletionResult{}, latestStateError
			}
			releaseProbeError := input.runLock.Release()
			if releaseProbeError != nil {
				return waitForOwnerCompletionResult{}, fmt.Errorf(
					"release lock while probing owner completion: %w",
					releaseProbeError,
				)
			}

			if latestStateFound && didObserveOwnerCompletionForCurrentWait(
				latestState,
				sawRunningState,
				trackedRunID,
				initialDoneRunID,
			) {
				return waitForOwnerCompletionResult{
					joined:     true,
					ownerError: ownerErrorFromState(latestState),
				}, nil
			}
			return waitForOwnerCompletionResult{joined: false}, nil
		}
		if !errors.Is(acquireProbeError, lockfile.ErrLockHeld) {
			return waitForOwnerCompletionResult{}, fmt.Errorf(
				"probe lock while waiting for owner completion: %w",
				acquireProbeError,
			)
		}

		time.Sleep(input.ownerWaitPollingInterval)
	}
}

func didObserveOwnerCompletionForCurrentWait(
	stateSnapshot runStateFile,
	sawRunningState bool,
	trackedRunID string,
	initialDoneRunID string,
) bool {
	if stateSnapshot.Status != runStateStatusDone {
		return false
	}

	if sawRunningState {
		trackedRunID = strings.TrimSpace(trackedRunID)
		if trackedRunID == "" {
			return false
		}
		return stateSnapshot.RunID == trackedRunID
	}

	observedDoneRunID := strings.TrimSpace(stateSnapshot.RunID)
	if observedDoneRunID == "" {
		return false
	}
	return observedDoneRunID != initialDoneRunID
}

func runFunctionAsOwner(input runFunctionAsOwnerInput) error {
	releaseLocks := func() error {
		additionalLockReleaseError := releaseNamedLocks(
			input.additionalFailIfRunningLock,
		)

		var runLockReleaseError error
		if input.runLock != nil {
			runLockReleaseError = input.runLock.Release()
		}

		if additionalLockReleaseError == nil && runLockReleaseError == nil {
			return nil
		}
		return errors.Join(
			additionalLockReleaseError,
			runLockReleaseError,
		)
	}

	runID := buildRunID()
	startedAt := time.Now().UTC()
	runningState := runStateFile{
		SchemaVersion: runStateSchemaVersion,
		Key:           input.resolvedOptions.key,
		RunID:         runID,
		OwnerPID:      os.Getpid(),
		Status:        runStateStatusRunning,
		StartedAtUTC:  startedAt.Format(time.RFC3339Nano),
	}
	if writeRunningStateError := writeRunStateFile(
		input.runStatePath,
		runningState,
	); writeRunningStateError != nil {
		_ = releaseLocks()
		return writeRunningStateError
	}

	ownerFunctionError := input.resolvedOptions.functionForOwner()

	doneState := runningState
	doneState.Status = runStateStatusDone
	doneState.FinishedAtUTC = time.Now().UTC().Format(time.RFC3339Nano)
	if ownerFunctionError != nil {
		doneState.OwnerErrorMessage = ownerFunctionError.Error()
	}
	if writeDoneStateError := writeRunStateFile(input.runStatePath, doneState); writeDoneStateError != nil {
		_ = releaseLocks()
		if ownerFunctionError != nil {
			return fmt.Errorf(
				"owner function failed with %q and done-state write also failed: %w",
				ownerFunctionError.Error(),
				writeDoneStateError,
			)
		}
		return writeDoneStateError
	}

	if releaseError := releaseLocks(); releaseError != nil {
		if ownerFunctionError != nil {
			return fmt.Errorf(
				"owner function failed with %q and lock release also failed: %w",
				ownerFunctionError.Error(),
				releaseError,
			)
		}
		return fmt.Errorf("release owner lock: %w", releaseError)
	}

	return ownerFunctionError
}

func ownerErrorFromState(stateSnapshot runStateFile) error {
	if strings.TrimSpace(stateSnapshot.OwnerErrorMessage) == "" {
		return nil
	}
	return errors.New(stateSnapshot.OwnerErrorMessage)
}

func buildRunFilePaths(stateRootDirectoryPath string, coalescingKey string) (
	string,
	string,
) {
	runFileBaseName := buildRunFileBaseNameFromCoalescingKey(coalescingKey)

	runStatePath := filepath.Join(
		stateRootDirectoryPath,
		runFileBaseName+runStateFileSuffix,
	)
	runLockPath := filepath.Join(
		stateRootDirectoryPath,
		runFileBaseName+runLockFileSuffix,
	)
	return runStatePath, runLockPath
}

func buildRunLockPath(
	stateRootDirectoryPath string,
	coalescingKey string,
) string {
	_, runLockPath := buildRunFilePaths(
		stateRootDirectoryPath,
		coalescingKey,
	)
	return runLockPath
}

func buildRunFileBaseNameFromCoalescingKey(coalescingKey string) string {
	trimmedKey := strings.TrimSpace(coalescingKey)
	keyHash := sha256.Sum256([]byte(trimmedKey))
	return strings.ToLower(
		base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(
			keyHash[:8],
		),
	)
}

func buildRunID() string {
	return fmt.Sprintf(
		"%d-%d",
		time.Now().UTC().UnixNano(),
		os.Getpid(),
	)
}

func writeRunStateFile(
	runStatePath string,
	stateSnapshot runStateFile,
) error {
	encodedStateBytes, encodeStateError := json.MarshalIndent(
		stateSnapshot,
		"",
		"\t",
	)
	if encodeStateError != nil {
		return fmt.Errorf(
			"encode run state file %q: %w",
			runStatePath,
			encodeStateError,
		)
	}
	encodedStateBytes = append(encodedStateBytes, '\n')

	if ensureStateDirectoryError := os.MkdirAll(filepath.Dir(runStatePath), 0o755); ensureStateDirectoryError != nil {
		return fmt.Errorf(
			"create run state directory for %q: %w",
			runStatePath,
			ensureStateDirectoryError,
		)
	}

	temporaryFileHandle, createTemporaryFileError := os.CreateTemp(
		filepath.Dir(runStatePath),
		filepath.Base(runStatePath)+".tmp-*",
	)
	if createTemporaryFileError != nil {
		return fmt.Errorf(
			"create temporary run state file for %q: %w",
			runStatePath,
			createTemporaryFileError,
		)
	}
	temporaryFilePath := temporaryFileHandle.Name()
	cleanupTemporaryFilePath := true
	defer func() {
		if cleanupTemporaryFilePath {
			_ = os.Remove(temporaryFilePath)
		}
	}()

	if _, writeStateError := temporaryFileHandle.Write(encodedStateBytes); writeStateError != nil {
		_ = temporaryFileHandle.Close()
		return fmt.Errorf(
			"write temporary run state file %q: %w",
			temporaryFilePath,
			writeStateError,
		)
	}
	if chmodTemporaryFileError := temporaryFileHandle.Chmod(0o644); chmodTemporaryFileError != nil {
		_ = temporaryFileHandle.Close()
		return fmt.Errorf(
			"chmod temporary run state file %q: %w",
			temporaryFilePath,
			chmodTemporaryFileError,
		)
	}
	if closeTemporaryFileError := temporaryFileHandle.Close(); closeTemporaryFileError != nil {
		return fmt.Errorf(
			"close temporary run state file %q: %w",
			temporaryFilePath,
			closeTemporaryFileError,
		)
	}
	if renameStateFileError := os.Rename(temporaryFilePath, runStatePath); renameStateFileError != nil {
		return fmt.Errorf(
			"rename temporary run state file %q to %q: %w",
			temporaryFilePath,
			runStatePath,
			renameStateFileError,
		)
	}
	cleanupTemporaryFilePath = false
	return nil
}

func readRunStateFile(runStatePath string) (runStateFile, bool, error) {
	runStateBytes, readStateError := os.ReadFile(runStatePath)
	if readStateError != nil {
		if os.IsNotExist(readStateError) {
			return runStateFile{}, false, nil
		}
		return runStateFile{}, false, fmt.Errorf(
			"read run state file %q: %w",
			runStatePath,
			readStateError,
		)
	}

	var stateSnapshot runStateFile
	if decodeStateError := json.Unmarshal(runStateBytes, &stateSnapshot); decodeStateError != nil {
		return runStateFile{}, false, fmt.Errorf(
			"%w: decode run state file %q: %v",
			errInvalidRunStateFile,
			runStatePath,
			decodeStateError,
		)
	}
	if stateSnapshot.SchemaVersion != runStateSchemaVersion {
		return runStateFile{}, false, fmt.Errorf(
			"%w: unsupported run state schema version %d in %q",
			errInvalidRunStateFile,
			stateSnapshot.SchemaVersion,
			runStatePath,
		)
	}
	return stateSnapshot, true, nil
}

func resolveOptions(options Options) (resolvedOptions, error) {
	resolved := resolvedOptions{
		key:                      strings.TrimSpace(options.Key),
		functionForOwner:         options.Func,
		failIfRunningKeys:        normalizeFailIfRunningKeys(options.FailIfRunning),
		stateRootDirectoryPath:   strings.TrimSpace(options.StateRootDirectoryPath),
		ownerWaitPollingInterval: options.OwnerWaitPollingInterval,
	}

	if resolved.key == "" {
		return resolvedOptions{}, errors.New("key is required")
	}
	if resolved.functionForOwner == nil {
		return resolvedOptions{}, errors.New("func is required")
	}
	if resolved.stateRootDirectoryPath == "" {
		panic("coalescecmd.Options.StateRootDirectoryPath is required")
	}
	if resolved.ownerWaitPollingInterval <= 0 {
		resolved.ownerWaitPollingInterval = DefaultPollInterval
	}

	return resolved, nil
}

func normalizeFailIfRunningKeys(rawFailIfRunningKeys []string) []string {
	normalizedFailIfRunningKeys := make([]string, 0, len(rawFailIfRunningKeys))
	seenKeys := make(map[string]struct{}, len(rawFailIfRunningKeys))

	for _, rawFailIfRunningKey := range rawFailIfRunningKeys {
		normalizedFailIfRunningKey := strings.TrimSpace(rawFailIfRunningKey)
		if normalizedFailIfRunningKey == "" {
			continue
		}
		if _, isSeen := seenKeys[normalizedFailIfRunningKey]; isSeen {
			continue
		}
		seenKeys[normalizedFailIfRunningKey] = struct{}{}
		normalizedFailIfRunningKeys = append(
			normalizedFailIfRunningKeys,
			normalizedFailIfRunningKey,
		)
	}

	sort.Strings(normalizedFailIfRunningKeys)
	return normalizedFailIfRunningKeys
}
