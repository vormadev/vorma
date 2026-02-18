package vormabuild

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildRuntimeRegistration_E2EMultiPackageDiscoveredRoutes(
	t *testing.T,
) {
	repoRootDir := mustResolveRepositoryRootDir(t)
	fixtureRootDir := t.TempDir()

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "go.mod"),
		[]byte(fmt.Sprintf(`
module e2eapp

go 1.24

require github.com/vormadev/vorma v0.0.0

replace github.com/vormadev/vorma => %s
`, filepath.ToSlash(repoRootDir))),
	)

	mustWriteFile(t, filepath.Join(fixtureRootDir, "backend/wave.go"), []byte(`
package backend

import (
	"os"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
)

var waveFS = os.DirFS(".")

var Wave = wave.New(wave.Config{
	WaveConfigJSON: fsutil.MustReadFile(waveFS, "backend/wave.config.json"),
	DistStaticFS:   fsutil.MustSub(waveFS, "backend", "dist", "static"),
})
`))

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "backend/wave.config.json"),
		[]byte(`
{
	"Core": {
		"MainAppEntry": "backend/cmd/check",
		"DistDir": "backend/dist",
		"StaticAssetDirs": {
			"Private": "backend/assets/private",
			"Public": "backend/assets/public"
		},
		"PublicPathPrefix": "/",
		"ServerOnlyMode": true
	},
	"Vorma": {
		"MainBuildEntry": "backend/cmd/build",
		"UIVariant": "react",
		"HTMLTemplateLocation": "entry.go.html",
		"ClientEntry": "frontend/src/vorma.entry.tsx",
		"ClientRouteDefinitionPatterns": ["frontend/src/**/*vorma.routes.ts"],
		"ServerRouteDefinitionPatterns": ["backend/src/routes/**/*.go"],
		"TSGenOutDir": "frontend/src/vorma.gen",
		"BuildtimePublicURLFuncName": "waveBuildtimeURL"
	}
}
`),
	)

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "backend/assets/private/entry.go.html"),
		[]byte("<!doctype html><html><body></body></html>"),
	)
	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "frontend/src/components/root.tsx"),
		[]byte("export const Root = () => null;"),
	)
	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "frontend/src/vorma.routes.ts"),
		[]byte(`
import { route } from "vorma/buildtime";

route("/", import("./components/root.tsx"), "Root");
`),
	)

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "backend/src/app/app.go"),
		[]byte(`
package app

import (
	"e2eapp/backend"

	"github.com/vormadev/vorma"
)

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: backend.Wave,
})
`),
	)

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "backend/src/routes/register.go"),
		[]byte(`
package routes

import (
	_ "e2eapp/backend/src/routes/accounts"
	_ "e2eapp/backend/src/routes/users"
)
`),
	)

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "backend/src/routes/users/routes.go"),
		[]byte(`
package users

import (
	"e2eapp/backend/src/app"

	"github.com/vormadev/vorma"
)

type LoaderCtx struct {
	*vorma.LoaderReqData
}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: rd}
}

func DefineLoaderForRegistration[O any](
	pattern string,
	loader func(*LoaderCtx) (O, error),
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(
		app.App,
		pattern,
		loader,
		decorateLoaderCtx,
	)
}

var _ = DefineLoaderForRegistration("/users", func(*LoaderCtx) (string, error) {
	return "users", nil
})
`),
	)

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "backend/src/routes/accounts/routes.go"),
		[]byte(`
package accounts

import (
	"e2eapp/backend/src/app"

	"github.com/vormadev/vorma"
)

type LoaderCtx struct {
	*vorma.LoaderReqData
}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: rd}
}

func newLoader(
	pattern string,
	loader func(*LoaderCtx) (string, error),
) *vorma.Loader[string] {
	return vorma.DefineLoaderForRegistration(
		app.App,
		pattern,
		loader,
		decorateLoaderCtx,
	)
}

func registerAccountsRoute() any {
	_ = newLoader("/accounts", func(*LoaderCtx) (string, error) {
		return "accounts", nil
	})
	return nil
}

var _ = registerAccountsRoute()
`),
	)

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "backend/cmd/build/main.go"),
		[]byte(`
package main

import (
	"context"

	"e2eapp/backend"
	"e2eapp/backend/src/app"
	_ "e2eapp/backend/src/routes"

	"github.com/vormadev/vorma/vormabuild"
)

func main() {
	backend.Wave.SetFrameworkRunBuildHookRunner(func(context.Context, bool) error {
		return nil
	})
	vormabuild.Build(app.App)
}
`),
	)

	mustWriteFile(
		t,
		filepath.Join(fixtureRootDir, "backend/cmd/check/main.go"),
		[]byte(`
package main

import (
	"fmt"
	"os"

	"e2eapp/backend/src/app"
	_ "e2eapp/backend/src/routes"
)

func main() {
	if !app.App.HasRegisteredLoaderTask("/users") {
		fmt.Println("missing /users loader registration")
		os.Exit(1)
	}
	if !app.App.HasRegisteredLoaderTask("/accounts") {
		fmt.Println("missing /accounts loader registration")
		os.Exit(1)
	}
	fmt.Println("registered")
}
`),
	)

	buildOutput, buildErr := runCommandAndCaptureOutput(
		fixtureRootDir,
		"go",
		"run",
		"-mod=mod",
		"./backend/cmd/build",
	)
	if buildErr != nil {
		t.Fatalf("build command failed: %v\n%s", buildErr, buildOutput)
	}

	compiledBinaryPath := filepath.Join(fixtureRootDir, "backend/dist/main")
	if runtime.GOOS == "windows" {
		compiledBinaryPath += ".exe"
	}
	runtimeCheckOutput, runtimeCheckErr := runCommandAndCaptureOutput(
		fixtureRootDir,
		compiledBinaryPath,
	)
	if runtimeCheckErr != nil {
		t.Fatalf(
			"compiled runtime check failed: %v\n%s",
			runtimeCheckErr,
			runtimeCheckOutput,
		)
	}
	if !strings.Contains(runtimeCheckOutput, "registered") {
		t.Fatalf(
			"runtime check output = %q, expected \"registered\"",
			runtimeCheckOutput,
		)
	}
}

func mustResolveRepositoryRootDir(t *testing.T) string {
	t.Helper()

	currentWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve current working directory: %v", err)
	}
	repositoryRootDir := filepath.Clean(filepath.Join(currentWorkingDir, ".."))
	if _, err := os.Stat(filepath.Join(repositoryRootDir, "go.mod")); err != nil {
		t.Fatalf("resolve repository root from %q: %v", currentWorkingDir, err)
	}
	return repositoryRootDir
}

func runCommandAndCaptureOutput(
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
