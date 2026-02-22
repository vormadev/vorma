package restartengine

import "testing"

func TestDeriveRunBuildExecutionOrderingDecision(t *testing.T) {
	testCases := []struct {
		Name                                string
		ShouldRecompileGo                   bool
		SequentialGoBuild                   bool
		ExpectedShouldCompileGo             bool
		ExpectedGoCompilationOrderingPolicy GoCompilationOrderingPolicy
	}{
		{
			Name:                                "recompile disabled does not schedule compile",
			ShouldRecompileGo:                   false,
			SequentialGoBuild:                   false,
			ExpectedShouldCompileGo:             false,
			ExpectedGoCompilationOrderingPolicy: GoCompilationOrderingPolicyNotRequested,
		},
		{
			Name:                                "non-sequential mode compiles concurrently with build hooks",
			ShouldRecompileGo:                   true,
			SequentialGoBuild:                   false,
			ExpectedShouldCompileGo:             true,
			ExpectedGoCompilationOrderingPolicy: GoCompilationOrderingPolicyConcurrentWithBuildHooks,
		},
		{
			Name:                                "sequential mode compiles after build hooks",
			ShouldRecompileGo:                   true,
			SequentialGoBuild:                   true,
			ExpectedShouldCompileGo:             true,
			ExpectedGoCompilationOrderingPolicy: GoCompilationOrderingPolicyAfterBuildHooks,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			orderingDecision := DeriveRunBuildExecutionOrderingDecision(
				testCase.ShouldRecompileGo,
				testCase.SequentialGoBuild,
			)

			if orderingDecision.OrderingPolicy != testCase.ExpectedGoCompilationOrderingPolicy {
				t.Fatalf(
					"OrderingPolicy=%q, want %q",
					orderingDecision.OrderingPolicy,
					testCase.ExpectedGoCompilationOrderingPolicy,
				)
			}
			if orderingDecision.ShouldCompileGo != testCase.ExpectedShouldCompileGo {
				t.Fatalf(
					"ShouldCompileGo=%t, want %t",
					orderingDecision.ShouldCompileGo,
					testCase.ExpectedShouldCompileGo,
				)
			}
		})
	}
}
