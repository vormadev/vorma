package vormabuild

import (
	"errors"
	"fmt"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

type postRouteSyncHook func(*vormaruntime.Vorma) error

type routeSyncExecutionOptions struct {
	parseClientRoutes          func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	generateBuildID            func() (string, error)
	parseClientRoutesErrorText string
	postSyncHook               postRouteSyncHook
}

func prepareParsedRouteSyncInput(
	v *vormaruntime.Vorma,
	parseClientRoutes func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error),
	generateBuildID func() (string, error),
	parseClientRoutesErrorContext string,
) (map[string]*vormaruntime.Path, string, error) {
	clientPaths, err := parseClientRoutes(v)
	if err != nil {
		if parseClientRoutesErrorContext != "" {
			return nil, "", fmt.Errorf("%s: %w", parseClientRoutesErrorContext, err)
		}
		return nil, "", err
	}

	buildID := ""
	if generateBuildID != nil {
		buildID, err = generateBuildID()
		if err != nil {
			return nil, "", err
		}
	}

	return clientPaths, buildID, nil
}

func runRouteSyncExecution(
	v *vormaruntime.Vorma,
	options routeSyncExecutionOptions,
) error {
	if options.parseClientRoutes == nil {
		return errors.New("route sync parse function is required")
	}

	clientPaths, buildID, err := prepareParsedRouteSyncInput(
		v,
		options.parseClientRoutes,
		options.generateBuildID,
		options.parseClientRoutesErrorText,
	)
	if err != nil {
		return err
	}

	return syncClientRoutesFromParsedPathsWithLock(v, clientPaths, buildID, options.postSyncHook)
}

func syncClientRoutesFromParsedPathsWithLock(
	v *vormaruntime.Vorma,
	clientPaths map[string]*vormaruntime.Path,
	buildID string,
	postSyncHook postRouteSyncHook,
) error {
	var previousRuntimeState buildRuntimeStateSnapshot
	var currentAttemptCommittedBuildID string
	return runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					previousRuntimeState = captureBuildRuntimeStateSnapshot(l)
					commitRuntimeStateWithLock(
						l,
						runtimeStateCommitInput{
							shouldCommitBuildID:  buildID != "",
							buildID:              buildID,
							routePaths:           clientPaths,
							routePathsUpdateMode: runtimeStateRoutePathsUpdateModeSyncFromDevReload,
						},
					)
					currentAttemptCommittedBuildID = l.GetBuildID()
				})

				if postSyncHook != nil {
					return postSyncHook(v)
				}
				return nil
			},
			rollbackOnFailure: func() error {
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					rollbackRouteSyncStateAfterPostSyncFailure(
						l,
						previousRuntimeState,
						currentAttemptCommittedBuildID,
					)
				})
				return nil
			},
		},
	)
}

func rollbackRouteSyncStateAfterPostSyncFailure(
	l *vormaruntime.LockedVorma,
	previousRuntimeState buildRuntimeStateSnapshot,
	currentAttemptCommittedBuildID string,
) {
	if !shouldRollbackRouteSyncStateAfterPostSyncFailure(
		l.GetBuildID(),
		currentAttemptCommittedBuildID,
	) {
		return
	}
	restoreBuildRuntimeStateSnapshot(l, previousRuntimeState)
}

func shouldRollbackRouteSyncStateAfterPostSyncFailure(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	return shouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		currentBuildID,
		currentAttemptCommittedBuildID,
	)
}
