package wavebuild

import (
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave2"
)

/////////////////////////////////////////////////////////////////////
/////// Canonical Dist Layout
/////////////////////////////////////////////////////////////////////

// Canonical dist layout:
//
//	.wavedist/
//	  main | main.exe
//	  static/
//	    .keep
//	    assets/
//	      public/
//	        ...
//	      private/
//	        ...
//	    wave_owned/
//	      runtime_cfg.json
//	      vite_manifest.json
//	      critical.css
//	      normal_css_file_ref.txt
//	      public_file_map_file_ref.txt
//	      public_filemap.gob
//	      private_filemap.gob
//	      build_output_ledger.txt
var dist = dist_tree{
	binary: dist_binary_tree{
		unix:    ".wavedist/main",
		windows: ".wavedist/main.exe",
	},
	static: dist_static_tree{
		keep: ".wavedist/static/.keep",
		assets: dist_assets_tree{
			public:  ".wavedist/static/assets/public",
			private: ".wavedist/static/assets/private",
		},
		wave_owned: dist_wave_owned_tree{
			runtime_cfg_json:             ".wavedist/static/" + wave2.RuntimeConfigPath,
			vite_manifest_json:           ".wavedist/static/wave_owned/vite_manifest.json",
			critical_css:                 ".wavedist/static/wave_owned/critical.css",
			normal_css_file_ref_txt:      ".wavedist/static/wave_owned/normal_css_file_ref.txt",
			public_file_map_file_ref_txt: ".wavedist/static/wave_owned/public_file_map_file_ref.txt",
			public_filemap_gob:           ".wavedist/static/wave_owned/public_filemap.gob",
			private_filemap_gob:          ".wavedist/static/wave_owned/private_filemap.gob",
			build_output_ledger_txt:      ".wavedist/static/wave_owned/build_output_ledger.txt",
		},
	},
}

/////////////////////////////////////////////////////////////////////
/////// Dist Types
/////////////////////////////////////////////////////////////////////

type dist_tree struct {
	binary dist_binary_tree
	static dist_static_tree
}

func (tree dist_tree) root() dir {
	return ".wavedist"
}

type dist_binary_tree struct {
	unix    file
	windows file
}

type dist_static_tree struct {
	keep       file
	assets     dist_assets_tree
	wave_owned dist_wave_owned_tree
}

func (tree dist_static_tree) path() dir {
	return ".wavedist/static"
}

type dist_assets_tree struct {
	public  dir
	private dir
}

type dist_wave_owned_tree struct {
	runtime_cfg_json             file
	vite_manifest_json           file
	critical_css                 file
	normal_css_file_ref_txt      file
	public_file_map_file_ref_txt file
	public_filemap_gob           file
	private_filemap_gob          file
	build_output_ledger_txt      file
}

/////////////////////////////////////////////////////////////////////
/////// Canonical Authored Cfg JSON
/////////////////////////////////////////////////////////////////////

// Canonical authored cfg shape:
//
//	{
//	  "Core": { ... },
//	  "Vite": { ... },
//	  "Watch": {
//	    "Include": [ { ... } ],
//	    "Exclude": { ... }
//	  }
//	}
var cfg = cfg_tree{
	core: cfg_core_tree{
		main_app_entry:             "Core.MainAppEntry",
		resolve_root:               "Core.ResolveRoot",
		dev_build_hook:             "Core.DevBuildHook",
		dev_build_hook_timeout_ms:  "Core.DevBuildHookTimeoutMs",
		prod_build_hook:            "Core.ProdBuildHook",
		prod_build_hook_timeout_ms: "Core.ProdBuildHookTimeoutMs",
		public_path_prefix:         "Core.PublicPathPrefix",
		server_only_mode:           "Core.ServerOnlyMode",
		sequential_go_build:        "Core.SequentialGoBuild",
		static_asset_dirs: cfg_core_static_asset_dirs_tree{
			private: "Core.StaticAssetDirs.Private",
			public:  "Core.StaticAssetDirs.Public",
		},
		css_entry_files: cfg_core_css_entry_files_tree{
			critical:     "Core.CSSEntryFiles.Critical",
			non_critical: "Core.CSSEntryFiles.NonCritical",
		},
	},
	vite: cfg_vite_tree{
		js_pkg_mgr_base_cmd: "Vite.JSPackageManagerBaseCmd",
		js_pkg_mgr_cmd_dir:  "Vite.JSPackageManagerCmdDir",
		default_port:        "Vite.DefaultPort",
		vite_cfg_file:       "Vite.ViteConfigFile",
	},
	watch: cfg_watch_tree{
		healthcheck_endpoint: "Watch.HealthcheckEndpoint",
		include: cfg_watch_include_list_tree{
			entry: cfg_watch_include_entry_tree{
				pattern:                                 "Watch.Include[].Pattern",
				recompile_go_binary:                     "Watch.Include[].RecompileGoBinary",
				restart_app:                             "Watch.Include[].RestartApp",
				only_run_client_defined_revalidate_func: "Watch.Include[].OnlyRunClientDefinedRevalidateFunc",
				run_on_change_only:                      "Watch.Include[].RunOnChangeOnly",
				skip_rebuilding_notification:            "Watch.Include[].SkipRebuildingNotification",
				treat_as_non_go:                         "Watch.Include[].TreatAsNonGo",
				on_change_hooks: cfg_watch_on_change_hooks_list_tree{
					entry: cfg_watch_on_change_hook_entry_tree{
						cmd:                                  "Watch.Include[].OnChangeHooks[].Cmd",
						command_timeout_ms:                   "Watch.Include[].OnChangeHooks[].CommandTimeoutMs",
						disable_stage_command_timeout:        "Watch.Include[].OnChangeHooks[].DisableStageCommandTimeout",
						callback_timeout_ms:                  "Watch.Include[].OnChangeHooks[].CallbackTimeoutMs",
						disable_stage_callback_timeout:       "Watch.Include[].OnChangeHooks[].DisableStageCallbackTimeout",
						run_combined_dev_build_hook_commands: "Watch.Include[].OnChangeHooks[].RunCombinedDevBuildHookCommands",
						timing:                               "Watch.Include[].OnChangeHooks[].Timing",
						exclude:                              "Watch.Include[].OnChangeHooks[].Exclude",
					},
				},
			},
		},
		exclude: cfg_watch_exclude_tree{
			dirs:  "Watch.Exclude.Dirs",
			files: "Watch.Exclude.Files",
		},
	},
}

/////////////////////////////////////////////////////////////////////
/////// Cfg Types
/////////////////////////////////////////////////////////////////////

type cfg_tree struct {
	core  cfg_core_tree
	vite  cfg_vite_tree
	watch cfg_watch_tree
}

type cfg_core_tree struct {
	main_app_entry             json_key
	static_asset_dirs          cfg_core_static_asset_dirs_tree
	css_entry_files            cfg_core_css_entry_files_tree
	resolve_root               json_key
	dev_build_hook             json_key
	dev_build_hook_timeout_ms  json_key
	prod_build_hook            json_key
	prod_build_hook_timeout_ms json_key
	public_path_prefix         json_key
	server_only_mode           json_key
	sequential_go_build        json_key
}

func (tree cfg_core_tree) node() json_key {
	return "Core"
}

type cfg_core_static_asset_dirs_tree struct {
	private json_key
	public  json_key
}

func (tree cfg_core_static_asset_dirs_tree) node() json_key {
	return "Core.StaticAssetDirs"
}

type cfg_core_css_entry_files_tree struct {
	critical     json_key
	non_critical json_key
}

func (tree cfg_core_css_entry_files_tree) node() json_key {
	return "Core.CSSEntryFiles"
}

type cfg_vite_tree struct {
	js_pkg_mgr_base_cmd json_key
	js_pkg_mgr_cmd_dir  json_key
	default_port        json_key
	vite_cfg_file       json_key
}

func (tree cfg_vite_tree) node() json_key {
	return "Vite"
}

type cfg_watch_tree struct {
	healthcheck_endpoint json_key
	include              cfg_watch_include_list_tree
	exclude              cfg_watch_exclude_tree
}

func (tree cfg_watch_tree) node() json_key {
	return "Watch"
}

type cfg_watch_include_list_tree struct {
	entry cfg_watch_include_entry_tree
}

func (tree cfg_watch_include_list_tree) node() json_key {
	return "Watch.Include"
}

type cfg_watch_include_entry_tree struct {
	pattern                                 json_key
	on_change_hooks                         cfg_watch_on_change_hooks_list_tree
	recompile_go_binary                     json_key
	restart_app                             json_key
	only_run_client_defined_revalidate_func json_key
	run_on_change_only                      json_key
	skip_rebuilding_notification            json_key
	treat_as_non_go                         json_key
}

type cfg_watch_on_change_hooks_list_tree struct {
	entry cfg_watch_on_change_hook_entry_tree
}

func (tree cfg_watch_on_change_hooks_list_tree) node() json_key {
	return "Watch.Include[].OnChangeHooks"
}

type cfg_watch_on_change_hook_entry_tree struct {
	cmd                                  json_key
	command_timeout_ms                   json_key
	disable_stage_command_timeout        json_key
	callback_timeout_ms                  json_key
	disable_stage_callback_timeout       json_key
	run_combined_dev_build_hook_commands json_key
	timing                               json_key
	exclude                              json_key
}

type cfg_watch_exclude_tree struct {
	dirs  json_key
	files json_key
}

func (tree cfg_watch_exclude_tree) node() json_key {
	return "Watch.Exclude"
}

/////////////////////////////////////////////////////////////////////
/////// Generic Helper Types
/////////////////////////////////////////////////////////////////////

type file = _file_path
type dir = _file_path

// don't use _file_path directly, use file or dir for semantic clarity instead
type _file_path string

type json_key string

func (p _file_path) full() string {
	return string(p)
}

func (p _file_path) last() string {
	as_slash := filepath.ToSlash(string(p))
	last_slash_index := strings.LastIndex(as_slash, "/")
	if last_slash_index == -1 {
		return as_slash
	}
	return as_slash[last_slash_index+1:]
}

func (k json_key) full() string {
	return string(k)
}

func (k json_key) last() string {
	full_key := string(k)
	last_dot_index := strings.LastIndex(full_key, ".")
	if last_dot_index == -1 {
		return full_key
	}
	return full_key[last_dot_index+1:]
}
