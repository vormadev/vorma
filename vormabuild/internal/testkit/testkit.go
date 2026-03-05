// Package testkit provides shared test fixtures and helpers for vormabuild
// packages so test setup semantics stay consistent across package boundaries.
package testkit

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/waveartifacts"
)

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
	Config              *vormaruntime.VormaConfigJSON
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
	t.Chdir(rootDir)
	distDirectoryName := ".wavedist"
	distDir := filepath.Join(rootDir, distDirectoryName)
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

	cfg := vormaruntime.VormaConfigJSON{
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
		Core struct {
			ProjectID       string `json:"ProjectID"`
			MainAppEntry    string `json:"MainAppEntry"`
			StaticAssetDirs struct {
				Private string `json:"Private"`
				Public  string `json:"Public"`
			} `json:"StaticAssetDirs"`
			PublicPathPrefix string `json:"PublicPathPrefix"`
		} `json:"Core"`
		Vorma vormaruntime.VormaConfigJSON `json:"Vorma"`
	}{Vorma: cfg}
	rawConfig.Core.ProjectID = "vormabuild-testkit-fixture"
	rawConfig.Core.MainAppEntry = "backend/cmd/serve"
	rawConfig.Core.StaticAssetDirs.Private = filepath.ToSlash(
		filepath.Join(
			distDirectoryName,
			"static",
			waveartifacts.AssetsDirname,
			waveartifacts.PrivateDirname,
		),
	)
	rawConfig.Core.StaticAssetDirs.Public = filepath.ToSlash(
		filepath.Join(
			distDirectoryName,
			"static",
			waveartifacts.AssetsDirname,
			waveartifacts.PublicDirname,
		),
	)
	rawConfig.Core.PublicPathPrefix = "/"
	if options != nil && options.WaveMainAppEntry != nil {
		rawConfig.Core.MainAppEntry = *options.WaveMainAppEntry
	}

	cfgJSON, err := json.Marshal(rawConfig)
	if err != nil {
		t.Fatalf("marshal test config: %v", err)
	}
	MustWriteFile(t, "wave.config.json", cfgJSON)

	w := wave.New(wave.Config{
		FS:         os.DirFS(rootDir),
		ConfigPath: "wave.config.json",
		Logger:     TestLogger(),
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

// MustMutateAppVormaConfig reparses Vorma config from Wave raw JSON after
// applying one mutation callback, then installs the parsed config on the app.
func MustMutateAppVormaConfig(
	t *testing.T,
	app *vormaruntime.Vorma,
	mutate func(*vormaruntime.VormaConfigJSON),
) {
	t.Helper()
	if app == nil {
		t.Fatal("app is required")
	}
	if app.Wave == nil {
		t.Fatal("app.Wave is required")
	}

	rawConfig := struct {
		Vorma vormaruntime.VormaConfigJSON `json:"Vorma"`
	}{}
	if unmarshalError := json.Unmarshal(
		app.Wave.RawConfigJSON(),
		&rawConfig,
	); unmarshalError != nil {
		t.Fatalf("parse Wave raw config JSON for Vorma mutation: %v", unmarshalError)
	}
	if mutate != nil {
		mutate(&rawConfig.Vorma)
	}

	mutatedVormaConfigPayload, marshalError := json.Marshal(rawConfig)
	if marshalError != nil {
		t.Fatalf("marshal mutated Vorma config payload: %v", marshalError)
	}
	parsedConfig, parseError := runtimeconfig.ParseVormaConfigJSON(
		mutatedVormaConfigPayload,
		app.Wave.ParsedConfig(),
	)
	if parseError != nil {
		t.Fatalf("parse mutated Vorma config payload: %v", parseError)
	}
	if parsedConfig == nil {
		t.Fatal("parsed mutated Vorma config payload returned nil config")
	}
	app.Config = parsedConfig
}

// MustParseVormaConfigJSONForTest parses one raw Vorma config fixture through
// the real parser using a temporary Wave parsed config root.
func MustParseVormaConfigJSONForTest(
	tb testing.TB,
	rawVormaConfig vormaruntime.VormaConfigJSON,
) vormaruntime.VormaConfig {
	tb.Helper()

	currentWorkingDirectory, getwdError := os.Getwd()
	if getwdError != nil {
		tb.Fatalf("resolve current working directory for Vorma config parse fixture: %v", getwdError)
	}
	return MustParseVormaConfigJSONForRootForTest(tb, currentWorkingDirectory, rawVormaConfig)
}

// MustParseVormaConfigJSONForRootForTest parses one raw Vorma config fixture
// through the real parser using the provided effective root.
func MustParseVormaConfigJSONForRootForTest(
	tb testing.TB,
	rootDir string,
	rawVormaConfig vormaruntime.VormaConfigJSON,
) vormaruntime.VormaConfig {
	tb.Helper()

	parsedWaveConfig := wavetest.NewParsedConfigAtRoot(tb, rootDir)
	rawVormaPayload, marshalError := json.Marshal(
		struct {
			Vorma vormaruntime.VormaConfigJSON `json:"Vorma"`
		}{
			Vorma: rawVormaConfig,
		},
	)
	if marshalError != nil {
		tb.Fatalf("marshal raw Vorma config fixture: %v", marshalError)
	}
	parsedVormaConfig, parseError := runtimeconfig.ParseVormaConfigJSON(
		rawVormaPayload,
		parsedWaveConfig,
	)
	if parseError != nil {
		tb.Fatalf("parse raw Vorma config fixture: %v", parseError)
	}
	return parsedVormaConfig
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

	_, currentSourceFilePath, currentSourceFileLine, currentSourceFileOK := runtime.Caller(0)
	if !currentSourceFileOK {
		t.Fatalf("resolve current source file path for repository-root lookup")
	}

	repositoryRootDir := filepath.Dir(currentSourceFilePath)
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
		"resolve repository root from %q:%d: go.mod not found in ancestor directories",
		currentSourceFilePath,
		currentSourceFileLine,
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
