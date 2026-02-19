package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
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
	s *devserver.Server,
	restartRequestForQueue devserverengine.RestartRequest,
) {
	if s == nil {
		return
	}
	s.QueueRestartRequest(restartRequestForQueue)
}

func consumePendingRestartRequestForToolingTests(
	s *devserver.Server,
) (devserverengine.RestartRequest, bool) {
	if s == nil {
		return devserverengine.RestartRequest{}, false
	}
	return s.ConsumePendingRestartRequest()
}

func assertNoPendingRestartRequestForToolingTests(
	t *testing.T,
	s *devserver.Server,
) {
	t.Helper()

	pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		s,
	)
	if hasPendingRestartRequest {
		t.Fatalf(
			"expected no pending restart request, got %#v",
			pendingRestartRequest,
		)
	}
}

func waitForPendingRestartRequestForToolingTests(
	t *testing.T,
	s *devserver.Server,
	timeout time.Duration,
) devserverengine.RestartRequest {
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
	return devserverengine.RestartRequest{}
}

func mustConfigureAndGetWaveAppPortForToolingTests(t *testing.T) int {
	t.Helper()

	waveshared.ResetDefaultResolverForTest()
	t.Cleanup(waveshared.ResetDefaultResolverForTest)

	listener, listenError := net.Listen("tcp", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("failed to reserve app port for test: %v", listenError)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if closeError := listener.Close(); closeError != nil {
		t.Fatalf(
			"failed to release reserved app port %d for test: %v",
			port,
			closeError,
		)
	}

	t.Setenv("__WAVE_MODE", "production")
	t.Setenv("__WAVE_PORT_HAS_BEEN_SET", "true")
	t.Setenv("PORT", strconv.Itoa(port))

	return wave.MustGetPort()
}
