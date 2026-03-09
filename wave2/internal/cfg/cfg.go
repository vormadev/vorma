package cfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/strict"
)

// __TODO ensure that the private static dir is not a child of the public static dir

/* INVARIANTS:

`Core.ResolveRoot` must be relative to the dir in which the config file lives
(if not set, `Core.ResolveRoot` defaults to ".", which would be the dir in
which the config file lives).

All other config filepaths are resolved against `Core.ResolveRoot`.

All filepath values may escape the directory against which they are resolved,
but they must not be machine-absolute. If any filepath value is set by the user
as machine-absolute, parsing must fail.

*/

/////////////////////////////////////////////////////////////////////
/////// Raw Incoming JSON Config
/////////////////////////////////////////////////////////////////////

type Raw struct {
	ConfigPath strict.CWDRelPath `json:"-"`
	Core       RawCore           `json:"Core"`
	Vite       RawVite           `json:"Vite"`
	Watch      RawWatch          `json:"Watch"`
}

type RawCore struct {
	/////// universal

	ResolveRoot       string
	MainAppEntry      string
	DevBuildHook      string
	ProdBuildHook     string
	ServerOnlyMode    bool
	SequentialGoBuild bool

	/////// full-stack only

	PublicPathPrefix string
	StaticAssetDirs  struct {
		Private string
		Public  string
	}
	CSSEntryFiles struct {
		Critical    string
		NonCritical string
	}
}

type RawVite struct {
	JSPackageManagerBaseCmd string
	JSPackageManagerCmdDir  string
	DefaultPort             int
	ViteConfigFile          string
}

type RawWatch struct {
	HealthcheckEndpoint string
	Include             []struct {
		Pattern       string // glob
		OnChangeHooks []struct {
			Cmd     string
			Timing  string   // pre, concurrent, concurrent-no-wait, post
			Exclude []string // glob
		}
		RecompileGoBinary                  bool
		RestartApp                         bool
		OnlyRunClientDefinedRevalidateFunc bool
		RunOnChangeOnly                    bool
		SkipRebuildingNotification         bool
		TreatAsNonGo                       bool
	}
	Exclude []string // glob
}

/////////////////////////////////////////////////////////////////////
/////// Parsed Config (validated and normalized)
/////////////////////////////////////////////////////////////////////

type Parsed struct {
	ConfigPath strict.CWDRelPath
	Core       ParsedCore
	Vite       ParsedVite
	Watch      ParsedWatch
}

/////// CORE

type ParsedCore struct {
	MainAppEntry        strict.CWDRelPath
	PublicPathPrefix    strict.URLPathPrefix
	StaticPrivateDir    strict.CWDRelPath
	StaticPublicDir     strict.CWDRelPath
	CriticalCSSEntry    strict.CWDRelPath
	NonCriticalCSSEntry strict.CWDRelPath
	ServerOnlyMode      bool
	SequentialGoBuild   bool
	DevBuildHook        strict.Cmd
	ProdBuildHook       strict.Cmd
}

/////// VITE

type ParsedVite struct {
	JSPackageManagerBaseCmd string
	JSPackageManagerCmdDir  strict.CWDRelPath
	DefaultPort             strict.Port
	ViteConfigFile          strict.CWDRelPath
}

/////// WATCH

type ParsedWatch struct {
	HealthcheckEndpoint string
	Include             []ParsedWatchIncludeEntry
	Exclude             []strict.CWDRelPath
}

type ParsedWatchIncludeEntry struct {
	Pattern                            strict.CWDRelPath
	OnChangeHooks                      []ParsedOnChangeHook
	RecompileGoBinary                  bool
	RestartApp                         bool
	OnlyRunClientDefinedRevalidateFunc bool
	RunOnChangeOnly                    bool
	SkipRebuildingNotification         bool
	TreatAsNonGo                       bool
}

type ParsedOnChangeHook struct {
	Cmd     strict.Cmd
	Timing  OnChangeHookTiming
	Exclude []strict.CWDRelPath
}

type OnChangeHookTiming string

const (
	OnChangeHookTimingPre              OnChangeHookTiming = "pre"
	OnChangeHookTimingConcurrent       OnChangeHookTiming = "concurrent"
	OnChangeHookTimingConcurrentNoWait OnChangeHookTiming = "concurrent-no-wait"
	OnChangeHookTimingPost             OnChangeHookTiming = "post"
)

func ConfigPathToRaw(path string) *Raw {
	cfg_path := parse_path(path, "config file path")
	assert_is_file(cfg_path, "config file path")

	json_bytes, err := os.ReadFile(string(cfg_path))
	if err != nil {
		panic("config: error reading config file: " + err.Error())
	}
	if !json.Valid(json_bytes) {
		panic("config: config file is not valid JSON")
	}

	var raw Raw
	if err := json.Unmarshal(json_bytes, &raw); err != nil {
		panic("config: error parsing config JSON: " + err.Error())
	}

	raw.ConfigPath = cfg_path
	return &raw
}

func RawToParsed(raw *Raw) *Parsed {
	if raw == nil {
		panic("config: raw config is nil")
	}

	var parsed Parsed
	var reserved reserved_paths

	// ConfigPath
	// __TODO add test case for this
	parsed.ConfigPath = raw.ConfigPath

	// Core.ResolveRoot
	// this resolves against config dir
	// everything else resolves against this
	resolve_root := strict.CWDRelPath(filepath.Dir(string(parsed.ConfigPath))).
		Join(
			parse_path(raw.Core.ResolveRoot, "Core.ResolveRoot"),
		)
	assert_is_dir(resolve_root, "Core.ResolveRoot")

	// Core.MainAppEntry
	parsed.Core.MainAppEntry = resolve_root.Join(
		parse_path(raw.Core.MainAppEntry, "Core.MainAppEntry"),
	)
	assert_is_file(parsed.Core.MainAppEntry, "Core.MainAppEntry")
	reserved.add(parsed.Core.MainAppEntry)

	// Core.DevBuildHook
	parsed.Core.DevBuildHook = strict.Cmd(
		strings.TrimSpace(raw.Core.DevBuildHook),
	)
	// Core.ProdBuildHook
	parsed.Core.ProdBuildHook = strict.Cmd(
		strings.TrimSpace(raw.Core.ProdBuildHook),
	)
	// Core.ServerOnlyMode
	parsed.Core.ServerOnlyMode = raw.Core.ServerOnlyMode
	// Core.SequentialGoBuild
	parsed.Core.SequentialGoBuild = raw.Core.SequentialGoBuild

	if !parsed.Core.ServerOnlyMode {
		// Core.PublicPathPrefix (not a file path)
		// must start and end with "/"
		public_path_prefix := raw.Core.PublicPathPrefix
		public_path_prefix = strings.TrimSpace(public_path_prefix)
		public_path_prefix = matcher.EnsureLeadingAndTrailingSlash(
			public_path_prefix,
		)
		parsed.Core.PublicPathPrefix = strict.URLPathPrefix(public_path_prefix)

		// Core.StaticAssetDirs.Private
		parsed.Core.StaticPrivateDir = resolve_root.Join(
			parse_path(
				raw.Core.StaticAssetDirs.Private,
				"Core.StaticAssetDirs.Private",
			),
		)
		assert_is_dir(
			parsed.Core.StaticPrivateDir,
			"Core.StaticAssetDirs.Private",
		)
		reserved.add(parsed.Core.StaticPrivateDir)

		// Core.StaticAssetDirs.Public
		parsed.Core.StaticPublicDir = resolve_root.Join(
			parse_path(
				raw.Core.StaticAssetDirs.Public,
				"Core.StaticAssetDirs.Public",
			),
		)
		assert_is_dir(
			parsed.Core.StaticPublicDir,
			"Core.StaticAssetDirs.Public",
		)
		reserved.add(parsed.Core.StaticPublicDir)

		// Core.CSSEntryFiles.Critical
		parsed.Core.CriticalCSSEntry = resolve_root.Join(
			parse_path(
				raw.Core.CSSEntryFiles.Critical,
				"Core.CSSEntryFiles.Critical",
			),
		)
		assert_is_file(
			parsed.Core.CriticalCSSEntry,
			"Core.CSSEntryFiles.Critical",
		)
		reserved.add(parsed.Core.CriticalCSSEntry)

		// Core.CSSEntryFiles.NonCritical
		parsed.Core.NonCriticalCSSEntry = resolve_root.Join(
			parse_path(
				raw.Core.CSSEntryFiles.NonCritical,
				"Core.CSSEntryFiles.NonCritical",
			),
		)
		assert_is_file(
			parsed.Core.NonCriticalCSSEntry,
			"Core.CSSEntryFiles.NonCritical",
		)
		reserved.add(parsed.Core.NonCriticalCSSEntry)
	}

	using_vite := !reflect.DeepEqual(RawVite{}, raw.Vite)
	if using_vite {
		// Vite.JSPackageManagerBaseCmd
		parsed.Vite.JSPackageManagerBaseCmd = strings.TrimSpace(
			raw.Vite.JSPackageManagerBaseCmd,
		)
		if parsed.Vite.JSPackageManagerBaseCmd == "" {
			panic("config: Vite.JSPackageManagerBaseCmd is required")
		}

		// Vite.JSPackageManagerCmdDir
		parsed.Vite.JSPackageManagerCmdDir = resolve_root.Join(
			parse_path(
				raw.Vite.JSPackageManagerCmdDir,
				"Vite.JSPackageManagerCmdDir",
			),
		)
		assert_is_dir(
			parsed.Vite.JSPackageManagerCmdDir,
			"Vite.JSPackageManagerCmdDir",
		)

		// Vite.DefaultPort
		if raw.Vite.DefaultPort != 0 {
			if raw.Vite.DefaultPort <= 0 ||
				raw.Vite.DefaultPort > 65535 {
				panic("config: Vite.DefaultPort must be between 1 and 65535")
			}
			parsed.Vite.DefaultPort = strict.Port(raw.Vite.DefaultPort)
		} else {
			parsed.Vite.DefaultPort = 5173
		}

		// Vite.ViteConfigFile
		if raw.Vite.ViteConfigFile != "" {
			parsed.Vite.ViteConfigFile = resolve_root.Join(
				parse_path(raw.Vite.ViteConfigFile, "Vite.ViteConfigFile"),
			)
			assert_is_file(parsed.Vite.ViteConfigFile, "Vite.ViteConfigFile")
			reserved.add(parsed.Vite.ViteConfigFile)
		}
	}

	// Watch.HealthcheckEndpoint
	parsed.Watch.HealthcheckEndpoint = matcher.EnsureLeadingSlash(
		strings.TrimSpace(
			raw.Watch.HealthcheckEndpoint,
		),
	)

	// Watch.Include
	parsed.Watch.Include = make(
		[]ParsedWatchIncludeEntry,
		len(raw.Watch.Include),
	)
	for i, include := range raw.Watch.Include {
		// Watch.Include[i].Pattern
		pattern := parse_path(
			include.Pattern,
			fmt.Sprintf("Watch.Include[%d].Pattern", i),
		)
		assert_valid_pattern(pattern, fmt.Sprintf(
			"config: Watch.Include[%d].Pattern: %s",
			i,
			pattern,
		))
		parsed.Watch.Include[i].Pattern = resolve_root.Join(pattern)

		// Watch.Include[i].OnChangeHooks
		parsed.Watch.Include[i].OnChangeHooks = make(
			[]ParsedOnChangeHook,
			len(include.OnChangeHooks),
		)
		for j, hook := range include.OnChangeHooks {
			// Watch.Include[i].OnChangeHooks[j].Cmd
			parsed.Watch.Include[i].OnChangeHooks[j].Cmd = strict.Cmd(
				strings.TrimSpace(hook.Cmd),
			)
			if parsed.Watch.Include[i].OnChangeHooks[j].Cmd == "" {
				panic(
					fmt.Sprintf(
						"config: Watch.Include[%d].OnChangeHooks[%d].Cmd is required",
						i,
						j,
					),
				)
			}

			// Watch.Include[i].OnChangeHooks[j].Timing
			timing := OnChangeHookTiming(
				strings.ToLower(strings.TrimSpace(hook.Timing)),
			)
			if timing == "" {
				timing = OnChangeHookTimingPre
			}
			switch timing {
			case OnChangeHookTimingPre, OnChangeHookTimingConcurrent,
				OnChangeHookTimingConcurrentNoWait, OnChangeHookTimingPost:
				parsed.Watch.Include[i].OnChangeHooks[j].Timing = OnChangeHookTiming(
					timing,
				)
			default:
				panic(
					fmt.Sprintf(
						"config: Watch.Include[%d].OnChangeHooks[%d].Timing must be one of %q, %q, %q, or %q: got %q",
						i,
						j,
						OnChangeHookTimingPre,
						OnChangeHookTimingConcurrent,
						OnChangeHookTimingConcurrentNoWait,
						OnChangeHookTimingPost,
						timing,
					),
				)
			}

			// Watch.Include[i].OnChangeHooks[j].Exclude
			for k, _exclude := range hook.Exclude {
				exclude := parse_path(
					_exclude,
					fmt.Sprintf(
						"Watch.Include[%d].OnChangeHooks[%d].Exclude[%d]",
						i,
						j,
						k,
					),
				)
				assert_valid_pattern(exclude, fmt.Sprintf(
					"config: Watch.Include[%d].OnChangeHooks[%d].Exclude[%d]: %s",
					i,
					j,
					k,
					exclude,
				))
				parsed.Watch.Include[i].OnChangeHooks[j].Exclude = append(
					parsed.Watch.Include[i].OnChangeHooks[j].Exclude,
					resolve_root.Join(exclude),
				)
			}
		}

		// Watch.Include[i].RecompileGoBinary
		parsed.Watch.Include[i].RecompileGoBinary = include.RecompileGoBinary

		// Watch.Include[i].RestartApp
		parsed.Watch.Include[i].RestartApp = include.RestartApp

		// Watch.Include[i].OnlyRunClientDefinedRevalidateFunc
		parsed.Watch.Include[i].OnlyRunClientDefinedRevalidateFunc = include.OnlyRunClientDefinedRevalidateFunc

		// Watch.Include[i].RunOnChangeOnly
		parsed.Watch.Include[i].RunOnChangeOnly = include.RunOnChangeOnly

		// Watch.Include[i].SkipRebuildingNotification
		parsed.Watch.Include[i].SkipRebuildingNotification = include.SkipRebuildingNotification

		// Watch.Include[i].TreatAsNonGo
		parsed.Watch.Include[i].TreatAsNonGo = include.TreatAsNonGo
	}

	// Watch.Exclude
	for i, _exclude := range raw.Watch.Exclude {
		exclude := parse_path(_exclude, fmt.Sprintf("Watch.Exclude[%d]", i))
		assert_valid_pattern(
			exclude,
			fmt.Sprintf("config: Watch.Exclude[%d]: %s", i, exclude),
		)
		parsed.Watch.Exclude = append(
			parsed.Watch.Exclude,
			resolve_root.Join(
				parse_path(exclude, fmt.Sprintf("Watch.Exclude[%d]", i)),
			),
		)
	}

	return &parsed
}

/////////////////////////////////////////////////////////////////////
/////// Utils
/////////////////////////////////////////////////////////////////////

type reserved_paths struct{ paths []strict.CWDRelPath }

func (r *reserved_paths) add(dir strict.CWDRelPath) {
	if slices.Contains(r.paths, dir) {
		panic("config: path already used: " + string(dir))
	}
	r.paths = append(r.paths, dir)
}

func is_dir(path strict.CWDRelPath, label string) bool {
	info, err := os.Stat(string(path))
	if err != nil {
		if os.IsNotExist(err) {
			panic("config: path does not exist: " + label)
		}
		panic("config: os.Stat error: " + label + ": " + err.Error())
	}
	return info.IsDir()
}

func assert_is_dir(value strict.CWDRelPath, label string) {
	if !is_dir(value, label) {
		panic("config: unexpected non-directory path: " + label)
	}
}

func assert_is_file(value strict.CWDRelPath, label string) {
	if is_dir(value, label) {
		panic("config: unexpected directory path: " + label)
	}
}

func assert_valid_pattern(pattern strict.CWDRelPath, label string) {
	if !doublestar.ValidatePathPattern(string(pattern)) {
		panic("config: invalid glob pattern: " + label)
	}
}

func parse_path(path any, label string) strict.CWDRelPath {
	switch p := path.(type) {
	case string, strict.CWDRelPath:
		path_str := fmt.Sprint(p)
		if filepath.IsAbs(path_str) {
			panic("config: unexpected absolute path: " + label)
		}
		// __TODO test that all paths are os-specific
		return strict.CWDRelPath(filepath.FromSlash(
			filepath.Clean(strings.TrimSpace(fmt.Sprint(p))),
		))
	default:
		panic("config: unexpected type in clean_trim")
	}
}
