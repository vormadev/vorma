package vormabuild

import (
	"errors"
	"fmt"

	"github.com/vormadev/vorma/vormaruntime"
)

type postRouteSyncHook func(*vormaruntime.LockedVorma) error

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
	var syncErr error
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		previousRuntimeState := captureRouteBuildRuntimeStateSnapshot(l)
		syncErr = runWithRollbackOnFailureAndPanic(
			rollbackTransactionOptions{
				run: func() error {
					if buildID != "" {
						l.SetBuildID(buildID)
					}
					l.Routes().SyncFromDevReload(clientPaths)
					if postSyncHook != nil {
						return postSyncHook(l)
					}
					return nil
				},
				rollbackOnFailure: func() error {
					rollbackRouteSyncStateAfterPostSyncFailure(
						l,
						previousRuntimeState,
					)
					return nil
				},
			},
		)
	})
	return syncErr
}

func rollbackRouteSyncStateAfterPostSyncFailure(
	l *vormaruntime.LockedVorma,
	previousRuntimeState routeBuildRuntimeStateSnapshot,
) {
	restoreRouteBuildRuntimeStateSnapshot(l, previousRuntimeState)
}
