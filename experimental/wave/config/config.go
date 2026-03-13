package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

type Config interface {
	__cfg_marker()
	Get() *unsafe_config
}

type Checkpoint string

const (
	Checkpoint_1_CycleStart     Checkpoint = "1_cycle_start"
	Checkpoint_2_BuildStart     Checkpoint = "2_build_start"
	Checkpoint_3_GoCompileStart Checkpoint = "3_go_compile_start"
	Checkpoint_4_BuildEnd       Checkpoint = "4_build_end"
	Checkpoint_5_CycleEnd       Checkpoint = "5_cycle_end"
)

func is_valid_checkpoint(s Checkpoint) bool {
	switch s {
	case Checkpoint_1_CycleStart,
		Checkpoint_2_BuildStart,
		Checkpoint_3_GoCompileStart,
		Checkpoint_4_BuildEnd,
		Checkpoint_5_CycleEnd:
		return true
	default:
		return false
	}
}

func checkpoint_order(c Checkpoint) int {
	switch c {
	case Checkpoint_1_CycleStart:
		return 1
	case Checkpoint_2_BuildStart:
		return 2
	case Checkpoint_3_GoCompileStart:
		return 3
	case Checkpoint_4_BuildEnd:
		return 4
	case Checkpoint_5_CycleEnd:
		return 5
	default:
		return -1
	}
}

func validate_checkpoints(
	start Checkpoint,
	end Checkpoint,
	label string,
) error {
	if !is_valid_checkpoint(start) {
		return fmt.Errorf("%s has invalid StartAt checkpoint: %s", label, start)
	}
	if !is_valid_checkpoint(end) {
		return fmt.Errorf("%s has invalid FinishBy checkpoint: %s", label, end)
	}
	if checkpoint_order(start) > checkpoint_order(end) {
		return fmt.Errorf(
			"%s has StartAt checkpoint that is after FinishBy checkpoint: StartAt=%s, FinishBy=%s",
			label,
			start,
			end,
		)
	}
	return nil
}

type DownstreamEffect string

const (
	// DownstreamEffectGoCompile implies DownstreamEffectAppRestart
	DownstreamEffectGoCompile          DownstreamEffect = "go_compile"
	DownstreamEffectAppRestart         DownstreamEffect = "app_restart"
	DownstreamEffectFrontendRevalidate DownstreamEffect = "frontend_revalidate"
)

func is_valid_downstream_effect(s DownstreamEffect) bool {
	switch s {
	case DownstreamEffectGoCompile,
		DownstreamEffectAppRestart,
		DownstreamEffectFrontendRevalidate:
		return true
	default:
		return false
	}
}

type unsafe_config struct {
	__is_validated bool              `json:"-"`
	ConfigPath     strict.CWDRelPath `json:"-"`

	// The root directory to watch for file changes
	// and against which all other paths and patterns
	// in the config are resolved. Set this path
	// relative to the current working directory of
	// the underlying OS process.
	RootDir        strict.CWDRelPath
	Core           unsafe_core
	Vite           unsafe_vite
	LifecycleHooks []unsafe_lifecycle_hook
}

func (c *unsafe_config) __cfg_marker() {}
func (c *unsafe_config) Get() *unsafe_config {
	if !c.__is_validated {
		panic("config not validated")
	}
	return c
}

/////////////////////////////////////////////////////////////////////
/////// Config helpers
/////////////////////////////////////////////////////////////////////

func (c *unsafe_config) HasPrivateStatic() bool {
	return c.Core.StaticAssetDirs.Private != ""
}

func (c *unsafe_config) HasPublicStatic() bool {
	return c.Core.StaticAssetDirs.Public != ""
}

func (c *unsafe_config) HasCriticalCSS() bool {
	return c.Core.CSSEntryFiles.Critical != ""
}

func (c *unsafe_config) HasNonCriticalCSS() bool {
	return c.Core.CSSEntryFiles.NonCritical != ""
}

func (c *unsafe_config) PrivateDirPattern() strict.CWDRelPath {
	return c.Core.StaticAssetDirs.Private.Join("**/*")
}

func (c *unsafe_config) PublicDirPattern() strict.CWDRelPath {
	return c.Core.StaticAssetDirs.Public.Join("**/*")
}

func (c *unsafe_config) GoFilePattern() strict.CWDRelPath {
	return c.RootDir.Join("**/*.go")
}

/////////////////////////////////////////////////////////////////////
/////// Core / Vite / Lifecycle structs
/////////////////////////////////////////////////////////////////////

type unsafe_core struct {
	/////// universal

	MainAppEntry                   strict.CWDRelPath
	HealthcheckEndpoint            string
	GlobalWatchExcludePatterns     []strict.CWDRelPath
	PreventImplicitGoBuildPatterns []strict.CWDRelPath

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

type unsafe_lifecycle_hook struct {
	WatchIncludePatterns []strict.CWDRelPath
	WatchExcludePatterns []strict.CWDRelPath

	Cmd string

	StartAt  Checkpoint
	FinishBy Checkpoint

	DownstreamEffect                 DownstreamEffect
	IncludeFrontendRebuildingOverlay bool

	DevOnly  bool
	ProdOnly bool
}

func ConfigPathToValidatedConfig(
	config_path strict.CWDRelPath,
) (*unsafe_config, error) {
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
	cfg_path := strict.MustNormalize(_config_path)
	if err := check_file(cfg_path, "config path"); err != nil {
		return nil, err
	}

	json_bytes, err := os.ReadFile(string(cfg_path))
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
	return &raw, nil
}

// to_recursive_pattern joins a pattern with root, and if the result
// is an existing directory, appends "**/*".
func to_recursive_pattern(
	root strict.CWDRelPath, pattern strict.CWDRelPath,
) strict.CWDRelPath {
	p := root.Join(string(pattern.MustNormalize()))
	if ok, err := p.IsDir(); err == nil && ok {
		p = p.Join("**/*")
	}
	return p
}

func validate_config(c *unsafe_config) (*unsafe_config, error) {
	if c == nil {
		return nil, fmt.Errorf("nil unsafe config")
	}

	var err error
	var reserved reserved_paths

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
		p := c.RootDir.Join(string(_p.MustNormalize()))
		return p, check_file_and_reserve(p, label)
	}
	var join_check_and_reserve_dir = func(_p strict.CWDRelPath, label string) (strict.CWDRelPath, error) {
		p := c.RootDir.Join(string(_p.MustNormalize()))
		return p, check_dir_and_reserve(p, label)
	}
	var join_and_check_dir = func(_p strict.CWDRelPath, label string) (strict.CWDRelPath, error) {
		p := c.RootDir.Join(string(_p.MustNormalize()))
		return p, check_dir(p, label)
	}
	var str = func(s string) string {
		return strings.TrimSpace(s)
	}

	/////// Top level

	c.ConfigPath = c.ConfigPath.MustNormalize()
	if err := check_file_and_reserve(c.ConfigPath, "ConfigPath"); err != nil {
		return nil, err
	}
	c.RootDir = c.RootDir.MustNormalize()
	if err := check_dir_and_reserve(c.RootDir, "RootDir"); err != nil {
		return nil, err
	}

	/////// Core

	// MainAppEntry is joined manually because of the dir→main.go fallback,
	// so we use check_file rather than file() to avoid double-joining.
	c.Core.MainAppEntry = c.RootDir.Join(
		string(c.Core.MainAppEntry.MustNormalize()),
	)
	if ok, err := c.Core.MainAppEntry.IsDir(); err == nil && ok {
		c.Core.MainAppEntry = c.Core.MainAppEntry.Join("main.go")
	}
	if err := check_file_and_reserve(c.Core.MainAppEntry, "MainAppEntry"); err != nil {
		return nil, err
	}

	c.Core.HealthcheckEndpoint = str(c.Core.HealthcheckEndpoint)
	if c.Core.HealthcheckEndpoint == "" {
		c.Core.HealthcheckEndpoint = "/"
	}

	for i, pattern := range c.Core.GlobalWatchExcludePatterns {
		c.Core.GlobalWatchExcludePatterns[i] = to_recursive_pattern(
			c.RootDir,
			pattern,
		)
	}
	slices.Sort(c.Core.GlobalWatchExcludePatterns)

	for i, pattern := range c.Core.PreventImplicitGoBuildPatterns {
		c.Core.PreventImplicitGoBuildPatterns[i] = to_recursive_pattern(
			c.RootDir,
			pattern,
		)
	}
	slices.Sort(c.Core.PreventImplicitGoBuildPatterns)

	c.Core.PublicPathPrefix = str(c.Core.PublicPathPrefix)
	c.Core.PublicPathPrefix = matcher.EnsureLeadingAndTrailingSlash(
		c.Core.PublicPathPrefix,
	)

	has_private := c.Core.StaticAssetDirs.Private != ""
	has_public := c.Core.StaticAssetDirs.Public != ""
	has_critical_css := c.Core.CSSEntryFiles.Critical != ""
	has_non_critical_css := c.Core.CSSEntryFiles.NonCritical != ""

	if has_private {
		if c.Core.StaticAssetDirs.Private, err = join_check_and_reserve_dir(c.Core.StaticAssetDirs.Private, "StaticAssetDirs.Private"); err != nil {
			return nil, err
		}
	}
	if has_public {
		if c.Core.StaticAssetDirs.Public, err = join_check_and_reserve_dir(c.Core.StaticAssetDirs.Public, "StaticAssetDirs.Public"); err != nil {
			return nil, err
		}
	}
	if has_critical_css {
		if c.Core.CSSEntryFiles.Critical, err = join_check_and_reserve_file(c.Core.CSSEntryFiles.Critical, "CSSEntryFiles.Critical"); err != nil {
			return nil, err
		}
	}
	if has_non_critical_css {
		if c.Core.CSSEntryFiles.NonCritical, err = join_check_and_reserve_file(c.Core.CSSEntryFiles.NonCritical, "CSSEntryFiles.NonCritical"); err != nil {
			return nil, err
		}
	}

	/////// Vite

	c.Vite.UsingVite = !reflect.DeepEqual(unsafe_vite{}, c.Vite)
	if c.Vite.UsingVite {
		c.Vite.JSPackageManagerBaseCmd = str(
			c.Vite.JSPackageManagerBaseCmd,
		)
		if c.Vite.JSPackageManagerBaseCmd == "" {
			return nil, fmt.Errorf(
				"JSPackageManagerBaseCmd is required when using Vite",
			)
		}
		var err error
		// do not reserve the js package manager cmd dir
		if c.Vite.JSPackageManagerCmdDir, err = join_and_check_dir(c.Vite.JSPackageManagerCmdDir, "Vite.JSPackageManagerCmdDir"); err != nil {
			return nil, err
		}
		if c.Vite.DefaultPort == 0 {
			c.Vite.DefaultPort = 5173
		}
		if c.Vite.ViteConfigFile != nil {
			p, err := join_check_and_reserve_file(
				*c.Vite.ViteConfigFile,
				"Vite.ViteConfigFile",
			)
			if err != nil {
				return nil, err
			}
			c.Vite.ViteConfigFile = &p
		}
	}

	/////// Lifecycle Hooks

	for i := range c.LifecycleHooks {
		hook := &c.LifecycleHooks[i]

		if len(hook.WatchIncludePatterns) == 0 {
			return nil, fmt.Errorf(
				"lifecycle hook must have at least one WatchIncludePattern: hook index %d",
				i,
			)
		}

		sort_func := func(a, b strict.CWDRelPath) int {
			return strings.Compare(string(a), string(b))
		}
		for j, pattern := range hook.WatchIncludePatterns {
			hook.WatchIncludePatterns[j] = to_recursive_pattern(
				c.RootDir,
				pattern,
			)
		}
		slices.SortFunc(hook.WatchIncludePatterns, sort_func)
		for j, pattern := range hook.WatchExcludePatterns {
			hook.WatchExcludePatterns[j] = to_recursive_pattern(
				c.RootDir,
				pattern,
			)
		}
		slices.SortFunc(hook.WatchExcludePatterns, sort_func)

		hook.Cmd = str(hook.Cmd)
		if hook.Cmd == "" {
			return nil, fmt.Errorf(
				"lifecycle hook Cmd is required: hook index %d", i,
			)
		}
		if err := validate_checkpoints(hook.StartAt, hook.FinishBy, fmt.Sprintf("lifecycle hook index %d", i)); err != nil {
			return nil, err
		}
		if !is_valid_downstream_effect(hook.DownstreamEffect) {
			return nil, fmt.Errorf(
				"lifecycle hook DownstreamEffect has invalid value: hook index %d",
				i,
			)
		}
		if hook.DevOnly && hook.ProdOnly {
			return nil, fmt.Errorf(
				"lifecycle hook cannot be both DevOnly and ProdOnly: hook index %d",
				i,
			)
		}
	}

	slices.SortFunc(
		c.LifecycleHooks,
		func(a, b unsafe_lifecycle_hook) int {
			a_json, _ := jsonutil.Serialize(a)
			b_json, _ := jsonutil.Serialize(b)
			return strings.Compare(string(a_json), string(b_json))
		},
	)

	c.__is_validated = true
	return c, nil
}

type reserved_paths struct{ paths set.Set[strict.CWDRelPath] }

func (r *reserved_paths) add(p strict.CWDRelPath, label string) error {
	if r.paths.Has(p) {
		return fmt.Errorf(
			"config path %q is already in use by another config field: %s",
			p,
			label,
		)
	}
	r.paths.Add(p)
	return nil
}
