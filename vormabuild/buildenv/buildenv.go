// Package buildenv owns Vorma build-environment configuration wiring.
//
// It centralizes schema registration and framework build-hook wiring so
// vormabuild orchestration can consume a single configuration entrypoint without
// duplicating hook/overlay setup logic.
package buildenv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/vormabuild/backendroutes"
	"github.com/vormadev/vorma/vormabuild/backendroutes/registraroverlay"
	"github.com/vormadev/vorma/vormabuild/devreload"
	"github.com/vormadev/vorma/wave"
)

type frameworkBuildHookExecutionDependencies struct {
	prepareDiscoveredRouteRegistrarOverlayWithArtifactCache func(
		*vormaruntime.Vorma,
		*registraroverlay.DiscoveredRouteRegistrarArtifactCache,
	) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error)
	runGoCommandWithContext func(context.Context, []string) error
}

type frameworkBuildHookExecutor struct {
	dependencies frameworkBuildHookExecutionDependencies
}

var defaultFrameworkBuildHookExecutor = newFrameworkBuildHookExecutor(
	frameworkBuildHookExecutionDependencies{},
)

func defaultFrameworkBuildHookExecutionDependencies() frameworkBuildHookExecutionDependencies {
	return frameworkBuildHookExecutionDependencies{
		prepareDiscoveredRouteRegistrarOverlayWithArtifactCache: backendroutes.PrepareDiscoveredRouteRegistrarOverlayWithArtifactCache,
		runGoCommandWithContext: func(
			commandExecutionContext context.Context,
			goArguments []string,
		) error {
			if commandExecutionContext == nil {
				commandExecutionContext = context.Background()
			}

			goCommand := exec.CommandContext(
				commandExecutionContext,
				"go",
				goArguments...)
			goCommand.Stdout = os.Stdout
			goCommand.Stderr = os.Stderr
			return goCommand.Run()
		},
	}
}

func normalizeFrameworkBuildHookExecutionDependencies(
	dependencies frameworkBuildHookExecutionDependencies,
) frameworkBuildHookExecutionDependencies {
	defaultDependencies := defaultFrameworkBuildHookExecutionDependencies()
	if dependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache == nil {
		dependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache = defaultDependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache
	}
	if dependencies.runGoCommandWithContext == nil {
		dependencies.runGoCommandWithContext = defaultDependencies.runGoCommandWithContext
	}
	return dependencies
}

func newFrameworkBuildHookExecutor(
	dependencies frameworkBuildHookExecutionDependencies,
) frameworkBuildHookExecutor {
	return frameworkBuildHookExecutor{
		dependencies: normalizeFrameworkBuildHookExecutionDependencies(
			dependencies,
		),
	}
}

func Configure(v *vormaruntime.Vorma) *wave.ParsedConfig {
	return ConfigureInConfig(v, v.Wave.BuildtimeParsedConfig())
}

func ConfigureInConfig(
	v *vormaruntime.Vorma,
	cfg *wave.ParsedConfig,
) *wave.ParsedConfig {
	return configureInConfigWithFrameworkBuildHookExecutor(
		v,
		cfg,
		defaultFrameworkBuildHookExecutor,
	)
}

func configureInConfigWithFrameworkBuildHookExecutor(
	v *vormaruntime.Vorma,
	cfg *wave.ParsedConfig,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) *wave.ParsedConfig {
	if cfg == nil {
		return nil
	}

	discoveredRegistrarArtifactsCache := registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
		registraroverlay.DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
	)

	registerVormaSchemaInConfig(cfg)
	devreload.InjectDefaultWatchPatternsInConfig(cfg, v)
	injectFrameworkBuildHooksInConfig(cfg, v)
	injectFrameworkBuildHookRunnerInConfig(
		cfg,
		v,
		discoveredRegistrarArtifactsCache,
		frameworkBuildHookExecutor,
	)
	injectFrameworkGoBuildOverlayPreparationInConfig(
		cfg,
		v,
		discoveredRegistrarArtifactsCache,
		frameworkBuildHookExecutor,
	)
	return cfg
}

func registerVormaSchemaInConfig(cfg *wave.ParsedConfig) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkSchemaExtensions == nil {
		cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	cfg.FrameworkSchemaExtensions["Vorma"] = vormaSchema
}

func injectFrameworkBuildHooksInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkDevBuildHook == "" {
		cfg.FrameworkDevBuildHook = fmt.Sprintf(
			"go run ./%s --dev --hook",
			v.Config.MainBuildEntry,
		)
	}
	if cfg.FrameworkProdBuildHook == "" {
		cfg.FrameworkProdBuildHook = fmt.Sprintf(
			"go run ./%s --hook",
			v.Config.MainBuildEntry,
		)
	}
}

func injectFrameworkBuildHookRunnerInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
	discoveredRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkRunBuildHook != nil {
		return
	}

	cfg.FrameworkRunBuildHook = func(
		commandExecutionContext context.Context,
		runInDevelopmentMode bool,
	) error {
		if v == nil {
			return errors.New("vorma runtime is required")
		}
		if v.Config == nil {
			return errors.New("vorma config is required")
		}
		mainBuildEntry := strings.TrimSpace(v.Config.MainBuildEntry)
		if mainBuildEntry == "" {
			return errors.New("vorma config MainBuildEntry is required")
		}
		if commandExecutionContext == nil {
			commandExecutionContext = context.Background()
		}

		goRunArgs := []string{"run"}
		discoveredRouteRegistrarOverlay, err := frameworkBuildHookExecutor.dependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
			v,
			discoveredRegistrarArtifactsCache,
		)
		if err != nil {
			return fmt.Errorf(
				"prepare discovered route registrar overlay for framework build hook: %w",
				err,
			)
		}
		if discoveredRouteRegistrarOverlay != nil &&
			strings.TrimSpace(
				discoveredRouteRegistrarOverlay.GoOverlayConfigPath(),
			) != "" {
			goRunArgs = append(
				goRunArgs,
				"-overlay="+discoveredRouteRegistrarOverlay.GoOverlayConfigPath(),
			)
		}

		trimmedMainBuildEntry := strings.TrimPrefix(mainBuildEntry, "./")
		goRunArgs = append(goRunArgs, "./"+trimmedMainBuildEntry)
		if runInDevelopmentMode {
			goRunArgs = append(goRunArgs, "--dev")
		}
		goRunArgs = append(goRunArgs, "--hook")

		runHookCommandErr := frameworkBuildHookExecutor.dependencies.runGoCommandWithContext(
			commandExecutionContext,
			goRunArgs,
		)
		var cleanupOverlayErr error
		if discoveredRouteRegistrarOverlay != nil {
			cleanupOverlayErr = discoveredRouteRegistrarOverlay.Cleanup()
		}
		if runHookCommandErr != nil {
			if cleanupOverlayErr != nil {
				return fmt.Errorf(
					"run framework build hook command: %w (cleanup discovered route registrar overlay failed: %v)",
					runHookCommandErr,
					cleanupOverlayErr,
				)
			}
			return fmt.Errorf(
				"run framework build hook command: %w",
				runHookCommandErr,
			)
		}
		if cleanupOverlayErr != nil {
			return fmt.Errorf(
				"cleanup discovered route registrar overlay after framework build hook: %w",
				cleanupOverlayErr,
			)
		}
		return nil
	}
}

func injectFrameworkGoBuildOverlayPreparationInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
	discoveredRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkPrepareGoBuildOverlay != nil {
		return
	}

	cfg.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		discoveredRouteRegistrarOverlay, err := frameworkBuildHookExecutor.dependencies.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
			v,
			discoveredRegistrarArtifactsCache,
		)
		if err != nil {
			return nil, err
		}
		if discoveredRouteRegistrarOverlay == nil {
			return nil, nil
		}

		return &wave.GoBuildOverlay{
			OverlayConfigPath: discoveredRouteRegistrarOverlay.GoOverlayConfigPath(),
			Cleanup: func() error {
				return discoveredRouteRegistrarOverlay.Cleanup()
			},
		}, nil
	}
}

var vormaSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: "Vorma framework configuration.",
	RequiredChildren: []string{
		"MainBuildEntry",
		"UIVariant",
		"HTMLTemplateLocation",
		"ClientEntry",
		"ClientRouteDefinitionPatterns",
		"TSGenOutDir",
	},
	Properties: struct {
		IncludeDefaults               jsonschema.Entry
		MainBuildEntry                jsonschema.Entry
		UIVariant                     jsonschema.Entry
		HTMLTemplateLocation          jsonschema.Entry
		ClientEntry                   jsonschema.Entry
		ClientRouteDefinitionPatterns jsonschema.Entry
		ServerRouteDefinitionPatterns jsonschema.Entry
		TSGenOutDir                   jsonschema.Entry
		BuildtimePublicURLFuncName    jsonschema.Entry
		UnresolvedRoutePolicy         jsonschema.Entry
		DevReloadRoutesEndpointPath   jsonschema.Entry
		DevReloadTemplateEndpointPath jsonschema.Entry
		TemplateDataKeyHeadElements   jsonschema.Entry
		TemplateDataKeyBodyScripts    jsonschema.Entry
		TemplateDataKeySSRScript      jsonschema.Entry
		TemplateDataKeySSRScriptHash  jsonschema.Entry
		TemplateDataKeyRootElementID  jsonschema.Entry
		ClientRootElementID           jsonschema.Entry
	}{
		IncludeDefaults:               includeDefaultsSchema,
		MainBuildEntry:                mainBuildEntrySchema,
		UIVariant:                     uiVariantSchema,
		HTMLTemplateLocation:          htmlTemplateLocationSchema,
		ClientEntry:                   clientEntrySchema,
		ClientRouteDefinitionPatterns: clientRouteDefinitionPatternsSchema,
		ServerRouteDefinitionPatterns: serverRouteDefinitionPatternsSchema,
		TSGenOutDir:                   tsGenOutDirSchema,
		BuildtimePublicURLFuncName:    buildtimePublicURLFuncNameSchema,
		UnresolvedRoutePolicy:         unresolvedRoutePolicySchema,
		DevReloadRoutesEndpointPath:   devReloadRoutesEndpointPathSchema,
		DevReloadTemplateEndpointPath: devReloadTemplateEndpointPathSchema,
		TemplateDataKeyHeadElements:   templateDataKeyHeadElementsSchema,
		TemplateDataKeyBodyScripts:    templateDataKeyBodyScriptsSchema,
		TemplateDataKeySSRScript:      templateDataKeySSRScriptSchema,
		TemplateDataKeySSRScriptHash:  templateDataKeySSRScriptHashSchema,
		TemplateDataKeyRootElementID:  templateDataKeyRootElementIDSchema,
		ClientRootElementID:           clientRootElementIDSchema,
	},
})

var includeDefaultsSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true (default), Vorma injects default watch patterns for routes, templates, and Go files.`,
	Default:     true,
})

var mainBuildEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to the Vorma build command entry point.`,
	Examples:    []string{"backend/cmd/build", "cmd/build"},
})

var uiVariantSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `The UI framework to use for client-side rendering.`,
	Enum:        []string{"react", "preact", "solid"},
})

var htmlTemplateLocationSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your HTML template file, relative to the private static directory.`,
	Examples:    []string{"entry.go.html"},
})

var clientEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your client-side entry file.`,
	Examples:    []string{"frontend/src/vorma.entry.tsx"},
})

var clientRouteDefinitionPatternsSchema = jsonschema.RequiredArray(
	jsonschema.Def{
		Description: `Glob patterns that resolve to client route definition files.`,
		Items:       jsonschema.RequiredString(jsonschema.Def{}),
		Examples: []string{
			"frontend/src/**/*vorma.routes.ts",
			"frontend/src/routes/core.vorma.routes.ts",
		},
	},
)

var serverRouteDefinitionPatternsSchema = jsonschema.OptionalArray(
	jsonschema.Def{
		Description: `Optional backend route definition patterns to merge into the route manifest for server-only handlers.`,
		Items:       jsonschema.OptionalString(jsonschema.Def{}),
		Examples:    []string{"backend/src/**/*vorma.routes.go"},
	},
)

var tsGenOutDirSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Directory where Vorma generates TypeScript route artifacts and filemap outputs.`,
	Examples:    []string{"frontend/src/vorma.gen"},
})

var buildtimePublicURLFuncNameSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Name of the global function injected by the Vite plugin for resolving public asset URLs at build time.`,
	Default:     "waveBuildtimeURL",
	Examples:    []string{"waveBuildtimeURL", "getAssetURL"},
})

var unresolvedRoutePolicySchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `How unresolved route module expressions are handled. Defaults to "warn" in dev and "error" in production.`,
	Enum:        []string{"warn", "error"},
	Examples:    []string{"warn", "error"},
})

var devReloadRoutesEndpointPathSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Dev-only endpoint path that triggers in-process route reload.`,
		Default:     "/__vorma_internal/reload-routes",
		Examples:    []string{"/__vorma_internal/reload-routes"},
	},
)

var devReloadTemplateEndpointPathSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Dev-only endpoint path that triggers in-process template reload.`,
		Default:     "/__vorma_internal/reload-template",
		Examples:    []string{"/__vorma_internal/reload-template"},
	},
)

var templateDataKeyHeadElementsSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Template data key for serialized head elements.`,
		Default:     "VormaHeadEls",
		Examples:    []string{"VormaHeadEls"},
	},
)

var templateDataKeyBodyScriptsSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for serialized body script tags.`,
	Default:     "VormaBodyScripts",
	Examples:    []string{"VormaBodyScripts"},
})

var templateDataKeySSRScriptSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for the SSR bootstrap script.`,
	Default:     "VormaSSRScript",
	Examples:    []string{"VormaSSRScript"},
})

var templateDataKeySSRScriptHashSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Template data key for the SSR script CSP hash.`,
		Default:     "VormaSSRScriptSha256Hash",
		Examples:    []string{"VormaSSRScriptSha256Hash"},
	},
)

var templateDataKeyRootElementIDSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Template data key for the root element ID placeholder.`,
		Default:     "VormaRootID",
		Examples:    []string{"VormaRootID"},
	},
)

var clientRootElementIDSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Client root element ID used for hydration/mount.`,
	Default:     "vorma-root",
	Examples:    []string{"vorma-root"},
})
