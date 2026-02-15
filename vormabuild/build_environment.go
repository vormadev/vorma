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
	prepareDiscoveredRouteRegistrarOverlay func(*vormaruntime.Vorma) (*discoveredRouteRegistrarOverlay, error)
	runGoCommandWithContext                func(context.Context, []string) error
}

var frameworkBuildHookExecutionDeps = frameworkBuildHookExecutionDependencies{
	prepareDiscoveredRouteRegistrarOverlay: prepareDiscoveredRouteRegistrarOverlay,
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
	registerVormaSchemaInConfig(cfg)
	injectDefaultWatchPatternsInConfig(cfg, v)
	injectFrameworkBuildHooksInConfig(cfg, v)
	injectFrameworkBuildHookRunnerInConfig(cfg, v)
	injectFrameworkGoBuildOverlayPreparationInConfig(cfg, v)
	return cfg
}

func injectFrameworkBuildHookRunnerInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
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
		discoveredRouteRegistrarOverlay, err := frameworkBuildHookExecutionDeps.prepareDiscoveredRouteRegistrarOverlay(v)
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

		runHookCommandErr := frameworkBuildHookExecutionDeps.runGoCommandWithContext(
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
) {
	if cfg == nil {
		return
	}
	if cfg.FrameworkPrepareGoBuildOverlay != nil {
		return
	}

	cfg.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		discoveredRouteRegistrarOverlay, err := prepareDiscoveredRouteRegistrarOverlay(v)
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
