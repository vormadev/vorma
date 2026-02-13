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
