package tooling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

const (
	flexibilityContractViteHelperProcessEnv = "WAVE_TOOLING_FLEXIBILITY_CONTRACT_VITE_HELPER_PROCESS"
	flexibilityContractViteHelperArgsEnv    = "WAVE_TOOLING_FLEXIBILITY_CONTRACT_VITE_HELPER_ARGS_FILE"
	flexibilityContractViteHelperCwdEnv     = "WAVE_TOOLING_FLEXIBILITY_CONTRACT_VITE_HELPER_CWD_FILE"
)

// TestWaveToolingFlexibilityContractViteHelperProcess runs in a subprocess and
// records CWD/args so tests can assert user-provided Vite command options are
// honored.
func TestWaveToolingFlexibilityContractViteHelperProcess(t *testing.T) {
	if os.Getenv(flexibilityContractViteHelperProcessEnv) != "1" {
		return
	}

	if err := runWaveToolingFlexibilityContractViteHelperProcess(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestFlexibilityContract_WatchExcludeDirsExcludesDirectoryTree(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Watch.Exclude.Dirs = []string{"generated"}

	generatedDir := filepath.Join(root, "generated")
	generatedNestedDir := filepath.Join(generatedDir, "nested")
	allowedDir := filepath.Join(root, "app")
	if err := os.MkdirAll(generatedNestedDir, 0o755); err != nil {
		t.Fatalf("failed creating excluded test directory: %v", err)
	}
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatalf("failed creating allowed test directory: %v", err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	if err := watcher.AddDir(root); err != nil {
		t.Fatalf("AddDir returned error: %v", err)
	}

	if !watcher.IsIgnoredDir(generatedDir) {
		t.Fatalf("expected %q to be ignored by Watch.Exclude.Dirs", generatedDir)
	}
	if !watcher.IsIgnoredDir(generatedNestedDir) {
		t.Fatalf("expected nested directory %q to be ignored by Watch.Exclude.Dirs", generatedNestedDir)
	}

	if _, ok := watcher.watchedDirs.Load(watcher.norm(generatedDir)); ok {
		t.Fatalf("did not expect excluded directory to be watched: %q", generatedDir)
	}
	if _, ok := watcher.watchedDirs.Load(watcher.norm(generatedNestedDir)); ok {
		t.Fatalf("did not expect nested excluded directory to be watched: %q", generatedNestedDir)
	}

	if watcher.IsIgnoredDir(allowedDir) {
		t.Fatalf("did not expect non-excluded directory to be ignored: %q", allowedDir)
	}
	if _, ok := watcher.watchedDirs.Load(watcher.norm(allowedDir)); !ok {
		t.Fatalf("expected non-excluded directory to be watched: %q", allowedDir)
	}
}

func TestFlexibilityContract_AbsoluteWatchIncludePatternMatches(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	absolutePattern := filepath.Join(root, "custom-watch", "**", "*.txt")
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         absolutePattern,
			RunOnChangeOnly: true,
		},
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	absoluteWatchedFile := filepath.Join(root, "custom-watch", "nested", "notes.txt")
	if err := os.MkdirAll(filepath.Dir(absoluteWatchedFile), 0o755); err != nil {
		t.Fatalf("failed creating watched file parent dir: %v", err)
	}
	if err := os.WriteFile(absoluteWatchedFile, []byte("notes"), 0o644); err != nil {
		t.Fatalf("failed writing watched file: %v", err)
	}

	matchedWatchedFile := watcher.FindWatchedFile(absoluteWatchedFile)
	if matchedWatchedFile == nil {
		t.Fatal("expected absolute Watch.Include pattern to match watched file")
	}
	if !matchedWatchedFile.RunOnChangeOnly {
		t.Fatalf("expected matched watched file to preserve RunOnChangeOnly=true, got %#v", matchedWatchedFile)
	}
}

func TestFlexibilityContract_ConfigMutationsTriggerConfigRestart(t *testing.T) {
	runConfigMutationAndPathShapeMatrix(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCase,
			pathShapeCaseForRun configEventPathShapeCase,
		) {
			cfg, _, configFilePath := setupConfigEventTestConfig(t)
			if configMutationCaseForRun.prepareEvent != nil {
				configMutationCaseForRun.prepareEvent(t, configFilePath)
			}

			s := setupProcessEventsServerForToolingTests(t, cfg)

			configEventPath := pathShapeCaseForRun.buildPath(t, configFilePath)
			s.processEvents([]fsnotify.Event{{
				Name: configEventPath,
				Op:   configMutationCaseForRun.op,
			}})

			select {
			case req := <-s.restartCh:
				if !req.isConfigRestart || !req.recompileGo {
					t.Fatalf(
						"expected config restart with Go recompile on config %s/%s, got %#v",
						configMutationCaseForRun.name,
						pathShapeCaseForRun.name,
						req,
					)
				}
			default:
				t.Fatalf(
					"expected config %s/%s to trigger config restart request",
					configMutationCaseForRun.name,
					pathShapeCaseForRun.name,
				)
			}
		},
	)
}

func TestFlexibilityContract_ViteDevBuildHonorsCmdDirAndConfigFile(t *testing.T) {
	t.Setenv(flexibilityContractViteHelperProcessEnv, "1")

	root := t.TempDir()
	argsFilePath := filepath.Join(root, "helper.args")
	cwdFilePath := filepath.Join(root, "helper.cwd")
	t.Setenv(flexibilityContractViteHelperArgsEnv, argsFilePath)
	t.Setenv(flexibilityContractViteHelperCwdEnv, cwdFilePath)

	commandWorkingDirectory := filepath.Join(root, "frontend")
	if err := os.MkdirAll(commandWorkingDirectory, 0o755); err != nil {
		t.Fatalf("failed creating Vite command directory: %v", err)
	}

	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: flexibilityContractViteHelperBaseCommand(t),
		JSPackageManagerCmdDir:  commandWorkingDirectory,
		ViteConfigFile:          "./vite.custom.config.ts",
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	viteContext, err := builder.NewViteDevContext()
	if err != nil {
		t.Fatalf("NewViteDevContext returned error: %v", err)
	}
	if viteContext == nil {
		t.Fatal("expected non-nil Vite context")
	}
	defer viteContext.Cleanup()
	viteContext.Wait()

	helperCwd, err := readTrimmedFile(cwdFilePath)
	if err != nil {
		t.Fatalf("failed reading helper cwd: %v", err)
	}
	expectedHelperCwd := filepath.Clean(commandWorkingDirectory)
	if helperCwd != expectedHelperCwd {
		t.Fatalf("expected Vite command to run from %q, got %q", expectedHelperCwd, helperCwd)
	}

	helperArgs, err := readHelperArgs(argsFilePath)
	if err != nil {
		t.Fatalf("failed reading helper args: %v", err)
	}

	if len(helperArgs) < 1 {
		t.Fatalf("expected helper args to include vite invocation, got %v", helperArgs)
	}
	if helperArgs[0] != "vite" {
		t.Fatalf("expected vite invocation, got args %v", helperArgs)
	}

	configArgValue, ok := helperFlagValue(helperArgs, "--config")
	if !ok {
		t.Fatalf("expected helper args to include --config flag, got %v", helperArgs)
	}
	if configArgValue != "./vite.custom.config.ts" {
		t.Fatalf("expected --config value %q, got %q", "./vite.custom.config.ts", configArgValue)
	}
}

func runWaveToolingFlexibilityContractViteHelperProcess() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.WriteFile(os.Getenv(flexibilityContractViteHelperCwdEnv), []byte(cwd), 0o644); err != nil {
		return err
	}

	helperArgs, err := helperArgsAfterDoubleDash(os.Args)
	if err != nil {
		return err
	}

	if err := os.WriteFile(
		os.Getenv(flexibilityContractViteHelperArgsEnv),
		[]byte(strings.Join(helperArgs, "\n")),
		0o644,
	); err != nil {
		return err
	}

	outDir, hasOutDir := helperFlagValue(helperArgs, "--outDir")
	manifestName, hasManifestName := helperFlagValue(helperArgs, "--manifest")
	if hasOutDir {
		resolvedOutDir := outDir
		if !filepath.IsAbs(resolvedOutDir) {
			resolvedOutDir = filepath.Join(cwd, resolvedOutDir)
		}
		if err := os.MkdirAll(resolvedOutDir, 0o755); err != nil {
			return err
		}

		manifestFileName := "__temp_viteutil_manifest__.json"
		if hasManifestName && manifestName != "" {
			manifestFileName = manifestName
		}

		manifestPath := filepath.Join(resolvedOutDir, manifestFileName)
		if err := os.WriteFile(manifestPath, []byte("{}"), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func flexibilityContractViteHelperBaseCommand(t *testing.T) string {
	t.Helper()

	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error: %v", err)
	}

	if strings.ContainsAny(executablePath, " \t\n") {
		t.Skip("test binary path contains whitespace; strings.Fields parser cannot represent this safely")
	}

	return executablePath + " -test.run=^TestWaveToolingFlexibilityContractViteHelperProcess$ --"
}

func helperArgsAfterDoubleDash(args []string) ([]string, error) {
	for i, arg := range args {
		if arg == "--" {
			return append([]string(nil), args[i+1:]...), nil
		}
	}
	return nil, fmt.Errorf("helper process missing -- argument separator")
}

func readTrimmedFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readHelperArgs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

func helperFlagValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}
