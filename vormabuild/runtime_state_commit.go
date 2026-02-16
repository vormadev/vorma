package vormabuild

import (
	"fmt"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

type runtimeStateRoutePathsUpdateMode uint8

const (
	runtimeStateRoutePathsUpdateModeNoPathMutation runtimeStateRoutePathsUpdateMode = iota
	runtimeStateRoutePathsUpdateModeReplaceParsedPathsForInit
	runtimeStateRoutePathsUpdateModeSyncFromDevReload
)

type runtimeStateCommitInput struct {
	shouldCommitIsDev             bool
	isDev                         bool
	shouldCommitBuildID           bool
	buildID                       string
	shouldCommitRouteManifestFile bool
	routeManifestFile             string
	routePaths                    map[string]*vormaruntime.Path
	routePathsUpdateMode          runtimeStateRoutePathsUpdateMode
	shouldRebuildNestedRouter     bool
}

func commitRuntimeState(v *vormaruntime.Vorma, runtimeStateCommitInput runtimeStateCommitInput) {
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		commitRuntimeStateWithLock(l, runtimeStateCommitInput)
	})
}

func commitRuntimeStateWithLock(
	l *vormaruntime.LockedVorma,
	runtimeStateCommitInput runtimeStateCommitInput,
) {
	if runtimeStateCommitInput.shouldCommitIsDev {
		l.SetIsDev(runtimeStateCommitInput.isDev)
	}

	switch runtimeStateCommitInput.routePathsUpdateMode {
	case runtimeStateRoutePathsUpdateModeNoPathMutation:
		// no-op
	case runtimeStateRoutePathsUpdateModeReplaceParsedPathsForInit:
		l.Routes().ReplaceParsedPathsForInit(
			runtimeStateCommitInput.routePaths,
			runtimeStateCommitInput.shouldRebuildNestedRouter,
		)
	case runtimeStateRoutePathsUpdateModeSyncFromDevReload:
		l.Routes().SyncFromDevReload(runtimeStateCommitInput.routePaths)
	default:
		panic(fmt.Sprintf(
			"unsupported runtime state route paths update mode: %d",
			runtimeStateCommitInput.routePathsUpdateMode,
		))
	}

	if runtimeStateCommitInput.shouldCommitBuildID {
		l.SetBuildID(runtimeStateCommitInput.buildID)
	}

	if runtimeStateCommitInput.shouldCommitRouteManifestFile {
		l.SetRouteManifestFile(runtimeStateCommitInput.routeManifestFile)
	}
}

func shouldRebuildNestedRouterFromCurrentRuntimeState(
	l *vormaruntime.LockedVorma,
) bool {
	return l.Vorma().LoadersRouter() != nil &&
		l.Vorma().LoadersRouter().NestedRouter != nil
}

func currentBuildIDWithReadLock(v *vormaruntime.Vorma) string {
	var currentBuildID string
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		currentBuildID = l.GetBuildID()
	})
	return currentBuildID
}

func shouldRestoreRuntimeStateSnapshotForAttemptBuildID(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	if currentAttemptCommittedBuildID == "" {
		return true
	}
	return currentBuildID == currentAttemptCommittedBuildID
}
