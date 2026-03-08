// Package wavebuild defines the build/dev orchestration contracts for Wave2.
package wavebuild

import (
	"errors"
	"strings"
)

// mode selects development or production behavior.
type mode string

const (
	// mode_dev applies development-time policy.
	mode_dev mode = "dev"
	// mode_prod applies production-time policy.
	mode_prod mode = "prod"
)

type p1_input struct {
	mode                 mode
	generation_id        string
	batch_watcher_events []batch_watcher_event
}

type p1_batch_input struct {
	p1 *p1_input
}

var (
	err_p1_input_required = errors.New(
		"wavebuild: phase-1 input is required",
	)
	err_generation_id_required = errors.New(
		"wavebuild: generation id is required",
	)
)

func (input p1_batch_input) normalize_generation_id() (string, error) {
	if input.p1 == nil {
		return "", err_p1_input_required
	}
	normalized_generation_id := strings.TrimSpace(input.p1.generation_id)
	if normalized_generation_id == "" {
		return "", err_generation_id_required
	}
	return normalized_generation_id, nil
}

type fw_notif_destination_key string

type fw_notif_failure_policy string

const (
	fw_notif_failure_policy_fail_pipeline                      fw_notif_failure_policy = "fail-pipeline"
	fw_notif_failure_policy_restart_backend_without_go_compile fw_notif_failure_policy = "restart-backend-without-go-compile"
)

type fw_notif struct {
	destination_key fw_notif_destination_key
	freshness_token string
	trigger         string
	metadata        map[string]string
	wait_for_app    bool
	wait_for_vite   bool
	failure_policy  fw_notif_failure_policy
}

// FrameworkNotificationFailurePolicy controls how Wave handles framework
// notification delivery failures.
type FrameworkNotificationFailurePolicy = fw_notif_failure_policy

// FrameworkNotification describes one framework notification emitted by Wave.
type FrameworkNotification = fw_notif

const (
	// FrameworkNotificationFailurePolicyFailPipeline surfaces notification
	// transport failure as pipeline failure.
	FrameworkNotificationFailurePolicyFailPipeline FrameworkNotificationFailurePolicy = fw_notif_failure_policy_fail_pipeline
	// FrameworkNotificationFailurePolicyRestartBackendWithoutGoCompile requests
	// backend restart without Go recompilation and skips frontend settling.
	FrameworkNotificationFailurePolicyRestartBackendWithoutGoCompile FrameworkNotificationFailurePolicy = fw_notif_failure_policy_restart_backend_without_go_compile
)

// DestinationKey returns framework notification destination registration key.
func (notif fw_notif) DestinationKey() string {
	return string(notif.destination_key)
}

// FreshnessToken returns the freshness token for stale-attempt rejection.
func (notif fw_notif) FreshnessToken() string {
	return notif.freshness_token
}

// Trigger returns stable notification trigger text.
func (notif fw_notif) Trigger() string {
	return notif.trigger
}

// Metadata returns optional stable key/value metadata.
func (notif fw_notif) Metadata() map[string]string {
	return notif.metadata
}

// WaitForApp returns whether notification transport is app-readiness gated.
func (notif fw_notif) WaitForApp() bool {
	return notif.wait_for_app
}

// WaitForVite returns whether notification transport is vite-readiness gated.
func (notif fw_notif) WaitForVite() bool {
	return notif.wait_for_vite
}

// FailurePolicy returns policy used when notification transport fails.
func (notif fw_notif) FailurePolicy() FrameworkNotificationFailurePolicy {
	return notif.failure_policy
}
