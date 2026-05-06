package vormabuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/internal/pkg/staticproc"
	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/globset"
	"github.com/vormadev/vorma/kit/matcher"
)

type vorma_cfg struct{ C *vorma.Config }

const (
	ui_variant_react  = "react"
	ui_variant_preact = "preact"
	ui_variant_remix  = "remix"
	ui_variant_solid  = "solid"
)

/////////////////////////////////////////////////////////////////////
/////// RUN STATE -- HIGH-LEVEL CONFIG GETTER
/////////////////////////////////////////////////////////////////////

// Acquires lock.
// Returns config if available, otherwise error.
func (rs *run_state) get_config() (*vorma_cfg, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.cfg != nil {
		return rs.cfg, nil
	}
	return nil, fmt.Errorf("config not available yet")
}

/////////////////////////////////////////////////////////////////////
/////// CONFIG -- INITIAL SETUP / VALIDATION
/////////////////////////////////////////////////////////////////////

// Converts user-provided config to internal config struct
// and validates certain fields. Does not set any fields, as
// all config fields are JIT-derived from live state. This
// massively simplifies the overall mental model.
func to_cfg(c *vorma.Config) (*vorma_cfg, error) {
	cfg := &vorma_cfg{C: c}

	if _, err := cfg.__validate_ui_variant(); err != nil {
		return nil, fmt.Errorf("error with UI variant: %w", err)
	}
	if _, err := cfg.__validate_pub_src_pattern(); err != nil {
		return nil, fmt.Errorf("error with public static source pattern: %w", err)
	}
	if _, err := cfg.__validate_global_watch_patterns(); err != nil {
		return nil, fmt.Errorf("error with global ignore patterns: %w", err)
	}
	if _, err := cfg.__validate_server_watch_patterns(); err != nil {
		return nil, fmt.Errorf("error with server watch patterns: %w", err)
	}
	if _, err := cfg.__validate_client_revalidate_on_change_patterns(); err != nil {
		return nil, fmt.Errorf("error with client revalidate on change patterns: %w", err)
	}
	if _, err := cfg.__validate_vorma_out_abs_slash_pattern(); err != nil {
		return nil, fmt.Errorf("__validate_vorma_out_abs_slash_pattern err: %w", err)
	}
	if _, err := cfg.__validate_vorma_out_watch_ignore_pattern(); err != nil {
		return nil, fmt.Errorf("error with Vorma output watch ignore pattern: %w", err)
	}
	if _, err := cfg.__validate_ts_gen_out_file_abs_slash(); err != nil {
		return nil, fmt.Errorf("__validate_gen_out_file_abs_slash err: %w", err)
	}
	if _, err := cfg.__validate_ts_entry_abs_slash(); err != nil {
		return nil, fmt.Errorf("__validate_ts_entry_abs_slash err: %w", err)
	}
	if _, err := cfg.__validate_vite_config_file(); err != nil {
		return nil, fmt.Errorf("error with Vite config file: %w", err)
	}

	return cfg, nil
}

/////////////////////////////////////////////////////////////////////
/////// CONFIG -- CORE FIELD GETTERS
/////////////////////////////////////////////////////////////////////

func (cfg vorma_cfg) watch_root() string      { return fsutil.SysNorm(cfg.C.DevWatchConfig.WatchRoot) }
func (cfg vorma_cfg) dist_dir() string        { return fsutil.SysNorm(cfg.C.DistDir) }
func (cfg vorma_cfg) ts_gen_out_file() string { return fsutil.SysNorm(cfg.C.TSGenConfig.OutFile) }

func (cfg vorma_cfg) global_watch_exclude_patterns() []string {
	x, _ := cfg.__validate_global_watch_patterns()
	return x
}
func (cfg vorma_cfg) __validate_global_watch_patterns() ([]string, error) {
	patterns := make([]string, len(cfg.C.DevWatchConfig.GlobalIgnore))
	for i, p := range cfg.C.DevWatchConfig.GlobalIgnore {
		patterns[i] = strings.TrimSpace(p)
	}
	if _, err := globset.Compile(patterns); err != nil {
		return nil, err
	}
	return patterns, nil
}

func (cfg vorma_cfg) server_entry() string { return fsutil.SysNorm(cfg.C.ServerEntry) }

func (cfg vorma_cfg) server_watch_patterns() []string {
	x, _ := cfg.__validate_server_watch_patterns()
	if len(x) == 0 {
		return []string{"**/*.go"}
	}
	return x
}
func (cfg vorma_cfg) __validate_server_watch_patterns() ([]string, error) {
	patterns := make([]string, len(cfg.C.DevWatchConfig.OnChangeRecompileGo))
	for i, p := range cfg.C.DevWatchConfig.OnChangeRecompileGo {
		patterns[i] = strings.TrimSpace(p)
	}
	if len(patterns) == 0 {
		patterns = []string{"**/*.go"}
	}
	if _, err := globset.Compile(patterns); err != nil {
		return nil, err
	}
	return patterns, nil
}

func (cfg vorma_cfg) client_revalidate_on_change_patterns() []string {
	x, _ := cfg.__validate_client_revalidate_on_change_patterns()
	return x
}
func (cfg vorma_cfg) __validate_client_revalidate_on_change_patterns() ([]string, error) {
	patterns := make([]string, len(cfg.C.DevWatchConfig.OnChangeClientRevalidate))
	for i, p := range cfg.C.DevWatchConfig.OnChangeClientRevalidate {
		patterns[i] = strings.TrimSpace(p)
	}
	if _, err := globset.Compile(patterns); err != nil {
		return nil, err
	}
	return patterns, nil
}

func (cfg vorma_cfg) public_static_src_dir() string {
	return fsutil.SysNorm(cfg.C.FrontendConfig.PublicStaticSrcDir)
}

func (cfg vorma_cfg) critical_css_entry() string {
	return fsutil.SysNorm(cfg.C.FrontendConfig.CriticalCSSFile)
}

func (cfg vorma_cfg) public_static_base_path() string {
	return matcher.EnsureLeadingAndTrailingSlash(
		strings.TrimSpace(cfg.C.PathConfig.PublicStaticBase),
	)
}
func (cfg vorma_cfg) actions_mount_root() string {
	return matcher.EnsureLeadingAndTrailingSlash(strings.TrimSpace(cfg.C.PathConfig.APIBase))
}

func (cfg vorma_cfg) ui_variant() string {
	x, _ := cfg.__validate_ui_variant()
	return x
}
func (cfg vorma_cfg) __validate_ui_variant() (string, error) {
	ui := strings.TrimSpace(cfg.C.FrontendConfig.UIVariant)
	if ui != ui_variant_react &&
		ui != ui_variant_preact &&
		ui != ui_variant_remix &&
		ui != ui_variant_solid {
		return "", fmt.Errorf("invalid UI variant: %s", ui)
	}
	return ui, nil
}

func (cfg vorma_cfg) ts_entry() string {
	return filepath.ToSlash(fsutil.SysNorm(cfg.C.FrontendConfig.EntryFile))
}
func (cfg vorma_cfg) js_package_manager_cmd_base() []string {
	return strings.Fields(strings.TrimSpace(cfg.C.FrontendConfig.JSPackageManagerBaseCmd))
}
func (cfg vorma_cfg) js_package_manager_dir() string {
	return fsutil.SysNorm(cfg.C.FrontendConfig.JSPackageManagerDir)
}

func (cfg vorma_cfg) vite_config_file() string {
	x, _ := cfg.__validate_vite_config_file()
	return x
}
func (cfg vorma_cfg) __validate_vite_config_file() (string, error) {
	if strings.TrimSpace(cfg.C.FrontendConfig.ViteConfigFile) == "" {
		return "", nil
	}
	cwd_rel := fsutil.SysNorm(cfg.C.FrontendConfig.ViteConfigFile)
	js_dir_rel, err := filepath.Rel(cfg.js_package_manager_dir(), cwd_rel)
	if err != nil {
		return "", fmt.Errorf("error calculating relative path: %w", err)
	}
	return js_dir_rel, nil
}

func (cfg vorma_cfg) root_html_template() string {
	return fsutil.SysNorm(cfg.C.HTMLConfig.Template)
}

/////////////////////////////////////////////////////////////////////
/////// CONFIG -- PATTERN MATCHING HELPERS
/////////////////////////////////////////////////////////////////////

func (cfg vorma_cfg) pub_src_pattern() string {
	x, _ := cfg.__validate_pub_src_pattern()
	return x
}
func (cfg vorma_cfg) __validate_pub_src_pattern() (string, error) {
	pattern := fsutil.ToCatchDirPattern(cfg.public_static_src_dir())
	if !doublestar.ValidatePathPattern(pattern) {
		return "", fmt.Errorf("invalid public static source pattern: %s", pattern)
	}
	return pattern, nil
}
func (cfg vorma_cfg) matches_pub_src(p string) bool {
	return doublestar.PathMatchUnvalidated(cfg.pub_src_pattern(), p)
}

func (cfg vorma_cfg) watch_relative_path(p string) string {
	rel_path, err := filepath.Rel(cfg.watch_root(), p)
	if err != nil {
		return p
	}
	return rel_path
}

func (cfg vorma_cfg) vorma_out_watch_ignore_pattern() string {
	x, _ := cfg.__validate_vorma_out_watch_ignore_pattern()
	return x
}
func (cfg vorma_cfg) __validate_vorma_out_watch_ignore_pattern() (string, error) {
	pattern := fsutil.ToCatchDirPattern(cfg.watch_relative_path(cfg.vorma_out()))
	if _, err := globset.Compile([]string{pattern}); err != nil {
		return "", err
	}
	return pattern, nil
}

/////////////////////////////////////////////////////////////////////
/////// CONFIG -- STATIC HELPERS
/////////////////////////////////////////////////////////////////////

func (cfg vorma_cfg) collect_physical_pub_files() (staticproc.Files, error) {
	return staticproc.CollectPhysical(
		cfg.public_static_src_dir(),
		[]string{vormarun.Prehashed_Dirname},
		vormarun.Public_Static_Out_Name_Prefix,
	)
}

func (cfg vorma_cfg) to_pub_fm(pub_files staticproc.Files) map[string]string {
	pub_fm := make(map[string]string)
	for rel, f := range pub_files {
		pub_fm[rel] = cfg.public_static_base_path() + f.OutName
	}
	return pub_fm
}

/////////////////////////////////////////////////////////////////////
/////// CONFIG -- DIST HELPERS
/////////////////////////////////////////////////////////////////////

/////// Core Vorma Out Dir

func (cfg vorma_cfg) vorma_out() string {
	return filepath.Join(cfg.dist_dir(), ".vorma")
}

/////// Server Binary Out Path

func (cfg vorma_cfg) server_bin_out() string {
	bin_name := "main"
	if runtime.GOOS == "windows" {
		bin_name += ".exe"
	}
	return filepath.Join(cfg.vorma_out(), bin_name)
}

/////// Public Static Out

func (cfg vorma_cfg) pub_out() string {
	return filepath.Join(cfg.vorma_out(), "static", "public")
}

/////// Git Ignore File

func (cfg vorma_cfg) gitignore_out() string {
	return filepath.Join(cfg.vorma_out(), ".gitignore")
}
func (cfg vorma_cfg) write_gitignore() error {
	err := write_str_to_file(gitignore_content, cfg.gitignore_out())
	if err != nil {
		return fmt.Errorf("error writing .gitignore: %w", err)
	}
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// CONFIG -- VITE PLUGIN HELPERS
/////////////////////////////////////////////////////////////////////

func (cfg vorma_cfg) vorma_out_abs_slash_pattern() string {
	x, _ := cfg.__validate_vorma_out_abs_slash_pattern()
	return x
}
func (cfg vorma_cfg) __validate_vorma_out_abs_slash_pattern() (string, error) {
	with_catch := fsutil.ToCatchDirPattern(cfg.vorma_out())
	abs, err := filepath.Abs(with_catch)
	if err != nil {
		return "", fmt.Errorf("error calculating absolute pattern: %w", err)
	}
	is_valid := doublestar.ValidatePathPattern(abs)
	if !is_valid {
		return "", fmt.Errorf("invalid absolute pattern: %s", abs)
	}
	return filepath.ToSlash(abs), nil
}

func (cfg vorma_cfg) gen_out_file_abs_slash() string {
	x, _ := cfg.__validate_ts_gen_out_file_abs_slash()
	return x
}
func (cfg vorma_cfg) __validate_ts_gen_out_file_abs_slash() (string, error) {
	abs, err := filepath.Abs(cfg.ts_gen_out_file())
	if err != nil {
		return "", fmt.Errorf("error calculating absolute path: %w", err)
	}
	return filepath.ToSlash(abs), nil
}

func (cfg vorma_cfg) ts_entry_abs_slash() string {
	x, _ := cfg.__validate_ts_entry_abs_slash()
	return x
}
func (cfg vorma_cfg) __validate_ts_entry_abs_slash() (string, error) {
	abs, err := filepath.Abs(cfg.ts_entry())
	if err != nil {
		return "", fmt.Errorf("error calculating absolute path: %w", err)
	}
	return filepath.ToSlash(abs), nil
}

func (cfg vorma_cfg) vite_ignored_patterns() []string {
	return []string{
		"**/*.go",
		cfg.vorma_out_abs_slash_pattern(),
		cfg.gen_out_file_abs_slash(),
	}
}

func (cfg vorma_cfg) vite_dedupe_list() []string {
	switch cfg.ui_variant() {
	case ui_variant_react:
		return []string{
			"react",
			"react-dom",
		}
	case ui_variant_preact:
		return []string{
			"preact",
			"preact/hooks",
			"@preact/signals",
			"preact/jsx-runtime",
			"preact/compat",
			"preact/test-utils",
		}
	case ui_variant_remix:
		return []string{
			"remix",
			"remix/ui",
			"@remix-run/ui",
		}
	case ui_variant_solid:
		return []string{
			"solid-js",
			"solid-js/web",
		}
	default:
		return nil
	}
}

/////////////////////////////////////////////////////////////////////
/////// CONFIG -- APP SERVER HELPERS
/////////////////////////////////////////////////////////////////////

func (cfg vorma_cfg) compile_app_server(ctx context.Context) error {
	cmd := new_cmd(
		ctx,
		"go",
		"build",
		"-o",
		filepath.FromSlash("./"+cfg.server_bin_out()),
		filepath.FromSlash("./"+cfg.server_entry()),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (cfg vorma_cfg) run_app_server_cmd(ctx context.Context, env []string) *exec.Cmd {
	cmd := new_cmd(ctx, cfg.server_bin_out())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env, env...)
	return cmd
}

/////////////////////////////////////////////////////////////////////
/////// CONFIG -- VITE SERVER HELPERS
/////////////////////////////////////////////////////////////////////

func (cfg vorma_cfg) run_vite_server_cmd(
	ctx context.Context,
	vite_port int,
	internal_dev_server_port int,
) *exec.Cmd {
	args := append(cfg.js_package_manager_cmd_base(),
		"vite",
		"--host", dev_loopback_host,
		"--port", fmt.Sprintf("%d", vite_port),
		"--clearScreen", "false",
		"--strictPort", "true",
	)
	cfg_file := cfg.vite_config_file()
	if cfg_file != "" {
		args = append(args, "--config", cfg_file)
	}

	cmd := new_cmd(ctx, args[0], args[1:]...)
	cmd.Dir = cfg.js_package_manager_dir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env, env_item_int(
		vite_plugin_go_port_env_key,
		internal_dev_server_port,
	))
	return cmd
}
