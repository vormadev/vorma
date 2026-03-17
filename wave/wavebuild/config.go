package wavebuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
	"github.com/vormadev/vorma/wave/internal/constants"
)

/////////////////////////////////////////////////////////////////////
/////// Config structs
/////////////////////////////////////////////////////////////////////

type unsafe_config struct {
	ConfigPath    strict.CWDRelPath `json:"-"`
	raw_file_json []byte            `json:"-"`

	// The root directory to watch for file changes
	// and against which all other paths and patterns
	// in the config are resolved. Set this path
	// relative to the current working directory of
	// the underlying OS process.
	RootDir        strict.CWDRelPath
	Core           unsafe_core
	Vite           unsafe_vite
	LifecycleHooks []LifecycleHook
}

type validated_config struct {
	config_path     strict.CWDRelPath
	raw_file_json   []byte
	root_dir        strict.CWDRelPath
	root_dir_abs    strict.MachAbsPath
	core            validated_core
	vite            validated_vite
	lifecycle_hooks []validated_lifecycle_hook
}

type validated_core struct {
	unsafe_core
	binary_output_path_abs strict.MachAbsPath
}
type validated_vite struct{ unsafe_vite }

/////////////////////////////////////////////////////////////////////
/////// Config helpers
/////////////////////////////////////////////////////////////////////

func (c *validated_config) using_private_static() bool {
	return c.core.StaticAssetDirs.Private != ""
}

func (c *validated_config) using_public_static() bool {
	return c.core.StaticAssetDirs.Public != ""
}

func (c *validated_config) using_critical_css() bool {
	return c.core.CSSEntryFiles.Critical != ""
}

func (c *validated_config) using_non_critical_css() bool {
	return c.core.CSSEntryFiles.NonCritical != ""
}

// panics if no private static dir configured. check `using_private_static()` first.
func (c *validated_config) private_dir_pattern() strict.CWDRelPath {
	if !c.using_private_static() {
		panic("private_dir_pattern called but no private static dir configured")
	}
	return to_catch_all_pattern(c.core.StaticAssetDirs.Private)
}

// panics if no public static dir configured. check `using_public_static()` first.
func (c *validated_config) public_dir_pattern() strict.CWDRelPath {
	if !c.using_public_static() {
		panic("public_dir_pattern called but no public static dir configured")
	}
	return to_catch_all_pattern(c.core.StaticAssetDirs.Public)
}

/////////////////////////////////////////////////////////////////////
/////// Core / Vite structs
/////////////////////////////////////////////////////////////////////

type unsafe_core struct {
	HealthcheckEndpoint        string
	GlobalWatchExcludePatterns []strict.CWDRelPath

	// Name of the compiled binary (e.g. "myapp"). Wave uses this
	// to derive the output path (.waveout/<n>) and exposes it
	// to hooks via the constants.ENV_KEY_BIN_OUTPUT_PATH environment variable.
	BinaryName string

	ServerOnlyMode bool

	/////// full-stack only
	// All four fields are independently optional.

	PublicPathPrefix string
	StaticAssetDirs  unsafe_static_asset_dirs
	CSSEntryFiles    unsafe_css_entry_files
}

type unsafe_static_asset_dirs struct {
	Private strict.CWDRelPath
	Public  strict.CWDRelPath
}

type unsafe_css_entry_files struct {
	Critical    strict.CWDRelPath
	NonCritical strict.CWDRelPath
}

type unsafe_vite struct {
	UsingVite               bool `json:"-"`
	JSPackageManagerBaseCmd string
	JSPackageManagerCmdDir  strict.CWDRelPath
	DefaultPort             strict.Port
	ViteConfigFile          *strict.CWDRelPath
}

type LifecycleHook struct {
	// Optional human-readable name for this hook, used in build logs.
	// If not set, defaults to "lifecycle hook index N".
	Name string

	WatchIncludePatterns []strict.CWDRelPath
	WatchExcludePatterns []strict.CWDRelPath

	Cmd string
	Fn  func(ctx *PluginCtx) (*PluginResult, error) `json:"-"`

	IsGoCompile bool

	StartAt  Checkpoint
	FinishBy Checkpoint

	// Effects are the effects this hook causes in the current build cycle.
	Effects []Effect

	DevOnly  bool
	ProdOnly bool
}

// config_path_to_validated_config parses and validates a config file.
// It does not create directories or write any files.
func config_path_to_validated_config(
	config_path strict.CWDRelPath,
) (*validated_config, error) {
	raw, err := config_path_to_unsafe_config(config_path)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return validate_config(raw)
}

func check_file(p strict.CWDRelPath, label string) error {
	is_file, err := p.IsFile()
	if err != nil {
		return fmt.Errorf("%s %q: %w", label, p, err)
	}
	if !is_file {
		return fmt.Errorf("%s %q is not a file", label, p)
	}
	return nil
}
func check_dir(p strict.CWDRelPath, label string) error {
	is_dir, err := p.IsDir()
	if err != nil {
		return fmt.Errorf("%s %q: %w", label, p, err)
	}
	if !is_dir {
		return fmt.Errorf("%s %q is not a directory", label, p)
	}
	return nil
}

func config_path_to_unsafe_config(
	_config_path strict.CWDRelPath,
) (*unsafe_config, error) {
	cfg_path := strict.MustNormalizeCWDRelPath(_config_path)
	if err := check_file(cfg_path, "config path"); err != nil {
		return nil, err
	}

	json_bytes, err := os.ReadFile(cfg_path.Str())
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	if !json.Valid(json_bytes) {
		return nil, fmt.Errorf("config file is not valid JSON")
	}

	var raw unsafe_config
	if err := json.Unmarshal(json_bytes, &raw); err != nil {
		return nil, fmt.Errorf("parsing config JSON: %w", err)
	}

	raw.ConfigPath = cfg_path
	raw.raw_file_json = json_bytes
	return &raw, nil
}

func has_waveout_conflict(p strict.CWDRelPath) bool {
	return slices.Contains(
		matcher.ParseSegments(p.Str()),
		constants.DIST_DIRNAME,
	)
}

func validate_config(__raw *unsafe_config) (*validated_config, error) {
	if __raw == nil {
		return nil, fmt.Errorf("nil unsafe config")
	}

	vc := &validated_config{raw_file_json: __raw.raw_file_json}

	var err error
	var reserved reserved_paths
	waveout_dir := __raw.ConfigPath.MustNormalize().Dir().Join(
		constants.DIST_DIRNAME,
	)

	check_no_waveout_conflict := func(
		p strict.CWDRelPath,
		label string,
	) error {
		if check_overlap(p, waveout_dir) || has_waveout_conflict(p) {
			return fmt.Errorf(
				"%s %q overlaps with %q, which is reserved by Wave for build output. Please choose a different path.",
				label,
				p,
				waveout_dir,
			)
		}
		return nil
	}

	var check_file_and_reserve = func(p strict.CWDRelPath, label string) error {
		if err := check_file(p, label); err != nil {
			return err
		}
		return reserved.add(p, label)
	}
	var check_dir_and_reserve = func(p strict.CWDRelPath, label string) error {
		if err := check_dir(p, label); err != nil {
			return err
		}
		return reserved.add(p, label)
	}
	var join_check_and_reserve_file = func(_p strict.CWDRelPath, label string) (strict.CWDRelPath, error) {
		p := vc.root_dir.Join(_p.MustNormalize().Str())
		if err := check_no_waveout_conflict(p, label); err != nil {
			return p, err
		}
		return p, check_file_and_reserve(p, label)
	}
	var join_check_and_reserve_dir = func(_p strict.CWDRelPath, label string) (strict.CWDRelPath, error) {
		p := vc.root_dir.Join(_p.MustNormalize().Str())
		if err := check_no_waveout_conflict(p, label); err != nil {
			return p, err
		}
		return p, check_dir_and_reserve(p, label)
	}
	var join_and_check_dir = func(_p strict.CWDRelPath, label string) (strict.CWDRelPath, error) {
		p := vc.root_dir.Join(_p.MustNormalize().Str())
		if err := check_no_waveout_conflict(p, label); err != nil {
			return p, err
		}
		return p, check_dir(p, label)
	}

	/////// Top level

	vc.config_path = __raw.ConfigPath.MustNormalize()
	if err := check_file_and_reserve(vc.config_path, "ConfigPath"); err != nil {
		return nil, err
	}
	vc.root_dir = vc.config_path.Dir().Join(__raw.RootDir.MustNormalize().Str())
	if !check_within_or_equal(vc.root_dir, vc.config_path) {
		return nil, fmt.Errorf(
			"ConfigPath %q must be within or equal to RootDir %q",
			vc.root_dir,
			vc.config_path,
		)
	}
	if err := check_dir_and_reserve(vc.root_dir, "RootDir"); err != nil {
		return nil, err
	}
	// pre-compute this because we pass it to env
	vc.root_dir_abs = vc.root_dir.MustAbs()

	/////// Core

	vc.core.HealthcheckEndpoint = strings.TrimSpace(
		__raw.Core.HealthcheckEndpoint,
	)
	if vc.core.HealthcheckEndpoint == "" {
		vc.core.HealthcheckEndpoint = "/"
	}

	vc.core.GlobalWatchExcludePatterns = make(
		[]strict.CWDRelPath,
		len(__raw.Core.GlobalWatchExcludePatterns),
	)
	for i, pattern := range __raw.Core.GlobalWatchExcludePatterns {
		vc.core.GlobalWatchExcludePatterns[i] = to_catch_all_pattern_if_dir(
			vc.root_dir.Join(pattern.MustNormalize().Str()),
		)
	}

	vc.core.BinaryName = strings.TrimSpace(__raw.Core.BinaryName)
	if vc.core.BinaryName == "" {
		return nil, fmt.Errorf("BinaryName is required")
	}
	if strings.ContainsAny(vc.core.BinaryName, "/\\") {
		return nil, fmt.Errorf(
			"BinaryName must be a plain name, not a path: %q",
			__raw.Core.BinaryName,
		)
	}
	reserved_names := []string{ // anything we ouput directly into .waveout/
		constants.STATIC_DIRNAME,
		constants.SCHEMA_JSON_FILENAME,
		constants.WAVE_LOCK_FILENAME,
		constants.APP_PID_FILENAME,
		constants.VITE_PID_FILENAME,
	}
	for _, reserved := range reserved_names {
		if vc.core.BinaryName == reserved {
			return nil, fmt.Errorf(
				"BinaryName cannot be %q because it is reserved by Wave",
				reserved,
			)
		}
	}
	if filepath.Ext(vc.core.BinaryName) != ".exe" && runtime.GOOS == "windows" {
		vc.core.BinaryName += ".exe"
	}
	bin_out_cwd := vc.config_path.Dir().Join(
		constants.DIST_DIRNAME,
		vc.core.BinaryName,
	)
	vc.core.binary_output_path_abs = bin_out_cwd.MustAbs()

	vc.core.PublicPathPrefix = strings.TrimSpace(
		__raw.Core.PublicPathPrefix,
	)
	vc.core.PublicPathPrefix = matcher.EnsureLeadingAndTrailingSlash(
		vc.core.PublicPathPrefix,
	)

	using_private := __raw.Core.StaticAssetDirs.Private != ""
	using_public := __raw.Core.StaticAssetDirs.Public != ""
	using_critical_css := __raw.Core.CSSEntryFiles.Critical != ""
	using_non_critical_css := __raw.Core.CSSEntryFiles.NonCritical != ""

	if using_private {
		vc.core.StaticAssetDirs.Private, err = join_check_and_reserve_dir(
			__raw.Core.StaticAssetDirs.Private,
			"StaticAssetDirs.Private",
		)
		if err != nil {
			return nil, err
		}
	}
	if using_public {
		vc.core.StaticAssetDirs.Public, err = join_check_and_reserve_dir(
			__raw.Core.StaticAssetDirs.Public,
			"StaticAssetDirs.Public",
		)
		if err != nil {
			return nil, err
		}
	}
	if using_private && using_public &&
		check_overlap(
			vc.core.StaticAssetDirs.Private,
			vc.core.StaticAssetDirs.Public,
		) {
		return nil, fmt.Errorf(
			"StaticAssetDirs.Private and StaticAssetDirs.Public must not overlap",
		)
	}
	if using_critical_css {
		vc.core.CSSEntryFiles.Critical, err = join_check_and_reserve_file(
			__raw.Core.CSSEntryFiles.Critical,
			"CSSEntryFiles.Critical",
		)
		if err != nil {
			return nil, err
		}
	}
	if using_non_critical_css {
		vc.core.CSSEntryFiles.NonCritical, err = join_check_and_reserve_file(
			__raw.Core.CSSEntryFiles.NonCritical,
			"CSSEntryFiles.NonCritical",
		)
		if err != nil {
			return nil, err
		}
	}

	// Ensure CSS entry files are not inside the static asset dirs
	if using_critical_css && using_public {
		if m, _ := path_match(vc.public_dir_pattern(), vc.core.CSSEntryFiles.Critical); m {
			return nil, fmt.Errorf(
				"CSSEntryFiles.Critical must not be inside StaticAssetDirs.Public",
			)
		}
	}
	if using_critical_css && using_private {
		if m, _ := path_match(vc.private_dir_pattern(), vc.core.CSSEntryFiles.Critical); m {
			return nil, fmt.Errorf(
				"CSSEntryFiles.Critical must not be inside StaticAssetDirs.Private",
			)
		}
	}
	if using_non_critical_css && using_public {
		if m, _ := path_match(vc.public_dir_pattern(), vc.core.CSSEntryFiles.NonCritical); m {
			return nil, fmt.Errorf(
				"CSSEntryFiles.NonCritical must not be inside StaticAssetDirs.Public",
			)
		}
	}
	if using_non_critical_css && using_private {
		if m, _ := path_match(vc.private_dir_pattern(), vc.core.CSSEntryFiles.NonCritical); m {
			return nil, fmt.Errorf(
				"CSSEntryFiles.NonCritical must not be inside StaticAssetDirs.Private",
			)
		}
	}

	/////// Vite

	vc.vite.JSPackageManagerBaseCmd = strings.TrimSpace(
		__raw.Vite.JSPackageManagerBaseCmd,
	)
	vc.vite.UsingVite = vc.vite.JSPackageManagerBaseCmd != ""
	if vc.vite.UsingVite {
		var err error
		// do not reserve the js package manager cmd dir
		vc.vite.JSPackageManagerCmdDir, err = join_and_check_dir(
			__raw.Vite.JSPackageManagerCmdDir,
			"Vite.JSPackageManagerCmdDir",
		)
		if err != nil {
			return nil, err
		}
		vc.vite.DefaultPort = __raw.Vite.DefaultPort
		if vc.vite.DefaultPort == 0 {
			vc.vite.DefaultPort = 5173
		}
		if __raw.Vite.ViteConfigFile != nil {
			p, err := join_check_and_reserve_file(
				*__raw.Vite.ViteConfigFile,
				"Vite.ViteConfigFile",
			)
			if err != nil {
				return nil, err
			}
			vc.vite.ViteConfigFile = &p
		}
	}

	/////// Lifecycle Hooks

	for i := range __raw.LifecycleHooks {
		label := fmt.Sprintf("user idx %d hook", i)
		if __raw.LifecycleHooks[i].IsGoCompile {
			label = "user Go compile hook"
		}
		hook, err := __raw.LifecycleHooks[i].to_validated_hook(
			label,
			vc.root_dir,
		)
		if err != nil {
			return nil, err
		}
		vc.lifecycle_hooks = append(vc.lifecycle_hooks, *hook)
	}

	return vc, nil
}

type reserved_paths struct{ paths set.Set[strict.CWDRelPath] }

func (r *reserved_paths) add(p strict.CWDRelPath, label string) error {
	if r.paths.Has(p) {
		return fmt.Errorf(
			"another field is already using path %q, cannot use it for %s",
			p,
			label,
		)
	}
	r.paths.Add(p)
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// User Config
/////////////////////////////////////////////////////////////////////

type UserConfig struct{ *validated_config }

func (r *UserConfig) RootDir() strict.CWDRelPath {
	return r.root_dir
}

// BinaryOutputPathAbs returns the path where the compiled binary should
// be written (e.g. ".waveout/myapp").
func (r *UserConfig) BinaryOutputPathAbs() strict.MachAbsPath {
	return r.core.binary_output_path_abs
}

func (r *UserConfig) PublicStaticDir() (strict.CWDRelPath, bool) {
	if !r.using_public_static() {
		return "", false
	}
	return r.core.StaticAssetDirs.Public, true
}

func (r *UserConfig) PrivateStaticDir() (strict.CWDRelPath, bool) {
	if !r.using_private_static() {
		return "", false
	}
	return r.core.StaticAssetDirs.Private, true
}

func (r *UserConfig) CriticalCSSEntry() (strict.CWDRelPath, bool) {
	if !r.using_critical_css() {
		return "", false
	}
	return r.core.CSSEntryFiles.Critical, true
}

func (r *UserConfig) NonCriticalCSSEntry() (strict.CWDRelPath, bool) {
	if !r.using_non_critical_css() {
		return "", false
	}
	return r.core.CSSEntryFiles.NonCritical, true
}
