package vitecmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	testHelperProcessEnv = "VITECMD_TEST_HELPER_PROCESS"
	testHelperModeEnv    = "VITECMD_TEST_HELPER_MODE"
)

// TestBuildCtxHelperProcess is executed in a subprocess to emulate vite command behavior.
func TestBuildCtxHelperProcess(t *testing.T) {
	if os.Getenv(testHelperProcessEnv) != "1" {
		return
	}

	switch os.Getenv(testHelperModeEnv) {
	case "block_until_terminated":
		waitForTerminationSignalAndExit()
	case "exit_immediately":
		os.Exit(0)
	case "prod_success":
		runProdBuildHelperAndExit(0)
	case "prod_success_require_absolute_outdir":
		runProdBuildHelperRequiringAbsoluteOutDirAndExit(0)
	case "prod_fail":
		os.Exit(17)
	default:
		os.Exit(0)
	}
}

func TestNewBuildCtx_DefaultPortAndNilOptions(t *testing.T) {
	ctx := NewBuildCtx(nil)
	if got := ctx.Port(); got != 5199 {
		t.Fatalf("expected default port 5199, got %d", got)
	}
}

func TestViteConfigFileArgumentPath(t *testing.T) {
	commandWorkingDirectory := filepath.Join(t.TempDir(), "frontend")
	if err := os.MkdirAll(commandWorkingDirectory, 0o755); err != nil {
		t.Fatalf("create command working directory: %v", err)
	}
	configFileInCommandWorkingDirectory := filepath.Join(
		commandWorkingDirectory,
		"vite.custom.config.ts",
	)
	configFileOutsideCommandWorkingDirectory := filepath.Join(
		filepath.Dir(commandWorkingDirectory),
		"vite.root.config.ts",
	)

	testCases := []struct {
		name                    string
		viteConfigFile          string
		commandWorkingDirectory string
		expectedConfigArg       string
	}{
		{
			name:                    "relative config path is preserved",
			viteConfigFile:          "./vite.custom.config.ts",
			commandWorkingDirectory: commandWorkingDirectory,
			expectedConfigArg:       "./vite.custom.config.ts",
		},
		{
			name:                    "absolute config file under command directory becomes dot-slash relative",
			viteConfigFile:          configFileInCommandWorkingDirectory,
			commandWorkingDirectory: commandWorkingDirectory,
			expectedConfigArg:       "./vite.custom.config.ts",
		},
		{
			name:                    "absolute config file outside command directory becomes dot-dot relative",
			viteConfigFile:          configFileOutsideCommandWorkingDirectory,
			commandWorkingDirectory: commandWorkingDirectory,
			expectedConfigArg:       "../vite.root.config.ts",
		},
		{
			name:                    "absolute config file remains absolute when command directory is omitted",
			viteConfigFile:          configFileInCommandWorkingDirectory,
			commandWorkingDirectory: "",
			expectedConfigArg:       filepath.Clean(configFileInCommandWorkingDirectory),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			configArg := viteConfigFileArgumentPath(
				testCase.viteConfigFile,
				testCase.commandWorkingDirectory,
			)
			if configArg != testCase.expectedConfigArg {
				t.Fatalf(
					"viteConfigFileArgumentPath(%q, %q) = %q, want %q",
					testCase.viteConfigFile,
					testCase.commandWorkingDirectory,
					configArg,
					testCase.expectedConfigArg,
				)
			}
		})
	}
}

func TestDevBuild_ReturnsErrorForEmptyBaseCommand(t *testing.T) {
	ctx := NewBuildCtx(&BuildCtxOptions{JSPackageManagerBaseCmd: "   "})
	err := ctx.DevBuild()
	if err == nil || !strings.Contains(err.Error(), "JSPackageManagerBaseCmd is required") {
		t.Fatalf("expected missing command error, got %v", err)
	}
}

func TestProdBuild_ReturnsErrorForEmptyBaseCommand(t *testing.T) {
	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: "   ",
		OutDir:                  filepath.Join(t.TempDir(), "dist"),
		ManifestOut:             filepath.Join(t.TempDir(), "manifest.json"),
	})
	err := ctx.ProdBuild()
	if err == nil || !strings.Contains(err.Error(), "JSPackageManagerBaseCmd is required") {
		t.Fatalf("expected missing command error, got %v", err)
	}
}

func TestProdBuild_ReturnsErrorForMissingOutDir(t *testing.T) {
	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: "echo",
		ManifestOut:             filepath.Join(t.TempDir(), "manifest.json"),
	})

	err := ctx.ProdBuild()
	if err == nil || !strings.Contains(err.Error(), "OutDir is required") {
		t.Fatalf("expected missing OutDir error, got %v", err)
	}
}

func TestProdBuild_ReturnsErrorForMissingManifestOut(t *testing.T) {
	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: "echo",
		OutDir:                  filepath.Join(t.TempDir(), "dist"),
	})

	err := ctx.ProdBuild()
	if err == nil || !strings.Contains(err.Error(), "ManifestOut is required") {
		t.Fatalf("expected missing ManifestOut error, got %v", err)
	}
}

func TestDevBuild_ReturnsStartError(t *testing.T) {
	stubInitPort(t, func(port int) (int, error) {
		return port, nil
	})

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: "command_that_does_not_exist_for_vitecmd_tests",
		DefaultPort:             5199,
	})

	if err := ctx.DevBuild(); err == nil {
		t.Fatal("expected command start error")
	}
}

func TestDevBuild_UsesInitPortAndAppendsExpectedArgs(t *testing.T) {
	t.Setenv(testHelperProcessEnv, "1")
	t.Setenv(testHelperModeEnv, "block_until_terminated")

	stubInitPort(t, func(port int) (int, error) {
		return 6200, nil
	})

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: helperBaseCommand(t),
		DefaultPort:             5199,
	})

	if err := ctx.DevBuild(); err != nil {
		t.Fatalf("DevBuild() error = %v", err)
	}
	t.Cleanup(ctx.Cleanup)

	if got := ctx.Port(); got != 6200 {
		t.Fatalf("expected initPort return value to become current port, got %d", got)
	}

	ctx.mu.Lock()
	args := append([]string(nil), ctx.cmd.Args...)
	ctx.mu.Unlock()

	assertArgPair(t, args, "--port", "6200")
	assertArgPair(t, args, "--host", "127.0.0.1")
	assertArgPair(t, args, "--clearScreen", "false")
	assertArgPair(t, args, "--strictPort", "true")
}

func TestWaitAndCleanup_ConcurrentWaitsReturn(t *testing.T) {
	t.Setenv(testHelperProcessEnv, "1")
	t.Setenv(testHelperModeEnv, "block_until_terminated")

	stubInitPort(t, func(port int) (int, error) {
		return port, nil
	})

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: helperBaseCommand(t),
		DefaultPort:             5199,
	})

	if err := ctx.DevBuild(); err != nil {
		t.Fatalf("DevBuild() error = %v", err)
	}
	t.Cleanup(ctx.Cleanup)

	waitFinishedA := make(chan struct{})
	waitFinishedB := make(chan struct{})

	go func() {
		ctx.Wait()
		close(waitFinishedA)
	}()

	waitForCondition(t, 2*time.Second, func() bool {
		ctx.mu.Lock()
		defer ctx.mu.Unlock()
		return ctx.waitInProgress
	}, "wait did not enter waitInProgress state")

	go func() {
		ctx.Wait()
		close(waitFinishedB)
	}()

	ctx.Cleanup()

	select {
	case <-waitFinishedA:
	case <-time.After(2 * time.Second):
		t.Fatal("first Wait() did not return after Cleanup()")
	}

	select {
	case <-waitFinishedB:
	case <-time.After(2 * time.Second):
		t.Fatal("second Wait() did not return after Cleanup()")
	}
}

func TestWait_ConcurrentStress(t *testing.T) {
	t.Setenv(testHelperProcessEnv, "1")
	t.Setenv(testHelperModeEnv, "block_until_terminated")

	stubInitPort(t, func(port int) (int, error) {
		return port, nil
	})

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: helperBaseCommand(t),
		DefaultPort:             5199,
	})

	if err := ctx.DevBuild(); err != nil {
		t.Fatalf("DevBuild() error = %v", err)
	}
	t.Cleanup(ctx.Cleanup)

	const waiterCount = 8
	var waitGroup sync.WaitGroup
	waitGroup.Add(waiterCount)

	for i := 0; i < waiterCount; i++ {
		go func() {
			defer waitGroup.Done()
			ctx.Wait()
		}()
	}

	waitForCondition(t, 2*time.Second, func() bool {
		ctx.mu.Lock()
		defer ctx.mu.Unlock()
		return ctx.waitInProgress
	}, "wait did not enter waitInProgress state")

	ctx.Cleanup()

	allDone := make(chan struct{})
	go func() {
		waitGroup.Wait()
		close(allDone)
	}()

	select {
	case <-allDone:
	case <-time.After(2 * time.Second):
		t.Fatal("not all concurrent Wait() callers returned")
	}
}

func TestDevBuild_RestartsExistingProcess(t *testing.T) {
	t.Setenv(testHelperProcessEnv, "1")
	t.Setenv(testHelperModeEnv, "block_until_terminated")

	stubInitPort(t, func(port int) (int, error) {
		return port, nil
	})

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: helperBaseCommand(t),
		DefaultPort:             5199,
	})

	if err := ctx.DevBuild(); err != nil {
		t.Fatalf("first DevBuild() error = %v", err)
	}
	firstPID := processPID(t, ctx)

	if err := ctx.DevBuild(); err != nil {
		t.Fatalf("second DevBuild() error = %v", err)
	}
	t.Cleanup(ctx.Cleanup)

	secondPID := processPID(t, ctx)
	if firstPID == secondPID {
		t.Fatalf("expected restarted process PID to change, both were %d", firstPID)
	}
}

func TestDevBuild_RestartsAfterPriorProcessExited(t *testing.T) {
	t.Setenv(testHelperProcessEnv, "1")
	t.Setenv(testHelperModeEnv, "exit_immediately")

	stubInitPort(t, func(port int) (int, error) {
		return port, nil
	})

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: helperBaseCommand(t),
		DefaultPort:             5199,
	})

	if err := ctx.DevBuild(); err != nil {
		t.Fatalf("first DevBuild() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := ctx.DevBuild(); err != nil {
		t.Fatalf("second DevBuild() should restart even if prior process already exited, got %v", err)
	}
}

func TestProdBuild_WritesManifestAndLeavesParentEnvUnchanged(t *testing.T) {
	t.Setenv(testHelperProcessEnv, "1")
	t.Setenv(testHelperModeEnv, "prod_success")
	t.Setenv("ROLLDOWN_OPTIONS_VALIDATION", "strict")

	outDir := filepath.Join(t.TempDir(), "dist", "static", "assets", "public")
	manifestOut := filepath.Join(t.TempDir(), "manifest.json")

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: helperBaseCommand(t),
		OutDir:                  outDir,
		ManifestOut:             manifestOut,
	})

	if err := ctx.ProdBuild(); err != nil {
		t.Fatalf("ProdBuild() error = %v", err)
	}

	if _, err := os.Stat(manifestOut); err != nil {
		t.Fatalf("expected manifest file at output location: %v", err)
	}

	tempManifestPath := filepath.Join(outDir, "__temp_viteutil_manifest__.json")
	if _, err := os.Stat(tempManifestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temp manifest to be moved away, stat err = %v", err)
	}

	if got := os.Getenv("ROLLDOWN_OPTIONS_VALIDATION"); got != "strict" {
		t.Fatalf("expected parent env to remain unchanged, got %q", got)
	}
}

func TestProdBuild_CreatesMissingManifestOutputParentDirectory(t *testing.T) {
	t.Setenv(testHelperProcessEnv, "1")
	t.Setenv(testHelperModeEnv, "prod_success_require_absolute_outdir")

	outDir := filepath.Join(t.TempDir(), "dist", "static", "assets", "public")
	manifestOut := filepath.Join(
		t.TempDir(),
		"nested",
		"manifest",
		"dir",
		"manifest.json",
	)

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: helperBaseCommand(t),
		OutDir:                  outDir,
		ManifestOut:             manifestOut,
	})

	if err := ctx.ProdBuild(); err != nil {
		t.Fatalf("ProdBuild() error = %v", err)
	}

	if _, err := os.Stat(manifestOut); err != nil {
		t.Fatalf("expected manifest file at nested output location: %v", err)
	}
}

func TestProdBuild_ReturnsCommandError(t *testing.T) {
	t.Setenv(testHelperProcessEnv, "1")
	t.Setenv(testHelperModeEnv, "prod_fail")

	outDir := filepath.Join(t.TempDir(), "dist")
	manifestOut := filepath.Join(t.TempDir(), "manifest.json")

	ctx := NewBuildCtx(&BuildCtxOptions{
		JSPackageManagerBaseCmd: helperBaseCommand(t),
		OutDir:                  outDir,
		ManifestOut:             manifestOut,
	})

	if err := ctx.ProdBuild(); err == nil {
		t.Fatal("expected ProdBuild() to return command execution error")
	}
}

func TestWaitAndCleanup_NoProcessNoOp(t *testing.T) {
	ctx := NewBuildCtx(&BuildCtxOptions{})
	ctx.Wait()
	ctx.Cleanup()
}

func runProdBuildHelperAndExit(exitCode int) {
	if exitCode != 0 {
		os.Exit(exitCode)
	}

	outDir, manifestName, err := parseProdBuildArgs(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	manifestPath := filepath.Join(outDir, manifestName)
	if err := os.WriteFile(manifestPath, []byte(`{"index.html":{"file":"assets/index.js"}}`), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	os.Exit(0)
}

func runProdBuildHelperRequiringAbsoluteOutDirAndExit(exitCode int) {
	if exitCode != 0 {
		os.Exit(exitCode)
	}

	outDir, manifestName, err := parseProdBuildArgs(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if !filepath.IsAbs(outDir) {
		fmt.Fprintf(os.Stderr, "expected absolute --outDir, got %q\n", outDir)
		os.Exit(3)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	manifestPath := filepath.Join(outDir, manifestName)
	if err := os.WriteFile(manifestPath, []byte(`{"index.html":{"file":"assets/index.js"}}`), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	os.Exit(0)
}

func parseProdBuildArgs(args []string) (string, string, error) {
	var outDir string
	var manifestName string

	for i := 0; i < len(args)-1; i++ {
		switch args[i] {
		case "--outDir":
			outDir = args[i+1]
		case "--manifest":
			manifestName = args[i+1]
		}
	}

	if outDir == "" {
		return "", "", fmt.Errorf("missing --outDir argument")
	}
	if manifestName == "" {
		return "", "", fmt.Errorf("missing --manifest argument")
	}

	return outDir, manifestName, nil
}

func waitForTerminationSignalAndExit() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, helperTerminationSignals()...)
	defer signal.Stop(signalChan)

	select {
	case <-signalChan:
		os.Exit(0)
	case <-time.After(20 * time.Second):
		os.Exit(19)
	}
}

func helperTerminationSignals() []os.Signal {
	if runtime.GOOS == "windows" {
		return []os.Signal{os.Interrupt}
	}
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

func helperBaseCommand(t *testing.T) string {
	t.Helper()

	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	if strings.ContainsAny(executablePath, " \t\n") {
		t.Skip("test binary path contains whitespace; strings.Fields parser cannot represent this safely")
	}

	return executablePath + " -test.run=^TestBuildCtxHelperProcess$ --"
}

func stubInitPort(t *testing.T, stub func(port int) (int, error)) {
	t.Helper()

	original := initPort
	initPort = stub
	t.Cleanup(func() {
		initPort = original
	})
}

func waitForCondition(
	t *testing.T,
	timeout time.Duration,
	condition func() bool,
	failureMessage string,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal(failureMessage)
}

func assertArgPair(t *testing.T, args []string, key string, expectedValue string) {
	t.Helper()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key {
			if args[i+1] != expectedValue {
				t.Fatalf("expected %s=%s, got %s", key, expectedValue, args[i+1])
			}
			return
		}
	}
	t.Fatalf("expected command args to contain %s %s, args=%v", key, expectedValue, args)
}

func processPID(t *testing.T, ctx *BuildCtx) int {
	t.Helper()

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if ctx.cmd == nil || ctx.cmd.Process == nil {
		t.Fatal("expected command process to be present")
	}

	return ctx.cmd.Process.Pid
}
