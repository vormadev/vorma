package wavebuild

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/lockfile"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/constants"
	"github.com/vormadev/vorma/wave/internal/cssbundle"
	"github.com/vormadev/vorma/wave/internal/fswatcher"
	"github.com/vormadev/vorma/wave/internal/staticproc"
	"golang.org/x/sync/errgroup"
)

/////////////////////////////////////////////////////////////////////
/////// Build step (table-driven build orchestration)
/////////////////////////////////////////////////////////////////////

// build_step pairs a checkpoint with an optional phase function
// that runs after the checkpoint's hooks have been awaited.
// A nil phase means no wave-owned work at that checkpoint.
// Phase functions return (aborted, error).
type build_step struct {
	checkpoint CheckpointOrd
	phase      func() (bool, error)
}

/////////////////////////////////////////////////////////////////////
/////// super_state
/////////////////////////////////////////////////////////////////////

type super_state struct {
	// config
	cfg_path strict.CWDRelPath
	cfg      *validated_config
	is_dev   bool

	// build state
	private_fm *staticproc.Filemap
	public_fm  *staticproc.Filemap
	// durable plugin public file store, survives across cycles
	plugin_public_files       map[string][]byte
	critical_css_patterns     *set.Set[strict.CWDRelPath]
	non_critical_css_patterns *set.Set[strict.CWDRelPath]
	// cached by most recent build to avoid fs read, used by browser settle
	critical_css_bytes []byte
	watcher            *fswatcher.Watcher
	logger             *slog.Logger

	// plugin
	plugins     []*Plugin
	plugin_cfgs []*validated_plugin_config

	all_hooks []validated_lifecycle_hook

	// process supervision (dev only)
	dev_port          int
	supervisor        *supervisor
	pending_stop_done chan struct{} // tracks in-flight supervisor stop from previous aborted cycle
	vite_port         int
	vite_sup          *vite_supervisor
	vite_build_cfg    *vite_build_config // shared between dev supervisor and prod builds
	refresh_port      int
	refresh_token     string
	browser           *browser_sync
	lock              *lockfile.PIDLock // project lock, released on shutdown

	// watch loop state (guarded by mu)
	mu                   sync.Mutex
	pending_triggers     *set.Set[trigger]
	pending_evt_paths    *set.Set[strict.CWDRelPath]
	pending_hook_indices *set.Set[int]
	is_building          bool
	build_cycle_id       string

	// per-cycle state (set at top of run_build, valid for its duration)
	cycle_triggers   *set.Set[trigger]
	cycle_evt_paths  *set.Set[strict.CWDRelPath]
	cycle_is_initial bool
	cycle_shared     *plugin_shared_state
	cycle_holds      *checkpoint_holds
}

func (s *super_state) new_private_phantom_filemap() *staticproc.Filemap {
	sp := &staticproc.StaticProcessor{
		OutDir: s.cfg_path.Dir().Join(
			constants.DIST_DIRNAME,
			constants.STATIC_ASSETS_PRIVATE_DIR,
		),
	}
	return sp.PhantomFilemap()
}

func (s *super_state) new_public_phantom_filemap() *staticproc.Filemap {
	sp := &staticproc.StaticProcessor{
		OutDir: s.cfg_path.Dir().Join(
			constants.DIST_DIRNAME,
			constants.STATIC_ASSETS_PUBLIC_DIR,
		),
		OutFilePrefix: constants.PUBLIC_STATIC_FILE_PREFIX,
	}
	return sp.PhantomFilemap()
}

func (s *super_state) parse_cfg() (*validated_config, *vite_build_config, error) {
	s.logger.Info("parsing wave config")
	cfg, err := config_path_to_validated_config(s.cfg_path)
	if err != nil {
		return nil, nil, fmt.Errorf("wave config parse failed: %w", err)
	}

	// update vite build config from the (possibly changed) config
	if !cfg.core.ServerOnlyMode && cfg.vite.UsingVite {
		vite_cfg_file := ""
		if cfg.vite.ViteConfigFile != nil {
			vite_cfg_file = cfg.vite.ViteConfigFile.Str()
		}
		vite_build_cfg := new_vite_build_config(
			cfg.vite.JSPackageManagerBaseCmd,
			cfg.vite.JSPackageManagerCmdDir.Str(),
			vite_cfg_file,
		)
		return cfg, vite_build_cfg, nil
	}

	return cfg, nil, nil
}

func (s *super_state) write_runtime_cfg() error {
	json_pretty_bytes, err := jsonutil.SerializePretty(wave.RuntimeConfig{
		PublicPathPrefix:      s.cfg.core.PublicPathPrefix,
		IsUsingCriticalCSS:    s.cfg.using_critical_css(),
		IsUsingNonCriticalCSS: s.cfg.using_non_critical_css(),
	})
	if err != nil {
		return fmt.Errorf("failed to serialize runtime config: %w", err)
	}
	runtime_cfg_path := s.cfg_path.Dir().Join(
		constants.DIST_DIRNAME,
		constants.STATIC_INTERNAL_DIR,
		constants.RUNTIME_CFG_JSON_FILENAME,
	)
	if err := os.WriteFile(
		runtime_cfg_path.Str(),
		append(json_pretty_bytes, '\n'),
		0644,
	); err != nil {
		return fmt.Errorf("failed to write runtime config: %w", err)
	}
	return nil
}

// reload_config orchestrates a full config reload: parse and
// validate the user config, call each plugin's Config.Parse (if
// any), then re-validate the plugins' hooks against the
// new root_dir.
func (s *super_state) reload_config() error {
	old_root := strict.CWDRelPath("")
	if s.cfg != nil {
		old_root = s.cfg.root_dir
	}

	cfg, vite_build_cfg, err := s.parse_cfg()
	if err != nil {
		return err
	}

	if err := s.run_plugin_config_parse(cfg); err != nil {
		return err
	}

	plugin_hooks, err := s.validate_plugin_hooks_for_cfg(cfg)
	if err != nil {
		return err
	}

	all_hooks := slices.Clone(cfg.lifecycle_hooks)
	all_hooks = append(all_hooks, plugin_hooks...)

	s.cfg = cfg
	s.vite_build_cfg = vite_build_cfg
	s.all_hooks = all_hooks

	if !s.cfg.using_private_static() {
		s.private_fm = s.new_private_phantom_filemap()
	}
	if !s.cfg.using_public_static() {
		s.public_fm = s.new_public_phantom_filemap()
	}
	if !s.cfg.using_critical_css() {
		s.critical_css_patterns = nil
		s.critical_css_bytes = nil
	}
	if !s.cfg.using_non_critical_css() {
		s.non_critical_css_patterns = nil
	}

	if s.watcher != nil && s.cfg.root_dir != old_root {
		s.watcher.SetWatchRoot(s.cfg.root_dir)
	}
	return nil
}

// run_plugin_config_parse extracts each plugin's config section from
// the raw config JSON and calls its Parse function with a
// ConfigReader that provides read access to the validated user config.
func (s *super_state) run_plugin_config_parse(cfg *validated_config) error {
	if len(s.plugin_cfgs) == 0 {
		return nil
	}
	if len(cfg.raw_file_json) == 0 {
		return nil
	}
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(cfg.raw_file_json, &sections); err != nil {
		return fmt.Errorf("failed to parse config sections: %w", err)
	}
	for _, plugin_cfg := range s.plugin_cfgs {
		if plugin_cfg == nil || plugin_cfg.json_key == "" ||
			plugin_cfg.parse_fn == nil {
			continue
		}
		section, exists := sections[plugin_cfg.json_key]
		var raw_json *json.RawMessage
		if exists {
			raw_json = &section
		}
		parse_fn_ctx := &PluginConfigParseCtx{
			RawJSON:    raw_json,
			UserConfig: &UserConfig{validated_config: cfg},
		}
		if err := plugin_cfg.parse_fn(parse_fn_ctx); err != nil {
			return fmt.Errorf(
				"config section %q: %w", plugin_cfg.json_key, err,
			)
		}
	}
	return nil
}

// Re-validates all plugin hooks against the given root_dir.
// Called on every config reload so that watch patterns are resolved
// against the (possibly changed) root_dir.
func (s *super_state) validate_plugin_hooks_for_cfg(
	cfg *validated_config,
) ([]validated_lifecycle_hook, error) {
	if len(s.plugins) == 0 {
		return nil, nil
	}
	plugin_hooks := make(
		[]validated_lifecycle_hook,
		0,
		len(s.plugins),
	)
	for _, plugin := range s.plugins {
		hooks, err := validate_plugin_hooks(
			strings.TrimSpace(plugin.Name),
			plugin.LifecycleHooks,
			cfg.root_dir,
		)
		if err != nil {
			return nil, fmt.Errorf("plugin hook validation failed: %w", err)
		}
		plugin_hooks = append(plugin_hooks, hooks...)
	}
	return plugin_hooks, nil
}

func (s *super_state) new_derive_workset_opts(
	triggers *set.Set[trigger],
	no_frontend_settling bool,
) derive_workset_opts {
	return derive_workset_opts{
		triggers:             triggers,
		has_private_static:   s.cfg.using_private_static(),
		has_public_static:    s.cfg.using_public_static(),
		has_critical_css:     s.cfg.using_critical_css(),
		has_non_critical_css: s.cfg.using_non_critical_css(),
		no_frontend_settling: no_frontend_settling,
	}
}

func (s *super_state) log_build_err(err error) {
	s.logger.Warn(fmt.Sprintf(
		"Build error — %s — edit the offending watched files to retry",
		err,
	))
	if s.browser != nil {
		s.browser.show_build_error(err.Error())
	}
}

/////////////////////////////////////////////////////////////////////
/////// Binary output path, PID file paths & hook environment
/////////////////////////////////////////////////////////////////////

func (s *super_state) app_pid_file_path() string {
	return s.cfg_path.Dir().Join(
		constants.DIST_DIRNAME,
		constants.DEV_DIRNAME,
		constants.APP_PID_FILENAME,
	).Str()
}

func (s *super_state) vite_pid_file_path() string {
	return s.cfg_path.Dir().Join(
		constants.DIST_DIRNAME,
		constants.DEV_DIRNAME,
		constants.VITE_PID_FILENAME,
	).Str()
}

// hook_env returns the environment variables injected into shell
// hook commands and plugin contexts. Contains buildtime vars
// (binary output path, root dir) plus the dev/prod mode flag.
// Recomputed each build cycle so config changes are picked up.
func (s *super_state) hook_env() []string {
	env := []string{
		constants.ENV_KEY_BUILDTIME_BIN_OUTPUT_PATH + "=" + s.cfg.core.binary_output_path_abs.Str(),
		constants.ENV_KEY_BUILDTIME_ROOT_DIR + "=" + s.cfg.root_dir_abs.
			Str(),
	}
	if s.is_dev {
		env = append(env, constants.ENV_KEY_DEV_BUILDTIME_IS_DEV+"=1")
		env = append(env, constants.ENV_KEY_BUILDTIME_BUILD_TAGS+"=dev")
	} else {
		env = append(env, constants.ENV_KEY_DEV_BUILDTIME_IS_DEV+"=0")
		env = append(env, constants.ENV_KEY_BUILDTIME_BUILD_TAGS+"=prod")
	}
	return env
}

// app_env returns the environment variables injected into the
// supervised app server process in dev mode. Contains only the
// runtime vars the app needs (port, mode, vite port, refresh port).
func (s *super_state) app_env() []string {
	static_dir, _ := filepath.Abs(
		s.cfg_path.Dir().
			Join(constants.DIST_DIRNAME, constants.STATIC_DIRNAME).
			Str(),
	)
	env := []string{
		fmt.Sprintf("%s=%d", constants.ENV_KEY_RUNTIME_PORT, s.dev_port),
		constants.ENV_KEY_DEV_RUNTIME_IS_DEV + "=1",
		constants.ENV_KEY_DEV_RUNTIME_STATIC_DIR + "=" + static_dir,
		constants.ENV_KEY_DEV_RUNTIME_VITE_PORT + "=" + fmt.Sprintf(
			"%d",
			s.vite_port,
		),
	}
	if s.refresh_port > 0 {
		env = append(
			env,
			fmt.Sprintf(
				"%s=%d",
				constants.ENV_KEY_DEV_RUNTIME_REFRESH_PORT,
				s.refresh_port,
			),
		)
		env = append(
			env,
			fmt.Sprintf(
				"%s=%s",
				constants.ENV_KEY_DEV_RUNTIME_REFRESH_TOKEN,
				s.refresh_token,
			),
		)
	}
	return env
}

/////////////////////////////////////////////////////////////////////
/////// Hook helpers (unified)
/////////////////////////////////////////////////////////////////////

// Returns every validated hook index that applies to the current
// dev/prod mode and build context. When is_initial is true, hooks
// marked incremental_only are excluded.
func (s *super_state) all_applicable_hook_indices(
	is_initial bool,
) *set.Set[int] {
	indices := &set.Set[int]{}
	for i, h := range s.all_hooks {
		if h.dev_only && !s.is_dev {
			continue
		}
		if h.prod_only && s.is_dev {
			continue
		}
		if h.incremental_only && is_initial {
			continue
		}
		indices.Add(i)
	}
	return indices
}

// Returns the indices of validated hooks whose watch patterns match
// the given event, per-hook excludes, and dev/prod mode. Assumes global
// excludes have been pre-filtered.
func (s *super_state) classify_hooks_for_evt(
	evt fswatcher.Evt,
) ([]int, error) {
	var matched []int
	for i, h := range s.all_hooks {
		if h.dev_only && !s.is_dev {
			continue
		}
		if h.prod_only && s.is_dev {
			continue
		}

		excluded := false
		for _, pattern := range h.watch_exclude_patterns {
			m, err := path_match(pattern, evt.Path)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid hook exclude pattern (%s): %w",
					h.name, err,
				)
			}
			if m {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		for _, pattern := range h.watch_include_patterns {
			m, err := path_match(pattern, evt.Path)
			if err != nil {
				return nil, fmt.Errorf(
					"invalid hook include pattern (%s): %w",
					h.name, err,
				)
			}
			if m {
				matched = append(matched, i)
				break
			}
		}
	}
	return matched, nil
}

// Converts triggered hook indices into the scheduled_hook slice consumed by
// hook_scheduler. For plugin hooks a PluginCtx is created (with a nil ctx
// that the scheduler fills in with its own cancellation context).
func (s *super_state) build_scheduled_hooks(
	indices *set.Set[int],
	hooks []validated_lifecycle_hook,
) []scheduled_hook {
	if indices == nil || indices.Len() == 0 {
		return nil
	}
	var result []scheduled_hook
	for i, vh := range hooks {
		if !indices.Has(i) {
			continue
		}
		sh := scheduled_hook{
			name:      vh.name,
			start_at:  vh.start_at,
			finish_by: vh.finish_by,
			effects:   vh.effects,
			cmd:       vh.cmd,
			fn:        vh.fn,
		}
		if vh.fn != nil {
			sh.plugin_ctx = new_plugin_ctx(s, &hooks[i])
		}
		result = append(result, sh)
	}
	return result
}

/////////////////////////////////////////////////////////////////////
/////// Event classification
/////////////////////////////////////////////////////////////////////

type classify_result struct {
	trigger trigger
	matched bool
}

// Determines which core trigger (if any) a filesystem event corresponds to.
// When ServerOnlyMode is true, only config file changes produce triggers and
// frontend file changes are ignored. Assumes global excludes have been
// pre-filtered.
func (s *super_state) classify_evt(evt fswatcher.Evt) (classify_result, error) {
	// private static
	if s.cfg.using_private_static() {
		priv_match, err := path_match(s.cfg.private_dir_pattern(), evt.Path)
		if err != nil {
			return classify_result{}, fmt.Errorf(
				"invalid private pattern: %w", err,
			)
		}
		if priv_match {
			return classify_result{
				trigger: private_static_src_changed, matched: true,
			}, nil
		}
	}

	// public static
	if s.cfg.using_public_static() {
		pub_match, err := path_match(s.cfg.public_dir_pattern(), evt.Path)
		if err != nil {
			return classify_result{}, fmt.Errorf(
				"invalid public pattern: %w", err,
			)
		}
		if pub_match {
			return classify_result{
				trigger: public_static_src_changed, matched: true,
			}, nil
		}
	}

	// critical css
	has_css_ext := extension(evt.Path) == ".css"
	if has_css_ext &&
		s.critical_css_patterns != nil &&
		s.critical_css_patterns.Has(evt.Path) {
		return classify_result{
			trigger: critical_css_src_changed, matched: true,
		}, nil
	}

	// non-critical css
	if has_css_ext &&
		s.non_critical_css_patterns != nil &&
		s.non_critical_css_patterns.Has(evt.Path) {
		return classify_result{
			trigger: non_critical_css_src_changed, matched: true,
		}, nil
	}

	return classify_result{matched: false}, nil
}

/////////////////////////////////////////////////////////////////////
/////// Build phases
/////////////////////////////////////////////////////////////////////

func (s *super_state) get_private_filemap(
	evt_paths *set.Set[strict.CWDRelPath],
) error {
	if !s.cfg.using_private_static() {
		return nil
	}
	s.logger.Debug("Getting private filemap")
	var err error
	sp := &staticproc.StaticProcessor{
		SrcDir: s.cfg.core.StaticAssetDirs.Private,
		OutDir: s.cfg_path.Dir().Join(
			constants.DIST_DIRNAME,
			constants.STATIC_ASSETS_PRIVATE_DIR,
		),
	}
	s.private_fm, err = sp.PhysicalFilemap(s.private_fm, evt_paths)
	if err != nil {
		return fmt.Errorf("getting private filemap failed: %w", err)
	}
	return nil
}

func (s *super_state) get_public_filemap(
	evt_paths *set.Set[strict.CWDRelPath],
) error {
	if !s.cfg.using_public_static() {
		return nil
	}
	s.logger.Debug("Getting public filemap")
	var err error
	sp := &staticproc.StaticProcessor{
		SrcDir:              s.cfg.core.StaticAssetDirs.Public,
		PassthroughDirnames: []string{constants.PUBLIC_STATIC_EXCLUDE_DIR},
		OutDir: s.cfg_path.Dir().Join(
			constants.DIST_DIRNAME,
			constants.STATIC_ASSETS_PUBLIC_DIR,
		),
		OutFilePrefix: constants.PUBLIC_STATIC_FILE_PREFIX,
	}
	s.public_fm, err = sp.PhysicalFilemap(s.public_fm, evt_paths)
	if err != nil {
		return fmt.Errorf("getting public filemap failed: %w", err)
	}
	return nil
}

func (s *super_state) build_critical_css() error {
	if !s.cfg.using_critical_css() {
		return nil
	}
	s.logger.Debug("Building critical CSS")
	css_bundle_out, err := cssbundle.Bundle(
		s.cfg.core.CSSEntryFiles.Critical,
		s.public_fm.Map(),
		s.cfg.core.PublicPathPrefix,
	)
	if err != nil {
		return fmt.Errorf("critical css bundle failed: %w", err)
	}
	s.critical_css_patterns = set.New(css_bundle_out.Imports)

	// cache for browser settle so we don't re-read from disk
	s.critical_css_bytes = []byte(css_bundle_out.CSS)

	critical_css_out := s.cfg_path.Dir().Join(
		constants.DIST_DIRNAME, constants.STATIC_INTERNAL_DIR, constants.CRITICAL_CSS_FILENAME,
	)
	if err := os.WriteFile(critical_css_out.Str(), s.critical_css_bytes, 0644); err != nil {
		return fmt.Errorf("failed to write critical css: %w", err)
	}
	return nil
}

func (s *super_state) build_non_critical_css() error {
	if !s.cfg.using_non_critical_css() {
		return nil
	}
	s.logger.Debug("Building non-critical CSS")
	css_bundle_out, err := cssbundle.Bundle(
		s.cfg.core.CSSEntryFiles.NonCritical,
		s.public_fm.Map(),
		s.cfg.core.PublicPathPrefix,
	)
	if err != nil {
		return fmt.Errorf("non-critical css bundle failed: %w", err)
	}
	s.non_critical_css_patterns = set.New(css_bundle_out.Imports)
	s.public_fm.Set(map[string][]byte{
		constants.NON_CRITICAL_CSS_FILENAME: []byte(css_bundle_out.CSS),
	})
	return nil
}

func (s *super_state) apply_diff(
	fm *staticproc.Filemap, json_path strict.CWDRelPath,
) error {
	s.logger.Debug(fmt.Sprintf(
		"Applying static diff (%s)",
		filepath.Base(fm.OutDir().Str()),
	))
	return fm.ApplyDiffAndWriteFilemapJSON(json_path)
}

/////////////////////////////////////////////////////////////////////
/////// Build orchestration
/////////////////////////////////////////////////////////////////////

type build_params struct {
	triggers     *set.Set[trigger]
	evt_paths    *set.Set[strict.CWDRelPath]
	hook_indices *set.Set[int]
	should_abort func() bool
	is_initial   bool
}

func (s *super_state) wipe_internal_dir() error {
	internal_dir := s.cfg_path.Dir().Join(
		constants.DIST_DIRNAME, constants.STATIC_INTERNAL_DIR,
	)
	if err := os.RemoveAll(internal_dir.Str()); err != nil {
		return fmt.Errorf("failed to clear internal dir: %w", err)
	}
	if err := fsutil.EnsureDir(internal_dir.Str()); err != nil {
		return fmt.Errorf("failed to create internal dir: %w", err)
	}
	return nil
}

// run_build executes one build cycle. Returns (completed, error).
// completed=false means aborted early because new events are
// pending — the caller should merge state back and retry.
//
// The build is driven by a table of checkpoint/phase pairs:
//
//	Checkpoint 1 (cycle_start) → Phase A: public asset pipeline (wave-owned)
//	Checkpoint 2 (userland_public_filemap_ready) → Phase B: finalize public filemap (merge plugin contributions, apply diffs, write filemap JSON)
//	Checkpoint 3 (full_public_filemap_finalized) → Phase C: private asset pipeline (wave-owned)
//	Checkpoint 4 (go_compile) → Phase D: no-op
//	Checkpoint 5 (go_compile_complete) → Phase E: backend settling (app restart)
//	Checkpoint 6 (service_restarted) → Phase F: frontend settling (browser signals)
//	Checkpoint 7 (cycle_end) → (no phase — cycle end)
//
// At each step the loop: starts all hooks with start_at=N, checks
// for abort, awaits all hooks with finish_by=N, merges their
// effects, then runs the phase. Hooks span checkpoints for
// concurrency: start_at=1, finish_by=4 runs concurrently across
// the public asset pipeline, checkpoints 2 and 3, and compilation.
func (s *super_state) run_build(p build_params) (bool, error) {
	var wave_processing_duration time.Duration

	type_log := "incremental"
	if p.is_initial {
		type_log = "initial"
	}
	triggers_log := ""
	if p.triggers == nil || p.triggers.Len() == 0 {
		triggers_log = "(none)"
	} else {
		var names []string
		for t := range p.triggers.Range() {
			names = append(names, string(t))
		}
		triggers_log = strings.Join(names, ", ")
	}
	s.logger.Info("starting build process",
		"type", type_log,
		"triggers", triggers_log,
	)

	// set new cycle id
	cycle_id, err := id.New(12)
	if err != nil {
		return false, fmt.Errorf("failed to generate build cycle id: %w", err)
	}
	s.build_cycle_id = cycle_id

	// set per-cycle state
	s.cycle_triggers = p.triggers
	s.cycle_evt_paths = p.evt_paths
	s.cycle_is_initial = p.is_initial
	s.cycle_holds = new_checkpoint_holds()
	s.cycle_shared = &plugin_shared_state{
		contributions_open: true,
		public_fm_status:   PublicFileMapUnprocessed,
	}

	cycle_hooks := slices.Clone(s.all_hooks)

	has_matched_no_frontend_settling_hook := false
	if p.hook_indices != nil {
		for i, hook := range cycle_hooks {
			if !p.hook_indices.Has(i) {
				continue
			}
			if slices.Contains(hook.effects, EffectNoFrontendSettling) {
				has_matched_no_frontend_settling_hook = true
			}
			if has_matched_no_frontend_settling_hook {
				break
			}
		}
	}

	// ensure public_fm is always initialized
	if s.public_fm == nil {
		s.public_fm = s.new_public_phantom_filemap()
	}

	// compute effects from triggers
	fx := derive_workset(
		s.new_derive_workset_opts(
			p.triggers,
			has_matched_no_frontend_settling_hook,
		),
	)
	force_private_diff := p.triggers.Has(config_changed) &&
		!s.cfg.using_private_static()
	force_public_diff := p.triggers.Has(config_changed) &&
		!s.cfg.using_public_static()

	// initial dev builds always start the server
	if p.is_initial && s.is_dev {
		fx.Add(EffectRestartApp)
	}

	// compute hook env once for this cycle — picks up any config changes
	env := s.hook_env()

	// set up the hook scheduler with all triggered hooks
	hooks := s.build_scheduled_hooks(p.hook_indices, cycle_hooks)

	for _, h := range hooks {
		for _, effect := range h.effects {
			switch effect {
			case EffectRestartApp,
				EffectHardReloadBrowser,
				EffectShowFrontendRebuildingOverlay:
				fx.Add(EffectShowFrontendRebuildingOverlay)
			}
		}
	}

	scheduler := new_hook_scheduler(
		hooks,
		env,
		s.cfg.root_dir,
		s.cycle_holds,
		s.logger,
	)
	defer scheduler.abort()

	var stop_done chan struct{}
	stop_started := false
	ensure_stop_started := func() {
		if stop_started || !s.is_dev ||
			!fx.Has(EffectRestartApp) ||
			s.supervisor == nil {
			return
		}

		// Wait for any in-flight stop from a previous aborted cycle
		if s.pending_stop_done != nil {
			<-s.pending_stop_done
		}
		stop_done = make(chan struct{})
		s.pending_stop_done = stop_done
		go func() {
			s.supervisor.stop()
			close(stop_done)
		}()
		stop_started = true
	}
	ensure_stop_started()

	if s.is_dev && !p.is_initial && s.browser != nil &&
		fx.Has(EffectShowFrontendRebuildingOverlay) {
		s.browser.send_rebuilding()
	}

	// Phase closures capture fx and p from the enclosing scope.
	// Each returns (aborted, error).

	phase_public_asset_pipeline := func() (bool, error) {
		phase_start := time.Now()

		// Nuke internal dir to clear stale artifacts from
		// features that may have been disabled (e.g. critical
		// CSS removed from config). The directory is re-created
		// immediately and repopulated during the build phases.
		if p.triggers.Has(config_changed) {
			if err := s.wipe_internal_dir(); err != nil {
				return false, fmt.Errorf(
					"failed to clear internal dir: %w",
					err,
				)
			}
		}
		if err := s.write_runtime_cfg(); err != nil {
			return false, fmt.Errorf("failed to write runtime config: %w", err)
		}

		// get public filemap
		if fx.Has(effect_build_public_filemap) {
			if err := s.get_public_filemap(p.evt_paths); err != nil {
				return false, err
			}
		}

		// css bundles in parallel
		var g2 errgroup.Group
		if fx.Has(effect_build_critical_css) {
			g2.Go(s.build_critical_css)
		}
		if fx.Has(effect_build_non_critical_css) {
			g2.Go(s.build_non_critical_css)
		}
		if err := g2.Wait(); err != nil {
			return false, err
		}

		// mark public filemap as userland-committed — phase A is done,
		// plugins can now read the in-memory filemap at checkpoint 2
		s.cycle_shared.mu.Lock()
		s.cycle_shared.public_fm_status = PublicFileMapUserlandCommitted
		s.cycle_shared.mu.Unlock()

		wave_processing_duration += time.Since(phase_start)

		return false, nil
	}

	phase_finalize_public := func() (bool, error) {
		phase_start := time.Now()

		// Close the contribution window — any plugin calling
		// SetPublicFiles/DeletePublicFiles after this point will
		// receive an error.
		s.cycle_shared.mu.Lock()
		s.cycle_shared.contributions_open = false
		plugin_files_modified := s.cycle_shared.plugin_files_modified
		s.cycle_shared.mu.Unlock()

		// Re-merge durable plugin files into the public filemap
		// when the physical filemap was rebuilt (which replaces
		// s.public_fm entirely) or when a plugin modified the
		// durable store this cycle.
		filemap_rebuilt := fx.Has(effect_build_public_filemap)
		has_plugin_files := len(s.plugin_public_files) > 0
		if has_plugin_files && (filemap_rebuilt || plugin_files_modified) {
			s.public_fm.Set(s.plugin_public_files)

			s.cycle_shared.mu.Lock()
			s.cycle_shared.public_fm_status = PublicFileMapPluginCommitted
			s.cycle_shared.mu.Unlock()
		}

		// generate client-side filemap JSON for browser consumption
		needs_public_write := force_public_diff ||
			plugin_files_modified || fx.HasAny(
			effect_build_public_filemap,
			effect_build_critical_css,
			effect_build_non_critical_css,
		)

		if needs_public_write {
			client_map := make(map[string]string)
			for k, v := range s.public_fm.Map() {
				client_map[k] = path.Join(s.cfg.core.PublicPathPrefix, v)
			}
			client_json, err := jsonutil.SerializePretty(client_map)
			if err != nil {
				return false, fmt.Errorf(
					"failed to generate public filemap JSON: %w", err,
				)
			}
			s.public_fm.Set(map[string][]byte{
				constants.PUBLIC_FILEMAP_FILENAME: client_json,
			})

			// apply public diffs and write filemap JSON (includes wave-owned
			// assets, CSS, and plugin contributions)
			if err := s.apply_diff(
				s.public_fm,
				s.cfg_path.Dir().Join(constants.DIST_DIRNAME, constants.STATIC_INTERNAL_DIR, constants.PUBLIC_FILEMAP_JSON_FILENAME),
			); err != nil {
				return false, err
			}

			s.cycle_shared.mu.Lock()
			s.cycle_shared.public_fm_status = PublicFileMapWrittenToDisk
			s.cycle_shared.mu.Unlock()
		}

		wave_processing_duration += time.Since(phase_start)

		return false, nil
	}

	phase_private_asset_pipeline := func() (bool, error) {
		phase_start := time.Now()

		if fx.Has(effect_build_private_filemap) {
			if err := s.get_private_filemap(p.evt_paths); err != nil {
				return false, err
			}
		}

		if (fx.Has(effect_build_private_filemap) || force_private_diff) &&
			s.private_fm != nil {
			if err := s.apply_diff(
				s.private_fm,
				s.cfg_path.Dir().Join(constants.DIST_DIRNAME, constants.STATIC_INTERNAL_DIR, constants.PRIVATE_FILEMAP_JSON_FILENAME),
			); err != nil {
				return false, err
			}
		}

		wave_processing_duration += time.Since(phase_start)

		s.logger.Info(
			"completed wave static processing",
			"duration",
			wave_processing_duration.Round(time.Millisecond),
		)

		return false, nil
	}

	phase_backend_settle := func() (bool, error) {
		bin_out := s.cfg.core.binary_output_path_abs
		if s.is_dev && fx.Has(EffectRestartApp) {
			if stop_done != nil {
				<-stop_done
				s.pending_stop_done = nil
			}
			s.supervisor.update(
				bin_out,
				s.cfg.core.HealthcheckEndpoint,
				s.app_env(),
			)
			if p.should_abort() {
				return true, nil
			}
			if _, err := os.Stat(bin_out.Str()); err != nil {
				return false, fmt.Errorf("binary not found at %s", bin_out)
			}
			if err := s.supervisor.start(); err != nil {
				return false, fmt.Errorf(
					"app server start failed: %w", err,
				)
			}
		}
		return false, nil
	}

	phase_frontend_settle := func() (bool, error) {
		if has_matched_no_frontend_settling_hook ||
			fx.Has(EffectNoFrontendSettling) {
			return false, nil
		}
		if s.cfg.core.ServerOnlyMode {
			return false, nil
		}
		if s.is_dev && !p.is_initial && s.browser != nil {
			needs_settle := fx.HasAny(
				EffectHardReloadBrowser,
				EffectRevalidateClientData,
				effect_build_critical_css,
				effect_build_non_critical_css,
			)

			if needs_settle {
				if p.should_abort() {
					return true, nil
				}
				non_critical_css_url := ""
				if fx.Has(effect_build_non_critical_css) {
					non_critical_css_url = path.Join(
						s.cfg.core.PublicPathPrefix,
						s.public_fm.Map()[constants.NON_CRITICAL_CSS_FILENAME],
					)
				}
				s.browser.settle(
					fx,
					s.critical_css_bytes,
					non_critical_css_url,
				)
			}
		}
		return false, nil
	}

	// The build step table. Each entry pairs a checkpoint with an
	// optional phase that runs after the checkpoint's hooks complete.
	steps := []build_step{
		{1, phase_public_asset_pipeline},
		{2, phase_finalize_public},
		{3, phase_private_asset_pipeline},
		{4, nil},
		{5, phase_backend_settle},
		{6, phase_frontend_settle},
		{7, nil},
	}

	for _, step := range steps {
		scheduler.start_hooks_at(step.checkpoint)

		if p.should_abort() {
			return false, nil
		}

		hook_fx, err := scheduler.await_checkpoint(step.checkpoint)
		if err != nil {
			if p.should_abort() {
				s.logger.Warn("Hook error (superseded by new events)",
					"error", err,
				)
				return false, nil
			}
			return true, err
		}
		for e := range hook_fx.Range() {
			fx.Add(e)
		}
		ensure_stop_started()

		if step.phase != nil {
			aborted, err := step.phase()
			if aborted {
				return false, nil
			}
			if err != nil {
				return true, err
			}
		}
	}

	if s.is_dev && !p.is_initial && s.browser != nil {
		s.browser.hide_rebuilding()
	}

	return true, nil
}

/////////////////////////////////////////////////////////////////////
/////// Watch loop
/////////////////////////////////////////////////////////////////////

func (s *super_state) has_pending() bool {
	return s.pending_triggers != nil || s.pending_hook_indices != nil
}

func (s *super_state) should_abort() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.has_pending()
}

// drain_and_build runs build cycles in a loop until there are no
// more pending events. If a build is aborted mid-cycle (because new
// events arrived), the original needs are merged back into pending
// state and retried on the next iteration.
func (s *super_state) drain_and_build() {
	log_err_and_unlock := func(err error) {
		s.log_build_err(err)
		s.mu.Lock()
		s.is_building = false
		s.mu.Unlock()
	}

	for {
		s.mu.Lock()
		if !s.has_pending() {
			s.is_building = false
			s.mu.Unlock()
			return
		}
		triggers := s.pending_triggers
		evt_paths := s.pending_evt_paths
		hook_indices := s.pending_hook_indices
		s.pending_triggers = nil
		s.pending_evt_paths = nil
		s.pending_hook_indices = nil
		s.mu.Unlock()

		// full rebuild: reload config, discard incremental paths,
		// and run all hooks
		if triggers != nil && triggers.Has(config_changed) {
			if err := s.reload_config(); err != nil {
				log_err_and_unlock(err)
				return
			}
			evt_paths = nil
			hook_indices = s.all_applicable_hook_indices(false)
		}

		// ensure non-nil triggers for derive
		if triggers == nil {
			triggers = &set.Set[trigger]{}
		}

		completed, err := s.run_build(build_params{
			triggers:     triggers,
			evt_paths:    evt_paths,
			hook_indices: hook_indices,
			should_abort: s.should_abort,
			is_initial:   false,
		})
		if err != nil {
			log_err_and_unlock(err)
			return
		}
		if !completed {
			// aborted — merge original needs back so expensive steps
			// are retried on the next iteration
			s.mu.Lock()
			if triggers.Len() > 0 {
				if s.pending_triggers == nil {
					s.pending_triggers = triggers
				} else {
					s.pending_triggers.Union(triggers)
				}
			}
			if evt_paths != nil {
				if s.pending_evt_paths == nil {
					s.pending_evt_paths = evt_paths
				} else {
					s.pending_evt_paths.Union(evt_paths)
				}
			}
			if hook_indices != nil {
				if s.pending_hook_indices == nil {
					s.pending_hook_indices = hook_indices
				} else {
					s.pending_hook_indices.Union(hook_indices)
				}
			}
			s.mu.Unlock()
		}
	}
}

// handle_watch_evts is the callback invoked by the file watcher
// when filesystem events arrive. It classifies events into triggers
// and hook indices, merges them into the pending state, and starts
// a drain_and_build goroutine if one is not already running.
func (s *super_state) handle_watch_evts(evts []fswatcher.Evt) error {
	// fast path: if a ConfigChanged full rebuild is already pending,
	// skip classification — just ensure drain_and_build is running
	s.mu.Lock()
	if s.pending_triggers != nil &&
		s.pending_triggers.Has(config_changed) {
		if !s.is_building {
			s.is_building = true
			go s.drain_and_build()
		}
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// classify events into triggers and hook indices
	triggers := &set.Set[trigger]{}
	hook_indices := &set.Set[int]{}
	evt_paths := &set.Set[strict.CWDRelPath]{}
	var parse_err error
	for _, evt := range evts {
		excluded, err := s.is_globally_excluded(evt.Path)
		if err != nil {
			parse_err = err
			break
		}
		if excluded {
			continue
		}

		evt_paths.Add(evt.Path)

		// config file gets special handling: parse to compute a
		// normalized hash and compare against the current hash
		// without mutating global state. The real parse_cfg call
		// happens in drain_and_build when ConfigChanged fires.
		if evt.Path == s.cfg_path {
			uc, err := config_path_to_unsafe_config(s.cfg_path)
			if err != nil {
				parse_err = fmt.Errorf("config parse error: %w", err)
				break
			}
			if string(uc.raw_file_json) != string(s.cfg.raw_file_json) {
				s.logger.Info(
					"Config changed, triggering full build",
				)
				triggers.Add(config_changed)
				break
			}
			s.logger.Info("Config file unchanged")
			continue
		}

		// classify against core triggers (private/public/css)
		result, err := s.classify_evt(evt)
		if err != nil {
			parse_err = err
			break
		}

		// classify against validated hooks (config + plugin, unified)
		matched_hooks, err := s.classify_hooks_for_evt(evt)
		if err != nil {
			parse_err = err
			break
		}

		if result.matched || len(matched_hooks) > 0 {
			s.logger.Info("[watcher]",
				"op", string(evt.Op),
				"path", evt.Path,
			)
		}

		if result.matched {
			triggers.Add(result.trigger)
		}
		for _, idx := range matched_hooks {
			hook_indices.Add(idx)
		}
	}

	// merge classified results into pending state under the lock
	s.mu.Lock()
	if parse_err != nil {
		// log and do nothing — the config or patterns are broken;
		// the user must fix the offending file. The next FS event
		// on that file will retry naturally.
		s.log_build_err(parse_err)
	}
	if triggers.Len() > 0 || hook_indices.Len() > 0 {
		if triggers.Len() > 0 {
			if s.pending_triggers == nil {
				s.pending_triggers = triggers
			} else {
				s.pending_triggers.Union(triggers)
			}
		}
		if hook_indices.Len() > 0 {
			if s.pending_hook_indices == nil {
				s.pending_hook_indices = hook_indices
			} else {
				s.pending_hook_indices.Union(hook_indices)
			}
		}
		if s.pending_evt_paths == nil {
			s.pending_evt_paths = evt_paths
		} else {
			s.pending_evt_paths.Union(evt_paths)
		}
	}
	if s.has_pending() && !s.is_building {
		s.is_building = true
		go s.drain_and_build()
	}
	s.mu.Unlock()
	return nil
}

func (s *super_state) is_globally_excluded(
	path strict.CWDRelPath,
) (bool, error) {
	for _, pattern := range s.cfg.core.GlobalWatchExcludePatterns {
		match, err := path_match(pattern, path)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

/////////////////////////////////////////////////////////////////////
/////// Entrypoint
/////////////////////////////////////////////////////////////////////

//go:embed default_gitignore.txt
var default_gitignore []byte

type BuildOpts struct {
	// Must be relative to your literal process CWD.
	ConfigPath strict.CWDRelPath
	IsDev      bool
	Logger     *slog.Logger
	Plugins    []*Plugin
}

func Build(opts BuildOpts) {
	build_ctx, build_cancel := context.WithCancel(context.Background())
	defer build_cancel()
	build_exit_code := 0

	logger := opts.Logger
	if logger == nil {
		logger = colorlog.New("wave")
	}
	cfg_path := strict.MustNormalizeCWDRelPath(opts.ConfigPath)

	// validate config path exists
	if is_file, err := cfg_path.IsFile(); err != nil || !is_file {
		if err != nil {
			logger.Error("config path error: " + err.Error())
		} else {
			logger.Error("config path is not a file: " + cfg_path.Str())
		}
		os.Exit(1)
	}

	s := &super_state{
		cfg_path: cfg_path,
		is_dev:   opts.IsDev,
		logger:   logger,
	}

	// validate plugin config (key not reserved) and grab schema
	// contribution. This is the only plugin work that happens before
	// the user config is parsed — everything else (Config.Parse,
	// runtime hook validation) requires a validated config.
	if len(opts.Plugins) > 0 {
		s.plugins = make([]*Plugin, 0, len(opts.Plugins))
		s.plugin_cfgs = make([]*validated_plugin_config, 0, len(opts.Plugins))
		used_json_keys := make(map[string]string)
		for i, plugin := range opts.Plugins {
			if plugin == nil {
				s.logger.Error(
					fmt.Sprintf("Plugin at index %d cannot be nil", i),
				)
				os.Exit(1)
			}
			plugin_name := strings.TrimSpace(plugin.Name)
			if plugin_name == "" {
				s.logger.Error(
					fmt.Sprintf("Plugin at index %d name cannot be empty", i),
				)
				os.Exit(1)
			}
			plugin_cfg, err := validate_plugin_config(plugin.Config)
			if err != nil {
				s.logger.Error(
					"Plugin config validation failed: " + err.Error(),
				)
				os.Exit(1)
			}
			if plugin_cfg.json_key != "" {
				if prior_plugin_name, exists := used_json_keys[plugin_cfg.json_key]; exists {
					s.logger.Error(fmt.Sprintf(
						"Plugin config key %q is used by both %q and %q",
						plugin_cfg.json_key,
						prior_plugin_name,
						plugin_name,
					))
					os.Exit(1)
				}
				used_json_keys[plugin_cfg.json_key] = plugin_name
			}
			s.plugins = append(s.plugins, plugin)
			s.plugin_cfgs = append(s.plugin_cfgs, plugin_cfg)
		}
	}

	// ensure output directories
	waveout := cfg_path.Dir().Join(constants.DIST_DIRNAME)
	if err := fsutil.EnsureDirs(
		waveout.Join(constants.STATIC_ASSETS_PRIVATE_DIR).Str(),
		waveout.Join(constants.STATIC_ASSETS_PUBLIC_DIR).Str(),
		waveout.Join(constants.STATIC_INTERNAL_DIR).Str(),
		waveout.Join(constants.DEV_DIRNAME).Str(),
	); err != nil {
		s.logger.Error("Failed to ensure output directories: " + err.Error())
		os.Exit(1)
	}
	if err := os.WriteFile(
		waveout.Join(constants.STATIC_DIRNAME, constants.KEEP_FILENAME).Str(),
		[]byte("//go:embed directives require at least one file to compile\n"),
		0644,
	); err != nil {
		s.logger.Error("Failed to write .keep file: " + err.Error())
		os.Exit(1)
	}
	if err := os.WriteFile(
		waveout.Join(".gitignore").Str(),
		default_gitignore,
		0644,
	); err != nil {
		s.logger.Error("Failed to write .gitignore: " + err.Error())
		os.Exit(1)
	}

	// build and write config schema
	// Done early so users get IDE autocomplete even if their config
	// is currently broken. Uses the plugins' static schema contributions.
	json_schema_bytes, err := jsonutil.SerializePretty(
		build_schema(s.plugin_cfgs),
	)
	if err != nil {
		s.logger.Error("Failed to serialize config schema: " + err.Error())
		os.Exit(1)
	}
	if err := os.WriteFile(
		waveout.Join(constants.SCHEMA_JSON_FILENAME).Str(),
		append(json_schema_bytes, '\n'),
		0644,
	); err != nil {
		s.logger.Error("Failed to write config schema: " + err.Error())
		os.Exit(1)
	}

	// parse and validate user config, then plugin config parse,
	// then validate plugin runtime hooks
	if err := s.reload_config(); err != nil {
		s.logger.Error(err.Error())
		os.Exit(1)
	}

	// dev-only: acquire project lock
	// Prevents two Wave dev servers from running on the same project.
	// If the prior Wave crashed, the lock's heartbeat goes stale and
	// the new instance takes over automatically.
	if s.is_dev {
		s.lock = lockfile.NewPIDLock(
			waveout.Join(
				constants.DEV_DIRNAME,
				constants.WAVE_LOCK_FILENAME,
			).Str(),
		)
		if err := s.lock.Acquire(); err != nil {
			s.logger.Error(
				"Another wave dev server is already running for this project",
			)
			os.Exit(1)
		}
	}

	var sig_ch chan os.Signal
	if s.is_dev {
		var cleanup_once sync.Once
		dev_cleanup := func() {
			cleanup_once.Do(func() {
				build_cancel()
				if sig_ch != nil {
					signal.Stop(sig_ch)
				}
				if s.supervisor != nil {
					s.supervisor.stop()
				}
				if s.vite_sup != nil {
					s.vite_sup.stop()
				}
				if s.browser != nil {
					s.browser.stop()
				}
				if s.lock != nil {
					s.lock.Release()
				}
			})
		}
		defer func() {
			dev_cleanup()
			if build_exit_code != 0 {
				os.Exit(build_exit_code)
			}
		}()
	}

	// dev-only setup: supervisors, browser sync, signal handler
	if s.is_dev {
		// kill stale processes from prior crashed runs
		kill_stale_pid(s.app_pid_file_path(), "app server", s.logger)
		kill_stale_pid(s.vite_pid_file_path(), "vite", s.logger)

		// find a free port for the app server
		port, err := netutil.GetFreePort(8080)
		if err != nil {
			s.logger.Error("Failed to find free port: " + err.Error())
			build_exit_code = 1
			return
		}
		s.dev_port = port

		// start vite dev server (frontend only)
		if !s.cfg.core.ServerOnlyMode && s.cfg.vite.UsingVite {
			vite_default_port := int(s.cfg.vite.DefaultPort)
			vite_port, err := netutil.GetFreePort(vite_default_port)
			if err != nil {
				s.logger.Error("Failed to find free Vite port: " + err.Error())
				build_exit_code = 1
				return
			}
			s.vite_port = vite_port

			s.vite_sup = new_vite_supervisor(
				s.vite_build_cfg,
				s.vite_port,
				s.hook_env(),
				s.vite_pid_file_path(),
				s.logger,
			)

			if err := s.vite_sup.start(); err != nil {
				s.logger.Error("Failed to start Vite: " + err.Error())
				build_exit_code = 1
				return
			}
		}

		// start browser sync server (frontend only)
		if !s.cfg.core.ServerOnlyMode {
			refresh_port, err := netutil.GetRandomFreePort()
			if err != nil {
				s.logger.Error(
					"Failed to find free refresh port: " + err.Error(),
				)
				build_exit_code = 1
				return
			}
			s.refresh_port = refresh_port
			refresh_token, err := id.New(12)
			if err != nil {
				s.logger.Error(
					"Failed to generate browser refresh token: " + err.Error(),
				)
				build_exit_code = 1
				return
			}
			s.refresh_token = refresh_token
			s.browser = new_browser_sync(
				s.refresh_port,
				refresh_token,
				s.logger,
			)
			s.browser.start()
		}

		// create app server supervisor
		s.supervisor = &supervisor{
			bin_out_path_abs: s.cfg.core.binary_output_path_abs,
			port:             s.dev_port,
			health:           s.cfg.core.HealthcheckEndpoint,
			env:              s.app_env(),
			pid_file:         s.app_pid_file_path(),
			logger:           s.logger,
		}

		// signal handling: first signal is graceful, second is
		// immediate. PID files left by interrupted cleanup are
		// handled on the next startup by kill_stale_pid.
		sig_ch = make(chan os.Signal, 2)
		signal.Notify(sig_ch, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sig_ch
			s.logger.Info("received signal. shutting down.",
				"signal", sig,
			)

			build_cancel()

			// second signal exits immediately
			go func() {
				<-sig_ch
				s.logger.Info("received second signal. forcing exit.")
				os.Exit(1)
			}()
		}()
	}

	// initial build
	initial_triggers := &set.Set[trigger]{}
	initial_triggers.Add(config_changed)
	if _, err := s.run_build(build_params{
		triggers:     initial_triggers,
		hook_indices: s.all_applicable_hook_indices(true),
		should_abort: func() bool { return build_ctx.Err() != nil },
		is_initial:   true,
	}); err != nil {
		s.logger.Error("initial build failed: " + err.Error())
		build_exit_code = 1
		return
	}

	// watch loop (dev only)
	if s.is_dev {
		s.watcher = fswatcher.NewWatcher(fswatcher.WatcherOptions{
			WatchRoot: s.cfg.root_dir,
			OnRemovePath: func(p strict.CWDRelPath) {
				s.logger.Info("removing watch on", "path", p)
			},
		})

		s.watcher.Watch(build_ctx, func(evts []fswatcher.Evt) error {
			return s.handle_watch_evts(evts)
		})
	}
}
