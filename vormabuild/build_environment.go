package vormabuild

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave"
)

type frameworkBuildHookExecutionDependencies struct {
	prepareDiscoveredRouteRegistrarOverlayWithArtifactCache func(
		*vormaruntime.Vorma,
		*discoveredRouteRegistrarArtifactCache,
	) (*discoveredRouteRegistrarOverlay, error)
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
		prepareDiscoveredRouteRegistrarOverlayWithArtifactCache: prepareDiscoveredRouteRegistrarOverlayWithArtifactCache,
		runGoCommandWithContext: func(
			commandExecutionContext context.Context,
			goArguments []string,
		) error {
			if commandExecutionContext == nil {
				commandExecutionContext = context.Background()
			}

			goCommand := exec.CommandContext(commandExecutionContext, "go", goArguments...)
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
		dependencies: normalizeFrameworkBuildHookExecutionDependencies(dependencies),
	}
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
		cfg.FrameworkDevBuildHook = fmt.Sprintf("go run ./%s --dev --hook", v.Config.MainBuildEntry)
	}
	if cfg.FrameworkProdBuildHook == "" {
		cfg.FrameworkProdBuildHook = fmt.Sprintf("go run ./%s --hook", v.Config.MainBuildEntry)
	}
}

func configureBuildEnvironment(v *vormaruntime.Vorma) *wave.ParsedConfig {
	return configureBuildEnvironmentInConfig(v, v.Wave.GetBuildtimeParsedConfig())
}

func configureBuildEnvironmentInConfig(
	v *vormaruntime.Vorma,
	cfg *wave.ParsedConfig,
) *wave.ParsedConfig {
	return configureBuildEnvironmentInConfigWithFrameworkBuildHookExecutor(
		v,
		cfg,
		defaultFrameworkBuildHookExecutor,
	)
}

func configureBuildEnvironmentInConfigWithFrameworkBuildHookExecutor(
	v *vormaruntime.Vorma,
	cfg *wave.ParsedConfig,
	frameworkBuildHookExecutor frameworkBuildHookExecutor,
) *wave.ParsedConfig {
	if cfg == nil {
		return nil
	}

	discoveredRegistrarArtifactsCache := newDiscoveredRouteRegistrarArtifactCache(
		discoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
	)

	registerVormaSchemaInConfig(cfg)
	injectDefaultWatchPatternsInConfig(cfg, v)
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

func injectFrameworkBuildHookRunnerInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
	discoveredRegistrarArtifactsCache *discoveredRouteRegistrarArtifactCache,
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
			return errors.New("Vorma runtime is required")
		}
		if v.Config == nil {
			return errors.New("Vorma config is required")
		}
		mainBuildEntry := strings.TrimSpace(v.Config.MainBuildEntry)
		if mainBuildEntry == "" {
			return errors.New("Vorma config MainBuildEntry is required")
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
		if discoveredRouteRegistrarOverlay != nil && strings.TrimSpace(discoveredRouteRegistrarOverlay.goOverlayConfigPath) != "" {
			goRunArgs = append(goRunArgs, "-overlay="+discoveredRouteRegistrarOverlay.goOverlayConfigPath)
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
			cleanupOverlayErr = discoveredRouteRegistrarOverlay.cleanup()
		}
		if runHookCommandErr != nil {
			if cleanupOverlayErr != nil {
				return fmt.Errorf(
					"run framework build hook command: %w (cleanup discovered route registrar overlay failed: %v)",
					runHookCommandErr,
					cleanupOverlayErr,
				)
			}
			return fmt.Errorf("run framework build hook command: %w", runHookCommandErr)
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
	discoveredRegistrarArtifactsCache *discoveredRouteRegistrarArtifactCache,
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
			OverlayConfigPath: discoveredRouteRegistrarOverlay.goOverlayConfigPath,
			Cleanup: func() error {
				return discoveredRouteRegistrarOverlay.cleanup()
			},
		}, nil
	}
}
