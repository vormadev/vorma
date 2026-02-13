package vormaruntime

import (
	"fmt"
	"sync"

	"github.com/vormadev/vorma/kit/modulegraph"
)

type AppModuleRegistrationHook func(app *Vorma) error

type AppModule struct {
	ModuleID            string
	DependencyModuleIDs []string

	RegisterLoaders AppModuleRegistrationHook
	RegisterActions AppModuleRegistrationHook
	RegisterRoutes  AppModuleRegistrationHook
	Finalize        AppModuleRegistrationHook
}

type AppRoot struct {
	app                    *Vorma
	modules                []AppModule
	moduleRegistrationOnce sync.Once
	moduleRegistrationErr  error
}

func NewAppRoot(app *Vorma, modules []AppModule) *AppRoot {
	if app == nil {
		panic("vormaruntime.NewAppRoot: app cannot be nil")
	}

	return &AppRoot{
		app:     app,
		modules: append([]AppModule(nil), modules...),
	}
}

func (appRoot *AppRoot) GetApp() *Vorma {
	appRoot.moduleRegistrationOnce.Do(func() {
		appRoot.moduleRegistrationErr = RunAppModuleRegistrationLifecycle(
			appRoot.app,
			appRoot.modules,
		)
	})
	if appRoot.moduleRegistrationErr != nil {
		panic(appRoot.moduleRegistrationErr)
	}
	return appRoot.app
}

func RunAppModuleRegistrationLifecycle(app *Vorma, modules []AppModule) error {
	if app == nil {
		return fmt.Errorf("app cannot be nil")
	}
	if len(modules) == 0 {
		return nil
	}

	moduleGraphModules := make([]modulegraph.Module, 0, len(modules))
	for _, configuredModule := range modules {
		module := configuredModule

		moduleGraphModules = append(moduleGraphModules, modulegraph.Module{
			ModuleID:            module.ModuleID,
			DependencyModuleIDs: append([]string(nil), module.DependencyModuleIDs...),
			RegisterLoaders:     bindAppModuleRegistrationHook(app, module.RegisterLoaders),
			RegisterActions:     bindAppModuleRegistrationHook(app, module.RegisterActions),
			RegisterRoutes:      bindAppModuleRegistrationHook(app, module.RegisterRoutes),
			Finalize:            bindAppModuleRegistrationHook(app, module.Finalize),
		})
	}

	if err := modulegraph.RunDeterministicModuleRegistrationLifecycle(moduleGraphModules); err != nil {
		return fmt.Errorf("app module registration failed: %w", err)
	}

	return nil
}

func bindAppModuleRegistrationHook(
	app *Vorma,
	hook AppModuleRegistrationHook,
) func() error {
	if hook == nil {
		return nil
	}
	return func() error {
		return hook(app)
	}
}
