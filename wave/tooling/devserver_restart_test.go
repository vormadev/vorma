package tooling

import (
	"io"
	"log/slog"
	"testing"
)

func newServerForRestartChannelTest() *server {
	return &server{
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		restartCh: make(chan restartRequest, 1),
	}
}

func TestTriggerRestartWithOpts_UpgradeSemantics(t *testing.T) {
	t.Run("upgrades pending no-go restart to go-recompile restart", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerRestartNoGo()
		s.triggerRestart()

		req := <-s.restartCh
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

		req := <-s.restartCh
		if !req.isConfigRestart || !req.recompileGo {
			t.Fatalf("expected config restart with recompile, got %#v", req)
		}
	})

	t.Run("pending config restart is never downgraded", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerConfigRestart()
		s.triggerRestartNoGo()

		req := <-s.restartCh
		if !req.isConfigRestart || !req.recompileGo {
			t.Fatalf("expected config restart to remain pending, got %#v", req)
		}
	})

	t.Run("weaker request does not downgrade stronger pending request", func(t *testing.T) {
		s := newServerForRestartChannelTest()

		s.triggerRestart()
		s.triggerRestartNoGo()

		req := <-s.restartCh
		if !req.recompileGo {
			t.Fatalf("expected pending request to keep recompileGo=true, got %#v", req)
		}
	})
}

func TestMergeRestartRequests(t *testing.T) {
	tests := []struct {
		name             string
		pending          restartRequest
		incoming         restartRequest
		expectedCombined restartRequest
	}{
		{
			name: "config restart takes precedence over non-config restart",
			pending: restartRequest{
				recompileGo:     false,
				isConfigRestart: false,
			},
			incoming: restartRequest{
				recompileGo:     false,
				isConfigRestart: true,
			},
			expectedCombined: restartRequest{
				recompileGo:     true,
				isConfigRestart: true,
			},
		},
		{
			name: "pending config restart remains config restart",
			pending: restartRequest{
				recompileGo:     true,
				isConfigRestart: true,
			},
			incoming: restartRequest{
				recompileGo:     true,
				isConfigRestart: false,
			},
			expectedCombined: restartRequest{
				recompileGo:     true,
				isConfigRestart: true,
			},
		},
		{
			name: "non-config requests OR their recompile requirement",
			pending: restartRequest{
				recompileGo:     false,
				isConfigRestart: false,
			},
			incoming: restartRequest{
				recompileGo:     true,
				isConfigRestart: false,
			},
			expectedCombined: restartRequest{
				recompileGo:     true,
				isConfigRestart: false,
			},
		},
		{
			name: "weaker incoming request does not downgrade stronger pending request",
			pending: restartRequest{
				recompileGo:     true,
				isConfigRestart: false,
			},
			incoming: restartRequest{
				recompileGo:     false,
				isConfigRestart: false,
			},
			expectedCombined: restartRequest{
				recompileGo:     true,
				isConfigRestart: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			combined := mergeRestartRequests(tt.pending, tt.incoming)
			if combined != tt.expectedCombined {
				t.Fatalf(
					"mergeRestartRequests(%#v, %#v) = %#v, want %#v",
					tt.pending,
					tt.incoming,
					combined,
					tt.expectedCombined,
				)
			}
		})
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

		req := <-s.restartCh
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

		req := <-s.restartCh
		if !req.recompileGo {
			t.Fatalf("expected recompileGo=true, got %#v", req)
		}
		if req.isConfigRestart {
			t.Fatalf("expected isConfigRestart=false, got %#v", req)
		}
	})
}
