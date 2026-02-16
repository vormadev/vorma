package vormaruntime

type runtimeLifecycleState string

const (
	runtimeLifecycleStateUninitialized   runtimeLifecycleState = "uninitialized"
	runtimeLifecycleStateInitializing    runtimeLifecycleState = "initializing"
	runtimeLifecycleStateReady           runtimeLifecycleState = "ready"
	runtimeLifecycleStateReloadingRoutes runtimeLifecycleState = "reloading-routes"
	runtimeLifecycleStateReloadingHTML   runtimeLifecycleState = "reloading-html-template"
)

func (v *Vorma) transitionLifecycleStateLocked(
	nextState runtimeLifecycleState,
	reason string,
	lastError string,
) {
	previousState := v._lifecycleState
	v._lifecycleTransitionSeq++
	v._lifecycleState = nextState
	v._lifecycleLastError = lastError

	v.Log.Debug(
		"Vorma lifecycle transition",
		"seq",
		v._lifecycleTransitionSeq,
		"from",
		previousState,
		"to",
		nextState,
		"reason",
		reason,
		"error",
		lastError,
		"build_id",
		v._buildID,
		"route_data_snapshot_version",
		v._routeDataSnapshotVersion,
		"is_dev",
		v._isDev,
		"route_count",
		len(v._paths),
		"route_manifest_file",
		v._routeManifestFile,
	)
}

func (v *Vorma) lifecycleStateForRouteCommitLocked() runtimeLifecycleState {
	if v._paths == nil {
		return runtimeLifecycleStateInitializing
	}
	return runtimeLifecycleStateReloadingRoutes
}
