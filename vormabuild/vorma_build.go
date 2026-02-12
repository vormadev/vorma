package vormabuild

import (
	"flag"
	"fmt"
	"log"

	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
	wavebuild "github.com/vormadev/vorma/wave/tooling"
)

// Build parses flags and runs the build or dev server.
func Build(v *vormaruntime.Vorma) {
	dev := flag.Bool("dev", false, "run in development mode")
	hook := flag.Bool("hook", false, "run build hook only (internal use)")
	noBinary := flag.Bool("no-binary", false, "skip go binary compilation")
	flag.Parse()

	if *hook {
		configureBuildEnvironment(v)
		if err := runBuildHook(v, *dev); err != nil {
			log.Fatalf("build hook failed: %v", err)
		}
		if !*dev {
			if err := runProdHookPostProcessing(v); err != nil {
				log.Fatalf("%v", err)
			}
		}
		return
	}

	if err := build(v, *dev, *noBinary); err != nil {
		log.Fatalf("build failed: %v", err)
	}
}

// build performs a full Vorma build.
func build(v *vormaruntime.Vorma, isDev bool, noBinary bool) error {
	configureBuildEnvironment(v)

	if isDev {
		return runDevBuild(v)
	}

	return runProductionBuild(v, noBinary)
}

func runBuildHook(v *vormaruntime.Vorma, isDev bool) error {
	return buildInner(v, &buildInnerOptions{isDev: isDev})
}

func runProdHookPostProcessing(v *vormaruntime.Vorma) error {
	builder := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
	defer builder.Close()

	if err := builder.ViteProdBuild(); err != nil {
		return fmt.Errorf("Vite production build failed: %w", err)
	}

	if err := postViteProdBuild(v); err != nil {
		return fmt.Errorf("post Vite production build failed: %w", err)
	}

	return nil
}

func runDevBuild(v *vormaruntime.Vorma) error {
	wave.SetModeToDev()
	// Set isDev on this process's Vorma instance so callbacks
	// (like rebuildRoutesOnly) can run in the dev server process.
	v.SetIsDev(true)
	return wavebuild.RunDev(v.Wave.GetParsedConfig(), v.Wave.Logger())
}

func runProductionBuild(v *vormaruntime.Vorma, noBinary bool) error {
	// Production Build
	//
	// The build flow is:
	// 1. wb.Build() sets up dist directory, then runs ProdBuildHook
	// 2. ProdBuildHook (e.g., "go run ./cmd/build --hook") runs in subprocess:
	//    a. buildInner() - parses routes, writes artifacts
	//    b. ViteProdBuild() - runs Vite
	//    c. postViteProdBuild() - processes Vite output
	// 3. wb.Build() compiles the Go binary
	//
	// All Vorma logic runs in the subprocess (via --hook) so state is shared.
	wb := wavebuild.NewBuilder(v.Wave.GetParsedConfig(), v.Wave.Logger())
	defer wb.Close()

	return wb.Build(wavebuild.BuildOpts{
		CompileGo: !noBinary,
		IsDev:     false,
		IsRebuild: false,
	})
}
