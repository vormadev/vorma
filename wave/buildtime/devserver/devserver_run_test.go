package devserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/testpath"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/restartengine"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
	"github.com/vormadev/vorma/wave/internal/wavelock"
)

func TestRunDev_ReturnsValidationErrorForInvalidConfig(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.MainAppEntry = ""

	runError := RunDev(cfg, newDiscardLogger())
	if runError == nil {
		t.Fatal("expected RunDev to fail validation for missing MainAppEntry")
	}
	if !strings.Contains(runError.Error(), "config validation failed") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}
}

func TestRunDev_ReturnsErrorForNilConfig(t *testing.T) {
	runError := RunDev(nil, newDiscardLogger())
	if runError == nil {
		t.Fatal("expected RunDev to fail for nil config")
	}
	if !strings.Contains(runError.Error(), "config is nil") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}
}

func TestRunDev_ReturnsLockHeldErrorWhenProjectIsAlreadyLocked(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	lock := wavelock.NewDevLock(cfg.Dist.Static())
	if lockAcquireError := lock.Acquire(); lockAcquireError != nil {
		t.Fatalf("failed to acquire initial lock: %v", lockAcquireError)
	}
	defer func() {
		_ = lock.Release()
	}()

	runError := RunDev(cfg, newDiscardLogger())
	if runError == nil {
		t.Fatal("expected RunDev to fail when lock is already held")
	}
	if !errors.Is(runError, wavelock.ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got: %v", runError)
	}
}

func TestRunDev_WithNilLoggerReleasesLockWhenRunReturnsError(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root")

	runError := RunDev(cfg, nil)
	if runError == nil {
		t.Fatal("expected RunDev to fail when watch root does not exist")
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}

	lock := wavelock.NewDevLock(cfg.Dist.Static())
	if lockAcquireError := lock.Acquire(); lockAcquireError != nil {
		t.Fatalf(
			"expected lock to be released after RunDev error, acquire failed: %v",
			lockAcquireError,
		)
	}
	defer func() {
		_ = lock.Release()
	}()
}

func TestServerRun_ReturnsInitWatcherErrorWhenWatchRootMissing(t *testing.T) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.WatchRoot = filepath.Join(root, "does-not-exist")

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal("expected Run to fail when watch root does not exist")
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}
}

func TestServerRun_BuildFailureThenRetryThenInitWatcherFailure(t *testing.T) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/run"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			serverForTest.Mu.Lock()
			watcherReady := serverForTest.Watcher != nil
			serverForTest.Mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-after-retry"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		serverForTest.QueueRestartRequest(
			restartengine.RestartRequest{RecompileGo: false},
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after retry cycle",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for Run helper goroutine")
	}
}

func TestServerRun_SequentialCompileFailureThenRetryThenInitWatcherFailure(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.SequentialGoBuild = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/sequential"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			serverForTest.Mu.Lock()
			watcherReady := serverForTest.Watcher != nil
			serverForTest.Mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-after-sequential-retry"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		serverForTest.QueueRestartRequest(
			restartengine.RestartRequest{RecompileGo: false},
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after sequential compile retry",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for Run helper goroutine")
	}
}

func TestServerRun_BuildFailureThenConfigFixRecoversAutomatically(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/recovery"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForMainEntryAndWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Core.MainAppEntry,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			3*time.Second,
		)
		if firstWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-initial-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			true,
			3*time.Second,
		) {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-retry-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		if writeError := writeToolingConfigForMainEntryAndWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			"../../../internal/cmd/sum",
			cfg.Watch.WatchRoot,
		); writeError != nil {
			t.Error(writeError)
			return
		}

		secondWatcher := waitForWatcherPointer(
			serverForTest,
			firstWatcher,
			4*time.Second,
		)
		if secondWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-recovery-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		time.Sleep(200 * time.Millisecond)

		if writeError := writeToolingConfigForMainEntryAndWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			"../../../internal/cmd/sum",
			filepath.Join(root, "missing-watch-root-after-recovery"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after recovery validation path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for recovery helper goroutine")
	}

	runLogOutput := runLogBuffer.String()
	if !strings.Contains(runLogOutput, "build failed") {
		t.Fatalf(
			"expected initial build failure log, got logs: %s",
			runLogOutput,
		)
	}
	if !strings.Contains(runLogOutput, "DONE building Wave") {
		t.Fatalf(
			"expected recovered build success log after config fix, got logs: %s",
			runLogOutput,
		)
	}
	if strings.Contains(
		runLogOutput,
		"app did not become ready before timeout",
	) {
		t.Fatalf(
			"expected config-fix recovery path not to log app readiness timeout, got logs: %s",
			runLogOutput,
		)
	}
}

func TestServerRun_GoSyntaxErrorThenQuickFixRecoversWithoutReadinessStall(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")
	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating go main parent dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Public, 0o755); mkdirError != nil {
		t.Fatalf("failed creating public static dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Private, 0o755); mkdirError != nil {
		t.Fatalf("failed creating private static dir: %v", mkdirError)
	}
	if writeError := writeGoMainFileForDevserverRunTests(
		goMainPath,
		`package main

import "time"

func main() {
	for {
		time.Sleep(10 * time.Second)
	}
}
`,
	); writeError != nil {
		t.Fatalf("failed writing initial go main file: %v", writeError)
	}
	cfg.Core.MainAppEntry = goMainPath

	if writeError := writeToolingConfigForMainEntryAndWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Core.MainAppEntry,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	recoveryDurationCh := make(chan time.Duration, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)

		failAndStopRun := func(failureError error) {
			if failureError != nil {
				t.Error(failureError)
			}
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-go-syntax-recovery-timeout"),
			); writeError != nil {
				t.Error(writeError)
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
		}

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			5*time.Second,
		)
		if firstWatcher == nil {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for initial watcher before go syntax recovery sequence",
				),
			)
			return
		}

		if writeError := writeGoMainFileForDevserverRunTests(
			goMainPath,
			`package main

func main() {
	this is not valid go syntax
}
`,
		); writeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed writing broken go source file: %w",
					writeError,
				),
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			true,
			5*time.Second,
		) {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for build-retry state after go syntax error",
				),
			)
			return
		}

		fixWriteStart := time.Now()
		if writeError := writeGoMainFileForDevserverRunTests(
			goMainPath,
			`package main

import "time"

func main() {
	for {
		time.Sleep(10 * time.Second)
	}
}
`,
		); writeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed writing fixed go source file: %w",
					writeError,
				),
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			false,
			8*time.Second,
		) {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for retry recovery after go syntax fix",
				),
			)
			return
		}
		recoveryDuration := time.Since(fixWriteStart)
		if recoveryDuration > 8*time.Second {
			failAndStopRun(
				fmt.Errorf(
					"go syntax fix recovery exceeded responsiveness budget: %s",
					recoveryDuration,
				),
			)
			return
		}
		recoveryDurationCh <- recoveryDuration

		time.Sleep(200 * time.Millisecond)

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-go-syntax-recovery"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after go syntax recovery validation path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for go syntax recovery helper goroutine")
	}

	select {
	case recoveryDuration := <-recoveryDurationCh:
		if recoveryDuration > 8*time.Second {
			t.Fatalf(
				"expected go syntax fix recovery under 8s, got %s",
				recoveryDuration,
			)
		}
	default:
		t.Fatal(
			"expected go syntax recovery helper to report measured recovery duration",
		)
	}

	runLogOutput := runLogBuffer.String()
	if !strings.Contains(runLogOutput, "build failed") {
		t.Fatalf(
			"expected go syntax build failure log, got logs: %s",
			runLogOutput,
		)
	}
	if !strings.Contains(runLogOutput, "DONE building Wave") {
		t.Fatalf(
			"expected go syntax fix to produce build success log, got logs: %s",
			runLogOutput,
		)
	}
	if strings.Contains(
		runLogOutput,
		"reload readiness failed; skipping browser broadcast",
	) {
		t.Fatalf(
			"expected go syntax recovery path not to block on reload readiness timeout, got logs: %s",
			runLogOutput,
		)
	}
}

func TestServerRun_GoTypeErrorThenQuickFixRecoversWithoutReadinessStall(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")
	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating go main parent dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Public, 0o755); mkdirError != nil {
		t.Fatalf("failed creating public static dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Private, 0o755); mkdirError != nil {
		t.Fatalf("failed creating private static dir: %v", mkdirError)
	}
	if writeError := writeGoMainFileForDevserverRunTests(
		goMainPath,
		`package main

import "time"

func main() {
	for {
		time.Sleep(10 * time.Second)
	}
}
`,
	); writeError != nil {
		t.Fatalf("failed writing initial go main file: %v", writeError)
	}
	cfg.Core.MainAppEntry = goMainPath

	if writeError := writeToolingConfigForMainEntryAndWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Core.MainAppEntry,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	recoveryDurationCh := make(chan time.Duration, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)

		failAndStopRun := func(failureError error) {
			if failureError != nil {
				t.Error(failureError)
			}
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-go-type-recovery-timeout"),
			); writeError != nil {
				t.Error(writeError)
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
		}

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			5*time.Second,
		)
		if firstWatcher == nil {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for initial watcher before go type-error recovery sequence",
				),
			)
			return
		}

		if writeError := writeGoMainFileForDevserverRunTests(
			goMainPath,
			`package main

func main() {
	_ = doesNotExist
}
`,
		); writeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed writing broken go source file: %w",
					writeError,
				),
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			true,
			5*time.Second,
		) {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for build-retry state after go type error",
				),
			)
			return
		}

		fixWriteStart := time.Now()
		if writeError := writeGoMainFileForDevserverRunTests(
			goMainPath,
			`package main

import "time"

func main() {
	for {
		time.Sleep(10 * time.Second)
	}
}
`,
		); writeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed writing fixed go source file: %w",
					writeError,
				),
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			false,
			8*time.Second,
		) {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for retry recovery after go type-error fix",
				),
			)
			return
		}
		recoveryDuration := time.Since(fixWriteStart)
		if recoveryDuration > 8*time.Second {
			failAndStopRun(
				fmt.Errorf(
					"go type-error fix recovery exceeded responsiveness budget: %s",
					recoveryDuration,
				),
			)
			return
		}
		recoveryDurationCh <- recoveryDuration

		time.Sleep(200 * time.Millisecond)

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-go-type-recovery"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after go type-error recovery validation path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for go type-error recovery helper goroutine")
	}

	select {
	case recoveryDuration := <-recoveryDurationCh:
		if recoveryDuration > 8*time.Second {
			t.Fatalf(
				"expected go type-error fix recovery under 8s, got %s",
				recoveryDuration,
			)
		}
	default:
		t.Fatal(
			"expected go type-error recovery helper to report measured recovery duration",
		)
	}

	runLogOutput := runLogBuffer.String()
	if !strings.Contains(runLogOutput, "build failed") {
		t.Fatalf(
			"expected go type-error build failure log, got logs: %s",
			runLogOutput,
		)
	}
	if !strings.Contains(runLogOutput, "DONE building Wave") {
		t.Fatalf(
			"expected go type-error fix to produce build success log, got logs: %s",
			runLogOutput,
		)
	}
	if strings.Contains(
		runLogOutput,
		"reload readiness failed; skipping browser broadcast",
	) {
		t.Fatalf(
			"expected go type-error recovery path not to block on reload readiness timeout, got logs: %s",
			runLogOutput,
		)
	}
}

func TestServerRun_MainAppEntryTypoThenQuickFixRecoversWithoutReadinessStall(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")
	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating go main parent dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Public, 0o755); mkdirError != nil {
		t.Fatalf("failed creating public static dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Private, 0o755); mkdirError != nil {
		t.Fatalf("failed creating private static dir: %v", mkdirError)
	}
	if writeError := writeGoMainFileForDevserverRunTests(
		goMainPath,
		`package main

import "time"

func main() {
	for {
		time.Sleep(10 * time.Second)
	}
}
`,
	); writeError != nil {
		t.Fatalf("failed writing initial go main file: %v", writeError)
	}
	cfg.Core.MainAppEntry = goMainPath

	if writeError := writeToolingConfigForMainEntryAndWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Core.MainAppEntry,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	recoveryDurationCh := make(chan time.Duration, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)

		failAndStopRun := func(testError error) {
			if testError != nil {
				t.Error(testError)
			}
			_ = writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(
					root,
					"missing-watch-root-main-entry-recovery-timeout",
				),
			)
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				2*time.Second,
			)
		}

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			5*time.Second,
		)
		if firstWatcher == nil {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for initial watcher before main entry recovery sequence",
				),
			)
			return
		}

		if writeError := writeToolingConfigForMainEntryAndWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			goMainPath+"2",
			cfg.Watch.WatchRoot,
		); writeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed writing broken main entry config: %w",
					writeError,
				),
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			true,
			5*time.Second,
		) {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for build-retry state after main entry typo",
				),
			)
			return
		}

		fixWriteStart := time.Now()
		if writeError := writeToolingConfigForMainEntryAndWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			goMainPath,
			cfg.Watch.WatchRoot,
		); writeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed writing fixed main entry config: %w",
					writeError,
				),
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			false,
			8*time.Second,
		) {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for retry recovery after main entry fix",
				),
			)
			return
		}
		recoveryDuration := time.Since(fixWriteStart)
		if recoveryDuration > 8*time.Second {
			failAndStopRun(
				fmt.Errorf(
					"main entry fix recovery exceeded responsiveness budget: %s",
					recoveryDuration,
				),
			)
			return
		}
		recoveryDurationCh <- recoveryDuration

		time.Sleep(200 * time.Millisecond)

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-main-entry-recovery"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after main entry recovery validation path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for main entry recovery helper goroutine")
	}

	select {
	case recoveryDuration := <-recoveryDurationCh:
		if recoveryDuration > 8*time.Second {
			t.Fatalf(
				"expected main entry fix recovery under 8s, got %s",
				recoveryDuration,
			)
		}
	default:
		t.Fatal(
			"expected main entry recovery helper to report measured recovery duration",
		)
	}

	runLogOutput := runLogBuffer.String()
	if !strings.Contains(runLogOutput, "build failed") {
		t.Fatalf(
			"expected main entry typo build failure log, got logs: %s",
			runLogOutput,
		)
	}
	if !strings.Contains(runLogOutput, "DONE building Wave") {
		t.Fatalf(
			"expected main entry fix to produce build success log, got logs: %s",
			runLogOutput,
		)
	}
	if strings.Contains(
		runLogOutput,
		"reload readiness failed; skipping browser broadcast",
	) {
		t.Fatalf(
			"expected main entry recovery path not to block on reload readiness timeout, got logs: %s",
			runLogOutput,
		)
	}
}

func TestServerRun_MainAppEntryTypo_NonConfigEventThenQuickFixRecoversWithoutReadinessStall(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")
	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating go main parent dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Public, 0o755); mkdirError != nil {
		t.Fatalf("failed creating public static dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Private, 0o755); mkdirError != nil {
		t.Fatalf("failed creating private static dir: %v", mkdirError)
	}
	privateStaticPath := filepath.Join(
		cfg.Core.StaticAssetDirs.Private,
		"retry_wait_delete.txt",
	)
	if writeError := os.WriteFile(
		privateStaticPath,
		[]byte("static file to delete during retry wait"),
		0o644,
	); writeError != nil {
		t.Fatalf("failed writing private static file: %v", writeError)
	}
	if writeError := writeGoMainFileForDevserverRunTests(
		goMainPath,
		`package main

import "time"

func main() {
	for {
		time.Sleep(10 * time.Second)
	}
}
`,
	); writeError != nil {
		t.Fatalf("failed writing initial go main file: %v", writeError)
	}
	cfg.Core.MainAppEntry = goMainPath

	if writeError := writeToolingConfigForMainEntryAndWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Core.MainAppEntry,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	recoveryDurationCh := make(chan time.Duration, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)

		failAndStopRun := func(testError error) {
			if testError != nil {
				t.Error(testError)
			}
			_ = writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(
					root,
					"missing-watch-root-main-entry-non-config-recovery-timeout",
				),
			)
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				2*time.Second,
			)
		}

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			5*time.Second,
		)
		if firstWatcher == nil {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for initial watcher before non-config recovery sequence",
				),
			)
			return
		}

		if writeError := writeToolingConfigForMainEntryAndWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			goMainPath+"2",
			cfg.Watch.WatchRoot,
		); writeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed writing broken main entry config: %w",
					writeError,
				),
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			true,
			5*time.Second,
		) {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for build-retry state after main entry typo",
				),
			)
			return
		}

		if removeError := os.Remove(privateStaticPath); removeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed removing private static file during retry wait: %w",
					removeError,
				),
			)
			return
		}
		time.Sleep(150 * time.Millisecond)

		fixWriteStart := time.Now()
		if writeError := writeToolingConfigForMainEntryAndWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			goMainPath,
			cfg.Watch.WatchRoot,
		); writeError != nil {
			failAndStopRun(
				fmt.Errorf(
					"failed writing fixed main entry config: %w",
					writeError,
				),
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			false,
			8*time.Second,
		) {
			failAndStopRun(
				fmt.Errorf(
					"timed out waiting for retry recovery after main entry fix",
				),
			)
			return
		}
		recoveryDuration := time.Since(fixWriteStart)
		if recoveryDuration > 8*time.Second {
			failAndStopRun(
				fmt.Errorf(
					"main entry fix recovery after non-config event exceeded responsiveness budget: %s",
					recoveryDuration,
				),
			)
			return
		}
		recoveryDurationCh <- recoveryDuration

		time.Sleep(200 * time.Millisecond)

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-main-entry-non-config-recovery"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after non-config recovery validation path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for non-config recovery helper goroutine")
	}

	select {
	case recoveryDuration := <-recoveryDurationCh:
		if recoveryDuration > 8*time.Second {
			t.Fatalf(
				"expected main entry fix recovery after non-config event under 8s, got %s",
				recoveryDuration,
			)
		}
	default:
		t.Fatal(
			"expected non-config recovery helper to report measured recovery duration",
		)
	}

	runLogOutput := runLogBuffer.String()
	if !strings.Contains(runLogOutput, "build failed") {
		t.Fatalf(
			"expected main entry typo build failure log, got logs: %s",
			runLogOutput,
		)
	}
	if !strings.Contains(runLogOutput, "DONE building Wave") {
		t.Fatalf(
			"expected main entry fix to produce build success log, got logs: %s",
			runLogOutput,
		)
	}
	if strings.Contains(
		runLogOutput,
		"reload readiness failed; skipping browser broadcast",
	) {
		t.Fatalf(
			"expected non-config recovery path not to block on reload readiness timeout, got logs: %s",
			runLogOutput,
		)
	}
}

func TestServerRun_MainAppEntryTypoThenConfigErrorEventTerminatesRun(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	testCases := []struct {
		name             string
		writeConfigError func(*testing.T, *waveconfig.ParsedConfig)
	}{
		{
			name: "syntax_error",
			writeConfigError: func(t *testing.T, cfg *waveconfig.ParsedConfig) {
				t.Helper()
				if writeError := os.WriteFile(
					cfg.Core.ConfigLocation,
					[]byte("{ invalid config payload"),
					0o644,
				); writeError != nil {
					t.Fatalf(
						"failed writing syntax-error config payload: %v",
						writeError,
					)
				}
			},
		},
		{
			name: "validation_error",
			writeConfigError: func(t *testing.T, cfg *waveconfig.ParsedConfig) {
				t.Helper()
				if writeError := writeToolingConfigForMainEntryAndWatchRoot(
					cfg.Core.ConfigLocation,
					cfg,
					"",
					cfg.Watch.WatchRoot,
				); writeError != nil {
					t.Fatalf(
						"failed writing validation-error config payload: %v",
						writeError,
					)
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := newParsedConfigForToolingTestsAtRoot(root)
			cfg.Core.ServerOnlyMode = false
			cfg.Watch.HealthcheckEndpoint = "/healthz"
			cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")
			goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
			if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
				t.Fatalf("failed creating go main parent dir: %v", mkdirError)
			}
			if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Public, 0o755); mkdirError != nil {
				t.Fatalf("failed creating public static dir: %v", mkdirError)
			}
			if mkdirError := os.MkdirAll(cfg.Core.StaticAssetDirs.Private, 0o755); mkdirError != nil {
				t.Fatalf("failed creating private static dir: %v", mkdirError)
			}
			if writeError := writeGoMainFileForDevserverRunTests(
				goMainPath,
				`package main

import "time"

func main() {
	for {
		time.Sleep(10 * time.Second)
	}
}
`,
			); writeError != nil {
				t.Fatalf("failed writing initial go main file: %v", writeError)
			}
			cfg.Core.MainAppEntry = goMainPath

			if writeError := writeToolingConfigForMainEntryAndWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				cfg.Core.MainAppEntry,
				cfg.Watch.WatchRoot,
			); writeError != nil {
				t.Fatalf(
					"failed writing initial tooling config: %v",
					writeError,
				)
			}

			var runLogBuffer bytes.Buffer
			serverForTest := &Server{
				Cfg: cfg,
				Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
				RestartIntents: restartengine.NewRestartIntentAccumulator(
					make(chan restartengine.RestartRequest, 1),
				),
			}

			runErrCh := make(chan error, 1)
			go func() {
				runErrCh <- serverForTest.Run()
			}()

			firstWatcher := waitForWatcherPointer(
				serverForTest,
				nil,
				5*time.Second,
			)
			if firstWatcher == nil {
				t.Fatal("timed out waiting for initial watcher before config-error run sequence")
			}

			if writeError := writeToolingConfigForMainEntryAndWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				goMainPath+"2",
				cfg.Watch.WatchRoot,
			); writeError != nil {
				t.Fatalf("failed writing broken main entry config: %v", writeError)
			}

			if !waitForWaitingForBuildRetryFlag(
				serverForTest,
				true,
				5*time.Second,
			) {
				t.Fatal("timed out waiting for build-retry state after main entry typo")
			}

			testCase.writeConfigError(t, cfg)

			select {
			case runError := <-runErrCh:
				if runError == nil {
					t.Fatal("expected Run to exit on config reload failure")
				}
				if !strings.Contains(
					runError.Error(),
					"reload config during cycle prepare",
				) {
					t.Fatalf("unexpected Run error: %v", runError)
				}
			case <-time.After(8 * time.Second):
				t.Fatal("timed out waiting for run failure after config error event")
			}

			runLogOutput := runLogBuffer.String()
			if !strings.Contains(runLogOutput, "build failed") {
				t.Fatalf(
					"expected build failure log before config error event, got logs: %s",
					runLogOutput,
				)
			}
			if !strings.Contains(runLogOutput, "config reload failed") {
				t.Fatalf(
					"expected config reload failure log for %s case, got logs: %s",
					testCase.name,
					runLogOutput,
				)
			}
		})
	}
}

func TestServerRun_NoOpConfigWriteFirstSaveLogsNoopWithoutWatcherRestart(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "../../../internal/cmd/sum"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}
	configFromDisk, parseError := waveconfig.ParseConfigFile(cfg.Core.ConfigLocation)
	if parseError != nil {
		t.Fatalf("parse initial tooling config from disk: %v", parseError)
	}
	waveframework.CopyRuntimeStateForToolingReload(configFromDisk, cfg)
	cfg = configFromDisk

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	var observedUnexpectedWatcherRestart atomic.Bool
	var observedNoopConfigWrite atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			3*time.Second,
		)
		if firstWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-noop-initial-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		unchangedConfigBytes, readError := os.ReadFile(cfg.Core.ConfigLocation)
		if readError != nil {
			t.Error(readError)
			return
		}
		if writeError := os.WriteFile(
			cfg.Core.ConfigLocation,
			unchangedConfigBytes,
			0o644,
		); writeError != nil {
			t.Error(writeError)
			return
		}
		observedNoopConfigWrite.Store(true)

		if restartedWatcher := waitForWatcherPointer(
			serverForTest,
			firstWatcher,
			700*time.Millisecond,
		); restartedWatcher != nil {
			observedUnexpectedWatcherRestart.Store(true)
		}

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-noop"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after no-op validation path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for no-op validation helper goroutine")
	}

	if !observedNoopConfigWrite.Load() {
		t.Fatal("expected helper to execute no-op config write")
	}
	if observedUnexpectedWatcherRestart.Load() {
		t.Fatal("expected no-op config first save not to restart watcher")
	}

	runLogOutput := runLogBuffer.String()
	if !strings.Contains(
		runLogOutput,
		"no changes to wave.config.json; skipping restart",
	) {
		t.Fatalf(
			"expected no-op config save to log explicit no-op message, got logs: %s",
			runLogOutput,
		)
	}
}

func TestServerRun_ViteStartFailureExitsRunLifecycleImmediately(t *testing.T) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "../../../internal/cmd/sum"
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "command_that_does_not_exist_for_wave_run_vite_test"
	cfg.Vite.DefaultPort = 5199
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal("expected Run to fail when Vite startup fails")
	}
	if !strings.Contains(runError.Error(), "start vite:") {
		t.Fatalf("unexpected Run error: %v", runError)
	}
	if _, statError := os.Stat(cfg.Dist.Binary()); statError != nil {
		t.Fatalf(
			"expected first pass to compile binary before Vite start attempt, stat error: %v",
			statError,
		)
	}
}

func TestServerRun_ConfigRestartReloadsConfigWithoutWaitingForStaleAppReadiness(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.MainAppEntry = "../../../internal/cmd/sum"
	cfg.Watch.HealthcheckEndpoint = "/healthz"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	appPort := mustConfigureAndGetWaveAppPortForDevserverRunTests(t)
	appListener, listenError := net.Listen(
		"tcp",
		fmt.Sprintf("127.0.0.1:%d", appPort),
	)
	if listenError != nil {
		t.Skipf(
			"unable to bind app port %d for config-restart test: %v",
			appPort,
			listenError,
		)
	}
	defer appListener.Close()

	var appHealthHits atomic.Int32
	appServer := &http.Server{
		Handler: http.HandlerFunc(func(
			responseWriter http.ResponseWriter,
			request *http.Request,
		) {
			if request.URL.Path == "/healthz" {
				appHealthHits.Add(1)
				responseWriter.WriteHeader(http.StatusOK)
				return
			}
			responseWriter.WriteHeader(http.StatusNotFound)
		}),
	}
	defer appServer.Close()
	go func() {
		_ = appServer.Serve(appListener)
	}()

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			3*time.Second,
		)
		if firstWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-fallback"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		if !sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{
				RecompileGo:     false,
				IsConfigRestart: true,
			},
			2*time.Second,
		) {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-send-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		secondWatcher := waitForWatcherPointer(
			serverForTest,
			firstWatcher,
			4*time.Second,
		)
		if secondWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(
					root,
					"missing-watch-root-second-iteration-timeout",
				),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		// Allow the config-restart iteration to execute BroadcastReload(waitApp=true).
		time.Sleep(150 * time.Millisecond)

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-config-restart"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after orchestration path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}
	if appHealthHits.Load() != 0 {
		t.Fatalf(
			"expected config restart path to reload config without waiting on stale app health probes, got %d probe(s)",
			appHealthHits.Load(),
		)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for orchestration helper goroutine")
	}
}

func TestWaitForBuildRetry_QueuedNoGoRestartIntentIsPreservedForNextPass(
	t *testing.T,
) {
	serverForTest := &Server{
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}
	serverForTest.QueueRestartRequest(
		restartengine.RestartRequest{RecompileGo: false},
	)

	restartRequestForRetry := serverForTest.WaitForBuildRetry()
	nextRunIntent := restartengine.DeriveRunIntentFromRestartRequest(
		restartRequestForRetry,
	)

	if nextRunIntent.RecompileGo {
		t.Fatalf(
			"expected queued retry restart to preserve recompileGo=false, got %#v",
			nextRunIntent,
		)
	}
	if restartRequestForRetry.IsConfigRestart {
		t.Fatalf(
			"expected queued retry restart to preserve isConfigRestart=false, got %#v",
			restartRequestForRetry,
		)
	}

	serverForTest.Mu.Lock()
	waitingForBuildRetry := serverForTest.WaitingForBuildRetry
	serverForTest.Mu.Unlock()
	if waitingForBuildRetry {
		t.Fatal("expected WaitForBuildRetry to clear waiting-for-retry guard")
	}
}

func TestQueueRestartRequest_WaitingForBuildRetryStillMergesToStrongestIntent(
	t *testing.T,
) {
	serverForTest := &Server{
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}
	serverForTest.SetWaitingForBuildRetry(true)

	serverForTest.QueueRestartRequest(
		restartengine.RestartRequest{RecompileGo: false},
	)
	serverForTest.QueueRestartRequest(
		restartengine.RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: true,
		},
	)

	restartRequestForRetry, hasRestartRequest := serverForTest.ConsumePendingRestartRequest()
	if !hasRestartRequest {
		t.Fatal("expected queued restart request while waiting for build retry")
	}
	if !restartRequestForRetry.RecompileGo ||
		!restartRequestForRetry.IsConfigRestart {
		t.Fatalf(
			"expected strongest merged restart intent while waiting for build retry, got %#v",
			restartRequestForRetry,
		)
	}
}

func TestServerRun_ConfigReloadFailureTerminatesRun(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	testCases := []struct {
		name               string
		writeInvalidConfig func(*testing.T, *waveconfig.ParsedConfig)
	}{
		{
			name: "syntax_error",
			writeInvalidConfig: func(t *testing.T, cfg *waveconfig.ParsedConfig) {
				t.Helper()
				if writeError := os.WriteFile(
					cfg.Core.ConfigLocation,
					[]byte("{ invalid config payload"),
					0o644,
				); writeError != nil {
					t.Fatalf("write invalid syntax config payload: %v", writeError)
				}
			},
		},
		{
			name: "validation_error",
			writeInvalidConfig: func(t *testing.T, cfg *waveconfig.ParsedConfig) {
				t.Helper()
				if writeError := writeToolingConfigForMainEntryAndWatchRoot(
					cfg.Core.ConfigLocation,
					cfg,
					"",
					cfg.Watch.WatchRoot,
				); writeError != nil {
					t.Fatalf("write invalid semantic config payload: %v", writeError)
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := newParsedConfigForToolingTestsAtRoot(root)
			cfg.Core.ServerOnlyMode = true
			cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				cfg.Watch.WatchRoot,
			); writeError != nil {
				t.Fatalf("failed writing initial tooling config: %v", writeError)
			}

			serverForTest := &Server{
				Cfg: cfg,
				Log: newDiscardLogger(),
				RestartIntents: restartengine.NewRestartIntentAccumulator(
					make(chan restartengine.RestartRequest, 1),
				),
			}

			runErrCh := make(chan error, 1)
			go func() {
				runErrCh <- serverForTest.Run()
			}()

			firstWatcher := waitForWatcherPointer(serverForTest, nil, 4*time.Second)
			if firstWatcher == nil {
				t.Fatal("timed out waiting for initial watcher setup")
			}

			testCase.writeInvalidConfig(t, cfg)
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{
					RecompileGo:     true,
					IsConfigRestart: true,
				},
				2*time.Second,
			)

			select {
			case runError := <-runErrCh:
				if runError == nil {
					t.Fatal("expected run to exit on config reload failure")
				}
				if !strings.Contains(
					runError.Error(),
					"reload config during cycle prepare",
				) {
					t.Fatalf("unexpected Run error: %v", runError)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("timed out waiting for run failure after config reload error")
			}
		})
	}
}

func TestServerRun_InvalidConfigWriteTriggersConfigRestartAndTerminatesRun(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- serverForTest.Run()
	}()

	firstWatcher := waitForWatcherPointer(serverForTest, nil, 4*time.Second)
	if firstWatcher == nil {
		t.Fatal("timed out waiting for initial watcher setup")
	}

	if writeError := os.WriteFile(
		cfg.Core.ConfigLocation,
		[]byte("{ invalid config payload"),
		0o644,
	); writeError != nil {
		t.Fatalf("write invalid syntax config payload: %v", writeError)
	}

	select {
	case runError := <-runErrCh:
		if runError == nil {
			t.Fatal("expected run to exit on config reload failure after watcher-side invalid write")
		}
		if !strings.Contains(runError.Error(), "reload config during cycle prepare") {
			t.Fatalf("unexpected Run error: %v", runError)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("timed out waiting for run failure after watcher-side invalid write")
	}
}

func TestWriteToolingConfigForWatchRoot_UpdatesConfigFileOnly(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")
	originalWatchRoot := cfg.Watch.WatchRoot
	updatedWatchRoot := filepath.Join(root, "updated-watch-root")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		updatedWatchRoot,
	); writeError != nil {
		t.Fatalf("failed to write updated tooling config: %v", writeError)
	}

	reloadedConfig, parseError := waveconfig.ParseConfigFile(cfg.Core.ConfigLocation)
	if parseError != nil {
		t.Fatalf("failed to parse updated config: %v", parseError)
	}

	if cfg.Watch.WatchRoot != originalWatchRoot {
		t.Fatalf(
			"expected base config watch root to remain %q, got %q",
			originalWatchRoot,
			cfg.Watch.WatchRoot,
		)
	}
	expectedUpdatedWatchRoot := filepath.Clean(updatedWatchRoot)
	if reloadedConfig.Watch == nil ||
		reloadedConfig.Watch.WatchRoot != expectedUpdatedWatchRoot {
		t.Fatalf(
			"expected reloaded config watch root to be %q, got %#v",
			expectedUpdatedWatchRoot,
			reloadedConfig.Watch,
		)
	}
}

func waitForWatcherPointer(
	serverForTest *Server,
	previousWatcher *watch.Watcher,
	timeout time.Duration,
) *watch.Watcher {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		serverForTest.Mu.Lock()
		currentWatcher := serverForTest.Watcher
		serverForTest.Mu.Unlock()

		if currentWatcher != nil && currentWatcher != previousWatcher {
			return currentWatcher
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func sendRestartRequestWithTimeout(
	serverForTest *Server,
	request restartengine.RestartRequest,
	timeout time.Duration,
) bool {
	if serverForTest == nil {
		return false
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		serverForTest.QueueRestartRequest(request)
		return true
	}
	return false
}

func waitForWaitingForBuildRetryFlag(
	serverForTest *Server,
	expectedFlag bool,
	timeout time.Duration,
) bool {
	if serverForTest == nil {
		return false
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		serverForTest.Mu.Lock()
		waitingForBuildRetry := serverForTest.WaitingForBuildRetry
		serverForTest.Mu.Unlock()

		if waitingForBuildRetry == expectedFlag {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func writeGoMainFileForDevserverRunTests(
	goMainPath string,
	fileContents string,
) error {
	if strings.TrimSpace(goMainPath) == "" {
		return errors.New("go main path is empty")
	}
	if writeError := os.WriteFile(
		goMainPath,
		[]byte(fileContents),
		0o644,
	); writeError != nil {
		return fmt.Errorf("write go main file: %w", writeError)
	}
	return nil
}

func writeToolingConfigForWatchRoot(
	configFilePath string,
	baseConfig *waveconfig.ParsedConfig,
	watchRoot string,
) error {
	return writeToolingConfigForMainEntryAndWatchRoot(
		configFilePath,
		baseConfig,
		baseConfig.Core.MainAppEntry,
		watchRoot,
	)
}

func writeToolingConfigForMainEntryAndWatchRoot(
	configFilePath string,
	baseConfig *waveconfig.ParsedConfig,
	mainAppEntry string,
	watchRoot string,
) error {
	if baseConfig == nil || baseConfig.Core == nil {
		return fmt.Errorf("base config missing required core section")
	}

	configForDisk := *baseConfig
	configCoreForDisk := *baseConfig.Core
	configCoreForDisk.ConfigLocation = configFilePath
	configCoreForDisk.MainAppEntry = mainAppEntry

	currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
	if currentWorkingDirectoryError != nil {
		return fmt.Errorf(
			"resolve current working directory: %w",
			currentWorkingDirectoryError,
		)
	}

	currentWorkingDirectoryRelativeMainAppEntry, mainAppEntryError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configCoreForDisk.MainAppEntry,
	)
	if mainAppEntryError != nil {
		return fmt.Errorf("normalize MainAppEntry: %w", mainAppEntryError)
	}
	configCoreForDisk.MainAppEntry = currentWorkingDirectoryRelativeMainAppEntry

	currentWorkingDirectoryRelativeDistDir, distDirError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configCoreForDisk.DistDir,
	)
	if distDirError != nil {
		return fmt.Errorf("normalize DistDir: %w", distDirError)
	}
	configCoreForDisk.DistDir = currentWorkingDirectoryRelativeDistDir

	currentWorkingDirectoryRelativePublicStaticDir, publicStaticDirError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configCoreForDisk.StaticAssetDirs.Public,
	)
	if publicStaticDirError != nil {
		return fmt.Errorf(
			"normalize StaticAssetDirs.Public: %w",
			publicStaticDirError,
		)
	}
	configCoreForDisk.StaticAssetDirs.Public = currentWorkingDirectoryRelativePublicStaticDir

	currentWorkingDirectoryRelativePrivateStaticDir, privateStaticDirError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configCoreForDisk.StaticAssetDirs.Private,
	)
	if privateStaticDirError != nil {
		return fmt.Errorf(
			"normalize StaticAssetDirs.Private: %w",
			privateStaticDirError,
		)
	}
	configCoreForDisk.StaticAssetDirs.Private = currentWorkingDirectoryRelativePrivateStaticDir

	currentWorkingDirectoryRelativeCriticalCSSEntry, criticalCSSEntryError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configCoreForDisk.CSSEntryFiles.Critical,
	)
	if criticalCSSEntryError != nil {
		return fmt.Errorf(
			"normalize CSSEntryFiles.Critical: %w",
			criticalCSSEntryError,
		)
	}
	configCoreForDisk.CSSEntryFiles.Critical = currentWorkingDirectoryRelativeCriticalCSSEntry

	currentWorkingDirectoryRelativeNonCriticalCSSEntry, nonCriticalCSSEntryError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configCoreForDisk.CSSEntryFiles.NonCritical,
	)
	if nonCriticalCSSEntryError != nil {
		return fmt.Errorf(
			"normalize CSSEntryFiles.NonCritical: %w",
			nonCriticalCSSEntryError,
		)
	}
	configCoreForDisk.CSSEntryFiles.NonCritical = currentWorkingDirectoryRelativeNonCriticalCSSEntry
	configForDisk.Core = &configCoreForDisk

	configWatchForDisk := &waveconfig.WatchConfig{}
	if baseConfig.Watch != nil {
		*configWatchForDisk = *baseConfig.Watch
	}

	currentWorkingDirectoryRelativeWatchRoot, watchRootError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		watchRoot,
	)
	if watchRootError != nil {
		return fmt.Errorf("normalize Watch.WatchRoot: %w", watchRootError)
	}
	configWatchForDisk.WatchRoot = currentWorkingDirectoryRelativeWatchRoot

	for excludeDirectoryPatternIndex, excludeDirectoryPattern := range configWatchForDisk.Exclude.Dirs {
		currentWorkingDirectoryRelativeExcludeDirectoryPattern, excludeDirectoryPatternError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
			currentWorkingDirectory,
			excludeDirectoryPattern,
		)
		if excludeDirectoryPatternError != nil {
			return fmt.Errorf(
				"normalize Watch.Exclude.Dirs[%d]: %w",
				excludeDirectoryPatternIndex,
				excludeDirectoryPatternError,
			)
		}
		configWatchForDisk.Exclude.Dirs[excludeDirectoryPatternIndex] = currentWorkingDirectoryRelativeExcludeDirectoryPattern
	}
	for excludeFilePatternIndex, excludeFilePattern := range configWatchForDisk.Exclude.Files {
		currentWorkingDirectoryRelativeExcludeFilePattern, excludeFilePatternError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
			currentWorkingDirectory,
			excludeFilePattern,
		)
		if excludeFilePatternError != nil {
			return fmt.Errorf(
				"normalize Watch.Exclude.Files[%d]: %w",
				excludeFilePatternIndex,
				excludeFilePatternError,
			)
		}
		configWatchForDisk.Exclude.Files[excludeFilePatternIndex] = currentWorkingDirectoryRelativeExcludeFilePattern
	}
	for watchIncludeIndex, watchedFile := range configWatchForDisk.Include {
		currentWorkingDirectoryRelativePattern, includePatternError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
			currentWorkingDirectory,
			watchedFile.Pattern,
		)
		if includePatternError != nil {
			return fmt.Errorf(
				"normalize Watch.Include[%d].Pattern: %w",
				watchIncludeIndex,
				includePatternError,
			)
		}
		configWatchForDisk.Include[watchIncludeIndex].Pattern = currentWorkingDirectoryRelativePattern
		for hookIndex, onChangeHook := range watchedFile.OnChangeHooks {
			for excludedPatternIndex, excludedPattern := range onChangeHook.Exclude {
				currentWorkingDirectoryRelativeExcludedPattern, excludedPatternError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
					currentWorkingDirectory,
					excludedPattern,
				)
				if excludedPatternError != nil {
					return fmt.Errorf(
						"normalize Watch.Include[%d].OnChangeHooks[%d].Exclude[%d]: %w",
						watchIncludeIndex,
						hookIndex,
						excludedPatternIndex,
						excludedPatternError,
					)
				}
				configWatchForDisk.Include[watchIncludeIndex].OnChangeHooks[hookIndex].Exclude[excludedPatternIndex] = currentWorkingDirectoryRelativeExcludedPattern
			}
		}
	}
	configForDisk.Watch = configWatchForDisk

	if baseConfig.Vite != nil {
		configViteForDisk := *baseConfig.Vite

		currentWorkingDirectoryRelativePackageManagerCommandDirectory, packageManagerCommandDirectoryError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
			currentWorkingDirectory,
			configViteForDisk.JSPackageManagerCmdDir,
		)
		if packageManagerCommandDirectoryError != nil {
			return fmt.Errorf(
				"normalize Vite.JSPackageManagerCmdDir: %w",
				packageManagerCommandDirectoryError,
			)
		}
		configViteForDisk.JSPackageManagerCmdDir = currentWorkingDirectoryRelativePackageManagerCommandDirectory

		currentWorkingDirectoryRelativeViteConfigFilePath, viteConfigFilePathError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
			currentWorkingDirectory,
			configViteForDisk.ViteConfigFile,
		)
		if viteConfigFilePathError != nil {
			return fmt.Errorf(
				"normalize Vite.ViteConfigFile: %w",
				viteConfigFilePathError,
			)
		}
		configViteForDisk.ViteConfigFile = currentWorkingDirectoryRelativeViteConfigFilePath
		configForDisk.Vite = &configViteForDisk
	}

	configForDiskJSON, marshalError := json.Marshal(configForDisk)
	if marshalError != nil {
		return fmt.Errorf("marshal updated tooling config: %w", marshalError)
	}

	if writeError := os.WriteFile(
		configFilePath,
		configForDiskJSON,
		0o644,
	); writeError != nil {
		return fmt.Errorf("write tooling config file: %w", writeError)
	}
	return nil
}

func pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
	currentWorkingDirectory string,
	configuredPath string,
) (string, error) {
	return testpath.PathRelativeToConfiguredWorkingDirectory(
		currentWorkingDirectory,
		configuredPath,
	)
}

func mustConfigureAndGetWaveAppPortForDevserverRunTests(t *testing.T) int {
	t.Helper()

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
	t.Setenv("PORT", fmt.Sprintf("%d", port))

	return port
}
