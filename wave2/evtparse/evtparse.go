package evtparse

// import (
// 	"os"
// 	"path/filepath"
// 	"slices"
// 	"strings"

// 	"github.com/bmatcuk/doublestar/v4"
// 	"github.com/fsnotify/fsnotify"
// 	"github.com/vormadev/vorma/kit/set"
// 	"github.com/vormadev/vorma/kit/strict"
// 	"github.com/vormadev/vorma/wave2/config"
// )

// type Workset struct {
// 	ShouldShortCircuit bool
// 	ShortCircuitReason ShortCircuitReason

// 	// Expected supervisor behavior:
// 	// - if `Watcher.PerformFullReset` is true, fully rebuild watcher from resolve root
// 	// - else if `Watcher.WatchNewDirs` is non-empty, watch new dirs incrementally
// 	Watcher struct {
// 		WatchNewDirs     []strict.CWDRelPath
// 		PerformFullReset bool
// 	}

// 	Builder struct {
// 		RunImplicitBuild              bool
// 		PreferFrontendRevalidate      bool
// 		CompileGoBinary               bool
// 		RestartAppServer              bool
// 		HandleCriticalCSS             bool
// 		HandleNormalCSS               bool
// 		HandlePublicStatic            bool
// 		HandlePrivateStatic           bool
// 		SkipFrontendRebuildingOverlay bool

// 		HookCmds struct {
// 			Pre              []string
// 			Concurrent       []string
// 			ConcurrentNoWait []string
// 			Post             []string
// 		}
// 	}
// }

// type ShortCircuitReason string

// const (
// 	ShortCircuitNotApplicable        ShortCircuitReason = ""
// 	ShortCircuitReasonConfigChange   ShortCircuitReason = "config_change"
// 	ShortCircuitReasonBuildRetryWait ShortCircuitReason = "build_retry_wait"
// 	ShortCircuitReasonNoOp           ShortCircuitReason = "noop"
// )

// type CallerCtx struct {
// 	WaitingForBuildRetry bool
// }

// func RawBatchToWorkset(
// 	raw_batch []fsnotify.Event,
// 	cfg *config.Parsed,
// 	caller_ctx CallerCtx,
// ) Workset {
// 	if cfg == nil {
// 		panic("evtparse: parsed_cfg is nil")
// 	}

// 	normalized_evts := make([]evt, 0, len(raw_batch))
// 	for _, raw_evt := range raw_batch {
// 		path := strict.CWDRelPath(
// 			filepath.Clean(strings.TrimSpace(raw_evt.Name)),
// 		)
// 		if path == "." {
// 			continue
// 		}

// 		stat, stat_err := os.Stat(string(path))
// 		is_non_empty_file := stat_err == nil && !stat.IsDir() && stat.Size() > 0
// 		is_dir := stat_err == nil && stat.IsDir()

// 		has_chmod := raw_evt.Has(fsnotify.Chmod)
// 		has_create := raw_evt.Has(fsnotify.Create)
// 		has_write := raw_evt.Has(fsnotify.Write)
// 		has_remove := raw_evt.Has(fsnotify.Remove)
// 		has_rename := raw_evt.Has(fsnotify.Rename)

// 		if has_chmod &&
// 			is_non_empty_file &&
// 			!has_create &&
// 			!has_write &&
// 			!has_remove &&
// 			!has_rename {
// 			continue
// 		}

// 		var op evt_op
// 		switch {
// 		case has_remove, has_rename:
// 			op = evt_op_delete
// 		case has_create:
// 			op = evt_op_create
// 		case has_write, has_chmod:
// 			op = evt_op_edit
// 		default:
// 			continue
// 		}

// 		normalized_evts = append(normalized_evts, evt{
// 			path:   path,
// 			op:     op,
// 			is_dir: is_dir,
// 		})
// 	}

// 	slices.SortStableFunc(
// 		normalized_evts,
// 		func(a, b evt) int {
// 			return strings.Compare(string(a.path), string(b.path))
// 		},
// 	)

// 	w := Workset{}
// 	has_config_path_evt := false
// 	has_builder_work := false
// 	all_processed_evts_skip_rebuilding_notification := true

// 	created_dirs := set.Set[strict.CWDRelPath]{}

// 	for _, evt := range normalized_evts {
// 		if evt.path == "" || evt.path == "." {
// 			continue
// 		}

// 		if evt.path == cfg.ConfigPath {
// 			has_config_path_evt = true
// 			continue
// 		}

// 		var is_excluded bool
// 		for _, exclude_pattern := range cfg.Watch.Exclude {
// 			is_excluded = is_pattern_match(exclude_pattern, evt.path)
// 			if is_excluded {
// 				break
// 			}
// 		}
// 		if is_excluded {
// 			continue
// 		}
// 		if evt.op == evt_op_create && evt.is_dir {
// 			created_dirs.Add(evt.path)
// 		}
// 		if evt.op == evt_op_delete {
// 			w.Watcher.PerformFullReset = true
// 		}

// 		has_include_match := false
// 		recompile_go_binary := false
// 		include_restart_app := false
// 		only_run_client_defined_revalidate_func := false
// 		run_on_change_only := true
// 		treat_as_non_go := true
// 		skip_rebuilding_notification := true

// 		var pre_hook_cmds []string
// 		var concurrent_hook_cmds []string
// 		var concurrent_no_wait_hook_cmds []string
// 		var post_hook_cmds []string

// 		for _, include_entry := range cfg.Watch.Include {
// 			if !is_pattern_match(include_entry.Pattern, evt.path) {
// 				continue
// 			}
// 			has_include_match = true
// 			if include_entry.RecompileGoBinary {
// 				recompile_go_binary = true
// 			}
// 			if include_entry.RestartApp {
// 				include_restart_app = true
// 			}
// 			if include_entry.OnlyRunClientDefinedRevalidateFunc {
// 				only_run_client_defined_revalidate_func = true
// 			}
// 			if !include_entry.RunOnChangeOnly {
// 				run_on_change_only = false
// 			}
// 			if !include_entry.TreatAsNonGo {
// 				treat_as_non_go = false
// 			}
// 			if !include_entry.SkipRebuildingNotification {
// 				skip_rebuilding_notification = false
// 			}

// 			for _, hook := range include_entry.OnChangeHooks {
// 				var is_excluded bool
// 				for _, exclude_pattern := range hook.Exclude {
// 					is_excluded = is_pattern_match(exclude_pattern, evt.path)
// 					if is_excluded {
// 						break
// 					}
// 				}
// 				if is_excluded {
// 					continue
// 				}

// 				switch hook.Timing {
// 				case config.OnChangeHookTimingPre:
// 					if !slices.Contains(pre_hook_cmds, hook.Cmd) {
// 						pre_hook_cmds = append(pre_hook_cmds, hook.Cmd)
// 					}
// 				case config.OnChangeHookTimingConcurrent:
// 					if !slices.Contains(concurrent_hook_cmds, hook.Cmd) {
// 						concurrent_hook_cmds = append(
// 							concurrent_hook_cmds,
// 							hook.Cmd,
// 						)
// 					}
// 				case config.OnChangeHookTimingConcurrentNoWait:
// 					if !slices.Contains(
// 						concurrent_no_wait_hook_cmds,
// 						hook.Cmd,
// 					) {
// 						concurrent_no_wait_hook_cmds = append(
// 							concurrent_no_wait_hook_cmds,
// 							hook.Cmd,
// 						)
// 					}
// 				case config.OnChangeHookTimingPost:
// 					if !slices.Contains(post_hook_cmds, hook.Cmd) {
// 						post_hook_cmds = append(post_hook_cmds, hook.Cmd)
// 					}
// 				default:
// 					panic("evtparse: unexpected hook timing")
// 				}
// 			}
// 		}
// 		if !has_include_match {
// 			run_on_change_only = false
// 			treat_as_non_go = false
// 			skip_rebuilding_notification = false
// 		}

// 		is_go := strings.EqualFold(filepath.Ext(string(evt.path)), ".go")
// 		if is_go && has_include_match && treat_as_non_go {
// 			is_go = false
// 		}

// 		is_critical_css := evt.path == cfg.Core.CriticalCSSEntry
// 		is_normal_css := evt.path == cfg.Core.NonCriticalCSSEntry

// 		public_static_pattern := strict.CWDRelPath(
// 			filepath.Join(string(cfg.Core.StaticPublicDir), "**", "*"),
// 		)
// 		private_static_pattern := strict.CWDRelPath(
// 			filepath.Join(string(cfg.Core.StaticPrivateDir), "**", "*"),
// 		)
// 		is_public_static := is_pattern_match(public_static_pattern, evt.path)
// 		is_private_static := is_pattern_match(private_static_pattern, evt.path)

// 		if evt.is_dir && evt.op != evt_op_delete &&
// 			!is_public_static && !is_private_static {
// 			continue
// 		}

// 		if !is_go &&
// 			!is_critical_css &&
// 			!is_normal_css &&
// 			!is_public_static &&
// 			!is_private_static &&
// 			!has_include_match {
// 			continue
// 		}
// 		for _, cmd := range pre_hook_cmds {
// 			if slices.Contains(w.Builder.HookCmds.Pre, cmd) {
// 				continue
// 			}
// 			w.Builder.HookCmds.Pre = append(
// 				w.Builder.HookCmds.Pre,
// 				cmd,
// 			)
// 			has_builder_work = true
// 		}
// 		for _, cmd := range concurrent_hook_cmds {
// 			if slices.Contains(w.Builder.HookCmds.Concurrent, cmd) {
// 				continue
// 			}
// 			w.Builder.HookCmds.Concurrent = append(
// 				w.Builder.HookCmds.Concurrent,
// 				cmd,
// 			)
// 			has_builder_work = true
// 		}
// 		for _, cmd := range concurrent_no_wait_hook_cmds {
// 			if slices.Contains(
// 				w.Builder.HookCmds.ConcurrentNoWait,
// 				cmd,
// 			) {
// 				continue
// 			}
// 			w.Builder.HookCmds.ConcurrentNoWait = append(
// 				w.Builder.HookCmds.ConcurrentNoWait,
// 				cmd,
// 			)
// 			has_builder_work = true
// 		}
// 		for _, cmd := range post_hook_cmds {
// 			if slices.Contains(w.Builder.HookCmds.Post, cmd) {
// 				continue
// 			}
// 			w.Builder.HookCmds.Post = append(
// 				w.Builder.HookCmds.Post,
// 				cmd,
// 			)
// 			has_builder_work = true
// 		}
// 		all_processed_evts_skip_rebuilding_notification =
// 			all_processed_evts_skip_rebuilding_notification &&
// 				skip_rebuilding_notification
// 		if has_include_match && run_on_change_only {
// 			continue
// 		}

// 		if !run_on_change_only {
// 			w.Builder.RunImplicitBuild = true
// 			has_builder_work = true
// 		}
// 		if only_run_client_defined_revalidate_func {
// 			w.Builder.PreferFrontendRevalidate = true
// 			has_builder_work = true
// 		}

// 		switch {
// 		case is_go:
// 			w.Builder.CompileGoBinary = true
// 			has_builder_work = true
// 			w.Builder.RestartAppServer = true
// 			has_builder_work = true
// 		case is_critical_css || is_normal_css:
// 			if is_critical_css {
// 				w.Builder.HandleCriticalCSS = true
// 				has_builder_work = true
// 			}
// 			if is_normal_css {
// 				w.Builder.HandleNormalCSS = true
// 				has_builder_work = true
// 			}
// 			if has_include_match &&
// 				(include_restart_app || recompile_go_binary) {
// 				w.Builder.RestartAppServer = true
// 				has_builder_work = true
// 			}
// 		case is_public_static:
// 			w.Builder.HandlePublicStatic = true
// 			has_builder_work = true
// 		case is_private_static:
// 			w.Builder.HandlePrivateStatic = true
// 			has_builder_work = true
// 		default:
// 			if recompile_go_binary {
// 				w.Builder.CompileGoBinary = true
// 				has_builder_work = true
// 			}
// 			if include_restart_app || recompile_go_binary {
// 				w.Builder.RestartAppServer = true
// 				has_builder_work = true
// 			}
// 		}

// 	}
// 	w.Watcher.WatchNewDirs = created_dirs.Slice()
// 	slices.SortStableFunc(
// 		w.Watcher.WatchNewDirs,
// 		func(a, b strict.CWDRelPath) int {
// 			return strings.Compare(string(a), string(b))
// 		},
// 	)

// 	builder_noop := !has_builder_work
// 	watcher_noop :=
// 		len(w.Watcher.WatchNewDirs) == 0 &&
// 			!w.Watcher.PerformFullReset

// 	if has_config_path_evt {
// 		w.ShouldShortCircuit = true
// 		w.ShortCircuitReason = ShortCircuitReasonConfigChange
// 		return w
// 	}
// 	if caller_ctx.WaitingForBuildRetry {
// 		w.ShouldShortCircuit = true
// 		w.ShortCircuitReason = ShortCircuitReasonBuildRetryWait
// 		return w
// 	}
// 	if builder_noop && watcher_noop {
// 		w.ShouldShortCircuit = true
// 		w.ShortCircuitReason = ShortCircuitReasonNoOp
// 		return w
// 	}
// 	if !builder_noop {
// 		w.Builder.SkipFrontendRebuildingOverlay =
// 			all_processed_evts_skip_rebuilding_notification
// 	}
// 	return w
// }

// type evt struct {
// 	path   strict.CWDRelPath
// 	op     evt_op
// 	is_dir bool
// }

// type evt_op string

// const (
// 	evt_op_create evt_op = "create"
// 	evt_op_edit   evt_op = "edit"
// 	evt_op_delete evt_op = "delete"
// )

// /////////////////////////////////////////////////////////////////////
// /////// Utils
// /////////////////////////////////////////////////////////////////////

// func is_pattern_match(
// 	pattern strict.CWDRelPath,
// 	path strict.CWDRelPath,
// ) bool {
// 	// already validated when cfg was parsed
// 	return doublestar.PathMatchUnvalidated(string(pattern), string(path))
// }
