package tooling

import (
	"testing"

	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
)

func TestDeriveRunBuildExecutionOrderingDecision(t *testing.T) {
	testCases := []struct {
		Name                                string
		ShouldRecompileGo                   bool
		SequentialGoBuild                   bool
		ExpectedGoCompilationOrderingPolicy devserverengine.GoCompilationOrderingPolicy
		ExpectedRunCompileInParallel        bool
		ExpectedRunCompileAfterBuildHooks   bool
	}{
		{
			Name:                                "recompile disabled does not schedule compile",
			ShouldRecompileGo:                   false,
			SequentialGoBuild:                   false,
			ExpectedGoCompilationOrderingPolicy: devserverengine.GoCompilationOrderingPolicyNotRequested,
		},
		{
			Name:                                "non-sequential mode compiles concurrently with build hooks",
			ShouldRecompileGo:                   true,
			SequentialGoBuild:                   false,
			ExpectedGoCompilationOrderingPolicy: devserverengine.GoCompilationOrderingPolicyConcurrentWithBuildHooks,
			ExpectedRunCompileInParallel:        true,
		},
		{
			Name:                                "sequential mode compiles after build hooks",
			ShouldRecompileGo:                   true,
			SequentialGoBuild:                   true,
			ExpectedGoCompilationOrderingPolicy: devserverengine.GoCompilationOrderingPolicyAfterBuildHooks,
			ExpectedRunCompileAfterBuildHooks:   true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			orderingDecision := devserverengine.DeriveRunBuildExecutionOrderingDecision(
				testCase.ShouldRecompileGo,
				testCase.SequentialGoBuild,
			)

			if orderingDecision.GoCompilationOrderingPolicy != testCase.ExpectedGoCompilationOrderingPolicy {
				t.Fatalf(
					"devserverengine.GoCompilationOrderingPolicy=%q, want %q",
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
