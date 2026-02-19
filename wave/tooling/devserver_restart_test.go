package tooling

import (
	"fmt"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
	"io"
	"log/slog"
	"testing"
)

func newServerForRestartChannelTest() *devserver.Server {
	return &devserver.Server{
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		RestartIntents: devserverengine.NewRestartIntentAccumulator(make(chan devserverengine.RestartRequest, 1)),
	}
}

func mustConsumePendingRestartRequestForRestartTests(
	t *testing.T,
	s *devserver.Server,
) devserverengine.RestartRequest {
	t.Helper()

	pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(s)
	if !hasPendingRestartRequest {
		t.Fatal("expected pending restart request")
	}
	return pendingRestartRequest
}

func TestTriggerRestartWithOpts_UpgradeSemantics(t *testing.T) {
	t.Run("waiting for build retry keeps first pending restart request", func(t *testing.T) {
		s := newServerForRestartChannelTest()
		s.SetWaitingForBuildRetry(true)

		s.TriggerRestartNoGo()
		s.TriggerRestart()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if req.RecompileGo || req.IsConfigRestart {
			t.Fatalf("expected first pending no-go restart to be preserved, got %#v", req)
		}
	})

	t.Run("upgrades pending no-go restart to go-recompile restart", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.TriggerRestartNoGo()
		s.TriggerRestart()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.RecompileGo {
			t.Fatalf("expected recompileGo=true, got %#v", req)
		}
		if req.IsConfigRestart {
			t.Fatalf("expected isConfigRestart=false, got %#v", req)
		}
	})

	t.Run("config restart supersedes non-config restart", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.TriggerRestartNoGo()
		s.TriggerConfigRestart()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.IsConfigRestart || !req.RecompileGo {
			t.Fatalf("expected config restart with recompile, got %#v", req)
		}
	})

	t.Run("pending config restart is never downgraded", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.TriggerConfigRestart()
		s.TriggerRestartNoGo()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.IsConfigRestart || !req.RecompileGo {
			t.Fatalf("expected config restart to remain pending, got %#v", req)
		}
	})

	t.Run("weaker request does not downgrade stronger pending request", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.TriggerRestart()
		s.TriggerRestartNoGo()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.RecompileGo {
			t.Fatalf("expected pending request to keep recompileGo=true, got %#v", req)
		}
	})
}

func TestNormalizeRestartRequest(t *testing.T) {
	tests := []struct {
		Name           string
		InputRequest   devserverengine.RestartRequest
		ExpectedResult devserverengine.RestartRequest
	}{
		{
			Name: "non-config request unchanged",
			InputRequest: devserverengine.RestartRequest{
				RecompileGo:     false,
				IsConfigRestart: false,
			},
			ExpectedResult: devserverengine.RestartRequest{
				RecompileGo:     false,
				IsConfigRestart: false,
			},
		},
		{
			Name: "config request always recompiles go",
			InputRequest: devserverengine.RestartRequest{
				RecompileGo:     false,
				IsConfigRestart: true,
			},
			ExpectedResult: devserverengine.RestartRequest{
				RecompileGo:     true,
				IsConfigRestart: true,
			},
		},
		{
			Name: "already-strong config request preserved",
			InputRequest: devserverengine.RestartRequest{
				RecompileGo:     true,
				IsConfigRestart: true,
			},
			ExpectedResult: devserverengine.RestartRequest{
				RecompileGo:     true,
				IsConfigRestart: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			normalizedRequest := devserverengine.NormalizeRestartRequest(tt.InputRequest)
			if normalizedRequest != tt.ExpectedResult {
				t.Fatalf(
					"devserverengine.NormalizeRestartRequest(%#v) = %#v, want %#v",
					tt.InputRequest,
					normalizedRequest,
					tt.ExpectedResult,
				)
			}
		})
	}
}

func TestResolveQueuedRestartRequest(t *testing.T) {
	t.Run("no pending request uses normalized incoming request", func(t *testing.T) {
		incomingRequest := devserverengine.RestartRequest{
			RecompileGo:     false,
			IsConfigRestart: true,
		}

		resolvedRequest := devserverengine.ResolveQueuedRestartRequest(nil, incomingRequest)
		if !resolvedRequest.RecompileGo || !resolvedRequest.IsConfigRestart {
			t.Fatalf("expected normalized config restart, got %#v", resolvedRequest)
		}
	})

	t.Run("pending request merges with incoming request", func(t *testing.T) {
		pendingRequest := devserverengine.RestartRequest{
			RecompileGo:     false,
			IsConfigRestart: false,
		}
		incomingRequest := devserverengine.RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: false,
		}

		resolvedRequest := devserverengine.ResolveQueuedRestartRequest(&pendingRequest, incomingRequest)
		if !resolvedRequest.RecompileGo || resolvedRequest.IsConfigRestart {
			t.Fatalf("expected go-recompile non-config restart, got %#v", resolvedRequest)
		}
	})
}

func TestMergeRestartRequests(t *testing.T) {
	allPossibleRequests := []devserverengine.RestartRequest{
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
				normalizedPendingRequest := devserverengine.NormalizeRestartRequest(pendingRequestForTest)
				normalizedIncomingRequest := devserverengine.NormalizeRestartRequest(incomingRequestForTest)
				combined := devserverengine.MergeRestartRequests(normalizedPendingRequest, normalizedIncomingRequest)
				expectedCombined := expectedMergedRestartRequest(
					normalizedPendingRequest,
					normalizedIncomingRequest,
				)

				if combined != expectedCombined {
					t.Fatalf(
						"devserverengine.MergeRestartRequests(%#v, %#v) = %#v, want %#v",
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
	pendingRequest devserverengine.RestartRequest,
	incomingRequest devserverengine.RestartRequest,
) devserverengine.RestartRequest {
	if pendingRequest.IsConfigRestart || incomingRequest.IsConfigRestart {
		return devserverengine.RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: true,
		}
	}

	return devserverengine.RestartRequest{
		RecompileGo:     pendingRequest.RecompileGo || incomingRequest.RecompileGo,
		IsConfigRestart: false,
	}
}

func TestTriggerRestartFromRefreshActions(t *testing.T) {
	t.Run("trigger restart without go compile", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.TriggerRestartFromRefreshActions(
			devserver.RefreshActionApplicationResult{
				RestartRequested: true,
				RecompileGo:      false,
			},
		)

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if req.RecompileGo {
			t.Fatalf("expected recompileGo=false, got %#v", req)
		}
		if req.IsConfigRestart {
			t.Fatalf("expected isConfigRestart=false, got %#v", req)
		}
	})

	t.Run("trigger restart with go compile", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.TriggerRestartFromRefreshActions(
			devserver.RefreshActionApplicationResult{
				RestartRequested: true,
				RecompileGo:      true,
			},
		)

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.RecompileGo {
			t.Fatalf("expected recompileGo=true, got %#v", req)
		}
		if req.IsConfigRestart {
			t.Fatalf("expected isConfigRestart=false, got %#v", req)
		}
	})
}

func TestRestartIntentAccumulator_ConsumePendingRestartRequestClearsPendingState(t *testing.T) {
	accumulator := devserverengine.NewRestartIntentAccumulator(make(chan devserverengine.RestartRequest, 1))
	accumulator.QueueRestartRequest(devserverengine.RestartRequest{
		RecompileGo: true,
	})

	consumedRequest, hasConsumedRequest := accumulator.ConsumePendingRestartRequest()
	if !hasConsumedRequest {
		t.Fatal("expected pending restart request to be consumed")
	}
	if !consumedRequest.RecompileGo || consumedRequest.IsConfigRestart {
		t.Fatalf("unexpected consumed restart request: %#v", consumedRequest)
	}

	_, hasSecondPendingRequest := accumulator.ConsumePendingRestartRequest()
	if hasSecondPendingRequest {
		t.Fatal("expected consumed restart request to be atomically cleared")
	}
}
