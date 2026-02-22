package restartengine_test

import (
	"fmt"
	"testing"

	"github.com/vormadev/vorma/wave/tooling/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/runloop"
)

type restartChannelHarness struct {
	waitingForBuildRetry bool
	restartIntents       *restartengine.RestartIntentAccumulator
}

func newRestartChannelHarnessForRestartTests() *restartChannelHarness {
	return &restartChannelHarness{
		restartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}
}

func (harness *restartChannelHarness) setWaitingForBuildRetry(waiting bool) {
	harness.waitingForBuildRetry = waiting
}

func (harness *restartChannelHarness) queueRestartRequest(
	request restartengine.RestartRequest,
) {
	if harness == nil || harness.restartIntents == nil {
		return
	}
	if harness.waitingForBuildRetry && harness.restartIntents.HasQueuedOrPendingRequest() {
		return
	}
	harness.restartIntents.Queue(request)
}

func (harness *restartChannelHarness) consumePendingRestartRequest() (
	restartengine.RestartRequest,
	bool,
) {
	if harness == nil || harness.restartIntents == nil {
		return restartengine.RestartRequest{}, false
	}
	return harness.restartIntents.ConsumePending()
}

func (harness *restartChannelHarness) triggerRestart() {
	harness.queueRestartRequest(
		restartengine.RestartRequest{
			RecompileGo: true,
		},
	)
}

func (harness *restartChannelHarness) triggerRestartNoGo() {
	harness.queueRestartRequest(
		restartengine.RestartRequest{},
	)
}

func (harness *restartChannelHarness) triggerConfigRestart() {
	harness.queueRestartRequest(
		restartengine.RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: true,
		},
	)
}

func mustConsumePendingRestartRequestForRestartTests(
	t *testing.T,
	harness *restartChannelHarness,
) restartengine.RestartRequest {
	t.Helper()

	pendingRequest, hasPendingRequest := harness.consumePendingRestartRequest()
	if !hasPendingRequest {
		t.Fatal("expected pending restart request")
	}
	return pendingRequest
}

func TestTriggerRestartWithOpts_UpgradeSemantics(t *testing.T) {
	t.Run("waiting for build retry keeps first pending restart request", func(t *testing.T) {
		harness := newRestartChannelHarnessForRestartTests()
		harness.setWaitingForBuildRetry(true)

		harness.triggerRestartNoGo()
		harness.triggerRestart()

		request := mustConsumePendingRestartRequestForRestartTests(
			t,
			harness,
		)
		if request.RecompileGo || request.IsConfigRestart {
			t.Fatalf(
				"expected first pending no-go restart to be preserved, got %#v",
				request,
			)
		}
	})

	t.Run("upgrades pending no-go restart to go-recompile restart", func(t *testing.T) {
		harness := newRestartChannelHarnessForRestartTests()

		harness.triggerRestartNoGo()
		harness.triggerRestart()

		request := mustConsumePendingRestartRequestForRestartTests(
			t,
			harness,
		)
		if !request.RecompileGo {
			t.Fatalf("expected recompileGo=true, got %#v", request)
		}
		if request.IsConfigRestart {
			t.Fatalf("expected isConfigRestart=false, got %#v", request)
		}
	})

	t.Run("config restart supersedes non-config restart", func(t *testing.T) {
		harness := newRestartChannelHarnessForRestartTests()

		harness.triggerRestartNoGo()
		harness.triggerConfigRestart()

		request := mustConsumePendingRestartRequestForRestartTests(
			t,
			harness,
		)
		if !request.IsConfigRestart || !request.RecompileGo {
			t.Fatalf("expected config restart with recompile, got %#v", request)
		}
	})

	t.Run("pending config restart is never downgraded", func(t *testing.T) {
		harness := newRestartChannelHarnessForRestartTests()

		harness.triggerConfigRestart()
		harness.triggerRestartNoGo()

		request := mustConsumePendingRestartRequestForRestartTests(
			t,
			harness,
		)
		if !request.IsConfigRestart || !request.RecompileGo {
			t.Fatalf("expected config restart to remain pending, got %#v", request)
		}
	})

	t.Run("weaker request does not downgrade stronger pending request", func(t *testing.T) {
		harness := newRestartChannelHarnessForRestartTests()

		harness.triggerRestart()
		harness.triggerRestartNoGo()

		request := mustConsumePendingRestartRequestForRestartTests(
			t,
			harness,
		)
		if !request.RecompileGo {
			t.Fatalf(
				"expected pending request to keep recompileGo=true, got %#v",
				request,
			)
		}
	})
}

func TestNormalizeRestartRequest(t *testing.T) {
	testCases := []struct {
		Name           string
		InputRequest   restartengine.RestartRequest
		ExpectedResult restartengine.RestartRequest
	}{
		{
			Name: "non-config request unchanged",
			InputRequest: restartengine.RestartRequest{
				RecompileGo:     false,
				IsConfigRestart: false,
			},
			ExpectedResult: restartengine.RestartRequest{
				RecompileGo:     false,
				IsConfigRestart: false,
			},
		},
		{
			Name: "config request always recompiles go",
			InputRequest: restartengine.RestartRequest{
				RecompileGo:     false,
				IsConfigRestart: true,
			},
			ExpectedResult: restartengine.RestartRequest{
				RecompileGo:     true,
				IsConfigRestart: true,
			},
		},
		{
			Name: "already-strong config request preserved",
			InputRequest: restartengine.RestartRequest{
				RecompileGo:     true,
				IsConfigRestart: true,
			},
			ExpectedResult: restartengine.RestartRequest{
				RecompileGo:     true,
				IsConfigRestart: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			normalizedRequest := restartengine.NormalizeRestartRequest(
				testCase.InputRequest,
			)
			if normalizedRequest != testCase.ExpectedResult {
				t.Fatalf(
					"restartengine.NormalizeRestartRequest(%#v) = %#v, want %#v",
					testCase.InputRequest,
					normalizedRequest,
					testCase.ExpectedResult,
				)
			}
		})
	}
}

func TestResolveQueuedRestartRequest(t *testing.T) {
	t.Run("no pending request uses normalized incoming request", func(t *testing.T) {
		incomingRequest := restartengine.RestartRequest{
			RecompileGo:     false,
			IsConfigRestart: true,
		}

		resolvedRequest := restartengine.ResolveQueuedRestartRequest(
			nil,
			incomingRequest,
		)
		if !resolvedRequest.RecompileGo || !resolvedRequest.IsConfigRestart {
			t.Fatalf("expected normalized config restart, got %#v", resolvedRequest)
		}
	})

	t.Run("pending request merges with incoming request", func(t *testing.T) {
		pendingRequest := restartengine.RestartRequest{
			RecompileGo:     false,
			IsConfigRestart: false,
		}
		incomingRequest := restartengine.RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: false,
		}

		resolvedRequest := restartengine.ResolveQueuedRestartRequest(
			&pendingRequest,
			incomingRequest,
		)
		if !resolvedRequest.RecompileGo || resolvedRequest.IsConfigRestart {
			t.Fatalf(
				"expected go-recompile non-config restart, got %#v",
				resolvedRequest,
			)
		}
	})
}

func TestMergeRestartRequests(t *testing.T) {
	allPossibleRequests := []restartengine.RestartRequest{
		{RecompileGo: false, IsConfigRestart: false},
		{RecompileGo: true, IsConfigRestart: false},
		{RecompileGo: false, IsConfigRestart: true},
		{RecompileGo: true, IsConfigRestart: true},
	}

	for _, pendingRequestForTest := range allPossibleRequests {
		for _, incomingRequestForTest := range allPossibleRequests {
			testName := fmt.Sprintf(
				"pending_go_%t_config_%t__incoming_go_%t_config_%t",
				pendingRequestForTest.RecompileGo,
				pendingRequestForTest.IsConfigRestart,
				incomingRequestForTest.RecompileGo,
				incomingRequestForTest.IsConfigRestart,
			)

			t.Run(testName, func(t *testing.T) {
				normalizedPendingRequest := restartengine.NormalizeRestartRequest(
					pendingRequestForTest,
				)
				normalizedIncomingRequest := restartengine.NormalizeRestartRequest(
					incomingRequestForTest,
				)
				combined := restartengine.MergeRestartRequests(
					normalizedPendingRequest,
					normalizedIncomingRequest,
				)
				expectedCombined := expectedMergedRestartRequest(
					normalizedPendingRequest,
					normalizedIncomingRequest,
				)

				if combined != expectedCombined {
					t.Fatalf(
						"restartengine.MergeRestartRequests(%#v, %#v) = %#v, want %#v",
						normalizedPendingRequest,
						normalizedIncomingRequest,
						combined,
						expectedCombined,
					)
				}
			})
		}
	}
}

func expectedMergedRestartRequest(
	pendingRequest restartengine.RestartRequest,
	incomingRequest restartengine.RestartRequest,
) restartengine.RestartRequest {
	if pendingRequest.IsConfigRestart || incomingRequest.IsConfigRestart {
		return restartengine.RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: true,
		}
	}

	return restartengine.RestartRequest{
		RecompileGo:     pendingRequest.RecompileGo || incomingRequest.RecompileGo,
		IsConfigRestart: false,
	}
}

func TestTriggerRestartFromRefreshActions(t *testing.T) {
	t.Run("trigger restart without go compile", func(t *testing.T) {
		triggerRestartCalls := 0
		triggerRestartNoGoCalls := 0
		engine := runloop.New(
			runloop.Dependencies{
				TriggerRestart: func() {
					triggerRestartCalls++
				},
				TriggerRestartNoGo: func() {
					triggerRestartNoGoCalls++
				},
			},
		)

		engine.TriggerRestartFromRefreshActions(
			eventpipeline.RefreshActionApplicationResult{
				RestartRequested: true,
				RecompileGo:      false,
			},
		)

		if triggerRestartCalls != 0 {
			t.Fatalf("expected triggerRestartCalls=0, got %d", triggerRestartCalls)
		}
		if triggerRestartNoGoCalls != 1 {
			t.Fatalf(
				"expected triggerRestartNoGoCalls=1, got %d",
				triggerRestartNoGoCalls,
			)
		}
	})

	t.Run("trigger restart with go compile", func(t *testing.T) {
		triggerRestartCalls := 0
		triggerRestartNoGoCalls := 0
		engine := runloop.New(
			runloop.Dependencies{
				TriggerRestart: func() {
					triggerRestartCalls++
				},
				TriggerRestartNoGo: func() {
					triggerRestartNoGoCalls++
				},
			},
		)

		engine.TriggerRestartFromRefreshActions(
			eventpipeline.RefreshActionApplicationResult{
				RestartRequested: true,
				RecompileGo:      true,
			},
		)

		if triggerRestartCalls != 1 {
			t.Fatalf("expected triggerRestartCalls=1, got %d", triggerRestartCalls)
		}
		if triggerRestartNoGoCalls != 0 {
			t.Fatalf(
				"expected triggerRestartNoGoCalls=0, got %d",
				triggerRestartNoGoCalls,
			)
		}
	})
}

func TestRestartIntentAccumulator_ConsumePendingRestartRequestClearsPendingState(
	t *testing.T,
) {
	accumulator := restartengine.NewRestartIntentAccumulator(
		make(chan restartengine.RestartRequest, 1),
	)
	accumulator.Queue(restartengine.RestartRequest{
		RecompileGo: true,
	})

	consumedRequest, hasConsumedRequest := accumulator.ConsumePending()
	if !hasConsumedRequest {
		t.Fatal("expected pending restart request to be consumed")
	}
	if !consumedRequest.RecompileGo || consumedRequest.IsConfigRestart {
		t.Fatalf("unexpected consumed restart request: %#v", consumedRequest)
	}

	_, hasSecondPendingRequest := accumulator.ConsumePending()
	if hasSecondPendingRequest {
		t.Fatal("expected consumed restart request to be atomically cleared")
	}
}
