package wavebuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave2"
)

/////////////////////////////////////////////////////////////////////
/////// Source Config Contracts
/////////////////////////////////////////////////////////////////////

// source_cfg is the authored Wave2 config document shape consumed by
// wavebuild.
type source_cfg struct {
	core  source_core_cfg
	vite  *source_vite_cfg
	watch *source_watch_cfg
}

type source_core_cfg struct {
	resolve_root                string
	dev_build_hook              string
	dev_build_hook_timeout_ms   int // __in_main?
	prod_build_hook             string
	prod_build_hook_timeout_ms  int // __in_main?
	main_app_entry              string
	static_asset_dirs_private   string
	static_asset_dirs_public    string
	critical_css_entry_file     string
	non_critical_css_entry_file string
	public_path_prefix          string
	server_only_mode            bool
	sequential_go_build         bool
}

type source_vite_cfg struct {
	js_ppg_mgr_base_cmd string
	js_pkg_mgr_cmd_dir  string
	default_port        int
	vite_cfg_file       string
}

type source_watch_cfg struct {
	healthcheck_endpoint string
	include              []source_watched_file
	exclude_dirs         []string
	exclude_files        []string
}

type source_watched_file struct {
	pattern                                 string
	on_change_hooks                         []source_on_change_hook
	recompile_go_binary                     bool
	restart_app                             bool
	only_run_client_defined_revalidate_func bool
	run_on_change_only                      bool
	skip_rebuilding_notification            bool
	treat_as_non_go                         bool
}

type source_on_change_hook struct {
	cmd                                  string
	command_timeout_ms                   int  // __in_main?
	disable_stage_command_timeout        bool // __in_main?
	callback_timeout_ms                  int  // __in_main?
	disable_stage_callback_timeout       bool // __in_main?
	run_combined_dev_build_hook_commands bool // __in_main?
	timing                               string
	exclude                              []string
}

/////////////////////////////////////////////////////////////////////
/////// Normalized Build Config Contracts
/////////////////////////////////////////////////////////////////////

// normalized_build_cfg is the CWD-relative build/dev config produced from one
// authored source config file.
type normalized_build_cfg struct {
	cfg_path               string
	cfg_dir                string
	effective_resolve_root string
	dist_root              string
	dist_static_root       string
	core                   normalized_core_cfg
	vite                   *normalized_vite_cfg
	watch                  *normalized_watch_cfg
	runtime_cfg            wave2.RuntimeConfig
}

type normalized_core_cfg struct {
	dev_build_hook              string
	dev_build_hook_timeout_ms   int // __in_main?
	prod_build_hook             string
	prod_build_hook_timeout_ms  int // __in_main?
	main_app_entry              string
	static_asset_dirs_private   string
	static_asset_dirs_public    string
	critical_css_entry_file     string
	non_critical_css_entry_file string
	public_path_prefix          string
	server_only_mode            bool
	sequential_go_build         bool
}

type normalized_vite_cfg struct {
	js_pkg_mgr_base_cmd string
	js_pkg_mgr_cmd_dir  string
	default_port        int
	vite_cfg_file       string
}

type normalized_watch_cfg struct {
	healthcheck_endpoint string
	include              []normalized_watched_file
	exclude_dirs         []string
	exclude_files        []string
}

type normalized_watched_file struct {
	pattern                                 string
	on_change_hooks                         []normalized_on_change_hook
	recompile_go_binary                     bool
	restart_app                             bool
	only_run_client_defined_revalidate_func bool
	run_on_change_only                      bool
	skip_rebuilding_notification            bool
	treat_as_non_go                         bool
}

type normalized_on_change_hook struct {
	cmd                                  string
	command_timeout_ms                   int  // __in_main?
	disable_stage_command_timeout        bool // __in_main?
	callback_timeout_ms                  int  // __in_main?
	disable_stage_callback_timeout       bool // __in_main?
	run_combined_dev_build_hook_commands bool // __in_main?
	timing                               string
	exclude                              []string
}

/////////////////////////////////////////////////////////////////////
/////// Parse + Normalize Boundary
/////////////////////////////////////////////////////////////////////

// load_normalized_build_cfg parses one authored config file and converts all
// filesystem fields into CWD-relative paths for downstream build/dev use.
func load_normalized_build_cfg(
	cfg_path string,
) (normalized_build_cfg, error) {
	cfg_path = filepath.Clean(strings.TrimSpace(cfg_path))
	if cfg_path == "." {
		return normalized_build_cfg{}, fmt.Errorf(
			"wavebuild: BuildOptions.ConfigPath is required",
		)
	}
	if filepath.IsAbs(cfg_path) {
		return normalized_build_cfg{}, fmt.Errorf(
			"wavebuild: BuildOptions.ConfigPath must be relative to process CWD: %q",
			cfg_path,
		)
	}

	source_cfg_json, err := os.ReadFile(cfg_path)
	if err != nil {
		return normalized_build_cfg{}, fmt.Errorf(
			"wavebuild: read config %q: %w",
			cfg_path,
			err,
		)
	}

	source_cfg, err := parse_source_cfg_json(source_cfg_json)
	if err != nil {
		return normalized_build_cfg{}, fmt.Errorf(
			"wavebuild: parse config %q: %w",
			cfg_path,
			err,
		)
	}

	cfg_dir := filepath.Clean(filepath.Dir(cfg_path))
	if cfg_dir == "" {
		cfg_dir = "."
	}

	effective_resolve_root, err := normalize_optional_relative_path_against_base(
		source_cfg.core.resolve_root,
		cfg_dir,
		cfg.core.resolve_root.full(),
	)
	if err != nil {
		return normalized_build_cfg{}, fmt.Errorf(
			"wavebuild: config %q: %w",
			cfg_path,
			err,
		)
	}
	if effective_resolve_root == "" {
		effective_resolve_root = cfg_dir
	}

	normalized_core_cfg, err := normalize_core_cfg(
		source_cfg.core,
		effective_resolve_root,
	)
	if err != nil {
		return normalized_build_cfg{}, fmt.Errorf(
			"wavebuild: config %q: %w",
			cfg_path,
			err,
		)
	}

	normalized_vite_cfg, err := normalize_vite_cfg(
		source_cfg.vite,
		effective_resolve_root,
	)
	if err != nil {
		return normalized_build_cfg{}, fmt.Errorf(
			"wavebuild: config %q: %w",
			cfg_path,
			err,
		)
	}

	normalized_watch_cfg, err := normalize_watch_cfg(
		source_cfg.watch,
		effective_resolve_root,
	)
	if err != nil {
		return normalized_build_cfg{}, fmt.Errorf(
			"wavebuild: config %q: %w",
			cfg_path,
			err,
		)
	}

	dist_root := filepath.Clean(filepath.Join(cfg_dir, dist.root().full()))

	return normalized_build_cfg{
		cfg_path:               cfg_path,
		cfg_dir:                cfg_dir,
		effective_resolve_root: effective_resolve_root,
		dist_root:              dist_root,
		dist_static_root: filepath.Join(
			cfg_dir,
			dist.static.path().full(),
		),
		core:  normalized_core_cfg,
		vite:  normalized_vite_cfg,
		watch: normalized_watch_cfg,
		runtime_cfg: wave2.RuntimeConfig{
			PublicPathPrefix: normalized_core_cfg.public_path_prefix,
		},
	}, nil
}

func parse_source_cfg_json(
	source_cfg_json []byte,
) (source_cfg, error) {
	source_cfg_document, err := parse_json_object(source_cfg_json, "config")
	if err != nil {
		return source_cfg{}, err
	}

	raw_core_cfg, ok := source_cfg_document[cfg.core.node().last()]
	if !ok {
		return source_cfg{}, fmt.Errorf(
			"%s is required",
			cfg.core.node().full(),
		)
	}
	core, err := parse_source_core_cfg(raw_core_cfg)
	if err != nil {
		return source_cfg{}, err
	}

	var vite *source_vite_cfg
	raw_vite_cfg, has_vite_cfg := source_cfg_document[cfg.vite.node().last()]
	if has_vite_cfg {
		parsed_vite_cfg, err := parse_source_vite_cfg(raw_vite_cfg)
		if err != nil {
			return source_cfg{}, err
		}
		vite = &parsed_vite_cfg
	}

	var watch *source_watch_cfg
	raw_watch_cfg, has_watch_cfg := source_cfg_document[cfg.watch.node().last()]
	if has_watch_cfg {
		parsed_watch_cfg, err := parse_source_watch_cfg(raw_watch_cfg)
		if err != nil {
			return source_cfg{}, err
		}
		watch = &parsed_watch_cfg
	}

	return source_cfg{
		core:  core,
		vite:  vite,
		watch: watch,
	}, nil
}

func parse_source_core_cfg(
	raw_core_cfg json.RawMessage,
) (source_core_cfg, error) {
	core_document, err := parse_json_object(
		raw_core_cfg,
		cfg.core.node().full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}

	main_app_entry, err := parse_required_string_field(
		core_document,
		cfg.core.main_app_entry.last(),
		cfg.core.main_app_entry.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}

	static_asset_dirs_document, err := parse_optional_object_field(
		core_document,
		cfg.core.static_asset_dirs.node().last(),
		cfg.core.static_asset_dirs.node().full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}

	css_entry_files_document, err := parse_optional_object_field(
		core_document,
		cfg.core.css_entry_files.node().last(),
		cfg.core.css_entry_files.node().full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}

	resolve_root, err := parse_optional_string_field(
		core_document,
		cfg.core.resolve_root.last(),
		cfg.core.resolve_root.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	dev_build_hook, err := parse_optional_string_field(
		core_document,
		cfg.core.dev_build_hook.last(),
		cfg.core.dev_build_hook.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	dev_build_hook_timeout_ms, err := parse_optional_int_field(
		core_document,
		cfg.core.dev_build_hook_timeout_ms.last(),
		cfg.core.dev_build_hook_timeout_ms.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	prod_build_hook, err := parse_optional_string_field(
		core_document,
		cfg.core.prod_build_hook.last(),
		cfg.core.prod_build_hook.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	prod_build_hook_timeout_ms, err := parse_optional_int_field(
		core_document,
		cfg.core.prod_build_hook_timeout_ms.last(),
		cfg.core.prod_build_hook_timeout_ms.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	static_asset_dirs_private, err := parse_optional_string_field(
		static_asset_dirs_document,
		cfg.core.static_asset_dirs.private.last(),
		cfg.core.static_asset_dirs.private.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	static_asset_dirs_public, err := parse_optional_string_field(
		static_asset_dirs_document,
		cfg.core.static_asset_dirs.public.last(),
		cfg.core.static_asset_dirs.public.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	critical_css_entry_file, err := parse_optional_string_field(
		css_entry_files_document,
		cfg.core.css_entry_files.critical.last(),
		cfg.core.css_entry_files.critical.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	non_critical_css_entry_file, err := parse_optional_string_field(
		css_entry_files_document,
		cfg.core.css_entry_files.non_critical.last(),
		cfg.core.css_entry_files.non_critical.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	public_path_prefix, err := parse_optional_string_field(
		core_document,
		cfg.core.public_path_prefix.last(),
		cfg.core.public_path_prefix.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	server_only_mode, err := parse_optional_bool_field(
		core_document,
		cfg.core.server_only_mode.last(),
		cfg.core.server_only_mode.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}
	sequential_go_build, err := parse_optional_bool_field(
		core_document,
		cfg.core.sequential_go_build.last(),
		cfg.core.sequential_go_build.full(),
	)
	if err != nil {
		return source_core_cfg{}, err
	}

	return source_core_cfg{
		resolve_root:                resolve_root,
		dev_build_hook:              dev_build_hook,
		dev_build_hook_timeout_ms:   dev_build_hook_timeout_ms,
		prod_build_hook:             prod_build_hook,
		prod_build_hook_timeout_ms:  prod_build_hook_timeout_ms,
		main_app_entry:              main_app_entry,
		static_asset_dirs_private:   static_asset_dirs_private,
		static_asset_dirs_public:    static_asset_dirs_public,
		critical_css_entry_file:     critical_css_entry_file,
		non_critical_css_entry_file: non_critical_css_entry_file,
		public_path_prefix:          public_path_prefix,
		server_only_mode:            server_only_mode,
		sequential_go_build:         sequential_go_build,
	}, nil
}

func parse_source_vite_cfg(
	raw_vite_cfg json.RawMessage,
) (source_vite_cfg, error) {
	vite_document, err := parse_json_object(
		raw_vite_cfg,
		cfg.vite.node().full(),
	)
	if err != nil {
		return source_vite_cfg{}, err
	}

	js_pkg_mgr_base_cmd, err := parse_optional_string_field(
		vite_document,
		cfg.vite.js_pkg_mgr_base_cmd.last(),
		cfg.vite.js_pkg_mgr_base_cmd.full(),
	)
	if err != nil {
		return source_vite_cfg{}, err
	}
	js_pkg_mgr_cmd_dir, err := parse_optional_string_field(
		vite_document,
		cfg.vite.js_pkg_mgr_cmd_dir.last(),
		cfg.vite.js_pkg_mgr_cmd_dir.full(),
	)
	if err != nil {
		return source_vite_cfg{}, err
	}
	default_port, err := parse_optional_int_field(
		vite_document,
		cfg.vite.default_port.last(),
		cfg.vite.default_port.full(),
	)
	if err != nil {
		return source_vite_cfg{}, err
	}
	vite_cfg_file, err := parse_optional_string_field(
		vite_document,
		cfg.vite.vite_cfg_file.last(),
		cfg.vite.vite_cfg_file.full(),
	)
	if err != nil {
		return source_vite_cfg{}, err
	}

	return source_vite_cfg{
		js_ppg_mgr_base_cmd: js_pkg_mgr_base_cmd,
		js_pkg_mgr_cmd_dir:  js_pkg_mgr_cmd_dir,
		default_port:        default_port,
		vite_cfg_file:       vite_cfg_file,
	}, nil
}

func parse_source_watch_cfg(
	raw_watch_cfg json.RawMessage,
) (source_watch_cfg, error) {
	watch_document, err := parse_json_object(
		raw_watch_cfg,
		cfg.watch.node().full(),
	)
	if err != nil {
		return source_watch_cfg{}, err
	}

	healthcheck_endpoint, err := parse_optional_string_field(
		watch_document,
		cfg.watch.healthcheck_endpoint.last(),
		cfg.watch.healthcheck_endpoint.full(),
	)
	if err != nil {
		return source_watch_cfg{}, err
	}

	raw_include, ok := watch_document[cfg.watch.include.node().last()]
	include := []source_watched_file(nil)
	if ok {
		include, err = parse_source_watched_file_list(
			raw_include,
			cfg.watch.include.node().full(),
		)
		if err != nil {
			return source_watch_cfg{}, err
		}
	}

	exclude_document, err := parse_optional_object_field(
		watch_document,
		cfg.watch.exclude.node().last(),
		cfg.watch.exclude.node().full(),
	)
	if err != nil {
		return source_watch_cfg{}, err
	}
	exclude_dirs, err := parse_optional_string_list_field(
		exclude_document,
		cfg.watch.exclude.dirs.last(),
		cfg.watch.exclude.dirs.full(),
	)
	if err != nil {
		return source_watch_cfg{}, err
	}
	exclude_files, err := parse_optional_string_list_field(
		exclude_document,
		cfg.watch.exclude.files.last(),
		cfg.watch.exclude.files.full(),
	)
	if err != nil {
		return source_watch_cfg{}, err
	}

	return source_watch_cfg{
		healthcheck_endpoint: healthcheck_endpoint,
		include:              include,
		exclude_dirs:         exclude_dirs,
		exclude_files:        exclude_files,
	}, nil
}

func parse_source_watched_file_list(
	raw_watched_file_list json.RawMessage,
	field_path string,
) ([]source_watched_file, error) {
	var raw_watched_files []json.RawMessage
	if err := json.Unmarshal(raw_watched_file_list, &raw_watched_files); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array: %w", field_path, err)
	}

	watched_files := make([]source_watched_file, 0, len(raw_watched_files))
	for watched_file_index, raw_watched_file := range raw_watched_files {
		watched_file, err := parse_source_watched_file(
			raw_watched_file,
			fmt.Sprintf("%s[%d]", field_path, watched_file_index),
		)
		if err != nil {
			return nil, err
		}
		watched_files = append(watched_files, watched_file)
	}
	return watched_files, nil
}

func parse_source_watched_file(
	raw_watched_file json.RawMessage,
	field_path string,
) (source_watched_file, error) {
	watched_file_document, err := parse_json_object(
		raw_watched_file,
		field_path,
	)
	if err != nil {
		return source_watched_file{}, err
	}

	pattern, err := parse_required_string_field(
		watched_file_document,
		cfg.watch.include.entry.pattern.last(),
		dotsep(field_path, cfg.watch.include.entry.pattern.last()),
	)
	if err != nil {
		return source_watched_file{}, err
	}

	raw_on_change_hooks, ok := watched_file_document[cfg.watch.include.entry.on_change_hooks.node().last()]
	on_change_hooks := []source_on_change_hook(nil)
	if ok {
		on_change_hooks, err = parse_source_on_change_hook_list(
			raw_on_change_hooks,
			dotsep(
				field_path,
				cfg.watch.include.entry.on_change_hooks.node().last(),
			),
		)
		if err != nil {
			return source_watched_file{}, err
		}
	}

	recompile_go_binary, err := parse_optional_bool_field(
		watched_file_document,
		cfg.watch.include.entry.recompile_go_binary.last(),
		dotsep(field_path, cfg.watch.include.entry.recompile_go_binary.last()),
	)
	if err != nil {
		return source_watched_file{}, err
	}
	restart_app, err := parse_optional_bool_field(
		watched_file_document,
		cfg.watch.include.entry.restart_app.last(),
		dotsep(field_path, cfg.watch.include.entry.restart_app.last()),
	)
	if err != nil {
		return source_watched_file{}, err
	}
	only_run_client_defined_revalidate_func, err := parse_optional_bool_field(
		watched_file_document,
		cfg.watch.include.entry.only_run_client_defined_revalidate_func.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.only_run_client_defined_revalidate_func.last(),
		),
	)
	if err != nil {
		return source_watched_file{}, err
	}
	run_on_change_only, err := parse_optional_bool_field(
		watched_file_document,
		cfg.watch.include.entry.run_on_change_only.last(),
		dotsep(field_path, cfg.watch.include.entry.run_on_change_only.last()),
	)
	if err != nil {
		return source_watched_file{}, err
	}
	skip_rebuilding_notification, err := parse_optional_bool_field(
		watched_file_document,
		cfg.watch.include.entry.skip_rebuilding_notification.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.skip_rebuilding_notification.last(),
		),
	)
	if err != nil {
		return source_watched_file{}, err
	}
	treat_as_non_go, err := parse_optional_bool_field(
		watched_file_document,
		cfg.watch.include.entry.treat_as_non_go.last(),
		dotsep(field_path, cfg.watch.include.entry.treat_as_non_go.last()),
	)
	if err != nil {
		return source_watched_file{}, err
	}

	return source_watched_file{
		pattern:                                 pattern,
		on_change_hooks:                         on_change_hooks,
		recompile_go_binary:                     recompile_go_binary,
		restart_app:                             restart_app,
		only_run_client_defined_revalidate_func: only_run_client_defined_revalidate_func,
		run_on_change_only:                      run_on_change_only,
		skip_rebuilding_notification:            skip_rebuilding_notification,
		treat_as_non_go:                         treat_as_non_go,
	}, nil
}

func parse_source_on_change_hook_list(
	raw_on_change_hook_list json.RawMessage,
	field_path string,
) ([]source_on_change_hook, error) {
	var raw_on_change_hooks []json.RawMessage
	if err := json.Unmarshal(raw_on_change_hook_list, &raw_on_change_hooks); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array: %w", field_path, err)
	}

	on_change_hooks := make(
		[]source_on_change_hook,
		0,
		len(raw_on_change_hooks),
	)
	for on_change_hook_index, raw_on_change_hook := range raw_on_change_hooks {
		on_change_hook, err := parse_source_on_change_hook(
			raw_on_change_hook,
			fmt.Sprintf("%s[%d]", field_path, on_change_hook_index),
		)
		if err != nil {
			return nil, err
		}
		on_change_hooks = append(on_change_hooks, on_change_hook)
	}
	return on_change_hooks, nil
}

func parse_source_on_change_hook(
	raw_on_change_hook json.RawMessage,
	field_path string,
) (source_on_change_hook, error) {
	on_change_hook_document, err := parse_json_object(
		raw_on_change_hook,
		field_path,
	)
	if err != nil {
		return source_on_change_hook{}, err
	}

	cmd, err := parse_optional_string_field(
		on_change_hook_document,
		cfg.watch.include.entry.on_change_hooks.entry.cmd.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.on_change_hooks.entry.cmd.last(),
		),
	)
	if err != nil {
		return source_on_change_hook{}, err
	}
	command_timeout_ms, err := parse_optional_int_field(
		on_change_hook_document,
		cfg.watch.include.entry.on_change_hooks.entry.command_timeout_ms.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.on_change_hooks.entry.command_timeout_ms.last(),
		),
	)
	if err != nil {
		return source_on_change_hook{}, err
	}
	disable_stage_command_timeout, err := parse_optional_bool_field(
		on_change_hook_document,
		cfg.watch.include.entry.on_change_hooks.entry.disable_stage_command_timeout.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.on_change_hooks.entry.disable_stage_command_timeout.last(),
		),
	)
	if err != nil {
		return source_on_change_hook{}, err
	}
	callback_timeout_ms, err := parse_optional_int_field(
		on_change_hook_document,
		cfg.watch.include.entry.on_change_hooks.entry.callback_timeout_ms.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.on_change_hooks.entry.callback_timeout_ms.last(),
		),
	)
	if err != nil {
		return source_on_change_hook{}, err
	}
	disable_stage_callback_timeout, err := parse_optional_bool_field(
		on_change_hook_document,
		cfg.watch.include.entry.on_change_hooks.entry.disable_stage_callback_timeout.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.on_change_hooks.entry.disable_stage_callback_timeout.last(),
		),
	)
	if err != nil {
		return source_on_change_hook{}, err
	}
	run_combined_dev_build_hook_commands, err := parse_optional_bool_field(
		on_change_hook_document,
		cfg.watch.include.entry.on_change_hooks.entry.run_combined_dev_build_hook_commands.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.on_change_hooks.entry.run_combined_dev_build_hook_commands.last(),
		),
	)
	if err != nil {
		return source_on_change_hook{}, err
	}
	timing, err := parse_optional_string_field(
		on_change_hook_document,
		cfg.watch.include.entry.on_change_hooks.entry.timing.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.on_change_hooks.entry.timing.last(),
		),
	)
	if err != nil {
		return source_on_change_hook{}, err
	}
	exclude, err := parse_optional_string_list_field(
		on_change_hook_document,
		cfg.watch.include.entry.on_change_hooks.entry.exclude.last(),
		dotsep(
			field_path,
			cfg.watch.include.entry.on_change_hooks.entry.exclude.last(),
		),
	)
	if err != nil {
		return source_on_change_hook{}, err
	}

	return source_on_change_hook{
		cmd:                                  cmd,
		command_timeout_ms:                   command_timeout_ms,
		disable_stage_command_timeout:        disable_stage_command_timeout,
		callback_timeout_ms:                  callback_timeout_ms,
		disable_stage_callback_timeout:       disable_stage_callback_timeout,
		run_combined_dev_build_hook_commands: run_combined_dev_build_hook_commands,
		timing:                               timing,
		exclude:                              exclude,
	}, nil
}

func normalize_core_cfg(
	core_cfg source_core_cfg,
	effective_resolve_root string,
) (normalized_core_cfg, error) {
	main_app_entry, err := normalize_optional_relative_path_against_base(
		core_cfg.main_app_entry,
		effective_resolve_root,
		cfg.core.main_app_entry.full(),
	)
	if err != nil {
		return normalized_core_cfg{}, err
	}

	static_asset_dirs_private, err := normalize_optional_relative_path_against_base(
		core_cfg.static_asset_dirs_private,
		effective_resolve_root,
		cfg.core.static_asset_dirs.private.full(),
	)
	if err != nil {
		return normalized_core_cfg{}, err
	}

	static_asset_dirs_public, err := normalize_optional_relative_path_against_base(
		core_cfg.static_asset_dirs_public,
		effective_resolve_root,
		cfg.core.static_asset_dirs.public.full(),
	)
	if err != nil {
		return normalized_core_cfg{}, err
	}

	critical_css_entry_file, err := normalize_optional_relative_path_against_base(
		core_cfg.critical_css_entry_file,
		effective_resolve_root,
		cfg.core.css_entry_files.critical.full(),
	)
	if err != nil {
		return normalized_core_cfg{}, err
	}

	non_critical_css_entry_file, err := normalize_optional_relative_path_against_base(
		core_cfg.non_critical_css_entry_file,
		effective_resolve_root,
		cfg.core.css_entry_files.non_critical.full(),
	)
	if err != nil {
		return normalized_core_cfg{}, err
	}

	return normalized_core_cfg{
		dev_build_hook:              core_cfg.dev_build_hook,
		dev_build_hook_timeout_ms:   core_cfg.dev_build_hook_timeout_ms,
		prod_build_hook:             core_cfg.prod_build_hook,
		prod_build_hook_timeout_ms:  core_cfg.prod_build_hook_timeout_ms,
		main_app_entry:              main_app_entry,
		static_asset_dirs_private:   static_asset_dirs_private,
		static_asset_dirs_public:    static_asset_dirs_public,
		critical_css_entry_file:     critical_css_entry_file,
		non_critical_css_entry_file: non_critical_css_entry_file,
		public_path_prefix: normalize_public_path_prefix(
			core_cfg.public_path_prefix,
		),
		server_only_mode:    core_cfg.server_only_mode,
		sequential_go_build: core_cfg.sequential_go_build,
	}, nil
}

func normalize_vite_cfg(
	vite_cfg *source_vite_cfg,
	effective_resolve_root string,
) (*normalized_vite_cfg, error) {
	if vite_cfg == nil {
		return nil, nil
	}

	js_pkg_mgr_cmd_dir, err := normalize_optional_relative_path_against_base(
		vite_cfg.js_pkg_mgr_cmd_dir,
		effective_resolve_root,
		cfg.vite.js_pkg_mgr_cmd_dir.full(),
	)
	if err != nil {
		return nil, err
	}

	vite_cfg_file, err := normalize_optional_relative_path_against_base(
		vite_cfg.vite_cfg_file,
		effective_resolve_root,
		cfg.vite.vite_cfg_file.full(),
	)
	if err != nil {
		return nil, err
	}

	return &normalized_vite_cfg{
		js_pkg_mgr_base_cmd: vite_cfg.js_ppg_mgr_base_cmd,
		js_pkg_mgr_cmd_dir:  js_pkg_mgr_cmd_dir,
		default_port:        vite_cfg.default_port,
		vite_cfg_file:       vite_cfg_file,
	}, nil
}

func normalize_watch_cfg(
	watch_cfg *source_watch_cfg,
	effective_resolve_root string,
) (*normalized_watch_cfg, error) {
	if watch_cfg == nil {
		return nil, nil
	}

	include := make([]normalized_watched_file, 0, len(watch_cfg.include))
	for watch_index, watched_file := range watch_cfg.include {
		normalized_watched_file, err := normalize_watched_file(
			watched_file,
			effective_resolve_root,
			watch_index,
		)
		if err != nil {
			return nil, err
		}
		include = append(include, normalized_watched_file)
	}

	exclude_dirs, err := normalize_string_slice_relative_to_base(
		watch_cfg.exclude_dirs,
		effective_resolve_root,
		cfg.watch.exclude.dirs.full(),
	)
	if err != nil {
		return nil, err
	}

	exclude_files, err := normalize_string_slice_relative_to_base(
		watch_cfg.exclude_files,
		effective_resolve_root,
		cfg.watch.exclude.files.full(),
	)
	if err != nil {
		return nil, err
	}

	return &normalized_watch_cfg{
		healthcheck_endpoint: watch_cfg.healthcheck_endpoint,
		include:              include,
		exclude_dirs:         exclude_dirs,
		exclude_files:        exclude_files,
	}, nil
}

func normalize_watched_file(
	watched_file source_watched_file,
	effective_resolve_root string,
	watch_index int,
) (normalized_watched_file, error) {
	pattern, err := normalize_optional_relative_path_against_base(
		watched_file.pattern,
		effective_resolve_root,
		fmt.Sprintf(
			"%s[%d].%s",
			cfg.watch.include.node().full(),
			watch_index,
			cfg.watch.include.entry.pattern.last(),
		),
	)
	if err != nil {
		return normalized_watched_file{}, err
	}

	on_change_hooks := make(
		[]normalized_on_change_hook,
		0,
		len(watched_file.on_change_hooks),
	)
	for hook_index, on_change_hook := range watched_file.on_change_hooks {
		normalized_on_change_hook, err := normalize_on_change_hook(
			on_change_hook,
			effective_resolve_root,
			watch_index,
			hook_index,
		)
		if err != nil {
			return normalized_watched_file{}, err
		}
		on_change_hooks = append(on_change_hooks, normalized_on_change_hook)
	}

	return normalized_watched_file{
		pattern:                                 pattern,
		on_change_hooks:                         on_change_hooks,
		recompile_go_binary:                     watched_file.recompile_go_binary,
		restart_app:                             watched_file.restart_app,
		only_run_client_defined_revalidate_func: watched_file.only_run_client_defined_revalidate_func,
		run_on_change_only:                      watched_file.run_on_change_only,
		skip_rebuilding_notification:            watched_file.skip_rebuilding_notification,
		treat_as_non_go:                         watched_file.treat_as_non_go,
	}, nil
}

func normalize_on_change_hook(
	on_change_hook source_on_change_hook,
	effective_resolve_root string,
	watch_index int,
	hook_index int,
) (normalized_on_change_hook, error) {
	exclude, err := normalize_string_slice_relative_to_base(
		on_change_hook.exclude,
		effective_resolve_root,
		fmt.Sprintf(
			"%s[%d].%s[%d].%s",
			cfg.watch.include.node().full(),
			watch_index,
			cfg.watch.include.entry.on_change_hooks.node().last(),
			hook_index,
			cfg.watch.include.entry.on_change_hooks.entry.exclude.last(),
		),
	)
	if err != nil {
		return normalized_on_change_hook{}, err
	}

	return normalized_on_change_hook{
		cmd:                                  on_change_hook.cmd,
		command_timeout_ms:                   on_change_hook.command_timeout_ms,
		disable_stage_command_timeout:        on_change_hook.disable_stage_command_timeout,
		callback_timeout_ms:                  on_change_hook.callback_timeout_ms,
		disable_stage_callback_timeout:       on_change_hook.disable_stage_callback_timeout,
		run_combined_dev_build_hook_commands: on_change_hook.run_combined_dev_build_hook_commands,
		timing:                               on_change_hook.timing,
		exclude:                              exclude,
	}, nil
}

func normalize_string_slice_relative_to_base(
	values []string,
	base string,
	field_path string,
) ([]string, error) {
	normalized_values := make([]string, 0, len(values))
	for value_index, value := range values {
		normalized_value, err := normalize_optional_relative_path_against_base(
			value,
			base,
			fmt.Sprintf("%s[%d]", field_path, value_index),
		)
		if err != nil {
			return nil, err
		}
		normalized_values = append(normalized_values, normalized_value)
	}
	return normalized_values, nil
}

func normalize_optional_relative_path_against_base(
	raw_path string,
	base string,
	field_path string,
) (string, error) {
	trimmed_path := strings.TrimSpace(raw_path)
	if trimmed_path == "" {
		return "", nil
	}
	if filepath.IsAbs(trimmed_path) {
		return "", fmt.Errorf(
			"%s must be relative, got %q",
			field_path,
			raw_path,
		)
	}
	return filepath.Clean(filepath.Join(base, trimmed_path)), nil
}

func normalize_public_path_prefix(public_path_prefix string) string {
	trimmed_public_path_prefix := strings.TrimSpace(public_path_prefix)
	if trimmed_public_path_prefix == "" || trimmed_public_path_prefix == "/" {
		return "/"
	}
	if !strings.HasPrefix(trimmed_public_path_prefix, "/") {
		trimmed_public_path_prefix = "/" + trimmed_public_path_prefix
	}
	if !strings.HasSuffix(trimmed_public_path_prefix, "/") {
		trimmed_public_path_prefix += "/"
	}
	return trimmed_public_path_prefix
}

func parse_json_object(
	raw_object json.RawMessage,
	field_path string,
) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw_object, &object); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", field_path, err)
	}
	return object, nil
}

func parse_optional_object_field(
	document map[string]json.RawMessage,
	json_key string,
	field_path string,
) (map[string]json.RawMessage, error) {
	if document == nil {
		return nil, nil
	}
	raw_field, ok := document[json_key]
	if !ok {
		return nil, nil
	}
	return parse_json_object(raw_field, field_path)
}

func parse_required_string_field(
	document map[string]json.RawMessage,
	json_key string,
	field_path string,
) (string, error) {
	raw_field, ok := document[json_key]
	if !ok {
		return "", fmt.Errorf("%s is required", field_path)
	}

	var value string
	if err := json.Unmarshal(raw_field, &value); err != nil {
		return "", fmt.Errorf("%s must be a string: %w", field_path, err)
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", field_path)
	}
	return value, nil
}

func parse_optional_string_field(
	document map[string]json.RawMessage,
	json_key string,
	field_path string,
) (string, error) {
	if document == nil {
		return "", nil
	}
	raw_field, ok := document[json_key]
	if !ok {
		return "", nil
	}

	var value string
	if err := json.Unmarshal(raw_field, &value); err != nil {
		return "", fmt.Errorf("%s must be a string: %w", field_path, err)
	}
	return value, nil
}

func parse_optional_bool_field(
	document map[string]json.RawMessage,
	json_key string,
	field_path string,
) (bool, error) {
	if document == nil {
		return false, nil
	}
	raw_field, ok := document[json_key]
	if !ok {
		return false, nil
	}

	var value bool
	if err := json.Unmarshal(raw_field, &value); err != nil {
		return false, fmt.Errorf("%s must be a bool: %w", field_path, err)
	}
	return value, nil
}

func parse_optional_int_field(
	document map[string]json.RawMessage,
	json_key string,
	field_path string,
) (int, error) {
	if document == nil {
		return 0, nil
	}
	raw_field, ok := document[json_key]
	if !ok {
		return 0, nil
	}

	var value int
	if err := json.Unmarshal(raw_field, &value); err != nil {
		return 0, fmt.Errorf("%s must be an int: %w", field_path, err)
	}
	return value, nil
}

func parse_optional_string_list_field(
	document map[string]json.RawMessage,
	json_key string,
	field_path string,
) ([]string, error) {
	if document == nil {
		return nil, nil
	}
	raw_field, ok := document[json_key]
	if !ok {
		return nil, nil
	}

	var values []string
	if err := json.Unmarshal(raw_field, &values); err != nil {
		return nil, fmt.Errorf("%s must be a string array: %w", field_path, err)
	}
	return values, nil
}

/////////////////////////////////////////////////////////////////////
/////// Utils
/////////////////////////////////////////////////////////////////////

func dotsep(parts ...string) string {
	return strings.Join(parts, ".")
}
