package vormabuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/vormadev/vorma/internal/pkg/jsonschema"
	"github.com/vormadev/vorma/kit/fsutil"
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
	ServerEntry                   string
	ClientEntry                   string
	ClientRouteDefinitionPatterns []string
	GenOutDir                     string
	BuildtimePublicURLFuncName    string
	GoCompileCmd                  string
	GoCompileCmdDev               string
	GoCompileCmdProd              string
}

type parsed_vorma_config struct {
	user_root_dir                    strict.CWDRelPath
	ui_variant                       string
	root_template_path               types.PrivateFSRelPath
	server_entry                     string
	client_entry_path                strict.CWDRelPath
	client_route_definition_patterns []strict.CWDRelPath
	gen_out_dir                      strict.CWDRelPath
	buildtime_public_url_func_name   string
	go_compile_cmd_dev               string
	go_compile_cmd_prod              string
}

type plugin_state struct {
	plugin *wavebuild.Plugin
	cfg    *parsed_vorma_config
	app    *vorma2.Vorma

	// Set to true at build_hook entry, reset via defer at exit.
	// Checked by route_fast_path_hook to skip redundant work
	// when both hooks fire in the same cycle.
	build_hook_active atomic.Bool
}

func NewPlugin(app *vorma2.Vorma) *wavebuild.Plugin {
	if app == nil {
		panic("[vorma2]: app is required")
	}
	state := &plugin_state{app: app}
	plugin := &wavebuild.Plugin{
		Name: "vorma",
		Config: wavebuild.PluginConfig{
			JSONKey:    json_cfg_key,
			JSONSchema: vorma_schema,
			ParseFunc:  state.parse_config,
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

	// root_template_path
	root_template_path := types.PrivateFSRelPath(
		filepath.Clean(strings.TrimSpace(raw.HTMLTemplateLocation)),
	)
	if root_template_path == "" || root_template_path == "." {
		return fmt.Errorf("Vorma.HTMLTemplateLocation is required")
	}
	if filepath.IsAbs(root_template_path.Str()) {
		return fmt.Errorf(
			"Vorma.HTMLTemplateLocation must be private-FS-relative, not absolute",
		)
	}

	// client_entry_path
	client_entry_raw := strings.TrimSpace(raw.ClientEntry)
	if client_entry_raw == "" {
		return fmt.Errorf("Vorma.ClientEntry is required")
	}

	// server_entry
	server_entry := strings.TrimSpace(raw.ServerEntry)
	if server_entry == "" {
		return fmt.Errorf("Vorma.ServerEntry is required")
	}
	if !strings.HasPrefix(server_entry, ".") {
		server_entry = "./" + server_entry
	}
	client_entry_path := ctx.UserRootDir().Join(
		strict.MustNormalizeCWDRelPath(client_entry_raw).Str(),
	)

	// client_route_definition_patterns
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

	// gen_out_dir
	gen_out_raw := strings.TrimSpace(raw.GenOutDir)
	if gen_out_raw == "" {
		return fmt.Errorf("Vorma.GenOutDir is required")
	}
	gen_out_dir := ctx.UserRootDir().Join(
		strict.MustNormalizeCWDRelPath(gen_out_raw).Str(),
	)

	// buildtime_public_url_func_name
	buildtime_func := strings.TrimSpace(raw.BuildtimePublicURLFuncName)
	if buildtime_func == "" {
		buildtime_func = "waveBuildtimeURL"
	}

	// go compile commands
	go_compile_cmd := strings.TrimSpace(raw.GoCompileCmd)
	go_compile_cmd_dev := strings.TrimSpace(raw.GoCompileCmdDev)
	go_compile_cmd_prod := strings.TrimSpace(raw.GoCompileCmdProd)

	has_unified := go_compile_cmd != ""
	has_dev := go_compile_cmd_dev != ""
	has_prod := go_compile_cmd_prod != ""

	if has_unified && (has_dev || has_prod) {
		return fmt.Errorf(
			"Vorma.GoCompileCmd cannot be combined with GoCompileCmdDev/GoCompileCmdProd",
		)
	}
	if has_dev != has_prod {
		return fmt.Errorf(
			"Vorma.GoCompileCmdDev and GoCompileCmdProd must both be set or both omitted",
		)
	}

	var resolved_dev, resolved_prod string
	default_cmd := fmt.Sprintf(
		`go build -tags "$WAVE_BUILD_TAGS" -o "$WAVE_BIN_OUTPUT_PATH" %s`,
		server_entry,
	)
	if has_unified {
		resolved_dev = go_compile_cmd
		resolved_prod = go_compile_cmd
	} else if has_dev {
		resolved_dev = go_compile_cmd_dev
		resolved_prod = go_compile_cmd_prod
	} else {
		resolved_dev = default_cmd
		resolved_prod = default_cmd
	}

	s.cfg = &parsed_vorma_config{
		user_root_dir:                    ctx.UserRootDir(),
		ui_variant:                       ui_variant,
		root_template_path:               root_template_path,
		server_entry:                     server_entry,
		client_entry_path:                client_entry_path,
		client_route_definition_patterns: client_route_definition_patterns,
		gen_out_dir:                      gen_out_dir,
		buildtime_public_url_func_name:   buildtime_func,
		go_compile_cmd_dev:               resolved_dev,
		go_compile_cmd_prod:              resolved_prod,
	}

	// ensure gen dir has placeholder files so Vite can start
	if err := fsutil.EnsureDir(s.cfg.gen_out_dir.Str()); err != nil {
		return fmt.Errorf("creating gen output dir: %w", err)
	}
	if err := s.write_placeholders(s.cfg, ctx); err != nil {
		return fmt.Errorf("writing placeholders: %w", err)
	}

	// update plugin hooks now that we have parsed config
	exclude := []strict.CWDRelPath{
		strict.CWDRelPath(s.cfg.gen_out_dir.Str()),
	}

	go_watch := []strict.CWDRelPath{
		strict.CWDRelPath("**/*.go"),
	}

	s.plugin.LifecycleHooks = []wavebuild.LifecycleHook{
		{
			Name:                 "vorma2 build",
			WatchIncludePatterns: go_watch,
			WatchExcludePatterns: exclude,
			Fn:                   s.build_hook,
			Timing:               []int{1, 3},
			Effects: []wavebuild.Effect{
				wavebuild.EffectHardReloadBrowser,
			},
		},
		{
			Name:                 "vorma2 routes fast",
			WatchIncludePatterns: s.cfg.client_route_definition_patterns,
			WatchExcludePatterns: exclude,
			Fn:                   s.route_fast_path_hook,
			Timing:               []int{1, 3},
			Effects: []wavebuild.Effect{
				wavebuild.EffectHardReloadBrowser,
			},
			DevOnly:         true,
			IncrementalOnly: true,
		},
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
		{
			Name: "vorma2 filemap refresh",
			WatchIncludePatterns: []strict.CWDRelPath{
				ctx.PublicStaticSourceDir(),
			},
			WatchExcludePatterns: exclude,
			Fn:                   s.filemap_refresh_hook,
			Timing:               []int{1, 3},
			Effects: []wavebuild.Effect{
				wavebuild.EffectHardReloadBrowser,
			},
			DevOnly:         true,
			IncrementalOnly: true,
		},
	}

	// Go compile hooks
	if s.cfg.go_compile_cmd_dev == s.cfg.go_compile_cmd_prod {
		s.plugin.LifecycleHooks = append(
			s.plugin.LifecycleHooks,
			wavebuild.LifecycleHook{
				Name:                 "vorma2 go compile",
				WatchIncludePatterns: go_watch,
				WatchExcludePatterns: exclude,
				Cmd:                  s.cfg.go_compile_cmd_dev,
				Timing:               []int{4},
				Effects: []wavebuild.Effect{
					wavebuild.EffectRestartApp,
				},
			},
		)
	} else {
		s.plugin.LifecycleHooks = append(
			s.plugin.LifecycleHooks,
			wavebuild.LifecycleHook{
				Name:                 "vorma2 go compile",
				WatchIncludePatterns: go_watch,
				WatchExcludePatterns: exclude,
				Cmd:                  s.cfg.go_compile_cmd_dev,
				Timing:               []int{4},
				Effects:              []wavebuild.Effect{wavebuild.EffectRestartApp},
				DevOnly:              true,
			},
			wavebuild.LifecycleHook{
				Name:                 "vorma2 go compile",
				WatchIncludePatterns: go_watch,
				WatchExcludePatterns: exclude,
				Cmd:                  s.cfg.go_compile_cmd_prod,
				Timing:               []int{4},
				Effects:              []wavebuild.Effect{wavebuild.EffectRestartApp},
				ProdOnly:             true,
			},
		)
	}

	return nil
}

func (s *plugin_state) vorma_runtime_dir(
	ctx *wavebuild.PluginCtx,
) strict.CWDRelPath {
	return ctx.WaveOutRuntimeStaticDir().Join(constants.RUNTIME_DIRNAME)
}

func (s *plugin_state) write_placeholders(
	cfg *parsed_vorma_config,
	ctx *wavebuild.PluginConfigParseCtx,
) error {
	gen := cfg.gen_out_dir.Str()

	// filemap.json placeholder
	if err := os.WriteFile(
		filepath.Join(gen, constants.GENERATED_JSON_FILEMAP_FILENAME),
		[]byte("{}"),
		0644,
	); err != nil {
		return fmt.Errorf(
			"writing %s placeholder: %w",
			constants.GENERATED_JSON_FILEMAP_FILENAME,
			err,
		)
	}

	// filemap.ts placeholder
	if err := os.WriteFile(
		filepath.Join(gen, constants.GENERATED_TS_FILEMAP_FILENAME),
		[]byte("export const staticPublicAssetMap = {} as const;\n"),
		0644,
	); err != nil {
		return fmt.Errorf(
			"writing %s placeholder: %w",
			constants.GENERATED_TS_FILEMAP_FILENAME,
			err,
		)
	}

	// index.ts placeholder — uses real config values via shared renderers
	app_cfg_str := render_app_config_const(
		extract_app_config_params(s.app, false),
	)

	wave_out_dir := filepath.ToSlash(ctx.WaveOutDir().Str())
	gen_dir := filepath.ToSlash(gen)

	vite_cfg_str := render_vite_config_block(vite_config_params{
		RollupInput:      []string{},
		PublicPathPrefix: ctx.PublicPathPrefix(),
		FuncName:         cfg.buildtime_public_url_func_name,
		IgnoredPatterns:  compute_ignored_patterns(wave_out_dir, gen_dir),
		DedupeList:       dedupe_for_variant(cfg.ui_variant),
	})

	var sb strings.Builder
	sb.WriteString("const routes = [] as const;\n")
	sb.WriteString(app_cfg_str)
	sb.WriteString(vite_cfg_str)

	if err := os.WriteFile(
		filepath.Join(gen, constants.GENERATED_TS_INDEX_FILENAME),
		[]byte(sb.String()),
		0644,
	); err != nil {
		return fmt.Errorf("writing index.ts placeholder: %w", err)
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
/////// JSON Schema
/////////////////////////////////////////////////////////////////////

var vorma_schema = jsonschema.RequiredObject(jsonschema.Def{
	Description: "Vorma framework configuration.",
	RequiredChildren: []string{
		"UIVariant",
		"HTMLTemplateLocation",
		"ServerEntry",
		"ClientEntry",
		"ClientRouteDefinitionPatterns",
		"GenOutDir",
	},
	Properties: map[string]jsonschema.Entry{
		"UIVariant": jsonschema.RequiredString(jsonschema.Def{
			Description: "UI framework variant.",
			Enum:        []string{"react", "preact", "solid"},
		}),
		"HTMLTemplateLocation": jsonschema.RequiredString(jsonschema.Def{
			Description: "Path to the root HTML template, relative to the private static assets directory.",
			Examples:    []string{"root.html"},
		}),
		"ServerEntry": jsonschema.RequiredString(jsonschema.Def{
			Description: "Go package path for the server entry point, relative to RootDir. Used in the default Go compile command.",
			Examples:    []string{".", "./cmd/server"},
		}),
		"ClientEntry": jsonschema.RequiredString(jsonschema.Def{
			Description: "Client-side entry point file, relative to RootDir.",
			Examples:    []string{"frontend/src/entry.tsx"},
		}),
		"ClientRouteDefinitionPatterns": jsonschema.RequiredArray(
			jsonschema.Def{
				Description: "Glob patterns matching route definition files, relative to RootDir. Must have at least one pattern.",
				Items:       jsonschema.Entry{Type: jsonschema.TypeString},
				MinItems:    1,
			},
		),
		"GenOutDir": jsonschema.RequiredString(jsonschema.Def{
			Description: "Output directory for generated TypeScript and Go files, relative to RootDir.",
			Examples:    []string{"frontend/src/gen/vorma"},
		}),
		"BuildtimePublicURLFuncName": jsonschema.OptionalString(jsonschema.Def{
			Description: "Name of the global function used in client code to resolve public asset URLs at build time.",
			Default:     "waveBuildtimeURL",
		}),
		"GoCompileCmd": jsonschema.OptionalString(jsonschema.Def{
			Description: "Shell command for Go compilation, used for both dev and prod builds. Mutually exclusive with GoCompileCmdDev/GoCompileCmdProd. Defaults to: go build -tags \"$WAVE_BUILD_TAGS\" -o \"$WAVE_BIN_OUTPUT_PATH\" <ServerEntry>.",
			Examples: []string{
				`go build -tags "$WAVE_BUILD_TAGS" -o "$WAVE_BIN_OUTPUT_PATH" ./cmd/server`,
			},
		}),
		"GoCompileCmdDev": jsonschema.OptionalString(jsonschema.Def{
			Description: "Shell command for Go compilation in dev mode. Must be paired with GoCompileCmdProd. Mutually exclusive with GoCompileCmd.",
		}),
		"GoCompileCmdProd": jsonschema.OptionalString(jsonschema.Def{
			Description: "Shell command for Go compilation in prod mode. Must be paired with GoCompileCmdDev. Mutually exclusive with GoCompileCmd.",
		}),
	},
})
