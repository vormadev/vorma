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
