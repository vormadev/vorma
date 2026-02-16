package tooling

type goCompilationOrderingPolicy string

const (
	goCompilationOrderingPolicyConcurrentWithBuildHooks goCompilationOrderingPolicy = "compile_concurrently_with_build_hooks"
	goCompilationOrderingPolicyAfterBuildHooks          goCompilationOrderingPolicy = "compile_after_build_hooks"
	goCompilationOrderingPolicyNotRequested             goCompilationOrderingPolicy = "compile_not_requested"
)

type runBuildExecutionOrderingDecision struct {
	goCompilationOrderingPolicy goCompilationOrderingPolicy
	runCompileInParallel        bool
	runCompileAfterBuildHooks   bool
}

func deriveRunBuildExecutionOrderingDecision(
	shouldRecompileGo bool,
	sequentialGoBuild bool,
) runBuildExecutionOrderingDecision {
	if !shouldRecompileGo {
		return runBuildExecutionOrderingDecision{
			goCompilationOrderingPolicy: goCompilationOrderingPolicyNotRequested,
		}
	}

	if sequentialGoBuild {
		return runBuildExecutionOrderingDecision{
			goCompilationOrderingPolicy: goCompilationOrderingPolicyAfterBuildHooks,
			runCompileAfterBuildHooks:   true,
		}
	}

	return runBuildExecutionOrderingDecision{
		goCompilationOrderingPolicy: goCompilationOrderingPolicyConcurrentWithBuildHooks,
		runCompileInParallel:        true,
	}
}
