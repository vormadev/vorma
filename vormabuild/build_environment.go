package vormabuild

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/vormaruntime"
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

func registerVormaSchema(v *vormaruntime.Vorma) {
	cfg := v.Wave.GetParsedConfig()
	if cfg.FrameworkSchemaExtensions == nil {
		cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	cfg.FrameworkSchemaExtensions["Vorma"] = VormaSchema
}

func injectFrameworkBuildHooks(v *vormaruntime.Vorma) {
	cfg := v.Wave.GetParsedConfig()
	if cfg.FrameworkDevBuildHook == "" {
		cfg.FrameworkDevBuildHook = fmt.Sprintf("go run ./%s --dev --hook", v.Config.MainBuildEntry)
	}
	if cfg.FrameworkProdBuildHook == "" {
		cfg.FrameworkProdBuildHook = fmt.Sprintf("go run ./%s --hook", v.Config.MainBuildEntry)
	}
}

func configureBuildEnvironment(v *vormaruntime.Vorma) {
	registerVormaSchema(v)
	injectDefaultWatchPatterns(v)
	injectFrameworkBuildHooks(v)
	injectFrameworkBuildHookRunner(v)
	injectFrameworkGoBuildOverlayPreparation(v)
}

func injectFrameworkBuildHookRunner(v *vormaruntime.Vorma) {
	cfg := v.Wave.GetParsedConfig()
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

func injectFrameworkGoBuildOverlayPreparation(v *vormaruntime.Vorma) {
	cfg := v.Wave.GetParsedConfig()
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
