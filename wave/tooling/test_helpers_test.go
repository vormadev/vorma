package tooling

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newParsedConfigForToolingTestsAtRoot(root string) *wave.ParsedConfig {
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: wave.StaticAssetDirs{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}
	return cfg
}

func queueRestartRequestForToolingTests(
	s *server,
	restartRequestForQueue restartRequest,
) {
	if s == nil {
		return
	}
	s.queueRestartRequest(restartRequestForQueue)
}

func consumePendingRestartRequestForToolingTests(
	s *server,
) (restartRequest, bool) {
	if s == nil {
		return restartRequest{}, false
	}
	return s.consumePendingRestartRequest()
}

func assertNoPendingRestartRequestForToolingTests(
	t *testing.T,
	s *server,
) {
	t.Helper()

	pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		s,
	)
	if hasPendingRestartRequest {
		t.Fatalf("expected no pending restart request, got %#v", pendingRestartRequest)
	}
}

func waitForPendingRestartRequestForToolingTests(
	t *testing.T,
	s *server,
	timeout time.Duration,
) restartRequest {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
			s,
		)
		if hasPendingRestartRequest {
			return pendingRestartRequest
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for pending restart request after %s", timeout)
	return restartRequest{}
}
