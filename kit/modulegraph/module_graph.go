package modulegraph

import (
	"fmt"
	"sort"
	"strings"
)

type RegistrationPhase string

const (
	RegistrationPhaseRegisterLoaders RegistrationPhase = "register-loaders"
	RegistrationPhaseRegisterActions RegistrationPhase = "register-actions"
	RegistrationPhaseRegisterRoutes  RegistrationPhase = "register-routes"
	RegistrationPhaseFinalize        RegistrationPhase = "finalize"
)

type Module struct {
	ModuleID            string
	DependencyModuleIDs []string

	RegisterLoaders func() error
	RegisterActions func() error
	RegisterRoutes  func() error
	Finalize        func() error
}

func ResolveDeterministicModuleRegistrationOrder(modules []Module) ([]Module, error) {
	if len(modules) == 0 {
		return nil, nil
	}

	moduleByID := make(map[string]Module, len(modules))
	moduleInDegreeByID := make(map[string]int, len(modules))
	dependentModulesByDependencyID := make(map[string][]string, len(modules))

	for moduleIndex, module := range modules {
		if strings.TrimSpace(module.ModuleID) == "" {
			return nil, fmt.Errorf("module at index %d has empty module id", moduleIndex)
		}
		if module.ModuleID != strings.TrimSpace(module.ModuleID) {
			return nil, fmt.Errorf(
				"module %q cannot contain leading or trailing whitespace",
				module.ModuleID,
			)
		}
		if _, hasDuplicate := moduleByID[module.ModuleID]; hasDuplicate {
			return nil, fmt.Errorf("duplicate module id: %q", module.ModuleID)
		}

		moduleByID[module.ModuleID] = module
		moduleInDegreeByID[module.ModuleID] = 0
	}

	for _, module := range modules {
		for dependencyIndex, dependencyModuleID := range module.DependencyModuleIDs {
			if strings.TrimSpace(dependencyModuleID) == "" {
				return nil, fmt.Errorf(
					"module %q has empty dependency id at index %d",
					module.ModuleID,
					dependencyIndex,
				)
			}
			if dependencyModuleID != strings.TrimSpace(dependencyModuleID) {
				return nil, fmt.Errorf(
					"module %q dependency %q cannot contain leading or trailing whitespace",
					module.ModuleID,
					dependencyModuleID,
				)
			}
			if dependencyModuleID == module.ModuleID {
				return nil, fmt.Errorf("module %q cannot depend on itself", module.ModuleID)
			}
			if _, exists := moduleByID[dependencyModuleID]; !exists {
				return nil, fmt.Errorf(
					"module %q depends on missing module %q",
					module.ModuleID,
					dependencyModuleID,
				)
			}

			moduleInDegreeByID[module.ModuleID]++
			dependentModulesByDependencyID[dependencyModuleID] = append(
				dependentModulesByDependencyID[dependencyModuleID],
				module.ModuleID,
			)
		}
	}

	for dependencyModuleID := range dependentModulesByDependencyID {
		sort.Strings(dependentModulesByDependencyID[dependencyModuleID])
	}

	var moduleIDsReadyForScheduling []string
	for moduleID, inDegree := range moduleInDegreeByID {
		if inDegree == 0 {
			moduleIDsReadyForScheduling = append(moduleIDsReadyForScheduling, moduleID)
		}
	}
	sort.Strings(moduleIDsReadyForScheduling)

	orderedModules := make([]Module, 0, len(modules))
	for len(moduleIDsReadyForScheduling) > 0 {
		nextModuleID := moduleIDsReadyForScheduling[0]
		moduleIDsReadyForScheduling = moduleIDsReadyForScheduling[1:]

		orderedModules = append(orderedModules, moduleByID[nextModuleID])

		for _, dependentModuleID := range dependentModulesByDependencyID[nextModuleID] {
			moduleInDegreeByID[dependentModuleID]--
			if moduleInDegreeByID[dependentModuleID] == 0 {
				moduleIDsReadyForScheduling = append(
					moduleIDsReadyForScheduling,
					dependentModuleID,
				)
				sort.Strings(moduleIDsReadyForScheduling)
			}
		}
	}

	if len(orderedModules) != len(modules) {
		var moduleIDsInCycle []string
		for moduleID, inDegree := range moduleInDegreeByID {
			if inDegree > 0 {
				moduleIDsInCycle = append(moduleIDsInCycle, moduleID)
			}
		}
		sort.Strings(moduleIDsInCycle)
		return nil, fmt.Errorf(
			"module dependency cycle detected among modules: %s",
			strings.Join(moduleIDsInCycle, ", "),
		)
	}

	return orderedModules, nil
}

func RunDeterministicModuleRegistrationLifecycle(modules []Module) error {
	orderedModules, err := ResolveDeterministicModuleRegistrationOrder(modules)
	if err != nil {
		return err
	}

	registrationPhases := []struct {
		phaseName RegistrationPhase
		getHook   func(Module) func() error
	}{
		{
			phaseName: RegistrationPhaseRegisterLoaders,
			getHook: func(module Module) func() error {
				return module.RegisterLoaders
			},
		},
		{
			phaseName: RegistrationPhaseRegisterActions,
			getHook: func(module Module) func() error {
				return module.RegisterActions
			},
		},
		{
			phaseName: RegistrationPhaseRegisterRoutes,
			getHook: func(module Module) func() error {
				return module.RegisterRoutes
			},
		},
		{
			phaseName: RegistrationPhaseFinalize,
			getHook: func(module Module) func() error {
				return module.Finalize
			},
		},
	}

	for _, registrationPhase := range registrationPhases {
		for _, module := range orderedModules {
			modulePhaseHook := registrationPhase.getHook(module)
			if modulePhaseHook == nil {
				continue
			}
			if err := modulePhaseHook(); err != nil {
				return fmt.Errorf(
					"module %q phase %q failed: %w",
					module.ModuleID,
					registrationPhase.phaseName,
					err,
				)
			}
		}
	}

	return nil
}
