package tooling

import "testing"

func TestDeriveRunBuildExecutionOrderingDecision(t *testing.T) {
	testCases := []struct {
		name                                string
		shouldRecompileGo                   bool
		sequentialGoBuild                   bool
		expectedGoCompilationOrderingPolicy goCompilationOrderingPolicy
		expectedRunCompileInParallel        bool
		expectedRunCompileAfterBuildHooks   bool
	}{
		{
			name:                                "recompile disabled does not schedule compile",
			shouldRecompileGo:                   false,
			sequentialGoBuild:                   false,
			expectedGoCompilationOrderingPolicy: goCompilationOrderingPolicyNotRequested,
		},
		{
			name:                                "non-sequential mode compiles concurrently with build hooks",
			shouldRecompileGo:                   true,
			sequentialGoBuild:                   false,
			expectedGoCompilationOrderingPolicy: goCompilationOrderingPolicyConcurrentWithBuildHooks,
			expectedRunCompileInParallel:        true,
		},
		{
			name:                                "sequential mode compiles after build hooks",
			shouldRecompileGo:                   true,
			sequentialGoBuild:                   true,
			expectedGoCompilationOrderingPolicy: goCompilationOrderingPolicyAfterBuildHooks,
			expectedRunCompileAfterBuildHooks:   true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			orderingDecision := deriveRunBuildExecutionOrderingDecision(
				testCase.shouldRecompileGo,
				testCase.sequentialGoBuild,
			)

			if orderingDecision.goCompilationOrderingPolicy != testCase.expectedGoCompilationOrderingPolicy {
				t.Fatalf(
					"goCompilationOrderingPolicy=%q, want %q",
					orderingDecision.goCompilationOrderingPolicy,
					testCase.expectedGoCompilationOrderingPolicy,
				)
			}
			if orderingDecision.runCompileInParallel != testCase.expectedRunCompileInParallel {
				t.Fatalf(
					"runCompileInParallel=%t, want %t",
					orderingDecision.runCompileInParallel,
					testCase.expectedRunCompileInParallel,
				)
			}
			if orderingDecision.runCompileAfterBuildHooks != testCase.expectedRunCompileAfterBuildHooks {
				t.Fatalf(
					"runCompileAfterBuildHooks=%t, want %t",
					orderingDecision.runCompileAfterBuildHooks,
					testCase.expectedRunCompileAfterBuildHooks,
				)
			}
		})
	}
}
