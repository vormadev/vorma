package evtparse

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/strict"
	"github.com/vormadev/vorma/wave2/internal/cfg"
)

type evt_op string

const (
	evt_op_create evt_op = "create"
	evt_op_edit   evt_op = "edit"
	evt_op_delete evt_op = "delete"
)

type evt struct {
	path   strict.CWDRelPath
	op     evt_op
	is_dir bool
}

func normalize_evts(evts []fsnotify.Event) []evt {
	if len(evts) == 0 {
		return nil
	}

	normalized := make([]evt, 0, len(evts))
	for _, raw_evt := range evts {
		path := strict.CWDRelPath(
			filepath.Clean(strings.TrimSpace(raw_evt.Name)),
		)
		if path == "." {
			continue
		}

		stat, stat_err := os.Stat(string(path))
		is_empty_file := stat_err == nil && stat.Size() > 0
		is_dir := stat_err == nil && stat.IsDir()

		has_chmod := raw_evt.Has(fsnotify.Chmod)
		has_create := raw_evt.Has(fsnotify.Create)
		has_write := raw_evt.Has(fsnotify.Write)
		has_remove := raw_evt.Has(fsnotify.Remove)
		has_rename := raw_evt.Has(fsnotify.Rename)

		if has_chmod &&
			is_empty_file &&
			!has_create &&
			!has_write &&
			!has_remove &&
			!has_rename {
			continue
		}

		var op evt_op
		switch {
		case has_remove, has_rename:
			op = evt_op_delete
		case has_create:
			op = evt_op_create
		case has_write, has_chmod:
			op = evt_op_edit
		default:
			continue
		}

		normalized = append(normalized, evt{
			path:   path,
			op:     op,
			is_dir: is_dir,
		})
	}

	slices.SortStableFunc(
		normalized,
		func(a, b evt) int {
			return strings.Compare(string(a.path), string(b.path))
		},
	)

	return normalized
}

type caller_context struct {
	waiting_for_build_retry bool
}

type builder_workset struct {
	short_circuit_for_config_restart   bool
	short_circuit_for_build_retry_wait bool
	short_circuit_for_noop             bool

	run_implicit_build bool
	prefer_revalidate  bool

	compile_go_binary  bool
	build_critical_css bool
	build_normal_css   bool

	process_public_static  bool
	process_private_static bool
}

func evts_to_builder_workset(
	evts []evt,
	cfg *cfg.Parsed,
	caller_context caller_context,
) builder_workset {
	if cfg == nil {
		panic("evtparse: parsed_cfg is nil")
	}

	workset := builder_workset{}
	processed_evt_count := 0

	for _, evt := range evts {
		if evt.path == "" || evt.path == "." {
			continue
		}

		if evt.path == cfg.ConfigPath {
			workset.short_circuit_for_config_restart = true
			continue
		}

		var is_excluded bool
		for _, exclude_pattern := range cfg.Watch.Exclude {
			is_excluded = is_pattern_match(exclude_pattern, evt.path)
		}
		if is_excluded {
			continue
		}

		has_include_match := false
		recompile_go_binary := false
		restart_app := false
		only_run_client_defined_revalidate_func := false
		run_on_change_only := true
		treat_as_non_go := true
		for _, include_entry := range cfg.Watch.Include {
			if !is_pattern_match(include_entry.Pattern, evt.path) {
				continue
			}
			has_include_match = true
			if include_entry.RecompileGoBinary {
				recompile_go_binary = true
			}
			if include_entry.RestartApp {
				restart_app = true
			}
			if include_entry.OnlyRunClientDefinedRevalidateFunc {
				only_run_client_defined_revalidate_func = true
			}
			if !include_entry.RunOnChangeOnly {
				run_on_change_only = false
			}
			if !include_entry.TreatAsNonGo {
				treat_as_non_go = false
			}
		}
		if !has_include_match {
			run_on_change_only = false
			treat_as_non_go = false
		}

		is_go := strings.EqualFold(filepath.Ext(string(evt.path)), ".go")
		if is_go && has_include_match && treat_as_non_go {
			is_go = false
		}

		is_critical_css := evt.path == cfg.Core.CriticalCSSEntry
		is_normal_css := evt.path == cfg.Core.NonCriticalCSSEntry

		public_static_pattern := strict.CWDRelPath(
			filepath.Join(string(cfg.Core.StaticPublicDir), "**", "*"),
		)
		private_static_pattern := strict.CWDRelPath(
			filepath.Join(string(cfg.Core.StaticPrivateDir), "**", "*"),
		)
		is_public_static := is_pattern_match(public_static_pattern, evt.path)
		is_private_static := is_pattern_match(private_static_pattern, evt.path)

		if evt.is_dir && evt.op != evt_op_delete &&
			!is_public_static && !is_private_static {
			continue
		}

		if !is_go &&
			!is_critical_css &&
			!is_normal_css &&
			!is_public_static &&
			!is_private_static &&
			!has_include_match {
			continue
		}

		processed_evt_count++
		if !run_on_change_only {
			workset.run_implicit_build = true
		}
		if only_run_client_defined_revalidate_func {
			workset.prefer_revalidate = true
		}

		switch {
		case is_go:
			workset.compile_go_binary = true
		case is_critical_css || is_normal_css:
			if is_critical_css {
				workset.build_critical_css = true
			}
			if is_normal_css {
				workset.build_normal_css = true
			}
		case is_public_static:
			workset.process_public_static = true
		case is_private_static:
			workset.process_private_static = true
		default:
			if recompile_go_binary {
				workset.compile_go_binary = true
			}
		}

		if restart_app || recompile_go_binary {
			workset.prefer_revalidate = workset.prefer_revalidate ||
				only_run_client_defined_revalidate_func
		}
	}

	if workset.short_circuit_for_config_restart {
		return builder_workset{
			short_circuit_for_config_restart: true,
		}
	}
	if caller_context.waiting_for_build_retry {
		return builder_workset{
			short_circuit_for_build_retry_wait: true,
		}
	}
	if processed_evt_count == 0 {
		return builder_workset{
			short_circuit_for_noop: true,
		}
	}
	return workset
}

func is_pattern_match(
	pattern strict.CWDRelPath,
	path strict.CWDRelPath,
) bool {
	// already validated when cfg was parsed
	return doublestar.PathMatchUnvalidated(string(pattern), string(path))
}
