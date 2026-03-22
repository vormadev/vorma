package wavebuild

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

type scheduled_hook struct {
	sched_id  int // assigned by new_hook_scheduler; unique across all hooks
	name      string
	start_at  CheckpointOrd
	finish_by CheckpointOrd
	effects   []Effect

	// exactly one of cmd or fn may be set; both empty is valid
	// (the hook exists solely for its effects)
	cmd        string
	fn         func(ctx *PluginCtx) (*PluginResult, error)
	plugin_ctx *PluginCtx
}

func (h *scheduled_hook) is_noop() bool {
	return h.cmd == "" && h.fn == nil
}

type hook_result struct {
	err      error
	extras   *set.Set[Effect] // non-nil only for plugin hooks
	duration time.Duration
}

type tagged_hook_result struct {
	sched_id int
	result   hook_result
}

type hook_scheduler struct {
	hooks     []scheduled_hook
	ctx       context.Context
	cancel    context.CancelFunc
	results   chan tagged_hook_result    // single shared channel for all hooks
	collected map[int]tagged_hook_result // early arrivals from spanning hooks stashed here
	env       []string                   // extra env vars for shell hooks
	root_dir  strict.CWDRelPath          // working directory for shell hooks
	holds     *checkpoint_holds
	logger    *slog.Logger
}

func new_hook_scheduler(
	hooks []scheduled_hook,
	env []string,
	root_dir strict.CWDRelPath,
	holds *checkpoint_holds,
	logger *slog.Logger,
) *hook_scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	for i := range hooks {
		hooks[i].sched_id = i
		// plugin hooks share the scheduler's context so there is
		// a single cancellation source for both cmd and fn hooks
		if hooks[i].plugin_ctx != nil {
			hooks[i].plugin_ctx.ctx = ctx
		}
	}
	return &hook_scheduler{
		hooks:     hooks,
		ctx:       ctx,
		cancel:    cancel,
		results:   make(chan tagged_hook_result, len(hooks)),
		collected: make(map[int]tagged_hook_result),
		env:       env,
		root_dir:  root_dir,
		holds:     holds,
		logger:    logger,
	}
}

// start_hooks_at signals the checkpoint on all plugin contexts (so
// WaitFor calls unblock) and then launches all hooks whose StartAt
// matches the given checkpoint.
func (hs *hook_scheduler) start_hooks_at(checkpoint CheckpointOrd) {
	// signal checkpoint on every plugin ctx, not just those starting
	// here — a plugin started earlier may be blocking on WaitFor.
	for i := range hs.hooks {
		if hs.hooks[i].plugin_ctx != nil {
			hs.hooks[i].plugin_ctx.signal_checkpoint(checkpoint)
		}
	}

	var plugin_fn_started []chan struct{}
	for _, h := range hs.hooks {
		if h.start_at != checkpoint {
			continue
		}

		if h.is_noop() {
			hs.results <- tagged_hook_result{sched_id: h.sched_id}
			continue
		}

		hs.logger.Debug(fmt.Sprintf("Running %s", h.name))

		if h.fn != nil {
			started := make(chan struct{})
			plugin_fn_started = append(plugin_fn_started, started)
			go func(h scheduled_hook, started chan struct{}) {
				close(started)
				start := time.Now()
				result, err := h.fn(h.plugin_ctx)
				var extras *set.Set[Effect]
				if result != nil {
					extras = result.ExtraEffects
				}
				hs.results <- tagged_hook_result{
					sched_id: h.sched_id,
					result: hook_result{
						err:      err,
						extras:   extras,
						duration: time.Since(start),
					},
				}
			}(h, started)
		} else {
			go func(h scheduled_hook) {
				start := time.Now()
				err := run_shell_cmd(hs.ctx, h.cmd, hs.root_dir, hs.env)
				hs.results <- tagged_hook_result{
					sched_id: h.sched_id,
					result: hook_result{
						err:      err,
						duration: time.Since(start),
					},
				}
			}(h)
		}
	}

	for _, started := range plugin_fn_started {
		<-started
	}

	// Same-checkpoint BlockAt(...) is supported when called immediately at
	// hook entry or immediately after WaitFor(checkpoint) returns.
	if len(plugin_fn_started) > 0 {
		time.Sleep(10 * time.Millisecond)
	}
}

// await_checkpoint waits for all hooks whose FinishBy matches the
// given checkpoint. Returns the set of cycle effects implied by
// completed hooks' Effects values (with implication chains
// pre-resolved), plus any extra effects returned by plugin hooks.
// Also waits for all checkpoint holds to be released before
// returning.
//
// Reads from the shared results channel. Results that belong to
// later checkpoints are stashed in collected and consumed by the
// corresponding later await_checkpoint call. Fails fast on the
// first hook error, cancelling all running hooks immediately.
func (hs *hook_scheduler) await_checkpoint(
	checkpoint CheckpointOrd,
) (*set.Set[Effect], error) {
	expected := make(map[int]scheduled_hook)
	for _, h := range hs.hooks {
		if h.finish_by == checkpoint {
			expected[h.sched_id] = h
		}
	}

	fx := &set.Set[Effect]{}

	// check for early arrivals from previous checkpoint reads
	for id, tagged := range hs.collected {
		h, ok := expected[id]
		if !ok {
			continue
		}
		delete(hs.collected, id)
		delete(expected, id)
		if tagged.result.err != nil {
			hs.cancel()
			return nil, fmt.Errorf("%s failed: %w", h.name, tagged.result.err)
		}
		hs.apply_hook_result(h, tagged.result, fx)
	}

	// read from shared channel until all expected hooks report in
	for len(expected) > 0 {
		tagged := <-hs.results

		h, is_expected := expected[tagged.sched_id]
		if !is_expected {
			// belongs to a later checkpoint — stash it
			hs.collected[tagged.sched_id] = tagged
			continue
		}
		delete(expected, tagged.sched_id)

		if tagged.result.err != nil {
			hs.cancel()
			return nil, fmt.Errorf("%s failed: %w", h.name, tagged.result.err)
		}
		hs.apply_hook_result(h, tagged.result, fx)
	}

	// close the checkpoint for new holds, then wait for any already-
	// registered holds to be released.
	hs.holds.close_and_wait(checkpoint)

	return fx, nil
}

func (hs *hook_scheduler) apply_hook_result(
	h scheduled_hook,
	r hook_result,
	fx *set.Set[Effect],
) {
	if !h.is_noop() {
		hs.logger.Info(
			fmt.Sprintf("completed %s", h.name),
			"duration", r.duration.Round(time.Millisecond),
		)
	}
	for _, effect := range h.effects {
		for _, implied_effect := range hook_effect_to_effects(effect) {
			fx.Add(implied_effect)
		}
	}
	if r.extras != nil {
		for e := range r.extras.Range() {
			fx.Add(e)
		}
	}
}

// abort cancels the context, killing all running hook processes and
// unblocking any plugin WaitFor calls.
func (hs *hook_scheduler) abort() {
	hs.cancel()
}

// hook_effect_to_effects maps a hook Effect to the full set of cycle
// effects it implies, including implication chains.
func hook_effect_to_effects(e Effect) []Effect {
	switch e {
	case EffectRestartApp:
		return []Effect{
			EffectRestartApp,
			EffectShowFrontendRebuildingOverlay,
			EffectHardReloadBrowser,
		}
	case EffectHardReloadBrowser:
		return []Effect{
			EffectShowFrontendRebuildingOverlay,
			EffectHardReloadBrowser,
		}
	case EffectRevalidateClientData:
		return []Effect{EffectRevalidateClientData}
	case EffectProcessPrivateStatic:
		return []Effect{effect_build_private_filemap}
	case EffectShowFrontendRebuildingOverlay:
		return []Effect{EffectShowFrontendRebuildingOverlay}
	case EffectNoFrontendSettling:
		return []Effect{EffectNoFrontendSettling}
	}
	return nil
}

func run_shell_cmd(
	ctx context.Context,
	cmd string,
	dir strict.CWDRelPath,
	env []string,
) error {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/C", cmd)
	} else {
		c = exec.CommandContext(ctx, "sh", "-c", cmd)
	}
	var stderr_buf bytes.Buffer
	c.Stdout = os.Stdout
	c.Stderr = io.MultiWriter(os.Stderr, &stderr_buf)
	c.Dir = dir.Str()
	if len(env) > 0 {
		c.Env = append(os.Environ(), env...)
	}
	if err := c.Run(); err != nil {
		captured := strings.TrimSpace(stderr_buf.String())
		if captured != "" {
			return fmt.Errorf("%w: %s", err, captured)
		}
		return err
	}
	return nil
}
