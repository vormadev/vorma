package wavebuild

import (
	"context"
	"os"
	"sync"

	"github.com/vormadev/vorma/kit/tasks"
)

const waveInternalTestModeEnvVar = "__WAVE_INTERNAL_TEST_MODE"

/////////////////////////////////////////////////////////////////////
/////// Effect Labels
/////////////////////////////////////////////////////////////////////

const (
	_LABEL_P2_BUILD_GO_BINARY               = "phase_2.build_go_binary"
	_LABEL_P2_BUILD_CRITICAL_CSS            = "phase_2.build_critical_css"
	_LABEL_P2_BUILD_NORMAL_CSS              = "phase_2.build_normal_css"
	_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS  = "phase_2.process_public_static_assets"
	_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC   = "phase_2.cleanup_stale_public_static"
	_LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS = "phase_2.process_private_static_assets"

	_LABEL_P3_APPLY_DEV_SERVER_RESTART         = "phase_3.apply_dev_server_restart"
	_LABEL_P3_QUEUE_RETRY_WAIT_RESTART         = "phase_3.queue_retry_wait_restart"
	_LABEL_P3_RESTART_APP_PROCESS              = "phase_3.restart_app_process"
	_LABEL_P3_RESTART_VITE_PROCESS             = "phase_3.restart_vite_process"
	_LABEL_P3_REFRESH_FRAMEWORK_ROUTE          = "phase_3.refresh_framework_route"
	_LABEL_P3_REFRESH_FRAMEWORK_TEMPLATE       = "phase_3.refresh_framework_template"
	_LABEL_P3_REFRESH_FRAMEWORK_PUBLIC_FILEMAP = "phase_3.refresh_framework_public_filemap"

	_LABEL_P4_AWAIT_BACKEND_READINESS = "phase_4.await_backend_readiness"

	_LABEL_P5_BROADCAST_CSS_HOT_RELOAD           = "phase_5.broadcast_css_hot_reload"
	_LABEL_P5_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED = "phase_5.notify_vite_public_filemap_changed"
	_LABEL_P5_BROADCAST_REVALIDATE               = "phase_5.broadcast_revalidate"
	_LABEL_P5_BROADCAST_HARD_RELOAD              = "phase_5.broadcast_hard_reload"
	_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE    = "phase_5.publish_no_reload_needed_notice"
)

/////////////////////////////////////////////////////////////////////
/////// Test Recorder
/////////////////////////////////////////////////////////////////////

type testEffectRecorder struct {
	mutex        sync.Mutex
	effectLabels []string
}

type testEffectRecorderContextKey struct{}

func withTestEffectRecorder(
	nativeContext context.Context,
	effectRecorder *testEffectRecorder,
) context.Context {
	return context.WithValue(
		nativeContext,
		testEffectRecorderContextKey{},
		effectRecorder,
	)
}

func (effectRecorder *testEffectRecorder) snapshotEffectLabels() []string {
	effectRecorder.mutex.Lock()
	defer effectRecorder.mutex.Unlock()
	return append([]string(nil), effectRecorder.effectLabels...)
}

func isTestEnv() bool {
	return os.Getenv(waveInternalTestModeEnvVar) == "1"
}

func recordTestEffect(
	tasksCtx *tasks.Ctx,
	effectLabel string,
) bool {
	if !isTestEnv() {
		return false
	}
	effectRecorder := tasksCtx.NativeContext().Value(
		testEffectRecorderContextKey{},
	).(*testEffectRecorder)
	effectRecorder.mutex.Lock()
	effectRecorder.effectLabels = append(
		effectRecorder.effectLabels,
		effectLabel,
	)
	effectRecorder.mutex.Unlock()
	return true
}
