package vormabuild

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
	"github.com/vormadev/vorma/vorma2"
	"github.com/vormadev/vorma/vorma2/internal/constants"
	"github.com/vormadev/vorma/vorma2/internal/types"
	"github.com/vormadev/vorma/wave/wavebuild"
)

const json_cfg_key = "Vorma"

type unsafe_vorma_config struct {
	UIVariant                     string
	HTMLTemplateLocation          string
	ClientEntry                   string
	ClientRouteDefinitionPatterns []string
	TSGenOutDir                   string
	BuildtimePublicURLFuncName    string
}

type parsed_vorma_config struct {
	user_root_dir                    strict.CWDRelPath
	ui_variant                       string
	root_template_path               types.PrivateFSRelPath
	client_entry_path                strict.CWDRelPath
	client_route_definition_patterns []strict.CWDRelPath
	ts_gen_out_dir                   strict.CWDRelPath
	buildtime_public_url_func_name   string
}

type plugin_state struct {
	plugin *wavebuild.Plugin
	cfg    *parsed_vorma_config
	app    *vorma2.Vorma
}

func NewPlugin(app *vorma2.Vorma) *wavebuild.Plugin {
	if app == nil {
		panic("[vorma2]: app is required")
	}
	state := &plugin_state{app: app}
	plugin := &wavebuild.Plugin{
		Name: "vorma2",
		Config: wavebuild.PluginConfig{
			JSONKey:   json_cfg_key,
			ParseFunc: state.parse_config,
		},
	}
	state.plugin = plugin
	return plugin
}

func (s *plugin_state) parse_config(
	ctx *wavebuild.PluginConfigParseCtx,
) error {
	s.cfg = nil

	if ctx == nil {
		return fmt.Errorf("nil plugin config parse ctx")
	}
	if ctx.RawJSON == nil {
		return fmt.Errorf("Vorma config section is required in wave config")
	}

	var raw unsafe_vorma_config
	if err := json.Unmarshal(*ctx.RawJSON, &raw); err != nil {
		return fmt.Errorf("parsing Vorma JSON: %w", err)
	}

	// ui_variant
	ui_variant := strings.TrimSpace(raw.UIVariant)
	if ui_variant == "" {
		return fmt.Errorf("Vorma.UIVariant is required")
	}
	switch ui_variant {
	case "react", "preact", "solid":
	default:
		return fmt.Errorf(
			"Vorma.UIVariant must be react, preact, or solid, got %q",
			ui_variant,
		)
	}

	// root_template_path: private-FS-relative, not CWD-relative
	root_template_path := types.PrivateFSRelPath(
		path.Clean(strings.TrimSpace(raw.HTMLTemplateLocation)),
	)
	if root_template_path == "" || root_template_path == "." {
		return fmt.Errorf("Vorma.HTMLTemplateLocation is required")
	}
	if path.IsAbs(root_template_path.Str()) {
		return fmt.Errorf(
			"Vorma.HTMLTemplateLocation must be private-FS-relative, not absolute",
		)
	}

	// client_entry_path: CWD-relative, joined with root dir
	client_entry_raw := strings.TrimSpace(raw.ClientEntry)
	if client_entry_raw == "" {
		return fmt.Errorf("Vorma.ClientEntry is required")
	}
	client_entry_path := ctx.UserRootDir().Join(
		strict.MustNormalizeCWDRelPath(client_entry_raw).Str(),
	)

	// client_route_definition_patterns: CWD-relative, joined with root dir
	if len(raw.ClientRouteDefinitionPatterns) == 0 {
		return fmt.Errorf("Vorma.ClientRouteDefinitionPatterns is required")
	}
	client_route_definition_patterns := make(
		[]strict.CWDRelPath,
		len(raw.ClientRouteDefinitionPatterns),
	)
	seen := &set.Set[string]{}
	for i, raw_pattern := range raw.ClientRouteDefinitionPatterns {
		trimmed := strings.TrimSpace(raw_pattern)
		if trimmed == "" {
			return fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d] cannot be empty",
				i,
			)
		}
		normalized := strict.MustNormalizeCWDRelPath(trimmed)
		if normalized == "" || normalized == "." {
			return fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d] is invalid",
				i,
			)
		}
		joined := ctx.UserRootDir().Join(normalized.Str())
		if seen.Has(joined.Str()) {
			return fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d]=%q duplicates an earlier pattern",
				i,
				raw_pattern,
			)
		}
		seen.Add(joined.Str())
		client_route_definition_patterns[i] = joined
	}

	// ts_gen_out_dir: CWD-relative, joined with root dir
	ts_gen_out_raw := strings.TrimSpace(raw.TSGenOutDir)
	if ts_gen_out_raw == "" {
		return fmt.Errorf("Vorma.TSGenOutDir is required")
	}
	ts_gen_out_dir := ctx.UserRootDir().Join(
		strict.MustNormalizeCWDRelPath(ts_gen_out_raw).Str(),
	)

	// buildtime_public_url_func_name
	buildtime_func := strings.TrimSpace(raw.BuildtimePublicURLFuncName)
	if buildtime_func == "" {
		buildtime_func = "waveBuildtimeURL"
	}

	s.cfg = &parsed_vorma_config{
		user_root_dir:                    ctx.UserRootDir(),
		ui_variant:                       ui_variant,
		root_template_path:               root_template_path,
		client_entry_path:                client_entry_path,
		client_route_definition_patterns: client_route_definition_patterns,
		ts_gen_out_dir:                   ts_gen_out_dir,
		buildtime_public_url_func_name:   buildtime_func,
	}

	// update plugin hooks now that we have parsed config
	exclude := []strict.CWDRelPath{
		strict.CWDRelPath(s.cfg.ts_gen_out_dir.Str()),
	}

	s.plugin.LifecycleHooks = []wavebuild.LifecycleHook{
		// main build hook — full build on Go file changes
		{
			Name: "vorma2 build",
			WatchIncludePatterns: []strict.CWDRelPath{
				strict.CWDRelPath("**/*.go"),
			},
			WatchExcludePatterns: exclude,
			Fn:                   s.build_hook,
			StartAt:              wavebuild.Checkpoint_1_CycleStart,
			FinishBy:             wavebuild.Checkpoint_3_FullPublicFilemapFinalized,
			Effects: []wavebuild.Effect{
				wavebuild.EffectHardReloadBrowser,
			},
		},
		// route fast path — routes-only rebuild without Go compile
		{
			Name:                 "vorma2 routes fast",
			WatchIncludePatterns: s.cfg.client_route_definition_patterns,
			WatchExcludePatterns: exclude,
			Fn:                   s.route_fast_path_hook,
			StartAt:              wavebuild.Checkpoint_1_CycleStart,
			FinishBy:             wavebuild.Checkpoint_3_FullPublicFilemapFinalized,
			Effects: []wavebuild.Effect{
				wavebuild.EffectHardReloadBrowser,
			},
			DevOnly: true,
		},
		// template reload — browser reload on template edit
		{
			Name: "vorma2 template reload",
			WatchIncludePatterns: []strict.CWDRelPath{
				ctx.PrivateStaticSourceDir().Join(
					s.cfg.root_template_path.Str(),
				),
			},
			Effects: []wavebuild.Effect{wavebuild.EffectHardReloadBrowser},
			DevOnly: true,
		},
		// filemap refresh — regenerate filemap.ts on public static changes
		{
			Name: "vorma2 filemap refresh",
			WatchIncludePatterns: []strict.CWDRelPath{
				ctx.PublicStaticSourceDir(),
			},
			WatchExcludePatterns: exclude,
			Fn:                   s.filemap_refresh_hook,
			StartAt:              wavebuild.Checkpoint_1_CycleStart,
			FinishBy:             wavebuild.Checkpoint_3_FullPublicFilemapFinalized,
			Effects: []wavebuild.Effect{
				wavebuild.EffectHardReloadBrowser,
			},
			DevOnly: true,
		},
	}

	return nil
}

func (s *plugin_state) vorma_runtime_dir(
	ctx *wavebuild.PluginCtx,
) strict.CWDRelPath {
	return ctx.WaveOutRuntimeStaticDir().Join(constants.RUNTIME_DIRNAME)
}
