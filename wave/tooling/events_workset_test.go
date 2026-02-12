package tooling

import (
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestWorkSetAddFromRefreshAction(t *testing.T) {
	work := &workSet{}
	work.addFromRefreshAction(wave.RefreshAction{
		TriggerRestart: true,
		RecompileGo:    true,
		ReloadBrowser:  true,
		WaitForApp:     true,
		WaitForVite:    true,
	})

	if !work.restartApp {
		t.Fatal("expected restartApp=true")
	}
	if !work.compileGo {
		t.Fatal("expected compileGo=true")
	}
	if !work.reloadBrowser || !work.waitForApp || !work.waitForVite {
		t.Fatalf(
			"expected reload/wait flags true, got reload=%v waitApp=%v waitVite=%v",
			work.reloadBrowser,
			work.waitForApp,
			work.waitForVite,
		)
	}
}

func TestWorkSetAddImplicitWork(t *testing.T) {
	t.Run("go files imply compile and restart", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{fileType: fileTypeGo})
		if !work.compileGo || !work.restartApp {
			t.Fatalf("expected compile+restart, got compile=%v restart=%v", work.compileGo, work.restartApp)
		}
	})

	t.Run("run-on-change-only suppresses implicit work", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{
			fileType:    fileTypeGo,
			watchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
		})
		if work.compileGo || work.restartApp {
			t.Fatalf("expected no implicit work, got compile=%v restart=%v", work.compileGo, work.restartApp)
		}
	})

	t.Run("other file with recompile implies restart", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypeOther,
			watchedFile: &wave.WatchedFile{
				RecompileGoBinary: true,
			},
		})
		if !work.compileGo || !work.restartApp {
			t.Fatalf("expected compile+restart, got compile=%v restart=%v", work.compileGo, work.restartApp)
		}
	})

	t.Run("revalidate preference is recorded", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypeOther,
			watchedFile: &wave.WatchedFile{
				OnlyRunClientDefinedRevalidateFunc: true,
			},
		})
		if !work.preferRevalidate {
			t.Fatal("expected preferRevalidate=true")
		}
	})
}

func TestWorkSetDetermineBrowserBehavior(t *testing.T) {
	t.Run("restart takes precedence", func(t *testing.T) {
		work := &workSet{restartApp: true}
		work.determineBrowserBehavior(true)

		if !work.reloadBrowser || !work.waitForApp || !work.waitForVite {
			t.Fatalf(
				"expected reload+wait for restart path, got reload=%v waitApp=%v waitVite=%v",
				work.reloadBrowser,
				work.waitForApp,
				work.waitForVite,
			)
		}
	})

	t.Run("revalidate preference overrides hot reload optimizations", func(t *testing.T) {
		work := &workSet{
			preferRevalidate: true,
			buildNormalCSS:   true,
		}
		work.determineBrowserBehavior(true)

		if !work.revalidate {
			t.Fatal("expected revalidate=true")
		}
		if work.hotReloadCSS {
			t.Fatal("expected hotReloadCSS=false when revalidate is preferred")
		}
	})

	t.Run("css-only work uses css hot reload", func(t *testing.T) {
		work := &workSet{buildCriticalCSS: true}
		work.determineBrowserBehavior(true)
		if !work.hotReloadCSS {
			t.Fatal("expected hotReloadCSS=true")
		}
		if work.reloadBrowser {
			t.Fatal("expected reloadBrowser=false for css-only path")
		}
	})

	t.Run("public static work invalidates vite", func(t *testing.T) {
		work := &workSet{processPublicFiles: true}
		work.determineBrowserBehavior(true)
		if !work.invalidateVite {
			t.Fatal("expected invalidateVite=true")
		}
		if work.reloadBrowser {
			t.Fatal("expected reloadBrowser=false on invalidate-vite path")
		}
	})

	t.Run("private static work uses full reload", func(t *testing.T) {
		work := &workSet{processPrivateFiles: true}
		work.determineBrowserBehavior(false)

		if !work.reloadBrowser || !work.waitForApp {
			t.Fatalf("expected reload+waitApp for private static path, got reload=%v waitApp=%v", work.reloadBrowser, work.waitForApp)
		}
		if work.waitForVite {
			t.Fatal("expected waitForVite=false when vite is disabled")
		}
	})
}

func TestWorkSetResolve_CompileImpliesRestart(t *testing.T) {
	work := &workSet{compileGo: true}
	work.resolve(false)
	if !work.restartApp {
		t.Fatal("expected restartApp=true when compileGo=true")
	}
	if !work.reloadBrowser {
		t.Fatal("expected reloadBrowser=true on restart path")
	}
}
