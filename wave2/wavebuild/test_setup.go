package wavebuild

import (
	"context"
	"os"
	"sync"

	"github.com/vormadev/vorma/kit/tasks"
)

const wave_internal_test_mode_env_var = "__WAVE_INTERNAL_TEST_MODE"

/////////////////////////////////////////////////////////////////////
/////// Effect Labels
/////////////////////////////////////////////////////////////////////

const (
	_LABEL_P2_BUILD_GO_BINARY               = "phase-2.build-go-binary"
	_LABEL_P2_BUILD_CRITICAL_CSS            = "phase-2.build-critical-css"
	_LABEL_P2_BUILD_NORMAL_CSS              = "phase-2.build-normal-css"
	_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS  = "phase-2.process-public-static-assets"
	_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC   = "phase-2.cleanup-stale-public-static"
	_LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS = "phase-2.process-private-static-assets"

	_LABEL_P3_APPLY_DEV_SERVER_RESTART   = "phase-3.apply-dev-server-restart"
	_LABEL_P3_QUEUE_RETRY_WAIT_RESTART   = "phase-3.queue-retry-wait-restart"
	_LABEL_P3_RESTART_APP_PROCESS        = "phase-3.restart-app-process"
	_LABEL_P3_RESTART_VITE_PROCESS       = "phase-3.restart-vite-process"
	_LABEL_P3_EXECUTE_Fw_MUTATION_EFFECT = "phase-3.execute-fw-mutation-effect"

	_LABEL_P4_AWAIT_BACKEND_READINESS = "phase-4.await-backend-readiness"
	_LABEL_P4_EXECUTE_Fw_NOTIFICATION = "phase-4.execute-fw-notification"

	_LABEL_P5_BROADCAST_CSS_HOT_RELOAD           = "phase-5.broadcast-css-hot-reload"
	_LABEL_P5_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED = "phase-5.notify-vite-public-filemap-changed"
	_LABEL_P5_BROADCAST_REVALIDATE               = "phase-5.broadcast-revalidate"
	_LABEL_P5_BROADCAST_HARD_RELOAD              = "phase-5.broadcast-hard-reload"
	_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE    = "phase-5.publish-no-reload-needed-notice"
)

/////////////////////////////////////////////////////////////////////
/////// Test Recorder
/////////////////////////////////////////////////////////////////////

type test_effect_recorder struct {
	mutex         sync.Mutex
	effect_labels []string
}

type test_effect_recorder_context_key struct{}

func with_test_effect_recorder(
	native_context context.Context,
	effect_recorder *test_effect_recorder,
) context.Context {
	return context.WithValue(
		native_context,
		test_effect_recorder_context_key{},
		effect_recorder,
	)
}

func (recorder *test_effect_recorder) snapshot_effect_labels() []string {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	return append([]string(nil), recorder.effect_labels...)
}

func is_test_env() bool {
	return os.Getenv(wave_internal_test_mode_env_var) == "1"
}

func record_test_effect(
	tasks_ctx *tasks.Ctx,
	effect_label string,
) bool {
	if !is_test_env() {
		return false
	}
	effect_recorder := tasks_ctx.NativeContext().Value(
		test_effect_recorder_context_key{},
	).(*test_effect_recorder)
	effect_recorder.mutex.Lock()
	effect_recorder.effect_labels = append(
		effect_recorder.effect_labels,
		effect_label,
	)
	effect_recorder.mutex.Unlock()
	return true
}
