package devserver

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"

	"github.com/vormadev/vorma/internal/testpath"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/restartengine"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
	"github.com/vormadev/vorma/wave/internal/wavelock"
)

func TestRunDev_ReturnsValidationErrorForInvalidConfig(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t, t.TempDir())
	wavetest.SetWatchHealthcheckEndpoint(cfg, "invalid-healthcheck-endpoint")

	runError := RunDev(cfg, "", newDiscardLogger())
	if runError == nil {
		t.Fatal(
			"expected RunDev to fail validation for invalid healthcheck endpoint",
		)
	}
	if !strings.Contains(runError.Error(), "config validation failed") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}
}

func TestRunDev_ReturnsErrorForNilConfig(t *testing.T) {
	runError := RunDev(nil, "", newDiscardLogger())
	if runError == nil {
		t.Fatal("expected RunDev to fail for nil config")
	}
	if !strings.Contains(runError.Error(), "config is nil") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}
}

func TestRunDev_ReturnsLockHeldErrorWhenProjectIsAlreadyLocked(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)

	lock := wavelock.NewDevLock(cfg.Dist().Static())
	if lockAcquireError := lock.Acquire(); lockAcquireError != nil {
		t.Fatalf("failed to acquire initial lock: %v", lockAcquireError)
	}
	defer func() {
		_ = lock.Release()
	}()

	runError := RunDev(cfg, "", newDiscardLogger())
	if runError == nil {
		t.Fatal("expected RunDev to fail when lock is already held")
	}
	if !errors.Is(runError, wavelock.ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got: %v", runError)
	}
}

func TestRunDev_WithNilLoggerReleasesLockWhenRunReturnsError(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)
	configFilePath := filepath.Join(root, "missing-config-dir", "wave.config.json")

	runError := RunDev(cfg, configFilePath, nil)
	if runError == nil {
		t.Fatal("expected RunDev to fail when config file directory does not exist")
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}

	lock := wavelock.NewDevLock(cfg.Dist().Static())
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

func TestServerRun_ReturnsInitWatcherErrorWhenConfigFileDirectoryMissing(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)
	configFilePath := filepath.Join(root, "missing-config-dir", "wave.config.json")

	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal("expected Run to fail when config file directory does not exist")
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}
}

func TestServerRun_BuildFailureThenRetryThenConfigReadFailure(t *testing.T) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)
	wavetest.SetCoreMainAppEntry(cfg, "missing/package/for/devserver/run")
	configFilePath := filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForResolveRoot(
		configFilePath,
		cfg,
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            newDiscardLogger(),
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

		if removeError := os.Remove(configFilePath); removeError != nil {
			t.Errorf("remove config file before retry: %v", removeError)
			return
		}
		serverForTest.QueueRestartRequest(
			restartengine.RestartRequest{RecompileGo: false},
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with config read error after retry cycle",
		)
	}
	if !strings.Contains(runError.Error(), "read config file") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for Run helper goroutine")
	}
}

func TestServerRun_SequentialCompileFailureThenRetryThenConfigReadFailure(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)
	wavetest.SetCoreSequentialGoBuild(cfg, true)
	wavetest.SetCoreMainAppEntry(cfg, "missing/package/for/devserver/sequential")
	configFilePath := filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForResolveRoot(
		configFilePath,
		cfg,
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            newDiscardLogger(),
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

		if removeError := os.Remove(configFilePath); removeError != nil {
			t.Errorf("remove config file before sequential retry: %v", removeError)
			return
		}
		serverForTest.QueueRestartRequest(
			restartengine.RestartRequest{RecompileGo: false},
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with config read error after sequential compile retry",
		)
	}
	if !strings.Contains(runError.Error(), "read config file") {
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
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)
	wavetest.SetCoreMainAppEntry(cfg, "missing/package/for/devserver/recovery")
	configFilePath := filepath.Join(root, "wave.config.json")
	fixedMainAppEntryAbsolutePath, fixedMainAppEntryAbsolutePathError := writeStableGoMainEntryForDevserverRunTests(
		root,
	)
	if fixedMainAppEntryAbsolutePathError != nil {
		t.Fatalf(
			"write fixed main app entry: %v",
			fixedMainAppEntryAbsolutePathError,
		)
	}
	fixedMainAppEntryPath, fixedMainAppEntryPathError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		filepath.Dir(configFilePath),
		fixedMainAppEntryAbsolutePath,
	)
	if fixedMainAppEntryPathError != nil {
		t.Fatalf(
			"resolve fixed main app entry path: %v",
			fixedMainAppEntryPathError,
		)
	}

	if writeError := writeToolingConfigForMainEntryAndResolveRoot(
		configFilePath,
		cfg,
		cfg.Core().MainAppEntry(),
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(root, "missing-resolve-root-initial-timeout"),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(root, "missing-resolve-root-retry-timeout"),
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

		if writeError := writeToolingConfigForMainEntryAndResolveRoot(
			configFilePath,
			cfg,
			fixedMainAppEntryPath,
			cfg.ConfigFileDirectory(),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(root, "missing-resolve-root-recovery-timeout"),
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

		if writeError := writeToolingConfigForMainEntryAndResolveRoot(
			configFilePath,
			cfg,
			fixedMainAppEntryPath,
			filepath.Join(root, "missing-resolve-root-after-recovery"),
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
	if !strings.Contains(runError.Error(), "read config file") {
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
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	wavetest.SetWatchHealthcheckEndpoint(cfg, "/healthz")
	configFilePath := filepath.Join(root, "wave.config.json")
	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating go main parent dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPublic(), 0o755); mkdirError != nil {
		t.Fatalf("failed creating public static dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPrivate(), 0o755); mkdirError != nil {
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
	wavetest.SetCoreMainAppEntry(cfg, goMainPath)

	if writeError := writeToolingConfigForMainEntryAndResolveRoot(
		configFilePath,
		cfg,
		cfg.Core().MainAppEntry(),
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(root, "missing-resolve-root-go-syntax-recovery-timeout"),
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

		if writeError := writeToolingConfigForResolveRoot(
			configFilePath,
			cfg,
			filepath.Join(root, "missing-resolve-root-after-go-syntax-recovery"),
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
	if !strings.Contains(runError.Error(), "read config file") {
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
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	wavetest.SetWatchHealthcheckEndpoint(cfg, "/healthz")
	configFilePath := filepath.Join(root, "wave.config.json")
	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating go main parent dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPublic(), 0o755); mkdirError != nil {
		t.Fatalf("failed creating public static dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPrivate(), 0o755); mkdirError != nil {
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
	wavetest.SetCoreMainAppEntry(cfg, goMainPath)

	if writeError := writeToolingConfigForMainEntryAndResolveRoot(
		configFilePath,
		cfg,
		cfg.Core().MainAppEntry(),
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(root, "missing-resolve-root-go-type-recovery-timeout"),
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

		if writeError := writeToolingConfigForResolveRoot(
			configFilePath,
			cfg,
			filepath.Join(root, "missing-resolve-root-after-go-type-recovery"),
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
	if !strings.Contains(runError.Error(), "read config file") {
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
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	wavetest.SetWatchHealthcheckEndpoint(cfg, "/healthz")
	configFilePath := filepath.Join(root, "wave.config.json")
	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating go main parent dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPublic(), 0o755); mkdirError != nil {
		t.Fatalf("failed creating public static dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPrivate(), 0o755); mkdirError != nil {
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
	wavetest.SetCoreMainAppEntry(cfg, goMainPath)

	if writeError := writeToolingConfigForMainEntryAndResolveRoot(
		configFilePath,
		cfg,
		cfg.Core().MainAppEntry(),
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
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
			_ = writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(
					root,
					"missing-resolve-root-main-entry-recovery-timeout",
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

		if writeError := writeToolingConfigForMainEntryAndResolveRoot(
			configFilePath,
			cfg,
			goMainPath+"2",
			cfg.ConfigFileDirectory(),
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
		if writeError := writeToolingConfigForMainEntryAndResolveRoot(
			configFilePath,
			cfg,
			goMainPath,
			cfg.ConfigFileDirectory(),
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

		if writeError := writeToolingConfigForResolveRoot(
			configFilePath,
			cfg,
			filepath.Join(root, "missing-resolve-root-after-main-entry-recovery"),
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
	if !strings.Contains(runError.Error(), "read config file") {
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
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	wavetest.SetWatchHealthcheckEndpoint(cfg, "/healthz")
	configFilePath := filepath.Join(root, "wave.config.json")
	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating go main parent dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPublic(), 0o755); mkdirError != nil {
		t.Fatalf("failed creating public static dir: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPrivate(), 0o755); mkdirError != nil {
		t.Fatalf("failed creating private static dir: %v", mkdirError)
	}
	privateStaticPath := filepath.Join(
		cfg.Core().StaticAssetDirsPrivate(),
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
	wavetest.SetCoreMainAppEntry(cfg, goMainPath)

	if writeError := writeToolingConfigForMainEntryAndResolveRoot(
		configFilePath,
		cfg,
		cfg.Core().MainAppEntry(),
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
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
			_ = writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(
					root,
					"missing-resolve-root-main-entry-non-config-recovery-timeout",
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

		if writeError := writeToolingConfigForMainEntryAndResolveRoot(
			configFilePath,
			cfg,
			goMainPath+"2",
			cfg.ConfigFileDirectory(),
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
		if writeError := writeToolingConfigForMainEntryAndResolveRoot(
			configFilePath,
			cfg,
			goMainPath,
			cfg.ConfigFileDirectory(),
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

		if writeError := writeToolingConfigForResolveRoot(
			configFilePath,
			cfg,
			filepath.Join(root, "missing-resolve-root-after-main-entry-non-config-recovery"),
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
	if !strings.Contains(runError.Error(), "read config file") {
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
		writeConfigError func(*testing.T, waveconfig.ParsedConfig, string)
	}{
		{
			name: "syntax_error",
			writeConfigError: func(
				t *testing.T,
				cfg waveconfig.ParsedConfig,
				configFilePath string,
			) {
				t.Helper()
				if writeError := os.WriteFile(
					configFilePath,
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
			writeConfigError: func(
				t *testing.T,
				cfg waveconfig.ParsedConfig,
				configFilePath string,
			) {
				t.Helper()
				invalidButSyntacticallyValidConfigJSON := []byte(
					`{"Core":{"ProjectID":"devserver-run-validation-error"}}`,
				)
				if writeError := os.WriteFile(
					configFilePath,
					invalidButSyntacticallyValidConfigJSON,
					0o644,
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
			cfg := newParsedConfigForToolingTestsAtRoot(t, root)
			wavetest.SetCoreServerOnlyMode(cfg, false)
			wavetest.SetWatchHealthcheckEndpoint(cfg, "/healthz")
			configFilePath := filepath.Join(root, "wave.config.json")
			goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
			if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
				t.Fatalf("failed creating go main parent dir: %v", mkdirError)
			}
			if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPublic(), 0o755); mkdirError != nil {
				t.Fatalf("failed creating public static dir: %v", mkdirError)
			}
			if mkdirError := os.MkdirAll(cfg.Core().StaticAssetDirsPrivate(), 0o755); mkdirError != nil {
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
			wavetest.SetCoreMainAppEntry(cfg, goMainPath)

			if writeError := writeToolingConfigForMainEntryAndResolveRoot(
				configFilePath,
				cfg,
				cfg.Core().MainAppEntry(),
				cfg.ConfigFileDirectory(),
			); writeError != nil {
				t.Fatalf(
					"failed writing initial tooling config: %v",
					writeError,
				)
			}

			var runLogBuffer bytes.Buffer
			serverForTest := &Server{
				Cfg:            cfg,
				ConfigFilePath: configFilePath,
				Log: slog.New(
					slog.NewTextHandler(&runLogBuffer, nil),
				),
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
				t.Fatal(
					"timed out waiting for initial watcher before config-error run sequence",
				)
			}

			if writeError := writeToolingConfigForMainEntryAndResolveRoot(
				configFilePath,
				cfg,
				goMainPath+"2",
				cfg.ConfigFileDirectory(),
			); writeError != nil {
				t.Fatalf(
					"failed writing broken main entry config: %v",
					writeError,
				)
			}

			if !waitForWaitingForBuildRetryFlag(
				serverForTest,
				true,
				5*time.Second,
			) {
				t.Fatal(
					"timed out waiting for build-retry state after main entry typo",
				)
			}

			testCase.writeConfigError(t, cfg, configFilePath)

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
				t.Fatal(
					"timed out waiting for run failure after config error event",
				)
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

func TestServerRun_NoOpConfigWriteFirstSaveDoesNotRestartWatcher(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)
	mainAppEntryPath, mainAppEntryPathError := writeStableGoMainEntryForDevserverRunTests(
		root,
	)
	if mainAppEntryPathError != nil {
		t.Fatalf("write main app entry: %v", mainAppEntryPathError)
	}
	wavetest.SetCoreMainAppEntry(cfg, mainAppEntryPath)
	configFilePath := filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForResolveRoot(
		configFilePath,
		cfg,
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}
	configFromDisk, parseError := waveconfig.ParseConfigFile(
		configFilePath,
	)
	if parseError != nil {
		t.Fatalf("parse initial tooling config from disk: %v", parseError)
	}
	waveframework.CopyRuntimeStateForToolingReload(configFromDisk, cfg)
	cfg = configFromDisk

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(root, "missing-resolve-root-noop-initial-timeout"),
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

		unchangedConfigBytes, readError := os.ReadFile(configFilePath)
		if readError != nil {
			t.Error(readError)
			return
		}
		if writeError := os.WriteFile(
			configFilePath,
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

		if writeError := writeToolingConfigForResolveRoot(
			configFilePath,
			cfg,
			filepath.Join(root, "missing-resolve-root-after-noop"),
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
	if !strings.Contains(runError.Error(), "read config file") {
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

}

func TestServerRun_ViteStartFailureExitsRunLifecycleImmediately(t *testing.T) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)
	mainAppEntryPath, mainAppEntryPathError := writeStableGoMainEntryForDevserverRunTests(
		root,
	)
	if mainAppEntryPathError != nil {
		t.Fatalf("write main app entry: %v", mainAppEntryPathError)
	}
	wavetest.SetCoreMainAppEntry(cfg, mainAppEntryPath)
	ensureViteConfigForToolingTests(t, cfg)
	wavetest.SetViteJSPackageManagerBaseCmd(cfg, "command_that_does_not_exist_for_wave_run_vite_test")
	wavetest.SetViteDefaultPort(cfg, 5199)
	configFilePath := filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForResolveRoot(
		configFilePath,
		cfg,
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            newDiscardLogger(),
	}

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal("expected Run to fail when Vite startup fails")
	}
	if !strings.Contains(runError.Error(), "start vite:") {
		t.Fatalf("unexpected Run error: %v", runError)
	}
	if _, statError := os.Stat(cfg.Dist().Binary()); statError != nil {
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
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	mainAppEntryPath, mainAppEntryPathError := writeStableGoMainEntryForDevserverRunTests(
		root,
	)
	if mainAppEntryPathError != nil {
		t.Fatalf("write main app entry: %v", mainAppEntryPathError)
	}
	wavetest.SetCoreMainAppEntry(cfg, mainAppEntryPath)
	wavetest.SetWatchHealthcheckEndpoint(cfg, "/healthz")
	configFilePath := filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForResolveRoot(
		configFilePath,
		cfg,
		cfg.ConfigFileDirectory(),
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
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            newDiscardLogger(),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(root, "missing-resolve-root-fallback"),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(root, "missing-resolve-root-send-timeout"),
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
			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				filepath.Join(
					root,
					"missing-resolve-root-second-iteration-timeout",
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

		if writeError := writeToolingConfigForResolveRoot(
			configFilePath,
			cfg,
			filepath.Join(root, "missing-resolve-root-after-config-restart"),
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
	if !strings.Contains(runError.Error(), "read config file") {
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
		writeInvalidConfig func(*testing.T, waveconfig.ParsedConfig, string)
	}{
		{
			name: "syntax_error",
			writeInvalidConfig: func(
				t *testing.T,
				cfg waveconfig.ParsedConfig,
				configFilePath string,
			) {
				t.Helper()
				if writeError := os.WriteFile(
					configFilePath,
					[]byte("{ invalid config payload"),
					0o644,
				); writeError != nil {
					t.Fatalf(
						"write invalid syntax config payload: %v",
						writeError,
					)
				}
			},
		},
		{
			name: "validation_error",
			writeInvalidConfig: func(
				t *testing.T,
				cfg waveconfig.ParsedConfig,
				configFilePath string,
			) {
				t.Helper()
				invalidButSyntacticallyValidConfigJSON := []byte(
					`{"Core":{"ProjectID":"devserver-run-validation-error"}}`,
				)
				if writeError := os.WriteFile(
					configFilePath,
					invalidButSyntacticallyValidConfigJSON,
					0o644,
				); writeError != nil {
					t.Fatalf(
						"write invalid semantic config payload: %v",
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
			cfg := newParsedConfigForToolingTestsAtRoot(t, root)
			wavetest.SetCoreServerOnlyMode(cfg, true)
			configFilePath := filepath.Join(root, "wave.config.json")

			if writeError := writeToolingConfigForResolveRoot(
				configFilePath,
				cfg,
				cfg.ConfigFileDirectory(),
			); writeError != nil {
				t.Fatalf(
					"failed writing initial tooling config: %v",
					writeError,
				)
			}

			serverForTest := &Server{
				Cfg:            cfg,
				ConfigFilePath: configFilePath,
				Log:            newDiscardLogger(),
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
				4*time.Second,
			)
			if firstWatcher == nil {
				t.Fatal("timed out waiting for initial watcher setup")
			}

			testCase.writeInvalidConfig(t, cfg, configFilePath)
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
				t.Fatal(
					"timed out waiting for run failure after config reload error",
				)
			}
		})
	}
}

func TestServerRun_InvalidConfigWriteTriggersConfigRestartAndTerminatesRun(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)
	configFilePath := filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForResolveRoot(
		configFilePath,
		cfg,
		cfg.ConfigFileDirectory(),
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg:            cfg,
		ConfigFilePath: configFilePath,
		Log:            newDiscardLogger(),
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
		configFilePath,
		[]byte("{ invalid config payload"),
		0o644,
	); writeError != nil {
		t.Fatalf("write invalid syntax config payload: %v", writeError)
	}

	select {
	case runError := <-runErrCh:
		if runError == nil {
			t.Fatal(
				"expected run to exit on config reload failure after watcher-side invalid write",
			)
		}
		if !strings.Contains(
			runError.Error(),
			"reload config during cycle prepare",
		) {
			t.Fatalf("unexpected Run error: %v", runError)
		}
	case <-time.After(6 * time.Second):
		t.Fatal(
			"timed out waiting for run failure after watcher-side invalid write",
		)
	}
}

func TestWriteToolingConfigForResolveRoot_DifferentResolveRootRemovesConfigFile(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	configFilePath := filepath.Join(root, "wave.config.json")
	originalResolveRoot := cfg.ConfigFileDirectory()
	updatedResolveRoot := filepath.Join(root, "updated-resolve-root")

	if writeError := writeToolingConfigForResolveRoot(
		configFilePath,
		cfg,
		updatedResolveRoot,
	); writeError != nil {
		t.Fatalf("failed to write updated tooling config: %v", writeError)
	}

	if cfg.ConfigFileDirectory() != originalResolveRoot {
		t.Fatalf(
			"expected base config resolve root to remain %q, got %q",
			originalResolveRoot,
			cfg.ConfigFileDirectory(),
		)
	}
	if _, statError := os.Stat(configFilePath); !errors.Is(
		statError,
		os.ErrNotExist,
	) {
		t.Fatalf(
			"expected tooling config file %q to be removed for mismatched resolve root",
			configFilePath,
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

func writeStableGoMainEntryForDevserverRunTests(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("root is empty")
	}

	goMainPath := filepath.Join(root, "backend", "cmd", "app_main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		return "", fmt.Errorf("create go main parent dir: %w", mkdirError)
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
		return "", writeError
	}
	return goMainPath, nil
}

func writeToolingConfigForResolveRoot(
	configFilePath string,
	baseConfig waveconfig.ParsedConfig,
	resolveRoot string,
) error {
	if baseConfig == nil || baseConfig.Core() == nil {
		return fmt.Errorf("base config missing required core section")
	}
	trimmedResolveRoot := strings.TrimSpace(resolveRoot)
	if trimmedResolveRoot != "" &&
		filepath.Clean(trimmedResolveRoot) !=
			filepath.Clean(baseConfig.ConfigFileDirectory()) {
		removeConfigFileError := os.Remove(configFilePath)
		if removeConfigFileError != nil &&
			!errors.Is(removeConfigFileError, os.ErrNotExist) {
			return fmt.Errorf(
				"remove tooling config file: %w",
				removeConfigFileError,
			)
		}
		return nil
	}

	return writeToolingConfigForMainEntryAndResolveRoot(
		configFilePath,
		baseConfig,
		baseConfig.Core().MainAppEntry(),
		baseConfig.ConfigFileDirectory(),
	)
}

func writeToolingConfigForMainEntryAndResolveRoot(
	configFilePath string,
	baseConfig waveconfig.ParsedConfig,
	mainAppEntry string,
	resolveRoot string,
) error {
	if baseConfig == nil || baseConfig.Core() == nil {
		return fmt.Errorf("base config missing required core section")
	}
	trimmedResolveRoot := strings.TrimSpace(resolveRoot)
	if trimmedResolveRoot != "" &&
		filepath.Clean(trimmedResolveRoot) !=
			filepath.Clean(baseConfig.ConfigFileDirectory()) {
		removeConfigFileError := os.Remove(configFilePath)
		if removeConfigFileError != nil &&
			!errors.Is(removeConfigFileError, os.ErrNotExist) {
			return fmt.Errorf(
				"remove tooling config file: %w",
				removeConfigFileError,
			)
		}
		return nil
	}

	configForDisk := baseConfig.Clone()
	if configForDisk.Core() == nil {
		return fmt.Errorf("base config missing required core section")
	}

	currentWorkingDirectory := filepath.Dir(configFilePath)

	currentWorkingDirectoryRelativeMainAppEntry, mainAppEntryError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		mainAppEntry,
	)
	if mainAppEntryError != nil {
		return fmt.Errorf("normalize MainAppEntry: %w", mainAppEntryError)
	}
	wavetest.SetCoreMainAppEntry(
		configForDisk,
		currentWorkingDirectoryRelativeMainAppEntry,
	)

	currentWorkingDirectoryRelativePublicStaticDir, publicStaticDirError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configForDisk.Core().StaticAssetDirsPublic(),
	)
	if publicStaticDirError != nil {
		return fmt.Errorf(
			"normalize StaticAssetDirs.Public: %w",
			publicStaticDirError,
		)
	}
	wavetest.SetCoreStaticAssetDirsPublic(
		configForDisk,
		currentWorkingDirectoryRelativePublicStaticDir,
	)

	currentWorkingDirectoryRelativePrivateStaticDir, privateStaticDirError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configForDisk.Core().StaticAssetDirsPrivate(),
	)
	if privateStaticDirError != nil {
		return fmt.Errorf(
			"normalize StaticAssetDirs.Private: %w",
			privateStaticDirError,
		)
	}
	wavetest.SetCoreStaticAssetDirsPrivate(
		configForDisk,
		currentWorkingDirectoryRelativePrivateStaticDir,
	)

	currentWorkingDirectoryRelativeCriticalCSSEntry, criticalCSSEntryError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configForDisk.Core().CriticalCSSEntryFile(),
	)
	if criticalCSSEntryError != nil {
		return fmt.Errorf(
			"normalize CSSEntryFiles.Critical: %w",
			criticalCSSEntryError,
		)
	}
	wavetest.SetCoreCriticalCSSEntryFile(
		configForDisk,
		currentWorkingDirectoryRelativeCriticalCSSEntry,
	)

	currentWorkingDirectoryRelativeNonCriticalCSSEntry, nonCriticalCSSEntryError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
		currentWorkingDirectory,
		configForDisk.Core().NonCriticalCSSEntryFile(),
	)
	if nonCriticalCSSEntryError != nil {
		return fmt.Errorf(
			"normalize CSSEntryFiles.NonCritical: %w",
			nonCriticalCSSEntryError,
		)
	}
	wavetest.SetCoreNonCriticalCSSEntryFile(
		configForDisk,
		currentWorkingDirectoryRelativeNonCriticalCSSEntry,
	)

	configWatchForDisk := configForDisk.Watch()
	if configWatchForDisk != nil {
		excludeDirectoryPatterns := configWatchForDisk.ExcludeDirs()
		for excludeDirectoryPatternIndex, excludeDirectoryPattern := range excludeDirectoryPatterns {
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
			excludeDirectoryPatterns[excludeDirectoryPatternIndex] = currentWorkingDirectoryRelativeExcludeDirectoryPattern
		}
		wavetest.SetWatchExcludeDirs(configForDisk, excludeDirectoryPatterns)

		excludeFilePatterns := configWatchForDisk.ExcludeFiles()
		for excludeFilePatternIndex, excludeFilePattern := range excludeFilePatterns {
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
			excludeFilePatterns[excludeFilePatternIndex] = currentWorkingDirectoryRelativeExcludeFilePattern
		}
		wavetest.SetWatchExcludeFiles(configForDisk, excludeFilePatterns)

		includeWatchedFiles := configWatchForDisk.Include()
		for watchIncludeIndex, watchedFile := range includeWatchedFiles {
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
			includeWatchedFiles[watchIncludeIndex].Pattern = currentWorkingDirectoryRelativePattern
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
					includeWatchedFiles[watchIncludeIndex].OnChangeHooks[hookIndex].Exclude[excludedPatternIndex] = currentWorkingDirectoryRelativeExcludedPattern
				}
			}
		}
		wavetest.SetWatchInclude(configForDisk, includeWatchedFiles)
	}

	configViteForDisk := configForDisk.Vite()
	if configViteForDisk != nil {
		currentWorkingDirectoryRelativePackageManagerCommandDirectory, packageManagerCommandDirectoryError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
			currentWorkingDirectory,
			configViteForDisk.JSPackageManagerCmdDir(),
		)
		if packageManagerCommandDirectoryError != nil {
			return fmt.Errorf(
				"normalize Vite.JSPackageManagerCmdDir: %w",
				packageManagerCommandDirectoryError,
			)
		}
		wavetest.SetViteJSPackageManagerCmdDir(
			configForDisk,
			currentWorkingDirectoryRelativePackageManagerCommandDirectory,
		)

		currentWorkingDirectoryRelativeViteConfigFilePath, viteConfigFilePathError := pathRelativeToCurrentWorkingDirectoryForDevserverRunConfigJSON(
			currentWorkingDirectory,
			configViteForDisk.ViteConfigFile(),
		)
		if viteConfigFilePathError != nil {
			return fmt.Errorf(
				"normalize Vite.ViteConfigFile: %w",
				viteConfigFilePathError,
			)
		}
		wavetest.SetViteConfigFile(
			configForDisk,
			currentWorkingDirectoryRelativeViteConfigFilePath,
		)
	}

	configForDiskJSON, marshalError := wavetest.MarshalParsedConfigToRawJSON(
		configForDisk,
	)
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
