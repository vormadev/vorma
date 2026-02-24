package eventpipeline_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/wavecore"
)

func TestWorkSetAddFromRefreshAction(t *testing.T) {
	work := &eventpipeline.WorkSet{}
	work.AddFromRefreshAction(wave.RefreshAction{
		TriggerRestart: true,
		RecompileGo:    true,
		ReloadBrowser:  true,
		WaitForApp:     true,
		WaitForVite:    true,
	})
	work.AddFromRefreshAction(wave.RefreshAction{
		FrameworkRuntimeReloadRequest: &wave.FrameworkRuntimeReloadRequest{
			EndpointPath:    "reload-routes",
			ReloadAttemptID: "attempt-1",
			ExpectedBuildID: "build-1",
			ReloadTrigger:   "routes-watch",
		},
	})

	if !work.Restart.RestartApp {
		t.Fatal("expected restart.restartApp=true")
	}
	if !work.Build.CompileGo {
		t.Fatal("expected build.compileGo=true")
	}
	if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
		!work.Browser.WaitForApp ||
		!work.Browser.WaitForVite {
		t.Fatalf(
			"expected hard reload + wait flags, got action=%v waitApp=%v waitVite=%v",
			work.Browser.Action,
			work.Browser.WaitForApp,
			work.Browser.WaitForVite,
		)
	}
	if len(work.FrameworkRuntimeReloadRequests) != 1 {
		t.Fatalf(
			"expected one framework runtime reload request, got %d",
			len(work.FrameworkRuntimeReloadRequests),
		)
	}
	if work.FrameworkRuntimeReloadRequests[0].EndpointPath != "/reload-routes" {
		t.Fatalf(
			"expected normalized endpoint path, got %q",
			work.FrameworkRuntimeReloadRequests[0].EndpointPath,
		)
	}
}

func TestDeriveRefreshActionWorkMutationDecision(t *testing.T) {
	testCases := []struct {
		Name                 string
		Action               wave.RefreshAction
		ExpectedWorkMutation eventpipeline.RefreshActionWorkMutationDecision
	}{
		{
			Name: "restart action sets restart and compile-go when requested",
			Action: wave.RefreshAction{
				TriggerRestart: true,
				RecompileGo:    true,
			},
			ExpectedWorkMutation: eventpipeline.RefreshActionWorkMutationDecision{
				RestartApp: true,
				CompileGo:  true,
			},
		},
		{
			Name: "reload action requests hard reload",
			Action: wave.RefreshAction{
				ReloadBrowser: true,
			},
			ExpectedWorkMutation: eventpipeline.RefreshActionWorkMutationDecision{
				RequestBrowserAction: true,
				BrowserAction:        eventpipeline.BrowserPhaseActionHardReload,
			},
		},
		{
			Name: "wait flags are carried",
			Action: wave.RefreshAction{
				WaitForApp:  true,
				WaitForVite: true,
			},
			ExpectedWorkMutation: eventpipeline.RefreshActionWorkMutationDecision{
				WaitForApp:  true,
				WaitForVite: true,
			},
		},
		{
			Name: "framework runtime reload request is carried",
			Action: wave.RefreshAction{
				FrameworkRuntimeReloadRequest: &wave.FrameworkRuntimeReloadRequest{
					EndpointPath:    "/reload-template",
					ReloadAttemptID: "attempt-7",
					ExpectedBuildID: "build-7",
					ReloadTrigger:   "template-watch",
				},
			},
			ExpectedWorkMutation: eventpipeline.RefreshActionWorkMutationDecision{
				FrameworkRuntimeReloadRequest: &wave.FrameworkRuntimeReloadRequest{
					EndpointPath:    "/reload-template",
					ReloadAttemptID: "attempt-7",
					ExpectedBuildID: "build-7",
					ReloadTrigger:   "template-watch",
				},
			},
		},
		{
			Name:                 "zero action maps to zero mutation",
			Action:               wave.RefreshAction{},
			ExpectedWorkMutation: eventpipeline.RefreshActionWorkMutationDecision{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			workMutationDecision := eventpipeline.DeriveRefreshActionWorkMutationDecision(
				testCase.Action,
			)
			if !reflect.DeepEqual(
				workMutationDecision,
				testCase.ExpectedWorkMutation,
			) {
				t.Fatalf(
					"eventpipeline.DeriveRefreshActionWorkMutationDecision()=%#v, want %#v",
					workMutationDecision,
					testCase.ExpectedWorkMutation,
				)
			}
		})
	}
}

func TestWorkSetApplyRefreshActionWorkMutationDecision(t *testing.T) {
	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action:      eventpipeline.BrowserPhaseActionRevalidate,
			WaitForApp:  false,
			WaitForVite: false,
		},
	}

	work.ApplyRefreshActionWorkMutationDecision(
		eventpipeline.RefreshActionWorkMutationDecision{
			RestartApp:           true,
			CompileGo:            true,
			RequestBrowserAction: true,
			BrowserAction:        eventpipeline.BrowserPhaseActionHardReload,
			WaitForApp:           true,
			WaitForVite:          true,
			FrameworkRuntimeReloadRequest: &wave.FrameworkRuntimeReloadRequest{
				EndpointPath:    "reload-routes",
				ReloadAttemptID: "attempt-1",
				ExpectedBuildID: "build-1",
				ReloadTrigger:   "route-watch",
			},
		},
	)
	work.ApplyRefreshActionWorkMutationDecision(
		eventpipeline.RefreshActionWorkMutationDecision{
			FrameworkRuntimeReloadRequest: &wave.FrameworkRuntimeReloadRequest{
				EndpointPath:    "/reload-routes",
				ReloadAttemptID: "attempt-2",
				ExpectedBuildID: "build-2",
				ReloadTrigger:   "route-watch-2",
			},
		},
	)
	work.ApplyRefreshActionWorkMutationDecision(
		eventpipeline.RefreshActionWorkMutationDecision{
			FrameworkRuntimeReloadRequest: &wave.FrameworkRuntimeReloadRequest{
				EndpointPath: "/reload-routes",
			},
		},
	)
	work.ApplyRefreshActionWorkMutationDecision(
		eventpipeline.RefreshActionWorkMutationDecision{
			FrameworkRuntimeReloadRequest: &wave.FrameworkRuntimeReloadRequest{
				EndpointPath:    "/reload-template",
				ReloadAttemptID: "attempt-3",
				ExpectedBuildID: "build-3",
				ReloadTrigger:   "template-watch",
			},
		},
	)
	work.ApplyRefreshActionWorkMutationDecision(
		eventpipeline.RefreshActionWorkMutationDecision{},
	)

	if !work.Restart.RestartApp {
		t.Fatal("expected restart.restartApp=true")
	}
	if !work.Build.CompileGo {
		t.Fatal("expected build.compileGo=true")
	}
	if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
		t.Fatalf(
			"expected browser hard reload action, got %v",
			work.Browser.Action,
		)
	}
	if !work.Browser.WaitForApp || !work.Browser.WaitForVite {
		t.Fatalf(
			"expected wait flags true, got waitApp=%v waitVite=%v",
			work.Browser.WaitForApp,
			work.Browser.WaitForVite,
		)
	}
	if len(work.FrameworkRuntimeReloadRequests) != 2 {
		t.Fatalf(
			"expected two deduplicated framework runtime reload requests, got %d",
			len(work.FrameworkRuntimeReloadRequests),
		)
	}
	if got, want := work.FrameworkRuntimeReloadRequests[0].EndpointPath, "/reload-routes"; got != want {
		t.Fatalf("endpoint path=%q, want %q", got, want)
	}
	if got, want := work.FrameworkRuntimeReloadRequests[0].ReloadAttemptID, "attempt-2"; got != want {
		t.Fatalf("reload attempt id=%q, want %q", got, want)
	}
	if got, want := work.FrameworkRuntimeReloadRequests[0].ExpectedBuildID, "build-2"; got != want {
		t.Fatalf("expected build id=%q, want %q", got, want)
	}
	if got, want := work.FrameworkRuntimeReloadRequests[0].ReloadTrigger, "route-watch-2"; got != want {
		t.Fatalf("reload trigger=%q, want %q", got, want)
	}
	if got, want := work.FrameworkRuntimeReloadRequests[1].EndpointPath, "/reload-template"; got != want {
		t.Fatalf("endpoint path=%q, want %q", got, want)
	}
}

func TestWorkSetApplyRefreshActions(t *testing.T) {
	t.Run(
		"returns restart request and stops processing remaining actions",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}

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
			if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
				t.Fatal("expected first non-restart action to be applied")
			}
			if work.Browser.WaitForApp {
				t.Fatal(
					"expected actions after restart request not to be applied",
				)
			}
		},
	)

	t.Run(
		"keeps actions before restart and surfaces restart recompile signal",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}

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
			if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
				t.Fatalf(
					"expected pre-restart reload action to be applied, got %v",
					work.Browser.Action,
				)
			}
			if !work.Browser.WaitForApp {
				t.Fatal("expected pre-restart wait-for-app to be applied")
			}
			if work.Browser.WaitForVite {
				t.Fatal("expected post-restart actions not to be applied")
			}
		},
	)

	t.Run(
		"merges stronger restart request from later action",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}

			result := work.ApplyRefreshActions([]wave.RefreshAction{
				{ReloadBrowser: true},
				{TriggerRestart: true, RecompileGo: false},
				{TriggerRestart: true, RecompileGo: true},
				{WaitForVite: true},
			})

			if !result.RestartRequested {
				t.Fatal("expected restartRequested=true")
			}
			if !result.RecompileGo {
				t.Fatal(
					"expected recompileGo=true from strongest restart action",
				)
			}
			if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
				t.Fatalf(
					"expected pre-restart reload action to be applied, got %v",
					work.Browser.Action,
				)
			}
			if work.Browser.WaitForVite {
				t.Fatal("expected post-restart actions not to be applied")
			}
		},
	)

	t.Run("merges non-restart actions into workset", func(t *testing.T) {
		work := &eventpipeline.WorkSet{}

		result := work.ApplyRefreshActions([]wave.RefreshAction{
			{ReloadBrowser: true, WaitForApp: true},
			{WaitForVite: true},
		})

		if result.RestartRequested {
			t.Fatal("expected restartRequested=false")
		}
		if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
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

		reductionDecision := eventpipeline.ReduceRefreshActionsInStableOrder(
			input,
		)
		if reductionDecision.ApplicationResult.RestartRequested {
			t.Fatal("expected restartRequested=false")
		}
		applied := reductionDecision.ActionsBeforeRestart
		if len(applied) != len(input) {
			t.Fatalf(
				"actionsBeforeRestart count=%d, want %d",
				len(applied),
				len(input),
			)
		}
		for i := range input {
			if applied[i] != input[i] {
				t.Fatalf("applied[%d]=%#v, want %#v", i, applied[i], input[i])
			}
		}
	})

	t.Run(
		"merges restart actions and excludes later non-restart actions",
		func(t *testing.T) {
			input := []wave.RefreshAction{
				{ReloadBrowser: true},
				{TriggerRestart: true, RecompileGo: false},
				{TriggerRestart: true, RecompileGo: true},
				{WaitForApp: true},
			}

			reductionDecision := eventpipeline.ReduceRefreshActionsInStableOrder(
				input,
			)
			if !reductionDecision.ApplicationResult.RestartRequested {
				t.Fatal("expected restartRequested=true")
			}
			if !reductionDecision.ApplicationResult.RecompileGo {
				t.Fatal(
					"expected merged restart actions to preserve recompileGo=true",
				)
			}
			applied := reductionDecision.ActionsBeforeRestart
			if len(applied) != 1 {
				t.Fatalf("actionsBeforeRestart count=%d, want 1", len(applied))
			}
			if !applied[0].ReloadBrowser {
				t.Fatalf("unexpected applied Actions: %#v", applied)
			}
		},
	)

	t.Run(
		"restart at first action yields no pre-restart actions",
		func(t *testing.T) {
			reductionDecision := eventpipeline.ReduceRefreshActionsInStableOrder(
				[]wave.RefreshAction{
					{TriggerRestart: true, RecompileGo: true},
					{ReloadBrowser: true},
				},
			)
			if !reductionDecision.ApplicationResult.RestartRequested {
				t.Fatal("expected restartRequested=true")
			}
			if !reductionDecision.ApplicationResult.RecompileGo {
				t.Fatal("expected recompileGo=true from first restart action")
			}
			if len(reductionDecision.ActionsBeforeRestart) != 0 {
				t.Fatalf(
					"expected zero pre-restart actions, got %#v",
					reductionDecision.ActionsBeforeRestart,
				)
			}
		},
	)
}

func TestDeriveImplicitWorkDecisionForClassifiedEvent(t *testing.T) {
	testCases := []struct {
		Name             string
		ClassifiedEvent  eventpipeline.ClassifiedEvent
		ExpectedDecision eventpipeline.ImplicitWorkDecision
	}{
		{
			Name: "go file compiles and restarts",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypeGo,
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				CompileGo:  true,
				RestartApp: true,
			},
		},
		{
			Name: "run-on-change-only disables implicit work",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType:    eventpipeline.FileTypeGo,
				WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{},
		},
		{
			Name: "revalidate preference is recorded",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					OnlyRunClientDefinedRevalidateFunc: true,
				},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				PreferRevalidate: true,
			},
		},
		{
			Name: "critical css can request hard reload",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType:    eventpipeline.FileTypeCriticalCSS,
				WatchedFile: &wave.WatchedFile{RestartApp: true},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				BuildCriticalCSS: true,
				RestartApp:       true,
			},
		},
		{
			Name: "normal css without hard reload only schedules css rebuild",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType:    eventpipeline.FileTypeNormalCSS,
				WatchedFile: &wave.WatchedFile{},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				BuildNormalCSS: true,
			},
		},
		{
			Name: "shared css with hard-reload request rebuilds both",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType:    eventpipeline.FileTypeCriticalAndNormalCSS,
				WatchedFile: &wave.WatchedFile{RecompileGoBinary: true},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				BuildCriticalCSS: true,
				BuildNormalCSS:   true,
				RestartApp:       true,
			},
		},
		{
			Name: "public static processing tracks changed file path",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypePublicStatic,
				Event:    fsnotify.Event{Name: "/tmp/public/logo.svg"},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				ProcessPublicFiles:          true,
				PublicStaticChangedFilePath: "/tmp/public/logo.svg",
			},
		},
		{
			Name: "private static processing tracks changed file path",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypePrivateStatic,
				Event:    fsnotify.Event{Name: "/tmp/private/home.html"},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				ProcessPrivateFiles:          true,
				PrivateStaticChangedFilePath: "/tmp/private/home.html",
			},
		},
		{
			Name: "other watched file recompile implies restart",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					RecompileGoBinary: true,
				},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				CompileGo:  true,
				RestartApp: true,
			},
		},
		{
			Name: "other watched file restart without recompile",
			ClassifiedEvent: eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					RestartApp: true,
				},
			},
			ExpectedDecision: eventpipeline.ImplicitWorkDecision{
				RestartApp: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := eventpipeline.DeriveImplicitWorkDecisionForClassifiedEvent(
				testCase.ClassifiedEvent,
			)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"eventpipeline.DeriveImplicitWorkDecisionForClassifiedEvent()=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestWorkSetApplyImplicitWorkDecision_MergesBuildRestartAndPathWork(
	t *testing.T,
) {
	work := &eventpipeline.WorkSet{}

	firstDecision := eventpipeline.ImplicitWorkDecision{
		CompileGo:                   true,
		BuildCriticalCSS:            true,
		ProcessPublicFiles:          true,
		PublicStaticChangedFilePath: "/tmp/public/logo.svg",
	}
	secondDecision := eventpipeline.ImplicitWorkDecision{
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
		t.Fatal(
			"expected build.compileGo=true after merged implicit work decisions",
		)
	}
	if !work.Build.BuildCriticalCSS {
		t.Fatal(
			"expected build.buildCriticalCSS=true after merged implicit work decisions",
		)
	}
	if !work.Build.ProcessPublicFiles {
		t.Fatal(
			"expected build.processPublicFiles=true after merged implicit work decisions",
		)
	}
	if !work.Build.ProcessPrivateFiles {
		t.Fatal(
			"expected build.processPrivateFiles=true after merged implicit work decisions",
		)
	}
	if !work.Restart.RestartApp {
		t.Fatal(
			"expected restart.restartApp=true after merged implicit work decisions",
		)
	}
	if !work.PreferRevalidate {
		t.Fatal(
			"expected preferRevalidate=true after merged implicit work decisions",
		)
	}

	expectedPublicPaths := []string{wavecore.Absolute("/tmp/public/logo.svg")}
	if !reflect.DeepEqual(
		work.Build.PublicStaticChangedFilePaths,
		expectedPublicPaths,
	) {
		t.Fatalf(
			"public static changed paths=%v, want %v",
			work.Build.PublicStaticChangedFilePaths,
			expectedPublicPaths,
		)
	}

	expectedPrivatePaths := []string{
		wavecore.Absolute("/tmp/private/home.html"),
	}
	if !reflect.DeepEqual(
		work.Build.PrivateStaticChangedFilePaths,
		expectedPrivatePaths,
	) {
		t.Fatalf(
			"private static changed paths=%v, want %v",
			work.Build.PrivateStaticChangedFilePaths,
			expectedPrivatePaths,
		)
	}
}

func TestWorkSetAddImplicitWork(t *testing.T) {
	t.Run("go files imply compile and restart", func(t *testing.T) {
		work := &eventpipeline.WorkSet{}
		work.AddImplicitWork(
			eventpipeline.ClassifiedEvent{FileType: eventpipeline.FileTypeGo},
		)
		if !work.Build.CompileGo || !work.Restart.RestartApp {
			t.Fatalf(
				"expected compile+restart, got compile=%v restart=%v",
				work.Build.CompileGo,
				work.Restart.RestartApp,
			)
		}
	})

	t.Run("run-on-change-only suppresses implicit work", func(t *testing.T) {
		work := &eventpipeline.WorkSet{}
		work.AddImplicitWork(eventpipeline.ClassifiedEvent{
			FileType:    eventpipeline.FileTypeGo,
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
		work := &eventpipeline.WorkSet{}
		work.AddImplicitWork(eventpipeline.ClassifiedEvent{
			FileType: eventpipeline.FileTypeOther,
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
		work := &eventpipeline.WorkSet{}
		work.AddImplicitWork(eventpipeline.ClassifiedEvent{
			FileType: eventpipeline.FileTypeOther,
			WatchedFile: &wave.WatchedFile{
				OnlyRunClientDefinedRevalidateFunc: true,
			},
		})
		if !work.PreferRevalidate {
			t.Fatal("expected preferRevalidate=true")
		}
	})

	t.Run(
		"critical css file requests css rebuild and can request hard reload",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypeCriticalCSS,
				WatchedFile: &wave.WatchedFile{
					RestartApp: true,
				},
			})
			if !work.Build.BuildCriticalCSS {
				t.Fatal("expected build.buildCriticalCSS=true")
			}
			if !work.Restart.RestartApp {
				t.Fatal(
					"expected restart.restartApp=true when critical css watched file requests hard reload",
				)
			}
		},
	)

	t.Run(
		"normal css file requests css rebuild and can request hard reload",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypeNormalCSS,
				WatchedFile: &wave.WatchedFile{
					RecompileGoBinary: true,
				},
			})
			if !work.Build.BuildNormalCSS {
				t.Fatal("expected build.buildNormalCSS=true")
			}
			if !work.Restart.RestartApp {
				t.Fatal(
					"expected restart.restartApp=true when normal css watched file requests hard reload",
				)
			}
		},
	)

	t.Run(
		"shared critical+normal css file requests both rebuilds and can request hard reload",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypeCriticalAndNormalCSS,
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
				t.Fatal(
					"expected restart.restartApp=true when shared css watched file requests hard reload",
				)
			}
		},
	)

	t.Run(
		"public static file requests public file processing",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			changedPublicFilePath := "/tmp/public/logo.svg"
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypePublicStatic,
				Event: fsnotify.Event{
					Name: changedPublicFilePath,
				},
			})
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypePublicStatic,
				Event: fsnotify.Event{
					Name: changedPublicFilePath,
				},
			})
			if !work.Build.ProcessPublicFiles {
				t.Fatal("expected build.processPublicFiles=true")
			}
			if len(work.Build.PublicStaticChangedFilePaths) != 1 {
				t.Fatalf(
					"expected one deduplicated public static path, got %#v",
					work.Build.PublicStaticChangedFilePaths,
				)
			}
			if work.Build.PublicStaticChangedFilePaths[0] != changedPublicFilePath {
				t.Fatalf(
					"expected tracked public static path %q, got %#v",
					changedPublicFilePath,
					work.Build.PublicStaticChangedFilePaths,
				)
			}
		},
	)

	t.Run(
		"public static changed paths are normalized and deduplicated by location",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}

			root := t.TempDir()
			canonicalFilePath := filepath.Join(
				root,
				"static",
				"public",
				"logo.svg",
			)
			equivalentFilePath := filepath.Join(
				root,
				"static",
				"public",
				".",
				"logo.svg",
			)
			expectedNormalizedPath := wavecore.Absolute(canonicalFilePath)

			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypePublicStatic,
				Event: fsnotify.Event{
					Name: canonicalFilePath,
				},
			})
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypePublicStatic,
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
		},
	)

	t.Run(
		"private static file requests private file processing",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			firstPrivatePath := "/tmp/private/a.txt"
			secondPrivatePath := "/tmp/private/b.txt"
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypePrivateStatic,
				Event: fsnotify.Event{
					Name: firstPrivatePath,
				},
			})
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypePrivateStatic,
				Event: fsnotify.Event{
					Name: secondPrivatePath,
				},
			})
			if !work.Build.ProcessPrivateFiles {
				t.Fatal("expected build.processPrivateFiles=true")
			}
			if len(work.Build.PrivateStaticChangedFilePaths) != 2 {
				t.Fatalf(
					"expected two tracked private static paths, got %#v",
					work.Build.PrivateStaticChangedFilePaths,
				)
			}
		},
	)

	t.Run(
		"other watched file can request restart without go compile",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			work.AddImplicitWork(eventpipeline.ClassifiedEvent{
				FileType: eventpipeline.FileTypeOther,
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
		},
	)
}

func TestWorkSetDetermineBrowserBehavior(t *testing.T) {
	t.Run("restart takes precedence", func(t *testing.T) {
		work := &eventpipeline.WorkSet{
			Restart: eventpipeline.RestartPhaseDecision{RestartApp: true},
		}
		work.DetermineBrowserBehavior(true)

		if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
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

	t.Run(
		"revalidate preference overrides hot reload optimizations",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{
				PreferRevalidate: true,
				Build: eventpipeline.BuildPhaseDecision{
					BuildNormalCSS: true,
				},
			}
			work.DetermineBrowserBehavior(true)

			if work.Browser.Action != eventpipeline.BrowserPhaseActionRevalidate {
				t.Fatalf(
					"expected revalidate action, got %v",
					work.Browser.Action,
				)
			}
		},
	)

	t.Run("css-only work uses css hot reload", func(t *testing.T) {
		work := &eventpipeline.WorkSet{
			Build: eventpipeline.BuildPhaseDecision{
				BuildCriticalCSS: true,
			},
		}
		work.DetermineBrowserBehavior(true)
		if work.Browser.Action != eventpipeline.BrowserPhaseActionHotReloadCSS {
			t.Fatalf(
				"expected hot reload css action, got %v",
				work.Browser.Action,
			)
		}
	})

	t.Run("public static work invalidates vite", func(t *testing.T) {
		work := &eventpipeline.WorkSet{
			Build: eventpipeline.BuildPhaseDecision{
				ProcessPublicFiles: true,
			},
		}
		work.DetermineBrowserBehavior(true)
		if work.Browser.Action != eventpipeline.BrowserPhaseActionInvalidateVite {
			t.Fatalf(
				"expected invalidate vite action, got %v",
				work.Browser.Action,
			)
		}
	})

	t.Run("private static work uses full reload", func(t *testing.T) {
		work := &eventpipeline.WorkSet{
			Build: eventpipeline.BuildPhaseDecision{
				ProcessPrivateFiles: true,
			},
		}
		work.DetermineBrowserBehavior(false)

		if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
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
		BuildDecision      eventpipeline.BuildPhaseDecision
		RestartDecision    eventpipeline.RestartPhaseDecision
		PreferRevalidate   bool
		UsingVite          bool
		ExpectedResolution eventpipeline.BrowserPhaseResolution
	}{
		{
			Name: "restart takes precedence over revalidate preference",
			RestartDecision: eventpipeline.RestartPhaseDecision{
				RestartApp: true,
			},
			PreferRevalidate: true,
			UsingVite:        true,
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action:         eventpipeline.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    true,
			},
		},
		{
			Name: "revalidate takes precedence over css-only optimization",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				BuildNormalCSS: true,
			},
			PreferRevalidate: true,
			UsingVite:        false,
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action:         eventpipeline.BrowserPhaseActionRevalidate,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    false,
			},
		},
		{
			Name: "revalidate preference does not override public static invalidation",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				ProcessPublicFiles: true,
			},
			PreferRevalidate: true,
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action: eventpipeline.BrowserPhaseActionInvalidateVite,
			},
		},
		{
			Name: "revalidate preference does not override mixed public and private static reload",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				ProcessPublicFiles:  true,
				ProcessPrivateFiles: true,
			},
			PreferRevalidate: true,
			UsingVite:        true,
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action:         eventpipeline.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    true,
			},
		},
		{
			Name: "css-only work uses hot reload css",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				BuildCriticalCSS: true,
			},
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action: eventpipeline.BrowserPhaseActionHotReloadCSS,
			},
		},
		{
			Name: "public static work invalidates vite",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				ProcessPublicFiles: true,
			},
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action: eventpipeline.BrowserPhaseActionInvalidateVite,
			},
		},
		{
			Name: "private static full reload takes precedence over public static invalidation",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				ProcessPublicFiles:  true,
				ProcessPrivateFiles: true,
			},
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action:         eventpipeline.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    false,
			},
		},
		{
			Name: "non-css-only css work takes precedence over public static invalidation",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				ProcessPublicFiles: true,
				BuildNormalCSS:     true,
			},
			UsingVite: true,
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action:         eventpipeline.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    true,
			},
		},
		{
			Name: "private static work uses hard reload",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				ProcessPrivateFiles: true,
			},
			UsingVite: true,
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action:         eventpipeline.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    true,
			},
		},
		{
			Name: "css and non-public static work uses hard reload",
			BuildDecision: eventpipeline.BuildPhaseDecision{
				BuildNormalCSS:      true,
				ProcessPrivateFiles: true,
			},
			UsingVite: true,
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action:         eventpipeline.BrowserPhaseActionHardReload,
				ApplyWaitFlags: true,
				WaitForApp:     true,
				WaitForVite:    true,
			},
		},
		{
			Name: "no build work produces no browser action",
			ExpectedResolution: eventpipeline.BrowserPhaseResolution{
				Action: eventpipeline.BrowserPhaseActionNone,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			resolution := eventpipeline.DeriveBrowserPhaseResolutionForWorkSet(
				testCase.BuildDecision,
				testCase.RestartDecision,
				testCase.PreferRevalidate,
				testCase.UsingVite,
			)
			if !reflect.DeepEqual(resolution, testCase.ExpectedResolution) {
				t.Fatalf(
					"eventpipeline.DeriveBrowserPhaseResolutionForWorkSet()=%#v, want %#v",
					resolution,
					testCase.ExpectedResolution,
				)
			}
		})
	}
}

func TestWorkSetResolve_CompileImpliesRestart(t *testing.T) {
	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			CompileGo: true,
		},
	}

	work.Resolve(false)
	if !work.Restart.RestartApp {
		t.Fatal("expected restart.restartApp=true when build.compileGo=true")
	}
	if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
		t.Fatal("expected hard reload action on restart path")
	}
}
