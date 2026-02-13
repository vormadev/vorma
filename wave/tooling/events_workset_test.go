package tooling

import (
	"path/filepath"
	"reflect"
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

func TestDeriveRefreshActionWorkMutationDecision(t *testing.T) {
	testCases := []struct {
		name                 string
		action               wave.RefreshAction
		expectedWorkMutation refreshActionWorkMutationDecision
	}{
		{
			name: "restart action sets restart and compile-go when requested",
			action: wave.RefreshAction{
				TriggerRestart: true,
				RecompileGo:    true,
			},
			expectedWorkMutation: refreshActionWorkMutationDecision{
				restartApp: true,
				compileGo:  true,
			},
		},
		{
			name: "reload action requests hard reload",
			action: wave.RefreshAction{
				ReloadBrowser: true,
			},
			expectedWorkMutation: refreshActionWorkMutationDecision{
				requestBrowserAction: true,
				browserAction:        browserPhaseActionHardReload,
			},
		},
		{
			name: "wait flags are carried",
			action: wave.RefreshAction{
				WaitForApp:  true,
				WaitForVite: true,
			},
			expectedWorkMutation: refreshActionWorkMutationDecision{
				waitForApp:  true,
				waitForVite: true,
			},
		},
		{
			name:                 "zero action maps to zero mutation",
			action:               wave.RefreshAction{},
			expectedWorkMutation: refreshActionWorkMutationDecision{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			workMutationDecision := deriveRefreshActionWorkMutationDecision(testCase.action)
			if !reflect.DeepEqual(workMutationDecision, testCase.expectedWorkMutation) {
				t.Fatalf(
					"deriveRefreshActionWorkMutationDecision()=%#v, want %#v",
					workMutationDecision,
					testCase.expectedWorkMutation,
				)
			}
		})
	}
}

func TestWorkSetApplyRefreshActionWorkMutationDecision(t *testing.T) {
	work := &workSet{
		browser: browserPhaseDecision{
			action:      browserPhaseActionRevalidate,
			waitForApp:  false,
			waitForVite: false,
		},
	}

	work.applyRefreshActionWorkMutationDecision(refreshActionWorkMutationDecision{
		restartApp:           true,
		compileGo:            true,
		requestBrowserAction: true,
		browserAction:        browserPhaseActionHardReload,
		waitForApp:           true,
		waitForVite:          true,
	})
	work.applyRefreshActionWorkMutationDecision(refreshActionWorkMutationDecision{})

	if !work.restart.restartApp {
		t.Fatal("expected restart.restartApp=true")
	}
	if !work.build.compileGo {
		t.Fatal("expected build.compileGo=true")
	}
	if work.browser.action != browserPhaseActionHardReload {
		t.Fatalf("expected browser hard reload action, got %v", work.browser.action)
	}
	if !work.browser.waitForApp || !work.browser.waitForVite {
		t.Fatalf(
			"expected wait flags true, got waitApp=%v waitVite=%v",
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

	t.Run("keeps actions before restart and surfaces restart recompile signal", func(t *testing.T) {
		work := &workSet{}

		result := work.applyRefreshActions([]wave.RefreshAction{
			{WaitForApp: true},
			{ReloadBrowser: true},
			{TriggerRestart: true, RecompileGo: true},
			{WaitForVite: true},
		})

		if !result.restartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if !result.recompileGo {
			t.Fatal("expected recompileGo=true from first restart action")
		}
		if work.browser.action != browserPhaseActionHardReload {
			t.Fatalf("expected pre-restart reload action to be applied, got %v", work.browser.action)
		}
		if !work.browser.waitForApp {
			t.Fatal("expected pre-restart wait-for-app to be applied")
		}
		if work.browser.waitForVite {
			t.Fatal("expected post-restart actions not to be applied")
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

		reductionDecision := reduceRefreshActionsInStableOrder(input)
		if reductionDecision.applicationResult.restartRequested {
			t.Fatal("expected restartRequested=false")
		}
		if reductionDecision.restartActionEncountered {
			t.Fatal("expected restartActionEncountered=false")
		}
		if reductionDecision.restartActionIndex != -1 {
			t.Fatalf("expected restartActionIndex=-1, got %d", reductionDecision.restartActionIndex)
		}
		applied := reductionDecision.actionsBeforeRestart
		if len(applied) != len(input) {
			t.Fatalf("actionsBeforeRestart count=%d, want %d", len(applied), len(input))
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

		reductionDecision := reduceRefreshActionsInStableOrder(input)
		if !reductionDecision.applicationResult.restartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if reductionDecision.applicationResult.recompileGo {
			t.Fatal("expected first restart action to determine recompileGo=false")
		}
		if !reductionDecision.restartActionEncountered {
			t.Fatal("expected restartActionEncountered=true")
		}
		if reductionDecision.restartActionIndex != 1 {
			t.Fatalf("expected restartActionIndex=1, got %d", reductionDecision.restartActionIndex)
		}
		applied := reductionDecision.actionsBeforeRestart
		if len(applied) != 1 {
			t.Fatalf("actionsBeforeRestart count=%d, want 1", len(applied))
		}
		if !applied[0].ReloadBrowser {
			t.Fatalf("unexpected applied actions: %#v", applied)
		}
	})

	t.Run("restart at first action yields no pre-restart actions", func(t *testing.T) {
		reductionDecision := reduceRefreshActionsInStableOrder([]wave.RefreshAction{
			{TriggerRestart: true, RecompileGo: true},
			{ReloadBrowser: true},
		})
		if !reductionDecision.restartActionEncountered {
			t.Fatal("expected restartActionEncountered=true")
		}
		if reductionDecision.restartActionIndex != 0 {
			t.Fatalf("expected restartActionIndex=0, got %d", reductionDecision.restartActionIndex)
		}
		if !reductionDecision.applicationResult.restartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if !reductionDecision.applicationResult.recompileGo {
			t.Fatal("expected recompileGo=true from first restart action")
		}
		if len(reductionDecision.actionsBeforeRestart) != 0 {
			t.Fatalf("expected zero pre-restart actions, got %#v", reductionDecision.actionsBeforeRestart)
		}
	})
}

func TestDeriveImplicitWorkDecisionForClassifiedEvent(t *testing.T) {
	testCases := []struct {
		name             string
		classifiedEvent  classifiedEvent
		expectedDecision implicitWorkDecision
	}{
		{
			name:            "go file compiles and restarts",
			classifiedEvent: classifiedEvent{fileType: fileTypeGo},
			expectedDecision: implicitWorkDecision{
				compileGo:  true,
				restartApp: true,
			},
		},
		{
			name: "run-on-change-only disables implicit work",
			classifiedEvent: classifiedEvent{
				fileType:    fileTypeGo,
				watchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			expectedDecision: implicitWorkDecision{},
		},
		{
			name: "revalidate preference is recorded",
			classifiedEvent: classifiedEvent{
				fileType: fileTypeOther,
				watchedFile: &wave.WatchedFile{
					OnlyRunClientDefinedRevalidateFunc: true,
				},
			},
			expectedDecision: implicitWorkDecision{
				preferRevalidate: true,
			},
		},
		{
			name: "critical css can request hard reload",
			classifiedEvent: classifiedEvent{
				fileType:    fileTypeCriticalCSS,
				watchedFile: &wave.WatchedFile{RestartApp: true},
			},
			expectedDecision: implicitWorkDecision{
				buildCriticalCSS: true,
				restartApp:       true,
			},
		},
		{
			name: "normal css without hard reload only schedules css rebuild",
			classifiedEvent: classifiedEvent{
				fileType:    fileTypeNormalCSS,
				watchedFile: &wave.WatchedFile{},
			},
			expectedDecision: implicitWorkDecision{
				buildNormalCSS: true,
			},
		},
		{
			name: "shared css with hard-reload request rebuilds both",
			classifiedEvent: classifiedEvent{
				fileType:    fileTypeCriticalAndNormalCSS,
				watchedFile: &wave.WatchedFile{RecompileGoBinary: true},
			},
			expectedDecision: implicitWorkDecision{
				buildCriticalCSS: true,
				buildNormalCSS:   true,
				restartApp:       true,
			},
		},
		{
			name: "public static processing tracks changed file path",
			classifiedEvent: classifiedEvent{
				fileType: fileTypePublicStatic,
				event:    fsnotify.Event{Name: "/tmp/public/logo.svg"},
			},
			expectedDecision: implicitWorkDecision{
				processPublicFiles:          true,
				publicStaticChangedFilePath: "/tmp/public/logo.svg",
			},
		},
		{
			name: "private static processing tracks changed file path",
			classifiedEvent: classifiedEvent{
				fileType: fileTypePrivateStatic,
				event:    fsnotify.Event{Name: "/tmp/private/home.html"},
			},
			expectedDecision: implicitWorkDecision{
				processPrivateFiles:          true,
				privateStaticChangedFilePath: "/tmp/private/home.html",
			},
		},
		{
			name: "other watched file recompile implies restart",
			classifiedEvent: classifiedEvent{
				fileType: fileTypeOther,
				watchedFile: &wave.WatchedFile{
					RecompileGoBinary: true,
				},
			},
			expectedDecision: implicitWorkDecision{
				compileGo:  true,
				restartApp: true,
			},
		},
		{
			name: "other watched file restart without recompile",
			classifiedEvent: classifiedEvent{
				fileType: fileTypeOther,
				watchedFile: &wave.WatchedFile{
					RestartApp: true,
				},
			},
			expectedDecision: implicitWorkDecision{
				restartApp: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := deriveImplicitWorkDecisionForClassifiedEvent(testCase.classifiedEvent)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf(
					"deriveImplicitWorkDecisionForClassifiedEvent()=%#v, want %#v",
					decision,
					testCase.expectedDecision,
				)
			}
		})
	}
}

func TestWorkSetApplyImplicitWorkDecision_MergesBuildRestartAndPathWork(t *testing.T) {
	work := &workSet{}

	firstDecision := implicitWorkDecision{
		compileGo:                   true,
		buildCriticalCSS:            true,
		processPublicFiles:          true,
		publicStaticChangedFilePath: "/tmp/public/logo.svg",
	}
	secondDecision := implicitWorkDecision{
		restartApp:                   true,
		preferRevalidate:             true,
		processPublicFiles:           true,
		publicStaticChangedFilePath:  "/tmp/public/./logo.svg",
		processPrivateFiles:          true,
		privateStaticChangedFilePath: "/tmp/private/home.html",
	}

	work.applyImplicitWorkDecision(firstDecision)
	work.applyImplicitWorkDecision(secondDecision)

	if !work.build.compileGo {
		t.Fatal("expected build.compileGo=true after merged implicit work decisions")
	}
	if !work.build.buildCriticalCSS {
		t.Fatal("expected build.buildCriticalCSS=true after merged implicit work decisions")
	}
	if !work.build.processPublicFiles {
		t.Fatal("expected build.processPublicFiles=true after merged implicit work decisions")
	}
	if !work.build.processPrivateFiles {
		t.Fatal("expected build.processPrivateFiles=true after merged implicit work decisions")
	}
	if !work.restart.restartApp {
		t.Fatal("expected restart.restartApp=true after merged implicit work decisions")
	}
	if !work.preferRevalidate {
		t.Fatal("expected preferRevalidate=true after merged implicit work decisions")
	}

	expectedPublicPaths := []string{pathnorm.Absolute("/tmp/public/logo.svg")}
	if !reflect.DeepEqual(work.build.publicStaticChangedFilePaths, expectedPublicPaths) {
		t.Fatalf("public static changed paths=%v, want %v", work.build.publicStaticChangedFilePaths, expectedPublicPaths)
	}

	expectedPrivatePaths := []string{pathnorm.Absolute("/tmp/private/home.html")}
	if !reflect.DeepEqual(work.build.privateStaticChangedFilePaths, expectedPrivatePaths) {
		t.Fatalf("private static changed paths=%v, want %v", work.build.privateStaticChangedFilePaths, expectedPrivatePaths)
	}
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

func TestDeriveBrowserPhaseResolutionForWorkSet(t *testing.T) {
	testCases := []struct {
		name               string
		buildDecision      buildPhaseDecision
		restartDecision    restartPhaseDecision
		preferRevalidate   bool
		usingVite          bool
		expectedResolution browserPhaseResolution
	}{
		{
			name:             "restart takes precedence over revalidate preference",
			restartDecision:  restartPhaseDecision{restartApp: true},
			preferRevalidate: true,
			usingVite:        true,
			expectedResolution: browserPhaseResolution{
				action:         browserPhaseActionHardReload,
				applyWaitFlags: true,
				waitForApp:     true,
				waitForVite:    true,
			},
		},
		{
			name:             "revalidate takes precedence over css-only optimization",
			buildDecision:    buildPhaseDecision{buildNormalCSS: true},
			preferRevalidate: true,
			usingVite:        false,
			expectedResolution: browserPhaseResolution{
				action:         browserPhaseActionRevalidate,
				applyWaitFlags: true,
				waitForApp:     true,
				waitForVite:    false,
			},
		},
		{
			name:          "css-only work uses hot reload css",
			buildDecision: buildPhaseDecision{buildCriticalCSS: true},
			expectedResolution: browserPhaseResolution{
				action: browserPhaseActionHotReloadCSS,
			},
		},
		{
			name:          "public static work invalidates vite",
			buildDecision: buildPhaseDecision{processPublicFiles: true},
			expectedResolution: browserPhaseResolution{
				action: browserPhaseActionInvalidateVite,
			},
		},
		{
			name: "public static invalidation takes precedence over private static full reload",
			buildDecision: buildPhaseDecision{
				processPublicFiles:  true,
				processPrivateFiles: true,
			},
			expectedResolution: browserPhaseResolution{
				action: browserPhaseActionInvalidateVite,
			},
		},
		{
			name:          "private static work uses hard reload",
			buildDecision: buildPhaseDecision{processPrivateFiles: true},
			usingVite:     true,
			expectedResolution: browserPhaseResolution{
				action:         browserPhaseActionHardReload,
				applyWaitFlags: true,
				waitForApp:     true,
				waitForVite:    true,
			},
		},
		{
			name: "css and non-public static work uses hard reload",
			buildDecision: buildPhaseDecision{
				buildNormalCSS:      true,
				processPrivateFiles: true,
			},
			usingVite: true,
			expectedResolution: browserPhaseResolution{
				action:         browserPhaseActionHardReload,
				applyWaitFlags: true,
				waitForApp:     true,
				waitForVite:    true,
			},
		},
		{
			name: "no build work produces no browser action",
			expectedResolution: browserPhaseResolution{
				action: browserPhaseActionNone,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolution := deriveBrowserPhaseResolutionForWorkSet(
				testCase.buildDecision,
				testCase.restartDecision,
				testCase.preferRevalidate,
				testCase.usingVite,
			)
			if !reflect.DeepEqual(resolution, testCase.expectedResolution) {
				t.Fatalf(
					"deriveBrowserPhaseResolutionForWorkSet()=%#v, want %#v",
					resolution,
					testCase.expectedResolution,
				)
			}
		})
	}
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
