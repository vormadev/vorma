package wavebuild

import (
	"io"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

func recover_panic_value(fn func()) (recovered any) {
	defer func() {
		recovered = recover()
	}()
	fn()
	return recovered
}

func TestCheckpointHolds_CloseAndWaitRejectsLateSameCheckpointHold(
	t *testing.T,
) {
	for i := 0; i < 20; i++ {
		holds := new_checkpoint_holds()
		release := holds.hold(2, "initial hook")

		wait_done := make(chan struct{})
		go func() {
			holds.close_and_wait(2)
			close(wait_done)
		}()

		deadline := time.Now().Add(time.Second)
		for {
			holds.mu.Lock()
			closed := holds.closed_upto >= 2
			holds.mu.Unlock()
			if closed {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf(
					"iteration %d: checkpoint 2 never closed for new holds",
					i,
				)
			}
			runtime.Gosched()
		}

		recovered := recover_panic_value(func() {
			holds.hold(2, "late hook")
		})
		if recovered == nil {
			t.Fatalf(
				"iteration %d: late same-checkpoint hold did not panic",
				i,
			)
		}

		select {
		case <-wait_done:
			t.Fatalf(
				"iteration %d: checkpoint 2 finished before release",
				i,
			)
		default:
		}

		release()

		select {
		case <-wait_done:
		case <-time.After(time.Second):
			t.Fatalf(
				"iteration %d: checkpoint 2 did not finish after release",
				i,
			)
		}
	}
}

func TestHookScheduler_SameCheckpointBlockAtHoldsAndThenRejectsLateBlock(
	t *testing.T,
) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	root_dir := strict.MustNormalizeCWDRelPath(".")

	for i := 0; i < 20; i++ {
		holds := new_checkpoint_holds()
		ss := &super_state{
			logger:          logger,
			cycle_holds:     holds,
			cycle_triggers:  &set.Set[trigger]{},
			cycle_evt_paths: &set.Set[strict.CWDRelPath]{},
			cycle_shared:    &plugin_shared_state{},
		}

		hook := validated_lifecycle_hook{
			name:        "test hook",
			plugin_name: "test plugin",
			start_at:    2,
			finish_by:   3,
		}

		release_allowed := make(chan struct{})
		finish_allowed := make(chan struct{})
		block_registered := make(chan struct{})
		late_block_recovered := make(chan any, 1)

		scheduled := scheduled_hook{
			name:      hook.name,
			start_at:  hook.start_at,
			finish_by: hook.finish_by,
			fn: func(ctx *PluginCtx) (*PluginResult, error) {
				release := ctx.BlockAt(2)
				close(block_registered)
				<-release_allowed
				release()
				late_block_recovered <- recover_panic_value(func() {
					ctx.BlockAt(2)
				})
				<-finish_allowed
				return nil, nil
			},
		}
		scheduled.plugin_ctx = new_plugin_ctx(ss, &hook)

		scheduler := new_hook_scheduler(
			[]scheduled_hook{scheduled},
			nil,
			root_dir,
			holds,
			logger,
		)

		scheduler.start_hooks_at(2)
		<-block_registered

		checkpoint_2_done := make(chan error, 1)
		go func() {
			_, err := scheduler.await_checkpoint(2)
			checkpoint_2_done <- err
		}()

		deadline := time.Now().Add(time.Second)
		for {
			holds.mu.Lock()
			closed := holds.closed_upto >= 2
			holds.mu.Unlock()
			if closed {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf(
					"iteration %d: scheduler never closed checkpoint 2",
					i,
				)
			}
			runtime.Gosched()
		}

		select {
		case err := <-checkpoint_2_done:
			t.Fatalf(
				"iteration %d: checkpoint 2 completed before release: %v",
				i,
				err,
			)
		default:
		}

		close(release_allowed)

		recovered := <-late_block_recovered
		if recovered == nil {
			t.Fatalf(
				"iteration %d: late BlockAt(2) did not panic",
				i,
			)
		}

		select {
		case err := <-checkpoint_2_done:
			if err != nil {
				t.Fatalf(
					"iteration %d: await_checkpoint(2) failed: %v",
					i,
					err,
				)
			}
		case <-time.After(time.Second):
			t.Fatalf(
				"iteration %d: checkpoint 2 did not complete after release",
				i,
			)
		}

		close(finish_allowed)

		if _, err := scheduler.await_checkpoint(3); err != nil {
			t.Fatalf(
				"iteration %d: await_checkpoint(3) failed: %v",
				i,
				err,
			)
		}
	}
}
