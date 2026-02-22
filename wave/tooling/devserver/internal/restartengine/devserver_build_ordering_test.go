package restartengine

import "testing"

func TestDeriveRunBuildExecutionOrderingDecision(t *testing.T) {
	testCases := []struct {
		Name                                string
		ShouldRecompileGo                   bool
		SequentialGoBuild                   bool
		ExpectedGoCompilationOrderingPolicy GoCompilationOrderingPolicy
		ExpectedRunCompileInParallel        bool
		ExpectedRunCompileAfterBuildHooks   bool
	}{
		{
			Name:                                "recompile disabled does not schedule compile",
			ShouldRecompileGo:                   false,
			SequentialGoBuild:                   false,
			ExpectedGoCompilationOrderingPolicy: GoCompilationOrderingPolicyNotRequested,
		},
		{
			Name:                                "non-sequential mode compiles concurrently with build hooks",
			ShouldRecompileGo:                   true,
			SequentialGoBuild:                   false,
			ExpectedGoCompilationOrderingPolicy: GoCompilationOrderingPolicyConcurrentWithBuildHooks,
			ExpectedRunCompileInParallel:        true,
		},
		{
			Name:                                "sequential mode compiles after build hooks",
			ShouldRecompileGo:                   true,
			SequentialGoBuild:                   true,
			ExpectedGoCompilationOrderingPolicy: GoCompilationOrderingPolicyAfterBuildHooks,
			ExpectedRunCompileAfterBuildHooks:   true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			orderingDecision := DeriveRunBuildExecutionOrderingDecision(
				testCase.ShouldRecompileGo,
				testCase.SequentialGoBuild,
			)

			if orderingDecision.GoCompilationOrderingPolicy != testCase.ExpectedGoCompilationOrderingPolicy {
				t.Fatalf(
					"GoCompilationOrderingPolicy=%q, want %q",
					orderingDecision.GoCompilationOrderingPolicy,
					testCase.ExpectedGoCompilationOrderingPolicy,
				)
			}
			if orderingDecision.RunCompileInParallel != testCase.ExpectedRunCompileInParallel {
				t.Fatalf(
					"runCompileInParallel=%t, want %t",
					orderingDecision.RunCompileInParallel,
					testCase.ExpectedRunCompileInParallel,
				)
			}
			if orderingDecision.RunCompileAfterBuildHooks != testCase.ExpectedRunCompileAfterBuildHooks {
				t.Fatalf(
					"runCompileAfterBuildHooks=%t, want %t",
					orderingDecision.RunCompileAfterBuildHooks,
					testCase.ExpectedRunCompileAfterBuildHooks,
				)
			}
		})
	}
}
