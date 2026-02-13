package tooling

import (
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
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

	if !work.restart.restartApp {
		t.Fatal("expected restart.restartApp=true")
	}
	if !work.build.compileGo {
		t.Fatal("expected build.compileGo=true")
	}
	if work.browser.action != browserPhaseActionHardReload ||
		!work.browser.waitForApp ||
		!work.browser.waitForVite {
		t.Fatalf(
			"expected hard reload + wait flags, got action=%v waitApp=%v waitVite=%v",
			work.browser.action,
			work.browser.waitForApp,
			work.browser.waitForVite,
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
		if work.browser.action != browserPhaseActionHardReload {
			t.Fatal("expected first non-restart action to be applied")
		}
		if work.browser.waitForApp {
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
		if work.browser.action != browserPhaseActionHardReload ||
			!work.browser.waitForApp ||
			!work.browser.waitForVite {
			t.Fatalf(
				"expected hard reload + wait flags, got action=%v waitApp=%v waitVite=%v",
				work.browser.action,
				work.browser.waitForApp,
				work.browser.waitForVite,
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
		if !work.build.compileGo || !work.restart.restartApp {
			t.Fatalf(
				"expected compile+restart, got compile=%v restart=%v",
				work.build.compileGo,
				work.restart.restartApp,
			)
		}
	})

	t.Run("run-on-change-only suppresses implicit work", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{
			fileType:    fileTypeGo,
			watchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
		})
		if work.build.compileGo || work.restart.restartApp {
			t.Fatalf(
				"expected no implicit work, got compile=%v restart=%v",
				work.build.compileGo,
				work.restart.restartApp,
			)
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
		if !work.build.compileGo || !work.restart.restartApp {
			t.Fatalf(
				"expected compile+restart, got compile=%v restart=%v",
				work.build.compileGo,
				work.restart.restartApp,
			)
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
		if !work.build.buildCriticalCSS {
			t.Fatal("expected build.buildCriticalCSS=true")
		}
		if !work.restart.restartApp {
			t.Fatal("expected restart.restartApp=true when critical css watched file requests hard reload")
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
		if !work.build.buildNormalCSS {
			t.Fatal("expected build.buildNormalCSS=true")
		}
		if !work.restart.restartApp {
			t.Fatal("expected restart.restartApp=true when normal css watched file requests hard reload")
		}
	})

	t.Run("shared critical+normal css file requests both rebuilds and can request hard reload", func(t *testing.T) {
		work := &workSet{}
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypeCriticalAndNormalCSS,
			watchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		})
		if !work.build.buildCriticalCSS {
			t.Fatal("expected build.buildCriticalCSS=true")
		}
		if !work.build.buildNormalCSS {
			t.Fatal("expected build.buildNormalCSS=true")
		}
		if !work.restart.restartApp {
			t.Fatal("expected restart.restartApp=true when shared css watched file requests hard reload")
		}
	})

	t.Run("public static file requests public file processing", func(t *testing.T) {
		work := &workSet{}
		changedPublicFilePath := "/tmp/public/logo.svg"
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypePublicStatic,
			event: fsnotify.Event{
				Name: changedPublicFilePath,
			},
		})
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypePublicStatic,
			event: fsnotify.Event{
				Name: changedPublicFilePath,
			},
		})
		if !work.build.processPublicFiles {
			t.Fatal("expected build.processPublicFiles=true")
		}
		if len(work.build.publicStaticChangedFilePaths) != 1 {
			t.Fatalf("expected one deduplicated public static path, got %#v", work.build.publicStaticChangedFilePaths)
		}
		if work.build.publicStaticChangedFilePaths[0] != changedPublicFilePath {
			t.Fatalf(
				"expected tracked public static path %q, got %#v",
				changedPublicFilePath,
				work.build.publicStaticChangedFilePaths,
			)
		}
	})

	t.Run("public static changed paths are normalized and deduplicated by location", func(t *testing.T) {
		work := &workSet{}

		root := t.TempDir()
		canonicalFilePath := filepath.Join(root, "static", "public", "logo.svg")
		equivalentFilePath := filepath.Join(root, "static", "public", ".", "logo.svg")
		expectedNormalizedPath := pathnorm.Absolute(canonicalFilePath)

		work.addImplicitWork(classifiedEvent{
			fileType: fileTypePublicStatic,
			event: fsnotify.Event{
				Name: canonicalFilePath,
			},
		})
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypePublicStatic,
			event: fsnotify.Event{
				Name: equivalentFilePath,
			},
		})

		if len(work.build.publicStaticChangedFilePaths) != 1 {
			t.Fatalf(
				"expected one normalized public static path, got %#v",
				work.build.publicStaticChangedFilePaths,
			)
		}
		if work.build.publicStaticChangedFilePaths[0] != expectedNormalizedPath {
			t.Fatalf(
				"expected normalized public static path %q, got %#v",
				expectedNormalizedPath,
				work.build.publicStaticChangedFilePaths,
			)
		}
	})

	t.Run("private static file requests private file processing", func(t *testing.T) {
		work := &workSet{}
		firstPrivatePath := "/tmp/private/a.txt"
		secondPrivatePath := "/tmp/private/b.txt"
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypePrivateStatic,
			event: fsnotify.Event{
				Name: firstPrivatePath,
			},
		})
		work.addImplicitWork(classifiedEvent{
			fileType: fileTypePrivateStatic,
			event: fsnotify.Event{
				Name: secondPrivatePath,
			},
		})
		if !work.build.processPrivateFiles {
			t.Fatal("expected build.processPrivateFiles=true")
		}
		if len(work.build.privateStaticChangedFilePaths) != 2 {
			t.Fatalf("expected two tracked private static paths, got %#v", work.build.privateStaticChangedFilePaths)
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
		if work.build.compileGo {
			t.Fatal("did not expect build.compileGo=true")
		}
		if !work.restart.restartApp {
			t.Fatal("expected restart.restartApp=true")
		}
	})
}

func TestWorkSetDetermineBrowserBehavior(t *testing.T) {
	t.Run("restart takes precedence", func(t *testing.T) {
		work := &workSet{
			restart: restartPhaseDecision{restartApp: true},
		}
		work.determineBrowserBehavior(true)

		if work.browser.action != browserPhaseActionHardReload ||
			!work.browser.waitForApp ||
			!work.browser.waitForVite {
			t.Fatalf(
				"expected hard reload + wait for restart path, got action=%v waitApp=%v waitVite=%v",
				work.browser.action,
				work.browser.waitForApp,
				work.browser.waitForVite,
			)
		}
	})

	t.Run("revalidate preference overrides hot reload optimizations", func(t *testing.T) {
		work := &workSet{
			preferRevalidate: true,
			build: buildPhaseDecision{
				buildNormalCSS: true,
			},
		}
		work.determineBrowserBehavior(true)

		if work.browser.action != browserPhaseActionRevalidate {
			t.Fatalf("expected revalidate action, got %v", work.browser.action)
		}
	})

	t.Run("css-only work uses css hot reload", func(t *testing.T) {
		work := &workSet{
			build: buildPhaseDecision{
				buildCriticalCSS: true,
			},
		}
		work.determineBrowserBehavior(true)
		if work.browser.action != browserPhaseActionHotReloadCSS {
			t.Fatalf("expected hot reload css action, got %v", work.browser.action)
		}
	})

	t.Run("public static work invalidates vite", func(t *testing.T) {
		work := &workSet{
			build: buildPhaseDecision{
				processPublicFiles: true,
			},
		}
		work.determineBrowserBehavior(true)
		if work.browser.action != browserPhaseActionInvalidateVite {
			t.Fatalf("expected invalidate vite action, got %v", work.browser.action)
		}
	})

	t.Run("private static work uses full reload", func(t *testing.T) {
		work := &workSet{
			build: buildPhaseDecision{
				processPrivateFiles: true,
			},
		}
		work.determineBrowserBehavior(false)

		if work.browser.action != browserPhaseActionHardReload ||
			!work.browser.waitForApp {
			t.Fatalf(
				"expected hard reload + waitApp for private static path, got action=%v waitApp=%v",
				work.browser.action,
				work.browser.waitForApp,
			)
		}
		if work.browser.waitForVite {
			t.Fatal("expected waitForVite=false when vite is disabled")
		}
	})
}

func TestWorkSetResolve_CompileImpliesRestart(t *testing.T) {
	work := &workSet{
		build: buildPhaseDecision{
			compileGo: true,
		},
	}

	work.resolve(false)
	if !work.restart.restartApp {
		t.Fatal("expected restart.restartApp=true when build.compileGo=true")
	}
	if work.browser.action != browserPhaseActionHardReload {
		t.Fatal("expected hard reload action on restart path")
	}
}
