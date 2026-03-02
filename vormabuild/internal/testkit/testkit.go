// Package testkit provides shared test fixtures and helpers for vormabuild
// packages so test setup semantics stay consistent across package boundaries.
package testkit

import (
	"bytes"
	"encoding/json"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave"
)

type staticAssetDirsForTests = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

// BuildTestFixture captures common runtime + filesystem fixture state used by
// vormabuild package tests.
type BuildTestFixture struct {
	App        *vormaruntime.Vorma
	RootDir    string
	DistDir    string
	StaticDir  string
	PrivateDir string
	PublicDir  string
}

// BuildTestFixtureOptions customizes NewBuildTestFixture behavior.
type BuildTestFixtureOptions struct {
	Config              *vormaruntime.VormaConfig
	LoadersRouterConfig vormaruntime.LoadersRouterOptions
	ActionsRouterConfig vormaruntime.ActionsRouterOptions
	WaveMainAppEntry    *string
}

// NewBuildTestFixture constructs a consistent vormabuild test fixture runtime.
func NewBuildTestFixture(
	t *testing.T,
	options *BuildTestFixtureOptions,
) *BuildTestFixture {
	t.Helper()

	rootDir := wavetest.NewWorkspaceTempDir(t, "vormabuild-fixture-")
	distDir := filepath.Join(rootDir, "dist")
	staticDir := filepath.Join(distDir, "static")
	privateDir := filepath.Join(
		staticDir,
		waveartifacts.AssetsDirname,
		waveartifacts.PrivateDirname,
	)
	publicDir := filepath.Join(
		staticDir,
		waveartifacts.AssetsDirname,
		waveartifacts.PublicDirname,
	)

	MustMkdirAll(t, privateDir)
	MustMkdirAll(t, publicDir)
	MustMkdirAll(t, filepath.Join(privateDir, runtimepaths.VormaInternalDirname))

	cfg := vormaruntime.VormaConfig{
		MainBuildEntry:       "backend/cmd/build",
		UIVariant:            string(vormaruntime.UIVariantReact),
		HTMLTemplateLocation: "entry.go.html",
		ClientEntry:          "frontend/src/vorma.entry.tsx",
		ClientRouteDefinitionPatterns: []string{
			"frontend/src/**/*vorma.routes.ts",
		},
		TSGenOutDir:                "frontend/src/vorma.gen",
		BuildtimePublicURLFuncName: "waveBuildtimeURL",
	}
	if options != nil && options.Config != nil {
		cfg = *options.Config
	}

	MustWriteFile(
		t,
		filepath.Join(privateDir, cfg.HTMLTemplateLocation),
		[]byte("<!doctype html><html><body></body></html>"),
	)

	rawConfig := struct {
		Core  waveconfig.CoreConfig    `json:"Core"`
		Vorma vormaruntime.VormaConfig `json:"Vorma"`
	}{
		Core: waveconfig.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      wavetest.MustCWDRelativePath(distDir),
			StaticAssetDirs: staticAssetDirsForTests{
				Private: wavetest.MustCWDRelativePath(privateDir),
				Public:  wavetest.MustCWDRelativePath(publicDir),
			},
			PublicPathPrefix: "/",
		},
		Vorma: cfg,
	}
	if options != nil && options.WaveMainAppEntry != nil {
		rawConfig.Core.MainAppEntry = *options.WaveMainAppEntry
	}

	cfgJSON, err := json.Marshal(rawConfig)
	if err != nil {
		t.Fatalf("marshal test config: %v", err)
	}

	w := wave.New(wave.Config{
		WaveConfigJSON: cfgJSON,
		DistStaticFS:   os.DirFS(staticDir),
		Logger:         TestLogger(),
	})

	var loadersOpts vormaruntime.LoadersRouterOptions
	var actionsOpts vormaruntime.ActionsRouterOptions
	if options != nil {
		loadersOpts = options.LoadersRouterConfig
		actionsOpts = options.ActionsRouterConfig
	}

	app := vormaruntime.NewVormaApp(vormaruntime.VormaAppConfig{
		Wave:                 w,
		LoadersRouterOptions: loadersOpts,
		ActionsRouterOptions: actionsOpts,
		Logger:               TestLogger(),
	})

	return &BuildTestFixture{
		App:        app,
		RootDir:    rootDir,
		DistDir:    distDir,
		StaticDir:  staticDir,
		PrivateDir: privateDir,
		PublicDir:  publicDir,
	}
}

// TestLogger returns a discard-backed logger suitable for deterministic tests.
func TestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// MustMkdirAll creates directories for tests and fails the calling test on
// error.
func MustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// MustWriteFile writes a file for tests and fails the calling test on error.
func MustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// MustWriteJSONFile marshals then writes JSON test data.
func MustWriteJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	MustWriteFile(t, path, bytes)
}

// MustResolveRepositoryRootDir resolves the workspace repository root from the
// current process working directory.
func MustResolveRepositoryRootDir(t *testing.T) string {
	t.Helper()

	currentWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve current working directory: %v", err)
	}

	repositoryRootDir := currentWorkingDir
	for {
		if _, statError := os.Stat(
			filepath.Join(repositoryRootDir, "go.mod"),
		); statError == nil {
			return repositoryRootDir
		}

		parentDir := filepath.Dir(repositoryRootDir)
		if parentDir == repositoryRootDir {
			break
		}
		repositoryRootDir = parentDir
	}

	t.Fatalf(
		"resolve repository root from %q: go.mod not found in ancestor directories",
		currentWorkingDir,
	)
	return ""
}

// RunCommandAndCaptureOutput executes a command and returns merged stdout/stderr
// text.
func RunCommandAndCaptureOutput(
	commandDir string,
	commandBinary string,
	commandArgs ...string,
) (string, error) {
	command := exec.Command(commandBinary, commandArgs...)
	command.Dir = commandDir

	var combinedOutput bytes.Buffer
	command.Stdout = &combinedOutput
	command.Stderr = &combinedOutput

	commandErr := command.Run()
	return combinedOutput.String(), commandErr
}

// AssertRuntimeWriteLockCanBeAcquiredPromptly guards that the runtime write
// lock is not leaked by the tested operation.
func AssertRuntimeWriteLockCanBeAcquiredPromptly(
	t *testing.T,
	v *vormaruntime.Vorma,
	stepName string,
) {
	t.Helper()

	lockAcquired := make(chan struct{})
	go func() {
		v.WithLock(func(*vormaruntime.LockedVorma) {})
		close(lockAcquired)
	}()

	select {
	case <-lockAcquired:
		return
	case <-time.After(time.Second):
		t.Fatalf("%s appears to run while runtime write lock is held", stepName)
	}
}

// WriteBootstrapStyleRoutesFixtureFiles writes a representative route fixture
// set used by route-artifact and build-inner tests.
func WriteBootstrapStyleRoutesFixtureFiles(t *testing.T) {
	t.Helper()

	MustWriteFile(
		t,
		"frontend/src/components/root.tsx",
		[]byte("export const Root = () => null;"),
	)
	MustWriteFile(
		t,
		"frontend/src/components/home.tsx",
		[]byte("export const Home = () => null;"),
	)
	MustWriteFile(
		t,
		"frontend/src/components/links.tsx",
		[]byte("export const Links = () => null;"),
	)

	MustWriteFile(t, "frontend/src/vorma.routes.ts", []byte(`
import { route } from "vorma/buildtime";

route("/", import("./components/root.tsx"), "Root");
route("/_index", import("./components/home.tsx"), "Home");
route("/links", import("./components/links.tsx"), "Links");
`))
}
