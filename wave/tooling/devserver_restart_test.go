package tooling

import (
	"fmt"
	"io"
	"log/slog"
	"testing"
)

func newServerForRestartChannelTest() *server {
	return &server{
		log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		restartIntents: newRestartIntentAccumulator(make(chan restartRequest, 1)),
	}
}

func mustConsumePendingRestartRequestForRestartTests(
	t *testing.T,
	s *server,
) restartRequest {
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
		s.setWaitingForBuildRetry(true)

		s.triggerRestartNoGo()
		s.triggerRestart()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if req.recompileGo || req.isConfigRestart {
			t.Fatalf("expected first pending no-go restart to be preserved, got %#v", req)
		}
	})

	t.Run("upgrades pending no-go restart to go-recompile restart", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerRestartNoGo()
		s.triggerRestart()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.recompileGo {
			t.Fatalf("expected recompileGo=true, got %#v", req)
		}
		if req.isConfigRestart {
			t.Fatalf("expected isConfigRestart=false, got %#v", req)
		}
	})

	t.Run("config restart supersedes non-config restart", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerRestartNoGo()
		s.triggerConfigRestart()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.isConfigRestart || !req.recompileGo {
			t.Fatalf("expected config restart with recompile, got %#v", req)
		}
	})

	t.Run("pending config restart is never downgraded", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerConfigRestart()
		s.triggerRestartNoGo()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.isConfigRestart || !req.recompileGo {
			t.Fatalf("expected config restart to remain pending, got %#v", req)
		}
	})

	t.Run("weaker request does not downgrade stronger pending request", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerRestart()
		s.triggerRestartNoGo()

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.recompileGo {
			t.Fatalf("expected pending request to keep recompileGo=true, got %#v", req)
		}
	})
}

func TestNormalizeRestartRequest(t *testing.T) {
	tests := []struct {
		name           string
		inputRequest   restartRequest
		expectedResult restartRequest
	}{
		{
			name: "non-config request unchanged",
			inputRequest: restartRequest{
				recompileGo:     false,
				isConfigRestart: false,
			},
			expectedResult: restartRequest{
				recompileGo:     false,
				isConfigRestart: false,
			},
		},
		{
			name: "config request always recompiles go",
			inputRequest: restartRequest{
				recompileGo:     false,
				isConfigRestart: true,
			},
			expectedResult: restartRequest{
				recompileGo:     true,
				isConfigRestart: true,
			},
		},
		{
			name: "already-strong config request preserved",
			inputRequest: restartRequest{
				recompileGo:     true,
				isConfigRestart: true,
			},
			expectedResult: restartRequest{
				recompileGo:     true,
				isConfigRestart: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedRequest := normalizeRestartRequest(tt.inputRequest)
			if normalizedRequest != tt.expectedResult {
				t.Fatalf(
					"normalizeRestartRequest(%#v) = %#v, want %#v",
					tt.inputRequest,
					normalizedRequest,
					tt.expectedResult,
				)
			}
		})
	}
}

func TestResolveQueuedRestartRequest(t *testing.T) {
	t.Run("no pending request uses normalized incoming request", func(t *testing.T) {
		incomingRequest := restartRequest{
			recompileGo:     false,
			isConfigRestart: true,
		}

		resolvedRequest := resolveQueuedRestartRequest(nil, incomingRequest)
		if !resolvedRequest.recompileGo || !resolvedRequest.isConfigRestart {
			t.Fatalf("expected normalized config restart, got %#v", resolvedRequest)
		}
	})

	t.Run("pending request merges with incoming request", func(t *testing.T) {
		pendingRequest := restartRequest{
			recompileGo:     false,
			isConfigRestart: false,
		}
		incomingRequest := restartRequest{
			recompileGo:     true,
			isConfigRestart: false,
		}

		resolvedRequest := resolveQueuedRestartRequest(&pendingRequest, incomingRequest)
		if !resolvedRequest.recompileGo || resolvedRequest.isConfigRestart {
			t.Fatalf("expected go-recompile non-config restart, got %#v", resolvedRequest)
		}
	})
}

func TestMergeRestartRequests(t *testing.T) {
	allPossibleRequests := []restartRequest{
		{recompileGo: false, isConfigRestart: false},
		{recompileGo: true, isConfigRestart: false},
		{recompileGo: false, isConfigRestart: true},
		{recompileGo: true, isConfigRestart: true},
	}

	for _, pendingRequestForTest := range allPossibleRequests {
		for _, incomingRequestForTest := range allPossibleRequests {
			testName := fmt.Sprintf(
				"pending_go_%t_config_%t__incoming_go_%t_config_%t",
				pendingRequestForTest.recompileGo,
				pendingRequestForTest.isConfigRestart,
				incomingRequestForTest.recompileGo,
				incomingRequestForTest.isConfigRestart,
			)

			t.Run(testName, func(t *testing.T) {
				normalizedPendingRequest := normalizeRestartRequest(pendingRequestForTest)
				normalizedIncomingRequest := normalizeRestartRequest(incomingRequestForTest)
				combined := mergeRestartRequests(normalizedPendingRequest, normalizedIncomingRequest)
				expectedCombined := expectedMergedRestartRequest(
					normalizedPendingRequest,
					normalizedIncomingRequest,
				)

				if combined != expectedCombined {
					t.Fatalf(
						"mergeRestartRequests(%#v, %#v) = %#v, want %#v",
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
	pendingRequest restartRequest,
	incomingRequest restartRequest,
) restartRequest {
	if pendingRequest.isConfigRestart || incomingRequest.isConfigRestart {
		return restartRequest{
			recompileGo:     true,
			isConfigRestart: true,
		}
	}

	return restartRequest{
		recompileGo:     pendingRequest.recompileGo || incomingRequest.recompileGo,
		isConfigRestart: false,
	}
}

func TestTriggerRestartFromRefreshActions(t *testing.T) {
	t.Run("trigger restart without go compile", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerRestartFromRefreshActions(
			refreshActionApplicationResult{
				restartRequested: true,
				recompileGo:      false,
			},
		)

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if req.recompileGo {
			t.Fatalf("expected recompileGo=false, got %#v", req)
		}
		if req.isConfigRestart {
			t.Fatalf("expected isConfigRestart=false, got %#v", req)
		}
	})

	t.Run("trigger restart with go compile", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerRestartFromRefreshActions(
			refreshActionApplicationResult{
				restartRequested: true,
				recompileGo:      true,
			},
		)

		req := mustConsumePendingRestartRequestForRestartTests(t, s)
		if !req.recompileGo {
			t.Fatalf("expected recompileGo=true, got %#v", req)
		}
		if req.isConfigRestart {
			t.Fatalf("expected isConfigRestart=false, got %#v", req)
		}
	})
}

func TestRestartIntentAccumulator_ConsumePendingRestartRequestClearsPendingState(t *testing.T) {
	accumulator := newRestartIntentAccumulator(make(chan restartRequest, 1))
	accumulator.queueRestartRequest(restartRequest{
		recompileGo: true,
	})

	consumedRequest, hasConsumedRequest := accumulator.consumePendingRestartRequest()
	if !hasConsumedRequest {
		t.Fatal("expected pending restart request to be consumed")
	}
	if !consumedRequest.recompileGo || consumedRequest.isConfigRestart {
		t.Fatalf("unexpected consumed restart request: %#v", consumedRequest)
	}

	_, hasSecondPendingRequest := accumulator.consumePendingRestartRequest()
	if hasSecondPendingRequest {
		t.Fatal("expected consumed restart request to be atomically cleared")
	}
}
