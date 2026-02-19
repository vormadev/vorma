package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
)

func TestWorkSetAddFromRefreshAction(t *testing.T) {
	work := &devserver.WorkSet{}
	work.AddFromRefreshAction(wave.RefreshAction{
		TriggerRestart: true,
		RecompileGo:    true,
		ReloadBrowser:  true,
		WaitForApp:     true,
		WaitForVite:    true,
	})

	if !work.Restart.RestartApp {
		t.Fatal("expected restart.restartApp=true")
	}
	if !work.Build.CompileGo {
		t.Fatal("expected build.compileGo=true")
	}
	if work.Browser.Action != devserver.BrowserPhaseActionHardReload ||
		!work.Browser.WaitForApp ||
		!work.Browser.WaitForVite {
		t.Fatalf(
			"expected hard reload + wait flags, got action=%v waitApp=%v waitVite=%v",
			work.Browser.Action,
			work.Browser.WaitForApp,
			work.Browser.WaitForVite,
		)
	}
}

func TestDeriveRefreshActionWorkMutationDecision(t *testing.T) {
	testCases := []struct {
		Name                 string
		Action               wave.RefreshAction
		ExpectedWorkMutation devserver.RefreshActionWorkMutationDecision
	}{
		{
			Name: "restart action sets restart and compile-go when requested",
			Action: wave.RefreshAction{
				TriggerRestart: true,
				RecompileGo:    true,
			},
			ExpectedWorkMutation: devserver.RefreshActionWorkMutationDecision{
				RestartApp: true,
				CompileGo:  true,
			},
		},
		{
			Name: "reload action requests hard reload",
			Action: wave.RefreshAction{
				ReloadBrowser: true,
			},
			ExpectedWorkMutation: devserver.RefreshActionWorkMutationDecision{
				RequestBrowserAction: true,
				BrowserAction:        devserver.BrowserPhaseActionHardReload,
			},
		},
		{
			Name: "wait flags are carried",
			Action: wave.RefreshAction{
				WaitForApp:  true,
				WaitForVite: true,
			},
			ExpectedWorkMutation: devserver.RefreshActionWorkMutationDecision{
				WaitForApp:  true,
				WaitForVite: true,
			},
		},
		{
			Name:                 "zero action maps to zero mutation",
			Action:               wave.RefreshAction{},
			ExpectedWorkMutation: devserver.RefreshActionWorkMutationDecision{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			workMutationDecision := devserver.DeriveRefreshActionWorkMutationDecision(testCase.Action)
			if !reflect.DeepEqual(workMutationDecision, testCase.ExpectedWorkMutation) {
				t.Fatalf(
					"devserver.DeriveRefreshActionWorkMutationDecision()=%#v, want %#v",
					workMutationDecision,
					testCase.ExpectedWorkMutation,
				)
			}
		})
	}
}

func TestWorkSetApplyRefreshActionWorkMutationDecision(t *testing.T) {
	work := &devserver.WorkSet{
		Browser: devserver.BrowserPhaseDecision{
			Action:      devserver.BrowserPhaseActionRevalidate,
			WaitForApp:  false,
			WaitForVite: false,
		},
	}

	work.ApplyRefreshActionWorkMutationDecision(devserver.RefreshActionWorkMutationDecision{
		RestartApp:           true,
		CompileGo:            true,
		RequestBrowserAction: true,
		BrowserAction:        devserver.BrowserPhaseActionHardReload,
		WaitForApp:           true,
		WaitForVite:          true,
	})
	work.ApplyRefreshActionWorkMutationDecision(devserver.RefreshActionWorkMutationDecision{})

	if !work.Restart.RestartApp {
		t.Fatal("expected restart.restartApp=true")
	}
	if !work.Build.CompileGo {
		t.Fatal("expected build.compileGo=true")
	}
	if work.Browser.Action != devserver.BrowserPhaseActionHardReload {
		t.Fatalf("expected browser hard reload action, got %v", work.Browser.Action)
	}
	if !work.Browser.WaitForApp || !work.Browser.WaitForVite {
		t.Fatalf(
			"expected wait flags true, got waitApp=%v waitVite=%v",
			work.Browser.WaitForApp,
			work.Browser.WaitForVite,
		)
	}
}

func TestWorkSetApplyRefreshActions(t *testing.T) {
	t.Run("returns restart request and stops processing remaining actions", func(t *testing.T) {
		work := &devserver.WorkSet{}

		result := work.ApplyRefreshActions([]wave.RefreshAction{
			{ReloadBrowser: true},
			{TriggerRestart: true},
			{WaitForApp: true},
		})

		if !result.RestartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if result.RecompileGo {
			t.Fatal("expected recompileGo=false")
		}
		if work.Browser.Action != devserver.BrowserPhaseActionHardReload {
			t.Fatal("expected first non-restart action to be applied")
		}
		if work.Browser.WaitForApp {
			t.Fatal("expected actions after restart request not to be applied")
		}
	})

	t.Run("keeps actions before restart and surfaces restart recompile signal", func(t *testing.T) {
		work := &devserver.WorkSet{}

		result := work.ApplyRefreshActions([]wave.RefreshAction{
			{WaitForApp: true},
			{ReloadBrowser: true},
			{TriggerRestart: true, RecompileGo: true},
			{WaitForVite: true},
		})

		if !result.RestartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if !result.RecompileGo {
			t.Fatal("expected recompileGo=true from first restart action")
		}
		if work.Browser.Action != devserver.BrowserPhaseActionHardReload {
			t.Fatalf("expected pre-restart reload action to be applied, got %v", work.Browser.Action)
		}
		if !work.Browser.WaitForApp {
			t.Fatal("expected pre-restart wait-for-app to be applied")
		}
		if work.Browser.WaitForVite {
			t.Fatal("expected post-restart actions not to be applied")
		}
	})

	t.Run("merges non-restart actions into workset", func(t *testing.T) {
		work := &devserver.WorkSet{}

		result := work.ApplyRefreshActions([]wave.RefreshAction{
			{ReloadBrowser: true, WaitForApp: true},
			{WaitForVite: true},
		})

		if result.RestartRequested {
			t.Fatal("expected restartRequested=false")
		}
		if work.Browser.Action != devserver.BrowserPhaseActionHardReload ||
			!work.Browser.WaitForApp ||
			!work.Browser.WaitForVite {
			t.Fatalf(
				"expected hard reload + wait flags, got action=%v waitApp=%v waitVite=%v",
				work.Browser.Action,
				work.Browser.WaitForApp,
				work.Browser.WaitForVite,
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

		reductionDecision := devserver.ReduceRefreshActionsInStableOrder(input)
		if reductionDecision.ApplicationResult.RestartRequested {
			t.Fatal("expected restartRequested=false")
		}
		if reductionDecision.RestartActionEncountered {
			t.Fatal("expected restartActionEncountered=false")
		}
		if reductionDecision.RestartActionIndex != -1 {
			t.Fatalf("expected restartActionIndex=-1, got %d", reductionDecision.RestartActionIndex)
		}
		applied := reductionDecision.ActionsBeforeRestart
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

		reductionDecision := devserver.ReduceRefreshActionsInStableOrder(input)
		if !reductionDecision.ApplicationResult.RestartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if reductionDecision.ApplicationResult.RecompileGo {
			t.Fatal("expected first restart action to determine recompileGo=false")
		}
		if !reductionDecision.RestartActionEncountered {
			t.Fatal("expected restartActionEncountered=true")
		}
		if reductionDecision.RestartActionIndex != 1 {
			t.Fatalf("expected restartActionIndex=1, got %d", reductionDecision.RestartActionIndex)
		}
		applied := reductionDecision.ActionsBeforeRestart
		if len(applied) != 1 {
			t.Fatalf("actionsBeforeRestart count=%d, want 1", len(applied))
		}
		if !applied[0].ReloadBrowser {
			t.Fatalf("unexpected applied Actions: %#v", applied)
		}
	})

	t.Run("restart at first action yields no pre-restart actions", func(t *testing.T) {
		reductionDecision := devserver.ReduceRefreshActionsInStableOrder([]wave.RefreshAction{
			{TriggerRestart: true, RecompileGo: true},
			{ReloadBrowser: true},
		})
		if !reductionDecision.RestartActionEncountered {
			t.Fatal("expected restartActionEncountered=true")
		}
		if reductionDecision.RestartActionIndex != 0 {
			t.Fatalf("expected restartActionIndex=0, got %d", reductionDecision.RestartActionIndex)
		}
		if !reductionDecision.ApplicationResult.RestartRequested {
			t.Fatal("expected restartRequested=true")
		}
		if !reductionDecision.ApplicationResult.RecompileGo {
			t.Fatal("expected recompileGo=true from first restart action")
		}
		if len(reductionDecision.ActionsBeforeRestart) != 0 {
			t.Fatalf("expected zero pre-restart actions, got %#v", reductionDecision.ActionsBeforeRestart)
		}
	})
}

func TestDeriveImplicitWorkDecisionForClassifiedEvent(t *testing.T) {
	testCases := []struct {
		Name             string
		ClassifiedEvent  devserver.ClassifiedEvent
		ExpectedDecision devserver.ImplicitWorkDecision
	}{
		{
			Name:            "go file compiles and restarts",
			ClassifiedEvent: devserver.ClassifiedEvent{FileType: devserver.FileTypeGo},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				CompileGo:  true,
				RestartApp: true,
			},
		},
		{
			Name: "run-on-change-only disables implicit work",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType:    devserver.FileTypeGo,
				WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{},
		},
		{
			Name: "revalidate preference is recorded",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType: devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					OnlyRunClientDefinedRevalidateFunc: true,
				},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				PreferRevalidate: true,
			},
		},
		{
			Name: "critical css can request hard reload",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType:    devserver.FileTypeCriticalCSS,
				WatchedFile: &wave.WatchedFile{RestartApp: true},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				BuildCriticalCSS: true,
				RestartApp:       true,
			},
		},
		{
			Name: "normal css without hard reload only schedules css rebuild",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType:    devserver.FileTypeNormalCSS,
				WatchedFile: &wave.WatchedFile{},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				BuildNormalCSS: true,
			},
		},
		{
			Name: "shared css with hard-reload request rebuilds both",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType:    devserver.FileTypeCriticalAndNormalCSS,
				WatchedFile: &wave.WatchedFile{RecompileGoBinary: true},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				BuildCriticalCSS: true,
				BuildNormalCSS:   true,
				RestartApp:       true,
			},
		},
		{
			Name: "public static processing tracks changed file path",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType: devserver.FileTypePublicStatic,
				Event:    fsnotify.Event{Name: "/tmp/public/logo.svg"},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				ProcessPublicFiles:          true,
				PublicStaticChangedFilePath: "/tmp/public/logo.svg",
			},
		},
		{
			Name: "private static processing tracks changed file path",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType: devserver.FileTypePrivateStatic,
				Event:    fsnotify.Event{Name: "/tmp/private/home.html"},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				ProcessPrivateFiles:          true,
				PrivateStaticChangedFilePath: "/tmp/private/home.html",
			},
		},
		{
			Name: "other watched file recompile implies restart",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType: devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					RecompileGoBinary: true,
				},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				CompileGo:  true,
				RestartApp: true,
			},
		},
		{
			Name: "other watched file restart without recompile",
			ClassifiedEvent: devserver.ClassifiedEvent{
				FileType: devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					RestartApp: true,
				},
			},
			ExpectedDecision: devserver.ImplicitWorkDecision{
				RestartApp: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := devserver.DeriveImplicitWorkDecisionForClassifiedEvent(testCase.ClassifiedEvent)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"devserver.DeriveImplicitWorkDecisionForClassifiedEvent()=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestWorkSetApplyImplicitWorkDecision_MergesBuildRestartAndPathWork(t *testing.T) {
	work := &devserver.WorkSet{}

	firstDecision := devserver.ImplicitWorkDecision{
		CompileGo:                   true,
		BuildCriticalCSS:            true,
		ProcessPublicFiles:          true,
		PublicStaticChangedFilePath: "/tmp/public/logo.svg",
	}
	secondDecision := devserver.ImplicitWorkDecision{
		RestartApp:                   true,
		PreferRevalidate:             true,
		ProcessPublicFiles:           true,
		PublicStaticChangedFilePath:  "/tmp/public/./logo.svg",
		ProcessPrivateFiles:          true,
		PrivateStaticChangedFilePath: "/tmp/private/home.html",
	}

	work.ApplyImplicitWorkDecision(firstDecision)
	work.ApplyImplicitWorkDecision(secondDecision)

	if !work.Build.CompileGo {
		t.Fatal("expected build.compileGo=true after merged implicit work decisions")
	}
	if !work.Build.BuildCriticalCSS {
		t.Fatal("expected build.buildCriticalCSS=true after merged implicit work decisions")
	}
	if !work.Build.ProcessPublicFiles {
		t.Fatal("expected build.processPublicFiles=true after merged implicit work decisions")
	}
	if !work.Build.ProcessPrivateFiles {
		t.Fatal("expected build.processPrivateFiles=true after merged implicit work decisions")
	}
	if !work.Restart.RestartApp {
		t.Fatal("expected restart.restartApp=true after merged implicit work decisions")
	}
	if !work.PreferRevalidate {
		t.Fatal("expected preferRevalidate=true after merged implicit work decisions")
	}

	expectedPublicPaths := []string{waveshared.Absolute("/tmp/public/logo.svg")}
	if !reflect.DeepEqual(work.Build.PublicStaticChangedFilePaths, expectedPublicPaths) {
		t.Fatalf("public static changed paths=%v, want %v", work.Build.PublicStaticChangedFilePaths, expectedPublicPaths)
	}

	expectedPrivatePaths := []string{waveshared.Absolute("/tmp/private/home.html")}
	if !reflect.DeepEqual(work.Build.PrivateStaticChangedFilePaths, expectedPrivatePaths) {
		t.Fatalf("private static changed paths=%v, want %v", work.Build.PrivateStaticChangedFilePaths, expectedPrivatePaths)
	}
}

func TestWorkSetAddImplicitWork(t *testing.T) {
	t.Run("go files imply compile and restart", func(t *testing.T) {
		work := &devserver.WorkSet{}
		work.AddImplicitWork(devserver.ClassifiedEvent{FileType: devserver.FileTypeGo})
		if !work.Build.CompileGo || !work.Restart.RestartApp {
			t.Fatalf(
				"expected compile+restart, got compile=%v restart=%v",
				work.Build.CompileGo,
				work.Restart.RestartApp,
			)
		}
	})

	t.Run("run-on-change-only suppresses implicit work", func(t *testing.T) {
		work := &devserver.WorkSet{}
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType:    devserver.FileTypeGo,
			WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
		})
		if work.Build.CompileGo || work.Restart.RestartApp {
			t.Fatalf(
				"expected no implicit work, got compile=%v restart=%v",
				work.Build.CompileGo,
				work.Restart.RestartApp,
			)
		}
	})

	t.Run("other file with recompile implies restart", func(t *testing.T) {
		work := &devserver.WorkSet{}
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypeOther,
			WatchedFile: &wave.WatchedFile{
				RecompileGoBinary: true,
			},
		})
		if !work.Build.CompileGo || !work.Restart.RestartApp {
			t.Fatalf(
				"expected compile+restart, got compile=%v restart=%v",
				work.Build.CompileGo,
				work.Restart.RestartApp,
			)
		}
	})

	t.Run("revalidate preference is recorded", func(t *testing.T) {
		work := &devserver.WorkSet{}
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypeOther,
			WatchedFile: &wave.WatchedFile{
				OnlyRunClientDefinedRevalidateFunc: true,
			},
		})
		if !work.PreferRevalidate {
			t.Fatal("expected preferRevalidate=true")
		}
	})

	t.Run("critical css file requests css rebuild and can request hard reload", func(t *testing.T) {
		work := &devserver.WorkSet{}
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypeCriticalCSS,
			WatchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		})
		if !work.Build.BuildCriticalCSS {
			t.Fatal("expected build.buildCriticalCSS=true")
		}
		if !work.Restart.RestartApp {
			t.Fatal("expected restart.restartApp=true when critical css watched file requests hard reload")
		}
	})

	t.Run("normal css file requests css rebuild and can request hard reload", func(t *testing.T) {
		work := &devserver.WorkSet{}
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypeNormalCSS,
			WatchedFile: &wave.WatchedFile{
				RecompileGoBinary: true,
			},
		})
		if !work.Build.BuildNormalCSS {
			t.Fatal("expected build.buildNormalCSS=true")
		}
		if !work.Restart.RestartApp {
			t.Fatal("expected restart.restartApp=true when normal css watched file requests hard reload")
		}
	})

	t.Run("shared critical+normal css file requests both rebuilds and can request hard reload", func(t *testing.T) {
		work := &devserver.WorkSet{}
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypeCriticalAndNormalCSS,
			WatchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		})
		if !work.Build.BuildCriticalCSS {
			t.Fatal("expected build.buildCriticalCSS=true")
		}
		if !work.Build.BuildNormalCSS {
			t.Fatal("expected build.buildNormalCSS=true")
		}
		if !work.Restart.RestartApp {
			t.Fatal("expected restart.restartApp=true when shared css watched file requests hard reload")
		}
	})

	t.Run("public static file requests public file processing", func(t *testing.T) {
		work := &devserver.WorkSet{}
		changedPublicFilePath := "/tmp/public/logo.svg"
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypePublicStatic,
			Event: fsnotify.Event{
				Name: changedPublicFilePath,
			},
		})
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypePublicStatic,
			Event: fsnotify.Event{
				Name: changedPublicFilePath,
			},
		})
		if !work.Build.ProcessPublicFiles {
			t.Fatal("expected build.processPublicFiles=true")
		}
		if len(work.Build.PublicStaticChangedFilePaths) != 1 {
			t.Fatalf("expected one deduplicated public static path, got %#v", work.Build.PublicStaticChangedFilePaths)
		}
		if work.Build.PublicStaticChangedFilePaths[0] != changedPublicFilePath {
			t.Fatalf(
				"expected tracked public static path %q, got %#v",
				changedPublicFilePath,
				work.Build.PublicStaticChangedFilePaths,
			)
		}
	})

	t.Run("public static changed paths are normalized and deduplicated by location", func(t *testing.T) {
		work := &devserver.WorkSet{}

		root := t.TempDir()
		canonicalFilePath := filepath.Join(root, "static", "public", "logo.svg")
		equivalentFilePath := filepath.Join(root, "static", "public", ".", "logo.svg")
		expectedNormalizedPath := waveshared.Absolute(canonicalFilePath)

		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypePublicStatic,
			Event: fsnotify.Event{
				Name: canonicalFilePath,
			},
		})
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypePublicStatic,
			Event: fsnotify.Event{
				Name: equivalentFilePath,
			},
		})

		if len(work.Build.PublicStaticChangedFilePaths) != 1 {
			t.Fatalf(
				"expected one normalized public static path, got %#v",
				work.Build.PublicStaticChangedFilePaths,
			)
		}
		if work.Build.PublicStaticChangedFilePaths[0] != expectedNormalizedPath {
			t.Fatalf(
				"expected normalized public static path %q, got %#v",
				expectedNormalizedPath,
				work.Build.PublicStaticChangedFilePaths,
			)
		}
	})

	t.Run("private static file requests private file processing", func(t *testing.T) {
		work := &devserver.WorkSet{}
		firstPrivatePath := "/tmp/private/a.txt"
		secondPrivatePath := "/tmp/private/b.txt"
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypePrivateStatic,
			Event: fsnotify.Event{
				Name: firstPrivatePath,
			},
		})
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypePrivateStatic,
			Event: fsnotify.Event{
				Name: secondPrivatePath,
			},
		})
		if !work.Build.ProcessPrivateFiles {
			t.Fatal("expected build.processPrivateFiles=true")
		}
		if len(work.Build.PrivateStaticChangedFilePaths) != 2 {
			t.Fatalf("expected two tracked private static paths, got %#v", work.Build.PrivateStaticChangedFilePaths)
		}
	})

	t.Run("other watched file can request restart without go compile", func(t *testing.T) {
		work := &devserver.WorkSet{}
		work.AddImplicitWork(devserver.ClassifiedEvent{
			FileType: devserver.FileTypeOther,
			WatchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		})
		if work.Build.CompileGo {
			t.Fatal("did not expect build.compileGo=true")
		}
		if !work.Restart.RestartApp {
			t.Fatal("expected restart.restartApp=true")
		}
	})
}

func TestWorkSetDetermineBrowserBehavior(t *testing.T) {
	t.Run("restart takes precedence", func(t *testing.T) {
		work := &devserver.WorkSet{
			Restart: devserver.RestartPhaseDecision{RestartApp: true},
		}
		work.DetermineBrowserBehavior(true)

		if work.Browser.Action != devserver.BrowserPhaseActionHardReload ||
			!work.Browser.WaitForApp ||
			!work.Browser.WaitForVite {
			t.Fatalf(
				"expected hard reload + wait for restart path, got action=%v waitApp=%v waitVite=%v",
				work.Browser.Action,
				work.Browser.WaitForApp,
				work.Browser.WaitForVite,
			)
		}
	})

	t.Run("revalidate preference overrides hot reload optimizations", func(t *testing.T) {
		work := &devserver.WorkSet{
			PreferRevalidate: true,
			Build: devserver.BuildPhaseDecision{
				BuildNormalCSS: true,
			},
		}
		work.DetermineBrowserBehavior(true)

		if work.Browser.Action != devserver.BrowserPhaseActionRevalidate {
			t.Fatalf("expected revalidate action, got %v", work.Browser.Action)
		}
	})

	t.Run("css-only work uses css hot reload", func(t *testing.T) {
		work := &devserver.WorkSet{
			Build: devserver.BuildPhaseDecision{
				BuildCriticalCSS: true,
			},
		}
		work.DetermineBrowserBehavior(true)
		if work.Browser.Action != devserver.BrowserPhaseActionHotReloadCSS {
			t.Fatalf("expected hot reload css action, got %v", work.Browser.Action)
		}
	})

	t.Run("public static work invalidates vite", func(t *testing.T) {
		work := &devserver.WorkSet{
			Build: devserver.BuildPhaseDecision{
				ProcessPublicFiles: true,
			},
		}
		work.DetermineBrowserBehavior(true)
		if work.Browser.Action != devserver.BrowserPhaseActionInvalidateVite {
			t.Fatalf("expected invalidate vite action, got %v", work.Browser.Action)
		}
	})

	t.Run("private static work uses full reload", func(t *testing.T) {
		work := &devserver.WorkSet{
			Build: devserver.BuildPhaseDecision{
				ProcessPrivateFiles: true,
			},
		}
		work.DetermineBrowserBehavior(false)

		if work.Browser.Action != devserver.BrowserPhaseActionHardReload ||
			!work.Browser.WaitForApp {
			t.Fatalf(
				"expected hard reload + waitApp for private static path, got action=%v waitApp=%v",
				work.Browser.Action,
				work.Browser.WaitForApp,
			)
		}
		if work.Browser.WaitForVite {
			t.Fatal("expected waitForVite=false when vite is disabled")
		}
	})
}

func TestDeriveBrowserPhaseResolutionForWorkSet(t *testing.T) {
	testCases := []struct {
		Name               string
		BuildDecision      devserver.BuildPhaseDecision
		RestartDecision    devserver.RestartPhaseDecision
		PreferRevalidate   bool
		UsingVite          bool
		ExpectedResolution devserver.BrowserPhaseResolution
	}{
		{
			Name:             "restart takes precedence over revalidate preference",
			RestartDecision:  devserver.RestartPhaseDecision{RestartApp: true},
			PreferRevalidate: true,
			UsingVite:        true,
			ExpectedResolution: devserver.BrowserPhaseResolution{
				Action:         devserver.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    true,
			},
		},
		{
			Name:             "revalidate takes precedence over css-only optimization",
			BuildDecision:    devserver.BuildPhaseDecision{BuildNormalCSS: true},
			PreferRevalidate: true,
			UsingVite:        false,
			ExpectedResolution: devserver.BrowserPhaseResolution{
				Action:         devserver.BrowserPhaseActionRevalidate,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    false,
			},
		},
		{
			Name:          "css-only work uses hot reload css",
			BuildDecision: devserver.BuildPhaseDecision{BuildCriticalCSS: true},
			ExpectedResolution: devserver.BrowserPhaseResolution{
				Action: devserver.BrowserPhaseActionHotReloadCSS,
			},
		},
		{
			Name:          "public static work invalidates vite",
			BuildDecision: devserver.BuildPhaseDecision{ProcessPublicFiles: true},
			ExpectedResolution: devserver.BrowserPhaseResolution{
				Action: devserver.BrowserPhaseActionInvalidateVite,
			},
		},
		{
			Name: "public static invalidation takes precedence over private static full reload",
			BuildDecision: devserver.BuildPhaseDecision{
				ProcessPublicFiles:  true,
				ProcessPrivateFiles: true,
			},
			ExpectedResolution: devserver.BrowserPhaseResolution{
				Action: devserver.BrowserPhaseActionInvalidateVite,
			},
		},
		{
			Name:          "private static work uses hard reload",
			BuildDecision: devserver.BuildPhaseDecision{ProcessPrivateFiles: true},
			UsingVite:     true,
			ExpectedResolution: devserver.BrowserPhaseResolution{
				Action:         devserver.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    true,
			},
		},
		{
			Name: "css and non-public static work uses hard reload",
			BuildDecision: devserver.BuildPhaseDecision{
				BuildNormalCSS:      true,
				ProcessPrivateFiles: true,
			},
			UsingVite: true,
			ExpectedResolution: devserver.BrowserPhaseResolution{
				Action:         devserver.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    true,
			},
		},
		{
			Name: "no build work produces no browser action",
			ExpectedResolution: devserver.BrowserPhaseResolution{
				Action: devserver.BrowserPhaseActionNone,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			resolution := devserver.DeriveBrowserPhaseResolutionForWorkSet(
				testCase.BuildDecision,
				testCase.RestartDecision,
				testCase.PreferRevalidate,
				testCase.UsingVite,
			)
			if !reflect.DeepEqual(resolution, testCase.ExpectedResolution) {
				t.Fatalf(
					"devserver.DeriveBrowserPhaseResolutionForWorkSet()=%#v, want %#v",
					resolution,
					testCase.ExpectedResolution,
				)
			}
		})
	}
}

func TestWorkSetResolve_CompileImpliesRestart(t *testing.T) {
	work := &devserver.WorkSet{
		Build: devserver.BuildPhaseDecision{
			CompileGo: true,
		},
	}

	work.Resolve(false)
	if !work.Restart.RestartApp {
		t.Fatal("expected restart.restartApp=true when build.compileGo=true")
	}
	if work.Browser.Action != devserver.BrowserPhaseActionHardReload {
		t.Fatal("expected hard reload action on restart path")
	}
}
