package wavebuild

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave/internal/workset"
)

type Plugin struct {
	Name           string
	Config         PluginConfig
	LifecycleHooks []LifecycleHook

	// __TODO -- hmm, this seems like it shouldn't be global:

	// OwnsPublicStaticFrontendSettling, when true, tells Wave that this
	// plugin handles browser settling for public static asset
	// changes (e.g. via vite notification). Wave will not add a
	// default HardReloadBrowser effect for public static changes.
	// The plugin is responsible for producing the correct effect
	// via its own hooks.
	OwnsPublicStaticFrontendSettling bool
}

// RawJSON is nil when the plugin's config section is absent from the
// config file. ParseFunc is still called on every reload in that case
// so the plugin can clear any captured config state.
type PluginConfigParseCtx struct {
	RawJSON *json.RawMessage
	*UserConfig
}

type PluginConfigParseFunc func(ctx *PluginConfigParseCtx) error

type PluginConfig struct {
	// JSONKey is the root-level JSON key in the config file that
	// belongs to this plugin (e.g. "Vorma"). Must not collide with
	// Wave's reserved keys. Optional — leave empty if the plugin
	// does not need config.
	JSONKey string

	// JSONSchema is the JSON schema entry for the plugin's config
	// section. Used when writing the config schema file. Required
	// when Key is set.
	JSONSchema jsonschema.Entry

	// ParseJSON is called with the plugin's raw JSON config section
	// and a ConfigReader that provides read access to the validated
	// user config. The function is responsible for unmarshaling,
	// validating, and normalizing the config into whatever the
	// plugin needs (typically via a struct pointer captured in the
	// closure). Called on every config reload, even when the
	// plugin's section is absent, in which case ctx.RawJSON is nil
	// and the plugin is responsible for clearing any captured
	// config state it wants reset.
	// Required when Key is set.
	ParseFunc PluginConfigParseFunc
}

// PluginResult lets a plugin inject additional effects into the
// current build cycle. Return nil for "nothing extra."
type PluginResult struct{ ExtraEffects *set.Set[workset.Effect] }

/////////////////////////////////////////////////////////////////////
/////// Validated plugin types
/////////////////////////////////////////////////////////////////////

type validated_plugin_config struct {
	json_key    string
	json_schema jsonschema.Entry
	parse_fn    PluginConfigParseFunc
}

func validate_plugin_config(
	plugin_cfg PluginConfig,
) (*validated_plugin_config, error) {
	if plugin_cfg.JSONKey != "" {
		if slices.Contains(reserved_json_cfg_keys, plugin_cfg.JSONKey) {
			return nil, fmt.Errorf(
				"plugin config key %q collides with a reserved Wave schema key",
				plugin_cfg.JSONKey,
			)
		}
		// __TODO wait why is ParseFunc required??
		if plugin_cfg.ParseFunc == nil {
			return nil, fmt.Errorf(
				"plugin config key %q is set but ParseFunc is nil",
				plugin_cfg.JSONKey,
			)
		}
	}
	return &validated_plugin_config{
		json_key:    plugin_cfg.JSONKey,
		json_schema: plugin_cfg.JSONSchema,
		parse_fn:    plugin_cfg.ParseFunc,
	}, nil
}

type validated_plugin_runtime struct {
	hooks                                []validated_lifecycle_hook
	owns_public_static_frontend_settling bool
}

func validate_plugin_runtime(
	plugin_name string,
	_hooks []LifecycleHook,
	owns_public_static_settling bool,
	root_dir strict.CWDRelPath,
) (*validated_plugin_runtime, error) {
	hooks := make([]validated_lifecycle_hook, len(_hooks))
	for i := range _hooks {
		label := fmt.Sprintf("plugin %q idx %d hook", plugin_name, i)
		if _hooks[i].IsGoCompile {
			label = plugin_name + " Go compile hook"
		}
		vh, err := _hooks[i].to_validated_hook(label, root_dir)
		if err != nil {
			return nil, err
		}
		vh.plugin_name = plugin_name
		hooks[i] = *vh
	}
	return &validated_plugin_runtime{
		hooks:                                hooks,
		owns_public_static_frontend_settling: owns_public_static_settling,
	}, nil
}

/////////////////////////////////////////////////////////////////////
/////// PluginCtx
/////////////////////////////////////////////////////////////////////

// PluginCtx is the execution context passed to a plugin hook's Fn.
// It provides read access to the current cycle's state and WaitFor*
// methods that block until the host reaches a given checkpoint.
//
// Config accessors (RootDir, BinaryOutputPath, User*Dir/Entry) are
// provided by the embedded *ConfigReader.
type PluginCtx struct {
	*UserConfig
	ss   *super_state
	hook *validated_lifecycle_hook
	ctx  context.Context

	gates [7]chan struct{}
}

func new_plugin_ctx(
	ss *super_state,
	hook *validated_lifecycle_hook,
) *PluginCtx {
	var gates [7]chan struct{}
	for i := 1; i <= 6; i++ {
		gates[i] = make(chan struct{})
	}
	return &PluginCtx{
		UserConfig: &UserConfig{validated_config: ss.cfg},
		ss:         ss,
		hook:       hook,
		gates:      gates,
	}
}

/////////////////////////////////////////////////////////////////////
/////// PluginCtx accessors
/////////////////////////////////////////////////////////////////////

func (p *PluginCtx) Context() context.Context { return p.ctx }

func (p *PluginCtx) Triggers() *set.Set[workset.Trigger] { return p.ss.cycle_triggers }

func (p *PluginCtx) EvtPaths() *set.Set[strict.CWDRelPath] { return p.ss.cycle_evt_paths }

func (p *PluginCtx) IsInitialBuild() bool { return p.ss.cycle_is_initial }

// VitePort returns the port the vite dev server is running on.
// Returns 0 when vite is not configured or in prod mode.
func (p *PluginCtx) VitePort() int { return p.ss.vite_port }

// BuildCycleID returns a unique ID for this build cycle. Useful
// for stale-reload rejection: pass it to the app reload request,
// and the app compares it against its current state to reject
// outdated reloads.
func (p *PluginCtx) BuildCycleID() string { return p.ss.build_cycle_id }

type PublicFileMapStatus string

const (
	PublicFileMapUnprocessed       PublicFileMapStatus = "unprocessed"
	PublicFileMapUserlandCommitted PublicFileMapStatus = "userland-committed"
	PublicFileMapPluginCommitted   PublicFileMapStatus = "plugin-committed"
	PublicFileMapWrittenToDisk     PublicFileMapStatus = "written-to-disk"
)

type PublicFileMapResult struct {
	Filemap map[string]string
	Status  PublicFileMapStatus
}

// ReadPublicFileMap returns the current in-memory public filemap
// and its status in the current build cycle. The filemap is nil
// when status is "unprocessed". The status progresses through:
// unprocessed → userland-committed → plugin-committed → written-to-disk.
func (p *PluginCtx) ReadPublicFileMap() PublicFileMapResult {
	p.ss.cycle_shared.mu.Lock()
	status := p.ss.cycle_shared.public_fm_status
	p.ss.cycle_shared.mu.Unlock()
	if status == PublicFileMapUnprocessed || p.ss.public_fm == nil {
		return PublicFileMapResult{Status: status}
	}
	return PublicFileMapResult{
		Filemap: p.ss.public_fm.Map(),
		Status:  status,
	}
}

// ContributePublicFiles adds files to the public filemap. Must be called by
// the end of checkpoint 2, otherwise the contributions will be rejected and
// an error will be returned. If the underlying hook spans multiple checkpoints
// and may not finish its contributions in time, use `BlockAtCheckpoint(2)` to
// hold the window open until ready.
func (p *PluginCtx) ContributePublicFiles(files map[string][]byte) error {
	p.ss.cycle_shared.mu.Lock()
	defer p.ss.cycle_shared.mu.Unlock()

	if !p.ss.cycle_shared.contributions_open {
		return fmt.Errorf(
			"plugin %q: ContributePublicFiles called too late -- ensure you call by the end of checkpoint 2",
			p.hook.plugin_name,
		)
	}

	if p.ss.cycle_shared.public_contributions == nil {
		p.ss.cycle_shared.public_contributions = make(map[string][]byte)
	}
	maps.Copy(p.ss.cycle_shared.public_contributions, files)
	return nil
}

// ViteProdBuild runs `vite build` synchronously using the vite
// config from the Wave config. The plugin controls where the output
// goes via opts. Panics if vite is not configured.
func (p *PluginCtx) ViteProdBuild(opts ViteProdBuildOpts) error {
	if p.ss.vite_build_cfg == nil {
		panic(fmt.Sprintf(
			"plugin %q: ViteProdBuild called but Vite is not configured",
			p.hook.plugin_name,
		))
	}
	return run_vite_prod_build(
		p.ss.vite_build_cfg, opts, p.ss.hook_env(), p.ss.logger,
	)
}

/////////////////////////////////////////////////////////////////////
/////// Checkpoint waiting and blocking
/////////////////////////////////////////////////////////////////////

func (p *PluginCtx) WaitForPostAssetPipeline() error { return p.wait_for(2) }
func (p *PluginCtx) WaitForGoCompile() error         { return p.wait_for(3) }
func (p *PluginCtx) WaitForPostCompile() error       { return p.wait_for(4) }
func (p *PluginCtx) WaitForPostBackendSettle() error { return p.wait_for(5) }
func (p *PluginCtx) WaitForCycleEnd() error          { return p.wait_for(6) }

func (p *PluginCtx) wait_for(checkpoint CheckpointOrd) error {
	if checkpoint <= p.hook.start_at {
		panic(fmt.Sprintf(
			"%s: WaitFor checkpoint %d, "+
				"but StartAt is checkpoint %d — a plugin cannot "+
				"wait for a checkpoint at or before its own start",
			p.hook.name,
			checkpoint,
			p.hook.start_at,
		))
	}
	select {
	case <-p.gates[checkpoint]:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// BlockAtCheckpoint registers a hold on the given checkpoint,
// preventing the build from advancing past it until the returned
// release function is called. Use this when a spanning hook (e.g.
// StartAt=2, FinishBy=3) needs to guarantee that its checkpoint-2
// work completes before the main goroutine proceeds past
// checkpoint 2.
//
// The checkpoint must be >= the hook's StartAt. Panics otherwise.
//
// The returned release function is safe to call multiple times;
// only the first call has any effect.
func (p *PluginCtx) BlockAtCheckpoint(checkpoint CheckpointOrd) func() {
	if checkpoint < p.hook.start_at {
		panic(fmt.Sprintf(
			"%s: BlockAtCheckpoint(%d), "+
				"but StartAt is checkpoint %d — cannot block a "+
				"checkpoint before the hook's own start",
			p.hook.name,
			checkpoint,
			p.hook.start_at,
		))
	}
	return p.ss.cycle_holds.hold(checkpoint)
}

// signal_checkpoint is called by the scheduler when a checkpoint is
// reached. Closing the channel unblocks all waiters idempotently.
func (p *PluginCtx) signal_checkpoint(checkpoint CheckpointOrd) {
	select {
	case <-p.gates[checkpoint]:
		// already closed
	default:
		close(p.gates[checkpoint])
	}
}

/////////////////////////////////////////////////////////////////////
/////// Checkpoint holds
/////////////////////////////////////////////////////////////////////

// checkpoint_holds tracks active holds that prevent the build from
// advancing past a checkpoint. Holds are registered by plugin hooks
// via BlockAtCheckpoint and awaited by the hook scheduler.
type checkpoint_holds struct {
	mu      sync.Mutex
	counts  map[CheckpointOrd]int
	waiters map[CheckpointOrd][]chan struct{}
}

func new_checkpoint_holds() *checkpoint_holds {
	return &checkpoint_holds{
		counts:  make(map[CheckpointOrd]int),
		waiters: make(map[CheckpointOrd][]chan struct{}),
	}
}

// hold increments the hold count for the given checkpoint and
// returns a release function. The release function is idempotent.
func (ch *checkpoint_holds) hold(checkpoint CheckpointOrd) func() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.counts[checkpoint]++
	var once sync.Once
	return func() {
		once.Do(func() {
			ch.mu.Lock()
			defer ch.mu.Unlock()
			ch.counts[checkpoint]--
			if ch.counts[checkpoint] == 0 {
				for _, w := range ch.waiters[checkpoint] {
					close(w)
				}
				delete(ch.waiters, checkpoint)
			}
		})
	}
}

// wait blocks until all holds on the given checkpoint are released.
// Returns immediately if there are no active holds.
func (ch *checkpoint_holds) wait(checkpoint CheckpointOrd) {
	ch.mu.Lock()
	if ch.counts[checkpoint] == 0 {
		ch.mu.Unlock()
		return
	}
	w := make(chan struct{})
	ch.waiters[checkpoint] = append(ch.waiters[checkpoint], w)
	ch.mu.Unlock()
	<-w
}

/////////////////////////////////////////////////////////////////////
/////// plugin_shared_state
/////////////////////////////////////////////////////////////////////

// plugin_shared_state holds mutable state shared across all plugin
// hook instances within a single build cycle.
type plugin_shared_state struct {
	mu                   sync.Mutex
	public_contributions map[string][]byte
	contributions_open   bool                // set to false after checkpoint 2 await
	public_fm_status     PublicFileMapStatus // progresses through the build cycle
}
