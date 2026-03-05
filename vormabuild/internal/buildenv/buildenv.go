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
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/vormabuild/internal/backendroutes"
	"github.com/vormadev/vorma/vormabuild/internal/backendroutes/registraroverlay"
	"github.com/vormadev/vorma/vormabuild/internal/devreload"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"
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

func Configure(v *vormaruntime.Vorma) waveconfig.ParsedConfig {
	if v == nil || v.Wave == nil {
		return nil
	}
	return ConfigureInConfig(
		v,
		v.Wave.ParsedConfig(),
	)
}

func ConfigureInConfig(
	v *vormaruntime.Vorma,
	cfg waveconfig.ParsedConfig,
) waveconfig.ParsedConfig {
	return configureInConfigWithFrameworkBuildHookExecutor(
		v,
		cfg,
		defaultFrameworkBuildHookExecutor,
	)
}

func configureInConfigWithFrameworkBuildHookExecutor(
	v *vormaruntime.Vorma,
	cfg waveconfig.ParsedConfig,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) waveconfig.ParsedConfig {
	if cfg == nil {
		return nil
	}

	discoveredRegistrarArtifactsCache := registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
		registraroverlay.DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
	)

	registerVormaSchemaInConfig(cfg)
	injectFrameworkToolingReloadConfiguratorInConfig(
		cfg,
		v,
		frameworkBuildHookExecutor,
	)
	injectFrameworkPublicFileMapReloadEndpointInConfig(cfg, v)
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

func injectFrameworkPublicFileMapReloadEndpointInConfig(
	cfg waveconfig.ParsedConfig,
	v *vormaruntime.Vorma,
) {
	if cfg == nil || v == nil {
		return
	}
	if v.Config == nil {
		return
	}
	waveframework.StateForConfig(cfg).PublicFileMapReloadEndpointPath = v.
		DevReloadPublicFileMapEndpointPath()
}

func injectFrameworkToolingReloadConfiguratorInConfig(
	cfg waveconfig.ParsedConfig,
	v *vormaruntime.Vorma,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) {
	if cfg == nil || v == nil {
		return
	}
	waveframework.StateForConfig(cfg).ConfigureForToolingReload = func(
		reloadedParsedConfig waveconfig.ParsedConfig,
		rawWaveConfigJSON []byte,
	) error {
		if reloadedParsedConfig == nil {
			return errors.New("reloaded wave config is required")
		}
		if len(rawWaveConfigJSON) == 0 {
			return errors.New("raw wave config JSON is required")
		}
		reloadedVormaConfig, parseError := runtimeconfig.ParseVormaConfigJSON(
			rawWaveConfigJSON,
			reloadedParsedConfig,
		)
		if parseError != nil {
			return fmt.Errorf(
				"parse Vorma config for tooling reload: %w",
				parseError,
			)
		}
		v.Config = reloadedVormaConfig
		configureInConfigWithFrameworkBuildHookExecutor(
			v,
			reloadedParsedConfig,
			frameworkBuildHookExecutor,
		)
		return nil
	}
}

func registerVormaSchemaInConfig(cfg waveconfig.ParsedConfig) {
	if cfg == nil {
		return
	}
	if waveframework.StateForConfig(cfg).SchemaExtensions == nil {
		waveframework.StateForConfig(cfg).SchemaExtensions = make(map[string]jsonschema.Entry)
	}
	waveframework.StateForConfig(cfg).SchemaExtensions["Vorma"] = vormaSchema
}

func injectFrameworkBuildHooksInConfig(
	cfg waveconfig.ParsedConfig,
	v *vormaruntime.Vorma,
) {
	if cfg == nil {
		return
	}
	mainBuildEntryGoRunTarget, normalizeMainBuildEntryError := normalizeMainBuildEntryForGoRun(
		cfg,
		v.Config.MainBuildEntry(),
	)
	if normalizeMainBuildEntryError != nil {
		trimmedMainBuildEntry := strings.TrimSpace(v.Config.MainBuildEntry())
		mainBuildEntryGoRunTarget = strings.TrimPrefix(trimmedMainBuildEntry, "./")
	}
	if waveframework.StateForConfig(cfg).DevBuildHook == "" {
		waveframework.StateForConfig(cfg).DevBuildHook = fmt.Sprintf(
			"go run ./%s --dev --hook",
			mainBuildEntryGoRunTarget,
		)
	}
	if waveframework.StateForConfig(cfg).ProdBuildHook == "" {
		waveframework.StateForConfig(cfg).ProdBuildHook = fmt.Sprintf(
			"go run ./%s --hook",
			mainBuildEntryGoRunTarget,
		)
	}
}

func injectFrameworkBuildHookRunnerInConfig(
	cfg waveconfig.ParsedConfig,
	v *vormaruntime.Vorma,
	discoveredRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) {
	if cfg == nil {
		return
	}
	if waveframework.StateForConfig(cfg).RunBuildHook != nil {
		return
	}

	waveframework.StateForConfig(cfg).RunBuildHook = func(
		commandExecutionContext context.Context,
		runInDevelopmentMode bool,
	) error {
		if v == nil {
			return errors.New("vorma runtime is required")
		}
		if v.Config == nil {
			return errors.New("vorma config is required")
		}
		mainBuildEntry := strings.TrimSpace(v.Config.MainBuildEntry())
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

		mainBuildEntryGoRunTarget, normalizeMainBuildEntryError := normalizeMainBuildEntryForGoRun(
			cfg,
			mainBuildEntry,
		)
		if normalizeMainBuildEntryError != nil {
			return fmt.Errorf(
				"normalize MainBuildEntry for go run: %w",
				normalizeMainBuildEntryError,
			)
		}
		goRunArgs = append(goRunArgs, "./"+mainBuildEntryGoRunTarget)
		if runInDevelopmentMode {
			goRunArgs = append(goRunArgs, "--dev")
		}
		goRunArgs = append(goRunArgs, "--hook-inner")

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

func normalizeMainBuildEntryForGoRun(
	cfg waveconfig.ParsedConfig,
	mainBuildEntry string,
) (string, error) {
	trimmedMainBuildEntry := strings.TrimSpace(mainBuildEntry)
	if trimmedMainBuildEntry == "" {
		return "", errors.New("main build entry is required")
	}

	resolveRootPath := ""
	if cfg != nil {
		resolveRootPath = cfg.ResolveRoot()
	}
	resolveRootRelativeMainBuildEntry, relativeMainBuildEntryError := runtimeconfig.NormalizePathOrPatternToResolveRootRelative(
		resolveRootPath,
		trimmedMainBuildEntry,
	)
	if relativeMainBuildEntryError != nil {
		return "", relativeMainBuildEntryError
	}

	normalizedMainBuildEntry := filepath.ToSlash(
		filepath.Clean(resolveRootRelativeMainBuildEntry),
	)
	normalizedMainBuildEntry = strings.TrimPrefix(normalizedMainBuildEntry, "./")
	if normalizedMainBuildEntry == "" || normalizedMainBuildEntry == "." {
		return "", errors.New("main build entry resolved to empty path")
	}
	return normalizedMainBuildEntry, nil
}

func injectFrameworkGoBuildOverlayPreparationInConfig(
	cfg waveconfig.ParsedConfig,
	v *vormaruntime.Vorma,
	discoveredRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) {
	if cfg == nil {
		return
	}
	if waveframework.StateForConfig(cfg).PrepareGoBuildOverlay != nil {
		return
	}

	waveframework.StateForConfig(cfg).PrepareGoBuildOverlay = func() (*waveframework.GoBuildOverlay, error) {
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

		return &waveframework.GoBuildOverlay{
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
		IncludeDefaults                    jsonschema.Entry
		MainBuildEntry                     jsonschema.Entry
		UIVariant                          jsonschema.Entry
		HTMLTemplateLocation               jsonschema.Entry
		ClientEntry                        jsonschema.Entry
		ClientRouteDefinitionPatterns      jsonschema.Entry
		ServerRouteDefinitionPatterns      jsonschema.Entry
		TSGenOutDir                        jsonschema.Entry
		BuildtimePublicURLFuncName         jsonschema.Entry
		UnresolvedRoutePolicy              jsonschema.Entry
		DevReloadRoutesEndpointPath        jsonschema.Entry
		DevReloadTemplateEndpointPath      jsonschema.Entry
		DevReloadPublicFileMapEndpointPath jsonschema.Entry
		TemplateDataKeyHeadElements        jsonschema.Entry
		TemplateDataKeyBodyScripts         jsonschema.Entry
		TemplateDataKeySSRScript           jsonschema.Entry
		TemplateDataKeySSRScriptHash       jsonschema.Entry
		TemplateDataKeyRootElementID       jsonschema.Entry
		ClientRootElementID                jsonschema.Entry
	}{
		IncludeDefaults:                    includeDefaultsSchema,
		MainBuildEntry:                     mainBuildEntrySchema,
		UIVariant:                          uiVariantSchema,
		HTMLTemplateLocation:               htmlTemplateLocationSchema,
		ClientEntry:                        clientEntrySchema,
		ClientRouteDefinitionPatterns:      clientRouteDefinitionPatternsSchema,
		ServerRouteDefinitionPatterns:      serverRouteDefinitionPatternsSchema,
		TSGenOutDir:                        tsGenOutDirSchema,
		BuildtimePublicURLFuncName:         buildtimePublicURLFuncNameSchema,
		UnresolvedRoutePolicy:              unresolvedRoutePolicySchema,
		DevReloadRoutesEndpointPath:        devReloadRoutesEndpointPathSchema,
		DevReloadTemplateEndpointPath:      devReloadTemplateEndpointPathSchema,
		DevReloadPublicFileMapEndpointPath: devReloadPublicFileMapEndpointPathSchema,
		TemplateDataKeyHeadElements:        templateDataKeyHeadElementsSchema,
		TemplateDataKeyBodyScripts:         templateDataKeyBodyScriptsSchema,
		TemplateDataKeySSRScript:           templateDataKeySSRScriptSchema,
		TemplateDataKeySSRScriptHash:       templateDataKeySSRScriptHashSchema,
		TemplateDataKeyRootElementID:       templateDataKeyRootElementIDSchema,
		ClientRootElementID:                clientRootElementIDSchema,
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

var devReloadPublicFileMapEndpointPathSchema = jsonschema.OptionalString(
	jsonschema.Def{
		Description: `Dev-only endpoint path that rewrites generated TypeScript public filemap output from Wave canonical artifacts.`,
		Default:     "/__vorma_internal/reload-public-filemap",
		Examples:    []string{"/__vorma_internal/reload-public-filemap"},
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
