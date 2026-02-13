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

func TestWorkSetApplyRefreshActions(t *testing.T) {
	t.Run("returns restart request and stops processing remaining actions", func(t *testing.T) {
		work := &workSet{}

		result := work.applyRefreshActions([]wave.RefreshAction{
			{ReloadBrowser: true},
			{TriggerRestart: true},
			{WaitForApp: true},
		})

		if !result.restartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if result.recompileGo {
			t.Fatal("expected recompileGo=false")
		}
		if !work.reloadBrowser {
			t.Fatal("expected first non-restart action to be applied")
		}
		if work.waitForApp {
			t.Fatal("expected actions after restart request not to be applied")
		}
	})

	t.Run("merges non-restart actions into workset", func(t *testing.T) {
		work := &workSet{}

		result := work.applyRefreshActions([]wave.RefreshAction{
			{ReloadBrowser: true, WaitForApp: true},
			{WaitForVite: true},
		})

		if result.restartRequested {
			t.Fatal("expected restartRequested=false")
		}
		if !work.reloadBrowser || !work.waitForApp || !work.waitForVite {
			t.Fatalf(
				"expected merged flags true, got reload=%v waitApp=%v waitVite=%v",
				work.reloadBrowser,
				work.waitForApp,
				work.waitForVite,
			)
		}
	})
}

func TestReduceRefreshActionsInStableOrder(t *testing.T) {
	t.Run("keeps non-restart actions in order", func(t *testing.T) {
		input := []wave.RefreshAction{
			{ReloadBrowser: true},
			{WaitForApp: true},
			{WaitForVite: true},
		}

		applied, result := reduceRefreshActionsInStableOrder(input)
		if result.restartRequested {
			t.Fatal("expected restartRequested=false")
		}
		if len(applied) != len(input) {
			t.Fatalf("applied action count=%d, want %d", len(applied), len(input))
		}
		for i := range input {
			if applied[i] != input[i] {
				t.Fatalf("applied[%d]=%#v, want %#v", i, applied[i], input[i])
			}
		}
	})

	t.Run("returns first restart action and excludes later actions", func(t *testing.T) {
		input := []wave.RefreshAction{
			{ReloadBrowser: true},
			{TriggerRestart: true, RecompileGo: false},
			{TriggerRestart: true, RecompileGo: true},
			{WaitForApp: true},
		}

		applied, result := reduceRefreshActionsInStableOrder(input)
		if !result.restartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if result.recompileGo {
			t.Fatal("expected first restart action to determine recompileGo=false")
		}
		if len(applied) != 1 {
			t.Fatalf("applied action count=%d, want 1", len(applied))
		}
		if !applied[0].ReloadBrowser {
			t.Fatalf("unexpected applied actions: %#v", applied)
		}
	})
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

	t.Run("critical css file requests css rebuild and can request hard reload", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypeCriticalCSS,
			watchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		})
		if !work.buildCriticalCSS {
			t.Fatal("expected buildCriticalCSS=true")
		}
		if !work.restartApp {
			t.Fatal("expected restartApp=true when critical css watched file requests hard reload")
		}
	})

	t.Run("normal css file requests css rebuild and can request hard reload", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypeNormalCSS,
			watchedFile: &wave.WatchedFile{
				RecompileGoBinary: true,
			},
		})
		if !work.buildNormalCSS {
			t.Fatal("expected buildNormalCSS=true")
		}
		if !work.restartApp {
			t.Fatal("expected restartApp=true when normal css watched file requests hard reload")
		}
	})

	t.Run("public static file requests public file processing", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{fileType: fileTypePublicStatic})
		if !work.processPublicFiles {
			t.Fatal("expected processPublicFiles=true")
		}
	})

	t.Run("private static file requests private file processing", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{fileType: fileTypePrivateStatic})
		if !work.processPrivateFiles {
			t.Fatal("expected processPrivateFiles=true")
		}
	})

	t.Run("other watched file can request restart without go compile", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypeOther,
			watchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		})
		if work.compileGo {
			t.Fatal("did not expect compileGo=true")
		}
		if !work.restartApp {
			t.Fatal("expected restartApp=true")
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
